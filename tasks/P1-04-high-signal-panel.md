# `P1-04`：统一浅色磨砂液态玻璃高信号情报工作台前端重构

## 元信息（Metadata）
- **Task ID**：`P1-04`
- **Phase / Milestone**：`Phase 1 (v0.1 视觉与高信号交互收口)`
- **前置门禁（Gate Dependency）**：`Gate 0 已批准 + P1-01 / P1-02 / P1-03 已 DONE 且云端 01d3015 闭环跑通`
- **当前状态（Status）**：`DONE`（2026-11-20 已补充双时间显示并通过 `check:i18n` / `typecheck` / `test` / `build` 核验合入 `main`）
- **调度通道与目标执行端（Channel / Runner）**：`通道 A / 手动转交（本地或外部编码模型执行，按本任务卡契约一比一还原与联调）`
- **工作区 / 分支策略（Worktree / Branch）**：**严禁直接在 `main` 提交代码**。推荐隔离方式二选一：
  1. **Worktree 隔离**：`git worktree add ../news_panel-p1-04 -b feat/p1-04-high-signal-panel`（`Temp/` 为未跟踪目录，此时从 `../news_panel/Temp/` 读取只读参考稿）；
  2. **同工作区独立分支（单手转交最省事）**：`git checkout -b feat/p1-04-high-signal-panel`（保留当前 `Temp/` 直接可读，且 `main` 保持不动方便随时切回）。
- **安全回滚锚点**：代码回滚基线 Tag 为 `baseline-pre-p1-04`（指向提交 `01d3015`，对应 GHCR 不可变回滚镜像标签 `sha-01d3015`）。本任务仅改造纯前端，不修改 Go 后端与 SQLite 表结构，任何异常均可零停机、无损退回旧版镜像。

---

## 目标与上下文（Objective & Context）
- **核心目标**：将目前仍偏上游 RSS 阅读器外观的 `frontend/`，深度重构为 `news_panel` 专属的**统一浅色磨砂液态玻璃高信号情报工作台**（支持右上角亮/暗模式切换），让用户登录后第一眼通过「一屏横向 Bento 总览」掌握核心高分要闻与信源健康状态，通过「统一浅色磨砂玻璃侧栏（含 AI 已拦截 `filtered` 快捷入口）」与「可折叠高级筛选栏 + 右侧平滑阅读抽屉」高频消费并核查 AI 清洗质量。
- **前置输入 / 依赖任务**：
  - `P1-01` 已在 `backend/` 落地 SQLite 扩展字段、`/api/v1/items` 过滤查询参数（`triage_status` / `domain` / `priority` / `min_score` / `unread` / `before`）、`/api/v1/plugins/heartbeats` 心跳列表，并在 `frontend/src/lib/api/`、`queries/`、`triage-search.ts` 预留了基础接法。
  - 用户已在仓库本地只读目录 `Temp/` 存放了 5 份视觉与交互参考稿（含截图评论与双份单文件 TSX 样稿），供对照视觉气质、材质变量与交互节奏（**注意：`Temp/` 仅作只读设计参考，严禁修改、删除或 `git add` 提交**）。

---

## 关联文档与约束（References & Constraints）
开始修改代码前必须按顺序阅读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)
4. [docs/PRD.md](../docs/PRD.md)
5. [docs/ROADMAP.md](../docs/ROADMAP.md)
6. [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)（重点核对第 3、4 节返回结构与 `items` 扩展字段）
7. [tasks/README.md](./README.md)

本任务硬性约束：
- [x] **零后端 / 零库表改动**：严禁触碰 `backend/**`、`plugins/**`、`deploy/**`、`Dockerfile`；全部指标与交互必须对齐当前已存在的后端 API（不臆造接口、不修改后端路由）。
- [x] **严禁假数据替代真业务**：总览 KPI、Top Picks、领域分布、插件心跳必须接真实 API（允许空库/加载/报错骨架与优雅空态，**禁止保留 `MOCK_ITEMS`、`MOCK_BEATS`、虚假 `Ask AI` 输入框、虚假股市涨跌幅或编造的 7 日历史柱状图**）。
- [x] **宽松协议与零重型依赖（1C1G VPS 打包兼容）**：保持 `frontend/package.json` 现有依赖体系（React 19 + Tailwind CSS v4 + `@tanstack/react-router` + `@tanstack/react-query` + `zustand` + `lucide-react` + `dompurify`），**禁止引入 ECharts / Chart.js / Recharts / Framer Motion / Playwright 等重型或冗余依赖**；小火花条/心跳条用内联轻量 SVG 或纯 CSS 实现。
- [x] **严格保留安全、鉴权、快捷键与已有路径兼容**：
  - 保留 `/login` 密码及 OIDC 登录跳转与 `routes/__root.tsx` 401 全局守卫、`settings-sync.ts` 远端主题/分页/自动已读同步；
  - 保留外链安全过滤 `toSafeExternalUrl`（`frontend/src/lib/safe-url.ts`）与正文 XSS 清洗 `processArticleContent`（`frontend/src/lib/content.ts`）；
  - 保留 `useArticlePinsStore` 未读撤回置顶逻辑、`useAutoMarkRead` 自动标记已读、`useKeyboardShortcuts` 快捷键、`pnpm run check:i18n` 八语种键同步校验。

