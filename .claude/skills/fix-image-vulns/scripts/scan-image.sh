#!/usr/bin/env bash
# 步骤 1b / 步骤 6：调内网扫描服务扫当前轮镜像，并按修复责任分类。
# 用法: scan-image.sh [轮次]   缺省用状态里的 ROUND
# 分类:
#   OS_REPORT_ONLY  os 包（基础镜像层）        → 不修复，如实报告
#   GO_STDLIB       go 二进制的 stdlib         → pin/升级 alauda-release.yaml 的 GOTOOLCHAIN
#   GO_MODULE       go 二进制的依赖库          → 升级 go.mod
# 输出: 漏洞明细 + 修复目标聚合（含当前 GOTOOLCHAIN pin / go.mod go directive 状态）
#       + SUMMARY + RESULT: CLEAN|REPORT_ONLY|FIX_NEEDED
# 退出码: 0=扫描完成（无论结论） 1=失败
# 注意: 服务端要拉镜像再扫，首扫可能几分钟，调用方把 Bash timeout 设为 600000。
# 环境变量: SCAN_API（默认 http://192.168.25.100:8888）、MAX_ATTEMPTS=3、RETRY_DELAY=15、SCAN_TIMEOUT=240

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

MAX_ATTEMPTS="${MAX_ATTEMPTS:-3}"
RETRY_DELAY="${RETRY_DELAY:-15}"
SCAN_TIMEOUT="${SCAN_TIMEOUT:-240}"

