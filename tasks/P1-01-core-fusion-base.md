# `P1-01`：核心底座深度重构与 `/api/v1` 契约贯通（基于 `0x2E/fusion`）

## 元信息（Metadata）
- **Task ID**：`P1-01`
- **Phase / Milestone**：`Phase 1 (v0.1 纵向切片)`
- **前置门禁（Gate Dependency）**：`Gate 0 已于 2026-11-15 正式批准`（已完成 `0x2E/fusion` 模块拆分调研，允许在授权目录内引入底座代码并实施重构）
- **当前状态（Status）**：`IN_REVIEW`
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
- **提交状态**：`IN_REVIEW`
- **执行端 / 通道**：`通道 B / CodexHost / Codex`
- **工作分支**：`feat/p1-01-core-fusion-base`；仅在本独立 worktree 实现并提交，未修改 `main` 或外部插件工作区。
- **实现结果**：
  - 导入 Fusion v1.3.0（`e90198ae06d03d5820d97c126c161d411fa24313`），Go 模块改为 `news_panel`，保留 `cmd/fusion` 和既有 `FUSION_*` 配置兼容性。保留完整上游 MIT 文本及版权，补充 NOTICE、第三方许可与字体许可；镜像包含许可文件。
  - 迁移 006 新增插件键、AI/分拣字段、原始作者/元数据/完整 UTC 发布时间与心跳表。保留上游实际的 RSS `(feed_id, guid)` 唯一索引；新部分索引只覆盖非空插件键。v1.3.0 实际 schema 无 `deleted_at`，没有为对齐文字描述而改造历史去重机制。
  - `/api/v1/ingest` 与 `/api/v1/heartbeat` 使用独立、默认拒绝的 Bearer 鉴权，未配置 token 亦返回 401。严格校验键名、JSON、批量 1..100、逐条字段；2 MiB 上限返回 413，批内错误独立统计。
  - 入库后执行可注入的 `triage.TriageEvaluator`；默认直通为 kept/other/normal，评分为空，标签与摘要为空。失败结果存为 failed。重复的 pending 条目使用原始存储信封恢复评估，已完成重复条目不再执行评估。
  - 普通与 `/api/v1` 条目读取别名、心跳读取沿用 reader session 权限；插件 token 不授予 reader 权限。条目与收藏支持领域、状态、优先级、最低分过滤，分页与总数一致。收藏读取关联条目的真实 triage，孤立快照明确标识无 triage。
  - React 19 既有列表/详情显示字段；默认条目视图 kept，URL 查询支持显式筛选。插件正文按文本呈现，RSS 继续使用原有 HTML 清理。未创建 AI 或 panel 保留目录。
