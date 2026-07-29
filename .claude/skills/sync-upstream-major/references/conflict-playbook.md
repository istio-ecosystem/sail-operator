# 冲突处理 Playbook 与决策依据

本文档记录大版本同步的冲突分层策略的完整依据，以及 C 层每个文件的具体处理方法。数据来自 2026-07 的实测（基于 alauda main @ b3425621，上游 release-1.29 / release-1.30 / main）。

## 为什么合并 release-1.XX 而不是 main

| 合并目标 | 冲突文件总数 | 其中 resources/ | 真实冲突（非 resources/） |
|---|---|---|---|
| upstream/main | 2318 | 2284 | 34 |
| upstream/release-1.30 | 1690 | 1657 | 33 |
| upstream/release-1.29 | 1145 | 1113 | 32 |

结论：

1. **冲突量与分支选择基本无关。** 真实冲突两者只差 1 个文件（一个测试文件），其余全是会被 `make gen` 重建的 `resources/`。"合 release 分支冲突更多"的担忧不成立。
2. **main 的真正问题是内容，不是冲突。** 实测时 upstream main 领先 release-1.30 分叉点 116 个提交，全部是未发布的下一版本开发代码；而 release-1.30 分支是 1.30.3 已发布代码加持续的 backport。产品版本应该基于已发布代码。
3. **历史上合 main 能工作**，是因为恰好在上游刚发布后不久同步（此时 main ≈ release 分叉点）。这依赖时机，而且后续拿不到该版本的 patch 修复，只能像过去一样零散 cherry-pick CVE 修复（`hotfix-2.1/*` 分支就是这么来的）。合 release 分支后，未来的小版本同步只需再次 `git merge upstream/release-1.XX` 拿到新 backport，干净得多。
4. **版本矩阵保证。** alauda 需要 operator 同时支持两个 istio 大版本（如 1.28 + 1.30）。istio 一年发 4 个大版本、alauda 一年发 3 个 mesh 版本，所以会不时跳过一个 istio 大版本——两个维护版本不保证连续，也不保证是偶数；但间距至多为 2，release-1.XX 的 operator 按上游 n-2 政策固定支持 1.XX 与其前两个大版本，恰好覆盖。而 main 会随着下一版本开发逐步丢弃老版本支持。

### 连续合并 release 分支的已知代价（可接受）

上游 release 分支带有版本专属内容（docs、tests/e2e 样例、versions.yaml、go.mod 里的版本号戳）。本次合并 release-1.30 后，下次合并 release-1.32 时这些文件会因"两侧版本戳不同"额外产生约 40 个冲突（实测链式合并为 ~70 个非 resources 冲突 vs 直接合并 ~33 个）。这些文件 alauda 全部不定制，脚本的 B 层策略直接取上游侧，机械解决，不增加人工工作量。

## 三层策略详表

### A 层：重新生成物 —— 取上游侧占位，`make gen` 覆盖

| 路径 | 由哪个目标重新生成 |
|---|---|
| `resources/` | `download-istio-charts`（按 alauda-versions.yaml 下载）+ `remove-old-versions.sh`（删除不在矩阵中的目录）。解决冲突后清掉版本目录再 gen 最干净，但**只能删 `resources/v*` 目录**——release-1.30 起 `resources/resources.go`（`//go:embed all:v*` 包，被 cmd/main.go import）是被跟踪的源码，整目录清空会导致编译失败（regen.sh 已按此实现） |
| `api/`（`*_types.go`、`values_types.gen.go`） | `gen-api`（api_transformer 从 istio.io/istio 依赖生成）+ `update-version-list.sh`（版本枚举注解）。注意：当前 alauda 对 api/ 的历史改动全部来自上游 cherry-pick，可放心取上游；若未来 alauda 加了自有 API 字段，该文件要升级为 C 层人工处理 |
| `bundle/`、`bundle.Dockerfile` | `bundle`（`operator-sdk generate bundle --overwrite`，channels 来自 Makefile.vendor.mk 的 `CHANNELS`，operator-sdk 版本标签来自 Makefile.core.mk 的 `OPERATOR_SDK_VERSION`，都不用手改 bundle.Dockerfile 本身）。注意：上游命名的 `bundle/manifests/sailoperator.clusterserviceversion.yaml` 会作为「取上游侧」的残留留下（alauda 的 operator 名是 servicemesh-operator2，`make gen` 只重写 alauda 命名的文件），regen.sh 已自动 `git rm` 清理 |
| `chart/crds/` | `gen-manifests`（controller-gen）+ `extract-istio-crds.sh`（从 go.mod 的 istio 依赖提取 Istio CRD——所以 go.mod 对齐要在 gen 之前做） |
| `chart/Chart.yaml` | `operator-chart` / `alauda-update-values`（sed 回写 version/appVersion 为 Makefile.vendor.mk 的 VERSION） |
| `docs/api-reference/sailoperator.io.md` | `gen-api-docs`（crd-ref-docs） |
| `licenses/` | `mirror-licenses`（license-lint --mirror） |
| `PROJECT` | `operator-name`（sed projectName） |

