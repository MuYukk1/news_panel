# `P2-02`：三级递进 AI 分拣管线（L1 规则 → 可选 L2 云端 Decision 门控 → L3 主模型精筛）与 RSS `ai_enabled` 接入

## 元信息（Metadata）
- **Task ID**：`P2-02`
- **Phase / Milestone**：`Phase 2 (v0.2 核心处理管线升级)`
- **前置门禁（Gate Dependency）**：`Gate 1 已于 2026-11-20 批准（DEC-006 / DEC-007 已锁定）+ P2-01 已 DONE 并合入 main`
- **当前状态（Status）**：`TODO`（可选：`TODO` | `IN_PROGRESS` | `IN_REVIEW` | `DONE` | `BLOCKED`）
- **调度通道与目标执行端（Channel / Runner）**：`通道 A / 通道 B 均可（纯 Go 后端切片 + 最小运维样例，边界干净）`
- **工作区 / 分支策略（Worktree / Branch）**：**严禁直接在 `main` 提交代码**。须在独立分支或 worktree 执行：
  - `git checkout -b feat/p2-02-three-stage-ai-pipeline`（或 `git worktree add ../news_panel-p2-02 -b feat/p2-02-three-stage-ai-pipeline`）
- **安全回滚锚点**：基线提交 `0db31f3`（标签 `baseline-pre-p2-02` / GHCR 镜像 `sha-0db31f3`）。本任务**零数据库结构变更**、**零外部三方依赖新增**、完全向后兼容既有 `AI_*` 环境变量（未配置 `AI_DECISION_*` 时自动跳过 L2，保持原有两段式行为零回归）。

---

## 目标与上下文（Objective & Context）
- **核心目标**：将现有两段式 AI 管线（L1 本地规则 → 主模型）升级为**三级递进处理管线**，在保持 1C1G–2C2G 微型 VPS 零本地重推理红线的前提下显著降低主模型（L3）调用量与延迟：
  1. **L1（本地程序规则层）**：沿用并保留零成本广告/水帖/转推/白名单/黑名单粗筛，命中 `filtered` 直接结束，不消耗任何云端 API；
  2. **L2（可选云端轻量 Decision 门控层）**：通过可插拔 `DecisionProvider` 云端 HTTP 抽象（兼容 OpenAI-compatible 轻量模型与通用云端 REST JSON 决策 API，不绑定特定供应商，保留 `DEC-008 [TBD]` 灵活性），仅判定**「放行/拦截」与初判优先级（`critical / high / normal / low`）及粗领域**；
     - **显式支持跳过与关闭（硬需求）**：**未配置 Decision API（URL/Key 缺失）时自动静默跳过 L2 直达 L3；或显式设置 `AI_DECISION_ENABLED=false` / `AI_DECISION_PROVIDER=off` 时彻底关闭 L2 层**，不得报错或阻塞启动；
     - 仅当 L2 判定 `pass` 且初判优先级命中 `AI_DECISION_L3_PRIORITIES`（默认 `critical,high,normal`）时，才放行至 L3 主模型；判 `filter` 直接拦截，判 `low`（未达 L3 门槛）则按配置跳过 L3 减少主模型工作量；
  3. **L3（主 LLM 精筛总结层）**：沿用现有 `openai` / `cli` 主模型能力，结合 L2 初判线索执行完整 `ai_score (0–100)`、中文高密度摘要 `ai_summary`、`ai_tags`、最终 `priority` 与最终放行/拦截裁定；
  4. **打通 `P2-01` 预埋的原生 RSS 单源 `ai_enabled` 开关（方案 A 闭环）**：在 `backend/internal/pull/puller.go` 中对 `ai_enabled=true` 的原生 RSS 新入库条目调用三级 `TriageEvaluator`；保持 `ai_enabled=false` 的 RSS 源 100% 零 AI 消耗、继续归入「未打分 (`unscored`)」；
  5. **轻量内存统计观测接口**：新增 `GET /api/v1/ai/stats`（受读者鉴权保护），返回运行期各层分流与省流计数（L1 拦截、L2 开关状态/拦截/跳过 L3/降级直通、L3 调用/放行/拦截/降级），方便验证三级管线降本效果。
