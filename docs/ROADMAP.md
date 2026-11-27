# `news_panel` 演进路线图（ROADMAP）

> **当前阶段**：**Phase 2（v0.2 体验收口、三级处理管线与个性化闭环 —— Gate 1 已于 2026-11-20 批准）**  
> **文档索引**：技术选型结论见 [DECISIONS.md](./DECISIONS.md)，v1 冻结最小插件契约见 [PLUGIN_SPEC.md](./PLUGIN_SPEC.md)，跨工具任务见 [tasks/INDEX.md](../tasks/INDEX.md)。

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

## 2. Phase 1（v0.1）：X 端到端纵向切片 Demo（✅ 已完成 — 2026-11-20）

### 目标
用最小工程复杂度在微型 Linux VPS（Docker 环境）上验证 `news_panel` 的核心假设：**AI 前置过滤能否显著提升高噪社交信息流（X）的信噪比**。

### 核心范围（Vertical Slice）
```text
[plugins/x-twifork (twifork+Cookie)] --(POST /api/v1/ingest & heartbeat)--> [Go 核心 L1+L2 AI 管线] --> [SQLite + React 19 面板]
```
- **Source 层（`P1-02` + 账号锁定/封禁静默熔断修复 `729e9bc`）**：独立外置插件 `plugins/x-twifork/`。
- **Core + Processor + Panel 层（`P1-01` / `P1-03` / `P1-04`）**：Go + SQLite + 内嵌 React 19 浅色磨砂液态玻璃工作台（含双时间显示），已完成云端 Docker Compose + Caddy HTTPS 实测上线。

### 🚦 Gate 1（✅ 已由用户批准 — 2026-11-20）
用户已完成 v0.1 实战验收并拍板锁定 `DEC-006`、`DEC-007`：
1. 冻结现行 `/api/v1/ingest` + `/api/v1/heartbeat` 为 v1 稳定契约，当前暂不加外置 Processor Webhook 钩子；
2. Phase 2 优先收口 UI/RSS 体验痛点（`P2-01`），核心投入「三级 AI 处理管线（`P2-02`）+ 用户反馈与有界个性化档案闭环（`P2-03`）」，新信源聚焦单一 Telegram 插件（`P2-04`）。

---

## 3. Phase 2（v0.2）：体验收口、三级处理管线、个性化档案与 Telegram 接入（🚧 当前阶段）

### 目标
修复 v0.1 实战暴露的 UI 与 RSS 兼容痛点，构建低开销高精度的三级递进 AI 分拣管线与用户个性化反馈闭环，并验证冻结 v1 契约接入第二个外置信源（Telegram）。

### 计划任务拆解
- **`P2-01`（体验与 RSS 兼容收口 — ✅ `DONE`）**：
  - 修复阅读抽屉打开/关闭卡顿与关闭先闪白问题；
  - 原生 RSS 未打分条目归入独立「未打分」分桶（方案 A：默认不走 AI，预留/落地单源 `ai_enabled` 开关）；
  - 订阅源与外置插件液态玻璃启停开关；
  - 高级筛选增加「按来源筛选」；
  - 多源探测与 OPML 导入升级为多选批量导入（支持直接粘贴 GitHub `.opml` 链接）。
- **`P2-02`（三级 AI 分拣管线底座 — ✅ `DONE`）**：L1 程序规则拦截 → L2 云端轻量 Decision 门控（可插拔 HTTP Adapter，支持显式关闭/自动跳过）→ L3 主模型评分/摘要/复核，打通 RSS `ai_enabled` 闭环与 `/api/v1/ai/stats`。
- **`P2-02a`（单源刷新 30s 超时热修 — ✅ `DONE`）**：解耦 RSS HTTP 抓取超时（`PullTimeout` 30s）与 AI 分拣超时（单源后台刷新放宽至 15 分钟并增加同源并发刷新去重守卫），解决 L3 CLI 冷却期下手动/首次刷新被 30s 截断标为 `failed` 的线上缺陷。
- **`P2-02b`（L2 原生多厂商 Decisions API 适配器与轻量 AI 设置面板 — ✅ `DONE`，落实 `DEC-008`）**：
  - 基于纯 Go 标准库双 Dialect 统一适配 `gemini_decision`（`gemini-decision-1`）、`openai_decision`（`gpt-6-luna`）、`typesafe`（`jev-latest`）、`cloudflare_clef`（`@cf/cloudflare/clef-flash`，支持 Cloudflare Account ID），兼容既有 `openai` / `http_json` / `off`；
  - 新增 `ai_settings` 单行表与独立 `/api/v1/ai/settings`（写密钥不回传明文、脱敏展示、SSRF 与防重定向保护）及 `POST /api/v1/ai/settings/test` 一键连通性测试；
  - `Evaluator` / `Puller` 零重启热更新（`atomic.Pointer` / `atomic.Int64`），并在 `SettingsDialog` 新增轻量「AI 分拣」Tab（不触碰插件市场 UI 等 Non-Goals）。
- **`P2-03`（用户反馈与有界个性化档案闭环）**：前端修改优先级 + 可选文字反馈 → SQLite 反馈表（关联轻量 `user_id`）→ 空闲窗口周期性压缩归纳为有长度上限的结构化偏好档案（支持查看/手工微调/回滚）→ 注入 `P2-02b` 统一 L2 多问构造器与 L3 判定。
- **`P2-04`（外置 Telegram 信源插件 — `DEC-009` 拍板后可与 `P2-02b`/`P2-03` 并行）**：兼容公开频道与个人号监听私有群，复用 v1 `/ingest` + `/heartbeat` 契约，文件边界完全独立（`plugins/telegram/**`）。

### 🚦 Gate 2（人工拍板门禁）
用户验收三级管线降本与准确度、个性化档案演进效果、Telegram 实时接入表现与容器内存占用后，方可进入 Phase 3（Linux.do / NodeSeek / QQ 群与外部高优通知等）。

---

## 4. Phase 3（v0.3+）：深度演进与个人情报工作台打磨

### 目标
进一步完成从"选型底座"向"独立项目 `news_panel`"的深度演进，打磨面向个人高频情报消费的面板交互与闭环反馈能力。

### 潜在演进方向（按需评估 `[TBD]`）
- 面向投资 / AI / 薅羊毛的分视图、多栏或快捷过滤工作台；
- 基于用户"标星/误判反馈"的轻量过滤 Prompt / 规则调优闭环；
- 核心代码库文档化与独立插件开发模板（供人类或 Agent 一键编写新插件）。
