#!/usr/bin/env bash
# 步骤 6：一致性校验，输出 PASS/FAIL/WARN 逐项清单。
# 退出码: 0=无 FAIL（可有 WARN）  2=存在 FAIL（修复后重跑到全过）  1=前置失败

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"
repo_root
load_state
ensure_yq

FAILS=0 WARNS=0
p() { echo "PASS: $*"; }
f() { echo "FAIL: $*"; FAILS=$((FAILS + 1)); }
w() { echo "WARN: $*"; WARNS=$((WARNS + 1)); }

MM=$(cut -d. -f1-2 <<<"$MESH_VERSION")
VF=pkg/istioversion/alauda-versions.yaml
VD=pkg/istiovalues/vendor_defaults.yaml
AV=alauda/values.yaml
CSV=bundle/manifests/servicemesh-operator2.clusterserviceversion.yaml

# 1) Makefile.vendor.mk
grep -qxF "VERSION = $MESH_VERSION" Makefile.vendor.mk \
  && p "Makefile.vendor.mk VERSION = $MESH_VERSION" \
  || f "Makefile.vendor.mk VERSION 不是 $MESH_VERSION"
grep -qxF "CHANNELS = \"$NEW_CHANNELS\"" Makefile.vendor.mk \
  && p "Makefile.vendor.mk CHANNELS = \"$NEW_CHANNELS\"" \
  || f "Makefile.vendor.mk CHANNELS 不是 \"$NEW_CHANNELS\""

# 2) bundle 的 channel（由 make bundle 从 CHANNELS 生成）
grep -q "channels.v1=\"$NEW_CHANNELS\"" bundle.Dockerfile \
  && p "bundle.Dockerfile channels = $NEW_CHANNELS" \
  || f "bundle.Dockerfile channels 不是 $NEW_CHANNELS（regen.sh 跑过了吗？）"
grep -q "channels.v1: \"$NEW_CHANNELS\"" bundle/metadata/annotations.yaml \
  && p "bundle/metadata/annotations.yaml channels = $NEW_CHANNELS" \
  || f "bundle/metadata/annotations.yaml channels 不是 $NEW_CHANNELS"

# 3) CSV 的名称与版本（relatedImages 本地不生成，GENERATE_RELATED_IMAGES 仅 release 流水线开启，不检查）
[[ "$(yq -r '.metadata.name' "$CSV")" == "servicemesh-operator2.v$MESH_VERSION" ]] \
  && p "CSV metadata.name = servicemesh-operator2.v$MESH_VERSION" \
  || f "CSV metadata.name 不是 servicemesh-operator2.v$MESH_VERSION"
[[ "$(yq -r '.spec.version' "$CSV")" == "$MESH_VERSION" ]] \
  && p "CSV spec.version = $MESH_VERSION" \
  || f "CSV spec.version 不是 $MESH_VERSION"
# 本地生成的 tag 是 $MESH_VERSION（alauda-update-values 写入）；release 流水线产物才是 $MM-latest
if grep -qE "containerImage: .*:($MESH_VERSION|$MM-latest)$" "$CSV"; then
  p "CSV containerImage tag 正确（$MESH_VERSION 或 $MM-latest）"
else
  w "CSV containerImage 不是 :$MESH_VERSION（确认 chart/values.yaml 的 operator 镜像版本行是否更新）"
fi

# 4) 上游命名的 CSV 残留
[[ -e bundle/manifests/sailoperator.clusterserviceversion.yaml ]] \
  && f "存在上游命名的 CSV 残留 bundle/manifests/sailoperator.clusterserviceversion.yaml（regen.sh 会清理）" \
  || p "无上游命名的 CSV 残留"

