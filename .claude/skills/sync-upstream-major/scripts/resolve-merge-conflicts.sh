#!/usr/bin/env bash
# 大版本同步冲突的机械化解决脚本（三层策略中的 A/B 层）。
# 用法：git merge upstream/release-1.XX 出现冲突后，在仓库根目录执行本脚本。
# A/B 层路径一律解决为上游侧（theirs）：A 层随后会被 make gen 整体重新生成，
# B 层是 alauda 从不定制的上游文件。剩余冲突（C 层）打印出来交给人工处理。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# 必须处于 merge 冲突状态，避免误操作普通工作区
if ! git rev-parse -q --verify MERGE_HEAD >/dev/null; then
  echo "错误：当前不在 merge 状态（找不到 MERGE_HEAD）。请先执行 git merge upstream/release-1.XX" >&2
  exit 1
fi

# A/B 层：解决为上游侧的路径前缀（目录以 / 结尾，也可以是具体文件）
THEIRS_PREFIXES=(
  # A 层：make gen 会重新生成
  "resources/"
  "api/"
  "bundle/"
  "bundle.Dockerfile"
  "chart/crds/"
  "chart/Chart.yaml"
  "docs/api-reference/"
  "licenses/"
  "PROJECT"
  # B 层：上游独有、alauda 不定制
  "docs/"
  "tests/e2e/"
  "common/"
  "hack/"
  "tools/"
  ".devcontainer/"
  "pkg/istioversion/versions.yaml"
  "go.sum"
)

# C 层已知文件的人工处理提示（详见 references/conflict-playbook.md）
declare -A HINTS=(
  ["Makefile.core.mk"]="取上游新内容，保留 alauda 追加的 alauda-update-values 目标和 GENERATE_RELATED_IMAGES 逻辑"
  ["chart/values.yaml"]="先取上游，make gen/alauda-update-values 后过 diff 补回 alauda 定制值"
  ["chart/templates/"]="rbac/role.yaml 取上游；olm 模板保留 alauda 改动"
  ["chart/samples/"]="取上游后把样例中的 istio 版本号对齐 alauda 默认版本"
  ["controllers/"]="核对 alauda 侧改动来源：上游 cherry-pick 取上游，自有逻辑保留"
  ["pkg/"]="核对来源；alauda 自有逻辑集中在 pkg/istiovalues/ 的 vendor_defaults 机制"
  ["tests/integration/"]="保留 vendor-defaults 相关的 alauda 改动，其余取上游"
  [".github/workflows/"]="alauda 独有 workflow 保留；上游 workflow 取上游后重放 alauda 修补"
  ["go.mod"]="取上游，校验 istio.io/istio、istio.io/api 与 alauda 最新 istio 版本一致，再 go mod tidy"
)

# 记录 MERGE_HEAD（上游侧）中存在的所有文件，用于区分“取上游内容”和“上游已删除”
declare -A IN_THEIRS=()
while IFS= read -r p; do IN_THEIRS["$p"]=1; done < <(git ls-tree -r --name-only MERGE_HEAD)

matches_theirs() {
  local f=$1 prefix
  for prefix in "${THEIRS_PREFIXES[@]}"; do
    if [[ "$f" == "$prefix" || "$f" == "$prefix"* && "$prefix" == */ ]]; then
      return 0
    fi
  done
  return 1
}

checkout_list=()  # 取上游内容
rm_list=()        # 上游已删除，跟随删除
manual_list=()    # C 层，人工处理

while IFS= read -r f; do
  [[ -n "$f" ]] || continue
  if matches_theirs "$f"; then
    if [[ -n "${IN_THEIRS[$f]:-}" ]]; then
      checkout_list+=("$f")
    else
      rm_list+=("$f")
    fi
  else
    manual_list+=("$f")
  fi
done < <(git diff --name-only --diff-filter=U)

total=$(( ${#checkout_list[@]} + ${#rm_list[@]} + ${#manual_list[@]} ))
if (( total == 0 )); then
  echo "没有未解决的冲突。"
  exit 0
fi

# 分批执行，避免超出命令行长度限制
if (( ${#checkout_list[@]} > 0 )); then
  printf '%s\0' "${checkout_list[@]}" | xargs -0 -n 200 git checkout MERGE_HEAD --
fi
if (( ${#rm_list[@]} > 0 )); then
  printf '%s\0' "${rm_list[@]}" | xargs -0 -n 200 git rm -q -f --
fi

echo "==== 机械化解决完成 ===="
echo "冲突总数           : $total"
echo "已取上游内容 (A/B) : ${#checkout_list[@]}"
echo "已随上游删除 (A/B) : ${#rm_list[@]}"
echo "剩余人工处理 (C)   : ${#manual_list[@]}"

if (( ${#manual_list[@]} > 0 )); then
  echo
  echo "==== C 层冲突清单（处理原则见 references/conflict-playbook.md）===="
  for f in "${manual_list[@]}"; do
    hint=""
    for key in "${!HINTS[@]}"; do
      if [[ "$f" == "$key" || "$f" == "$key"* && "$key" == */ ]]; then
        hint="${HINTS[$key]}"
        break
      fi
    done
    if [[ -n "$hint" ]]; then
      printf '  %-50s %s\n' "$f" "$hint"
    else
      printf '  %-50s %s\n' "$f" "（新出现的冲突文件，逐行人工判断）"
    fi
  done
  echo
  echo "C 层全部处理完后: git add <文件> && git commit -s --no-edit（禁止 amend），再执行 update-versions.sh"
else
  echo
  echo "无 C 层冲突: git commit -s --no-edit 完成合并，再执行 update-versions.sh"
fi
