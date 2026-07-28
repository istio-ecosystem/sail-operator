#!/usr/bin/env bash
# 公共函数与常量，供各步骤脚本 source 使用。

set -euo pipefail

die()  { echo "ERROR: $*" >&2; exit 1; }
warn() { echo "WARN: $*"; }
info() { echo "INFO: $*"; }

# 定位仓库根目录并 cd 过去
repo_root() {
  local root
  root=$(git rev-parse --show-toplevel 2>/dev/null) || die "当前目录不在 git 仓库内"
  [[ -f "$root/pkg/istioversion/alauda-versions.yaml" ]] || die "当前仓库不是 alauda-mesh/sail-operator（缺少 pkg/istioversion/alauda-versions.yaml）"
  cd "$root"
  ROOT="$root"
}

# 从 origin 推导 owner/repo。本仓库还有 upstream remote，gh 不带 --repo 时可能解析到
# istio-ecosystem 仓库，因此所有 gh 命令必须显式 --repo "$REPO_SLUG"
repo_slug() {
  local url
  url=$(git remote get-url origin 2>/dev/null) || die "找不到 origin remote"
  REPO_SLUG=$(sed -E 's#^(https://[^/]+/|git@[^:]+:|ssh://git@[^/]+(:[0-9]+)?/)##; s#\.git$##' <<<"$url")
  [[ "$REPO_SLUG" == */* ]] || die "无法从 origin URL 解析 owner/repo: $url"
}

STATE_DIR_REL="out/sync-upstream-major"   # out/ 已在 .gitignore 中
STATE_FILE_REL="$STATE_DIR_REL/state.env"

# save_state KEY VALUE：写入状态文件（幂等覆盖同名键）
save_state() {
  mkdir -p "$ROOT/$STATE_DIR_REL"
  touch "$ROOT/$STATE_FILE_REL"
  sed -i "/^$1=/d" "$ROOT/$STATE_FILE_REL"
  printf '%s=%q\n' "$1" "$2" >>"$ROOT/$STATE_FILE_REL"
}

# 加载 merge-upstream.sh 写入的状态（UPSTREAM_BRANCH/SYNC_BRANCH/MESH_VERSION 等）
load_state() {
  [[ -f "$ROOT/$STATE_FILE_REL" ]] || die "未找到 $STATE_FILE_REL，请先执行 merge-upstream.sh"
  # shellcheck disable=SC1090,SC1091
  source "$ROOT/$STATE_FILE_REL"
  [[ -n "${UPSTREAM_BRANCH:-}" && -n "${SYNC_BRANCH:-}" && -n "${MESH_VERSION:-}" ]] \
    || die "$STATE_FILE_REL 内容不完整，请重新执行 merge-upstream.sh"
}

# 构建版本 → istio 版本（1.30.3-asm-rc.4 → 1.30.3）
build_to_istio() { echo "${1%%-asm-*}"; }
# istio 版本 → 大版本（1.30.3 → 1.30）
istio_major() { echo "$1" | cut -d. -f1-2; }

# 确保 mikefarah yq v4 可用（脚本在宿主侧解析 YAML 用，与 make 容器内的 yq 无关）
ensure_yq() {
  export PATH="$ROOT/bin:$PATH"
  if command -v yq >/dev/null 2>&1 && yq --version 2>/dev/null | grep -qiE 'mikefarah|version v?4'; then
    return
  fi
  command -v go >/dev/null 2>&1 || die "缺少 mikefarah yq v4 且没有 go 可安装（go install github.com/mikefarah/yq/v4@latest）"
  info "安装 yq 到 ./bin ..."
  GOBIN="$ROOT/bin" go install github.com/mikefarah/yq/v4@latest
}

# 探测构建模式：有可用容器工具走容器模式（make 默认 BUILD_WITH_CONTAINER=1），
# 否则 BUILD_WITH_CONTAINER=0 本地工具链，并补齐 Makefile 不会自动安装的 yq / license-lint
detect_build_env() {
  local t
  for t in docker podman nerdctl; do
    if command -v "$t" >/dev/null 2>&1 && "$t" info >/dev/null 2>&1; then
      BUILD_MODE=container
      export CONTAINER_CLI="$t"
      info "构建模式: 容器（$t）"
      return
    fi
  done
  BUILD_MODE=local
  export BUILD_WITH_CONTAINER=0
  export PATH="$ROOT/bin:$PATH"
  info "构建模式: 本地工具链（未检测到可用容器工具，BUILD_WITH_CONTAINER=0）"
  command -v go >/dev/null 2>&1 || die "本地模式需要 go 1.24+"
  ensure_yq
  if ! command -v license-lint >/dev/null 2>&1; then
    info "安装 license-lint 到 ./bin ..."
    GOBIN="$ROOT/bin" go install istio.io/tools/cmd/license-lint@latest
  fi
}
