---
name: fix-image-vulns
description: 修复 alauda-mesh/sail-operator 的 Alauda Release workflow 流水线构建的 servicemesh-operator2 镜像安全漏洞。输入一个流水线 run（ID/URL）或镜像地址，完成：调内网扫描服务扫描镜像并按修复责任分类、基于当前分支创建修复分支（go stdlib → pin/升级 alauda-release.yaml 的 GOTOOLCHAIN；go.mod 依赖 → 升级库版本）、本地构建验证、创建 PR、在修复分支上触发流水线构建新镜像并监控、回归扫描（最多 3 轮修复）；os 级漏洞只报告不修复；-bundle 镜像不在扫描范围。仅限用户显式通过 /fix-image-vulns 调用。
argument-hint: "[RUN_ID | run URL | 镜像地址]，例如: /fix-image-vulns 30415369542 或 /fix-image-vulns build-harbor.alauda.cn/asm/servicemesh-operator2:2.2.0-r20260729015404"
disable-model-invocation: true
---

# 修复 servicemesh-operator2 镜像漏洞

对 Alauda Release workflow 构建的 servicemesh-operator2 镜像做漏洞扫描并修复，直到镜像干净或达到轮次上限。
下文的 `$SKILL_DIR` 指本 skill 的根目录（即调用时提示的 Base directory）。

## 参数

- `$ARGUMENTS`：**一个**流水线 run（纯数字 ID 或 run URL，只接受 Alauda Release workflow 的成功 run）**或**一个镜像地址（如 `build-harbor.alauda.cn/asm/servicemesh-operator2:2.2.0-r20260729015404`）。
- 参数里可能混有给助手的备注文字，只把 run/镜像部分传给脚本，备注按用户附加要求执行。
- 参数为空时用 AskUserQuestion 向用户询问，不要自行猜测。

## 背景知识

**构建链与修复责任**：流水线在 self-hosted runner 上执行 `make alauda-docker-buildx` → `common/scripts/gobuild.sh` 直接用 runner 本机 go 编译二进制 → `Dockerfile.alauda` 纯 COPY 进静态基础镜像（无 builder 阶段、几乎无 os 包）。因此：

| 漏洞类别 | 来源 | 处理 |
| --- | --- | --- |
| GO_STDLIB | 编译用的 go 版本过低 | pin/升级 `alauda-release.yaml` 顶层 env 的 `GOTOOLCHAIN` |
| GO_MODULE | go.mod 依赖库 | `go get` 升级库版本 |
| OS_REPORT_ONLY | 基础镜像层 os 包 | **不修复**，如实报告 |

- **GOTOOLCHAIN 机制与适用性**：镜像内 stdlib 版本 = 构建时实际使用的 go 版本（扫描报告 `[stdlib] 构建 go X.Y.Z` 即它），由 runner 本机 go、workflow env 的 `GOTOOLCHAIN` pin（初始没有，首次修复为新增）、go.mod 的 go directive 共同决定。pin 后 go1.21+ 会按需自动下载指定版本，覆盖 runner 本机版本，这是修 stdlib 漏洞的标准方案（与 alauda-mesh/istio 仓库做法一致）。**每次修 stdlib 前先确认方案仍适用**：`alauda-release.yaml` 构建步骤仍是 `make alauda-docker-buildx`、`Dockerfile.alauda` 仍是纯 COPY（无 FROM golang 之类的 builder 阶段）。若构建链已变（如改为容器内编译），GOTOOLCHAIN env 不再决定编译版本——停下来分析新方案（如升级构建镜像）并与用户确认，不要机械套用。
- **GOTOOLCHAIN 版本选择**：优先当前构建 go 同 minor 线内满足修复候选的最新 patch（如 1.26.5 → 1.26.6）；patch 满足不了才跨 minor/大版本，此时**必须在最终报告中着重强调说明**。pin 必须 ≥ go.mod 的 go directive（脚本会校验）。
- go.mod 中 `replace istio.io/istio => github.com/alauda-mesh/istio <伪版本>` 用于消除 istio.io/istio 伪版本导致的 Trivy 误报（详见 go.mod 尾部注释）。若扫描结果出现 `istio.io/istio` 自身的老 CVE，先检查该 replace 是否还在，而不是去改依赖。
- **流水线是 workflow_dispatch**：PR 不会自动构建镜像。修复 push 后要用 `run-release.sh` 在修复分支上手动触发 Alauda Release workflow，release_version 用 `<VERSION>-r<UTC时间戳>`（时间戳保证 tag 唯一，不与手动执行的流水线冲突），其余参数默认值。
- gh 命令必须显式 `--repo alauda-mesh/sail-operator`（脚本已内置）；commit 一律 `git commit -s`（签名）且带说明（`-m "标题" -m "补充说明"`）；全程禁止 `git commit --amend`，一律新建 commit。
- **commit-check 只校验 PR 的首个 commit**（`.github/workflows/commit-validation.yaml`）：message 正则强制"标题 + 空行 + 正文"结构；signoff 正则在仓库 `.commit-check.yml`（已放宽支持中文署名），且配置文件必须存在于被校验的那个 commit 中才生效。因此**首个 commit 不合规时追加 commit 救不回来**，又禁止改写历史，唯一出路是从基分支重建分支（同日重建脚本会撞名，手动 `git checkout -b fix/cve-<日期>-2 <基分支>` 并同步改 `state.env` 的 `FIX_BRANCH`）、重做 commit、建新 PR 取代旧 PR（`gh pr close <旧号> --comment "被 #<新号> 取代：<原因>"`）。
- **需要改上游继承的文件**（`.commit-check.yml`、workflow、docs 等非 alauda 独有文件）时，先查上游 istio-ecosystem/sail-operator 是否已有同类修改（搜上游 PR/commit 历史）；有则 `git fetch https://github.com/istio-ecosystem/sail-operator.git refs/pull/<N>/head` 后 `git cherry-pick -x -s <sha>`，**不要自造 fork 本地变体**——相同语义、不同文本的改动会加重未来上游同步的冲突（用户反馈，2026-07-29 signoff 正则一例：fork 变体已 revert，换成 cherry-pick 上游 #2005）。
- 修复轮次上限 **3 轮**（首轮 + 回归后最多再修 2 次），修不完就如实汇报。
- 状态目录 `out/fix-image-vulns/`（gitignore 内），各脚本经 `state.env` 串联；修复在独立 worktree 中进行，不打扰主工作区当前检出。

