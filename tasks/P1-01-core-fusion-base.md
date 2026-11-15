# `P1-01`：核心底座深度重构与 `/api/v1` 契约贯通（基于 `0x2E/fusion`）

## 元信息（Metadata）
- **Task ID**：`P1-01`
- **Phase / Milestone**：`Phase 1 (v0.1 纵向切片)`
- **前置门禁（Gate Dependency）**：`Gate 0 已于 2026-11-15 正式批准`（已完成 `0x2E/fusion` 模块拆分调研，允许在授权目录内引入底座代码并实施重构）
- **当前状态（Status）**：`TODO`
- **调度通道与目标执行端（Channel / Runner）**：`通道 B：CodexHost (Codex: gpt 6.1 sol high)`
- **工作区 / 分支策略（Worktree / Branch）**：`独立 Git worktree（如 feat/p1-01-core-fusion-base）。前置环境提醒：使用 Git worktree 隔离执行前需确保本仓库已初始化 Git 并至少存在一次基线提交（baseline commit）`

---

## 目标与上下文（Objective & Context）
- **核心目标**：导入并深度重构 **`0x2E/fusion`（MIT，`v1.3.0`）** 骨架至仓库根目录（完整保留 MIT 许可与版权归属声明），完成 SQLite 扩展迁移、严格对齐 [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md) 的 `/api/v1/ingest` 与 `/api/v1/heartbeat` 契约实现、可插拔 `TriageEvaluator` 内部接口（内置 v0.1 最小直通桩，将 `backend/internal/ai/**` 完整留给后续 ZCode 任务 `P1-03`），以及前端列表/详情对 `domain` / `ai_score` / `ai_tags` / `ai_summary` / `priority` / `triage_status` 的最小可见展示（将 `frontend/src/components/panel/**` 完整留给后续 ZCode 任务 `P1-04`）。
- **前置输入 / 依赖任务**：技术底座与最小契约已冻结于 [docs/DECISIONS.md](../docs/DECISIONS.md) 与 [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)。本任务为 Phase 1 核心主干，需与外置采集插件 `P1-02`（`plugins/x-twifork/**`）保持接口对齐且物理文件零重叠。

---

## 关联文档与约束（References & Constraints）
开始前必读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)（重点核对 `ADR-001`–`ADR-004`、`DEC-001`–`DEC-004`）
4. [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)（`/api/v1/ingest`、`/api/v1/heartbeat`、`items` AI 扩展字段及内部等价模型）
5. [docs/PRD.md](../docs/PRD.md) & [docs/ROADMAP.md](../docs/ROADMAP.md)
6. [tasks/README.md](./README.md)

本任务硬性约束：
- [x] **宽松协议与版权合规**：完整保留 `0x2E/fusion` 的 MIT `LICENSE` 与版权归属声明，并在根目录 `LICENSE` / `NOTICE` 明确记载演进来源；**严禁引入 GPL / AGPL 代码或依赖**
- [x] **微型 VPS 资源红线（1C1G–2C2G）**：保持单容器 Go + `modernc.org/sqlite`（无 CGO）+ 内嵌 React 19 SPA 低内存特征（~20–40 MB 级别），严禁引入 Postgres / Redis / Kafka / ELK / JVM 或无头浏览器
- [x] **Demo-First / 最小贯通边界**：不在本任务提前实现多信源适配或复杂 AI 规则引擎，通过接口边界将 `backend/internal/ai/**` 与 `frontend/src/components/panel/**` 干净预留给后续 ZCode 独立任务卡
- [x] **严格遵守 `Owned Files` 与独立 worktree 隔离**：禁止触碰 `plugins/**`、`backend/internal/ai/**`、`frontend/src/components/panel/**`、`docs/**`、`AGENTS.md` 等边界外路径

---

## 核心工程要求