- **前置输入 / 依赖任务**：
  - `P1-03` 已在 `backend/internal/ai/` 落地纯 Go 标准库 L1+L2 管线、`OpenAIProvider`、`CLIProvider` 及防御式 JSON 解析。
  - `P2-01` 已落地 `feeds.ai_enabled`（默认 `0`）、`triage_status=unscored` 分桶与前端开关 UI。
  - 后续 `P2-03` 将在此基础上注入「有界用户个性化档案」，因此本任务须保持 L2 / L3 Prompt 构造模块化，避免耦合硬编码。

---

## 关联文档与约束（References & Constraints）
开始修改代码前必须按顺序阅读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)（重点核对 `DEC-004`、`DEC-006`、`DEC-007`、`DEC-008`）
4. [docs/PRD.md](../docs/PRD.md)
5. [docs/ROADMAP.md](../docs/ROADMAP.md)
6. [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)
7. [tasks/README.md](./README.md)

本任务硬性约束：
- [x] **严守 `DEC-008 [TBD]` 边界**：不写死特定云端 Decision 供应商（如 Jev 私有专有 SDK）；通过纯标准库 `DecisionProvider` 接口 + 通用云端 HTTP 适配器（`openai`、`http_json`）解耦，未来接入任意特定云端 API 仅需新增单文件 Adapter。
- [x] **严守 1C1G VPS 与纯标准库红线**：`backend/internal/ai/` 仅使用 Go 标准库（严禁修改 `backend/go.mod` 引入三方 SDK、本地模型推理或无头浏览器）；共享复用的 `*http.Client`、响应体严格限长（≤ 1 MiB）、全路径密钥脱敏（`redact` 覆盖 `AI_API_KEY` 与 `AI_DECISION_API_KEY`，严禁泄露到日志、错误或 `/api/v1/ai/stats`）。
- [x] **严守 `P2-03` / `P2-04` 边界**：本任务**禁止**新增用户反馈表/档案表、**禁止**修改前端 UI（`frontend/**` 只读）、**禁止**修改 `plugins/**`。
- [x] **零回归向后兼容**：既有仅配 `AI_*`（不配 `AI_DECISION_*`）的部署必须 100% 保持原行为；`triage.TriageEvaluator` 接口签名不变；所有既有 Go 测试不破坏。

---

## 功能与工程详规（Detailed Spec）

### 模块 1：配置体系扩展（`backend/internal/ai/config.go`）
在现有 `Config` 结构体中新增 L2 Decision 门控配置字段（全部通过环境变量读取，绝不读/写 `.env` 文件；非法值告警并回退安全默认，绝不阻塞启动）：

