---
name: sync-upstream-major
description: 同步上游 istio-ecosystem/sail-operator 的指定 release-1.XX 分支到本仓库（alauda-mesh/sail-operator）的目标分支，完成 mesh 大版本升级：merge 上游并按三层策略解决冲突、按给定 istio 构建版本更新 alauda 版本矩阵、重新生成、分析上游 CSV 变更、校验、创建 PR、触发并监控 Alauda Release 流水线。仅适用于大版本同步（如 istio 1.28 → 1.30、mesh 2.1 → 2.2），小版本 patch 同步不在范围内。仅限用户显式通过 /sync-upstream-major 调用。
argument-hint: "[上游release分支] [目标分支] [istio构建版本...]，例如: release-1.30 main 1.30.3-asm-rc.4 1.28.6-asm-r4 1.28.3-asm-r3"
disable-model-invocation: true
---

# 上游大版本同步（istio-ecosystem/sail-operator → alauda-mesh/sail-operator）

把上游 sail-operator 的一个新大版本合并进 alauda fork：新增一个 istio 大版本、淘汰最老的大版本、发布新的 mesh 2.X。下文 `$SKILL_DIR` 指本 skill 根目录（即调用时提示的 Base directory）。

## 参数

- 上游 release 分支：`$0`（形如 `release-1.30`）
- 目标分支：`$1`（通常 `main`，同步 PR 的 base 分支）
- istio 构建版本：`$2` 起的其余参数：
  - **必须**至少含一个新大版本的构建（如 `1.30.3-asm-rc.4`，大版本须与 release 分支一致）；
  - **可选**再给上一个大版本的若干构建（如 `1.28.6-asm-r4 1.28.3-asm-r3`）。给了 → 该大版本在 `alauda-versions.yaml` 中的条目**以给出的列表为准**（新 channel 允许裁剪旧 revision）；没给 → 保持现有条目不变。

任一必填项缺失时用 AskUserQuestion 向用户询问（上游分支可先 `git branch -r | grep upstream/release-` 列出候选），不要自行猜测。新 mesh 版本默认从 `Makefile.vendor.mk` 的 VERSION 推导（次版本 +1、patch 归 0，如 2.1.2 → 2.2.0）；不符预期时以 `MESH_VERSION=2.X.0` 环境变量执行步骤 1 覆盖。

## 背景知识

- **版本模型**：alauda 只维护两个 istio 大版本（如 2.1 = 1.26 + 1.28，2.2 = 1.28 + 1.30）。istio 一年发 4 个大版本、alauda 一年发 3 个 mesh 版本，因此会不时跳过一个 istio 大版本——两个维护版本不保证连续，也不保证是偶数。
- **合并上游 `release-1.XX` 分支，而不是 `main`**：main 含大量未发布的下一版本开发代码；release 分支是已发布代码加 backport，且后续小版本同步只需再合同一分支。实测依据见 `references/conflict-playbook.md`。
- **冲突用三层路径策略机械化解决**：实测 98% 冲突在 `resources/` 等生成目录（1690 → 14），脚本自动处理 A/B 层，C 层（约 10~14 个 alauda 定制文件）人工按 playbook 处理。
- **构建环境自适应**：脚本自动探测 docker/podman/nerdctl——有可用的就走容器模式（make 默认），没有则 `BUILD_WITH_CONTAINER=0` 本地工具链（自动补装 yq、license-lint、go-junit-report、golangci-lint、shellcheck v0.8.0 到 `./bin`）。两种模式生成结果一致。**脱离脚本手动重跑单个 make 目标时必须带上 `PATH="$PWD/bin:$PATH" BUILD_WITH_CONTAINER=0`**，否则 yq 等工具找不到（Error 127）。lint-yaml 需要 yamllint（Python 工具，无法装进 ./bin）：环境有免密 sudo 时 `sudo apt-get install -y yamllint`，没有则如实汇报跳过。环境通常无 python3，临时文本处理用 awk/sed/yq。
- 脚本间通过 `out/sync-upstream-major/state.env` 传递状态（`out/` 已 gitignore）。步骤 7 之前不要 push、不要建 PR。
- 全程禁止 `git commit --amend`，一律新 commit，全部 `-s` 签名。
- 本仓库有 `upstream` remote，gh 不带 `--repo` 时可能解析错仓库——脚本内已统一处理；你手动执行 gh 时也要带 `--repo <origin 的 owner/repo>`。
- **大 PR 的 review 盲区**：同步 PR 有 600+ 变更文件，`gh pr view --json files` 只返回前 100 个、`gh pr diff`（REST diff）只含前 300 个文件的 patch、网页 files changed 需多次 Load more——排序靠后的文件（如 go.mod）容易被误判为"没改"。核对单个文件一律用本地 `git diff <上游快照> HEAD -- <文件>`，不要依赖 PR 视图。依赖版本的比较基准是 **release-1.XX**（不是 upstream main——main 是下一版本开发分支，依赖天然更新，与之对比会得出"很多库没升级"的错误结论）。PR 描述中写明合并的上游快照 SHA，方便 reviewer 用同一基准核对。
- **release-1.30 特例**：上游把 build-tools 镜像迁到了 registry.istio.io，`alauda-release.yaml` 的 `TOOLS_REGISTRY_PROVIDER` 需改为 `registry.istio.io`。update-versions.sh 已内置该分支逻辑；后续大版本同步不涉及，届时可移除。

