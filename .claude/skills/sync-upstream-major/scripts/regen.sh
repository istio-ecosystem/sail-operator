#!/usr/bin/env bash

# Copyright Alauda Mesh Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# 步骤 4：重新生成全部生成物。耗时可达 5~15 分钟（下载 charts、controller-gen、operator-sdk 等），
# 调用方要设大 timeout 或后台运行。幂等，失败修复后可整体重跑。
# 退出码: 0=OK  1=失败（make 的输出即错误现场）

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
repo_root
load_state
detect_build_env

git rev-parse -q --verify MERGE_HEAD >/dev/null \
  && die "merge 未完成：先解决 C 层冲突并 git commit -s --no-edit"
git diff --name-only --diff-filter=U | grep -q . && die "仍有未解决的冲突文件"
# C 层合并质量守卫：Makefile.core.mk 的 alauda 追加内容必须在（playbook 的 C 层第一条）
grep -qE '^alauda-update-values:' Makefile.core.mk \
  || die "Makefile.core.mk 缺少 alauda-update-values 目标——步骤 2 的 C 层合并丢了 alauda 追加内容（见 conflict-playbook），重放后重跑"

command -v go >/dev/null 2>&1 || die "需要 go（go mod tidy 在宿主侧执行）"
info "go mod tidy ..."
go mod tidy

# release-1.30 起 resources/ 还包含被跟踪的 resources.go（go:embed 包），只能清版本目录、不能整目录清空
info "清理 resources/ 版本目录（make gen 会按 alauda-versions.yaml 重新下载）..."
find resources -mindepth 1 -maxdepth 1 -name 'v*' -type d -exec rm -rf {} +

info "make gen ..."
make gen

info "make alauda-update-values ..."
make alauda-update-values

# 上游命名的 bundle 文件（CSV、metrics-reader clusterrole、metrics-service 等）是 A 层「取上游侧」
# 的合并残留（alauda 的 operator 名是 servicemesh-operator2），make gen 只会重写 alauda 命名的文件，
# 不会清理它们。注意 sailoperator.io_*.yaml 是 CRD 文件（API group 域名），不是上游命名残留，必须保留。
for leftover in bundle/manifests/sailoperator.*.yaml bundle/manifests/sailoperator-*.yaml bundle/manifests/sail-operator-*.yaml; do
  [[ -e "$leftover" ]] || continue
  [[ "$leftover" == bundle/manifests/sailoperator.io_* ]] && continue
  git rm -f -q --ignore-unmatch "$leftover" || true
  rm -f "$leftover"
  info "已清理上游命名的 bundle 残留 $leftover"
done

git add -A
echo "RESULT: OK 生成完成（构建模式: $BUILD_MODE），工作区变更统计:"
git status --short | awk '{print $1}' | sort | uniq -c | sort -rn | sed 's/^/  /'
echo "下一步: csv-diff.sh 提取 CSV 变更进行分析"
