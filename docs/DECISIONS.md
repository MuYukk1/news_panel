# `news_panel` 架构与选型决策日志（DECISIONS）

> **用途**：记录已确认的架构约束与技术决策。  
> **Gate 状态**：**Gate 0 已于 2026-11-15 由用户正式批准**（`DEC-001`–`DEC-005` 已锁定为 Phase 1 执行基线）；后续 Phase 2 扩展选型仍按门禁规则保持 `[TBD]`。

---

## 1. 已确认原则与约束（Confirmed Constraints）

| 编号 | 决策主题 | 已确认结论 | 来源 / 备注 |
| :--- | :--- | :--- | :--- |
| **ADR-000** | 治理与开发节奏 | 采用 **Demo-First / Vertical Slice** 与 **Human-Gated Checkpoints**；Gate 0 通过后仅围绕 v0.1 纵向切片开发，Gate 1 前严禁提前铺开 Phase 2 多信源；Phase 0/1 严守 Explicit Non-Goals。 | Phase 0 初始对齐 / Gate 0 |
| **ADR-001** | 开源协议边界 | 优先选用 **MIT / Apache-2.0 / BSD** 等宽松开源协议底座；**避免 GPL / AGPL** 传染性协议限制项目独立演化。 | 用户核心要求 |
| **ADR-002** | 底座使用策略 | 不从零重复构建基础 UI 与底座，但**不做浅层二改套壳项目**；以成熟开源项目作深度重构底座（可借鉴其他项目优秀设计思路），打造独立项目。 | 用户核心要求 |
| **ADR-003** | 双层插件与解耦 | 采用**内置 + 外置混合双层插件架构**（Source + Processing）；外置插件语言无关，通过 HTTP/Webhook 解耦边界接入，支持跨多台小服务器运行。 | 用户核心要求 / Gate 0 |
| **ADR-004** | 部署与资源约束 | 24/7 Docker 部署于 Linux 云 VPS；单台 CPU/RAM 极小（多台微型 VPS 协同 > 单台大机器）；允许调用云端 API 或云端 CLI 执行抓取与 AI 推理。 | 用户核心要求 |
| **ADR-005** | v0.1 首选验证信源 | 首个端到端 Demo 目标锁定为 **X (Twitter)**，用于在高噪声环境下验证 AI 过滤价值。 | 用户核心要求 |

---

## 2. Gate 0 已拍板技术决策（Approved Decisions — 2026-11-15）

> [!NOTE]
> 以下 `DEC-001` 至 `DEC-005` 均已于 **2026-11-15** 经用户 **Gate 0 正式批准（`APPROVED`）**，作为 Phase 1（v0.1）开发基线，未经新一轮决策不得随意推翻。

### DEC-001：面板 UI 与开源参考底座选型
- **状态**：`APPROVED (2026-11-15)`
- **拍板结论**：以 **`0x2E/fusion`（MIT）** 作为 Go + React 19 + SQLite 深度重构起点；前端卡片/多栏情报视图交互借鉴 **`newsnext/newsnow`（MIT）** 与 **`glanceapp/glance`（⚠️ AGPL-3.0 仅作 UI/交互思路参考，严禁复用代码）**，并持续吸收高质量插件契约设计思路，演进为独立项目 `news_panel`。
- **核心理由**：`0x2E/fusion` 原生具备基于单条 `items` 持久化存储的收件箱模型（去重唯一索引、已读/未读、收藏、分组、搜索）且单容器仅 ~20–40 MB RAM，改造成本与小 VPS 资源适配度最优。

### DEC-002：核心编程语言、框架与持久化存储选型
- **状态**：`APPROVED (2026-11-15)`
- **拍板结论**：核心服务技术栈锁定为 **Go + `modernc.org/sqlite`（无 CGO 纯 Go SQLite）+ 内嵌 React 19 SPA**。
- **核心理由**：编译为单二进制/单容器部署（日常内存 ~20–40 MB），无 Postgres/Redis/Kafka/JVM 等重型运行时依赖，完美适配 1C1G–2C2G 微型 Linux VPS。

### DEC-003：双层插件通信协议与最小契约规范
- **状态**：`APPROVED (2026-11-15)`
- **拍板结论**：采用**内置 + 外置混合双层插件架构**：
  1. **内置插件**：核心服务内置轻量规则/默认 LLM 处理器以及后续通用内置拉取器（零跨进程序列化开销）；
  2. **外置插件**：语言无关，v0.1 通过统一 HTTP 接口接入——`POST /api/v1/ingest`（Bearer Token + `(source_id, external_id)` 幂等键）与 `POST /api/v1/heartbeat`（健康心跳），详细 JSON Schema 见 [PLUGIN_SPEC.md](./PLUGIN_SPEC.md)；
  3. **后续扩展**：Gate 1 验收 v0.1 闭环后再冻结完整 v1 规范并扩展外置 Processor Webhook 钩子。

### DEC-004：AI 处理管线架构与模型调用方式
- **状态**：`APPROVED (2026-11-15)`
- **拍板结论**：v0.1 AI 过滤管线采用核心内置轻量两段式流水线（参考 `umputun/newscope` 与 `t0saki/AI-News-Dashboard`）：
  - **L1 粗筛层**：核心内置轻量规则 / 关键词 / 幂等去重粗筛；
  - **L2 精筛层**：调用云端 **OpenAI-compatible LLM API** 完成相关性评分（`ai_score` 0–100）、标签化（`ai_tags`）、去噪分拣（`triage_status`）、优先级路由（`priority`）与一句话摘要（`ai_summary`）。

### DEC-005：v0.1 X (Twitter) 采集插件实现路径
- **状态**：`APPROVED (2026-11-15)`
- **拍板结论**：v0.1 外置 X 采集插件（位于 `plugins/x-twifork/`）采用 **纯 HTTP Python + `PawiX25/twifork[impersonate]`（基于 `curl_cffi`，MIT）+ 用户备用 X 账号 Cookie**。
- **硬性优先级与风控约束**：
  1. **首要优先级是抗 Cloudflare 拦截与低封号风控**；
  2. 必须采用低频随机抖动拉取、拟人化作息节奏、`state.json` 增量断点（`last_seen_id`）、支持代理配置；
  3. 支持可选的 AI 主动查询规划接口（异常或未配置时降级回静态 `config.yaml`）；
  4. **严禁**引入无头 Chromium / Playwright，且禁止在 VPS 上执行自动化密码登录 `login()`。

---

## 3. 后续阶段待拍板项（Gate 1+ Pending）

- **DEC-006（v1 完整插件规范与外置 Processor Webhook 协议）**：`[TBD - 待 Gate 1 验收后拍板]`
- **DEC-007（Phase 2 多信源插件具体选型：Telegram / Linux.do / NodeSeek / QQ 群）**：`[TBD - 待 Gate 1 验收后拍板]`
