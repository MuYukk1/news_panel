# `news_panel` 架构与选型决策日志（DECISIONS）

> **用途**：记录已确认的架构约束与技术决策。  
> **Gate 状态**：**Gate 0（2026-11-15）与 Gate 1（2026-11-20）均已由用户正式批准**（`DEC-001`–`DEC-007` 已锁定为 Phase 2 执行基线）；未拍板的细分供应商/后续 Phase 3 扩展选型仍按门禁规则保持 `[TBD]`。

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

## 3. Gate 1 已拍板技术决策（Approved Decisions — 2026-11-20）

> [!NOTE]
> 以下 `DEC-006` 与 `DEC-007` 均已于 **2026-11-20** 经用户 **Gate 1 正式批准（`APPROVED`）**，作为 Phase 2（v0.2）开发基线。

### DEC-006：v1 插件契约冻结与外置 Processor Webhook 策略
- **状态**：`APPROVED (2026-11-20)`
- **拍板结论**：**当前 v1 / Phase 2 暂不引入外置 Processor Webhook 钩子**；冻结现有 `POST /api/v1/ingest` 与 `POST /api/v1/heartbeat`（允许向后兼容地在响应体中补充可选控制字段，如 `paused: boolean`）作为 v1 稳定 Source 插件契约。
- **核心理由**：全部处理、门控与个性化归纳逻辑保留在 Go 核心进程内执行，避免跨容器 Webhook 序列化/网络往返开销，持续适配 1C1G–2C2G 微型 VPS。

### DEC-007：Phase 2 研发重心、三级处理管线、个性化反馈档案与信源范围
- **状态**：`APPROVED (2026-11-20)`
- **拍板结论**：
  1. **研发优先级**：Phase 2 聚焦「`P2-01` 面板/RSS 体验收口 → `P2-02` 三级 AI 处理管线 → `P2-03` 用户反馈与有界个性化档案闭环 → `P2-04` 单一新信源 Telegram 插件」；**暂不铺开 Linux.do / NodeSeek / QQ 群信源**。
  2. **原生 RSS 策略（方案 A）**：原生 RSS 订阅条目默认归入**「未打分 (`unscored`)」**不消耗 AI 额度，每个 RSS 源提供独立的「启用 AI 评分」开关（`ai_enabled`，默认关闭）按需接入处理管线。
  3. **三级递进处理管线（L1 → L2 Decision → L3 主模型）**：
     - **L1（程序规则层）**：零成本去重、广告/水帖/转推规则拦截；
     - **L2（云端轻量 Decision 门控层）**：调用云端轻量 HTTP API（通过可插拔 Adapter 支持不同请求格式，如类 Jev 轻量决策 API 或其他云端小模型 API；具体供应商保留 `[TBD - 待用户调研确认]`），仅判定拦截与初判优先级，大幅削减 L3 工作量；
     - **L3（主 LLM 精筛总结层）**：仅对 L2 放行的高优/常规候选条目执行完整评分（0–100）、中文摘要、打标与最终优先级/拦截复核。
  4. **用户优先级反馈与有界个性化档案闭环**：支持用户调整任意条目的优先级并可选填写文字反馈，关联用户标识（轻量 `user_id`，默认 `"default"`，不引入多租户 SaaS）；由核心后台定时且在模型/队列空闲时增量压缩归纳为**有硬上限的结构化个性化偏好档案**（防止无限膨胀，保留历史版本可人工查看/编辑/回滚），注入后续 L2 与 L3 判定与推送。
  5. **Telegram 插件与后续轻配置面**：`P2-04` 需兼顾**公开频道**与**个人号私有群/线报群**场景；随插件配置复杂度上升，后续可规划轻量插件设置面板（严守 Non-Goals，不做重型插件市场 UI）。

---

## 4. 后续阶段待拍板项（Pending `[TBD]`）

- **DEC-008（L2 云端 Decision 门控模型具体供应商与请求格式）**：`[TBD - 待用户调研确认；P2-02 先以可插拔云端 HTTP Adapter 抽象]`
- **DEC-009（Telegram 插件具体协议库与轻量插件设置界面契约）**：`[TBD - 待 P2-04 立项前细化]`
- **DEC-010（Phase 2 后期或 Phase 3：Linux.do / NodeSeek / QQ 群与外部高优通知渠道）**：`[TBD - 待 Gate 2 后拍板]`