- **实际改动文件清单**（共 216 个路径，含导入文件，逐项列于下方）：
  - `.gitignore`
  - `Dockerfile`
  - `LICENSE`
  - `NOTICE`
  - `backend/README.md`
  - `backend/THIRD_PARTY_NOTICES.txt`
  - `backend/cmd/fusion/main.go`
  - `backend/go.mod`
  - `backend/go.sum`
  - `backend/internal/auth/oidc.go`
  - `backend/internal/auth/password.go`
  - `backend/internal/auth/password_test.go`
  - `backend/internal/config/config.go`
  - `backend/internal/config/config_test.go`
  - `backend/internal/config/ingest_test.go`
  - `backend/internal/handler/bookmark.go`
  - `backend/internal/handler/bookmark_test.go`
  - `backend/internal/handler/cors_test.go`
  - `backend/internal/handler/errors.go`
  - `backend/internal/handler/feed.go`
  - `backend/internal/handler/feed_refresh_test.go`
  - `backend/internal/handler/feed_test.go`
  - `backend/internal/handler/feed_validate_test.go`
  - `backend/internal/handler/fever.go`
  - `backend/internal/handler/fever_test.go`
  - `backend/internal/handler/frontend.go`
  - `backend/internal/handler/frontend_test.go`
  - `backend/internal/handler/group.go`
  - `backend/internal/handler/handler.go`
  - `backend/internal/handler/handler_test.go`
  - `backend/internal/handler/item.go`
  - `backend/internal/handler/item_test.go`
  - `backend/internal/handler/logging_middleware.go`
  - `backend/internal/handler/oidc.go`
  - `backend/internal/handler/plugin.go`
  - `backend/internal/handler/plugin_test.go`
  - `backend/internal/handler/search.go`
  - `backend/internal/handler/search_test.go`
  - `backend/internal/handler/session.go`
  - `backend/internal/handler/session_limiter_test.go`
  - `backend/internal/handler/session_test.go`
  - `backend/internal/handler/settings.go`
  - `backend/internal/handler/settings_test.go`
  - `backend/internal/handler/test_helpers_test.go`
  - `backend/internal/model/model.go`
  - `backend/internal/pkg/httpc/client.go`
  - `backend/internal/pkg/httpc/ssrf.go`
  - `backend/internal/pkg/httpc/ssrf_test.go`
  - `backend/internal/pull/parser.go`
  - `backend/internal/pull/parser_test.go`
  - `backend/internal/pull/puller.go`
  - `backend/internal/pull/puller_test.go`
  - `backend/internal/pullpolicy/schedule.go`
  - `backend/internal/pullpolicy/schedule_test.go`
  - `backend/internal/store/bookmark.go`
  - `backend/internal/store/bookmark_test.go`
  - `backend/internal/store/convert.go`
  - `backend/internal/store/errors.go`
  - `backend/internal/store/feed.go`
  - `backend/internal/store/feed_test.go`
  - `backend/internal/store/group.go`
  - `backend/internal/store/group_test.go`
  - `backend/internal/store/item.go`
  - `backend/internal/store/item_test.go`
  - `backend/internal/store/migrate.go`
  - `backend/internal/store/migrate_legacy.go`
  - `backend/internal/store/migrate_legacy_copy.go`
  - `backend/internal/store/migrate_legacy_detect.go`
  - `backend/internal/store/migrate_legacy_fileops.go`
  - `backend/internal/store/migrate_legacy_parse.go`
  - `backend/internal/store/migrate_legacy_schema.go`
  - `backend/internal/store/migrate_test.go`
  - `backend/internal/store/migrations/001_initial.sql`
  - `backend/internal/store/migrations/002_feed_fetch_state.sql`
  - `backend/internal/store/migrations/003_bookmark_feed_id.sql`
  - `backend/internal/store/migrations/004_settings.sql`
  - `backend/internal/store/migrations/005_auto_mark_read.sql`
  - `backend/internal/store/migrations/006_plugin_ingest.sql`
  - `backend/internal/store/plugin.go`
  - `backend/internal/store/plugin_test.go`
  - `backend/internal/store/settings.go`
  - `backend/internal/store/settings_test.go`
  - `backend/internal/store/store.go`
  - `backend/internal/store/store_test.go`
  - `backend/internal/triage/triage.go`
  - `backend/internal/triage/triage_test.go`
  - `backend/internal/web/assets.go`
  - `backend/internal/web/dist/.keep`
  - `backend/internal/web/fallback/index.html`
  - `frontend/.gitignore`
  - `frontend/components.json`
  - `frontend/eslint.config.js`
  - `frontend/index.html`
  - `frontend/package.json`
  - `frontend/pnpm-lock.yaml`
  - `frontend/pnpm-workspace.yaml`
  - `frontend/public/apple-touch-icon.png`
  - `frontend/public/favicon.ico`
  - `frontend/public/icon-192.png`
  - `frontend/public/icon-512.png`
  - `frontend/public/icon-96.png`
  - `frontend/public/licenses/geist-OFL.txt`
  - `frontend/public/licenses/geist-mono-OFL.txt`
  - `frontend/public/licenses/inter-OFL.txt`
  - `frontend/public/manifest.json`
  - `frontend/public/sw.js`
  - `frontend/scripts/check-i18n.mjs`
  - `frontend/src/components/article/article-drawer.tsx`
  - `frontend/src/components/article/article-item.tsx`
  - `frontend/src/components/article/article-list.tsx`
  - `frontend/src/components/article/article-page.tsx`
  - `frontend/src/components/article/triage-metadata.tsx`
  - `frontend/src/components/feed/add-feed-dialog.tsx`
  - `frontend/src/components/feed/edit-feed-dialog.tsx`
  - `frontend/src/components/feed/feed-favicon.tsx`
  - `frontend/src/components/feed/feed-group-card.tsx`
  - `frontend/src/components/feed/feed-group.tsx`
  - `frontend/src/components/feed/feed-item.tsx`
  - `frontend/src/components/feed/feed-list.tsx`
  - `frontend/src/components/feed/import-opml-dialog.tsx`
  - `frontend/src/components/group/add-group-dialog.tsx`
  - `frontend/src/components/layout/app-layout.tsx`
  - `frontend/src/components/layout/content-header.tsx`
  - `frontend/src/components/layout/shortcuts-dialog.tsx`
  - `frontend/src/components/layout/sidebar-trigger.tsx`
  - `frontend/src/components/layout/sidebar.tsx`
  - `frontend/src/components/search/search-dialog.tsx`
  - `frontend/src/components/settings/settings-dialog.tsx`
  - `frontend/src/components/ui/avatar.tsx`
  - `frontend/src/components/ui/button.tsx`
  - `frontend/src/components/ui/collapsible.tsx`
  - `frontend/src/components/ui/command.tsx`
  - `frontend/src/components/ui/dialog.tsx`
  - `frontend/src/components/ui/dropdown-menu.tsx`
  - `frontend/src/components/ui/input-group.tsx`
  - `frontend/src/components/ui/input.tsx`
  - `frontend/src/components/ui/scroll-area.tsx`
  - `frontend/src/components/ui/select.tsx`
  - `frontend/src/components/ui/separator.tsx`
  - `frontend/src/components/ui/sheet.tsx`
  - `frontend/src/components/ui/sonner.tsx`
  - `frontend/src/components/ui/switch.tsx`
  - `frontend/src/components/ui/tabs.tsx`
  - `frontend/src/components/ui/textarea.tsx`
  - `frontend/src/components/ui/tooltip.tsx`
  - `frontend/src/globals.d.ts`
  - `frontend/src/hooks/use-article-list.ts`
  - `frontend/src/hooks/use-auto-mark-read.ts`
  - `frontend/src/hooks/use-keyboard.ts`
  - `frontend/src/hooks/use-mobile.ts`
  - `frontend/src/hooks/use-pwa-install.ts`
  - `frontend/src/hooks/use-url-state.ts`
  - `frontend/src/index.css`
  - `frontend/src/lib/api/client.ts`
  - `frontend/src/lib/api/favicon.ts`
  - `frontend/src/lib/api/index.ts`
  - `frontend/src/lib/api/types.ts`
  - `frontend/src/lib/article-filter.test.ts`
  - `frontend/src/lib/article-filter.ts`
  - `frontend/src/lib/content.ts`
  - `frontend/src/lib/feed-error.test.ts`
  - `frontend/src/lib/feed-error.ts`
  - `frontend/src/lib/i18n.ts`
  - `frontend/src/lib/i18n/messages/de.ts`
  - `frontend/src/lib/i18n/messages/en.ts`
  - `frontend/src/lib/i18n/messages/es.ts`
  - `frontend/src/lib/i18n/messages/fr.ts`
  - `frontend/src/lib/i18n/messages/index.ts`
  - `frontend/src/lib/i18n/messages/pt.ts`
  - `frontend/src/lib/i18n/messages/ru.ts`
  - `frontend/src/lib/i18n/messages/sv.ts`
  - `frontend/src/lib/i18n/messages/types.ts`
  - `frontend/src/lib/i18n/messages/zh.ts`
  - `frontend/src/lib/opml.ts`
  - `frontend/src/lib/pwa.ts`
  - `frontend/src/lib/query-client.ts`
  - `frontend/src/lib/route-params.ts`
  - `frontend/src/lib/safe-url.test.ts`
  - `frontend/src/lib/safe-url.ts`
  - `frontend/src/lib/settings-sync.ts`
  - `frontend/src/lib/triage-search.test.ts`
  - `frontend/src/lib/triage-search.ts`
  - `frontend/src/lib/utils.ts`
  - `frontend/src/main.tsx`
  - `frontend/src/queries/bookmarks.ts`
  - `frontend/src/queries/feeds.ts`
  - `frontend/src/queries/groups.ts`
  - `frontend/src/queries/items.ts`
  - `frontend/src/queries/keys.ts`
  - `frontend/src/routes/$filter.lazy.tsx`
  - `frontend/src/routes/$filter.tsx`
  - `frontend/src/routes/__root.tsx`
  - `frontend/src/routes/feeds.lazy.tsx`
  - `frontend/src/routes/feeds.tsx`
  - `frontend/src/routes/feeds_.$feedId.$filter.lazy.tsx`
  - `frontend/src/routes/feeds_.$feedId.$filter.tsx`
  - `frontend/src/routes/feeds_.$feedId.tsx`
  - `frontend/src/routes/groups.$groupId.$filter.lazy.tsx`
  - `frontend/src/routes/groups.$groupId.$filter.tsx`
  - `frontend/src/routes/groups.$groupId.tsx`
  - `frontend/src/routes/index.tsx`
  - `frontend/src/routes/login.lazy.tsx`
  - `frontend/src/routes/login.tsx`
  - `frontend/src/store/article-pins.test.ts`
  - `frontend/src/store/article-pins.ts`
  - `frontend/src/store/index.ts`
  - `frontend/src/store/preferences.ts`
  - `frontend/src/store/ui.ts`
  - `frontend/src/typeset.css`
  - `frontend/tsconfig.app.json`
  - `frontend/tsconfig.json`
  - `frontend/tsconfig.node.json`
  - `frontend/vite.config.ts`
  - `scripts.sh`
  - `tasks/INDEX.md`
  - `tasks/P1-01-core-fusion-base.md`
