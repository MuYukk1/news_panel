# `P2-02b`：L2 原生多厂商 Decisions API 适配器与轻量 AI 设置面板

## 元信息（Metadata）
- **Task ID**：`P2-02b`
- **Phase / Milestone**：`Phase 2 (v0.2 / 落实 DEC-008)`
- **前置门禁（Gate Dependency）**：`Gate 1（2026-11-20）与 DEC-008（2026-11-27）已批准 + P2-02、P2-02a 均已 DONE`
- **当前状态（Status）**：`IN_REVIEW`
- **调度通道与目标执行端（Channel / Runner）**：`通道 A / 通道 B 均可（ZCode / Codex / Trae / Antigravity；Go 后端 + SQLite 迁移 + React 19 设置 Tab 全栈切片）`
- **工作区 / 分支策略（Worktree / Branch）**：**严禁直接在 `main` 提交代码**。须从包含 `P2-02a` 的最新 `main` 切出独立分支或 worktree：
  - `git checkout -b feat/p2-02b-decision-apis-and-ai-settings`（或 `git worktree add ../news_panel-p2-02b -b feat/p2-02b-decision-apis-and-ai-settings`）

---

## 目标与上下文（Objective & Context）
- **核心目标**：落实已批准的 `DEC-008`，将 L2 门控升级为支持 **2026 四大原生非生成式 Decisions API** 的统一纯标准库多预设架构，并在现有 `SettingsDialog` 中提供**零重启热更新的轻量「AI 分拣」配置面板**：
  1. **统一 L2 原生 Decision Provider 双 Dialect 引擎**（零 `go.mod` 新增依赖，纯 Go `net/http` + `encoding/json`）：
     - 新增 **`gemini_decision`**（Google Gemini Decisions API：`gemini-decision-1`，免费层 200 RPM / 10k RPD，强中文支持）；
     - 新增 **`openai_decision`**（OpenAI Decisions API：`gpt-6-luna`，GA，支持缓存计费与强中文）；
     - 重构复用 **`typesafe`**（TypeSafe AI Jev System One：`jev-latest`）；
     - 新增 **`cloudflare_clef`**（Cloudflare Workers AI：`@cf/cloudflare/clef-flash`，Apache-2.0 开源权重 + 每日免费 Neurons，支持 Cloudflare Account ID 配置）；
     - 100% 向后兼容既有 **`openai`**（OpenAI-compatible `/v1/chat/completions`）、**`http_json`** 与 **`off`**（/ `none`）。
  2. **独立 `ai_settings` SQLite 持久化与配置优先级（DB > ENV > Default）**：
     - 新增迁移 `008_ai_settings.sql` 单行表（`id = 1`），标准 Docker 卷 `0600` SQLite 存储，**不引入**额外加密密钥环境变量；
     - 暴露受读者鉴权保护的独立端点：`GET /api/v1/ai/settings`、`PATCH /api/v1/ai/settings` 与 `POST /api/v1/ai/settings/test`（一键连通性探测），**绝不混入** `GET/PATCH /api/settings` 及前端 `localStorage` 同步。
  3. **零重启热更新（Hot-Reload）**：
     - `Evaluator` 通过 `atomic.Pointer` 原子切换配置与 L2 Provider 快照（保留累计统计计数与既有 L3 `CLIProvider` 冷却锁状态）；
     - `Puller.rssLimit` 升级为 `atomic.Int64` 并暴露 `SetRSSLimit(int)`，支持在线调优 `AI_RSS_MAX_ITEMS_PER_PULL` 与 `AI_TOPICS`。
  4. **轻量前端「AI 分拣」Tab（`SettingsDialog`）**：
     - 预设选择器（含 `cloudflare_clef` 专属 Cloudflare Account ID 输入框）、脱敏 API Key 输入与清除、开关、一键「测试连接」（返回耗时与样例判别）、可折叠高级参数、`AI_TOPICS` 与 `AI_RSS_MAX_ITEMS_PER_PULL` 调优、只读 L3 状态与运行统计条。
- **前置输入 / 依赖任务**：
  - `P2-02`（`DONE`）已建立 `DecisionProvider` 接口、`Evaluator`、`/api/v1/ai/stats` 与 RSS `ai_enabled` 闭环；
  - `P2-02a`（`DONE`）已解耦 `puller.go` 与 `handler.go` 的单源刷新超时并增加同源刷新去重；
  - 后续 `P2-03`（用户反馈与有界个性化档案）将直接向本任务抽出的统一多问构造器与热更新 `Evaluator` 注入用户偏好档案，因此本任务必须保证问题构造模块化、无硬编码耦合。