| 环境变量 | 类型 / 默认值 | 说明 |
| :--- | :--- | :--- |
| `AI_DECISION_ENABLED` | `bool`（缺省自动检测） | 未设置时：仅当配置了有效 Decision 端点/凭证（`openai` 需 `AI_DECISION_API_KEY`，`http_json` 需 `AI_DECISION_BASE_URL`）才自动开启，否则为 `false`（静默跳过 L2）；显式设 `false` 时强制关闭 L2 |
| `AI_DECISION_PROVIDER` | `string`（默认 `openai`） | 可选：`openai` \| `http_json` \| `off`（`none` 等同 `off`，直接关闭 L2） |
| `AI_DECISION_BASE_URL` | `string`（`openai` 默认 `https://api.openai.com/v1`；`http_json` 默认空） | 云端轻量 Decision API 地址 |
| `AI_DECISION_API_KEY` | `string`（默认空，密钥脱敏） | Decision API 凭证（严禁出现在日志/错误/统计中） |
| `AI_DECISION_AUTH_HEADER` | `string`（默认 `Authorization`） | 仅 `http_json` 使用：自定义鉴权头名称；为 `Authorization` 时自动补 `Bearer ` 前缀（若值未含空格），其他头名（如 `X-API-Key`）直接传原值 |
| `AI_DECISION_MODEL` | `string`（默认 `gpt-4o-mini`） | Decision 模型标识（透传到请求体 `model` 字段） |
| `AI_DECISION_JSON_FORMAT` | `bool`（默认 `false`） | `openai` 模式下是否附加 `response_format: {"type":"json_object"}` |
| `AI_DECISION_TIMEOUT_SECONDS` | `int`（默认 `6`） | Decision 单次请求超时秒数（正整数） |
| `AI_DECISION_L3_PRIORITIES` | `string`（默认 `critical,high,normal`） | 逗号分隔的优先级白名单：仅 L2 判定 `action=pass` 且 `priority` 命中此集合的条目才送入 L3 主模型 |
| `AI_DECISION_LOW_ACTION` | `string`（默认 `keep_unscored`） | L2 判 `pass` 但优先级未命中 `AI_DECISION_L3_PRIORITIES`（例如 `low`）时的动作：`keep_unscored`（`triage_status=kept`, `ai_score=nil`, `tags=["l2-decision-bypass"]`，落入 `P2-01`「未打分」分桶不污染高分精选）或 `filter`（`triage_status=filtered`, `tags=["l2-decision-low-filtered"]`） |
| `AI_DECISION_FALLBACK_MODE` | `string`（默认 `skip_to_l3`） | L2 网络/超时/解析失败时的降级策略：`skip_to_l3`（fail-open 直接交给 L3，宁可多花一次 L3 也不漏突发要闻） \| `keep_l1`（直接采用 L1 结果并标 `l2-decision-fallback`） \| `fail`（返回脱敏错误记为 `failed`） |
| `AI_RSS_MAX_ITEMS_PER_PULL` | `int`（默认 `20`，`0` 表示不限制） | 单个已开启 `ai_enabled=true` 的 RSS 源单次拉取最多送入 AI 评估的新条目数（按最新优先）；超出上限的新条目直接以 `triage_status='kept'`、`ai_score=NULL` 落入「未打分」，防止大流量/首次拉取 RSS 瞬间打爆模型额度 |

> **关键边界**：若 `AI_ENABLED=false` 或主模型 `AI_PROVIDER=l1`，整条 AI 管线按原约定退化为 L1-only（不单独跑 L2）；若主模型已配置（`openai` / `cli`），而 `AI_DECISION_ENABLED=false` 或未配置 Decision 凭证/URL，则跳过 L2、直接由 L1 → L3，并在启动日志中打印清晰 `INFO` 提示（例如 `"ai L2 decision gate disabled/unconfigured; bypassing L2 directly to L3"`）。

---

### 模块 2：可插拔云端 Decision Provider 与解析（`backend/internal/ai/decision.go`）
1. **核心抽象**：
   ```go
   type DecisionOutcome struct {
       Action   string // "pass" | "filter"
       Priority string // "critical" | "high" | "normal" | "low"
       Domain   string // "" | "investing" | "ai" | "deals" | "other"
       Reason   string // 可选简短原因（截断 ≤ 64 rune，已脱敏）
   }

   type DecisionProvider interface {
       Name() string
       Decide(ctx context.Context, in triage.Input, l1 triage.Result) (DecisionOutcome, error)
   }
   ```
2. **适配器 A：`OpenAIDecisionProvider`（`AI_DECISION_PROVIDER=openai`）**：
   - 复用共享 `*http.Client` 与 `chatCompletionsURL`；
   - 独立极简门控 Prompt（不要求写长摘要、不要求打 0–100 分，大幅节省输出 Token 与响应时间）：
     - 严格要求单 JSON：`{"action":"pass|filter","priority":"critical|high|normal|low","domain":"investing|ai|deals|other","reason":"15字内简短理由"}`；
     - 若有广告/卖货/水帖/闲聊噪声直接 `action="filter", priority="low"`；突发催化剂/重要发布给 `action="pass", priority="critical|high"`；一般相关给 `normal`。
   - 复用防御式 JSON 提取逻辑解析响应，兼容 `action`（`"pass"/"keep"/"allow"` → `"pass"`；`"filter"/"drop"/"reject"/"block"` → `"filter"`）或布尔字段 `keep: true/false`。
