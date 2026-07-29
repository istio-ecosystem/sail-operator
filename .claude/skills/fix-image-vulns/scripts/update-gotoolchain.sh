#!/usr/bin/env bash
# 步骤 2b：在修复 worktree 中 pin/升级 alauda-release.yaml 的 GOTOOLCHAIN（修 go stdlib 漏洞）。
# 用法: update-gotoolchain.sh <go1.X.Y>   例如: update-gotoolchain.sh go1.26.6
# 机制: alauda-release.yaml 在 self-hosted runner 上直接用本机 go 编译二进制（gobuild.sh），
#       顶层 env 的 GOTOOLCHAIN pin 可强制构建用指定版本（go1.21+ 会按需自动下载）。
#       仓库初始没有这一行：首次修复为新增，之后为升级。
# 只改值不 commit。新增/升级后由模型用 Edit 在该行上方补/改注释（写明本次 CVE 编号），
# 脚本会打印 NOTICE 提示。
# 退出码: 0=OK  1=失败（含 pin 低于 go.mod go directive 的情况）

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

main() {
  repo_root
  load_state
  [[ $# -eq 1 ]] || die "用法: update-gotoolchain.sh <go1.X.Y>"
  local V="$1"
  [[ "$V" =~ ^go1\.[0-9]+\.[0-9]+$ ]] || die "GOTOOLCHAIN 版本格式应为 go1.X.Y，收到: $V"
  local WT; WT="$(resolve_worktree)"

  cd "$WT"
  local f=".github/workflows/$WORKFLOW_FILE"
  [[ -f "$f" ]] || die "缺少 $f"

  # pin 必须 ≥ go.mod 的 go directive，否则 go 会拒绝构建（toolchain 低于模块要求）
  local gomod_go
  gomod_go="$(sed -n 's/^go //p' go.mod | head -1)"
  if [[ -n "$gomod_go" ]]; then
    local lower
    lower="$(printf '%s\n%s\n' "${V#go}" "$gomod_go" | sort -V | head -1)"
    [[ "$lower" == "$gomod_go" ]] \
      || die "pin 版本 $V 低于 go.mod 的 go directive（$gomod_go），构建会失败。请选 ≥ $gomod_go 的版本"
  fi

  local old
  old="$(read_gotoolchain_pin "$WT")"
  if [[ "$old" == "$V" ]]; then
    echo "OK: $f GOTOOLCHAIN 已是 $V"
  elif [[ -n "$old" ]]; then
    sed -i -E "s|^([[:space:]]*)GOTOOLCHAIN:.*$|\1GOTOOLCHAIN: $V|" "$f"
    grep -qE "^[[:space:]]*GOTOOLCHAIN:[[:space:]]*$V$" "$f" \
      || die "$f GOTOOLCHAIN 替换未生效"
    echo "OK: $f GOTOOLCHAIN: $old → $V"
  else
    # 首次 pin：插入顶层 env: 块（行首 env:，job/step 内的 env 缩进不同不会误中）
    grep -q '^env:$' "$f" || die "$f 中找不到顶层 env: 块，构建链可能已变化。请先重新分析 GOTOOLCHAIN 方案是否仍适用（见 SKILL.md 背景知识），适用则用 Edit 手动加 pin"
    awk -v v="$V" '{print} /^env:$/ && !done {print "  GOTOOLCHAIN: " v; done=1}' "$f" >"$f.tmp" \
      && mv "$f.tmp" "$f"
    grep -qE "^[[:space:]]*GOTOOLCHAIN:[[:space:]]*$V$" "$f" \
      || die "$f GOTOOLCHAIN 插入未生效"
    echo "OK: $f 顶层 env 新增 GOTOOLCHAIN: $V"
  fi

  echo
  if [[ -n "$old" ]] && grep -B3 '^[[:space:]]*GOTOOLCHAIN:' "$f" | grep -q '#'; then
    echo "NOTICE: GOTOOLCHAIN 上方已有注释（上次 pin 的缘由），请审阅并用 Edit 更新为本次升级的缘由（CVE 编号等）："
    grep -n -B3 '^[[:space:]]*GOTOOLCHAIN:' "$f" | grep '#' | sed 's/^/    /'
  else
    echo "NOTICE: 请用 Edit 在 GOTOOLCHAIN 行上方补一行注释，写明本次 pin 的缘由（CVE 编号、修复版本），例如："
    echo "    # CVE-XXXX-XXXXX（stdlib）修于 $V；显式 pin 覆盖 runner 本机 go 版本"
  fi
  echo
  echo "GOTOOLCHAIN_UPDATED"
  git diff --stat -- "$f" | tail -3
}

main "$@"
