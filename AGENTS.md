# `news_panel` AI 协作总入口规范（AGENTS.md）

> **适用对象**：所有接入本仓库的 AI 编程工具与多 Agent 执行端（包括 Antigravity、ZCode、Qoder、Trae，以及经 CodexHost 调度的 Codex、WorkBuddy、OpenCode、DeepSeek Harness）。  
> **核心目标**：跨工具按额度灵活协同，同时严防上下文漂移、文件踩踏与过度设计。

---

## 1. 强制阅读顺序（开始任何工作前）

在修改或创建任何文件之前，必须按以下顺序阅读基线文档：
1. `AGENTS.md`（本文件：全局红线与协作契约）
2. [docs/HANDOFF.md](./docs/HANDOFF.md)（当前阶段快照、门禁状态与交接清单）
3. [docs/DECISIONS.md](./docs/DECISIONS.md)（已确认原则 `ADR` 与待拍板项 `DEC`）
4. [docs/PRD.md](./docs/PRD.md)（产品目标、双层插件架构、Explicit Non-Goals）
5. [docs/ROADMAP.md](./docs/ROADMAP.md)（Phase 规划与 Human Gate 条件）
6. 若你被分配了具体任务：先读 [tasks/README.md](./tasks/README.md) 与对应的 `tasks/<TASK-ID>.md` 任务卡

---

## 2. 不可逾越的全局红线（Anti-Drift Rules）

1. **Human Gate 门禁铁律**
   - 核对 [docs/HANDOFF.md](./docs/HANDOFF.md) 中的当前门禁状态（**当前：Gate 1 已于 2026-11-20 通过，已进入 Phase 2 / v0.2 开发，等待后续 Gate 2 验收**）。
   - **未过 Gate 禁越界**：任何未解锁阶段的跨期代码（如 Gate 2 前的 Phase 3 演进项或未拍板的 Linux.do/NodeSeek/QQ 信源）严禁提前编写；仅允许在已拆分的 `tasks/<TASK-ID>.md` 授权 `Owned Files` 范围内编写代码与 `Dockerfile`。
   - **严禁越权锁定 `[TBD]`**：凡在 [docs/DECISIONS.md](./docs/DECISIONS.md) 中标为 `[TBD]` 的技术选型，未经用户明确拍板，任何工具/Agent 不得擅自拍板或写入实现代码。
2. **Demo-First / Vertical Slice（演示优先）**
   - v0.1 必须先跑通端到端最小闭环：`1 个 X 信源 -> 1 个最小插件契约 -> 1 个 AI 过滤器 -> 在面板中可见`。
   - 跑通前严禁搭建庞大前置抽象、通用 SDK 或提前铺开 Phase 2 多信源。
3. **Explicit Non-Goals（Phase 0/1 明确不做）**
   - 禁止引入：多租户 SaaS、复杂 RBAC/Auth、原生移动端 App、内置插件市场 UI、重型分布式基建（Kafka / ELK / JVM 全家桶等）。
4. **宽松开源协议合规（Permissive License）**
   - 核心底座与直接依赖优先 **MIT / Apache-2.0 / BSD**；**严禁直接引入或复用 GPL / AGPL 代码**作为项目核心底座（GPL/AGPL 项目仅可作交互或思路参考）。
   - 拒绝"浅层二改套壳"，核心逻辑与插件边界须朝独立项目 `news_panel` 演进。
5. **微型 VPS 资源红线（1C1G–2C2G）**
   - 目标运行环境为 24/7 Docker on 多台微型 Linux 云 VPS。
   - 严禁在核心或常驻插件中引入无头浏览器（如 Chromium/Playwright）或高内存组件；优先纯 HTTP、轻量运行时、外部云端 API/CLI。

---

## 3. 文件所有权与防冲突边界（Owned Files）

多工具/多 Agent 跨额度接力或并行工作时，必须严格遵守任务卡中的范围约定：
- **只改 `Owned Files`**：只能创建或修改当前任务卡明确授权的 `Owned Files` 路径。
- **严禁越界修改**：未列在 `Owned Files` 中的文件（尤其是核心契约、`docs/DECISIONS.md`、他人负责模块及 `Forbidden Files`）一律视为**只读**。
- **发现需改边界外文件怎么办**：不要直接偷改；在任务卡的 `## Handoff Report` 中列明所需变更与原因，交由协调者或人类确认后再另行派单或统一合入。
- **Git 工作区隔离**：多任务并行（尤其经 CodexHost 调度时）须在独立 Git worktree / 分支中执行，严禁多个执行端直接混写同一工作目录。

---

## 4. 任务完成必填回执（Mandatory Handoff Report）

任何工具或 Agent 结束工作时，必须提供可审计的交付回执（写入对应 `tasks/<TASK-ID>.md` 的 `## Handoff Report`，或按调度渠道返回结构化结果）：
1. **状态变更**：说明目标状态（如 `IN_REVIEW` 或 `BLOCKED`）。
2. **改动文件清单**：逐一列出新增/修改的文件路径（须完全落入 `Owned Files`）。
3. **验证命令与实际输出**：只陈述**亲自运行并实际观察到**的命令与结果，禁止猜测、跳过或美化输出；文档任务需明确核对清单检查结果。
4. **遗留风险 / 阻塞项 / 下一步建议**：说明任何未决问题或需要人类/下一任工具接手的前提。
