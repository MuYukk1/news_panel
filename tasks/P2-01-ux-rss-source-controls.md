# `P2-01`：阅读抽屉流畅优化、RSS 未打分归类、订阅源/插件开关、来源筛选与 OPML 多选导入

## 元信息（Metadata）
- **Task ID**：`P2-01`
- **Phase / Milestone**：`Phase 2 (v0.2 体验收口与来源控制)`
- **前置门禁（Gate Dependency）**：`Gate 1 已于 2026-11-20 正式批准（DEC-006 / DEC-007 已锁定）`
- **当前状态（Status）**：`TODO`（可选：`TODO` | `IN_PROGRESS` | `IN_REVIEW` | `DONE` | `BLOCKED`）
- **调度通道与目标执行端（Channel / Runner）**：`通道 A / 通道 B 均可（单执行端负责全栈小切片，避免前后端接口错位）`
- **工作区 / 分支策略（Worktree / Branch）**：**严禁直接在 `main` 提交代码**。须在独立分支或 worktree 执行：
  - `git checkout -b feat/p2-01-ux-rss-source-controls`（或 `git worktree add ../news_panel-p2-01 -b feat/p2-01-ux-rss-source-controls`）
- **安全回滚锚点**：基线提交 `5925b42`（对应 GHCR 不可变回滚镜像 `sha-5925b42`）。本任务仅包含纯增量、向后兼容的 SQLite 迁移 `007_source_controls.sql`，不重建 `items` 表、不破坏任何既有 `/api/v1/ingest` 或 `/api/v1/heartbeat` 契约。

---

## 目标与上下文（Objective & Context）
- **核心目标**：集中解决 v0.1 线上实战验收中暴露的 5 项高频体验与 RSS 兼容痛点，为后续 `P2-02`（三级 AI 处理管线）与 `P2-03`（用户反馈闭环）打好干净的数据分桶与交互底座：
  1. 修复阅读抽屉打开/关闭卡顿，以及关闭时「正文瞬间消失、空壳再滑出」的闪烁缺陷；
  2. 按 `DEC-007` 方案 A 修复原生 RSS 兼容：未打分条目归入独立「未打分 (`unscored`)」分桶，不再污染「AI 精选放行 (`kept`)」与首页 Top Picks，并为 RSS 源预埋单源 `ai_enabled` 开关（默认关闭，供 `P2-02` 消费）；
  3. 在订阅管理 `/feeds` 与来源状态 `/sources` 为每个 RSS 订阅源及外置插件提供统一的液态玻璃胶囊启停开关；
  4. 在资讯流高级筛选面板增加「按来源（外置插件 / 具体 RSS 源）」筛选并与 URL 同步；
  5. 将多源探测弹窗与 OPML 导入升级为**多选批量导入**，并原生支持直接粘贴远程 `.opml` 链接（含 GitHub `blob` 链接自动转 `raw`，如 `https://github.com/xiangyugongzuoliu/awesome-rss-feeds-list/blob/main/feeds/cn-ai-research.opml`）。
- **前置输入 / 依赖任务**：
  - Phase 1（`P1-01`–`P1-04`）已全部 `DONE` 并合入 `main`（`5925b42`）。
  - 冻结的 v1 插件契约见 [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)，Gate 1 拍板决策见 [docs/DECISIONS.md](../docs/DECISIONS.md)（`DEC-006`、`DEC-007`）。

---

## 关联文档与约束（References & Constraints）
开始修改代码前必须按顺序阅读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)（重点核对 `DEC-006`、`DEC-007`）
4. [docs/PRD.md](../docs/PRD.md)
5. [docs/ROADMAP.md](../docs/ROADMAP.md)
6. [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)
7. [tasks/README.md](./README.md)

