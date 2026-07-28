#!/usr/bin/env bash
# 步骤 7：push 同步分支并创建 PR（幂等：分支已有 open PR 时直接复用）。
# 用法: create-pr.sh <PR正文文件>
# 退出码: 0=OK（输出 PR_NUMBER= / PR_URL=）  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"
repo_root
load_state
repo_slug

BODY_FILE="${1:-}"
[[ -n "$BODY_FILE" && -f "$BODY_FILE" ]] || die "用法: create-pr.sh <PR正文文件>（文件必须存在）"

command -v gh >/dev/null 2>&1 || die "未安装 gh"
gh auth status >/dev/null 2>&1 || die "gh 未认证，请提示用户执行: ! gh auth login"

CUR_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
[[ "$CUR_BRANCH" == "$SYNC_BRANCH" ]] || die "当前分支是 $CUR_BRANCH，应在 $SYNC_BRANCH 上执行"
git diff --quiet && git diff --cached --quiet || die "有未提交的修改，请先 commit（-s 签名，禁止 amend）"

info "push $SYNC_BRANCH 到 origin ..."
git push -u origin "$SYNC_BRANCH"

EXISTING="$(gh pr list --repo "$REPO_SLUG" --head "$SYNC_BRANCH" --state open --json number -q '.[0].number' 2>/dev/null || true)"
if [[ -n "$EXISTING" ]]; then
  info "分支已有 open PR #$EXISTING，复用"
  PR_NUMBER="$EXISTING"
else
  gh pr create --repo "$REPO_SLUG" \
    --base "$TARGET_BRANCH" --head "$SYNC_BRANCH" \
    --title "chore: sync upstream $UPSTREAM_BRANCH (mesh $MESH_VERSION)" \
    --body-file "$BODY_FILE" >/dev/null
  PR_NUMBER="$(gh pr list --repo "$REPO_SLUG" --head "$SYNC_BRANCH" --state open --json number -q '.[0].number')"
fi
[[ -n "$PR_NUMBER" ]] || die "PR 创建后未能查询到编号，请用 gh pr list --repo $REPO_SLUG 排查"

PR_URL="$(gh pr view "$PR_NUMBER" --repo "$REPO_SLUG" --json url -q .url)"
save_state PR_NUMBER "$PR_NUMBER"
save_state PR_URL "$PR_URL"

echo "PR_NUMBER=$PR_NUMBER"
echo "PR_URL=$PR_URL"
echo "下一步: run-release.sh 触发 Alauda Release 流水线"
