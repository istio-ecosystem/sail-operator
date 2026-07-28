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
# 步骤 3：机械更新版本信息（不 commit，便于 review）。
#   1) Makefile.vendor.mk: VERSION / CHANNELS
#   2) .github/workflows/alauda-release.yaml: bundle_channels 默认值；release-1.30 特例改 TOOLS_REGISTRY_PROVIDER
#   3) pkg/istioversion/alauda-versions.yaml: 按构建版本重建版本矩阵（新大版本 + 上一大版本 + EOL 收尾）
#   4) NOTICE: vendor_defaults.yaml 需人工处理的块、go.mod istio 依赖比对
# 退出码: 0=OK  2=PATTERN_MISMATCH（按 FAIL 清单用 Edit 手动完成，其余不要重复改）  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
repo_root
load_state
ensure_yq

FAILS=0
ok()     { echo "OK: $*"; }
fail()   { echo "FAIL: $*"; FAILS=$((FAILS + 1)); }
notice() { echo "NOTICE: $*"; }

read -ra NEW_BUILDS_ARR <<<"${NEW_BUILDS:-}"
read -ra PREV_BUILDS_ARR <<<"${PREV_BUILDS:-}"
[[ ${#NEW_BUILDS_ARR[@]} -ge 1 ]] || die "状态中没有新版本构建（NEW_BUILDS），请重新执行 merge-upstream.sh"

# ---------- 1) Makefile.vendor.mk ----------
VM=Makefile.vendor.mk
if grep -qE '^VERSION = [0-9]+\.[0-9]+\.[0-9]+$' "$VM"; then
  sed -i -E "s/^VERSION = [0-9.]+$/VERSION = $MESH_VERSION/" "$VM"
  ok "$VM: VERSION = $MESH_VERSION"
else
  fail "$VM: 未匹配到 'VERSION = x.y.z' 行，手动改为 VERSION = $MESH_VERSION"
fi
if grep -qE '^CHANNELS = "stable,stable-[0-9.]+"$' "$VM"; then
  sed -i -E "s/^CHANNELS = \"stable,stable-[0-9.]+\"$/CHANNELS = \"$NEW_CHANNELS\"/" "$VM"
  ok "$VM: CHANNELS = \"$NEW_CHANNELS\""
else
  fail "$VM: 未匹配到 CHANNELS 行，手动改为 CHANNELS = \"$NEW_CHANNELS\""
fi

# ---------- 2) .github/workflows/alauda-release.yaml ----------
WF=.github/workflows/alauda-release.yaml
if grep -qE '^[[:space:]]*default: "stable,stable-[0-9.]+"$' "$WF"; then
  sed -i -E "s/^([[:space:]]*)default: \"stable,stable-[0-9.]+\"$/\1default: \"$NEW_CHANNELS\"/" "$WF"
  ok "$WF: bundle_channels 默认值 = \"$NEW_CHANNELS\""
else
  fail "$WF: 未匹配到 bundle_channels 的 default 行，手动改为 default: \"$NEW_CHANNELS\""
fi
if [[ "$UPSTREAM_BRANCH" == "release-1.30" ]]; then
  # release-1.30 特例：上游 build-tools 镜像迁至 registry.istio.io，后续大版本同步不涉及
  if grep -qE '^[[:space:]]*TOOLS_REGISTRY_PROVIDER: registry\.istio\.io$' "$WF"; then
    ok "$WF: TOOLS_REGISTRY_PROVIDER 已是 registry.istio.io"
  elif grep -qE '^[[:space:]]*TOOLS_REGISTRY_PROVIDER: ' "$WF"; then
    sed -i -E "s#^([[:space:]]*)TOOLS_REGISTRY_PROVIDER: .*#\1TOOLS_REGISTRY_PROVIDER: registry.istio.io#" "$WF"
    ok "$WF: TOOLS_REGISTRY_PROVIDER = registry.istio.io（release-1.30 特例）"
  else
    fail "$WF: 未匹配到 TOOLS_REGISTRY_PROVIDER 行，手动改为 registry.istio.io"
  fi
fi

# ---------- 3) pkg/istioversion/alauda-versions.yaml ----------
VF=pkg/istioversion/alauda-versions.yaml
CHART_BASE="${CHART_BASE:-$(yq -r '[.versions[] | select(.eol != true) | .charts[]?] | .[0] // ""' "$VF" | sed -E 's#/[^/]+/helm/[^/]+$##')}"

# 解析现有条目到并行数组
N=$(yq '.versions | length' "$VF")
E_NAME=() E_EOL=() E_REF=() E_VER=() E_REPO=() E_COMMIT=() E_CHARTS=() E_MAJOR=()
for ((i = 0; i < N; i++)); do
  E_NAME[i]=$(yq -r ".versions[$i].name" "$VF")
  E_EOL[i]=$(yq -r ".versions[$i].eol // false" "$VF")
  E_REF[i]=$(yq -r ".versions[$i].ref // \"\"" "$VF")
  E_VER[i]=$(yq -r ".versions[$i].version // \"\"" "$VF")
  E_REPO[i]=$(yq -r ".versions[$i].repo // \"\"" "$VF")
  E_COMMIT[i]=$(yq -r ".versions[$i].commit // \"\"" "$VF")
  E_CHARTS[i]=$(yq -r ".versions[$i] | (.charts // [])[]" "$VF")
  E_MAJOR[i]=$(sed -E 's/^v//; s/-latest$//' <<<"${E_NAME[i]}" | cut -d. -f1-2)
done

# 现有大版本的先后顺序（文件从新到旧）及分类
ORDERED_MAJORS=()
for ((i = 0; i < N; i++)); do
  m="${E_MAJOR[i]}"
  found=0
  for x in "${ORDERED_MAJORS[@]:-}"; do [[ "$x" == "$m" ]] && found=1; done
  [[ $found -eq 0 ]] && ORDERED_MAJORS+=("$m")
done
major_is_eol() { # 该大版本的全部现有条目均 eol 才算 eol
  local m=$1 i
  for ((i = 0; i < N; i++)); do
    [[ "${E_MAJOR[i]}" == "$m" && "${E_EOL[i]}" != "true" ]] && return 1
  done
  return 0
}

emit_full_entry() { # $1=istio版本 $2=构建版本
  cat <<EOF
  - name: v$1
    version: $1
    repo: https://github.com/istio/istio
    commit: $1
    charts:
      - $CHART_BASE/$2/helm/base-$2.tgz
      - $CHART_BASE/$2/helm/istiod-$2.tgz
      - $CHART_BASE/$2/helm/gateway-$2.tgz
      - $CHART_BASE/$2/helm/cni-$2.tgz
      - $CHART_BASE/$2/helm/ztunnel-$2.tgz
EOF
}

emit_major_from_builds() { # $1=大版本 $2...=构建版本（本函数内部按版本降序排列）
  local major=$1 b latest
  shift
  local sorted
  sorted=$(printf '%s\n' "$@" | sort -rV)
  latest=$(build_to_istio "$(head -1 <<<"$sorted")")
  echo "  # v$major"
  echo "  - name: v$major-latest"
  echo "    ref: v$latest"
  while IFS= read -r b; do
    emit_full_entry "$(build_to_istio "$b")" "$b"
  done <<<"$sorted"
}

emit_preserved_entry() { # $1=现有条目下标，字段原样保留
  local i=$1 u
  echo "  - name: ${E_NAME[i]}"
  if [[ -n "${E_REF[i]}" ]]; then echo "    ref: ${E_REF[i]}"; fi
  if [[ -n "${E_VER[i]}" ]]; then
    echo "    version: ${E_VER[i]}"
    echo "    repo: ${E_REPO[i]}"
    echo "    commit: ${E_COMMIT[i]}"
  fi
  if [[ -n "${E_CHARTS[i]}" ]]; then
    echo "    charts:"
    while IFS= read -r u; do [[ -n "$u" ]] && echo "      - $u"; done <<<"${E_CHARTS[i]}"
  fi
  if [[ "${E_EOL[i]}" == "true" ]]; then echo "    eol: true"; fi
}

emit_major_eolified() { # 新淘汰的大版本：-latest 留 name/ref/eol，具体版本只留 name/eol
  local m=$1 i
  echo "  # v$m"
  for ((i = 0; i < N; i++)); do
    [[ "${E_MAJOR[i]}" == "$m" ]] || continue
    echo "  - name: ${E_NAME[i]}"
    if [[ "${E_NAME[i]}" == *-latest && -n "${E_REF[i]}" ]]; then
      echo "    ref: ${E_REF[i]}"
    fi
    echo "    eol: true"
  done
}

if [[ -z "$CHART_BASE" ]]; then
  fail "$VF: 无法推导 charts 基地址（CHART_BASE_URL 可覆盖），版本矩阵未修改"
else
  NEWLY_EOL=()
  TMP=$(mktemp)
  {
    awk '/^versions:/{exit} {print}' "$VF"
    echo "versions:"
    # 新大版本（幂等：现有同大版本条目被本段替换）
    emit_major_from_builds "$NEW_MAJOR" "${NEW_BUILDS_ARR[@]}"
    # 上一大版本：给了构建列表则重建，否则原样保留
    if [[ -n "${PREV_MAJOR:-}" ]]; then
      if [[ ${#PREV_BUILDS_ARR[@]} -gt 0 ]]; then
        emit_major_from_builds "$PREV_MAJOR" "${PREV_BUILDS_ARR[@]}"
      else
        echo "  # v$PREV_MAJOR"
        for ((i = 0; i < N; i++)); do
          [[ "${E_MAJOR[i]}" == "$PREV_MAJOR" ]] && emit_preserved_entry "$i"
        done
      fi
    fi
    # 其余大版本：原本非 EOL 的按 EOL 收尾；原本已 EOL 的原样保留
    for m in "${ORDERED_MAJORS[@]}"; do
      [[ "$m" == "$NEW_MAJOR" || "$m" == "${PREV_MAJOR:-}" ]] && continue
      if major_is_eol "$m"; then
        echo "  # v$m"
        for ((i = 0; i < N; i++)); do
          [[ "${E_MAJOR[i]}" == "$m" ]] && emit_preserved_entry "$i"
        done
      else
        NEWLY_EOL+=("$m")
        emit_major_eolified "$m"
      fi
    done
  } >"$TMP"

  # 产物自检：能被 yq 解析且首条目是新大版本的 -latest
  if ! yq '.versions | length' "$TMP" >/dev/null 2>&1; then
    rm -f "$TMP"
    fail "$VF: 生成结果无法被 yq 解析，版本矩阵未修改（请人工重建）"
  elif [[ "$(yq -r '.versions[0].name' "$TMP")" != "v$NEW_MAJOR-latest" ]]; then
    rm -f "$TMP"
    fail "$VF: 生成结果首条目不是 v$NEW_MAJOR-latest，版本矩阵未修改（请人工重建）"
  else
    mv "$TMP" "$VF"
    ok "$VF: 矩阵已重建 —— 保留 $NEW_MAJOR（${NEW_BUILDS_ARR[*]}）+ ${PREV_MAJOR:-无}${PREV_BUILDS:+（$PREV_BUILDS）}；新增 EOL: ${NEWLY_EOL[*]:-无}"
    save_state NEWLY_EOL_MAJORS "${NEWLY_EOL[*]:-}"
    echo "---- 新版本矩阵（非 EOL）----"
    yq -r '.versions[] | select(.eol != true) | .name' "$VF" | sed 's/^/  /'
  fi
fi

# ---------- 4) 人工项提示 ----------
VD=pkg/istiovalues/vendor_defaults.yaml
NONEOL_VERSIONS=$(yq -r '.versions[] | select(.eol != true) | .version // ""' "$VF" | grep -v '^$' || true)
VD_KEYS=$(yq -r 'keys | .[]' "$VD" 2>/dev/null || true)
for v in $NONEOL_VERSIONS; do
  grep -qx "v$v" <<<"$VD_KEYS" || notice "$VD 缺少 v$v 块：复制上一版本的块为基础按需调整（人工，multus/资源限额等逐项判断）"
done
for k in $VD_KEYS; do
  grep -qx "${k#v}" <<<"$NONEOL_VERSIONS" || notice "$VD 的 $k 块已不在非 EOL 矩阵中，确认后可删（人工）"
done

if diff <(git show "upstream/$UPSTREAM_BRANCH:go.mod" | grep -E '^	istio\.io/' | sort) \
        <(grep -E '^	istio\.io/' go.mod | sort) >/dev/null 2>&1; then
  ok "go.mod: istio.io 依赖与 upstream/$UPSTREAM_BRANCH 一致"
else
  notice "go.mod 的 istio.io 依赖与上游分支不一致（步骤 2 解冲突可能没取上游侧；verify.sh 会再查）"
fi

echo
if [[ $FAILS -gt 0 ]]; then
  echo "RESULT: PATTERN_MISMATCH 共 $FAILS 项 FAIL，按上面的期望值用 Edit 手动完成；其余已 OK 的项不要重复改"
  exit 2
fi
echo "RESULT: OK 机械修改完成；处理完 NOTICE 的人工项后执行 regen.sh"
