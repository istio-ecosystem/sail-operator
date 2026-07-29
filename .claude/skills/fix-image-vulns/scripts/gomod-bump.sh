#!/usr/bin/env bash
# 步骤 2c：在修复 worktree 中升级 go.mod 依赖。
# 用法: gomod-bump.sh <module@vX.Y.Z> [module@vX.Y.Z ...]
#   例: gomod-bump.sh oras.land/oras-go/v2@v2.6.1
# 版本号必须带 v 前缀（扫描给的修复候选没有 v，拼参数时要加上）。
# 只做 go get + go mod tidy；构建验证由 verify-build.sh 统一做。
# 退出码: 0=OK  非0=失败（保留现场供分析）
# 注意: go get 下载依赖可能要几分钟，Bash timeout 设 600000。

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

main() {
  repo_root
  load_state
  [[ $# -ge 1 ]] || die "用法: gomod-bump.sh <module@vX.Y.Z ...>"
  command -v go >/dev/null 2>&1 || die "找不到 go 工具链"
  local WT; WT="$(resolve_worktree)"

  cd "$WT"
  # go.mod 要求的 go 版本可能高于本机默认，auto 允许按需获取工具链
  export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"
  local before_go; before_go="$(sed -n 's/^go //p' go.mod | head -1)"

  info "go get $*"
  go get "$@"
  info "go mod tidy"
  go mod tidy

  echo
  echo "实际落位版本（依赖间约束可能使其高于请求版本，属正常，最终以此汇报）:"
  local spec mod
  for spec in "$@"; do
    mod="${spec%@*}"
    echo "  $(go list -m "$mod" 2>/dev/null || echo "$mod （已不在依赖图中）")"
  done
  local after_go; after_go="$(sed -n 's/^go //p' go.mod | head -1)"
  [[ "$before_go" != "$after_go" ]] \
    && warn "go.mod 的 go directive 被连带提升: $before_go → $after_go（GOTOOLCHAIN pin 需 ≥ 该版本，PR 正文中说明一句）"
  echo
  echo "变更文件:"
  git status --short
  echo "GOMOD_BUMPED"
  echo "下一步: verify-build.sh 做构建验证"
}

main "$@"
