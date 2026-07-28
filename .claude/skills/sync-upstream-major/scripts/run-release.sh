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
# 步骤 8a：在同步分支上触发 Alauda Release 流水线（workflow_dispatch）。
#   release_version = <mesh版本>-r<时间戳>（日期后缀避免与手动发布的版本号冲突）
#   bundle_channels = 新 channels（与 workflow 里更新后的默认值一致，显式传更稳）
#   is_draft_release / is_pre_release 保持 workflow 默认值
# 退出码: 0=已触发并拿到 RUN_ID  4=触发后未见 run  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
repo_root
load_state
repo_slug

WORKFLOW="alauda-release.yaml"
APPEAR_TIMEOUT="${APPEAR_TIMEOUT:-180}"

command -v gh >/dev/null 2>&1 || die "未安装 gh"
gh auth status >/dev/null 2>&1 || die "gh 未认证，请提示用户执行: ! gh auth login"
git ls-remote --exit-code --heads origin "$SYNC_BRANCH" >/dev/null \
  || die "origin 上没有分支 $SYNC_BRANCH（先执行 create-pr.sh）"

RELEASE_VERSION="${MESH_VERSION}-r$(date +%Y%m%d%H%M%S)"
PREV_RUN="$(gh run list --repo "$REPO_SLUG" --workflow "$WORKFLOW" --branch "$SYNC_BRANCH" --limit 1 \
  --json databaseId --jq '.[0].databaseId // ""' 2>/dev/null || true)"

info "触发 $WORKFLOW: ref=$SYNC_BRANCH release_version=$RELEASE_VERSION bundle_channels=$NEW_CHANNELS（其余输入用默认值）"
gh workflow run "$WORKFLOW" --repo "$REPO_SLUG" --ref "$SYNC_BRANCH" \
  -f release_version="$RELEASE_VERSION" \
  -f bundle_channels="$NEW_CHANNELS"

# 等新 run 注册（dispatch 后通常几秒~几十秒）
RUN_ID=""
START=$(date +%s)
while :; do
  sleep 10
  RUN_ID="$(gh run list --repo "$REPO_SLUG" --workflow "$WORKFLOW" --branch "$SYNC_BRANCH" --limit 1 \
    --json databaseId --jq '.[0].databaseId // ""' 2>/dev/null || true)"
  [[ -n "$RUN_ID" && "$RUN_ID" != "$PREV_RUN" ]] && break
  RUN_ID=""
  if (( $(date +%s) - START > APPEAR_TIMEOUT )); then
    echo "RESULT: RUN_NOT_FOUND 触发后 ${APPEAR_TIMEOUT}s 内未见新 run"
    echo "排查方向: 分支上的 workflow 文件是否有语法错误（gh workflow view $WORKFLOW --repo $REPO_SLUG）、self-hosted runner 是否在线"
    exit 4
  fi
done

RUN_URL="$(gh run view --repo "$REPO_SLUG" "$RUN_ID" --json url --jq .url)"
save_state RELEASE_VERSION "$RELEASE_VERSION"
save_state RUN_ID "$RUN_ID"
save_state RUN_URL "$RUN_URL"

echo "RELEASE_VERSION=$RELEASE_VERSION"
echo "RUN_ID=$RUN_ID"
echo "RUN_URL=$RUN_URL"
echo "下一步: 后台运行 watch-release.sh 监控（多平台镜像构建 20~60 分钟）"