---

## 关联文档与约束（References & Constraints）
开始前必读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)（重点核对 `DEC-004`、`DEC-006`、`DEC-007`、`DEC-008`）
4. [docs/PRD.md](../docs/PRD.md)
5. [docs/ROADMAP.md](../docs/ROADMAP.md)
6. [tasks/README.md](./README.md)

本任务硬性约束：
- [x] **严守 `DEC-008` 锁定范围与 Non-Goals**：不新增 OpenRouter Alpha 作为 Tier-1 预设；**不做**插件市场 UI、不做多租户、**绝不在 Web UI 暴露 L3 CLI Shell 命令（`AI_CLI_COMMAND` / `AI_CLI_ARGS`）编辑**。
- [x] **严守纯标准库与 1C1G–2C2G VPS 红线**：禁止修改 `backend/go.mod` / `backend/go.sum` 引入任何三方 AI SDK（如 `google.golang.org/genai`、`openai-go` 等）或本地重推理组件；单次响应体严格限长 `<= 1 MiB`。
- [x] **严守密钥与 SSRF 安全红线**：HTTP 接口绝不回传明文 `api_key`；绝不写入前端 `localStorage`、日志或错误；自定义 `base_url` 在保存与测试连接时均执行公网 IP SSRF 校验（含拨号时防 DNS 重绑定）且严禁 HTTP 重定向（仅当环境变量 `AI_ALLOW_PRIVATE_ENDPOINTS=true` 时允许私网地址）。
- [x] **零回归向后兼容**：既有仅配置 `AI_*` / `AI_DECISION_*` 环境变量（且未改动 Web 设置）的部署必须 100% 保持原行为；`AI_DECISION_PROVIDER=openai` 语义保持为 OpenAI-compatible Chat LLM 不变；`triage.TriageEvaluator` 接口签名不变。

---

## 功能与工程详规（Detailed Spec）

### 模块 1：已验证的 L2 原生 Decisions API 协议与统一适配（`backend/internal/ai/`）

#### 1.1 预设矩阵与已验证 Wire 协议（全部为单次前向多问、$0 输出计费）

| `provider` ID | 默认 Base URL | 默认 Model | 鉴权 Header | Wire Dialect | 关键细节与解析容错要求 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `gemini_decision` | `https://generativelanguage.googleapis.com/v1beta/decisions` | `gemini-decision-1` | `x-goog-api-key: <KEY>`（**严禁**走 `?key=` 查询参数） | **Map Dialect (`input`)** | - 模型 ID 不带 `models/` 前缀<br>- `questions` 键名须为 ASCII `^[A-Za-z0-9_-]{1,64}$`（`action`/`priority`/`domain` 天然合规）<br>- 响应 `answers[k]` 读取 `choice`（容错回退 `value`）与 `confidence`，兼容省略或包含 `type` 字段 |
| `openai_decision` | `https://api.openai.com/v1/decisions` | `gpt-6-luna` | `Authorization: Bearer <KEY>` | **OpenAI Array Dialect (`input`)** | - `questions` 为 JSON 数组：`[{"type":"choice","name":"action","instructions":"...","choices":[{"value":"pass","description":"..."}]}]`<br>- `answers` 为 JSON 数组，按 `name` 索引；解析器须容错 `choice` 与 `value` 两种字段名，且容忍 `probabilities` 为数组或对象（避免线上细微字段差异导致报错） |
| `typesafe` | `https://api.typesafe.ai/v1/systemone` | `jev-latest` | `Authorization: Bearer <KEY>` | **Map Dialect (`state`)** | - 延续现有 `typeSafeEndpoint` 规范化逻辑（补 `/v1/systemone`）<br>- 响应 `answers[k]` 读取 `choice` 与 `confidence` |
| `cloudflare_clef` | `https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}` | `@cf/cloudflare/clef-flash` | `Authorization: Bearer <KEY>` | **Map Dialect (`state`) + CF Envelope** | - 需要 `AccountID`（环境变量 `AI_DECISION_ACCOUNT_ID` 或 SQLite `decision_account_id`；若用户手填完整含 `/accounts/.../ai/run/` 的 `BaseURL` 也可直接生效）<br>- **安全校验**：`AccountID` 须严格匹配 `^[A-Za-z0-9_-]{1,64}$`，防止 URL 路径注入；`Model` 仅允许 `^[A-Za-z0-9@._/-]{1,128}$` 且禁止 `..`<br>- 响应通常包在 `{"success":true,"result":{"answers":{...}}}` 中：若 `success == false` 返回脱敏错误；优先从 `result.answers` 解析并兼容顶层 `answers`；answer 字段容错 `choice` / `selected` / `value` |
| `openai` | `https://api.openai.com/v1` | `gpt-4o-mini` | `Authorization: Bearer <KEY>` | 既有 OpenAI Chat Completions | 保持 100% 向后兼容 |
| `http_json` | *(空，必填)* | `gpt-4o-mini` | 自定义 `AI_DECISION_AUTH_HEADER` | 既有扁平 JSON POST | 保持 100% 向后兼容；`auth_header` 校验 `headerNamePattern` 且禁止 `Host` / `Cookie` / `Content-Length` / `Transfer-Encoding` |
| `off`（别名 `none`） | — | — | — | 关闭 L2 | 直接跳过 L2 进入 L3 |

