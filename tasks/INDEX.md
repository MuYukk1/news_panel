# `news_panel` 任务看板索引（Task Index）

> **当前状态**：**Phase 2（Gate 1 已于 2026-11-20 批准，v0.1 线上闭环验收完成，进入 Phase 2 / v0.2 开发）**
>
> **调度策略**：Phase 1（`P1-01`–`P1-04`）与 Phase 2 首张任务卡 `P2-01` 均已 `DONE` 并合入 `main`（回滚标签 `baseline-pre-p2-02` / 镜像 `sha-0db31f3`）。当前已正式发布 **`P2-02`**（三级递进 AI 处理管线：L1 规则 → 可选 L2 云端 Decision 门控 → L3 主模型精筛 + RSS `ai_enabled` 接入，须在独立分支 `feat/p2-02-three-stage-ai-pipeline` 执行）；完成后依次推进 `P2-03`（用户优先级反馈与有界个性化档案）、`P2-04`（Telegram 插件）。

---

## 活跃与历史任务列表

| Task ID | 任务名称 | Phase | 调度通道 / 执行端 | 状态 | Owned Scope | 任务卡 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `P1-01` | 核心底座深度重构与 `/api/v1` 契约贯通（`0x2E/fusion`） | Phase 1 (v0.1) | 通道 B / `CodexHost (Codex: gpt 6.1 sol high)` | `DONE` | `backend/**`(不含 `internal/ai/**`), `frontend/**`(不含 `components/panel/**`), 根 `Dockerfile`/`scripts.sh`/`.gitignore`/`LICENSE`/`NOTICE` | [P1-01-core-fusion-base.md](./P1-01-core-fusion-base.md) |
| `P1-02` | 外置 X (Twitter) 纯 HTTP 采集插件（`twifork[impersonate]`） | Phase 1 (v0.1) | 通道 A / `ZCode` | `DONE` | `plugins/x-twifork/**` | [P1-02-x-source-plugin.md](./P1-02-x-source-plugin.md) |
| `P1-03` | 核心内置 L1+L2 AI 过滤与分拣管线（OpenAI-compatible + 可选 CLI 适配器） | Phase 1 (v0.1) | 通道 A / `Qoder` | `DONE` | `backend/internal/ai/**`, `backend/cmd/fusion/main.go`（最小接入） | [P1-03-ai-triage-pipeline.md](./P1-03-ai-triage-pipeline.md) |
| `P1-04` | 统一浅色磨砂液态玻璃高信号情报工作台前端重构（含双时间显示） | Phase 1 (v0.1) | 通道 A / Antigravity（`feat/p1-04-high-signal-panel` 已合入 `main`） | `DONE` | `frontend/index.html`, `frontend/src/**` | [P1-04-high-signal-panel.md](./P1-04-high-signal-panel.md) |
| `P2-01` | 阅读抽屉流畅优化、RSS 未打分归类、订阅源/插件开关、来源筛选与 OPML 多选导入 | Phase 2 (v0.2) | 通道 A / Antigravity（`feat/p2-01-ux-rss-source-controls` 已合入 `main`） | `DONE` | `backend/internal/{model,store,handler,pull}/**`, `frontend/src/**` | [P2-01-ux-rss-source-controls.md](./P2-01-ux-rss-source-controls.md) |
| `P2-02` | 三级递进 AI 分拣管线（L1 → 可选 L2 云端 Decision 门控 → L3 精筛）与 RSS `ai_enabled` 接入 | Phase 2 (v0.2) | 待认领（独立分支 `feat/p2-02-three-stage-ai-pipeline`） | `TODO` | `backend/internal/ai/**`, `backend/internal/{store/item,pull/puller,handler}/**`, `backend/cmd/fusion/main.go`, `deploy/{docker-compose.yml,.env.example}` | [P2-02-three-stage-ai-pipeline.md](./P2-02-three-stage-ai-pipeline.md) |

---

## 索引维护约定
1. 每新增一张 `tasks/<TASK-ID>.md` 任务卡，必须同步在此表登记一行。
2. 状态须与任务卡内 `## 元信息` 保持一致（`TODO` / `IN_PROGRESS` / `IN_REVIEW` / `DONE` / `BLOCKED`）。
3. 使用模板 [tasks/_TEMPLATE.md](./_TEMPLATE.md) 创建新任务卡，并遵守 [tasks/README.md](./README.md) 的双通道调度与合入核验规范。
