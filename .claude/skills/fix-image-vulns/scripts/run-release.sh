#!/usr/bin/env bash
# 步骤 4：在修复分支上触发 Alauda Release workflow 流水线（workflow_dispatch）。
# 用法: run-release.sh
# release_version = <Makefile.vendor.mk 的 VERSION>-r<UTC时间戳>（如 2.2.0-r20260729015404），
# 时间戳后缀保证 tag 唯一，不会和手动执行的流水线冲突；其余流水线参数用默认值。
# 触发后按 run-name（"Release <release_version>"）精确定位 run 并写入状态。
# 退出码: 0=已触发并定位到 run  1=失败
# 环境变量: FIX_APPEAR_TIMEOUT=120（等 run 出现的秒数）

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

APPEAR_TIMEOUT="${FIX_APPEAR_TIMEOUT:-120}"

main() {
  repo_root
  load_state
  require_gh
  origin_is_alauda || die "origin 不是 $REPO，拒绝触发流水线（测试环境守卫）"
  [[ -n "${FIX_BRANCH:-}" ]] || die "状态中没有 FIX_BRANCH（先执行 create-fix-branch.sh）"
  local WT; WT="$(resolve_worktree)"

  # 流水线用的是 origin 上的修复分支，本地未 push 的 commit 不会包含在构建里
  local local_sha remote_sha
  local_sha="$(git -C "$WT" rev-parse HEAD)"
  remote_sha="$(git ls-remote origin "refs/heads/$FIX_BRANCH" | cut -f1)"
  [[ -n "$remote_sha" ]] || die "origin 上没有分支 $FIX_BRANCH（先执行 create-pr.sh push）"
  [[ "$local_sha" == "$remote_sha" ]] \
    || die "origin/$FIX_BRANCH（${remote_sha:0:10}）落后本地（${local_sha:0:10}），先执行 create-pr.sh push 最新 commit"

  local BASE_VERSION RELEASE_VERSION
  BASE_VERSION="$(sed -n 's/^VERSION[[:space:]]*=[[:space:]]*//p' "$WT/Makefile.vendor.mk" | head -1)"
  [[ -n "$BASE_VERSION" ]] || die "无法从 $WT/Makefile.vendor.mk 解析 VERSION"
  RELEASE_VERSION="${BASE_VERSION}-r$(date -u +%Y%m%d%H%M%S)"

  info "触发 $WORKFLOW_NAME: branch=$FIX_BRANCH release_version=$RELEASE_VERSION（其余参数默认值）"
  gh workflow run "$WORKFLOW_FILE" --repo "$REPO" --ref "$FIX_BRANCH" \
    -f release_version="$RELEASE_VERSION" || die "触发流水线失败"

  # run-name 为 "Release <release_version>"，时间戳唯一，可精确匹配
  info "等待 run 出现（最长 ${APPEAR_TIMEOUT}s）..."
  local start rid
  start="$(date +%s)"
  rid=""
  while :; do
    rid="$(gh run list --repo "$REPO" --workflow "$WORKFLOW_FILE" --branch "$FIX_BRANCH" --limit 10 \
      --json databaseId,displayTitle \
      --jq "[.[] | select(.displayTitle == \"Release $RELEASE_VERSION\")][0].databaseId // empty" 2>/dev/null || true)"
    [[ -n "$rid" ]] && break
    if (( $(date +%s) - start > APPEAR_TIMEOUT )); then
      die "触发后 ${APPEAR_TIMEOUT}s 内没有出现对应 run（displayTitle=Release $RELEASE_VERSION），请人工检查: gh run list --repo $REPO --workflow $WORKFLOW_FILE"
    fi
    sleep 5
  done

  set_state RELEASE_VERSION "$RELEASE_VERSION"
  set_state RELEASE_RUN_ID "$rid"

  echo
  echo "RUN_TRIGGERED"
  echo "RELEASE_VERSION=$RELEASE_VERSION"
  echo "RUN_ID=$rid"
  echo "RUN_URL=https://github.com/$REPO/actions/runs/$rid"
  echo "下一步: 后台执行 watch-run.sh 监控（Bash 的 run_in_background: true）"
}

main "$@"
