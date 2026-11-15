# `news_panel` 产品需求文档（PRD）

> **文档状态**：Phase 1 执行基线（Living Doc — **Gate 0 已于 2026-11-15 批准**）  
> **当前边界**：已进入 **Phase 1（v0.1 纵向切片开发）**；严守 Demo-First 原则，**Gate 1 验收通过前禁止扩展 Phase 2 多信源与重型抽象**。  
> **标记约定**：Gate 0 已锁定的核心选型详见 [DECISIONS.md](./DECISIONS.md) 与 [PLUGIN_SPEC.md](./PLUGIN_SPEC.md)；Phase 2+ 扩展项仍保留为 `[TBD - Gate 1 后确认]`。

---

## 1. 产品愿景与核心痛点

### 1.1 产品定位
`news_panel` 是一个**个人高信噪比情报与信息流聚合面板**（Personal high-signal-to-noise intelligence & information stream panel），旨在解决跨异构、高时效信息源带来的**认知过载（cognitive overload）**问题。

### 1.2 核心覆盖场景与典型信源
系统聚焦于对时效性与准确度要求极高的三类个人核心情报：
1. **投资 / 股票交易突发新闻与催化剂（Breaking news & catalysts）**
   - 关注市场异动触发点、突发性宏观/个股消息与关键催化事件。
2. **AI 前沿动态与一线泄露（AI updates）**
   - 追踪官方 X 账号、核心研发/意见领袖（key leads）、以及内测/灰测泄露（beta/grey-test leaks）。
3. **薅羊毛与实时线报（Deal-hunting & real-time tips）**
   - 聚合来自 **X (Twitter)、Linux.do、NodeSeek、QQ 群、Telegram 群** 等多渠道的实时优惠、限免、漏洞与社区一手信息。

---

## 2. 核心能力要求

`news_panel` 不是单纯的原文搬运/RSS 堆砌器，而是"采集 + 智能清洗过滤 + 面板消费"的完整链路：

1. **自动化采集（Automated Collection）**
   - 持续、低资源开销地接入多类异构且高动态的外部信源（社交平台、技术论坛、即时通讯群组）。
2. **AI / 程序化预处理工作流（AI / Programmatic Workflow）**
   - 信息在进入前端面板展示前，必须经过前置处理管线：
     - **去重（Dedup）**：跨渠道/同渠道重复转发与相似快讯收敛。
     - **去噪（Noise Rejection）**：剔除水贴、情绪化灌水、广告及无关噪声。
     - **相关性评分（Relevance Scoring）**：依据用户关注主题评估情报价值。
     - **智能打标（Tagging）**：自动归类领域（如投资 / AI / 薅羊毛）、信源属性与实体标签。
     - **优先级路由（Priority Routing）**：区分高优突发情报与常规阅读流，支撑分级展示或后续触达。
3. **高信噪比信息流面板（Panel Presentation）**
   - 提供直观、高效的信息浏览、筛选、阅读状态管理与重点突出体验（基于 React 19 SPA 演进，UI 交互借鉴 `newsnow`/`glance` 思路，详见 [DECISIONS.md](./DECISIONS.md)）。

---

## 3. 架构原则与开源策略

### 3.1 独立演进策略（不从零造轮子，也不做浅层套壳）
- **拒绝从零手写基础设施**：不从零重复实现基础面板 UI、通用状态管理或常规 Web 底座。
- **拒绝浅层二次修改（"不当 XX 二改套壳项目"）**：选定一个成熟、结构清晰的开源项目作为技术底座或参考（最多借鉴思路），在此基础上进行**深度重构与架构演进**，形成定位清晰、结构自治的独立项目 `news_panel`。

### 3.2 开源协议偏好（Permissive License）
- 底座选型与核心依赖**优先采用宽松开源协议**（**MIT / Apache-2.0 / BSD**）。
- **明确规避 GPL / AGPL 等传染性协议（copyleft constraints）**，确保项目未来的独立发布与演进自由度（GPL/AGPL 类知名项目仅可作为思路或 UI 交互参考，不得直接作为发布底座）。

### 3.3 双层插件化架构（Dual-Layer Pluginization）
核心系统与扩展能力之间必须清晰解耦，保证**在不重写核心代码的前提下**即可轻松替换或新增插件：
- **第一层：信源插件（Source Plugins）**
  - 负责对接具体平台的数据拉取或事件监听（如 X 追踪器、QQ 群机器人、TG 群监听器、Linux.do / NodeSeek 论坛监控等）。
- **第二层：处理与过滤插件（Processing / Filtering Plugins）**
  - 负责信息流清洗与决策流水线（如去重、AI 评分、打标、优先级路由等）。

### 3.4 解耦接入与跨语言/跨节点边界
- 考虑采用 **HTTP / Webhook 或解耦的插件通信边界**：
  - 允许插件使用**任意编程语言**编写（无论由用户手写还是由 AI Agent 辅助生成）。
  - 允许部分采集或处理插件运行在**多台不同的微型服务器上**，通过轻量协议向核心面板投递或拉取任务。

