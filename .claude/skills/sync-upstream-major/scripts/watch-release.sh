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
# 步骤 8b：监控 run-release.sh 触发的 Alauda Release 流水线 run。
# 会阻塞较久（多平台镜像构建 20~60 分钟），必须以后台方式运行（Bash 工具 run_in_background: true）。
# 环境变量: WATCH_INTERVAL 轮询间隔秒数（默认 30）  WATCH_TIMEOUT 等待上限秒数（默认 4800）
# 退出码: 0=PIPELINE_SUCCESS  2=PIPELINE_FAILED（附失败摘要与已知问题识别）  3=PIPELINE_TIMEOUT  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
repo_root
load_state
repo_slug

INTERVAL="${WATCH_INTERVAL:-30}"
TIMEOUT="${WATCH_TIMEOUT:-4800}"

command -v gh >/dev/null 2>&1 || die "未安装 gh"
[[ -n "${RUN_ID:-}" ]] || die "状态中无 RUN_ID，请先执行 run-release.sh"
RUN_URL="${RUN_URL:-https://github.com/$REPO_SLUG/actions/runs/$RUN_ID}"

echo "监控 run $RUN_ID（$RUN_URL），超时 ${TIMEOUT}s ..."
START=$(date +%s)
LAST_STATUS=""
STATUS="" CONCLUSION="-"
while :; do
  # gh 瞬时失败（网络抖动）只重试，不让 set -e 杀掉监控
  read -r STATUS CONCLUSION < <(gh run view --repo "$REPO_SLUG" "$RUN_ID" --json status,conclusion \
    --jq '"\(.status) \(.conclusion // "-")"' 2>/dev/null || echo "unknown -")
  if [[ "$STATUS" != "$LAST_STATUS" ]]; then
    echo "[$(date +%H:%M:%S)] status=$STATUS"
    LAST_STATUS="$STATUS"
  fi
  [[ "$STATUS" == "completed" ]] && break
  if (( $(date +%s) - START > TIMEOUT )); then
    echo "RESULT: PIPELINE_TIMEOUT 等待超过 ${TIMEOUT}s 仍未完成，稍后自行查看: $RUN_URL"
    exit 3
  fi
  sleep "$INTERVAL"
done

if [[ "$CONCLUSION" == "success" ]]; then
  # 提取产物镜像（workflow 的 Output image 步骤输出 BUILD_IMAGE=...）；
  # 排除以 $ 或 " 开头的值，跳过日志里未展开的命令回显 echo "BUILD_IMAGE=${IMAGE}"
  IMAGES="$(gh run view --repo "$REPO_SLUG" "$RUN_ID" --log 2>/dev/null \
    | grep -oE 'BUILD_IMAGE=[^"$[:space:]]\S*' | cut -d= -f2- | sort -u || true)"
  echo "RESULT: PIPELINE_SUCCESS $RUN_URL"
  [[ -n "$IMAGES" ]] && { echo "构建镜像:"; while IFS= read -r l; do echo "  $l"; done <<<"$IMAGES"; }
  exit 0
fi

echo "RESULT: PIPELINE_FAILED conclusion=$CONCLUSION $RUN_URL"
echo "==== 失败 job/step 概览 ===="
gh run view --repo "$REPO_SLUG" "$RUN_ID" 2>/dev/null | grep -E "^(X|✓|-|\*)" | head -20 || true

LOG_FILE="$ROOT/$STATE_DIR_REL/release-failed.log"
gh run view --repo "$REPO_SLUG" "$RUN_ID" --log-failed >"$LOG_FILE" 2>/dev/null \
  || echo "（拉取失败日志出错，请手动查看 $RUN_URL）"

# 已知问题识别：istio-testing/build-tools 镜像下载失败 —— 按约定交给用户处理，不要自行修复
BT_LINES="$(grep -i 'build-tools' "$LOG_FILE" 2>/dev/null \
  | grep -iE 'error|fail|pull|denied|unable|timeout|tls|refused|reset|no such host|manifest|not found|eof' || true)"
if [[ -n "$BT_LINES" ]]; then
  echo
  echo "KNOWN_ISSUE: BUILD_TOOLS_IMAGE_PULL"
  echo "疑似 istio-testing/build-tools 镜像下载问题，相关日志行:"
  head -5 <<<"$BT_LINES" | sed 's/^/  /'
  echo "按约定：此问题不要自行修复，直接报告用户处理。"
fi

# 已知问题识别：create-gh-release 的 --target release-2.X 分支不存在（HTTP 422）。
# 2026-07 起 alauda-release.yaml 已内置分支预检：分支缺失时跳过 release 创建并写 step summary，
# 验证 run 应正常 PIPELINE_SUCCESS——仍见此错误说明跑的是不含该预检的旧版 workflow（旧分支）
if grep -q 'Invalid target_commitish' "$LOG_FILE" 2>/dev/null; then
  echo
  echo "KNOWN_ISSUE: GH_RELEASE_TARGET_BRANCH_MISSING"
  echo "create-gh-release 的 --target release-2.X 分支尚不存在（PR 合并前的验证 run 属预期）。"
  echo "注意：当前 workflow 已内置分支缺失跳过（写 summary），正常不该再触发本错误；"
  echo "若触发，说明该 run 跑在不含预检的旧版 alauda-release.yaml 上（如旧分支）。"
  echo "核对镜像 step 是否全绿: gh run view --repo <repo> <run_id> --json jobs -q '.jobs[].steps[] | select(.conclusion != \"skipped\") | .name + \" => \" + .conclusion'"
  echo "全绿则同步验证通过，镜像名在 'Output image:' step 名里；GitHub release 待 PR 合并、release-2.X 分支创建后由正式发版补齐。"
fi

echo
echo "==== 失败日志末尾（完整日志: $LOG_FILE）===="
tail -80 "$LOG_FILE" 2>/dev/null || true
exit 2
