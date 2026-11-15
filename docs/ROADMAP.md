# `news_panel` 演进路线图（ROADMAP）

> **当前阶段**：**Phase 1（v0.1 纵向切片开发 —— Gate 0 已于 2026-11-15 批准）**  
> **文档索引**：技术选型结论见 [DECISIONS.md](./DECISIONS.md)，v0.1 最小插件契约见 [PLUGIN_SPEC.md](./PLUGIN_SPEC.md)，跨工具任务见 [tasks/INDEX.md](../tasks/INDEX.md)。

---

## 0. 全局治理红线（Anti-Drift & Multi-Agent Rules）

在任何阶段开启开发或委派多 Agent 并行工作前，必须核对以下三条红线：

1. **Demo-First / Vertical Slice（演示优先 / 纵向切片）**
   - v0.1 必须以最快路径打通端到端最小闭环：`1 个 X 信源 -> 1 个最小插件契约 -> 1 个 AI 过滤器 -> 在面板中可见`。
   - **禁止**在首个切片跑通前过度设计插件 SDK、分布式调度总线或通用配置平台。
2. **Human-Gated Checkpoints（人工审批门禁）**
   - 每个关键节点均设 **Human Gate**：**Gate 0** 已批准（2026-11-15）；**Gate 1** 须由用户验收 v0.1 后方可冻结完整 v1 契约并并行扩展 Phase 2。
   - **跨工具任务拆分策略**：核心底座重构与边界清晰的独立外置任务可基于 `Owned Files` 安全并行——尤其 **ZCode 额度充足且偏向拆分简单独立任务**（例如 `tasks/P1-02-x-source-plugin.md`）。
3. **Explicit Non-Goals 边界守卫（Phase 0/1）**
   - 严禁引入：多租户 SaaS、复杂 RBAC/Auth、原生移动端 App、内置插件市场 UI、重型分布式基建（Kafka / ELK/ JVM 等）。

---

## 1. Phase 0：需求对齐与底座选型（✅ 已完成 — 2026-11-15）

### 目标
完成产品边界对齐、开源面板底座调研、轻量 AI 处理管线与 X 采集可行性调研，确定不踩协议坑、适合小内存 VPS 的独立演进起点。

### 关键交付物
- `docs/PRD.md`、`docs/ROADMAP.md`、`docs/DECISIONS.md`、`docs/HANDOFF.md`、`docs/PLUGIN_SPEC.md`
- 跨工具/跨额度协作入口与交接骨架：`AGENTS.md`、`tasks/README.md`、`tasks/_TEMPLATE.md`、`tasks/INDEX.md`
- Phase 0 技术调研汇总（开源底座对比、AI 过滤管线思路、2026 X 纯 HTTP Cookie 采集方案对比）

### 🚦 Gate 0（✅ 已由用户批准 — 2026-11-15）
用户已拍板锁定 `DEC-001`–`DEC-005`：
1. 底座：`0x2E/fusion`（MIT）深度重构 + `newsnow`/`glance` UI 思路借鉴；
2. 核心栈：Go + `modernc.org/sqlite` + 内嵌 React 19 SPA；
3. 插件架构：内置 + 外置混合双层插件（v0.1 冻结 `/api/v1/ingest` 与 `/api/v1/heartbeat` 契约）；
4. AI 管线：核心内置 L1 规则粗筛 + L2 云端 OpenAI-compatible LLM 评分/打标/摘要；
5. v0.1 X 采集：`plugins/x-twifork/`（Python + `twifork[impersonate]` + 备用号 Cookie，抗 Cloudflare 优先）。

---

## 2. Phase 1（v0.1）：X 端到端纵向切片 Demo（🚧 当前阶段）

### 目标
用最小工程复杂度在微型 Linux VPS（Docker 环境）上验证 `news_panel` 的核心假设：**AI 前置过滤能否显著提升高噪社交信息流（X）的信噪比**。

### 核心范围（Vertical Slice）
```text
[plugins/x-twifork (twifork+Cookie)] --(POST /api/v1/ingest & heartbeat)--> [Go 核心 L1+L2 AI 管线] --> [SQLite + React 19 面板]
```
- **Source 层（`P1-02`，通道 A / ZCode）**：独立外置插件 `plugins/x-twifork/`（`twifork[impersonate]` + 备用号 Cookie、抗 Cloudflare 拟人抖动调度、`state.json` 增量断点、`--dry-run` 独立自测、可选 AI 规划降级静态配置）。
- **契约层**：严格落地 [docs/PLUGIN_SPEC.md](./PLUGIN_SPEC.md) 的 `POST /api/v1/ingest`、`POST /api/v1/heartbeat` 与 `items` AI 扩展字段。
- **Core + Processor + Panel 层（`P1-01` 及可拆分简单子任务）**：改造 `0x2E/fusion` 底座，接入 `/api/v1` 入站接口、内置 L1 规则 + L2 OpenAI-compatible LLM 过滤评分管线，并在 React 19 面板展示评分/标签/摘要/优先级与基础已读状态。

### 🚦 Gate 1（人工拍板门禁）
并行扩展多信源与多处理插件前，用户必须验收并拍板：
1. v0.1 端到端真实数据流与 AI 过滤质量达到预期；
2. 基于 v0.1 实战反馈，正式拍板冻结**通用双层插件契约规范（Plugin Contract v1）**与深度重构计划（防止成为原底座浅层套壳）。

---

## 3. Phase 2（v0.2）：双层插件化完善与多信源扩展

### 目标
在已验证的最小核心与冻结的插件契约之上，借助多 Agent 并行能力扩展异构信源与处理插件，实现多台小 VPS 节点协同部署。

### 候选扩展范围（待 Gate 1 后排期确认）
- **Source Plugins 扩展**：
  - Telegram 群组监听器 `[TBD]`
  - 社区论坛监控（Linux.do / NodeSeek）`[TBD]`
  - QQ 群机器人采集端 `[TBD]`
- **Processing Plugins 扩展**：
  - 跨源语义去重插件 `[TBD]`
  - 分领域精细评分与打标插件（投资催化 / AI 泄露 / 薅羊毛）`[TBD]`
  - 优先级路由与高优通知插件 `[TBD]`
- **多节点与运维**：
  - 插件跨小规格 VPS 节点解耦部署验证（HTTP/Webhook 安全校验、Docker Compose / 单节点轻量部署规范 `[TBD]`）。

### 🚦 Gate 2（人工拍板门禁）
用户验收多信源混合信息流质量、容器资源占用（CPU/RAM）与插件可热插拔性。

---

## 4. Phase 3（v0.3+）：深度演进与个人情报工作台打磨

### 目标
进一步完成从"选型底座"向"独立项目 `news_panel`"的深度演进，打磨面向个人高频情报消费的面板交互与闭环反馈能力。

### 潜在演进方向（按需评估 `[TBD]`）
- 面向投资 / AI / 薅羊毛的分视图、多栏或快捷过滤工作台；
- 基于用户"标星/误判反馈"的轻量过滤 Prompt / 规则调优闭环；
- 核心代码库文档化与独立插件开发模板（供人类或 Agent 一键编写新插件）。