---

## 设计仲裁与 UX 规格书（Design Arbitration Spec）

> **致执行模型（重要）**：请严格照本节逐条实现结构、颜色变量与布局密度，不要自由增删虚构卡片，也不要退回纯白直角列表。可同时查阅 `Temp/` 下的两份 TSX 代码与截图获取细节手感。

### 1. 来自 `Temp/` 样稿的取舍裁决表

| 参考资产（仅读） | 必须吸收的部分 | 明确禁止 / 调整的部分 |
| :--- | :--- | :--- |
| `Temp/我比较喜欢这种横向排布...总体概览分类从侧边栏进.png` | 首页横向一屏 Bento 布局节奏、浅冷紫/冰蓝液态玻璃观感、清晰信息层级 | 禁止保留无后端支撑的 `Ask AI` 框、市场涨跌幅、`Topic Map` 7天点击热度、虚假每日综述文本 |
| `Temp/旁边那张图的简略版本...png` | 用作密度下限参考，首页总览密度取「1号图」与「2号图」的**中间档** | 不要留出大块无内容空白 |
| `Temp/很丑但是筛选功能很详细...png` | 与后端 100% 吻合的高级筛选维度（`domain`、`triage_status`、`priority`、`min_score`、`unread`）、详情中的英文原文折叠与 `raw_meta.metrics` 互动指标、插件心跳面板 | **绝不**把厚重筛选边栏直接固定在首页或常态挤占列表；收进「资讯流」页的可折叠筛选条与「来源状态」页 |
| `Temp/整体风格包括布局动效蛮喜欢的...tsx` | 卡片 hover 微上浮（`translateY(-2px)`）、右侧滑入细节抽屉动画（`np-slide-in` + `prefers-reduced-motion`）、`Editor's Pick` 聚焦排版节奏 | **不要用深蓝色实体侧栏**（用户已定调**统一浅色磨砂玻璃侧栏**）；不要把所有卡片垂直堆出超长首页 |
| `Temp/分类蛮喜欢的不过这种底部docker栏...tsx` | **整套 CSS 液态玻璃画布 + 玻璃卡变量 + 弥散阴影**（`--bg`、`--g1`、`--g2`、`--panel`、`--pline`、`--shadow`）、`np-tile` 渐变微拟物图标块、`SignalBars` 10 格分值条、`grid-template-rows: 0fr -> 1fr` 折叠原文动画、视图拆分为「总览 / 资讯流 / 收藏 / 来源状态」 | **桌面端及平板宽度严禁渲染底部 Dock `TabBar`**（继续统一用 `AppLayout` 现有侧栏 / 窄屏 `Sheet` 抽屉）；**严禁照抄其中的 Mock 枚举名 `p0/inbox/score`**，必须映射到真实后端字段 |

---

### 2. 统一视觉系统（CSS Variables & Liquid Glass Tokens）

改造 `frontend/src/index.css`（保持与 `next-themes` / `html.dark` 切换机制完全一致 —— 注意当前项目通过 `.dark` class 与 `:root` 切换明暗主题，**不要改为仅依赖孤立局部 state 的 `data-theme`，确保系统设置弹窗里的主题开关与顶栏一键亮暗切换双向同步**）：

- **画布背景（浅色/深色双光晕渐变）**：
  - 浅色 `:root`：
    - `--bg: #E8EBF9`、`--g1: #D8D4FF`、`--g2: #C6E5FB`
    - `body` 使用复合径向背景：`radial-gradient(1100px 620px at 8% -8%, var(--g1), transparent 62%), radial-gradient(900px 600px at 100% 100%, var(--g2), transparent 60%), var(--bg)`（fixed 或全屏铺底，使左侧侧栏、顶栏与主内容共享同一连续液态玻璃画布）。
    - 玻璃面板变量：
      - `--glass-panel: rgba(255, 255, 255, 0.74)`
      - `--glass-sidebar: rgba(255, 255, 255, 0.62)`（**统一浅色磨砂玻璃侧栏，柔和内描边 + 轻磨砂透出底部冷紫蓝光晕**）
      - `--glass-border: rgba(255, 255, 255, 0.92)`
      - `--glass-inner-line: rgba(70, 80, 160, 0.12)`
      - `--glass-shadow: rgba(60, 64, 160, 0.22)`
      - `--glass-soft: rgba(84, 86, 230, 0.08)`
      - 强调色 `--glass-acc: #5456E6`、正常状态 `--status-ok: #1C9E6A`、紧急 `--prio-critical: #E5484D`、高优 `--prio-high: #E27A14`、常规 `--prio-normal: #2F7FE8`、低优 `--prio-low: #8D95B8`。
  - 深色 `.dark`：
    - `--bg: #0B0D1F`、`--g1: #25206A`、`--g2: #0D3351`
    - `--glass-panel: rgba(25, 28, 58, 0.66)`
    - `--glass-sidebar: rgba(19, 22, 47, 0.72)`
    - `--glass-border: rgba(255, 255, 255, 0.10)`
    - `--glass-inner-line: rgba(255, 255, 255, 0.08)`
    - `--glass-shadow: rgba(0, 0, 0, 0.58)`
    - `--glass-soft: rgba(148, 150, 255, 0.12)`
    - `--glass-acc: #9496FF`、`--status-ok: #47D79B`、紧急 `#FF7479`、高优 `#FFA24D`、常规 `#6AA8FF`。