## 步骤 1：合并上游

```bash
bash "$SKILL_DIR/scripts/merge-upstream.sh" <上游release分支> <目标分支> <istio构建版本>...
```

脚本自动：参数与构建版本分组校验 → 工作区检查 → fetch → r2.dev charts 可达性校验 → 推导 mesh 版本并写入状态 → 基于 `origin/<目标分支>` 创建 `sync/<日期>` 分支 → merge。按结果处理：

- **CONFLICT（退出码 2）**：正常路径（1500~2500 个冲突），继续步骤 2；
- **MERGED / UP_TO_DATE（0）**：无冲突直接进步骤 3；UP_TO_DATE 多为续跑场景，直接执行后续步骤；
- **退出码 1**：前置问题（charts 404 说明 alauda-mesh/istio 还没构建完、分支已存在、工作区不干净等），把报错原样告知用户并询问处理方式，不要擅自 stash 或删分支。

## 步骤 2：解决冲突（三层策略）

```bash
bash "$SKILL_DIR/scripts/resolve-merge-conflicts.sh"
```

脚本把 A 层（`make gen` 会重建的生成物）和 B 层（上游独有文件）机械解决为上游侧，并列出剩余 C 层清单及各文件处理提示。C 层人工处理原则：**以上游为新基底，重放 alauda 自有改动**——先 `git log --oneline <merge-base>..HEAD -- <文件>` 查来源，上游 cherry-pick（消息带上游 PR 号）取上游侧，alauda 自有改动（FIPS、multus、资源限额、servicemesh-operator2 命名、CI 定制）保留重放。每个文件的具体原则见 `references/conflict-playbook.md`；**每个 C 层文件的冲突点和解法记录下来，最终汇报逐条说明**。拿不准的冲突停下来向用户提问（附冲突片段和倾向方案）。

**语义冲突哨兵**（脚本末尾会输出核查清单）：git 自动合并"成功"的文件也可能坏——上游收编 alauda 的 cherry-pick 后重构签名/位置时，双侧改动会被 git 拼在一起产生**同名函数重复定义**（1.30 实测：`pkg/istiovalues/fips.go`/`fips_test.go` 各多出一个旧签名的 `ApplyZTunnelFipsValues`，git 无冲突但编译失败）。对哨兵清单里的文件逐个 `diff <(git show upstream/<分支>:<文件>) <文件>`：alauda 侧改动若已被上游收编（功能等价），直接对齐上游。

全部解决后 `git add <文件>` 并 `git commit -s --no-edit` 完成合并，随后 **`go build ./... && go vet ./...` 冒烟**确认无语义冲突（vet 会连测试文件一起编译）。

