# `news_panel` 交接指南（HANDOFF）

> **用途**：确保任何新开启的对话或新加入的 Swarm Worker 能在 1 分钟内对齐现状、边界与下一步动作，避免上下文断层或越界开发。

---

## 1. 当前项目快照

- **当前所处阶段**：**Phase 1（v0.1 X 纵向切片开发 —— Gate 0 已于 2026-11-15 正式批准）**
- **代码仓库现状**：v0.1 纵向切片全部任务 `P1-01`、`P1-02`、`P1-03`、`P1-04` 均已完成并合入 `main`（状态均为 `DONE`），并已完成 `plugins/x-twifork` 账号锁定/封禁静默熔断修复（`729e9bc`）与前端液态玻璃高信号工作台 + 双时间显示（安全回滚 Tag `baseline-pre-p1-04` / 镜像 `sha-01d3015`）。
- **协同与派发策略**：`main` 推送将自动触发 GitHub Actions 构建并发布最新 GHCR 镜像；云端 VPS 拉取最新镜像更新 `news-panel`（`x-twifork` 待备用号人工解锁后恢复），随后进入 **Gate 1 验收**。

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
2. **核对当前 Gate 与任务边界**：**Gate 0 已于 2026-11-15 批准**，项目已进入 **Phase 1（v0.1）**（注：若 `AGENTS.md` 仍留有早期 Gate 0 提示语，以本文件与 `DECISIONS.md` 为准）。
   - 执行端按 [tasks/README.md](../tasks/README.md) 领取任务卡（通道 A：ZCode / Qoder / Trae；通道 B：经 CodexHost 调度的 Codex / WorkBuddy / OpenCode / DeepSeek Harness）。
   - **严守边界**：只修改自己任务卡授权的 `Owned Files`（及独立 Git worktree），围绕 v0.1 纵向切片开发，**Gate 1 前严禁提前扩展 Phase 2 多信源**；结束时必须填妥 `## Handoff Report`。
3. **遵守防跑偏纪律**：
   - Demo-First：`1 个 X 信源 (twifork) -> HTTP 最小契约 -> Go 核心 L1/L2 AI 过滤 -> React 19 面板可见` 优先于一切通用抽象；
   - 协议红线：绝不引入 GPL / AGPL 代码或依赖；
   - 资源红线：所有组件须适配极小规格 Linux VPS Docker 环境（1C1G–2C2G），严禁引入无头浏览器。

---

## 4. 门禁状态与下一步待办（Gate 0 ✅ / Next: Gate 1）

### Gate 0（✅ 已于 2026-11-15 批准）
- [x] **DEC-001**：确定以 `0x2E/fusion`（MIT）作为深度重构底座（UI 借鉴 `newsnow`/`glance` 思路）
- [x] **DEC-002**：确定核心栈为 Go + `modernc.org/sqlite` + 内嵌 React 19 SPA
- [x] **DEC-003**：确定内置 + 外置混合双层插件架构，v0.1 契约冻结于 [PLUGIN_SPEC.md](./PLUGIN_SPEC.md)
- [x] **DEC-004**：确定 v0.1 AI 管线为内置 L1 规则粗筛 + L2 云端 OpenAI-compatible LLM 评分/打标/摘要
- [x] **DEC-005**：确定 v0.1 外置 X 插件采用纯 HTTP Python + `twifork[impersonate]` + 用户备用号 Cookie

### Next Human Gate：Gate 1（Phase 1 v0.1 验收门禁）
- [x] 完成 [tasks/P1-02-x-source-plugin.md](../tasks/P1-02-x-source-plugin.md)（通道 A / ZCode，`DONE`）
- [x] 完成 [tasks/P1-03-ai-triage-pipeline.md](../tasks/P1-03-ai-triage-pipeline.md)（通道 A / Qoder，`DONE`）
- [x] 完成 [tasks/P1-01-core-fusion-base.md](../tasks/P1-01-core-fusion-base.md) 容器镜像复验并在云服务器完成 `X 采集 + AI 筛选` 端到端 Docker 联调跑通（`DONE`，已于 `gcp-e2-west` 实测通过；注：1G `e2-micro` 突发型 CPU 上连续 Node CLI 冷启动会触发 CPU 节流，常驻建议切 `AI_PROVIDER=openai`）
- [x] 完成 [tasks/P1-04-high-signal-panel.md](../tasks/P1-04-high-signal-panel.md)（统一浅色磨砂液态玻璃工作台前端重构 + 双时间显示，`DONE`）
- [ ] 用户验收 v0.1 X 实时数据流去噪/评分效果与容器内存占用
- [ ] 用户拍板冻结正式 **v1 插件规范**（含外置 Processor Webhook 钩子）及 Phase 2 多信源排期（`DEC-006`、`DEC-007`）

---

## 5. Living Doc 维护约定

- 任何已拍板的技术选型，必须第一时间将 `DECISIONS.md` 中的 `[TBD - 待 Phase 0 选型拍板]` 替换为**明确结论、选型理由与日期**，并同步更新 `PRD.md` / `ROADMAP.md` 中的对应占位状态。
- 每次阶段里程碑交接前，必须更新 `HANDOFF.md` 的"当前项目快照"与"下一步清单"，确保信息对新会话即时可用。