- **公共玻璃与微动效工具类**（放于 `frontend/src/index.css` 或 `frontend/src/components/panel/panel.css`）：
  - `.np-glass`: `background: var(--glass-panel); border: 1px solid var(--glass-border); border-radius: 22px; backdrop-filter: blur(16px); -webkit-backdrop-filter: blur(16px); box-shadow: 0 14px 34px -18px var(--glass-shadow);`
  - `.np-sidebar-glass`: `background: var(--glass-sidebar); border-right: 1px solid var(--glass-border); backdrop-filter: blur(20px); -webkit-backdrop-filter: blur(20px);`
  - `.np-card-interactive`: hover `transform: translateY(-2px)` 与阴影轻微抬升（`< 220ms`，同时写 `@media (prefers-reduced-motion: reduce)` 禁用位移/动画）。
  - `.np-tile`: `44px–52px` 渐变圆角小图标块（顶部 `inset 0 1px 0 rgba(255,255,255,0.35)` 高光 + 彩色柔投影）。
  - `.np-orig-collapse`: 基于 `grid-template-rows: 0fr -> 1fr` 的 260ms 平滑展开收起。

---

### 3. 路由架构与信息架构（Routes & Navigation）

严格利用当前已接通的 `@tanstack/react-router` 文件路由系统（新增/更新路由后需运行 `pnpm run typecheck` 中的 `tsr generate` 重新生成 gitignored 的 `routeTree.gen.ts`）：

1. **路由定义清单**（保持原有链接与书签不失效）：
   - `/`（`routes/index.tsx`）：重定向或直接默认进入 **`/overview`**（`replace: true`）。
   - `/overview`（新增 `routes/overview.tsx` + `overview.lazy.tsx`）：**总览 Panel 页（一屏横向 Bento）**。
   - `/sources`（新增 `routes/sources.tsx` + `sources.lazy.tsx`）：**采集来源心跳与监控页**（Plugin Heartbeats + RSS/X 源概况）。
   - `/$filter`（保留 `/all`、`/unread`、`/starred`，及 `/feeds/$feedId/$filter`、`/groups/$groupId/$filter`）：**资讯流 / 收藏视图**，承载可折叠高级筛选栏与多状态条目流。
   - `/feeds`（保留）：现有订阅源管理页（嵌套在统一玻璃 `AppLayout` 下，替换硬边框为浅透玻璃风格）。
   - `/login`（保留）：复用登录逻辑，页面背景自然对齐玻璃光晕画布（可轻量化美化登录卡片）。

2. **统一浅色磨砂玻璃侧栏结构（`components/layout/sidebar.tsx` + `components/panel/`）**：
   - 桌面端固定宽 `244px–260px`，采用 `.np-sidebar-glass`（**统一浅色磨砂玻璃，不要做深蓝背景**），内含：
     - **品牌头**：`news_panel` 渐变微拟物图标块 + `个人 AI 情报面板` 副标题；
     - **全局搜索触发器**：保留现有 `Cmd+K / ?` 快捷键弹窗触发器（胶囊磨砂按钮）；
     - **主工作区导航**：
       - `总览 Overview`（`/overview`）
       - `全部资讯 All`（`/all`）
       - `未读 Unread`（`/unread`，带实时未读计数徽章）
       - `收藏夹 Starred`（`/starred`，带真实收藏数）
     - **AI 分拣与领域快捷入口（必须包含用户点名的「已拦截快捷入口」）**：
       - **`已拦截噪声 Filtered` 快捷入口**（高亮提示，点击直接跳转 `/all?triage_status=filtered`，带拦截总数微徽章，方便随时抽查 AI/L1 是否误杀）；
       - **领域快捷过滤入口**：`AI 前沿`（`/all?domain=ai`）、`投资催化`（`/all?domain=investing`）、`羊毛优惠`（`/all?domain=deals`）、`其他`（`/all?domain=other`）；
     - **订阅分组折叠区**：保留原有 `FeedList` 中的分组/源树（样式统一为轻量玻璃激活态，若为外置插件生成的 `urn:news_panel:source:*` 或既有 RSS 源均可在筛选列表中点击切换，避免侧栏杂乱过长）；
     - **运维与底部状态**：
       - `来源状态 Sources`（`/sources`）
       - `订阅管理`（`/feeds`）、`偏好设置`、`退出登录`
       - **底部精简玻璃状态卡**：展示真实心跳健康度（`ok / total` 个采集插件正常），点击跳转到 `/sources`。
   - **移动端响应**：保持 `AppLayout` 中小于 `md` 断点通过顶栏 `SidebarTrigger` 弹出 `Sheet` 侧栏；**不添加固定底部 Dock 栏**。

---

### 4. 核心页面布局详规（按模块拆解，降低本地模型还原门槛）