# 5) 矩阵内每个非 EOL 版本：resources/ 目录、alauda/values.yaml 镜像 annotation、vendor_defaults 块
NONEOL_NAMES=()
VD_KEYS=$(yq -r 'keys | .[]' "$VD" 2>/dev/null || true)
CNT=$(yq '.versions | length' "$VF")
for ((i = 0; i < CNT; i++)); do
  [[ "$(yq -r ".versions[$i].eol // false" "$VF")" == "true" ]] && continue
  ver=$(yq -r ".versions[$i].version // \"\"" "$VF")
  [[ -n "$ver" ]] || continue   # -latest 引用条目
  name="v$ver"
  NONEOL_NAMES+=("$name")
  build=$(yq -r ".versions[$i].charts[0] // \"\"" "$VF" | sed -E 's#.*/helm/base-##; s#\.tgz$##')
  vkey="v$(tr '.' '_' <<<"$ver")"

  [[ -d "resources/$name" ]] \
    && p "resources/$name 存在" \
    || f "resources/$name 缺失（regen.sh 未按新矩阵下载？）"

  miss=()
  for img in istiod proxy cni ztunnel; do
    grep -qE "images\.$vkey\.$img: .*:$build$" "$AV" || miss+=("$img")
  done
  [[ ${#miss[@]} -eq 0 ]] \
    && p "$AV images.$vkey.* 齐全且 tag=$build" \
    || f "$AV images.$vkey 缺失或 tag 不是 $build: ${miss[*]}（make alauda-update-values 跑过了吗？）"

  grep -qx "$name" <<<"$VD_KEYS" \
    && p "$VD 含 $name 块" \
    || f "$VD 缺少 $name 块（复制上一版本的块为基础人工调整）"
done

# resources/ 不应有矩阵之外的目录
for d in resources/*/; do
  [[ -d "$d" ]] || continue
  base=$(basename "$d")
  found=0
  for n in "${NONEOL_NAMES[@]:-}"; do [[ "$n" == "$base" ]] && found=1; done
  [[ $found -eq 1 ]] || f "resources/$base 不在非 EOL 矩阵中（应被 make gen 的 remove-old-versions 清掉）"
done

# vendor_defaults 的孤儿块
for k in $VD_KEYS; do
  found=0
  for n in "${NONEOL_NAMES[@]:-}"; do [[ "$n" == "$k" ]] && found=1; done
  [[ $found -eq 1 ]] || w "$VD 的 $k 块已不在非 EOL 矩阵中，确认后可删"
done

# 6) go.mod 的 istio.io 依赖与上游分支一致
if diff <(git show "upstream/$UPSTREAM_BRANCH:go.mod" | grep -E '^	istio\.io/' | sort) \
        <(grep -E '^	istio\.io/' go.mod | sort) >/dev/null 2>&1; then
  p "go.mod istio.io 依赖与 upstream/$UPSTREAM_BRANCH 一致"
else
  f "go.mod istio.io 依赖与 upstream/$UPSTREAM_BRANCH 不一致（影响生成的 CRD schema）"
fi

# 7) alauda-release.yaml 的修改落地
WF=.github/workflows/alauda-release.yaml
grep -qE "^[[:space:]]*default: \"$NEW_CHANNELS\"$" "$WF" \
  && p "$WF bundle_channels 默认值 = $NEW_CHANNELS" \
  || f "$WF bundle_channels 默认值不是 $NEW_CHANNELS"
if [[ "$UPSTREAM_BRANCH" == "release-1.30" ]]; then
  grep -qE '^[[:space:]]*TOOLS_REGISTRY_PROVIDER: registry\.istio\.io$' "$WF" \
    && p "$WF TOOLS_REGISTRY_PROVIDER = registry.istio.io（release-1.30 特例）" \
    || f "$WF TOOLS_REGISTRY_PROVIDER 未改为 registry.istio.io（release-1.30 特例）"
fi

# 8) 被淘汰大版本在样例中的残留（提示性）
for m in ${NEWLY_EOL_MAJORS:-}; do
  hits=$(grep -rl "v${m//./\\.}" chart/samples 2>/dev/null || true)
  [[ -z "$hits" ]] || w "chart/samples 中仍引用已 EOL 的 v$m（对齐到新默认版本）: $(tr '\n' ' ' <<<"$hits")"
done

echo
if [[ $FAILS -gt 0 ]]; then
  echo "RESULT: VERIFY_FAILED $FAILS 项 FAIL / $WARNS 项 WARN —— 逐项修复后重跑本脚本"
  exit 2
fi
echo "RESULT: VERIFY_PASSED 全部通过（$WARNS 项 WARN，逐条判断并写进汇报）"
echo "下一步: make lint 与 make test（耗时较长），然后进入提交与 PR"