#### 1.2 参考请求/响应 JSON 样例（供实现与 `httptest` 单测对照）
1. **Map Dialect（`gemini_decision` 用 `"input"`，`typesafe`/`cloudflare_clef` 用 `"state"`）**：
   ```json
   {
     "model": "gemini-decision-1",
     "input": "Source: x\nAuthor: ...\nTitle: ...\nContent: ...\nPublished: ...\nL1 Hint: domain=ai priority=normal",
     "questions": {
       "action": {
         "type": "choice",
         "instructions": "Decide whether this item is worth forwarding ...",
         "criteria": { "pass": "...", "filter": "..." }
       },
       "priority": {
         "type": "choice",
         "instructions": "Assign reader priority level ...",
         "criteria": { "critical": "...", "high": "...", "normal": "...", "low": "..." }
       },
       "domain": {
         "type": "choice",
         "instructions": "Classify the primary domain ...",
         "criteria": { "ai": "...", "investing": "...", "deals": "...", "other": "..." }
       }
     }
   }
   ```
   - `cloudflare_clef` 额外注意：模型已编码在路径 `.../ai/run/@cf/cloudflare/clef-flash` 中，载荷仍可附带 `state` 与 `questions`（无需强制发无效字段，或与 System One 保持一致带 `model`/`state`/`questions`），且解析器同时兼容 `{ "success": true, "result": { "answers": { ... } } }` 与直接 `{ "answers": { ... } }`。
2. **OpenAI Array Dialect（`openai_decision`）**：
   ```json
   {
     "model": "gpt-6-luna",
     "input": "Source: x\n...",
     "questions": [
       {
         "type": "choice",
         "name": "action",
         "instructions": "...",
         "choices": [
           { "value": "pass", "description": "..." },
           { "value": "filter", "description": "..." }
         ]
       },
       { "type": "choice", "name": "priority", "instructions": "...", "choices": [...] },
       { "type": "choice", "name": "domain", "instructions": "...", "choices": [...] }
     ]
   }
   ```
   对应响应解析：遍历 `answers[]`，按 `name`（`"action"` / `"priority"` / `"domain"`）取值，优先读 `choice`，空则回退 `value`，并提取 `confidence` 拼入脱敏 `Reason`（如 `openai_decision:pass(0.96)/high(0.91)`）。

#### 1.3 代码组织与共享抽象（`backend/internal/ai/`）
- 在 `decision_config.go` 扩展 `DecisionConfig`：新增 `AccountID string`（`AI_DECISION_ACCOUNT_ID`）与 `AllowPrivateEndpoints bool`（`AI_ALLOW_PRIVATE_ENDPOINTS`，默认 `false`，**仅来自环境变量，不可由 Web API 改写**）。
- 新增 `backend/internal/ai/decision_native.go`：
  - `buildDecisionSpec(topics string, in triage.Input, l1 triage.Result)`：单一权威源，构建 `action` / `priority` / `domain` 三个 `choice` 问题（保持独立函数，方便 `P2-03` 后续直接传入 `profileHint`）；
  - `nativeDecisionProvider`：通过声明式预设表驱动 `dialectSystemOneMap` 与 `dialectOpenAIArray`，共用 `doDecisionPOST(...)`（强制 `io.LimitReader(1<<20)`、`CheckRedirect: http.ErrUseLastResponse`、SSRF 安全 `DialContext` 与 `redactCredential`）；
  - `TestDecisionConfig(ctx context.Context, cfg Config)`：构造临时 `DecisionProvider`，发送一条轻量合成测试条目（如 `"OpenAI released a new model update with 2x lower latency."`），返回结构化 `DecisionTestResult{OK, Provider, Model, LatencyMS, Outcome, Error}`（所有错误信息严格脱敏）。
