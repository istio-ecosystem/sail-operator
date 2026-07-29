#!/usr/bin/env bash
# 步骤 3：push 修复分支并创建 PR（base = BASE_BRANCH）。
# 用法: create-pr.sh <PR正文文件>
# 幂等：修复分支已有 open PR 时直接复用（回归轮 push 新 commit 即可）。
# 输出: PR_NUMBER= / PR_URL=（同时写入状态）。

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

main() {
  repo_root
  load_state
  require_gh

  local BODY_FILE="${1:-}"
  [[ -n "$BODY_FILE" && -f "$BODY_FILE" ]] || die "PR 正文文件不存在: $BODY_FILE（先写好正文再执行）"
  origin_is_alauda || die "origin 不是 $REPO，拒绝 push/建 PR（测试环境守卫）"
  [[ -n "${FIX_BRANCH:-}" ]] || die "状态中没有 FIX_BRANCH（先执行 create-fix-branch.sh）"
  local WT; WT="$(resolve_worktree)"

  cd "$WT"
  [[ "$(git branch --show-current)" == "$FIX_BRANCH" ]] || die "worktree 当前分支不是 $FIX_BRANCH"
  [[ -z "$(git status --porcelain)" ]] || die "worktree 有未提交改动，请先 commit -s（禁止 amend，一律新建 commit）"
  local n
  n="$(git rev-list --count "refs/heads/$BASE_BRANCH..HEAD")"
  [[ "$n" -ge 1 ]] || die "相对 $BASE_BRANCH 没有新 commit，无内容可提 PR"
  git ls-remote --exit-code --heads origin "$BASE_BRANCH" >/dev/null 2>&1 \
    || die "origin 上没有分支 $BASE_BRANCH，无法以它为 base 建 PR（先推送基分支或换基分支重跑 resolve-input.sh）"

  info "push $FIX_BRANCH 到 origin ..."
  git push -u origin "$FIX_BRANCH" || die "push 失败"

  local PR_INFO
  PR_INFO="$(gh pr list --repo "$REPO" --head "$FIX_BRANCH" --state open \
    --json number,url --jq '.[0] | "\(.number) \(.url)"' 2>/dev/null || true)"
  if [[ -n "$PR_INFO" && "$PR_INFO" != "null null" ]]; then
    info "分支 $FIX_BRANCH 已有 open PR，直接复用"
  else
    # 标题日期取自修复分支名（fix/cve-YYYYMMDD）
    local d="${FIX_BRANCH##*/cve-}" TITLE
    TITLE="fix: servicemesh-operator2 image CVE on ${d:0:4}-${d:4:2}-${d:6:2}"
    gh pr create --repo "$REPO" --base "$BASE_BRANCH" --head "$FIX_BRANCH" \
      --title "$TITLE" --body-file "$BODY_FILE" >/dev/null || die "创建 PR 失败"
    PR_INFO="$(gh pr list --repo "$REPO" --head "$FIX_BRANCH" --state open \
      --json number,url --jq '.[0] | "\(.number) \(.url)"')"
  fi

  local PR_NUMBER="${PR_INFO%% *}" PR_URL="${PR_INFO##* }"
  set_state PR_NUMBER "$PR_NUMBER"
  set_state PR_URL "$PR_URL"

  echo
  echo "PR_NUMBER=$PR_NUMBER"
  echo "PR_URL=$PR_URL"
  echo "下一步: run-release.sh 触发 $WORKFLOW_NAME 流水线构建新镜像"
}

main "$@"