### B 层：上游独有、alauda 从不定制 —— 直接取上游侧

`docs/`（api-reference 之外）、`tests/e2e/`、`common/`、`hack/`、`tools/`、`.devcontainer/`、`pkg/istioversion/versions.yaml`（alauda 构建实际使用 alauda-versions.yaml，上游矩阵文件保持上游原样即可）、`go.sum`（随后的 `go mod tidy` 会修正）。

### C 层：alauda 定制文件 —— 人工合并

原则：**以上游为新基底，重放 alauda 自有改动**。先用 `git log --oneline <merge-base>..HEAD -- <file>` 看 alauda 侧改动来源：

- 来自上游 cherry-pick（commit message 带上游 PR 号，如 `(#1596)`）→ 上游 release 分支已包含正式版本，**取上游侧**。
- alauda 自有改动 → 保留，叠加到上游新版本上。

| 文件 | alauda 自有改动（截至 2026-07，1.30 同步后更新） | 处理 |
|---|---|---|
| `Makefile.core.mk` | 新增 `alauda-update-values` 目标、`GENERATE_RELATED_IMAGES` 分支逻辑 | 取上游新内容（VERSION、工具版本等），保留这两处 alauda 追加。1.30 实测冲突仅 VERSION 一行 |
| `chart/values.yaml` | operator 镜像地址、版本行由 `alauda-update-values` sed；`deployment.annotations`（矩阵内 alauda 镜像）与 `csv.longDescription` 版本列表为手动维护 | 先取上游，gen 后按 SKILL.md 步骤 4 明细回补（annotations 必须裁剪，否则上游键经 helm 深合并漏进 CSV） |
| `chart/templates/olm/clusterserviceversion.yaml` 等 `chart/templates/` | olm 模板唯一 alauda 改动：`provider.name` 模板化（值在 alauda/values.yaml）；`rbac/role.yaml` 历史改动来自上游 cherry-pick #1477 | role.yaml 取上游；olm 模板保留 alauda 改动（1.30 实测 git 自动合并已正确保留，无冲突） |
| `chart/samples/` | 样例里的 istio 版本号 | 取上游后把版本号对齐 alauda 默认版本。当 alauda 新构建基于上游 release 分支最新 patch 时（常态），上游侧版本号即目标值，直接取上游零修改 |
| `controllers/`、`pkg/` 下 .go | **1.30 起 FIPS（#1596/#1682）与 vendor_defaults 机制均已被上游收编**：上游有 `pkg/istiovalues/vendor_defaults.go/.yaml/_test.go`，且 `pkg/reconcile/cni.go`、`pkg/revision/values.go` 自带调用。alauda 仅剩差异：`vendor_defaults.go` 的 `USE_VENDOR_DEFAULTS` 环境变量开关 + `vendor_defaults.yaml` 的版本数据块 | controller/reconcile 文件直接取上游；核对 `USE_VENDOR_DEFAULTS` 开关与 yaml 数据块保留（自动合并通常正确）。**警惕 fips.go/fips_test.go 型语义冲突**（见下节） |
| `tests/integration/` | 无 alauda 自有改动残留（vendor defaults 开关生效于 `.github/workflows/integration-tests.yaml` 的 `USE_VENDOR_DEFAULTS: "false"`，不在测试代码里） | 直接取上游；冲突多为 import 排布 + 上游新增用例 |
| `.github/workflows/` | alauda 自有 CI（`alauda-release.yaml` 等）+ 对上游 workflow 的修补（去重、action 升级） | alauda 独有文件保留；上游文件取上游后重放 alauda 修补 |
| `go.mod` / `go.sum` | fork 可能带主动安全升级（CVE 修复提升库版本等） | **基线取上游 release 分支；fork 的安全升级允许在基线上偏离**，但每行差异必须有意为之。go.mod 是 git 自动合并的重灾区：双侧各改过不同依赖行时 git 静默合出"混血"版本、旧版本行不带冲突标记——verify.sh 8a 会列出与快照的全部差异行（WARN），逐行确认"这是我们的安全升级"还是"合并混血残留"，混血残留对齐上游后 `go mod tidy`。istio.io 系依赖仍须匹配矩阵最新版本（影响 CRD 生成，verify 第 6 项）。licenses/ 随 go.mod 镜像（verify.sh 8b）：go.mod 有意差异时重跑 mirror-licenses。reviewer 拿 upstream main 对比会看到"很多库没升级"——那是 main 在为下一版本升依赖，正常，比较基准永远是 release-1.XX 快照 |
| `Makefile.vendor.mk`、`alauda/values.yaml`、`pkg/istioversion/alauda-versions.yaml`、`pkg/istiovalues/vendor_defaults.yaml`、`Dockerfile.alauda`、`hack/alauda-patch-csv.sh` | alauda 独有文件，上游没有，不会冲突 | 按 SKILL.md 第 3 步更新内容即可 |