- 在 `backend/internal/pkg/httpc/` 导出辅助函数（如 `NewNoRedirectClient(timeout time.Duration, allowPrivate bool) *http.Client` 或导出 `ValidateDialTarget`），让 `internal/ai` 复用现成 SSRF 防护而零重复代码！

---

### 模块 2：SQLite `ai_settings` 持久化与零重启热更新

#### 2.1 数据库迁移（`backend/internal/store/migrations/008_ai_settings.sql`）
```sql
CREATE TABLE IF NOT EXISTS ai_settings (
    id                       INTEGER PRIMARY KEY CHECK (id = 1),
    decision_enabled         INTEGER, -- NULL=inherit env, 0=false, 1=true
    decision_provider        TEXT,
    decision_base_url        TEXT,
    decision_api_key         TEXT,
    decision_account_id      TEXT,
    decision_auth_header     TEXT,
    decision_model           TEXT,
    decision_json_format     INTEGER,
    decision_timeout_seconds INTEGER,
    decision_l3_priorities   TEXT,
    decision_low_action      TEXT,
    decision_fallback_mode   TEXT,
    topics                   TEXT,
    rss_max_items_per_pull   INTEGER,
    updated_at               INTEGER NOT NULL DEFAULT (unixepoch())
);
```
- 遵循 `004_settings.sql` 约定：`NULL` 表示「未在 Web 面板覆盖，继承环境变量/预设默认」；`store/ai_settings.go` 提供 `GetAISettings()`、`UpdateAISettings(params)`（支持部分更新、显式清除 `decision_api_key` 为 `NULL`、以及 `ResetAISettings()` 一键恢复继承环境变量）。
- 依赖标准 Docker 卷内 `0600` SQLite 文件存储，**不引入**额外加密密钥环境变量（避免密码轮换导致已保存 API Key 无法解密）。

#### 2.2 零重启热更新（`Evaluator` + `Puller` + `main.go`）
- **`Evaluator`（`backend/internal/ai/evaluator.go`）**：
  - 将易变运行态抽为不可变结构体 `evalSnapshot{cfg Config, l1 *l1Rules, decision DecisionProvider, provider CompletionProvider}`，存于 `state atomic.Pointer[evalSnapshot]`；
  - `Evaluate()` 与 `Stats()` 无锁原子读取 `e.state.Load()`，保留 `pipelineCounters` 跨重载连续计数；
  - 新增 `e.Reload(mergedCfg Config)`（受小型 `reloadMu sync.Mutex` 序列化写路径）：更新 `cfg`、`l1`、`decision` 并复用既有 `provider`（**不重建** `CLIProvider` 从而保护冷却锁与时间戳），写入 `e.state.Store(next)`；
  - 新增 `e.EnvConfig() Config` 返回启动时读取的只读环境变量基线，供合并优先级 `DB Override > Env > Default` 与「恢复环境变量默认」使用。
- **`Puller`（`backend/internal/pull/puller.go`）**：
  - 将 `rssLimit` 改为 `atomic.Int64`，新增 `SetRSSLimit(n int)` 与 `RSSLimit() int`。
- **`cmd/fusion/main.go`**：
  - 启动顺序：`store.New` → 加载 `ai_settings` 并与 `ai.ConfigFromEnv` 合并 → 初始化 `eval` 与 `puller.SetRSSLimit(...)` → 注入 `handler.New`。

---

### 模块 3：独立 AI 设置 HTTP API 与安全护栏（`backend/internal/handler/ai_settings.go`）

全部挂在 `handler.go` 的读者鉴权组 `auth` 下（`authMiddleware()`，插件 `INGEST_TOKEN` 访问返回 `401`）：