3. **适配器 B：`HTTPJSONDecisionProvider`（`AI_DECISION_PROVIDER=http_json`）**：
   - 面向类似 Jev 或其他自定义云端轻量决策 API 的通用 REST 适配器：
     - `POST AI_DECISION_BASE_URL`（`Content-Type: application/json` + 可选鉴权头）；
     - 请求 JSON 载荷：
       ```json
       {
         "model": "<AI_DECISION_MODEL>",
         "topics": "<AI_TOPICS>",
         "source_type": "...",
         "source_id": "...",
         "author": "...",
         "title": "...",
         "content": "<truncated <= 2000 runes>",
         "url": "...",
         "published_at": "RFC3339 UTC",
         "l1_domain": "...",
         "l1_priority": "..."
       }
       ```
     - 响应 JSON 解析：同时兼容顶层对象 `{"action":"pass","priority":"high","domain":"ai","reason":"..."}` 与常见包装形态 `{"data":{...}}` / `{"decision":{...}}` / `{"result":{...}}`，支持 `action` 或 `keep` 布尔值，缺失 `domain` 时安全回退 `l1.Domain`，缺失 `priority` 时按 `action=="filter"` 回退 `"low"`、否则 `"normal"`。
   - **安全护栏**：响应体 `io.LimitReader(resp.Body, 1<<20)`；所有错误均经 `redact(..., apiKey)` 脱敏。

---

### 模块 3：三级递进编排与统计（`backend/internal/ai/evaluator.go` + `stats.go`）
1. **完整执行流 `Evaluator.Evaluate(ctx, in)`**：
   ```text
   Input
     ├─▶ [L1 本地规则]
     │     ├─ filtered ──▶ 统计 L1Filtered++，直接返回（0 云端调用）
     │     └─ kept ──────▶ 若 L3 provider == nil（L1-only）──▶ 返回 l1（tag: "l1-only"）
     │
     ├─▶ [L2 可选云端 Decision 门控]
     │     ├─ 未启用 / 未配置 ──▶ 统计 L2Skipped++，直通 L3
     │     └─ 已启用 ──▶ decisionProvider.Decide(ctx, in, l1)
     │           ├─ 出错/超时（非 ctx 取消）：统计 L2Errors++
     │           │    ├─ skip_to_l3（默认）──▶ 告警日志，直通 L3
     │           │    ├─ keep_l1 ──▶ 返回 l1（tag: "l2-decision-fallback"）
     │           │    └─ fail ──▶ 返回脱敏 error
     │           ├─ action == "filter" ──▶ 统计 L2Filtered++，返回 filtered（ai_score=nil, tag: "l2-decision-filtered"，跳过 L3）
     │           └─ action == "pass" ──▶ 统计 L2Passed++
     │                ├─ priority 未命中 L3 白名单 ──▶ 统计 L2BypassedL3++，按 AI_DECISION_LOW_ACTION 返回（默认 kept + ai_score=nil + tag: "l2-decision-bypass"，跳过 L3）
     │                └─ priority 命中 L3 白名单 ──▶ 将 L2 初判 domain/priority 作为线索传入 L3
     │
     └─▶ [L3 主模型精筛与总结]
           ├─ 统计 L3Called++，调用现有 CompletionProvider（openai / cli）
           ├─ 成功 ──▶ 统计 L3Kept++ / L3Filtered++，返回完整评分、中文摘要、标签、最终优先级与裁定
           └─ 失败 ──▶ 统计 L3Fallback++，沿用现有 AI_FALLBACK_MODE（keep_l1 / fail）
   ```
2. **提示词模块化（为 `P2-03` 预留清晰扩展点）**：
   - 将 L3 提示词升级：当经过 L2 放行时，在 `user` prompt 中附带 `L2 门控初判：priority=%s domain=%s`（未经过 L2 时保持原格式不变）；
   - 保持 `decisionPrompts` 与 `l3Prompts` 为独立辅助函数，方便 `P2-03` 后续无侵入注入有界个性化档案。