## 步骤 1：漏洞检测

```bash
bash "$SKILL_DIR/scripts/resolve-input.sh" <RUN_ID|run URL|镜像地址>   # Bash timeout 120000
bash "$SKILL_DIR/scripts/scan-image.sh"                                # Bash timeout 600000（服务端拉镜像+扫描）
```

- resolve 会把当前检出分支记为基分支（修复分支从它切出、PR 以它为 base）。若提示 run 构建分支与当前分支不一致，先自行核实：`gh pr list --repo alauda-mesh/sail-operator --head <run分支> --base <当前分支> --state merged`——run 分支已合入当前分支（合并后源分支常被删除，这种"不一致"属正常时序）则当前分支就是正确基分支，直接继续；确实未合入才用 AskUserQuestion 和用户确认基分支，需要时检出正确分支后重跑。
- **无论有无漏洞，都先向用户输出扫描摘要**（漏洞明细、SUMMARY 分类计数、修复目标表）。然后按 `RESULT:` 分支：
  - **CLEAN**：无漏洞，汇报后直接结束；
  - **REPORT_ONLY**：剩余均为不修复项（os 级 / 无修复版本），列出明细并说明不修复的原因，结束；
  - **FIX_NEEDED**：执行步骤 2～6。

## 步骤 2：修复

```bash
bash "$SKILL_DIR/scripts/create-fix-branch.sh"    # 基于当前分支建 worktree；输出 WORKTREE= / BRANCH=
```

**go stdlib**（按背景知识先做适用性确认，再对照扫描输出选定版本）：

```bash
bash "$SKILL_DIR/scripts/update-gotoolchain.sh" <go1.X.Y>
```

按脚本 NOTICE 用 Edit 在 GOTOOLCHAIN 行上方补/改注释（写明本次 CVE 编号与选版缘由）。

**go.mod 依赖**（升级目标以扫描输出的"修复目标"表为准；候选没有 v 前缀，拼参数时要加上）：

```bash
bash "$SKILL_DIR/scripts/gomod-bump.sh" <module@vX.Y.Z> [...]   # timeout 600000
```

- 库之间有依赖约束，实际落位版本可能高于扫描给的修复候选，属正常，脚本会打印实际版本，最终以实际落位汇报；
- `go get` 报 `A@vX requires B@vY, not B@vZ`：把 B 的目标提到 vY 重跑（vY 更高，CVE 覆盖不受影响）；
- 同一发布系列的包（如 `go.opentelemetry.io/otel` 全家桶）版本要对齐，统一取其中最高者；
- 间接依赖解析崩时，优先参考上游 istio-ecosystem/sail-operator 或 alauda-mesh/istio 更高版本分支 go.mod 里的钉法，而不是自己试版本；
- 失败时分析原因（版本冲突、新版本要求更高 go、API 变更），能明确解决就解决，拿不准就带着报错向用户提问，不要凭猜测大版本连锁升级；
- 无修复版本的 CVE 升级修不了，记入最终汇报的"未修复项"。