1. **`GET /api/v1/ai/settings`**：
   - 返回当前生效配置（`effective`）、DB 覆盖摘要、字段来源标记、只读 L3 状态（`l3_enabled`, `l3_provider`, `l3_model` — **不含** `AI_API_KEY`、`AI_CLI_COMMAND`、`AI_CLI_ARGS`）、以及**仅脱敏后的 Key 状态**：
     - `decision_api_key_configured: bool`
     - `decision_api_key_source: "db" | "env" | "none"`
     - `decision_api_key_masked: string`（长度 ≥ 8 时 `"••••" + last4`，否则非空时 `"••••"`，空时 `""`）
   - 附带 `presets` 元数据数组（每个预设的 `id`, `label`, `default_base_url`, `default_model`, `requires_api_key`, `requires_account_id`, `auth_header_hint`），前端直接渲染无需硬编码重复 URL。
2. **`PATCH /api/v1/ai/settings`**：
   - 支持部分更新（指针字段）；
   - `decision_api_key` 为 `nil` 或空字符串 `""` 时**保留现有密钥**；显式传 `clear_decision_api_key: true` 时清空 DB 密钥（回落至环境变量或空）；显式传 `reset_to_env: true` 时清空整行 DB 覆盖并恢复全部环境变量配置；
   - 严格校验：
     - `decision_provider` 必须在白名单 `{gemini_decision, openai_decision, typesafe, cloudflare_clef, openai, http_json, off}`；
     - `decision_base_url` 非空时须通过 `httpc.ValidateRequestURL(ctx, url, allowPrivate)`（拒绝非 `http`/`https`、拒绝 `localhost`/私网/Link-Local `169.254.169.254`，除非 `AI_ALLOW_PRIVATE_ENDPOINTS=true`）；
     - `decision_account_id` 在 `cloudflare_clef`（且未手填完整自定义 URL）时校验正则 `^[A-Za-z0-9_-]{1,64}$`；
     - `decision_auth_header` 校验 `headerNamePattern` 且禁止 `Host`、`Cookie`、`Content-Length`、`Transfer-Encoding`；
     - `decision_timeout_seconds` 限定 `1..60`；`rss_max_items_per_pull` 限定 `0..500`；`decision_l3_priorities` / `decision_low_action` / `decision_fallback_mode` 枚举校验；
   - 持久化后立即调用 `evaluator.Reload(...)` 与 `puller.SetRSSLimit(...)`，返回与 `GET` 同格式的脱敏响应。
3. **`POST /api/v1/ai/settings/test`**：
   - 接受可选草稿覆盖字段（未填写新 `decision_api_key` 时自动使用已生效的 DB/ENV 密钥），执行与 `PATCH` 同等 URL/SSRF/Header 校验；
   - 增加轻量进程内限流（如 `sync.Mutex` 记录上次测试时间，2 秒冷却防连点刷额度，触发返回 `429 Too Many Requests`）；
   - 调用 `ai.TestDecisionConfig(...)`，返回 `{"data":{"ok":true,"provider":"...","model":"...","latency_ms":182,"outcome":{"action":"pass","priority":"high","domain":"ai","reason":"..."},"error":""}}`（`error` 经 `redactCredential` 脱敏，永不泄露凭证）。

---

### 模块 4：前端轻量「AI 分拣」设置 Tab（`frontend/src/`）