本任务硬性约束：
- [x] **严守 Gate 1 与 `P2-02` / `P2-03` 边界**：**禁止**在本任务中改造 `backend/internal/ai/**` 评分管线、**禁止**提前实现三级 Decision 模型、反馈表或 Telegram 插件。
- [x] **零破坏性 SQLite 迁移**：禁止重建 `items` 表修改 `triage_status` 的 `CHECK` 约束；仅通过新增 `backend/internal/store/migrations/007_source_controls.sql` 做纯增量 `ALTER TABLE ... ADD COLUMN ... DEFAULT ...`。
- [x] **保持 v1 插件契约向后兼容**：现有 `plugins/x-twifork` 无需改动即可继续工作；`POST /api/v1/heartbeat` 响应可向后兼容新增 `"paused": boolean`。
- [x] **SSRF 防护不可绕过**：后端拉取远程 OPML URL（含 GitHub `blob` 转 `raw`）必须经过现有 `httpc.ValidateRequestURL` + `httpc.NewClient` + 响应体积上限（如 2 MiB）。
- [x] **零新增前端重型依赖 & 八语种 i18n 完整**：禁止修改 `frontend/package.json`；所有新增前端文案键必须同步至 `frontend/src/lib/i18n/messages/{en,zh,de,es,fr,pt,ru,sv}.ts` 并通过 `pnpm run check:i18n`。

---

## 功能与工程详规（Detailed Spec）