#### A. 首页总览 `/overview`（桌面 1440px 高度一屏可读，严禁长列表无限拉长）
建议在 `frontend/src/components/panel/` 下拆分清晰子组件：
- **顶部紧凑 Header Bar（60px）**：
  - 左侧：时段问候（`早上好 / 下午好 / 晚上好` + 一句话真实状态，例如「当前有 X 条未读、Y 条 AI 精选待看」）；
  - 右侧：今日日期时间 + 快捷刷新（`queryClient.invalidateQueries()`）+ 明暗主题一键切换按钮（直接复用/写入 `useTheme()` / `settingsAPI.update` 保持同步）+ 设置按钮。
- **第一排：4 枚可点击真实数据 KPI 卡片（横排 4 列 `grid-cols-2 lg:grid-cols-4 gap-4`）**：
  - 全由现有 API 的 `total` 字段驱动（`limit: 1` 请求开销极低且准确）：
    1. **未读资讯**（`unread=true&triage_status=kept`）→ 点击跳 `/unread`
    2. **AI 精选放行**（`triage_status=kept`）→ 点击跳 `/all?triage_status=kept`
    3. **已拦截广告/水帖**（`triage_status=filtered`）→ 点击跳 `/all?triage_status=filtered`
    4. **我的收藏**（`/bookmarks?limit=1`）→ 点击跳 `/starred`
- **第二排：横向两栏 Bento 主工作区（`grid-cols-1 xl:grid-cols-[minmax(0,1.65fr)_340px] gap-5`）**：
  - **左主玻璃卡：「今日高分要闻 Top Picks」**
    - 数据来源：拉取 `itemAPI.list({ triage_status: "kept", limit: 50 })`，在客户端做副本按 `(ai_score ?? -1)` 降序，取前 **4–5 条**展示（确保一屏放得下、无长滚动）。
    - 每条渲染为横向紧凑行卡片：
      - 左：`np-tile` 领域渐变小图标（`ai` / `investing` / `deals` / `other` 专属图标与渐变）；
      - 中：标题（未读圆点标识）+ 2 行中文 `ai_summary`（若暂无摘要则截断 `content`）+ 底行元信息（领域徽章、`author` / 来源、`ai_tags.slice(0, 2)`、**双时间显示：原文发布时间 `published_at`/`pub_date` + 插件拉取时间 `created_at`**，hover 展示精确本地时间）；
      - 右：优先级徽章（`critical / high / normal / low`）+ `SignalBars`（10 格分值进度条 + `ai_score` 数字）+ 收藏快捷图标按钮。
      - 点击整卡：在当前 `/overview` 页直接打开 `ArticleDrawer`（利用 `?article=<id>` 或 URL 参数，**确保在 `/overview` 渲染抽屉或保持 `AppLayout` 下抽屉正常滑出并同步未读状态**！注意核查 `useUrlState` / `ArticleDrawer` 在非 `/$filter` 路由时的 `selectedArticleId` 兼容）。
    - 卡片右上角附 CTA：`查看全部资讯 →`（跳转到 `/unread` 或 `/all`）。
  - **右辅栏上下两张紧凑玻璃卡（总高度与左卡齐平）**：
    1. **「领域热力 Domain Pulse」**：统计最近 50 条 `kept` 情报中 `ai` / `investing` / `deals` / `other` 各自的条数、未读数及平均 `ai_score` 横向渐进进度条；点击任意领域行直接跳 `/all?domain=<domain>`。
    2. **「采集源心跳 Source Health」**：调用新增的 `pluginAPI.heartbeats()`（`GET /api/v1/plugins/heartbeats`），每行显示状态圆点（`ok` 绿 / `degraded` 橙 / `error` 红）、`source_id`、本轮采集条数 `fetched_count` 与相对上报时间，若有 `error_message` 显示摘要；右上角「详情 →」跳 `/sources`。

#### B. 资讯流 `/$filter` 与高级筛选工具栏（`components/panel/triage-toolbar.tsx` + `article-list.tsx` 升级）
- **保持所有已有列表能力**：支持全部/未读/收藏、`feedId` / `groupId` 过滤、`before` 游标分页加载更多、未读撤回 `pinRead`、空态提示与批量标记全部已读。
- **顶部玻璃控制条（同步 `triage-search.ts` URL query 参数，刷新链接不丢条件）**：
  - 第一行（常驻精简）：当前视图标题（或分组/源名称）+ `AI 放行 (kept) / 已拦截 (filtered) / 待处理 (pending) / 全部` 分段切换 + 搜索/刷新 + 展开/收起「高级筛选」开关按钮（显示当前已激活筛选条件数角标）。
  - 领域胶囊条（可横向滑动）：`全部领域`、`AI 前沿 (ai)`、`投资催化 (investing)`、`羊毛优惠 (deals)`、`其他 (other)`。
  - 可折叠高级筛选面板（按需展开）：
    - **优先级**：`全部 / 紧急 critical / 高 high / 普通 normal / 低 low`；
    - **最低分阈值 `min_score`**：快捷按钮 `不限 / ≥ 45 / ≥ 60 / ≥ 80` + 滑动条 `0–100`；
    - **列表排布排序**：客户端支持当前已加载分页按 `发布时间` 或 `AI 评分降序` 视图排序（说明仅重排已加载项以免误导分页游标）；
    - 一键「重置筛选」按钮。