- 在 `frontend/src/components/settings/settings-dialog.tsx` 的侧边导航新增第 3 个 Tab：`ai`（`AI 分拣` / `AI Triage`，图标如 `Sparkles` / `SlidersHorizontal`），将具体表单拆分为独立组件 `frontend/src/components/settings/ai-settings-tab.tsx`：
  1. **L2 决策门控区**：
     - 顶部状态条：显示当前 L2 开关、活跃 L2/L3 Provider 以及来自 `GET /api/v1/ai/stats` 的省流计数（L1 拦截 / L2 拦截 / L2 跳过 L3 / L3 调用）；
     - 启用开关 + **供应商预设下拉框**（切换预设自动预填推荐 `model` 与默认 `base_url` 占位提示，不覆盖用户已保存密钥）：
       - `Gemini Decision 1 (免费额度 / 推荐中文)`
       - `OpenAI Decisions (gpt-6-luna)`
       - `TypeSafe Jev System One`
       - `Cloudflare Workers AI Clef (Apache-2.0 / 免费额度)`
       - `OpenAI-Compatible LLM`
       - `Custom HTTP JSON`
       - `Off (关闭 L2，直达 L3)`
     - **Cloudflare Account ID 输入框**：当且仅当选中 `cloudflare_clef` 时显示（带简短提示）；
     - **API Key 输入框**（`type="password"`，`autoComplete="off"`）：展示已保存状态徽标（如 `已保存 ••••abcd` / `使用环境变量` / `未配置`），留空保存不覆盖旧密钥，提供显式「清除已保存密钥」按钮；
     - **一键「测试连接」按钮**：调用 `POST /api/v1/ai/settings/test` 测试当前表单草稿，内联展示绿色成功徽标（耗时 ms + `action/priority/domain`）或红色脱敏错误；
     - **折叠「高级参数」**：Base URL、Model、Auth Header（`http_json` 显示）、JSON Format（`openai` 显示）、超时秒数、进入 L3 的优先级多选/逗号配置、Low Action、Fallback Mode。
  2. **主题与 RSS 评估预算区**：
     - 关注主题（`AI_TOPICS`，多行文本输入）与单源每次拉取评估上限（`AI_RSS_MAX_ITEMS_PER_PULL`，数字输入，`0` 为不限）。
  3. **底部操作栏**：
     - 「保存并立即生效」（`PATCH /api/v1/ai/settings` + Toast 提示）与「恢复环境变量默认」（确认后 `reset_to_env: true`）。
- 同步补齐 `frontend/src/lib/api/types.ts`、`frontend/src/lib/api/index.ts` 及 `frontend/src/lib/i18n/messages/**`（确保 `zh.ts`、`en.ts` 与其余语言文件均通过 TypeScript 严格键检查）。
- **严禁**修改 `frontend/src/lib/settings-sync.ts` 或向 `localStorage` 写入任何 AI 配置/密钥。

---

### 模块 5：部署配置与文档更新（`deploy/` + `backend/internal/ai/README.md`）
- `deploy/docker-compose.yml` 与 `deploy/.env.example`：补齐 `AI_DECISION_ACCOUNT_ID` 与 `AI_ALLOW_PRIVATE_ENDPOINTS`（默认 `false`），更新 `AI_DECISION_PROVIDER` 注释列出全部 Tier-1 预设。
- `backend/internal/ai/README.md`：更新支持矩阵、双 Dialect 说明、热更新优先级（DB > ENV > Default）与安全护栏说明。

---

## 文件边界（Owned / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- **后端 AI 与安全网络**：
  - `backend/internal/ai/**`（含新增 `decision_native.go` 及单测、更新 `config.go`、`decision_config.go`、`decision.go`、`evaluator.go`、`stats.go`、`README.md`）
  - `backend/internal/pkg/httpc/**`（导出/复用 SSRF-safe 无重定向客户端辅助函数及单测）
- **后端模型、存储与热更新接入**：
  - `backend/internal/model/model.go`（新增 `AISettings` 结构体）
  - `backend/internal/store/migrations/008_ai_settings.sql`（新建）
  - `backend/internal/store/ai_settings.go`、`backend/internal/store/ai_settings_test.go`（新建）
  - `backend/internal/pull/puller.go`、`backend/internal/pull/puller_test.go`（仅 `rssLimit` 原子化与 `SetRSSLimit`）
  - `backend/internal/handler/handler.go`、`backend/internal/handler/ai_settings.go`、`backend/internal/handler/ai_settings_test.go`
  - `backend/cmd/fusion/main.go`（启动时加载 `ai_settings` 合并配置并注入 `Evaluator` / `Puller`）
- **前端轻量设置 Tab**：
  - `frontend/src/components/settings/settings-dialog.tsx`
  - `frontend/src/components/settings/ai-settings-tab.tsx`（新建）
  - `frontend/src/lib/api/types.ts`、`frontend/src/lib/api/index.ts`
  - `frontend/src/lib/i18n/messages/**`
