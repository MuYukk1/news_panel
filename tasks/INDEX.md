# `news_panel` 任务看板索引（Task Index）

> **当前状态**：**Phase 1（Gate 0 已于 2026-11-15 批准，v0.1 纵向切片已完成云端 Docker Compose 端到端实测，等待 Gate 1 验收）**
>
> **调度策略**：`P1-01`、`P1-02`、`P1-03` 均已 `DONE` 并合入 `main`，已在 `gcp-e2-west` 验证 `X 采集 (x-twifork) -> /api/v1/ingest -> L1+L2 AI 分拣 (CodeBuddy CLI) -> SQLite/SPA`。下一步等待用户验收 Gate 1，并按需推进 `AI_PROVIDER=openai` 常驻与 `P1-04` 高信号面板。

---

## 活跃与历史任务列表

| Task ID | 任务名称 | Phase | 调度通道 / 执行端 | 状态 | Owned Scope | 任务卡 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `P1-01` | 核心底座深度重构与 `/api/v1` 契约贯通（`0x2E/fusion`） | Phase 1 (v0.1) | 通道 B / `CodexHost (Codex: gpt 6.1 sol high)` | `DONE` | `backend/**`(不含 `internal/ai/**`), `frontend/**`(不含 `components/panel/**`), 根 `Dockerfile`/`scripts.sh`/`.gitignore`/`LICENSE`/`NOTICE` | [P1-01-core-fusion-base.md](./P1-01-core-fusion-base.md) |
| `P1-02` | 外置 X (Twitter) 纯 HTTP 采集插件（`twifork[impersonate]`） | Phase 1 (v0.1) | 通道 A / `ZCode` | `DONE` | `plugins/x-twifork/**` | [P1-02-x-source-plugin.md](./P1-02-x-source-plugin.md) |
| `P1-03` | 核心内置 L1+L2 AI 过滤与分拣管线（OpenAI-compatible + 可选 CLI 适配器） | Phase 1 (v0.1) | 通道 A / `Qoder` | `DONE` | `backend/internal/ai/**`, `backend/cmd/fusion/main.go`（最小接入） | [P1-03-ai-triage-pipeline.md](./P1-03-ai-triage-pipeline.md) |

---

## 索引维护约定
1. 每新增一张 `tasks/<TASK-ID>.md` 任务卡，必须同步在此表登记一行。
2. 状态须与任务卡内 `## 元信息` 保持一致（`TODO` / `IN_PROGRESS` / `IN_REVIEW` / `DONE` / `BLOCKED`）。
3. 使用模板 [tasks/_TEMPLATE.md](./_TEMPLATE.md) 创建新任务卡，并遵守 [tasks/README.md](./README.md) 的双通道调度与合入核验规范。
