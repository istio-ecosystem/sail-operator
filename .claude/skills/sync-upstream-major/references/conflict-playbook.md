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
4. **版本矩阵保证。** alauda 需要 operator 同时支持两个偶数大版本（如 1.28 + 1.30）。release-1.30 的 operator 按上游 n-2 政策固定支持 1.28~1.30；而 main 会随着下一版本开发逐步丢弃老版本支持。

### 连续合并 release 分支的已知代价（可接受）

上游 release 分支带有版本专属内容（docs、tests/e2e 样例、versions.yaml、go.mod 里的版本号戳）。本次合并 release-1.30 后，下次合并 release-1.32 时这些文件会因"两侧版本戳不同"额外产生约 40 个冲突（实测链式合并为 ~70 个非 resources 冲突 vs 直接合并 ~33 个）。这些文件 alauda 全部不定制，脚本的 B 层策略直接取上游侧，机械解决，不增加人工工作量。

## 三层策略详表

### A 层：重新生成物 —— 取上游侧占位，`make gen` 覆盖

| 路径 | 由哪个目标重新生成 |
|---|---|
| `resources/` | `download-istio-charts`（按 alauda-versions.yaml 下载）+ `remove-old-versions.sh`（删除不在矩阵中的目录）。解决冲突后直接 `rm -rf resources/*` 再 gen 最干净 |
| `api/`（`*_types.go`、`values_types.gen.go`） | `gen-api`（api_transformer 从 istio.io/istio 依赖生成）+ `update-version-list.sh`（版本枚举注解）。注意：当前 alauda 对 api/ 的历史改动全部来自上游 cherry-pick，可放心取上游；若未来 alauda 加了自有 API 字段，该文件要升级为 C 层人工处理 |
| `bundle/`、`bundle.Dockerfile` | `bundle`（`operator-sdk generate bundle --overwrite`，channels 来自 Makefile.vendor.mk 的 `CHANNELS`，operator-sdk 版本标签来自 Makefile.core.mk 的 `OPERATOR_SDK_VERSION`，都不用手改 bundle.Dockerfile 本身） |
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

| 文件 | alauda 自有改动（截至 2026-07） | 处理 |
|---|---|---|
| `Makefile.core.mk` | 新增 `alauda-update-values` 目标、`GENERATE_RELATED_IMAGES` 分支逻辑 | 取上游新内容（VERSION、工具版本等），保留这两处 alauda 追加 |
| `chart/values.yaml` | operator 镜像地址、版本行由 `alauda-update-values` sed；个别定制值 | 先取上游，gen 后过 diff 手动补回定制值 |
| `chart/templates/olm/clusterserviceversion.yaml` 等 `chart/templates/` | CSV 模板小改（如 2 行 alauda 调整）；`rbac/role.yaml` 历史改动来自上游 cherry-pick | role.yaml 取上游；olm 模板保留 alauda 改动 |
| `chart/samples/` | 样例里的 istio 版本号 | 取上游后把版本号对齐 alauda 默认版本 |
| `controllers/`、`pkg/` 下 .go | 历史上均为上游 cherry-pick；alauda 自有逻辑集中在 vendor_defaults 机制（`pkg/istiovalues/`） | 逐文件核对来源，cherry-pick 的取上游 |
| `tests/integration/` | `skip using vendor defaults in integration tests`（alauda 自有）+ cherry-pick 带来的用例 | 保留 vendor-defaults 相关改动，其余取上游 |
| `.github/workflows/` | alauda 自有 CI（`alauda-release.yaml` 等）+ 对上游 workflow 的修补（去重、action 升级） | alauda 独有文件保留；上游文件取上游后重放 alauda 修补 |
| `go.mod` | 无自有依赖，但 istio.io/istio、istio.io/api 必须匹配 alauda 最新 istio 版本 | 取上游，校验 istio 依赖版本，`go mod tidy` |
| `Makefile.vendor.mk`、`alauda/values.yaml`、`pkg/istioversion/alauda-versions.yaml`、`pkg/istiovalues/vendor_defaults.yaml`、`Dockerfile.alauda` | alauda 独有文件，上游没有，不会冲突 | 按 SKILL.md 第 5 步更新内容即可 |

## devpod 无 docker 的完整依据

- devpod 内无 `docker`/`podman`，也没有 `/var/run/docker.sock`。
- 仓库默认 `BUILD_WITH_CONTAINER=1`（`Makefile.overrides.mk`），所有 make 目标经 `common/scripts/run.sh` 进 build-tools 容器执行——这是 docker 依赖的唯一来源，`make gen`/`make alauda-update-values` 的实际工作本身不需要 docker。
- `BUILD_WITH_CONTAINER=0` 时直接本地执行 `Makefile.core.mk`。实测完整 `make gen`（含 chart 下载、controller-gen、crd-ref-docs、license-lint、operator-sdk generate bundle + validate）在无 docker 的 devpod 中全部通过。
- 本地缺失且 Makefile 不会自动安装的只有两个工具（build-tools 镜像内置所以上游没写引导）：`yq`（`download-istio-charts`、`bundle` 使用）和 `license-lint`（`mirror-licenses` 使用），都用 `go install` 装进 `./bin` 即可。
- 仍然需要 docker 的目标（`docker-build`、`bundle-build`、`alauda-docker-buildx` 等镜像构建）不属于同步流程，交给 CI。
