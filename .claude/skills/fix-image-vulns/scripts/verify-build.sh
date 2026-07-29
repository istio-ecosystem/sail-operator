#!/usr/bin/env bash
# 步骤 2d：本地构建验证（stdlib / go.mod 修复后统一执行一次）。
# 用法: verify-build.sh
# 用与流水线一致的 go 工具链编译：读修复 worktree 中 alauda-release.yaml 的 GOTOOLCHAIN pin，
# 有则强制使用（go 会按需自动下载该版本），无则 auto。go build ./... 覆盖全部包，
# 足以验证依赖升级/工具链升级未破坏构建（流水线构建二进制同样是 go build，无 docker 依赖）。
# 退出码: 0=构建验证通过（RESULT: BUILD_OK） 非0=失败（保留现场供分析）
# 注意: 首次可能要下载工具链和依赖，Bash timeout 设 600000。

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

main() {
  repo_root
  load_state
  command -v go >/dev/null 2>&1 || die "找不到 go 工具链"
  local WT; WT="$(resolve_worktree)"

  cd "$WT"
  local pin; pin="$(read_gotoolchain_pin "$WT")"
  export GOTOOLCHAIN="${pin:-auto}"
  info "GOTOOLCHAIN=$GOTOOLCHAIN（${pin:+与流水线 pin 一致}${pin:-workflow 未 pin，按 go.mod 要求自动选择}）"
  info "go version: $(go version)"

  info "构建验证: go build ./...（首次需下载依赖/工具链，可能几分钟）"
  go build ./...

  echo
  echo "RESULT: BUILD_OK"
  echo "下一步: 在 worktree 内 git add / git commit -s（禁止 amend），然后 create-pr.sh"
}

main "$@"