## 步骤 3：更新版本信息

```bash
bash "$SKILL_DIR/scripts/update-versions.sh"
```

脚本机械完成（不 commit，便于 review）：

- `Makefile.vendor.mk`：`VERSION` → 新 mesh 版本；`CHANNELS` → `"stable,stable-2.X"`（bundle.Dockerfile、CSV、annotations 的 channel 均由它生成，不用手改）；
- `.github/workflows/alauda-release.yaml`：`bundle_channels` 默认值 → `stable,stable-2.X`；release-1.30 时把 `TOOLS_REGISTRY_PROVIDER` 改为 `registry.istio.io`；
- `pkg/istioversion/alauda-versions.yaml`：按构建版本重建矩阵——新大版本条目（`vX.YY-latest` + 各构建的 charts URL）、上一大版本（给了构建列表则重建，否则原样保留）、被淘汰的大版本按既有 EOL 格式收尾；并输出新旧矩阵对照（进最终汇报）。

**PATTERN_MISMATCH（退出码 2）**：文件结构变化导致某些项没匹配上，按输出的 FAIL 清单用 Edit 手动完成，已 OK 的项不要重复改。

脚本之后的人工项（NOTICE 会逐条列出）：

- `pkg/istiovalues/vendor_defaults.yaml`：为每个新增 istio 版本添加默认值块（复制上一版本的块为基础，multus、资源限额等逐项判断是否仍适用）；被淘汰版本的孤儿块确认后可删；
- go.mod 的 istio.io 依赖若与上游分支不一致（NOTICE 提示），回查步骤 2 是否有文件没取上游侧。

## 步骤 4：重新生成

```bash
bash "$SKILL_DIR/scripts/regen.sh"
```

脚本自动探测构建模式，然后 `go mod tidy` → 清理 `resources/` 版本目录（保留 `resources.go` 等非版本文件，它是 release-1.30 起被跟踪的 go:embed 包）→ `make gen` → `make alauda-update-values` → 清理 `bundle/manifests/` 下上游命名的残留（`sailoperator.clusterserviceversion.yaml`、`sailoperator-*`、`sail-operator-*`，`sailoperator.io_*` 是 CRD 域名文件必须保留）→ `git add -A`。**耗时 5~15 分钟**，Bash 调用要设大 timeout（如 900000）或后台运行；失败时 make 的报错就是现场，修复后整体重跑（幂等）。

生成后过 `git diff HEAD -- chart/values.yaml` 回补 alauda 定制值。1.30 实测明细（`make gen` 不管理这两块，历史上 alauda 手动维护）：

- `deployment.annotations`：merge 取上游后是上游全量镜像矩阵（~88 个 registry.istio.io 键）。**必须裁剪**为 alauda 矩阵内版本 + build-harbor 镜像 + 本次构建号——`make bundle` 用 `HELM_VALUES_FILE=alauda/values.yaml` 与 chart 默认 values **深合并**，alauda 未覆盖的上游键会漏进 bundle CSV（实测残留 72 处）；
- `csv.longDescription` 的版本列表：改为新矩阵列表；
- `csv.version` 与 `image:` 两行由脚本 sed 自动回写，不用管；`platform:` 键被上游删除属正常（模板已无引用），不要补回；`alauda/values.yaml` 由 patch-values.sh 自动更新，无需人工。
- **回补 annotations 后必须重跑 `make bundle`**（`PATH="$PWD/bin:$PATH" BUILD_WITH_CONTAINER=0 make bundle`）让裁剪流入 CSV，然后 `grep -c registry.istio.io bundle/manifests/servicemesh-operator2.clusterserviceversion.yaml` 应为 0。

## 步骤 5：CSV 变更分析

```bash
bash "$SKILL_DIR/scripts/csv-diff.sh"
```

