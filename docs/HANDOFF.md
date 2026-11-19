# `news_panel` 交接指南（HANDOFF）

> **用途**：确保任何新开启的对话或新加入的 Swarm Worker 能在 1 分钟内对齐现状、边界与下一步动作，避免上下文断层或越界开发。

---

## 1. 当前项目快照

- **当前所处阶段**：**Phase 2（v0.2 体验收口、三级处理管线与个性化闭环 —— Gate 1 已于 2026-11-20 正式批准）**
- **代码仓库现状**：Phase 1（v0.1）全部任务 `P1-01`、`P1-02`（含 `729e9bc` 静默熔断修复）、`P1-03`、`P1-04`（含双时间显示）均已完成并合入 `main`（基线提交 `5925b42`，已于 `gcp-e2-west` 线上部署验证）。
- **协同与派发策略**：当前主控作为 LEAD 已正式发布 Phase 2 首张任务卡 [tasks/P2-01-ux-rss-source-controls.md](../tasks/P2-01-ux-rss-source-controls.md)（抽屉流畅动画 + RSS 未打分归类 + 订阅源/插件液态玻璃开关 + 来源筛选 + OPML/多源多选批量导入，**须在独立分支/worktree `feat/p2-01-ux-rss-source-controls` 执行**）；后续依次推进 `P2-02`（三级处理管线）、`P2-03`（用户反馈与有界个性化档案）、`P2-04`（Telegram 插件）。

---

## 2. 核心文档导航与职责边界

| 文档 | 核心职责 | 何时更新 |
| :--- | :--- | :--- |
| [AGENTS.md](../AGENTS.md) | 面向所有 AI 工具的根目录总入口规范：必读顺序、全局红线、`Owned Files` 防冲突边界与必填交付回执 | 全局协作规则或安全红线调整时 |
| [PRD.md](./PRD.md) | 唯一的需求真理源：产品定位、核心场景、架构/协议原则、资源约束、Non-Goals 与治理红线 | 需求范围或核心约束变更时 |
| [ROADMAP.md](./ROADMAP.md) | 分阶段演进路径、v0.1 纵向切片定义及各阶段 **Human Gate** 准出条件 | 进入新 Phase 或 Milestone 调整时 |
| [DECISIONS.md](./DECISIONS.md) | 已确认架构原则（ADR）与 Gate 0 已批准决策（`DEC-001`–`DEC-005`）记录 | 每次用户拍板或产生新架构决策时 |
| [PLUGIN_SPEC.md](./PLUGIN_SPEC.md) | **v0.1 双层插件最小契约规范**：`POST /api/v1/ingest`、`POST /api/v1/heartbeat`、`items` AI 字段与内置/外置等价模型 | 契约演进或 Gate 1 冻结 v1 规范时 |
| [HANDOFF.md](./HANDOFF.md) | 当前工程快照、新成员上手清单与下一步 Gate 待办动作 | 每次阶段切换、重要交接或状态变更时 |
| [tasks/README.md](../tasks/README.md) | 跨工具双通道调度规范（ZCode/Qoder/Trae 文档交接 & CodexHost 调度）、状态机与合入核验规范 | 调度流程或任务状态流转变更时 |
| [tasks/INDEX.md](../tasks/INDEX.md) | 当前轻量任务看板索引表（配合 [tasks/_TEMPLATE.md](../tasks/_TEMPLATE.md) 使用） | 创建、认领、阻塞或完成任意任务卡时 |

---

## 3. 新会话 / 新 Worker 快速上手清单

1. **按顺序阅读文档**：[AGENTS.md](../AGENTS.md) → `docs/HANDOFF.md` → `docs/DECISIONS.md` → `docs/PRD.md` → `docs/ROADMAP.md` →（涉及接口/插件必读）`docs/PLUGIN_SPEC.md` → `tasks/README.md` 与对应 `tasks/<TASK-ID>.md`。
2. **核对当前 Gate 与任务边界**：**Gate 0（2026-11-15）与 Gate 1（2026-11-20）均已批准**，项目已进入 **Phase 2（v0.2）**。
   - 执行端按 [tasks/README.md](../tasks/README.md) 领取任务卡（通道 A：ZCode / Qoder / Trae；通道 B：经 CodexHost 调度的 Codex / WorkBuddy / OpenCode / DeepSeek Harness）。
   - **严守边界**：只修改自己任务卡授权的 `Owned Files`（及独立 Git worktree），**Gate 2 前严禁擅自扩展未排期信源（Linux.do/NodeSeek/QQ）或重型插件市场 UI**；结束时必须填妥 `## Handoff Report`。
3. **遵守防跑偏纪律**：
   - Demo-First：`1 个 X 信源 (twifork) -> HTTP 最小契约 -> Go 核心 L1/L2 AI 过滤 -> React 19 面板可见` 优先于一切通用抽象；
   - 协议红线：绝不引入 GPL / AGPL 代码或依赖；
   - 资源红线：所有组件须适配极小规格 Linux VPS Docker 环境（1C1G–2C2G），严禁引入无头浏览器。

---

## 4. 门禁状态与下一步待办（Gate 0 ✅ / Gate 1 ✅ / Next: Gate 2）

### Gate 0（✅ 已于 2026-11-15 批准）
- [x] **DEC-001**–**DEC-005**：锁定 `0x2E/fusion` 底座、Go+SQLite+React 19 栈、双层插件最小契约、L1+L2 AI 管线、纯 HTTP `twifork` X 采集。

### Gate 1（✅ 已于 2026-11-20 批准）
- [x] 完成 `P1-01`、`P1-02`（含 `AccountLocked`/`AccountSuspended` 静默熔断）、`P1-03`、`P1-04`（含双时间显示）并完成云端实测验收
- [x] **DEC-006**：冻结现行 `/api/v1/ingest` + `/api/v1/heartbeat` 为 v1 稳定契约，当前版本暂不加外置 Processor Webhook 钩子
- [x] **DEC-007**：锁定 Phase 2 聚焦「UI/RSS 收口（`P2-01`）→ 三级云端处理管线（`P2-02`）→ 用户优先级反馈与有界个性化档案闭环（`P2-03`）→ Telegram 单一新信源（`P2-04`）」；原生 RSS 采用方案 A（默认「未打分」，单源可选开启 AI 评分）

### Next Human Gate：Gate 2（Phase 2 v0.2 验收门禁）
- [ ] 完成 [tasks/P2-01-ux-rss-source-controls.md](../tasks/P2-01-ux-rss-source-controls.md)（`TODO`，抽屉流畅动画 + RSS 未打分归类 + 订阅源/插件开关 + 来源筛选 + OPML/多源多选导入）
- [ ] 拆解并完成 `P2-02`（三级处理管线：L1 → L2 云端轻量 Decision Adapter → L3 精筛总结）
- [ ] 拆解并完成 `P2-03`（用户优先级反馈 + 空闲定时归纳有界个性化档案闭环）
- [ ] 拆解并完成 `P2-04`（外置 Telegram 信源插件，兼容公开频道与个人号私有群）
- [ ] 用户验收 Phase 2 整体效果并拍板 Gate 2（`DEC-010`）

---

## 5. Living Doc 维护约定

- 任何已拍板的技术选型，必须第一时间将 `DECISIONS.md` 中的 `[TBD - 待 Phase 0 选型拍板]` 替换为**明确结论、选型理由与日期**，并同步更新 `PRD.md` / `ROADMAP.md` 中的对应占位状态。
- 每次阶段里程碑交接前，必须更新 `HANDOFF.md` 的"当前项目快照"与"下一步清单"，确保信息对新会话即时可用。