3. **运行期原子统计（`Stats()` + `GET /api/v1/ai/stats`）**：
   - 使用 `sync/atomic` 维护轻量无锁计数器（零内存膨胀、并发安全）：
     ```go
     type PipelineStats struct {
         Enabled          bool   `json:"enabled"`
         L2Enabled        bool   `json:"l2_enabled"`
         L2Provider       string `json:"l2_provider"`
         L3Provider       string `json:"l3_provider"`
         L1Filtered       int64  `json:"l1_filtered"`
         L2Skipped        int64  `json:"l2_skipped"`
         L2Passed         int64  `json:"l2_passed"`
         L2Filtered       int64  `json:"l2_filtered"`
         L2BypassedL3     int64  `json:"l2_bypassed_l3"`
         L2Errors         int64  `json:"l2_errors"`
         L3Called         int64  `json:"l3_called"`
         L3Kept           int64  `json:"l3_kept"`
         L3Filtered       int64  `json:"l3_filtered"`
         L3Fallback       int64  `json:"l3_fallback"`
     }
     ```
   - 在 `backend/internal/handler/handler.go` 的 `auth` 路由组新增 `GET /api/v1/ai/stats`（通过可选接口 `interface{ Stats() ai.PipelineStats }` 或轻型 stats getter 从 `h.evaluator` 读取，未启用时返回默认零值结构体，绝不包含任何密钥或 URL 凭证）。

---

### 模块 4：打通原生 RSS 单源 `ai_enabled` 开关（`store/item.go` + `pull/puller.go` + `cmd/fusion/main.go`）
完成 `DEC-007` 方案 A 闭环：
1. `backend/internal/store/item.go`：
   - 保留 `BatchCreateItemsIgnore` 签名与行为不变；
   - 新增 `BatchCreateItemsForTriage(feedID int64, inputs []BatchCreateItemInput) ([]CreatedItem, error)`：仅在 `feed.AIEnabled == true` 时使用，以 `triage_status = 'pending'` 批量插入（`ON CONFLICT(feed_id, guid) DO NOTHING`）并返回本次**真正新插入**的条目列表（`ID` + 对应 `BatchCreateItemInput`），避免重复评估已入库旧条目！
2. `backend/internal/pull/puller.go`：
   - 新增 `SetEvaluator(eval triage.TriageEvaluator)`（不破坏 `pull.New(st, cfg)` 现有签名）；
   - 在 `pullFeed` 中：
     - 当 `!feed.AIEnabled || p.evaluator == nil`：完全保持原有逻辑（调用 `BatchCreateItemsIgnore`，零 AI 消耗，归入 `unscored`）；
     - 当 `feed.AIEnabled && p.evaluator != nil`：按发布时间新→旧最多取前 `AI_RSS_MAX_ITEMS_PER_PULL` 条（默认 20 条，超出部分仍走 `BatchCreateItemsIgnore` 保留为「未打分」不丢失也不爆额）调用 `BatchCreateItemsForTriage` 插入新条目，逐条构造 `triage.Input`（`SourceType: "rss"`, `ExternalID: item.GUID`, `URL: item.Link`, `Title: item.Title`, `Content: item.Content`, `Author: feed.Name`, `PublishedAt: time.Unix(item.PubDate, 0).UTC()`）调用 `p.evaluator.Evaluate(evalCtx, input)` 并 `p.store.UpdateTriage(evalCtx, created.ID, result)`；若单条评估失败或校验失败，记录告警并将该条更新为 `triage_status = 'failed'`（不中断本源其余新条目，也不影响 RSS 抓取状态成功记录）。
   - 注意：`pullFeed` 中 RSS 评估使用合理超时保护（或继承 `ctx`），不因单条 AI 慢请求卡死整个 puller 信号量。
3. `backend/cmd/fusion/main.go`：
   - 共用同一个 `eval := ai.NewFromEnv()` 实例注入 `puller.SetEvaluator(eval)` 与 `handler.New(st, cfg, puller, eval)`，确保外置插件与开启 AI 的 RSS 源共享同一份 CLI 冷却锁与统计计数。

---

### 模块 5：部署配置透传与文档更新（`deploy/` + `backend/internal/ai/README.md`）
- `deploy/docker-compose.yml`：在 `news-panel` 服务的 `environment` 块透传全部可选 `AI_DECISION_*` 环境变量（均带安全空/关默认值，老 `.env` 零改动即可平滑启动）；
- `deploy/.env.example`：补充清晰的 L2 Decision 门控可选配置段落及注释（标明不填即自动跳过 L2）；
- `backend/internal/ai/README.md`：更新为三级管线架构说明、环境变量表、关闭/跳过 L2 行为说明与 `/api/v1/ai/stats` 说明。

---

