---
name: sync-upstream-major
description: Use when syncing or merging upstream istio-ecosystem/sail-operator into alauda-mesh/sail-operator for a major version upgrade — 当用户提到「同步上游 / 合并上游 / merge upstream / upstream sync」「大版本同步 / 大版本升级」「添加 istio 1.3x」「mesh 2.x 升级」，或需要处理与上游合并产生的大量冲突时使用。仅适用于大版本同步（例如 istio 1.28 → 1.30、mesh 2.1 → 2.2）；小版本（patch）同步不在本 skill 范围内。
---

# 上游大版本同步（istio-ecosystem/sail-operator → alauda-mesh/sail-operator）

## 概述

把上游 sail-operator 的一个新大版本合并进 alauda fork：新增一个 istio 大版本、淘汰最老的大版本、发布新的 mesh 2.X。alauda 只维护两个偶数 istio 大版本（如 2.1 = 1.26 + 1.28，2.2 = 1.28 + 1.30）。

三条核心原则（依据见 `references/conflict-playbook.md` 中的实测数据）：

1. **合并上游 `release-1.XX` 分支，而不是 `main`。** main 含大量未发布的下一版本开发代码（实测领先 release 分叉点 100+ 提交）；release 分支是已发布代码加 backport 修复，且后续小版本同步只需再次合并同一分支。真实冲突数两者几乎相同（实测 33 vs 34 个文件），合 main 并不会更省事。
2. **冲突用三层路径策略机械化解决，生成物一律重新生成。** 实测 98% 的冲突在 `resources/` 等生成目录里，逐个手解是浪费。先跑 `scripts/resolve-merge-conflicts.sh`，剩下需要人工判断的通常只有约 10 个 alauda 定制文件。
3. **devpod 无 docker 也能完成全流程。** `make` 默认在 build-tools 容器里执行（`BUILD_WITH_CONTAINER=1`），但本仓库所有生成目标都支持 `BUILD_WITH_CONTAINER=0` 本地工具链，只缺 `yq` 和 `license-lint` 两个外部工具，均可 `go install`（已实测验证完整 `make gen` 通过）。

## 前置条件

- git remote `upstream` 指向 `istio-ecosystem/sail-operator`（本仓库已配置）。
- 新 istio 版本的 alauda 构建产物已就绪：`-asm-` 后缀的 helm charts（r2.dev URL，来自 alauda-mesh/istio 构建）和镜像（`build-harbor.alauda.cn/asm/`）。用 `curl -sfI <chart-url>` 逐个验证，404 则先去推动 alauda-mesh/istio 构建，不要继续。
- 本地 go 1.24+。无 docker 时先做一次工具准备：

```bash
export BUILD_WITH_CONTAINER=0
GOBIN=$PWD/bin go install github.com/mikefarah/yq/v4@latest
GOBIN=$PWD/bin go install istio.io/tools/cmd/license-lint@latest
export PATH=$PWD/bin:$PATH
```

`helm`、`operator-sdk`、`controller-gen` 等其余工具由 Makefile 自动下载到 `./bin`，无需手动安装。

## 流程

### 1. 确定目标版本

```bash
git fetch upstream --tags
git branch -r | grep 'upstream/release-'   # 找到要新增的 istio 大版本对应的 release 分支
```

同步目标 = 要新增的 istio 大版本对应的 `upstream/release-1.XX`（该分支的 operator 支持 n-2 版本矩阵，恰好覆盖 alauda 需要的两个偶数版本）。确定新的 mesh 版本号 `2.X.0` 和 channel `stable-2.X`。

### 2. 创建同步分支

```bash
git checkout main && git pull
git checkout -b sync/$(date +%Y-%m-%d)   # 旧命名 chore/upstream-main-n 已废弃
```

### 3. 合并上游

```bash
git merge upstream/release-1.XX
```

会产生 1500~2500 个冲突，属正常情况，绝大多数下一步机械解决。

### 4. 解决冲突（三层策略）

```bash
bash .claude/skills/sync-upstream-major/scripts/resolve-merge-conflicts.sh
```

脚本自动处理前两层，并列出剩余需人工处理的文件：

- **A 层（重新生成物）→ 先取上游侧占位，第 6 步 `make gen` 会全部覆盖：** `resources/`、`api/`、`bundle/`、`bundle.Dockerfile`、`chart/crds/`、`docs/api-reference/`、`licenses/`、`PROJECT`。
- **B 层（上游独有、alauda 从不定制）→ 直接取上游侧：** `docs/`、`tests/e2e/`、`common/`、`hack/`、`tools/`、`.devcontainer/`、`pkg/istioversion/versions.yaml`、`go.sum`。
- **C 层（alauda 定制文件）→ 人工合并，原则是"以上游为新基底，重放 alauda 自有改动"：** 通常约 10 个文件（`Makefile.core.mk`、`chart/values.yaml`、`chart/templates/`、`chart/samples/`、`controllers/`、`tests/integration/`、`.github/workflows/`、`go.mod` 等）。每个文件的具体处理原则见 `references/conflict-playbook.md`。