- **单条情报卡片**：去掉过去纯文本 `Domain: ... Score: ...` 裸 `<dl>` 堆砌，改为与总览一致的**紧凑液态玻璃行样式**（渐变域图标 + 中文标题/推文首行 + 中文 `ai_summary` + 标签胶囊 + **双时间显示（发布 `published_at`/`pub_date` · 拉取 `created_at`，收藏快照列表不误标拉取时间）** + 优先级彩色胶囊 + `SignalBars` 评分条 + hover 操作按钮：标记已读/未读、收藏、打开原链接）。在 `triage_status === "filtered"` 时给出清晰醒目的「已拦截原因 / L1 标签」提示，方便用户核查是否误杀。

#### C. 右侧滑入阅读抽屉 `ArticleDrawer`（玻璃质感 + 中英对照层次）
- **必须保留现有状态安全逻辑**：继续复用 `useItem`、`useAutoMarkRead`、`pinRead`/`unpinRead`、收藏与 `j / k / Esc / s / o` 键盘导航，**不要改坏收藏页（`article.id < 0` 快照条目）与未读撤回时序**。
- **视觉与阅读体验升级**：
  - `SheetContent` 使用液态玻璃半透明面板与滑入缓动；
  - 顶部动作栏：已读切换、收藏、打开原文、`上一条 / 下一条`、关闭；
  - 正文区优先级从上到下：
    1. 元信息标签条（领域胶囊 + 分拣状态 + 优先级 + `ai_score` 分 + 标签 `ai_tags`）
    2. 标题（若为推文/短贴，配合作者 `@author`、来源 `source_id`、**双时间徽章：原文发布时间 `published_at`/`pub_date` 与插件拉取时间 `created_at` 均同时展示相对时间与精确本地时间**）
    3. **高亮「AI 中文摘要与提炼」玻璃卡片**（展示 `article.ai_summary`，清晰醒目）
    4. **可平滑折叠/展开的「原文 / 原始推文内容」区块**（基于 `grid-template-rows: 0fr -> 1fr` 动画；默认若中文摘要已涵盖全文可折叠或保留展开开关，安全渲染经 `processArticleContent` 净化后的正文，含 `x.com` 原推文长文本）
    5. **互动元数据卡（条件渲染）**：仅当 `raw_meta.metrics` 存在数字字段（如 `likes`、`retweets`、`replies`、`views`）时渲染 4 格轻量数字卡，同时安全展示 `raw_meta.origin_label`（如命中的关注账号或搜索词），绝不显示敏感凭证或写死假数据。

#### D. 来源状态页 `/sources`
- 对接真实 `GET /api/v1/plugins/heartbeats` 与现有 `feedAPI.list()`：
  - 上半区：**外置插件心跳卡片网格**（展示 `source_id`、`plugin_version`、状态徽章 `ok / degraded / error`、最近一次 `fetched_count`、心跳时间 `timestamp` 与 `updated_at`、若存在 `error_message` 则以告警提示框高亮展示）；
  - 下半区：**内置 RSS 订阅源运行概况简表**（统计启用/暂停/错误数及最近抓取异常源，跳转 `/feeds` 管理）。

---

## 精确前端 API 接入清单（严禁猜字段名）

执行端更新 `frontend/src/lib/api/types.ts`、`frontend/src/lib/api/index.ts` 及 `frontend/src/queries/` 时，必须严格按后端真实模型 [backend/internal/model/model.go](../backend/internal/model/model.go) 与 [backend/internal/store/plugin.go](../backend/internal/store/plugin.go) 对接：

1. **`Item` 已有真实字段（禁止改为 `score` / `inbox` / `p0`）**：
   - `id: number`, `feed_id: number`, `title: string`, `link: string`, `content: string`, `pub_date: number`, `unread: boolean`, `created_at: number`
   - `source_id: string`, `external_id: string`, `source_type: string`, `published_at?: string`, `author: string`, `raw_meta: Record<string, unknown>`
   - `domain: string`（当前后端实际值含 `"ai" | "investing" | "deals" | "other"` 等）
   - `ai_score: number | null`（`0–100`）
   - `ai_tags: string[]`
   - `ai_summary: string`
   - `priority: "critical" | "high" | "normal" | "low"`
   - `triage_status: "pending" | "kept" | "filtered" | "failed"`