**构建验证 + 提交**：

```bash
bash "$SKILL_DIR/scripts/verify-build.sh"    # timeout 600000；按 GOTOOLCHAIN pin 用与流水线一致的 go 编译
```

BUILD_OK 后在 worktree 内提交（两类修复各自独立 commit，`-s` 签名，禁止 amend）：

- go.mod：`fix: bump vulnerable go modules`
- GOTOOLCHAIN：`fix: pin GOTOOLCHAIN to go1.X.Y for <CVE编号>`

首个 commit 前先做一次预检（后续 commit 不用重复）：用 python3 的 `re` 验证 `Signed-off-by: <git user.name> <git user.email>` 能被 worktree 内 `.commit-check.yml` 的 commit_signoff 正则 `re.search` 命中、完整 commit 消息（标题+空行+正文）能被 message 正则 `re.match` 命中；不命中就停下按背景知识 commit-check 条目分析，不要硬提交。

## 步骤 3：创建 PR

PR 正文写进 scratchpad 临时文件：扫描摘要（镜像、分类计数）+ 修复清单（每项：模块/工具链、版本变化、覆盖的 CVE）+ 本地构建验证说明 + 不修复项说明（os 级 / 无修复版本）+ 结尾一行 `🤖 Generated with [Claude Code](https://claude.com/claude-code)`。然后：

```bash
bash "$SKILL_DIR/scripts/create-pr.sh" <正文文件>    # 输出 PR_NUMBER= / PR_URL=
```

幂等：修复分支已有 open PR 时复用，回归轮 push 新 commit 后重跑即可。

建 PR 后马上 `gh pr checks <PR号> --repo alauda-mesh/sail-operator` 复核：commit-check 约半分钟出结果，fail 就按背景知识 commit-check 条目处理（别等到最后才发现首 commit 救不回）；coverage 等分钟级检查不用等，最终汇报前再确认一眼全绿。

## 步骤 4：触发流水线

```bash
bash "$SKILL_DIR/scripts/run-release.sh"    # 触发 Alauda Release workflow 并定位 RUN_ID
```

## 步骤 5：监控流水线

多平台镜像构建从几分钟（runner 缓存热）到几十分钟不等，**必须后台运行**（Bash 的 `run_in_background: true`）：

```bash
bash "$SKILL_DIR/scripts/watch-run.sh"
```

- **SUCCESS（0）**：已收集新镜像并把轮次 +1，进入步骤 6；
- **PIPELINE_FAILED（2）**：脚本已附失败 step 与日志摘要。**分析失败原因**：是本次修复引入（依赖升级编译错、GOTOOLCHAIN 版本 runner 下载不到）还是环境问题（self-hosted runner、registry、代理）。修复方向拿不准时向用户提问，不要盲目改了就重推；修好后在原 worktree 追加 commit → create-pr.sh → run-release.sh → 重跑本脚本；
- **PIPELINE_TIMEOUT（3）**：告知用户流水线仍在运行，附 run 链接，稍后重跑本脚本继续等。

## 步骤 6：回归扫描与迭代

```bash
bash "$SKILL_DIR/scripts/scan-image.sh"    # ROUND 已 +1，自动扫流水线构建的新镜像
```

- **CLEAN / REPORT_ONLY**：修复完成。把回归结论回写 PR（`gh pr edit <PR号> --repo alauda-mesh/sail-operator --body-file <更新后正文>`），落定首轮正文里"以回归扫描结果为准"之类的悬项，再进入最终汇报；
- **FIX_NEEDED**：先分析为什么还有漏洞（上轮目标版本仍带 CVE？升级未生效？新版本引入新漏洞？），再回到步骤 2 继续修——不新建分支，在原 worktree 追加 commit → create-pr.sh → run-release.sh → 后台 watch-run.sh → 再扫描。

**最多 3 轮修复**。到限仍未清零时停止，如实汇报剩余漏洞、已尝试的措施和失败原因，让用户决策。

## 最终汇报

用清晰列表汇报：

1. 输入（run/镜像）与基分支、修复分支；
2. 首轮扫描摘要（总数、分类计数）→ 修复清单（模块/工具链、版本变化、覆盖的 CVE、commit）→ PR 链接（`gh pr checks` 确认检查状态并汇报）→ 流水线 run 链接与结果 → 回归扫描结论；
3. 剩余不修复项：os 级漏洞明细、无修复版本或修不掉的项及原因（如实报告，注明不在修复范围）；
4. 若 GOTOOLCHAIN 跨了 go 大版本，**着重强调**该变化及原因。

不要自行 merge PR，等用户 review。
