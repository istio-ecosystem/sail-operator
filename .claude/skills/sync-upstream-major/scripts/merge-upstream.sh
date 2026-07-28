#!/usr/bin/env bash

# Copyright Alauda Mesh Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# shellcheck disable=SC2015  # p()/f() 恒为真，A && B || C 惯用法在此安全
# 步骤 1：前置检查 + 创建同步分支 + 合并上游 release 分支。
# 用法: merge-upstream.sh <上游release分支> <目标分支> <istio构建版本>...
#   构建版本至少 1 个（新大版本，如 1.30.3-asm-rc.4）；可再给上一大版本的若干构建（如 1.28.6-asm-r4）。
# 环境变量:
#   MESH_VERSION     覆盖自动推导的新 mesh 版本（缺省: 目标分支 Makefile.vendor.mk 的 VERSION 次版本 +1、patch 归 0）
#   CHART_BASE_URL   覆盖从 alauda-versions.yaml 推导的 charts 基地址
#   SKIP_CHART_CHECK =1 跳过 r2.dev charts 可达性检查（离线/测试用）
# 退出码: 0=MERGED 或 UP_TO_DATE  2=CONFLICT（正常，进入解冲突）  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
repo_root

UPSTREAM_BRANCH="${1:-}"
TARGET_BRANCH="${2:-}"
[[ -n "$UPSTREAM_BRANCH" && -n "$TARGET_BRANCH" && $# -ge 3 ]] \
  || die "用法: merge-upstream.sh <上游release分支> <目标分支> <istio构建版本>...  例: merge-upstream.sh release-1.30 main 1.30.3-asm-rc.4 1.28.6-asm-r4"
shift 2
BUILDS=("$@")

[[ "$UPSTREAM_BRANCH" =~ ^release-[0-9]+\.[0-9]+$ ]] || die "上游分支须形如 release-1.30，实际: $UPSTREAM_BRANCH"
NEW_MAJOR="${UPSTREAM_BRANCH#release-}"

# 校验构建版本格式，并按 istio 大版本分组为「新版本构建」和「上一版本构建」
NEW_BUILDS=() PREV_BUILDS=() PREV_MAJOR=""
for b in "${BUILDS[@]}"; do
  [[ "$b" =~ ^[0-9]+\.[0-9]+\.[0-9]+-asm-[a-z0-9][a-z0-9.-]*$ ]] \
    || die "构建版本格式不合法: $b（应形如 1.30.3-asm-rc.4 / 1.28.6-asm-r4）"
  m=$(istio_major "$(build_to_istio "$b")")
  if [[ "$m" == "$NEW_MAJOR" ]]; then
    NEW_BUILDS+=("$b")
  elif [[ -z "$PREV_MAJOR" || "$m" == "$PREV_MAJOR" ]]; then
    PREV_MAJOR="$m"; PREV_BUILDS+=("$b")
  else
    die "构建版本涉及超过两个大版本（$NEW_MAJOR / $PREV_MAJOR / $m），一次同步只处理新版本 + 上一版本"
  fi
done
[[ ${#NEW_BUILDS[@]} -ge 1 ]] || die "至少提供一个新大版本（$NEW_MAJOR）的构建版本，例如 ${NEW_MAJOR}.0-asm-r1"

# 工作区必须干净且不处于 merge 中
git rev-parse -q --verify MERGE_HEAD >/dev/null \
  && die "当前处于未完成的 merge 状态，先处理完（解决冲突并 commit，或 git merge --abort）"
git diff --quiet && git diff --cached --quiet \
  || die "工作区不干净，请先 commit 或 stash（不要让脚本擅自处理）"

info "fetch origin/$TARGET_BRANCH 与 upstream/$UPSTREAM_BRANCH ..."
git fetch origin "$TARGET_BRANCH" || die "fetch origin/$TARGET_BRANCH 失败（分支不存在？）"
git ls-remote --exit-code --heads upstream "$UPSTREAM_BRANCH" >/dev/null \
  || die "upstream 上不存在分支 $UPSTREAM_BRANCH"
git fetch upstream "$UPSTREAM_BRANCH"

# 从目标分支（合并前）的版本矩阵推导「应保留的上一大版本」
ensure_yq
VF_CONTENT=$(git show "origin/$TARGET_BRANCH:pkg/istioversion/alauda-versions.yaml")
EXISTING_NONEOL_MAJORS=$(yq -r '.versions[] | select(.eol != true) | .name' <<<"$VF_CONTENT" \
  | sed -E 's/^v//; s/-latest$//' | cut -d. -f1-2 | awk '!seen[$0]++')
EXPECTED_PREV=$(grep -vx "$NEW_MAJOR" <<<"$EXISTING_NONEOL_MAJORS" | head -1 || true)
if [[ ${#PREV_BUILDS[@]} -gt 0 ]]; then
  [[ "$PREV_MAJOR" == "$EXPECTED_PREV" ]] \
    || die "提供的上一大版本构建属于 $PREV_MAJOR，但按现有矩阵应保留的上一大版本是 ${EXPECTED_PREV:-（无）}"
else
  PREV_MAJOR="$EXPECTED_PREV"
  [[ -n "$PREV_MAJOR" ]] || warn "现有矩阵没有可保留的上一大版本，同步后矩阵只含 $NEW_MAJOR"
fi

# 推导新 mesh 版本与 channels
if [[ -z "${MESH_VERSION:-}" ]]; then
  CUR_VERSION=$(git show "origin/$TARGET_BRANCH:Makefile.vendor.mk" | sed -nE 's/^VERSION = ([0-9]+\.[0-9]+\.[0-9]+)$/\1/p')
  [[ -n "$CUR_VERSION" ]] || die "无法从 Makefile.vendor.mk 解析当前 VERSION，请用 MESH_VERSION=2.X.0 显式指定后重跑"
  MESH_VERSION="$(cut -d. -f1 <<<"$CUR_VERSION").$(( $(cut -d. -f2 <<<"$CUR_VERSION") + 1 )).0"
fi
[[ "$MESH_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "MESH_VERSION 格式不合法: $MESH_VERSION"
NEW_CHANNELS="stable,stable-$(cut -d. -f1-2 <<<"$MESH_VERSION")"

# 校验 alauda 构建产物（charts）可达性；404 说明 alauda-mesh/istio 还没构建完，不要继续
CHART_BASE="${CHART_BASE_URL:-$(yq -r '[.versions[] | select(.eol != true) | .charts[]?] | .[0] // ""' <<<"$VF_CONTENT" | sed -E 's#/[^/]+/helm/[^/]+$##')}"
[[ -n "$CHART_BASE" ]] || die "无法从 alauda-versions.yaml 推导 charts 基地址，请设置 CHART_BASE_URL"
if [[ "${SKIP_CHART_CHECK:-0}" != "1" ]]; then
  info "校验构建产物可达性（$CHART_BASE）..."
  MISSING=()
  for b in "${BUILDS[@]}"; do
    for c in base istiod gateway cni ztunnel; do
      url="$CHART_BASE/$b/helm/$c-$b.tgz"
      curl -sfI --max-time 30 "$url" >/dev/null || MISSING+=("$url")
    done
  done
  if [[ ${#MISSING[@]} -gt 0 ]]; then
    printf '%s\n' "缺失的 chart 产物:" "${MISSING[@]}" >&2
    die "${#MISSING[@]} 个 chart 不可达：先推动 alauda-mesh/istio 完成对应构建再同步"
  fi
fi

SYNC_BRANCH="sync/$(date +%Y-%m-%d)"
git rev-parse -q --verify "refs/heads/$SYNC_BRANCH" >/dev/null \
  && die "分支 $SYNC_BRANCH 已存在。续跑场景无需重跑本脚本（state.env 已有记录，直接执行后续步骤）；重新开始则先删除该分支"

BASE_SHA=$(git rev-parse "origin/$TARGET_BRANCH")
MERGE_BASE=$(git merge-base "$BASE_SHA" "upstream/$UPSTREAM_BRANCH")
# 记录本次合并的上游快照 SHA：release 分支会持续前进（Automator 等），后续 verify 的
# 全量差异审计必须以该快照为基准，避免 fetch 后 ref 漂移造成假差异
UPSTREAM_SHA=$(git rev-parse "upstream/$UPSTREAM_BRANCH")
git checkout -b "$SYNC_BRANCH" "$BASE_SHA"

save_state UPSTREAM_SHA "$UPSTREAM_SHA"
save_state UPSTREAM_BRANCH "$UPSTREAM_BRANCH"
save_state TARGET_BRANCH "$TARGET_BRANCH"
save_state SYNC_BRANCH "$SYNC_BRANCH"
save_state BASE_SHA "$BASE_SHA"
save_state MERGE_BASE "$MERGE_BASE"
save_state MESH_VERSION "$MESH_VERSION"
save_state NEW_CHANNELS "$NEW_CHANNELS"
save_state NEW_MAJOR "$NEW_MAJOR"
save_state PREV_MAJOR "$PREV_MAJOR"
save_state NEW_BUILDS "${NEW_BUILDS[*]}"
save_state PREV_BUILDS "${PREV_BUILDS[*]:-}"
save_state CHART_BASE "$CHART_BASE"

info "同步分支: $SYNC_BRANCH（基于 origin/$TARGET_BRANCH @ ${BASE_SHA:0:8}）"
info "mesh 版本: $MESH_VERSION  channels: $NEW_CHANNELS"
info "istio 矩阵: 新增 $NEW_MAJOR（${NEW_BUILDS[*]}）+ 保留 ${PREV_MAJOR:-无}${PREV_BUILDS[*]:+（更新为 ${PREV_BUILDS[*]}）}"

if [[ $(git rev-list --count "HEAD..upstream/$UPSTREAM_BRANCH") -eq 0 ]]; then
  echo "RESULT: UP_TO_DATE 目标分支已包含 upstream/$UPSTREAM_BRANCH，无需 merge（续跑场景可直接执行后续步骤）"
  exit 0
fi

if git merge --no-edit "upstream/$UPSTREAM_BRANCH"; then
  echo "RESULT: MERGED 无冲突（罕见），直接进入步骤 3（update-versions.sh）"
  exit 0
fi
git rev-parse -q --verify MERGE_HEAD >/dev/null || die "merge 异常终止（不在冲突状态），请检查上面的 git 输出"
N=$(git diff --name-only --diff-filter=U | wc -l)
echo "RESULT: CONFLICT 共 $N 个冲突文件（大版本同步 1500~2500 属正常）"
echo "下一步: bash $SCRIPT_DIR/resolve-merge-conflicts.sh 机械解决 A/B 层"
exit 2