### 语义冲突：git 自动合并成功 ≠ 编译通过（1.30 实测）

上游收编 alauda 的 cherry-pick 并重构签名/位置时，git 会把双侧改动拼在一起而不报冲突：

- `pkg/istiovalues/fips.go`：上游新增 `ApplyZTunnelFipsValues(*v1.ZTunnelValues)`，alauda 旧版 `ApplyZTunnelFipsValues(helm.Values)` 被拼在文件尾 → 同名重复定义，编译失败；`fips_test.go` 同样残留旧签名测试。解法：功能已被上游收编（等价），两文件直接对齐上游。
- resolve-merge-conflicts.sh 末尾的"语义冲突哨兵"会列出双侧改动且含 Fips/VendorDefaults/multus 关键词的 go 文件；merge commit 前必须 `go build ./... && go vet ./...`。

## 构建环境自适应（容器 / 本地工具链）的依据

- 仓库默认 `BUILD_WITH_CONTAINER=1`（`Makefile.overrides.mk`），所有 make 目标经 `common/scripts/run.sh` 进 build-tools 容器执行——这是容器依赖的唯一来源，`make gen`/`make alauda-update-values` 的实际工作本身不需要容器。
- 因此 `common.sh` 的 `detect_build_env` 按当前执行环境自适应：有可用的容器工具（docker/podman/nerdctl，CLI 存在且 `info` 成功）就用容器模式即 make 默认行为；没有则 `BUILD_WITH_CONTAINER=0` 直接本地执行 `Makefile.core.mk`。实测本地模式完整 `make gen`（含 chart 下载、controller-gen、crd-ref-docs、license-lint、operator-sdk generate bundle + validate）全部通过，且生成结果与已提交内容逐字节一致。
- 本地模式下缺失且 Makefile 不会自动安装的只有两个工具（build-tools 镜像内置所以上游没写引导）：`yq`（`download-istio-charts`、`bundle` 使用）和 `license-lint`（`mirror-licenses` 使用），`detect_build_env` 会自动 `go install` 进 `./bin`；`helm`、`operator-sdk`、`controller-gen` 等其余工具由 Makefile 自动下载。
- 任何模式下都需要容器的目标（`docker-build`、`bundle-build`、`alauda-docker-buildx` 等镜像构建）不属于本地同步流程，由 Alauda Release 流水线（CI）负责。