判断 C 层冲突时先查来源：alauda 侧改动如果是从上游 cherry-pick 来的（commit message 带上游 PR 号），上游 release 分支里已有正式版本，直接取上游侧；只有 alauda 自有改动（FIPS、multus、资源限额、servicemesh-operator2 命名、CI 定制等）才需要保留重放。

### 5. 更新 alauda 版本信息（只有 4 个文件需要手改）

| 文件 | 改什么 |
|---|---|
| `Makefile.vendor.mk` | `VERSION = 2.X.0`；`CHANNELS = "stable,stable-2.X"`（bundle.Dockerfile 和 CSV 的 channel 由此生成，不用手改它们） |
| `pkg/istioversion/alauda-versions.yaml` | 新增大版本条目：`vX.YY-latest`（ref 指向具体版本）+ 具体小版本（`version`/`repo`/`commit` + r2.dev 的 `-asm-` charts URL）；被淘汰的大版本设 `eol: true` 并只保留 `name`/`eol`/`ref` 字段 |
| `pkg/istiovalues/vendor_defaults.yaml` | 为每个新增 istio 版本添加默认值块（复制上一版本的块作为基础，按需调整） |
| `go.mod` | `istio.io/istio`、`istio.io/api` 必须与 alauda-versions.yaml 中最新 istio 版本一致（合并上游后通常已正确，校验即可），然后 `go mod tidy` |

### 6. 重新生成

```bash
export BUILD_WITH_CONTAINER=0 PATH=$PWD/bin:$PATH
rm -rf resources/*        # 清掉合并残留，按 alauda-versions.yaml 重新下载
make gen                  # 自动使用 alauda-versions.yaml / alauda/values.yaml（Makefile.vendor.mk 注入）
make alauda-update-values # 刷新 chart/values.yaml 版本行和 alauda/values.yaml 镜像 annotation
git add -A
```

### 7. 校验

- `alauda/values.yaml`：每个非 eol 版本都有 `images.vX_Y_Z.{istiod,proxy,cni,ztunnel}` annotation，镜像为 `build-harbor.alauda.cn/asm/...:X.Y.Z-asm-rN`。
- `bundle.Dockerfile` / `bundle/metadata/annotations.yaml`：channels 为 `stable,stable-2.X`。
- `bundle/manifests/servicemesh-operator2.clusterserviceversion.yaml`：版本号、镜像正确。
- `resources/` 只包含 alauda-versions.yaml 中非 eol 版本的目录。
- `chart/values.yaml` 逐段过一遍 diff——个别 alauda 定制值可能被上游改动或重新生成覆盖，需要手动补回。
- `make lint && make test`。

### 8. 提交与 PR

```bash
git add -A && git commit -s -m "chore: merge upstream release-1.XX and add istio 1.XX" -m "..."
```

版本信息更新、生成物刷新建议拆成后续独立提交（同样 `-s` 签名）。禁止 amend 已有提交，永远追加新提交。push 后向 `main` 发 PR。

## 常见错误

| 错误 | 后果 / 纠正 |
|---|---|
| 合并 `upstream/main` | 带入未发布的下一版本开发代码；改合 `release-1.XX` |
| 手工逐个解决 `resources/`、`api/`、`bundle/` 冲突 | 纯浪费，`make gen` 会全部覆盖；跑脚本机械处理 |
| 在容器模式跑 `make gen`（devpod 无 docker 会报 docker: command not found） | `export BUILD_WITH_CONTAINER=0` 并预装 yq / license-lint |
| 忘记更新 `vendor_defaults.yaml` | 新 istio 版本没有 alauda 默认值（multus、资源限额、FIPS），运行时行为错误 |
| 忘记 `go.mod` 的 istio 依赖对齐 | 生成的 CRD schema（含 Istio CRD）停留在旧版本 |
| 忘记跑 `make alauda-update-values` | `alauda/values.yaml` 缺新版本镜像 annotation，CSV relatedImages 不全 |
| 直接把 C 层冲突全取某一侧 | 要么丢 alauda 定制（FIPS/multus/CI），要么丢上游修复；逐文件按 playbook 处理 |
| amend 已推送的同步提交 | 团队规则禁止 amend，创建新提交 |