## 文件边界（Owned / Read-Only / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `backend/internal/ai/**`（核心三级管线、Decision Provider 适配器、统计、单测与 `README.md`）
- `backend/internal/store/item.go` 及 `backend/internal/store/item_test.go`（或新增 `backend/internal/store/rss_triage_test.go`）
- `backend/internal/pull/puller.go`、`backend/internal/pull/puller_test.go`
- `backend/internal/handler/handler.go`、`backend/internal/handler/plugin.go`（或新建 `backend/internal/handler/ai_stats.go` + 对应 `*_test.go`）
- `backend/cmd/fusion/main.go`（最小接入：共用 `eval` 实例并注入 `puller.SetEvaluator(eval)`）
- `deploy/docker-compose.yml`、`deploy/.env.example`（仅透传/注释新增可选 `AI_DECISION_*` 环境变量）
- *交接维护*：认领及结束时仅允许更新本任务卡 `tasks/P2-02-three-stage-ai-pipeline.md` 状态与 `## Handoff Report`，以及 `tasks/INDEX.md` 中的 `P2-02` 对应行

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/**`、`tasks/README.md`、`tasks/_TEMPLATE.md`
- `backend/internal/triage/**`、`backend/internal/model/**`、`backend/internal/store/migrations/**`
- `frontend/**`、`plugins/**`

### Forbidden / Out-of-Scope（严禁触碰）
- 未列在 `Owned Files` 内的所有其他文件
- **高风险禁区**：`backend/go.mod`、`backend/go.sum`、`backend/internal/store/migrations/**`、`frontend/**`、`plugins/**`、`Dockerfile`、`Temp/**`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）

- [ ] **未配置或显式关闭 L2 时自动跳过**：
  - 未配置 `AI_DECISION_API_KEY` / `AI_DECISION_BASE_URL` 时，`l2_enabled=false`，条目经 L1 后直接进入 L3，`l2_skipped` 递增，零报错；
  - 已配置凭证但设 `AI_DECISION_ENABLED=false` 或 `AI_DECISION_PROVIDER=off` 时，同样彻底关闭 L2、直接由 L1 → L3。
- [ ] **三级门控有效削减 L3 调用**：
  - L1 拦截条目：0 次 L2 调用、0 次 L3 调用；
  - L2 判 `filter` 条目：返回 `filtered`（`ai_score=nil`、`l2-decision-filtered`），0 次 L3 调用；
  - L2 判 `pass` 且优先级未命中 `AI_DECISION_L3_PRIORITIES`（如 `low`）：按 `AI_DECISION_LOW_ACTION` 跳过 L3（默认 `keep_unscored` 落入未打分分桶），0 次 L3 调用；
  - L2 判 `pass` 且命中 `critical / high / normal`：携带 L2 初判线索进入 L3 完成打分与中文摘要；
  - L2 调用异常/超时/畸形 JSON：按 `AI_DECISION_FALLBACK_MODE` 处理（默认 `skip_to_l3` fail-open 不丢要闻），且错误信息严格脱敏不泄露 `AI_DECISION_API_KEY`。
- [ ] **双云端适配器就绪**：`openai` 与 `http_json` 两种 `DecisionProvider` 均通过 `httptest.Server` 单元测试验证（含自定义鉴权头、包装 JSON 响应、`keep` 布尔兼容、超长响应截断与脱敏）。
- [ ] **原生 RSS `ai_enabled` 闭环与防洪护栏**：`puller` 对 `ai_enabled=false` 的 RSS 源不触发评估（零消耗）；对 `ai_enabled=true` 的 RSS 源仅对新插入条目运行三级评估并受 `AI_RSS_MAX_ITEMS_PER_PULL` 单轮上限保护（超限条目保留为未打分），重复拉取同一 GUID 不重复消耗 AI。
- [ ] **观测接口与部署透传**：`GET /api/v1/ai/stats` 需要读者鉴权（插件 Token 返回 `401`），返回准确计数且零密钥泄露；`deploy/docker-compose.yml` 与 `deploy/.env.example` 已补齐可选 `AI_DECISION_*` 变量。
- [ ] **全量测试通过 & 边界纯净**：
  ```bash
  # 1) Go 全量单测（含竞态检查建议）
  cd backend
  go test -count=1 ./...

  # 2) 边界核验
  git diff --name-only
  git status --short
  ```

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
