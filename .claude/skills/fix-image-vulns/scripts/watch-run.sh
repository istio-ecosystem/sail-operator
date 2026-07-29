#!/usr/bin/env bash
# 步骤 5：监控流水线 run，成功后收集新镜像供回归扫描。
# 用法: watch-run.sh [RUN_ID]   缺省用状态里的 RELEASE_RUN_ID
# 成功: ROUND+1，生成 images-round<N+1>.tsv（新镜像，-bundle 已排除）。
# 退出码: 0=成功  1=前置失败  2=run 失败（附失败日志摘要）  3=等待超时
# 环境变量: FIX_WATCH_INTERVAL=60  FIX_WATCH_TIMEOUT=5400
# 注意: 多平台镜像构建通常几十分钟，必须用后台方式运行（Bash 的 run_in_background: true）。

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

INTERVAL="${FIX_WATCH_INTERVAL:-60}"
TIMEOUT="${FIX_WATCH_TIMEOUT:-5400}"

main() {
  repo_root
  load_state
  require_gh

  local rid="${1:-${RELEASE_RUN_ID:-}}"
  [[ -n "$rid" ]] || die "没有 run ID（先执行 run-release.sh，或显式传入 RUN_ID）"

  # ---------- 轮询直至完成 ----------
  local start st cc
  start="$(date +%s)"
  while :; do
    read -r st cc < <(gh run view "$rid" --repo "$REPO" --json status,conclusion \
      --jq '"\(.status) \(.conclusion // "-")"' 2>/dev/null || echo "unknown -")
    [[ "$st" == "completed" ]] && break
    if (( $(date +%s) - start > TIMEOUT )); then
      echo "RESULT: PIPELINE_TIMEOUT 等待超过 ${TIMEOUT}s 仍未完成（当前 status=$st）"
      echo "  https://github.com/$REPO/actions/runs/$rid"
      echo "  稍后重跑本脚本继续等待"
      exit 3
    fi
    echo "[$(date -u +%H:%M:%S)] run $rid status=$st，${INTERVAL}s 后再查"
    sleep "$INTERVAL"
  done
  echo "[$(date -u +%H:%M:%S)] run $rid completed: $cc"

  # ---------- 收集结果 ----------
  if [[ "$cc" != "success" ]]; then
    echo
    echo "FAIL: run $rid conclusion=$cc"
    echo "  step 概览:"
    gh run view "$rid" --repo "$REPO" 2>/dev/null | grep -E '^(X|✓|-|\*)' | head -15 | sed 's/^/    /' || true
    echo "  失败日志摘要（完整: gh run view $rid --repo $REPO --log-failed）:"
    gh run view "$rid" --repo "$REPO" --log-failed 2>/dev/null | tail -80 | sed 's/^/    /' \
      || echo "    （拉取失败日志出错，请人工查看）"
    echo
    echo "RESULT: PIPELINE_FAILED（分析失败原因，修好后在原 worktree 追加 commit → create-pr.sh → run-release.sh → 重跑本脚本）"
    exit 2
  fi

  local imgs
  imgs="$(run_output_images "$rid")"
  [[ -n "$imgs" ]] || die "run $rid 成功但未提取到扫描范围内的镜像（$IMAGE_REPO_NAME），请人工检查 run 输出"

  local NEXT=$((ROUND + 1))
  local IMG_TSV="$STATE_DIR/images-round${NEXT}.tsv"
  : >"$IMG_TSV"   # 重跑时整体重建，保持幂等
  while IFS= read -r img; do
    printf '%s\trun:%s\t%s\n' "$NEXT" "$rid" "$img" >>"$IMG_TSV"
  done <<<"$imgs"
  set_state ROUND "$NEXT"

  echo
  echo "RESULT: SUCCESS"
  echo "新镜像:"
  sed 's/^/  /' <<<"$imgs"
  echo "下一步: 执行 scan-image.sh 做第 ${NEXT} 轮回归扫描（Bash timeout 600000）"
}

main "$@"