### 模块 1：阅读抽屉流畅动画与关闭不闪白（Frontend）
- **根因定位**：
  - [article-drawer.tsx](../frontend/src/components/article/article-drawer.tsx#L275-L281) 在关闭瞬间 `selectedArticleId` 变为 `null`，导致 `{article && ...}` 第 0 帧卸载正文，而 [sheet.tsx](../frontend/src/components/ui/sheet.tsx#L56) 仍在播放 200ms 退场过渡；
  - `SheetContent` 使用全属性 `transition` 且叠加 `backdrop-filter: blur(22px)` 导致位移期间掉帧。
- **实现要求**：
  1. 在 `ArticleDrawer` 中缓存最近一条非空展示条目 `displayArticle`（`open` 为 `true` 时实时更新；`open` 变为 `false` 关闭退场期间继续渲染 `displayArticle`，在退场结束如 `onOpenChangeComplete` 或 240ms 定时清理后再释放，避免任何「信息先消失、空框后关闭」现象）。
  2. 优化 `SheetContent` 与 `SheetOverlay`（[sheet.tsx](../frontend/src/components/ui/sheet.tsx)）及 `.np-drawer-glass`（[panel.css](../frontend/src/components/panel/panel.css)）：
     - 将 `transition` 收敛为 `transition-[transform,opacity]`（合成层 GPU 加速，`will-change: transform, opacity`）；
     - 遮罩层 `SheetOverlay` 移除高开销实时 `backdrop-blur-xs`（或改为轻半透明遮罩 `bg-black/15 dark:bg-black/35`），抽屉主体适当降低实时模糊半径（如 `blur(14px)` + 稍高底色不透明度），确保开关帧率顺滑且视觉仍保持统一浅色磨砂玻璃质感；
     - 保留 `@media (prefers-reduced-motion: reduce)` 兼容，不破坏 `useAutoMarkRead`、`pinRead`/`unpinRead` 与 `j / k / Esc / s / o` 键盘快捷键。

---

### 模块 2：原生 RSS 未打分归类与单源 `ai_enabled` 预埋（Backend + Frontend）
- **后端改动**：
  1. 新增迁移 `backend/internal/store/migrations/007_source_controls.sql`：
     ```sql
     ALTER TABLE feeds ADD COLUMN ai_enabled INTEGER NOT NULL DEFAULT 0;
     ALTER TABLE plugin_heartbeats ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
     ```
  2. `model.Feed` 新增 `AIEnabled bool` (`json:"ai_enabled"`)；`store` 的 `CreateFeed`/`GetFeed`/`ListFeeds`/`UpdateFeed` 与 `handler` 的 `updateFeedRequest` 支持读写 `ai_enabled`（默认 `false`，保持 `DEC-007` 方案 A：原生 RSS 默认不走 AI）。
  3. 扩展 `GET /api/items`（及 `/api/v1/items`、`GET /api/bookmarks`）查询参数（[item.go](../backend/internal/handler/item.go)、[plugin.go](../backend/internal/store/plugin.go)）：
     - 新增可选参数 `scored`（`"true"` → `ai_score IS NOT NULL`；`"false"` → `ai_score IS NULL`；非法值返回 `400`）；
     - `triage_status` 查询参数除既有 `pending | kept | filtered | failed` 外，新增虚拟查询别名 **`unscored`**，映射到 SQL：`ai_score IS NULL AND triage_status NOT IN ('filtered', 'failed')`（兼容 `bookmarks` 左连接场景）；
     - **严格向后兼容**：不传 `scored` 且传 `triage_status=kept` 时，保持原 SQL 语义不变，确保所有既有后端单测无需修改即全绿。
- **前端改动**：
  1. `TriageStatusFilter`（[triage-search.ts](../frontend/src/lib/triage-search.ts)）新增 `"unscored"`，面板筛选分段切换升级为：**`AI 精选 (kept)` / `未打分 (unscored)` / `已拦截 (filtered)` / `全部 (all)`**（`pending` 如存在仍兼容 URL 解析，并自然包含在 `unscored` 视口中）。
  2. **严格区分「AI 精选」与「未打分」**：
     - 选 `triage_status="kept"` 时，请求后端附带 `scored: true`（仅返回真正经过 AI 打分放行的条目）；
     - 选 `triage_status="unscored"` 时，请求 `triage_status: "unscored"`（集中展示所有未打分的原生 RSS 与待处理条目）；
     - 首页 `/overview` 的「AI 精选放行」KPI、Top Picks、Domain Pulse 均只统计 `triage_status="kept"&scored=true`（不让未打分 RSS 混进 AI 精选或拉偏平均分）；侧栏 AI 分拣区增加**「未打分 RSS / 待评」**快捷入口（跳转 `/all?triage_status=unscored`，带真实计数徽章）。
  3. 条目卡片与抽屉中：当 `article.ai_score === null && article.triage_status !== "filtered"` 时，渲染清晰温和的「未打分」徽章（替代空白评分条或误导性的常规 AI 放行态）。

---

### 模块 3：订阅源 & 外置插件液态玻璃启停开关（Backend + Frontend）
- **视觉设计（统一液态玻璃风格）**：
  - 在 `frontend/src/components/panel/` 新增轻量无障碍胶囊开关 `<GlassSwitch />`（`role="switch"`, `aria-checked`, 禁用态与加载态防连点）：
    - 开启态：半透明冰蓝/靛紫渐变轨道（`var(--glass-acc)` 柔光）+ 带内高光的白色圆钮滑块；
    - 关闭态：浅灰磨砂玻璃轨道 + 柔和描边，对应行条目轻降不透明度显示「已暂停」标签。
- **原生 RSS 订阅源开关（`/feeds`）**：
  - 在 [feed-group-card.tsx](../frontend/src/components/feed/feed-group-card.tsx) 每条 RSS 源右侧直接放置：
    1. **启停主开关**（`<GlassSwitch />`，直连现有 `PATCH /api/feeds/:id` `{ suspended: !checked }`，带乐观更新/toast 反馈）；
    2. **AI 评分小开关/芯片**（切换 `PATCH /api/feeds/:id` `{ ai_enabled }`，默认关闭，Tooltip 提示「开启后由 AI 管线打分，默认归入未打分不耗额度」）。
- **外置插件启停开关（Backend + `/sources` + `/feeds`）**：
  - **后端**：
    - `store.PluginHeartbeat` 增加 `Enabled bool`（`json:"enabled"`），`UpsertHeartbeat` 冲突更新时**不覆盖**已有 `enabled` 值；
    - 新增受鉴权管理接口 `PATCH /api/v1/plugins/:source_id`（请求体 `{"enabled": boolean}`），更新 `plugin_heartbeats.enabled` 并同步更新对应影子源 `urn:news_panel:source:<source_id>` 的 `feeds.suspended = !enabled`；
    - `POST /api/v1/ingest` 在处理前检查该 `source_id` 是否已暂停：若 `enabled == false`，直接返回 `200 OK`（`accepted: 0, deduplicated: 0, rejected: 0, errors: []`）且不落库、不触发 AI 推理（保护额度）；
    - `POST /api/v1/heartbeat` 返回体增加向后兼容字段 `"paused": !enabled`（供后续外置插件识别并自动休眠）。
  - **前端**：
    - 在 `/sources` 的每个外置插件卡片头部，以及 `/feeds` 页顶部的「外置采集插件」精简玻璃卡片区，均展示 `<GlassSwitch />` 控制插件启停（调用 `PATCH /api/v1/plugins/:source_id`），暂停时卡片明确标注「已暂停接收」，点击即时生效并刷新缓存。

---

### 模块 4：高级筛选增加「按来源筛选」（Frontend）
- 扩展 [triage-search.ts](../frontend/src/lib/triage-search.ts) 新增 `feed_id?: number`（仅接受正整数，非法值丢弃）；
- 更新 [use-article-list.ts](../frontend/src/hooks/use-article-list.ts) 与 `ArticleList`：在 `/$filter`（`/all`、`/unread`、`/starred`）视图中，将 `search.feed_id` 透传给 `useItems` / `useStarredItems`（若当前已在 `/feeds/$feedId/$filter` 路由下，仍以路径 `selectedFeedId` 优先）；
- 在 [triage-toolbar.tsx](../frontend/src/components/panel/triage-toolbar.tsx) 可折叠「高级筛选」面板中新增**「来源 Source」**选择器：
  - 选项分为：`全部来源`、`外置插件` 组（识别 `isPluginSourceFeed(feed.link)`）、`RSS 订阅源` 组；
  - 计入 `countActiveAdvancedFilters()` 角标计数，点击「重置筛选」时一并清除 `feed_id`。

---

### 模块 5：OPML & 多源探测多选批量导入（Backend + Frontend）
- **后端增强 `POST /api/feeds/validate`**（[feed.go](../backend/internal/handler/feed.go)）：
  - 扩展 `discoveredFeed` 可选字段：`SiteURL string` (`json:"site_url,omitempty"`)、`GroupName string` (`json:"group_name,omitempty"`)（向后兼容）；
  - 识别 GitHub `blob` 页面 URL（如 `https://github.com/<owner>/<repo>/blob/<ref>/<path>`，特别是 `.opml` / `.xml`），自动改写为 `https://raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>` 后再走 `httpc.ValidateRequestURL` + `httpc.NewClient`（限制响应体 ≤ 2 MiB）抓取；
  - 若目标内容为有效 OPML（根元素 `<opml>`），直接解析其中全部带 `xmlUrl` 的 `<outline>`（保留嵌套分类作为 `group_name`、`htmlUrl` 作为 `site_url`、去重 `link`），以 `validateFeedResponse{Feeds: [...]}` 返回；否则回退原有 `feedfinder` / `gofeed` 探测流程。
- **前端 `AddFeedDialog`（[add-feed-dialog.tsx](../frontend/src/components/feed/add-feed-dialog.tsx)）**：
  - 当探测结果 `feeds.length > 1` 时，弹出**多选批量添加列表**：
    - 支持搜索框按名称/URL 过滤、全选/取消全选、逐项复选框勾选（默认全选或提供一键全选，清晰展示「已选 M / 共 N 条」及来源分组标签）；
    - 若目标源带 `group_name`，允许选择「按 OPML 分组自动创建」或统一导入到指定分组；
    - 点击「批量添加 (M)」调用现有 `feedAPI.batchCreate`（若需自动建组则复用与 `ImportOpmlDialog` 一致的建组逻辑），并 toast 汇报成功/跳过数量。
- **前端 `ImportOpmlDialog`（[import-opml-dialog.tsx](../frontend/src/components/feed/import-opml-dialog.tsx)）**：
  - 支持两种输入方式切换：**「上传本地 OPML 文件」** 与 **「输入远程 OPML 链接（支持 GitHub blob 链接）」**（调用后端 `feedAPI.validate` 拉取解析，避免浏览器直连 CORS 问题）；
  - 解析完成后不再盲目一键全导入，而是展示**多选预览勾选列表**（默认全选，支持搜索、全选/取消全选、按分组/单条取消勾选），确认后再调用 `feedAPI.batchCreate` 仅导入用户选中的订阅源。

---

## 文件边界（Owned / Read-Only / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `backend/internal/store/migrations/007_source_controls.sql`（新建）
- `backend/internal/model/model.go`
- `backend/internal/store/{feed.go,item.go,bookmark.go,plugin.go,store_test.go,plugin_test.go,item_test.go,feed_test.go}`（以及同目录新增单测文件）
- `backend/internal/handler/{handler.go,feed.go,item.go,plugin.go,bookmark.go}` 及对应 `*_test.go`（含新建 OPML 解析辅助与测试文件）
- `frontend/src/**`（含 `components/ui/sheet.tsx`、`components/panel/**`、`components/article/**`、`components/feed/**`、`components/layout/**`、`routes/feeds.lazy.tsx`、`lib/api/**`、`lib/triage-search*`、`lib/i18n/messages/*.ts`、`queries/**`、`hooks/**` 及对应 Vitest 单测）
- *交接维护*：认领及结束时仅允许更新本任务卡 `tasks/P2-01-ux-rss-source-controls.md` 状态与 `## Handoff Report`，以及 `tasks/INDEX.md` 中的 `P2-01` 对应行

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/**`、`tasks/README.md`、`tasks/_TEMPLATE.md`
- `backend/internal/triage/**`、`backend/internal/ai/**`、`plugins/**`、`deploy/**`

### Forbidden / Out-of-Scope（严禁触碰）
- 未列在 `Owned Files` 内的所有其他文件
- **高风险禁区**：`backend/internal/ai/**`、`backend/cmd/**`、`plugins/**`、`deploy/**`、`Dockerfile`、`frontend/package.json`、`Temp/**`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）

- [ ] **抽屉动画流畅无闪白**：关闭阅读抽屉时正文随抽屉完整滑出不提前消失，开关无明显掉帧，且未破坏已读撤回、收藏快照与键盘快捷键。
- [ ] **RSS 未打分归类（方案 A）**：原生 RSS 新入库默认 `ai_enabled=false`、`ai_score=null`，自动归入「未打分 (`unscored`)」视图，不混入「AI 精选 (`kept`)」KPI / Top Picks / Domain Pulse；后端向后兼容不破坏旧单测。
- [ ] **订阅源与外置插件液态玻璃开关**：`/feeds` 可一键启停 RSS 源及切换 `ai_enabled`；`/sources` 与 `/feeds` 可一键启停外置插件，插件被暂停时 `POST /api/v1/ingest` 静默跳过不落库、`POST /api/v1/heartbeat` 返回 `"paused": true`。
- [ ] **高级筛选支持来源筛选**：`TriageToolbar` 高级筛选支持按外置插件或具体 RSS 源过滤，与 URL `?feed_id=` 同步并支持一键重置。
- [ ] **多源探测 & OPML 多选批量导入**：粘贴 GitHub `.opml` `blob` 链接（如 `cn-ai-research.opml`）或上传本地 `.opml` 文件均可解析出列表并支持多选勾选 + 全选/反选 + 批量导入，且受 SSRF 校验保护。
- [ ] **全量后端 + 前端验证通过**（最终修改后亲自执行并如实记录输出）：
  ```bash
  # 1) Go 后端全量测试
  cd backend
  go test ./...

  # 2) 前端 i18n、类型检查、单测与生产构建
  cd ../frontend
  pnpm run check:i18n
  pnpm run typecheck
  pnpm run test
  pnpm run build

  # 3) 文件边界核验
  git diff --name-only
  git status --short
  ```
- [ ] **单测覆盖**：
  - Go 后端新增单测覆盖：`007_source_controls.sql` 迁移、`scored=true|false` & `triage_status=unscored` 查询、插件暂停后 `ingest` 跳过与 `heartbeat` 返回 `paused: true`、GitHub `blob` → `raw` OPML URL 解析与多源返回；
  - 前端 Vitest 新增单测覆盖：`triage-search.ts` 的 `unscored` & `feed_id` 解析、多选 OPML/发现列表辅助逻辑及 API 参数映射。

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`TODO`（执行端完成后改为 `IN_REVIEW` 或 `BLOCKED`）
- **执行端 / 通道**：`<待认领>`
- **实际改动文件清单**：
  - `<待填写>`
- **实测验证命令与观察结果**（仅填写最终修改后亲自跑过并观察到的真实结果）：
  - `<待填写>`
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - `<待填写>`