1. **底座导入与许可留痕（`0x2E/fusion` MIT `v1.3.0`）**：
   - 将 `0x2E/fusion`（`v1.3.0`）`backend/`、`frontend/`、根 `Dockerfile`、`scripts.sh`、`.gitignore` 导入并整理至 `news_panel` 根目录；
   - 完整保留上游 MIT `LICENSE`，并在根目录 `LICENSE` / `NOTICE` 中保留 `0x2E/fusion` 原版权声明与 `news_panel` 独立演进归属声明。

2. **SQLite 扩展迁移（兼容原有 RSS 条目）**：
   - 沿用 `backend/internal/store/migrations/` 显式 SQLite 迁移与 raw SQL 访问风格；
   - 在 `items` 表新增扩展字段：`source_id`、`external_id`、`domain`、`ai_score`、`ai_tags`、`ai_summary`、`priority`、`triage_status`（均设合理默认值，确保既有 RSS `groups` / `feeds` / `items` 迁移与写入完全兼容、不崩溃）；
   - 为外置入站条目建立 `(source_id, external_id)` **联合幂等唯一索引**（须兼容原有仅靠 `(deleted_at, guid, feed_id)` 去重的 RSS 历史/新增条目，例如使用条件唯一索引 `WHERE source_id != '' AND external_id != ''`）；
   - 新增 `plugin_heartbeats` 状态表（记录 `source_id`、`plugin_version`、`status`、`fetched_count`、`error_message`、`timestamp` / `updated_at` 等），支持按 `source_id` 幂等 upsert。

3. **契约端点与强鉴权实现（严格对齐 `docs/PLUGIN_SPEC.md`）**：
   - **Bearer `INGEST_TOKEN` 鉴权（禁止无 Token 裸奔放行）**：
     - 从配置/环境变量读取 `INGEST_TOKEN`；
     - `/api/v1/ingest` 与 `/api/v1/heartbeat` 必须校验 `Authorization: Bearer <token>`（缺失、格式错误、Token 不匹配或服务端未配置 `INGEST_TOKEN` 时一律拒绝并返回 `401 Unauthorized` / 明确拒绝响应，严禁在未配置 Token 时默认放行）。
   - **`POST /api/v1/ingest`**：
     - 校验 `source_id`、`source_type`、`items` 顶层字段及批大小（`1 <= len(items) <= 100`，空批或超 100 返回 `400 Bad Request`；超大请求体返回 `413 Payload Too Large`）；
     - 逐条校验 `external_id`、`url`、`content`、`published_at`（RFC 3339），执行 `(source_id, external_id)` 幂等落库（跳过已存在条目计入 `deduplicated`），返回 `{"accepted", "deduplicated", "rejected", "errors"}`。
   - **`POST /api/v1/heartbeat`**：
     - 校验 `source_id`、`plugin_version`、`status`（仅允许 `ok` / `degraded` / `error`）、`fetched_count`（`>= 0`）、`timestamp`（RFC 3339），upsert `plugin_heartbeats` 并返回 `{"status": "ok", "server_time": "..."}`。
   - **只读查询端点**：
     - 在现有条目查询接口（或新增只读查询参数/端点）支持返回扩展 AI/分拣字段，支持按 `triage_status` / `domain` / `priority` / `min_score` 过滤；
     - 提供只读插件健康查询端点（如 `GET /api/v1/plugins/heartbeats`）读取 `plugin_heartbeats`。

4. **可插拔 `TriageEvaluator` 接口与 v0.1 最小直通桩（预留 `P1-03`）**：
   - 在核心服务层（非 `backend/internal/ai/**` 目录，如 `backend/internal/triage/` 或 `backend/internal/model/`）定义独立的 `TriageEvaluator` 接口及标准化输入/输出结构；
   - 提供 v0.1 最小直通桩（Pass-through stub）实现：当条目入库触发评估时，在无 `P1-03` AI 模块情况下默认设置 `triage_status = "kept"`（及默认 `domain` / `priority`），使端到端链路立即可运行；
   - 保持接口签名简洁清晰，待后续 `P1-03` 在 `backend/internal/ai/**` 实现 L1 规则 + L2 OpenAI-compatible LLM 后可直接注入替换。