脚本产出三个文件到 `out/sync-upstream-major/`：`csv-upstream.diff`（上游 CSV 在 merge-base → release-1.XX 间的变更）、`csv-upstream-full.yaml`（上游 CSV 全文）、`csv-alauda.diff`（alauda CSV 同步前 → 当前的变更）。Read 后逐块分析 `csv-upstream.diff`（版本号、镜像 tag、createdAt 变化属预期噪音，重点看结构性变更）：

- 关注类别：`clusterPermissions`/`permissions`（RBAC）、`deployments`（容器 args、env、探针、资源）、`customresourcedefinitions.owned`、`alm-examples`、webhooks、annotations、描述文本；
- 逐条核对是否已体现在 `csv-alauda.diff`。alauda CSV 由 `make bundle` 从 `chart/templates/olm/clusterserviceversion.yaml` 模板生成，上游多数变更经步骤 2/4 自动流入；名称（sailoperator → servicemesh-operator2）、镜像、displayName、provider 等 alauda 定制差异不算缺失；
- 上游变更没落地的：改 `chart/templates/olm/` 模板或对应 values → 重跑步骤 4 → 重跑本脚本确认；拿不准是否该跟的变更向用户提问；
- **逐条结论（自动流入了什么 / 补了什么 / 为什么跳过）写进最终汇报**——这是本步骤的硬性输出。

## 步骤 6：校验

```bash
bash "$SKILL_DIR/scripts/verify.sh"
```

脚本逐项校验：vendor.mk 版本与 channels、bundle 两处 channel、CSV 的 name/version、上游 CSV 残留、矩阵内每个非 EOL 版本的 `resources/` 目录与 `alauda/values.yaml` 镜像 annotation（build 号一致）、vendor_defaults 块齐全、go.mod istio 依赖、workflow 修改落地，以及**与上游合并快照（state 的 UPSTREAM_SHA）的全量差异审计**：go.mod/go.sum 与快照 diff（一致 PASS；有差异 WARN 并列出差异行——**fork 主动的 CVE/安全升级属合法差异**，但 go.mod 是 git 自动合并重灾区、混血旧版本行不带冲突标记，必须逐行确认差异是有意为之并写进汇报）、licenses/ 一致性（依赖对齐的独立佐证，go.mod 有意差异时确认 mirror-licenses 已重跑）、白名单外与上游不同的文件逐个 WARN（每个都必须能解释：本次刻意修改 → 写进汇报；解释不了 → 对齐上游）。**FAIL（退出码 2）逐项修复后重跑到全 PASS**；WARN 逐条判断并写进汇报。relatedImages 本地不生成（`GENERATE_RELATED_IMAGES` 仅 release 流水线开启），不算缺失。

然后跑 `make lint` 与 `make test`（耗时较长，设大 timeout 或后台运行；手动执行带 `PATH="$PWD/bin:$PATH" BUILD_WITH_CONTAINER=0`）。1.30 实测经验：

- **`make test` 必须带 `USE_VENDOR_DEFAULTS=false`**（与 CI 的 integration-tests.yaml 一致）——不设的话 vendor defaults 的 multus 注入会让 integration 大片超时失败；
- integration 偶发 1 例 optimistic-concurrency flaky（"the object has been modified"）：重跑 `make test.integration` 一次即可确认，非同步引入；
- **`lint-crds` 预期失败不用修**：它硬编码与上游 repo 的 PREVIOUS_VERSION 对比 version enum，alauda 矩阵天然是上游子集（且版本淘汰是 alauda 版本模型的预期行为），全部 NoEnumRemoval 报错属误报；alauda CI 无此门禁，写进汇报即可；
- 其余 lint 工具缺失时 common.sh 会自动补装；lint-yaml 的 yamllint 见背景知识。若 lint 报到 `.claude/skills` 自己的脚本，直接修脚本（shellcheck 干净 + Apache copyright 头是 lint-scripts/lint-copyright-banner 的硬要求）。

## 步骤 7：提交与 PR

把步骤 3~6 的修改拆成有逻辑的新 commit（全部 `-s` 签名并附说明，禁止 amend），建议拆分：版本信息更新、生成物刷新、CSV/模板修补（如有）。然后把 PR 描述写进 scratchpad 临时文件（最终汇报的精简版，结尾加一行 `🤖 Generated with [Claude Code](https://claude.com/claude-code)`）：