main() {
  repo_root
  load_state
  command -v jq >/dev/null 2>&1 || die "找不到 jq"

  local N="${1:-$ROUND}"
  local IMG_TSV="$STATE_DIR/images-round${N}.tsv"
  [[ -s "$IMG_TSV" ]] || die "没有第 ${N} 轮镜像清单: $IMG_TSV（第 1 轮先执行 resolve-input.sh；回归轮先执行 watch-run.sh）"

  local SCAN_DIR="$STATE_DIR/scans/round${N}"
  mkdir -p "$SCAN_DIR"
  local TSV="$SCAN_DIR/vulns.tsv"
  : >"$TSV"

  local img out resp encoded url i n rows
  while IFS= read -r img; do
    out="$SCAN_DIR/$(img_slug "$img").json"

    info "扫描 ${img} ..."
    encoded="$(jq -rn --arg v "$img" '$v|@uri')"
    url="${SCAN_API}/image/vulnerability/custom?image_full_address=${encoded}&trivy_db_date=latest&severity=low&vulnerability_type=os%2Clibrary&version=v4.4.0"
    resp=""
    for i in $(seq 1 "$MAX_ATTEMPTS"); do
      if resp="$(curl -sS --fail --max-time "$SCAN_TIMEOUT" -H 'accept: application/json' "$url")" \
         && jq -e 'has("os") and has("lang")' <<<"$resp" >/dev/null 2>&1; then
        break
      fi
      resp=""
      [[ "$i" -lt "$MAX_ATTEMPTS" ]] && { warn "本次扫描失败，${RETRY_DELAY}s 后重试（$i/$MAX_ATTEMPTS）"; sleep "$RETRY_DELAY"; }
    done
    [[ -n "$resp" ]] || die "扫描服务连续 $MAX_ATTEMPTS 次失败: $img（SCAN_API=$SCAN_API）"
    printf '%s\n' "$resp" >"$out"

    # 提取为 TSV: 分类/包/装机版本/修复候选(逗号分隔,去v前缀)/CVE/严重度
    # 注: 服务对无修复版本返回字符串 "null"，要当空处理，否则会算出 @vnull 的假目标
    rows="$(jq -r '
      def fixes: [(.FixedVersion // "") | split(",")[] | gsub("^\\s+|\\s+$";"") | sub("^v";"") | select(.!="" and .!="null")] | unique | join(",");
      def cat: if .__src == "os" then "OS_REPORT_ONLY"
               elif .PkgName == "stdlib" then "GO_STDLIB"
               else "GO_MODULE" end;
      ( [(.os // [])[] | . + {__src:"os"}] + [(.lang // [])[] | . + {__src:"lang"}] )
      | .[] | [cat, .PkgName, (.InstalledVersion // "?"), fixes, .VulnerabilityID, (.Severity // "?")] | @tsv' "$out")"

    n=0
    if [[ -n "$rows" ]]; then
      rows="$(sort -u <<<"$rows")"
      printf '%s\n' "$rows" >>"$TSV"
      n="$(grep -c . <<<"$rows")"
    fi
    echo
    echo "--- ${img}（漏洞 ${n} 条）---"
    [[ -n "$rows" ]] && sort <<<"$rows" | awk -F'\t' \
      '{printf "  [%s] %s %s %s %s → %s\n", $1, $2, $5, $6, $3, ($4 == "" ? "（无修复版本）" : $4)}'
  done < <(cut -f3 "$IMG_TSV" | sort -u)

  sort -u "$TSV" -o "$TSV"
  count_cat() { awk -F'\t' -v c="$1" '$1 == c' "$TSV" | grep -c . || true; }
  local TOTAL GOMOD STDLIB OS
  TOTAL="$(grep -c . "$TSV" || true)"
  GOMOD="$(count_cat GO_MODULE)"; STDLIB="$(count_cat GO_STDLIB)"; OS="$(count_cat OS_REPORT_ONLY)"

  # ---------- 修复目标聚合（对照修复 worktree；未建 worktree 时对照主工作区当前检出） ----------
  local SRC_DIR="${WORKTREE:-$ROOT}"
  if [[ "$STDLIB" -gt 0 || "$GOMOD" -gt 0 ]]; then
    echo
    echo "--- 修复目标（对照 $SRC_DIR）---"
    local pin gomod_go
    pin="$(read_gotoolchain_pin "$SRC_DIR")"
    gomod_go="$(sed -n 's/^go //p' "$SRC_DIR/go.mod" | head -1)"
    echo "  当前 GOTOOLCHAIN pin: ${pin:-（未设置，$WORKFLOW_FILE 中无此环境变量，将由 runner go 版本与 go.mod go directive 共同决定）}"
    echo "  go.mod go directive: ${gomod_go:-?}（GOTOOLCHAIN pin 必须 ≥ 它）"

    local stdlib_rows inst cands
    stdlib_rows="$(awk -F'\t' '$1=="GO_STDLIB"' "$TSV" | cut -f2- | sort -u)"
    if [[ -n "$stdlib_rows" ]]; then
      inst="$(head -1 <<<"$stdlib_rows" | cut -f2)"
      cands="$(cut -f3 <<<"$stdlib_rows" | tr ',' '\n' | grep -v '^$' | sort -uV | paste -sd'/' -)"
      echo "  [stdlib] 构建 go ${inst}  CVE×$(grep -c . <<<"$stdlib_rows")  修复候选: ${cands:-（无）} → update-gotoolchain.sh（优先同小版本线的 patch；需跨大版本时在最终报告中着重说明）"
    fi

    local pkg cves_fix cves_nofix target
    while IFS= read -r pkg; do
      [[ -n "$pkg" ]] || continue
      inst="$(awk -F'\t' -v p="$pkg" '$1=="GO_MODULE" && $2==p {print $3; exit}' "$TSV")"
      cves_fix="$(awk -F'\t' -v p="$pkg" '$1=="GO_MODULE" && $2==p && $4!="" {print $5}' "$TSV" | sort -u | grep -c . || true)"
      cves_nofix="$(awk -F'\t' -v p="$pkg" '$1=="GO_MODULE" && $2==p && $4=="" {print $5}' "$TSV" | sort -u | grep -c . || true)"
      cands="$(awk -F'\t' -v p="$pkg" '$1=="GO_MODULE" && $2==p {print $4}' "$TSV" \
        | tr ',' '\n' | grep -v '^$' | sort -uV || true)"
      if [[ -n "$cands" ]]; then
        target="$(tail -1 <<<"$cands")"
        echo "  [go.mod] $pkg $inst  CVE×${cves_fix}$([[ "$cves_nofix" -gt 0 ]] && echo "（另 ${cves_nofix} 个无修复版本）")  候选: $(paste -sd'/' - <<<"$cands")  → gomod-bump.sh ${pkg}@v${target}"
      else
        echo "  [go.mod] $pkg $inst  CVE×${cves_nofix}  （无修复版本，升级修不了，最终汇报中如实说明）"
      fi
    done < <(awk -F'\t' '$1=="GO_MODULE" {print $2}' "$TSV" | sort -u)
  fi

  echo
  echo "SUMMARY: ROUND=${N} TOTAL=${TOTAL} GO_MODULE=${GOMOD} GO_STDLIB=${STDLIB} OS_REPORT_ONLY=${OS}"
  # 可执行修复 = 有修复候选的 GO_MODULE / GO_STDLIB
  local FIXABLE
  FIXABLE="$(awk -F'\t' '($1=="GO_MODULE" || $1=="GO_STDLIB") && $4!=""' "$TSV" | grep -c . || true)"
  if [[ "$TOTAL" -eq 0 ]]; then
    echo "RESULT: CLEAN"
  elif [[ "$FIXABLE" -gt 0 ]]; then
    echo "RESULT: FIX_NEEDED"
  else
    echo "RESULT: REPORT_ONLY（剩余均为不修复项：os 级 / 无修复版本的条目）"
  fi
  echo "明细 TSV: $TSV"
}

main "$@"