5. **前端最小可见展示（预留 `P1-04`）**：
   - 更新 `frontend/src/lib/api/types.ts`、`frontend/src/queries/**` 及 `frontend/src/routes/**`（含基础条目列表/详情抽屉），确保能够拉取并展示入站条目的 `domain`、`ai_score`、`ai_tags`、`ai_summary`、`priority`、`triage_status`（最小可读展示即可，如文本/基础标签）；
   - 将独立的高信号卡片组件与多维筛选栏 UI 完整留给后续 `P1-04`（`frontend/src/components/panel/**`），不在本任务中抢占或新建该目录。

---

## 文件边界（Owned / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `backend/**`（**严格排除 `backend/internal/ai/**`**）
- `frontend/**`（**严格排除 `frontend/src/components/panel/**`**）
- 根目录：`Dockerfile`、`scripts.sh`、`.gitignore`、`LICENSE`、`NOTICE`
- *(注：开始与结束时允许仅更新 `tasks/P1-01-core-fusion-base.md` 自身状态/回执区及 `tasks/INDEX.md` 中 `P1-01` 对应行)*

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/HANDOFF.md`、`docs/DECISIONS.md`、`docs/PLUGIN_SPEC.md`、`docs/PRD.md`、`docs/ROADMAP.md`、`tasks/README.md`

### Forbidden / Out-of-Scope（严禁触碰）
- `plugins/**`（归 `P1-02` ZCode）
- `backend/internal/ai/**`（预留给 `P1-03` ZCode）
- `frontend/src/components/panel/**`（预留给 `P1-04` ZCode）
- `AGENTS.md`、`docs/**`、`tasks/README.md`、`tasks/_TEMPLATE.md`
- `docker-compose.yml`、`.env.example`、`scripts/smoke_v01.*`（后续独立任务范围）

---

## 验收标准与验证命令（Acceptance Criteria & Verification）
- [ ] 已保留上游 `0x2E/fusion` MIT 许可与版权归属声明（根目录 `LICENSE` / `NOTICE`），无任何 GPL/AGPL 依赖
- [ ] Go 单元测试全部通过，至少覆盖：
  - `401 Unauthorized`（缺 Token、错 Token、服务未配置 `INGEST_TOKEN` 时禁止裸奔放行）
  - `400 Bad Request`（非法 JSON、缺必填字段、`items` 为空或 `> 100`、非法 `heartbeat` 状态/负数计数）
  - `(source_id, external_id)` 幂等去重入库 + 原有 RSS 条目读写兼容
  - `POST /api/v1/heartbeat` upsert 与只读状态查询
  - `TriageEvaluator` 直通桩调用并回写 `items` 分拣字段
- [ ] Go 二进制编译成功（含内嵌前端构建产物），前端类型检查与打包构建成功
- [ ] `git status` / `git diff --name-only` 确认所有变更 100% 落入 `Owned Files`，未触碰任何 `Forbidden Files`

建议核验命令（*若运行环境中缺少 Go / Node / Docker 某项工具链，须在 Handoff Report 如实标注实际可跑环境与真实输出，严禁虚构结果*）：
```text
cd backend
go test -v ./...
go build -o /dev/null ./cmd/fusion

cd ../frontend
# 按导入底座实际包管理器（如 pnpm / npm）执行安装、类型检查与构建：
# tsc -b --noEmit && vite build（或执行根目录 ./scripts.sh）
```

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`IN_REVIEW` | `BLOCKED`
- **执行端 / 通道**：`通道 B / CodexHost (Codex: gpt 6.1 sol high)`
- **实际改动文件清单**：
  - `<待填写>`
- **实测验证命令与观察结果**（仅填写最终修改后亲自跑过并观察到的真实结果）：
  - 命令：`...` → 结果：`...`
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - `<待填写>`