2. **新增 `PluginHeartbeat` 类型与 API（对应 [plugin.go](../backend/internal/handler/plugin.go#L291-L298)）**：
   - 端点：`GET /v1/plugins/heartbeats`（`client.ts` 的 `API_BASE` 默认 `/api`，会自动拼为 `/api/v1/plugins/heartbeats`）
   - 响应体：`ListResponse<PluginHeartbeat>`（`{ data: PluginHeartbeat[], total: number }`）
   - 单条字段：
     ```ts
     export interface PluginHeartbeat {
       source_id: string;
       plugin_version: string;
       status: "ok" | "degraded" | "error";
       fetched_count: number;
       error_message: string | null;
       timestamp: string;
       updated_at: string;
     }
     ```
3. **i18n 护栏提示（极易踩坑！）**：
   - `frontend/package.json` 中的 `pnpm run check:i18n` 会校验 `src/lib/i18n/messages/en.ts` 的全部 key 是否同时存在于 `zh.ts`、`de.ts`、`fr.ts`、`es.ts`、`ru.ts`、`pt.ts`、`sv.ts` 共 **8 个文件**。
   - **所有新增 UI 文案键必须同步添加到 `en.ts`、`zh.ts` 及其余 6 个语种文件**（其余 6 个语种可先用英文同步键位确保字典完整性校验通过），不得在 TSX 中漏键导致 `check:i18n` 失败。
4. **URL 筛选与默认 `triageStatus` 注意**：
   - 目前 `frontend/src/queries/keys.ts` 中 `normalizeItemFilters` 默认把未传的 `triageStatus` 置为 `"kept"`；如果前端筛选器选了「全部状态（不过滤分拣状态）」，请检查后端 `GET /api/items`（不传 `triage_status` 时返回全部分拣状态）并谨慎扩展 `triageStatus?: Item["triage_status"] | "all"`（或允许 `undefined` 与不设默认 `"kept"` 时保持向后兼容、让各原有单测继续通过）。

---

## 文件边界（Owned / Read-Only / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `frontend/index.html`（如更新标题为 `news_panel` 或必要 meta）
- `frontend/src/**`（含新建 `frontend/src/components/panel/**`、样式 `index.css`、路由 `routes/**`、API/Queries、i18n 词典、组件及对应 Vitest 单元测试）
- *交接维护*：认领及结束时仅允许更新本任务卡 `tasks/P1-04-high-signal-panel.md` 状态与 `## Handoff Report`，以及 `tasks/INDEX.md` 中的 `P1-04` 对应行

### Read-Only Reference（仅供读取对照，禁止修改）
- `Temp/**`（`Temp/` 下 3 张 PNG + 2 个 TSX 样稿，**禁止修改、删除或 `git add`**）
- `AGENTS.md`、`docs/**`、`tasks/README.md`、`tasks/_TEMPLATE.md`
- `backend/**`（尤其是 `backend/internal/handler/handler.go`、`item.go`、`plugin.go`、`backend/internal/model/model.go`）
- `plugins/**`（尤其是 `plugins/x-twifork/x_twifork/transform.py` 的 `raw_meta` 字段）

### Forbidden / Out-of-Scope（严禁触碰）
- 未列在 `Owned Files` 内的所有其他文件
- **高风险禁区**：`backend/**`、`plugins/**`、`deploy/**`、`Dockerfile`、`Temp/**`、`frontend/package.json`（不得引入重型 UI/图表库；若确有极端必要调整配置须先在 `BLOCKED` 回执说明，禁止私自加库）

---

## 验收标准与验证命令（Acceptance Criteria & Verification）

- [x] **视觉与材质对齐**：浅色与深色模式均呈现淡紫/冰蓝径向光晕画布 + 统一浅色磨砂玻璃侧栏（暗色模式跟随切换）+ 玻璃描边弥散阴影卡片；**桌面端无底部 Dock 栏**。
- [x] **首页总览 `/overview` 一屏可读**：包含顶部问候与操作栏、4 个真数据可点击 KPI 卡片、左栏高分要闻精选（4–5 条）、右栏领域热力 + 真实插件心跳卡；无假数据、无 `Ask AI` 摆设。
- [x] **侧栏与快捷入口齐全**：侧栏含「总览 / 全部资讯 / 未读 / 收藏」，**并显式包含「已拦截 (filtered)」一键快捷入口**、常用领域快捷入口及「来源状态 `/sources`」入口。
- [x] **资讯流高级筛选 & 抽屉 & 双时间显示**：支持按 `triage_status` / `domain` / `priority` / `min_score` / `unread` 筛选并与 URL 同步；列表与总览卡片紧凑展示「发布 + 拉取」双时间（hover 显示精确时间），点击条目无论从 `/overview` 还是 `/$filter` 均能平滑打开右侧抽屉，展示双时间精确明细、AI 中文摘要、平滑折叠英文原文、`raw_meta.metrics` 互动数据、已读/未读与收藏切换、键盘快捷键。
- [ ] **全量校验通过（Windows 或 WSL 下在 `frontend/` 执行）**（`install` / `check:i18n` / `typecheck` / `test` / `build` 全部通过；`pnpm run lint` 因 `main` 基线 `frontend/package.json` 已锁定 `typescript@~7.0.2` 与 `typescript-eslint@8.71.0` 不兼容而报错，详见下方 Handoff Report）：
  ```bash
  cd frontend
  pnpm install --frozen-lockfile
  pnpm run check:i18n
  pnpm run lint
  pnpm run typecheck
  pnpm run test
  pnpm run build
  ```
- [x] **新增单测 & 边界核验**：为新增筛选/聚合/API 映射逻辑补充至少 2 组 Vitest 单元测试；提交前执行 `git diff --name-only` 与 `git status --short` 确认 100% 落入 `Owned Files` 且未夹带 `Temp/**`。

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`IN_REVIEW`
- **执行端 / 通道**：`Antigravity / 独立分支 feat/p1-04-high-signal-panel（基于 baseline-pre-p1-04 / 01d3015）`
- **实际改动文件清单**：
  - `frontend/index.html`（补充 `<meta name="color-scheme" content="light dark">` 与明暗 `theme-color`）
  - `frontend/src/index.css`（接入液态玻璃 `:root` / `.dark` 色彩与材质变量、双光晕径向画布背景、与 `next-themes` 同步的 `color-scheme` 与 `accent-color`）
  - `frontend/src/components/panel/panel.css`（新增液态玻璃面板 `.np-glass`、`.np-sidebar-glass`、`.np-drawer-glass`、`.np-card-interactive`、`.np-tile`、`.np-orig-collapse`、`.np-range` 及 `prefers-reduced-motion` 支持）
  - `frontend/src/components/panel/panel-utils.ts`（新增领域/优先级/心跳映射、Top Picks 排序、Domain Pulse 聚合、`raw_meta.metrics` 安全提取、已加载条目排序、双时间解析 `resolveItemTimestamps` 与精确时间格式化 `formatExactDateTime` 等纯函数）
  - `frontend/src/components/panel/panel-utils.test.ts`（新增 5 组 Vitest 单元测试覆盖聚合、排序、元数据清洗、心跳统计与双时间解析）
  - `frontend/src/components/panel/item-timestamps.tsx`（新增 `ItemTimestamps` 复用组件：卡片 `compact` 模式展示「发布 · 拉取」相对时间与 hover 精确时间，阅读抽屉 `detailed` 模式同时展示相对与精确本地时间，收藏快照安全隐藏非拉取时间）
  - `frontend/src/components/panel/signal-bars.tsx`（新增 `DomainTile`、`DomainPill`、`PriorityPill`、`TriageStatusPill`、`SignalBars` 组件）
  - `frontend/src/components/panel/theme-toggle-button.tsx`（新增顶栏一键亮/暗切换按钮，与 `next-themes` 及 `/api/settings` 双向同步）
  - `frontend/src/components/panel/kpi-strip.tsx`（新增 4 枚真实 API 数据驱动的可点击 KPI 卡片）
  - `frontend/src/components/panel/top-picks-card.tsx`（新增首页高分要闻精选卡，点击直接在 `/overview?article=<id>` 滑出抽屉）
  - `frontend/src/components/panel/domain-pulse-card.tsx`（新增领域热力分布卡，点击直达对应领域筛选）
  - `frontend/src/components/panel/source-health-card.tsx`（新增外置插件心跳概览卡，对接 `GET /api/v1/plugins/heartbeats`）
  - `frontend/src/components/panel/overview-page.tsx`（新增 `/overview` 一屏横向 Bento 总览页）
  - `frontend/src/components/panel/triage-toolbar.tsx`（新增资讯流顶部玻璃控制条、领域胶囊条与可平滑折叠的高级筛选面板）
  - `frontend/src/components/panel/sources-page.tsx`（新增 `/sources` 来源状态监控页：外置插件心跳网格 + 内置 RSS 订阅概况表）
  - `frontend/src/lib/api/types.ts`、`frontend/src/lib/api/index.ts`（新增 `PluginHeartbeat` 类型与 `pluginAPI.heartbeats()`）
  - `frontend/src/lib/api/plugin-api.test.ts`（新增 2 组 Vitest 单元测试校验 `/api/v1/plugins/heartbeats` 与 `/api/items` 查询参数映射）
  - `frontend/src/lib/triage-search.ts`、`frontend/src/lib/triage-search.test.ts`（扩展支持 `triage_status="all"` 与空白 `domain` 清洗并补充单测）
  - `frontend/src/queries/keys.ts`、`frontend/src/queries/items.ts`、`frontend/src/queries/bookmarks.ts`、`frontend/src/queries/plugins.ts`（新增 `useOverviewCounts`、`useOverviewKeptItems`、`usePluginHeartbeats`，支持 `triageStatus="all"` 省略查询参数，并同步已读状态乐观更新至总览缓存）
  - `frontend/src/hooks/use-url-state.ts`、`frontend/src/hooks/use-article-list.ts`、`frontend/src/hooks/use-keyboard.ts`（修复 `/overview` 页内打开/关闭 `?article=<id>` 抽屉兼容、暴露 `triageSearch`/`setTriageSearch`、新增 `g o` 与 `g h` 导航快捷键）
  - `frontend/src/components/layout/app-layout.tsx`、`frontend/src/components/layout/sidebar.tsx`、`frontend/src/components/layout/content-header.tsx`、`frontend/src/components/layout/shortcuts-dialog.tsx`（重构为统一浅色磨砂液态玻璃侧栏，内置「已拦截噪声」快捷入口、领域快捷过滤、折叠订阅树与底部插件健康卡，桌面端无底部 Dock 栏）
  - `frontend/src/components/feed/feed-list.tsx`、`frontend/src/components/feed/feed-group-card.tsx`（适配折叠订阅树与玻璃卡片视觉）
  - `frontend/src/components/article/article-list.tsx`、`frontend/src/components/article/article-item.tsx`、`frontend/src/components/article/article-drawer.tsx`、`frontend/src/components/article/triage-metadata.tsx`（升级资讯流玻璃卡片、拦截审计横幅、右侧玻璃滑入阅读抽屉、AI 中文摘要卡、平滑折叠原文区及条件渲染的 `raw_meta.metrics` 互动卡）
  - `frontend/src/routes/index.tsx`、`frontend/src/routes/overview.tsx`、`frontend/src/routes/overview.lazy.tsx`、`frontend/src/routes/sources.tsx`、`frontend/src/routes/sources.lazy.tsx`、`frontend/src/routes/feeds.lazy.tsx`、`frontend/src/routes/login.lazy.tsx`（新增 `/overview` 与 `/sources` 路由，`/` 默认跳转 `/overview`，对齐登录页与订阅管理页玻璃视觉）
  - `frontend/src/lib/i18n/messages/{en,zh,de,fr,es,ru,pt,sv}.ts`（8 语种词典全量同步新增 114 个键至 303 键，含双时间展示文案）
  - `tasks/P1-04-high-signal-panel.md`、`tasks/INDEX.md`（更新规格、状态与回执）
- **实测验证命令与观察结果**（均为最终修改后亲自执行并观察到的真实输出）：
  - `pnpm --dir frontend install --frozen-lockfile` → `✓ Lockfile passes supply-chain policies (621 entries)`, `Packages: +514`, `Done in 13.6s`（exit code 0）
  - `pnpm --dir frontend run check:i18n` → `[zh/de/fr/es/ru/pt/sv] OK (303 keys)`, `All locale dictionaries are complete.`（exit code 0）
  - `pnpm --dir frontend run lint` → **退出码 1（基线固有工具链冲突，修改代码前即存在）**：`Error: typescript-eslint does not support TS 7.0.`（因 `Forbidden Files` 中的 `frontend/package.json` 锁定了 `typescript@~7.0.2` 与 `typescript-eslint@8.71.0`，本任务严格遵守边界未改动 `frontend/package.json` 或 `eslint.config.js`）
  - `pnpm --dir frontend run typecheck` → `tsr generate && tsc -b --noEmit` 通过，零报错（exit code 0）
  - `pnpm --dir frontend run test` → `Test Files 7 passed (7)`, `Tests 34 passed (34)`（含新增 `panel-utils.test.ts` 5 用例、`plugin-api.test.ts` 2 用例与 `triage-search.test.ts` 扩展用例，exit code 0）
  - `pnpm --dir frontend run build` → `tsr generate && tsc -b && vite build` 成功，`2406 modules transformed`, `✓ built in 1.74s`（exit code 0）
  - `git diff --name-only` & `git status --short` → 确认全部修改与新增文件 100% 落入 `Owned Files`（`frontend/index.html`、`frontend/src/**`、`tasks/P1-04-high-signal-panel.md`、`tasks/INDEX.md`），未触碰 `backend/**`、`plugins/**`、`deploy/**`、`Dockerfile`、`frontend/package.json`，且未 `git add Temp/`
- **自检截图 / 界面效果确认**：
  - 使用临时独立 scratch 静态预览服务（不落盘到仓库）在 1440×900 桌面端浏览器实机验证全部 5 项检查点且零控制台报错：
    1. `/` 自动跳转 `/overview`：统一浅色磨砂玻璃侧栏（无深蓝底、无底部 Dock 栏）、60px 问候顶栏、4 张 KPI 卡、左侧 5 条高分要闻、右侧领域热力与采集源心跳一屏可读；
    2. 在 `/overview` 点击首条高分要闻后直接滑出右侧玻璃抽屉（`/overview?article=101`），完整展示元信息胶囊、AI 中文摘要卡、平滑展开/收起原文区及 4 格 `raw_meta.metrics` 互动数据；
    3. 点击侧栏「已拦截噪声」进入 `/all?triage_status=filtered`，展示拦截审计提示条，并验证「高级筛选」折叠面板（优先级、最低分滑块/预设、已加载排序）展开正常；
    4. 点击侧栏「来源状态」进入 `/sources`，正确展示外置插件心跳卡片（含 `degraded` 告警框）与内置 RSS 概况表；
    5. 顶栏点击切换深色模式后 `/overview` 完整切换至深色紫蓝光晕与深色磨砂玻璃面板，文字与徽章对比度清晰可读。
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - **边界外工具链提示（供 Lead 评估）**：仓库 `main` 基线的 `frontend/package.json` 锁定的 `typescript@~7.0.2` 与 `typescript-eslint@8.71.0` 存在上游不兼容，导致 `pnpm run lint` 在修改前后均直接抛出 `typescript-eslint does not support TS 7.0.`；因 `frontend/package.json` 属 `Forbidden Files`，本次未擅自修改。建议 Lead 审查合入后按需在独立维护提交中升级/适配 ESLint 依赖（或等 `typescript-eslint` 支持 TS 7）。
  - 下一步建议：Lead 审阅 `feat/p1-04-high-signal-panel` 分支改动与实机界面效果，确认无误后合入 `main` 并触发 GHCR 镜像构建部署验证，推进 Gate 1 验收。
