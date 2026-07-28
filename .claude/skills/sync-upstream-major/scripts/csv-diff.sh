#!/usr/bin/env bash
# 步骤 5：提取 CSV（ClusterServiceVersion）两侧变更，供模型做语义比对分析。
#   上游: bundle/manifests/sailoperator.clusterserviceversion.yaml（merge-base → upstream/release-1.XX）
#   alauda: bundle/manifests/servicemesh-operator2.clusterserviceversion.yaml（同步前基线 → 当前 HEAD）
# 产物写入 out/sync-upstream-major/。退出码: 0=OK（无变更会标注 NO_CHANGE）  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"
repo_root
load_state

OUT="$ROOT/$STATE_DIR_REL"
mkdir -p "$OUT"
UP_CSV=bundle/manifests/sailoperator.clusterserviceversion.yaml
AL_CSV=bundle/manifests/servicemesh-operator2.clusterserviceversion.yaml

# merge 的 rename 探测可能把 alauda CSV 判为「上游已删除」而移除，regen.sh 的 make bundle 会重建它
[[ -f "$AL_CSV" ]] || die "$AL_CSV 不存在：请先执行 regen.sh（本步骤应在重新生成之后运行）"

git diff "$MERGE_BASE" "upstream/$UPSTREAM_BRANCH" -- "$UP_CSV" >"$OUT/csv-upstream.diff"
git show "upstream/$UPSTREAM_BRANCH:$UP_CSV" >"$OUT/csv-upstream-full.yaml" 2>/dev/null \
  || warn "上游分支中没有 $UP_CSV（文件名变了？请人工确认）"
git diff "$BASE_SHA" HEAD -- "$AL_CSV" >"$OUT/csv-alauda.diff"

report_file() { # $1=文件 $2=说明
  local n
  n=$(grep -cE '^[+-][^+-]' "$1" 2>/dev/null || true)
  if [[ -s "$1" ]]; then
    echo "  $2: $1（${n} 行增删）"
  else
    echo "  $2: NO_CHANGE"
  fi
}

echo "==== CSV 变更提取完成 ===="
report_file "$OUT/csv-upstream.diff" "上游 CSV 变更（$MERGE_BASE → upstream/$UPSTREAM_BRANCH）"
report_file "$OUT/csv-alauda.diff"   "alauda CSV 变更（同步前 → HEAD）"
echo "  上游 CSV 全文: $OUT/csv-upstream-full.yaml"
echo
echo "下一步（模型执行）: Read 两个 diff 做语义比对——上游结构性变更（RBAC/env/容器参数/CRD/alm-examples/webhooks）"
echo "是否已体现在 alauda CSV；未落地的改 chart/templates/olm/ 模板后重跑 regen.sh 与本脚本；逐条结论写入最终汇报"