- **实测验证命令与观察结果**（只列亲自执行并观察的命令；完整终端日志保存在本机临时目录，未提交）：
  - `backend/.tools/go/bin/go.exe version` → `go version go1.26.0 windows/amd64`。系统 PATH 无 Go，使用 worktree 内忽略目录的官方便携工具链。
  - 在 `backend/`，`$env:CGO_ENABLED='0'; ./.tools/go/bin/go.exe test -count=1 -v ./...` → 退出 0，8 个测试包全部通过，141 个顶层测试通过；包括 401（缺失/错误/格式错误/空服务端 token）、400/413、100 条边界、逐条拒绝、精确大小写键名、跨信源/并发幂等、RSS v5 升级兼容、心跳 upsert/读取、分拣注入/失败、取消后按原始亚秒时间恢复、收藏真实字段与过滤。
  - `docker build --target backend -t news-panel:p1-01-builder .`；`docker run --rm news-panel:p1-01-builder sh -c 'CGO_ENABLED=0 go test -count=1 ./...'` → Linux Go 1.26 Alpine 中全部 8 个测试包通过。
  - 在 `frontend/`，`npx --yes pnpm@10 install --frozen-lockfile` → pnpm 10.34.9 安装完成；锁文件未修改。
  - 在 `frontend/`，`npx --yes pnpm@10 run typecheck` → `tsr generate && tsc -b --noEmit` 退出 0；`npx --yes pnpm@10 run build` → `tsr generate && tsc -b && vite build` 成功。
  - 在 `frontend/`，`npx --yes pnpm@10 run test` → 5 个文件、26 个测试通过；`npx --yes pnpm@10 run check:i18n` → 全部字典完整（189 keys）。
  - 同步 `frontend/dist` 到 `backend/internal/web/dist` 后，`$env:CGO_ENABLED='0'; ./backend/.tools/go/bin/go.exe -C backend build -trimpath -o .verify/news_panel.exe ./cmd/fusion`，再设置 `GOOS=linux, GOARCH=amd64` 编译 `.verify/news_panel-linux-amd64` → 两个内嵌 SPA 二进制成功，分别 41,618,432 / 41,303,544 bytes；构建产物不提交。
  - `docker build -t news-panel:p1-01 .` → 多阶段镜像构建成功；初期 Docker engine 未就绪，恢复后最终构建成功。
  - 启动两个临时容器（显式匿名 reader，仅一个配置测试用 INGEST_TOKEN），`python backend/.verify/live_check.py` → PASS：真实 HTTP 401（含未配置服务端 token）、accepted/deduplicated、stub kept 回写、完整 UTC 亚秒时间、过滤读取、心跳、内嵌 HTML/JS/字体许可。此脚本为本地忽略目录验证辅助文件。
  - `docker inspect news-panel-p101-check --format '{{.State.Health.Status}} {{.Config.User}}'` → `healthy news_panel`；`docker stats --no-stream --format '{{.Name}} {{.MemUsage}}' news-panel-p101-check` → 当次 stub 空闲观测 `6.125MiB / 7.369GiB`，不是实际 LLM 或长期负载测量。临时容器已停止并自动清除。
  - `docker run --rm --entrypoint sh news-panel:p1-01 -c 'test -s /usr/share/licenses/news_panel/THIRD_PARTY_NOTICES.txt && id -u'` → 退出 0，UID `100`，许可已包含于镜像。
  - `go list -deps` 对实际编译的 36 个 Go 外部模块检查根许可文件 → 无缺失、无 GPL/AGPL/LGPL 许可文本匹配；安装后检查 513 个 npm 版本声明 → 无缺失、无 GPL/AGPL/LGPL 声明。另执行 `go mod download`、`go list -m -json all` 与 `pnpm licenses list --prod --json`，收集 47 个 Go 模块、103 个生产 npm 条目的许可文本到 `backend/THIRD_PARTY_NOTICES.txt`。MPL 文本对 GNU 次级许可证的引用不代表引入 GNU 授权代码。
  - 对指定上游提交的 LICENSE 逐字前缀比对 → PASS，原 MIT 文本与版权完整保留。
  - `git diff --cached --check` → 退出 0；逐路径 ownership 审计 → 216 个 staged 路径全部在 Owned Files，Forbidden 命中 0；另核对 INDEX 仅 P1-01 行、任务卡仅状态/回执改变。
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - 无边界外变更申请。`plugins/**`、`backend/internal/ai/**`、`frontend/src/components/panel/**`、`docs/**`、根 compose/env/smoke 文件均未修改。P1-02、P1-03、P1-04 及 Gate 1 验收仍由后续任务负责，本任务只提供直通 evaluator 和最小字段展示。
  - 在请求/进程中断后，pending 恢复依赖信源重试；没有后台扫描队列。并发重试可能对同一 pending 条目重复评估，但持久化键保持唯一；P1-03 evaluator 应容忍至少一次调用并控制 LLM 超时。配置注入位置、API 与存储说明见 `backend/README.md`。
  - 浏览器视觉检查尝试失败：browser access host unavailable。未声称截图或真实浏览器交互验收通过；已验证内嵌 HTTP 资产、类型检查与前端构建。
  - 上游依赖包含 MPL-2.0 Lightning CSS 构建工具、OFL 字体；未发现 GPL/AGPL 声明。DOMPurify 选择 Apache-2.0。9 个 npm tarball 缺少独立许可文件，使用对应上游仓库许可并记录 URL；其中 react-remove-scroll-bar 的 registry gitHead 不可用，保留 upstream master 的 MIT 许可与公开版权归属。许可检查是元数据/许可文件检查，未作全部源码逐行审计。
  - 提交使用经 `gh api user` 核实账号生成的 GitHub noreply 身份；不使用当前配置的私人邮箱，提交消息无邮箱或 Co-authored-by trailer。完成后核验 author/committer 和消息，再由最终回复返回 commit id；不推送、不合并 main。
