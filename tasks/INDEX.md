# `news_panel` 任务看板索引（Task Index）

> **当前状态**：**Phase 1（Gate 0 已于 2026-11-15 批准，v0.1 纵向切片开发中）**  
> **调度策略**：**ZCode 额度充足且偏向拆分简单、边界干净的独立任务**；`P1-01` 与 `P1-02` 已分别由通道 B（CodexHost）与通道 A（ZCode）完成主体开发，待完成联调核验并拆分 `P1-03` / `P1-04`。

---

## 活跃与历史任务列表

| Task ID | 任务名称 | Phase | 调度通道 / 执行端 | 状态 | Owned Scope | 任务卡 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `P1-01` | 核心底座深度重构与 `/api/v1` 契约贯通（`0x2E/fusion`） | Phase 1 (v0.1) | 通道 B / `CodexHost (Codex: gpt 6.1 sol high)` | `IN_REVIEW` | `backend/**`(不含 `internal/ai/**`), `frontend/**`(不含 `components/panel/**`), 根 `Dockerfile`/`scripts.sh`/`.gitignore`/`LICENSE`/`NOTICE` | [P1-01-core-fusion-base.md](./P1-01-core-fusion-base.md) |
| `P1-02` | 外置 X (Twitter) 纯 HTTP 采集插件（`twifork[impersonate]`） | Phase 1 (v0.1) | 通道 A / `ZCode` | `DONE` | `plugins/x-twifork/**` | [P1-02-x-source-plugin.md](./P1-02-x-source-plugin.md) |

---

## 索引维护约定
1. 每新增一张 `tasks/<TASK-ID>.md` 任务卡，必须同步在此表登记一行。
2. 状态须与任务卡内 `## 元信息` 保持一致（`TODO` / `IN_PROGRESS` / `IN_REVIEW` / `DONE` / `BLOCKED`）。
3. 使用模板 [tasks/_TEMPLATE.md](./_TEMPLATE.md) 创建新任务卡，并遵守 [tasks/README.md](./README.md) 的双通道调度与合入核验规范。