### 3.5 已拍板核心技术栈与演进选型汇总（Gate 0 Approved — 2026-11-15）
具体决策理由见 [DECISIONS.md](./DECISIONS.md)，v0.1 契约见 [PLUGIN_SPEC.md](./PLUGIN_SPEC.md)：
- 面板 UI / 开源参考底座：**`0x2E/fusion`（MIT）深度重构演进**（UI 卡片/多栏交互借鉴 `newsnext/newsnow` MIT 与 `glanceapp/glance` 思路）
- 核心后端语言与 Web 框架：**Go + 内嵌 React 19 SPA**
- 数据存储与轻量队列/缓存选型：**`modernc.org/sqlite`（纯 Go 无 CGO）**，基于 `items` 持久化模型扩展 AI/分拣字段
- 双层插件架构与最小契约：**内置 + 外置混合双层插件**；外置插件走 `POST /api/v1/ingest` + `POST /api/v1/heartbeat`（v0.1 最小契约，Gate 1 后扩展外置 Processor Webhook 钩子）
- AI 处理管线集成方式：**核心内置两段式（L1 规则/关键词粗筛 + L2 云端 OpenAI-compatible LLM 评分/打标/去噪/摘要）**
- v0.1 X (Twitter) 采集技术路径：**外置纯 HTTP Python + `PawiX25/twifork[impersonate]`（`curl_cffi`，MIT）+ 备用账号 Cookie**（优先抗 Cloudflare 与低风控，严禁无头浏览器）
- Phase 2 多信源与完整 v1 契约扩展：`[TBD - Gate 1 后确认]`

---

## 4. 部署约束与资源预算

- **运行形态**：基于 Docker 在 Linux 云 VPS 上 **7×24 小时**持续运行。
- **单节点硬件约束**：单台 VPS 的 CPU 与内存资源**非常有限**（整体拓扑更接近"**多台微型 VPS 节点分散协同**"，而非依赖单台高配大内存机器）。
- **外部云能力借力**：允许合理利用云端 API 或云端 CLI 完成高开销的数据抓取或 AI 推理任务，以最大限度压缩本地容器的内存与算力占用。

---

## 5. 首个验证目标（v0.1 Demo）

- **首发目标信源**：**X (Twitter)**
- **选择理由**：X 平台实时信息密度高且噪声极大，是验证 `news_panel` 核心价值（AI 去噪、评分与高信噪比呈现）最理想的试金石。
- **v0.1 最小纵向切片（Vertical Slice）**：
  > `1 个 X 信源 -> 1 个最小插件契约 -> 1 个 AI 过滤器 -> 在面板中可见`

---

## 6. 明确非目标（Explicit Non-Goals - Phase 0/1）

为防止架构膨胀与精力分散，Phase 0 与 Phase 1 阶段**明确不做**以下事项：
- **多租户 SaaS 化**（Multi-tenant SaaS）
- **复杂的 RBAC 权限体系与认证中心**（Complex RBAC / auth）
- **原生移动端 App**（Native mobile apps，iOS / Android）
- **内置插件市场 UI**（Built-in plugin marketplace UI）
- **重型分布式基础设施**（Heavy distributed infra，如 Kafka、ELK、JVM 全家桶等）

---

## 7. 防跑偏与多 Agent 协同治理红线（Governance Rules）

> [!IMPORTANT]
> 本节为所有人类开发者与 AI Agent / Swarm Worker 在推进本项目时必须严格遵守的最高约束：

1. **Demo-First / Vertical Slice（演示优先 / 纵向切片）**
   - 在 v0.1 端到端闭环（`1 个 X 信源 -> 1 个最小插件契约 -> 1 个 AI 过滤器 -> 在面板中可见`）真实跑通之前，**严禁搭建庞大、泛化的前置抽象框架**。
   - 先有可运行验证的最小切片，再基于真实反馈演进通用抽象。
2. **Human-Gated Checkpoints（人工审批门禁）**
   - 任何关键阶段不得越过人工确认自行扩展或并行拆解：
     - **Gate 0（已完成 2026-11-15）**：底座选型、技术栈、AI 管线、X 方案与 v0.1 最小契约已由用户拍板；
     - **Gate 1（待验收）**：v0.1 纵向切片跑通并经用户验收后，方可冻结正式 v1 插件契约并启动 Phase 2 多信源并行开发。
3. **跨工具额度拆分与防踩踏纪律**
   - 协同策略上，**ZCode 额度充足且偏向拆分简单、边界干净的独立任务**（如外置插件 `tasks/P1-02-x-source-plugin.md`），核心架构重构则集中由主控任务收口；所有并行任务必须通过 `tasks/` 任务卡明确 `Owned Files` 与 Git worktree 隔离边界，禁止越界修改。
