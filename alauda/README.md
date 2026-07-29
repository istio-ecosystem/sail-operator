# Alauda Sail Operator 维护指南

alauda-mesh/sail-operator 的维护类 AI skill 索引。所有 skill 均为显式调用：在 Claude Code 会话中输入 `/skill名 参数`，模型不会自动触发。

## 版本升级

### 大版本同步（sync-upstream-major）

Skill: [.claude/skills/sync-upstream-major](../.claude/skills/sync-upstream-major/SKILL.md)

同步上游 istio-ecosystem/sail-operator 的 `release-1.XX` 分支到本仓库目标分支，完成 mesh 大版本升级（如 istio 1.28 → 1.30、mesh 2.1 → 2.2）：merge 上游并按三层策略解决冲突、更新 alauda 版本矩阵、重新生成、校验、创建 PR、触发并监控 Alauda Release 流水线。小版本 patch 同步不在其范围。

```text
/sync-upstream-major <上游release分支> <目标分支> <istio构建版本...>

# 示例
/sync-upstream-major release-1.30 main 1.30.3-asm-rc.4 1.28.6-asm-r4 1.28.3-asm-r3
```

### 小版本同步

TODO：小版本（patch）同步 skill 后续添加。

## 漏洞修复

### 镜像漏洞修复（fix-image-vulns）

Skill: [.claude/skills/fix-image-vulns](../.claude/skills/fix-image-vulns/SKILL.md)

扫描并修复 Alauda Release workflow 构建的 servicemesh-operator2 镜像安全漏洞：调内网扫描服务扫描并按修复责任分类 → go stdlib 漏洞 pin/升级 `alauda-release.yaml` 的 GOTOOLCHAIN、go.mod 依赖漏洞升级库版本 → 本地构建验证 → 创建 PR → 在修复分支上触发流水线构建新镜像 → 回归扫描（最多 3 轮修复）。os 级漏洞只报告不修复；`-bundle` 镜像不在扫描范围。

```text
/fix-image-vulns <RUN_ID | run URL | 镜像地址>

# 示例：输入 Alauda Release workflow 的 run
/fix-image-vulns 30415369542

# 示例：输入具体镜像
/fix-image-vulns build-harbor.alauda.cn/asm/servicemesh-operator2:2.2.0-r20260729015404
```
