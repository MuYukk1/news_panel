# `news_panel` 任务看板索引（Task Index）

> **当前状态**：**Phase 2（Gate 1 已于 2026-11-20 批准，v0.1 线上闭环验收完成，进入 Phase 2 / v0.2 开发）**
>
> **调度策略**：Phase 1（`P1-01`–`P1-04`）与 Phase 2 的 `P2-01`、`P2-02`、`P2-02a`、`P2-02b`（落实 `DEC-008`：L2 四大原生 Decisions APIs + 轻量 AI 设置面板与热更新）均已 `DONE` 并合入 `main`。下一步按序推进 **`P2-03`（用户优先级反馈与有界个性化档案闭环）**；**`P2-04`**（Telegram 外置插件）待 `DEC-009` 拍板后可并行派发。

---

## 活跃与历史任务列表

| Task ID | 任务名称 | Phase | 调度通道 / 执行端 | 状态 | Owned Scope | 任务卡 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `P1-01` | 核心底座深度重构与 `/api/v1` 契约贯通（`0x2E/fusion`） | Phase 1 (v0.1) | 通道 B / `CodexHost (Codex: gpt 6.1 sol high)` | `DONE` | `backend/**`(不含 `internal/ai/**`), `frontend/**`(不含 `components/panel/**`), 根 `Dockerfile`/`scripts.sh`/`.gitignore`/`LICENSE`/`NOTICE` | [P1-01-core-fusion-base.md](./P1-01-core-fusion-base.md) |
| `P1-02` | 外置 X (Twitter) 纯 HTTP 采集插件（`twifork[impersonate]`） | Phase 1 (v0.1) | 通道 A / `ZCode` | `DONE` | `plugins/x-twifork/**` | [P1-02-x-source-plugin.md](./P1-02-x-source-plugin.md) |
| `P1-03` | 核心内置 L1+L2 AI 过滤与分拣管线（OpenAI-compatible + 可选 CLI 适配器） | Phase 1 (v0.1) | 通道 A / `Qoder` | `DONE` | `backend/internal/ai/**`, `backend/cmd/fusion/main.go`（最小接入） | [P1-03-ai-triage-pipeline.md](./P1-03-ai-triage-pipeline.md) |
| `P1-04` | 统一浅色磨砂液态玻璃高信号情报工作台前端重构（含双时间显示） | Phase 1 (v0.1) | 通道 A / Antigravity（`feat/p1-04-high-signal-panel` 已合入 `main`） | `DONE` | `frontend/index.html`, `frontend/src/**` | [P1-04-high-signal-panel.md](./P1-04-high-signal-panel.md) |
| `P2-01` | 阅读抽屉流畅优化、RSS 未打分归类、订阅源/插件开关、来源筛选与 OPML 多选导入 | Phase 2 (v0.2) | 通道 A / Antigravity（`feat/p2-01-ux-rss-source-controls` 已合入 `main`） | `DONE` | `backend/internal/{model,store,handler,pull}/**`, `frontend/src/**` | [P2-01-ux-rss-source-controls.md](./P2-01-ux-rss-source-controls.md) |
| `P2-02` | 三级递进 AI 分拣管线（L1 → 可选 L2 云端 Decision 门控 → L3 精筛）与 RSS `ai_enabled` 接入 | Phase 2 (v0.2) | 通道 A / Codex（`feat/p2-02-three-stage-ai-pipeline` 已合入 `main`） | `DONE` | `backend/internal/ai/**`, `backend/internal/{store/item,pull/puller,handler}/**`, `backend/cmd/fusion/main.go`, `deploy/{docker-compose.yml,.env.example}` | [P2-02-three-stage-ai-pipeline.md](./P2-02-three-stage-ai-pipeline.md) |
| `P2-02a` | 单源 RSS 刷新 30s 超时热修（解耦 HTTP 抓取与 AI 分拣超时 + 同源刷新去重） | Phase 2 (v0.2 Hotfix) | Swarm Worker / `hotfix-p202a`（独立 `verifier` 复验通过） | `DONE` | `backend/internal/pull/{puller,puller_test}.go`, `backend/internal/handler/{feed,handler,feed_refresh_test}.go` | [P2-02a-feed-refresh-timeout-hotfix.md](./P2-02a-feed-refresh-timeout-hotfix.md) |
| `P2-02b` | L2 原生多厂商 Decisions API 适配器（Gemini/OpenAI/TypeSafe/Cloudflare Clef）与轻量 AI 设置面板 | Phase 2 (v0.2 / DEC-008) | 通道 A / Codex（`feat/p2-02b-decision-apis-and-ai-settings` 已合入 `main`） | `DONE` | `backend/internal/ai/**`, `backend/internal/pkg/httpc/**`, `backend/internal/model/model.go`, `backend/internal/store/migrations/008_ai_settings.sql`, `backend/internal/store/ai_settings*.go`, `backend/internal/pull/{puller,puller_test}.go`, `backend/internal/handler/{handler,ai_settings*}.go`, `backend/cmd/fusion/main.go`, `frontend/src/components/settings/**`, `frontend/src/lib/api/{types,index}.ts`, `frontend/src/lib/i18n/messages/**`, `deploy/{docker-compose.yml,.env.example}` | [P2-02b-decision-apis-and-ai-settings.md](./P2-02b-decision-apis-and-ai-settings.md) |

---

## 索引维护约定
1. 每新增一张 `tasks/<TASK-ID>.md` 任务卡，必须同步在此表登记一行。
2. 状态须与任务卡内 `## 元信息` 保持一致（`TODO` / `IN_PROGRESS` / `IN_REVIEW` / `DONE` / `BLOCKED`）。
3. 使用模板 [tasks/_TEMPLATE.md](./_TEMPLATE.md) 创建新任务卡，并遵守 [tasks/README.md](./README.md) 的双通道调度与合入核验规范。