- **部署样例与任务交接**：
  - `deploy/docker-compose.yml`、`deploy/.env.example`
  - *交接维护*：认领及结束时仅允许更新本任务卡 `tasks/P2-02b-decision-apis-and-ai-settings.md` 状态与 `## Handoff Report`，以及 `tasks/INDEX.md` 中的 `P2-02b` 对应行

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/**`、`tasks/README.md`、`tasks/_TEMPLATE.md`
- `backend/internal/triage/**`、`backend/internal/store/settings.go`、`backend/internal/handler/settings.go`
- `frontend/src/lib/settings-sync.ts`、`plugins/**`

### Forbidden / Out-of-Scope（严禁触碰）
- 未列在 `Owned Files` 内的所有其他文件
- **高风险禁区**：`backend/go.mod`、`backend/go.sum`、`backend/internal/store/settings.go`、`backend/internal/handler/settings.go`、`frontend/src/lib/settings-sync.ts`、`plugins/**`、`Dockerfile`、`Temp/**`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）
- [ ] **四大原生 Decisions API 预设 + 兼容模式全部经 `httptest.Server` 实测验证**：
  - `gemini_decision`：校验 `x-goog-api-key` 请求头（无 `?key=`）、`input` 键、`questions` map、`answers` 解析及 `choice`/`value` 容错；
  - `openai_decision`：校验 `Authorization: Bearer`、`input` 键、`questions[]` 数组、`answers[]` 解析及 `choice`/`value` 容错；
  - `typesafe`：校验 `state` 键与既有行为零回归；
  - `cloudflare_clef`：校验 `account_id` URL 拼接与非法字符拒绝、`{success:true, result:{answers}}` 解包、`success:false` 错误脱敏；
  - `openai` / `http_json` / `off` 保持 100% 兼容。
- [ ] **SQLite 持久化、脱敏与零重启热更新闭环**：
  - 启动时 `008_ai_settings.sql` 自动应用，DB 覆盖优先于环境变量，`NULL` 正确回落环境变量；
  - `GET`/`PATCH /api/v1/ai/settings` 均要求读者鉴权（插件 Token 返回 `401`），**绝不返回明文密钥**，留空不覆盖旧密钥，`clear_decision_api_key` 与 `reset_to_env` 行为准确；
  - 保存后 `Evaluator` 与 `Puller.RSSLimit()` 立即热生效且累计统计与 `CLIProvider` 实例不丢失；
  - `POST /api/v1/ai/settings/test` 支持草稿测试、限流防刷并脱敏错误。
- [ ] **SSRF 与重定向安全防线实测通过**：
  - 默认（`AI_ALLOW_PRIVATE_ENDPOINTS=false`）下，向 `PATCH` 或 `/test` 传入 `http://127.0.0.1:...`、`http://169.254.169.254/...`、`http://localhost:...` 均返回 `400` 拒绝；
  - 302 重定向响应被拒绝且不跟随转发鉴权头；
  - 恶意 `auth_header`（`Host`/`Cookie`/含 CRLF）被拒绝；
  - Web API 与 UI 不暴露任何 `AI_CLI_COMMAND` / `AI_CLI_ARGS` 读写能力。
- [ ] **前端构建与类型检查通过**：`SettingsDialog` 新增「AI 分拣」Tab，选中 `cloudflare_clef` 时正确显隐 Account ID 输入框，测试连接、保存、重置交互完整，零 `localStorage` 密钥写入。
- [ ] **所有变更均位于 `Owned Files` 范围内**，全量测试、静态检查、前端构建与 Compose 校验全部通过。

建议核验命令：
```bash
# 1) Go 全量单测与静态检查
cd backend
go test -count=1 ./...
go vet ./...

# 2) 前端类型检查、单测与生产构建
cd ../frontend
pnpm exec tsc --noEmit
pnpm test --run
pnpm build

# 3) Compose 配置语法校验
cd ..
docker compose --env-file /dev/null -f deploy/docker-compose.yml config --quiet

# 4) 文件边界与格式核验
git diff --name-only
git status --short
git diff --check
```

---

## Handoff Report（交付回执）

- **提交状态**：`IN_REVIEW`，2026-11-28 完成执行端自测，等待独立核验。
- **执行端 / 通道**：Codex / 通道 A，分支 `feat/p2-02b-decision-apis-and-ai-settings`，基线 `a8cd324`。
- **交付**：四种原生 L2 预设共用三问定义，保留既有兼容模式。读者可通过「设置 → AI 分拣」测试草稿、保存配置并恢复环境默认值。SQLite 保存部分覆盖，热更新沿用原 L3 实例与计数器。
- **实际改动文件清单**（含新增文件，共 40 个）：
  - `backend/cmd/fusion/main.go`
  - `backend/internal/ai/README.md`
  - `backend/internal/ai/config.go`
  - `backend/internal/ai/config_test.go`
  - `backend/internal/ai/decision.go`
  - `backend/internal/ai/decision_config.go`
  - `backend/internal/ai/decision_native.go`
  - `backend/internal/ai/decision_native_test.go`
  - `backend/internal/ai/decision_test.go`
  - `backend/internal/ai/evaluator.go`
  - `backend/internal/ai/evaluator_test.go`
  - `backend/internal/ai/settings.go`
  - `backend/internal/ai/stats.go`
  - `backend/internal/handler/ai_settings.go`
  - `backend/internal/handler/ai_settings_test.go`
  - `backend/internal/handler/handler.go`
  - `backend/internal/model/model.go`
  - `backend/internal/pkg/httpc/client.go`
  - `backend/internal/pkg/httpc/ssrf.go`
  - `backend/internal/pkg/httpc/ssrf_test.go`
  - `backend/internal/pull/puller.go`
  - `backend/internal/store/ai_settings.go`
  - `backend/internal/store/ai_settings_test.go`
  - `backend/internal/store/migrations/008_ai_settings.sql`
  - `deploy/.env.example`
  - `deploy/docker-compose.yml`
  - `frontend/src/components/settings/ai-settings-tab.tsx`
  - `frontend/src/components/settings/settings-dialog.tsx`
  - `frontend/src/lib/api/index.ts`
  - `frontend/src/lib/api/types.ts`
  - `frontend/src/lib/i18n/messages/de.ts`
  - `frontend/src/lib/i18n/messages/en.ts`
  - `frontend/src/lib/i18n/messages/es.ts`
  - `frontend/src/lib/i18n/messages/fr.ts`
  - `frontend/src/lib/i18n/messages/pt.ts`
  - `frontend/src/lib/i18n/messages/ru.ts`
  - `frontend/src/lib/i18n/messages/sv.ts`
  - `frontend/src/lib/i18n/messages/zh.ts`
  - `tasks/INDEX.md`
  - `tasks/P2-02b-decision-apis-and-ai-settings.md`
- **实测验证命令与观察结果**：
  - WSL Debian，`backend/` 执行 `go test -count=1 ./...`，9 个含测试包通过，3 个包无测试文件。`go vet ./...` 退出码 0。
  - 本地 `httptest` 覆盖四个原生协议、旧适配器、禁止重定向与私网拨号。处理器测试覆盖读者鉴权、字段校验、密钥掩码、留空保留、清除、恢复、RSS 在线调整与草稿探测限流。存储测试覆盖数据库重开与 Linux 文件权限，Evaluator 测试覆盖重载后 L3 CLI 实例与计数连续。
  - `frontend/` 执行 `pnpm run typecheck`、`pnpm exec tsc -p tsconfig.app.json --noEmit`、`pnpm build`，均退出 0。`pnpm test --run` 通过 8 个文件、40 项测试。`pnpm run check:i18n` 确认所有语言词典完整，每种 394 个键。
  - Edge 通过生产前端、真实 Go 路由及临时 SQLite 数据库完成交互验收。切换 Cloudflare 时账户栏正确显隐；HTTP JSON 草稿通过本地模拟服务返回 `pass/high/ai`，测试后 RSS 预算仍为 13。保存后读回 17，输入框已清空，接口仅显示掩码，浏览器 localStorage 未出现测试密钥。确认恢复后预算回到 13，390px 视口无页面横向溢出。
  - 在 WSL 执行 `docker compose --env-file /dev/null -f deploy/docker-compose.yml config --quiet`，为 Compose 必填的 `FUSION_PASSWORD`、`INGEST_TOKEN`、`X_AUTH_TOKEN` 传入临时占位值，退出 0。
  - `git status --porcelain` 核对 40 个任务变更均在 Owned Files 内，`git diff --check` 通过。原有未跟踪目录 `Temp/` 保持原状。
- **遗留事项**：
  - 外部厂商协议按任务卡与本地 HTTP 测试服务验收，真实账户的网络、鉴权及配额仍需在部署环境用「测试连接」核验。
  - 仓库的 TypeScript 7 对根配置中旧的 `baseUrl` 报 TS5102，直接运行 `pnpm exec tsc --noEmit` 失败；项目自带的引用工程类型检查与生产构建均通过。附加 ESLint 检查在加载插件时报告当前 typescript-eslint 不支持 TS 7。两个工具链问题涉及本卡只读的前端配置或依赖，留给协调者另行处理。
  - 等待独立核验及合入。