```bash
bash "$SKILL_DIR/scripts/create-pr.sh" <PR正文文件>
```

幂等（分支已有 open PR 时复用），输出 `PR_NUMBER=`/`PR_URL=`。gh 未认证时提示用户执行 `! gh auth login` 后重试。

## 步骤 8：触发并监控 Alauda Release 流水线

```bash
bash "$SKILL_DIR/scripts/run-release.sh"
```

脚本用 gh 在同步分支上 dispatch `alauda-release.yaml`：`release_version=<mesh版本>-r<时间戳>`（如 `2.2.0-r20260728093734`，日期后缀避免与手动发布冲突）、`bundle_channels=stable,stable-2.X`，其余输入用默认值；输出 `RUN_ID=`/`RUN_URL=`。**RUN_NOT_FOUND（退出码 4）**：按脚本提示排查（分支上 workflow 文件语法、runner 是否在线），如实告知用户。

然后监控（多平台镜像构建 20~60 分钟，**必须后台运行**——Bash 工具 `run_in_background: true`）：

```bash
bash "$SKILL_DIR/scripts/watch-release.sh"
```

- **PIPELINE_SUCCESS（0）**：把输出中的 operator / bundle 镜像名加入最终汇报；
- **PIPELINE_FAILED（2）**：输出附失败 step 概览与日志摘要（完整失败日志在 `out/sync-upstream-major/release-failed.log`）。若输出含 **`KNOWN_ISSUE: BUILD_TOOLS_IMAGE_PULL`**（istio-testing/build-tools 镜像下载问题），按约定**不要自行修复，直接报告用户处理**。若含 **`KNOWN_ISSUE: GH_RELEASE_TARGET_BRANCH_MISSING`**（1.30 首战实测）：create-gh-release 的 `--target release-2.X` 分支在 PR 合并前不存在（HTTP 422 Invalid target_commitish），是验证 run 的**预期尾部失败**——按脚本提示核对镜像 step 全绿后视为同步验证通过，镜像名从 `Output image:` step 名提取进汇报，GitHub release 留待 PR 合并、release-2.X 分支创建后的正式发版，写进遗留事项即可。其他失败：定位失败 step，判断是本次同步引入（构建错误、bundle 校验失败、workflow 编辑错误）还是环境问题（runner、registry 登录）；属同步引入的：修复 → 新 commit → `git push origin HEAD` → 重跑 run-release.sh（生成新时间戳版本）→ 重新后台 watch；拿不准的修复先向用户提问；
- **PIPELINE_TIMEOUT（3）**：告知用户仍在运行，附 RUN_URL。

## 最终汇报

用清晰列表向用户汇报（这是用户 review 的依据）：

1. 版本跨度：mesh 旧 → 新；istio 矩阵新旧对照（含各版本构建号，取步骤 3 输出）；
2. 合并：同步分支、合入提交数、冲突统计（A/B 机械 / C 人工），C 层逐文件冲突点与解法；
3. 版本文件修改：vendor.mk、alauda-versions.yaml、vendor_defaults 新增块要点、workflow 修改（channels；release-1.30 的 TOOLS_REGISTRY_PROVIDER 特例）；
4. **CSV 变更分析**：上游 CSV 逐条变更 → alauda 落地结论（必含，即使结论是「全部经模板自动流入」）；
5. 校验：verify 的 WARN 项说明、lint / test 结果、**全量差异审计结论**（与上游快照不同的文件数及归类：alauda 独有 / 定制 / 生成 / 刻意修改，白名单外 WARN 逐条解释）；
6. PR 链接（描述中注明合并的上游快照 SHA 与 review 基准提示）；
7. 流水线：release_version、RUN_URL、结果（成功镜像名 / 失败分析与处理）；
8. 遗留事项（需用户定夺的项）。

到此流程结束，等用户 review 与合并；不要自行 merge PR。
