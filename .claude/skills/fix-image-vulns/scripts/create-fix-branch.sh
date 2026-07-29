#!/usr/bin/env bash
# 步骤 2a：基于当前分支（BASE_BRANCH）创建修复分支（git worktree，不打扰主工作区当前检出）。
# 用法: create-fix-branch.sh
# 分支命名: fix/cve-<UTC日期>（如 fix/cve-20260729）
# worktree 放在 out/fix-image-vulns/worktree/（gitignore 内）。
# 幂等：worktree 已在对应修复分支上时直接复用（回归轮追加 commit 用）。
# 输出: WORKTREE= / BRANCH= / BASE=

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

main() {
  repo_root
  load_state
  [[ -n "${BASE_BRANCH:-}" ]] || die "状态中没有 BASE_BRANCH（先执行 resolve-input.sh）"
  git rev-parse --verify --quiet "refs/heads/$BASE_BRANCH" >/dev/null \
    || die "本地不存在分支 $BASE_BRANCH"

  local FIX_BRANCH="fix/cve-$(date -u +%Y%m%d)"
  local WT="$STATE_DIR/worktree"

  # 基分支落后 origin 时提醒（修复仍基于本地 HEAD，是否先同步由模型与用户判断）
  if git fetch -q origin "refs/heads/$BASE_BRANCH:refs/remotes/origin/$BASE_BRANCH" 2>/dev/null; then
    local behind
    behind="$(git rev-list --count "refs/heads/$BASE_BRANCH..refs/remotes/origin/$BASE_BRANCH" 2>/dev/null || echo 0)"
    [[ "$behind" -gt 0 ]] && warn "本地 $BASE_BRANCH 落后 origin/$BASE_BRANCH $behind 个提交，修复仍将基于本地 HEAD"
  else
    warn "origin 上没有分支 $BASE_BRANCH（本地分支未推送过？后续 create-pr.sh 将无法以它为 base 建 PR）"
  fi

  if [[ -e "$WT" ]]; then
    local cur
    cur="$(git -C "$WT" branch --show-current 2>/dev/null || true)"
    if [[ "$cur" == "$FIX_BRANCH" ]]; then
      info "worktree 已在分支 $FIX_BRANCH 上，直接复用"
    elif [[ -z "$(git -C "$WT" status --porcelain 2>/dev/null)" ]]; then
      info "移除残留的干净 worktree（原分支 ${cur:-未知}）"
      git worktree remove --force "$WT"
    else
      die "$WT 已存在且有未提交改动（分支 ${cur:-未知}），请人工确认后再处理"
    fi
  fi

  if [[ ! -e "$WT" ]]; then
    if git rev-parse --verify --quiet "refs/heads/$FIX_BRANCH" >/dev/null; then
      # 同日重跑：本地已有该修复分支（如上次中断），挂载 worktree 继续用
      warn "本地已存在分支 $FIX_BRANCH，直接挂载（其提交历史保留）"
      git worktree add -q "$WT" "$FIX_BRANCH"
    else
      git worktree add -q "$WT" -b "$FIX_BRANCH" "refs/heads/$BASE_BRANCH"
    fi
  fi

  set_state FIX_BRANCH "$FIX_BRANCH"
  set_state WORKTREE "$WT"

  echo
  echo "BRANCH_READY"
  echo "WORKTREE=$WT"
  echo "BRANCH=$FIX_BRANCH"
  echo "BASE=$BASE_BRANCH（$(git rev-parse --short "refs/heads/$BASE_BRANCH")）"
  echo "下一步: 按扫描的修复目标执行 update-gotoolchain.sh / gomod-bump.sh，再 verify-build.sh"
}

main "$@"
