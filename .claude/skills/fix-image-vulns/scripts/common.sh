#!/usr/bin/env bash
# fix-image-vulns 公共函数与状态管理。
# 状态目录 out/fix-image-vulns/（out/ 在 .gitignore 内）：
#   state.env          键值状态（ROUND、BASE_BRANCH、FIX_BRANCH、WORKTREE、PR_*、RELEASE_* 等）
#   images-roundN.tsv  第 N 轮待扫描镜像（轮次/来源/镜像地址）
#   scans/roundN/      扫描原始 JSON 与分类 TSV
#   worktree/          修复 worktree（基于当前分支，不打扰主工作区检出）
set -euo pipefail

REPO="${FIX_REPO:-alauda-mesh/sail-operator}"
SCAN_API="${SCAN_API:-http://192.168.25.100:8888}"
WORKFLOW_FILE="alauda-release.yaml"
WORKFLOW_NAME="Alauda Release workflow"
# 扫描/修复范围：只处理 servicemesh-operator2 主镜像；-bundle 镜像不会有漏洞，不在范围
IMAGE_REPO_NAME="servicemesh-operator2"

die()  { echo "错误: $*" >&2; exit 1; }
warn() { echo "警告: $*" >&2; }
info() { echo "==> $*" >&2; }

STATE_DIR_REL="out/fix-image-vulns"

repo_root() {
  ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die "当前目录不在 git 仓库内"
  # worktree 中执行时回到主仓库根（状态目录只放主仓库）。
  # --git-common-dir 在主仓库根返回相对的 ".git"，必须转成绝对路径再比较
  local common; common="$(cd "$(git rev-parse --git-common-dir)" && pwd)"
  [[ "$common" != "$ROOT/.git" ]] && ROOT="$(dirname "$common")"
  [[ -f "$ROOT/Dockerfile.alauda" && -f "$ROOT/Makefile.vendor.mk" ]] \
    || die "当前仓库不是 alauda sail-operator 仓库: $ROOT"
  cd "$ROOT"
  STATE_DIR="$ROOT/$STATE_DIR_REL"
  STATE_FILE="$STATE_DIR/state.env"
  mkdir -p "$STATE_DIR"
}

load_state() {
  [[ -f "$STATE_FILE" ]] || die "状态文件不存在: $STATE_FILE（先执行 resolve-input.sh）"
  # shellcheck disable=SC1090
  source "$STATE_FILE"
}

set_state() { # set_state KEY VALUE
  touch "$STATE_FILE"
  grep -v "^${1}=" "$STATE_FILE" >"$STATE_FILE.tmp" || true
  echo "${1}=\"${2}\"" >>"$STATE_FILE.tmp"
  mv "$STATE_FILE.tmp" "$STATE_FILE"
}

require_gh() {
  command -v gh >/dev/null 2>&1 || die "找不到 gh CLI"
  gh auth status >/dev/null 2>&1 || die "gh 未认证。请提示用户在会话中执行: ! gh auth login"
}

# push/建 PR/触发流水线前的安全守卫：测试克隆环境的 origin 不是 alauda 仓库时拒绝外发
origin_is_alauda() {
  local url; url="$(git remote get-url origin 2>/dev/null || true)"
  [[ "$url" =~ github\.com[:/]${REPO}(\.git)?$ ]]
}

# 镜像地址 → 安全文件名
img_slug() { tr '/:' '__' <<<"$1"; }

# 镜像地址 → 短仓库名（build-harbor.alauda.cn/asm/servicemesh-operator2:tag → servicemesh-operator2）
img_repo() { local p="${1%%:*}"; echo "${p##*/}"; }

# 是否在扫描范围（只扫主镜像，-bundle 排除）
in_scan_scope() { [[ "$(img_repo "$1")" == "$IMAGE_REPO_NAME" ]]; }

# 修复 worktree 路径（load_state 之后调用）
resolve_worktree() {
  [[ -n "${WORKTREE:-}" && -d "$WORKTREE" ]] \
    || die "修复 worktree 不存在（先执行 create-fix-branch.sh）"
  echo "$WORKTREE"
}

# 读取指定目录下 alauda-release.yaml 顶层 env 的 GOTOOLCHAIN pin（无则输出空）
read_gotoolchain_pin() { # read_gotoolchain_pin <仓库或worktree目录>
  sed -n 's/^[[:space:]]*GOTOOLCHAIN:[[:space:]]*//p' "$1/.github/workflows/$WORKFLOW_FILE" 2>/dev/null | head -1
}

# 提取某个 run 构建产出的扫描范围内镜像（来自 "Output image: " step 名，bundle 已排除）
run_output_images() { # run_output_images <RUN_ID>
  local imgs
  imgs="$(gh run view "$1" --repo "$REPO" --json jobs \
    --jq '.jobs[].steps[].name | select(startswith("Output image: ")) | sub("^Output image: "; "")' 2>/dev/null)" || return 1
  local img
  while IFS= read -r img; do
    [[ -n "$img" ]] && in_scan_scope "$img" && echo "$img"
  done <<<"$imgs" || true
}
