# `P1-03`：核心内置 L1+L2 AI 过滤与分拣管线（`backend/internal/ai/`）

## 元信息（Metadata）
- **Task ID**：`P1-03`
- **Phase / Milestone**：`Phase 1 (v0.1 纵向切片)`
- **前置门禁（Gate Dependency）**：`Gate 0 已于 2026-11-15 批准`，且核心底座任务 `P1-01` 已合入 `main`（commit `20c1a8e`，已提供 `backend/internal/triage.TriageEvaluator` 接口与 `handler.New(..., evaluator)` 注入点）
- **当前状态（Status）**：`DONE`（2026-11-16，通道 A / Qoder 开发完成，协调端已在 WSL Debian Linux 独立核验全量 Go 测试与编译通过，已合入 `main`）
- **调度通道与目标执行端（Channel / Runner）**：`通道 A：Qoder`（ZCode 额度已耗尽，切由 Qoder 按本任务卡接力开发）
- **工作区 / 分支策略（Worktree / Branch）**：`单工作区串行（直接在 main 工作或自建 feat/p1-03-ai-triage 分支，严格限定在下方 Owned Files 内改动）`

---

## 目标与上下文（Objective & Context）
- **核心目标**：基于 `P1-01` 预留的 `triage.TriageEvaluator` 契约接口，在 `backend/internal/ai/` 下以**纯 Go 标准库（零新增第三方依赖）**实现 `news_panel` v0.1 两段式 AI 信息分拣管线（L1 轻量规则粗筛 + L2 云端 OpenAI-compatible 精筛 + 可选受控 CLI 适配器），并在 `backend/cmd/fusion/main.go` 完成最小装配接入；交付后即可在 Linux 测试机端到端实测 `X 采集 -> 入库 -> AI 过滤 -> 面板展示`。
- **前置输入 / 依赖任务**：
  - 协议与决策基线：[docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md) 第 4/5 节、`DEC-004`（[docs/DECISIONS.md](../docs/DECISIONS.md)）；
  - 既有核心接入边界：[backend/internal/triage/triage.go](../backend/internal/triage/triage.go)、[backend/internal/handler/handler.go](../backend/internal/handler/handler.go#L40-L55)、[backend/internal/handler/plugin.go](../backend/internal/handler/plugin.go#L242-L254)、[backend/cmd/fusion/main.go](../backend/cmd/fusion/main.go#L44-L59)。

---

## 关联文档与约束（References & Constraints）
开始前**必须按顺序阅读**：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)（重点核对 `ADR-001`–`ADR-004`、`DEC-002`、`DEC-004`）
4. [docs/PRD.md](../docs/PRD.md) & [docs/ROADMAP.md](../docs/ROADMAP.md)
5. [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)
6. [tasks/README.md](./README.md) 与本任务卡

本任务硬性约束：
- [x] **严守 Phase 1 边界**：不提前做多信源、不新增 `POST /api/v1` 路由，不实现 Gate 1 后的外置 Processor Webhook 钩子（`DEC-006` 仍为 `[TBD]`）
- [x] **零新增第三方依赖**：严禁修改 `backend/go.mod` 与 `backend/go.sum`（全部使用 Go 标准库 + 仓库既有 `news_panel/internal/triage`，保证零 GPL/AGPL 风险与极小二进制体积）
- [x] **1C1G–2C2G 微型 VPS 资源红线**：常驻内存近似零增量；复用单个 `*http.Client` 连接池；CLI 适配器必须严格限制为单并发串行 + 超时保护
- [x] **安全与隐私红线**：严禁读取、修改或打印 `.env` 内容；错误与日志中严禁回显 `AI_API_KEY` 或原始凭证；不编写任何反代/多号刷分/绕过登录限制逻辑
- [x] **同步超时与降级安全**：`handler/plugin.go` 在单次 `POST /api/v1/ingest` 内同步循环调用 `Evaluate`；且当 `Evaluate` 返回 `err != nil` 时条目标记为 `triage_status="failed"` 并从默认列表隐藏。因此必须设计完善的超时护栏与无 Key/异常安全降级！

---

## 详细工程要求

### 1. 接口实现与默认降级行为
在 `backend/internal/ai/` 实现满足 `triage.TriageEvaluator` 的核心评估器：
- 输出必须100%通过 `result.Validate()`：
  - `Domain`: 仅限 `"investing"` | `"ai"` | `"deals"` | `"other"`
  - `AIScore`: `*int`（`0..100`；未走 L2 模型评分的兜底放行条目设为 `nil`，**不要伪造假分数**）
  - `AITags`: 始终非 `nil` 字符串切片（如 `[]string{}`，单条建议不超过 6 个短标签）
  - `AISummary`: 一句话高密度中文摘要（或 L1 过滤简要理由）
  - `Priority`: 仅限 `"critical"` | `"high"` | `"normal"` | `"low"`
  - `TriageStatus`: `"kept"` 或 `"filtered"`
- **关键降级规则（防止测试/无 Key 时所有推文全部变为隐藏的 `failed`）**：
  1. 当 `AI_ENABLED=false`，或 `AI_PROVIDER=l1`，或 `AI_PROVIDER=openai` 但 `AI_API_KEY` 为空时：**不报错、不阻塞启动**，退化为仅执行 L1 规则层——明显噪音判 `"filtered"`，其余条目返回 `"kept"` + L1 初步 `Domain` / `Priority` 猜测（`AIScore: nil`）；
  2. 当 `AI_ENABLED=true` 且 L2 调用失败（网络错误 / 超时 / 429 / 5xx / JSON 无法解析）时，支持 `AI_FALLBACK_MODE`：
     - 默认 `"keep_l1"`：记录脱敏警告日志，降级返回 L1 判定结果并附加 `"l2-fallback"` 标签（保证重要信息不丢失）；
     - 可选 `"fail"`：返回非 `nil` error，让 `handler/plugin.go` 按契约落库为 `triage_status = "failed"` 供严格审计或故障演练。
  3. 必须尊重 `ctx.Err() != nil`（若传入的 context 已取消，立即返回 `ctx.Err()`）。

### 2. Stage 1：L1 轻量规则粗筛（`l1.go`，< 1ms 本地执行）
针对高噪声社交媒体（尤其是 X）先本地拦截低价值信息，减少 L2 Token 与时延消耗：
1. **明显噪声过滤（命中即直接返回 `TriageStatus: "filtered"`, `Priority: "low"`, `Domain: "other"`, `AIScore: nil`, 标签带 `"l1-noise"`）**：
   - 去除 URL/空白/@mention 后正文有效字符过短（默认 `AI_L1_MIN_CONTENT_RUNES=10`，且既无高价值外链也无标题信息；注意若内容仅为占位 `"(media-only tweet)"` 且正文过短可按配置处理）；
   - 默认拦截典型无信息水帖/互粉抽奖/纯广告引流关键词（可配合 `AI_L1_BLOCK_KEYWORDS` 逗号分隔自定义黑名单），**注意严禁误杀“薅羊毛 / 限免 / VPS 优惠 / 折扣码 / 空投规则”等真实 `deals` 信号**；
   - 可选配置 `AI_L1_FILTER_RETWEETS=true|false`（默认 `false`：交由 L2 评估或按 `input.RawMeta["is_retweet"]` 处理）。
2. **三域规则识别与白名单放行**：
   - 支持 `AI_L1_ALLOW_KEYWORDS`（强制不被 L1 误拦截，直通 L2 或直接放行）；
   - 基于轻量关键词词典给出初始 `Domain`（`investing` / `ai` / `deals` / `other`）与初步优先级提示，既可作为无 LLM 时的兜底结果，也能作为上下文线索提供给 L2。

### 3. Stage 2：L2 AI 精筛与双 Provider 适配器（`provider.go`, `openai.go`, `cli.go`, `evaluator.go`）
1. **Provider 抽象**：
   ```go
   type CompletionProvider interface {
       Name() string
       Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
   }
   ```
2. **主通道 `openai`（默认，`DEC-004`）**：
   - 对接任意 OpenAI-compatible `POST /chat/completions`（依据 `AI_BASE_URL` 智能拼接路径：若已以 `/chat/completions` 结尾则直接使用，若含 `/v1` 补 `/chat/completions`，否则补 `/v1/chat/completions`）；
   - Header 携带 `Authorization: Bearer <AI_API_KEY>`、`Content-Type: application/json`；
   - 复用单实例 `*http.Client`，受 `AI_TIMEOUT_SECONDS`（默认 `8` 秒）单次超时控制；
   - 支持 `AI_OPENAI_JSON_FORMAT`（默认 `false` 或可配 `true`；为 `true` 时附加 `response_format: {"type": "json_object"}`，关闭时兼容所有不支持该字段的中转/开源模型）；
   - Prompt 要求评估用户三大核心情报域（`investing`：投资/财报/宏观催化剂；`ai`：AI 官方动态/前沿发布/内测灰度泄露；`deals`：高时效薅羊毛/限免/Bug 价/云主机折扣；`other`：其他），可注入自定义偏好 `AI_TOPICS`，并要求严格输出单段 JSON：
     `{"domain":"investing|ai|deals|other","score":0-100,"tags":["..."],"summary":"一句话高密度中文摘要","priority":"critical|high|normal|low","keep":true|false}`。
3. **可选通道 `cli`（供 Linux 测试机受控调用 `codebuddy -p` 等官方 Headless CLI）**：
   - 使用标准库 `exec.CommandContext` 调用配置的 `AI_CLI_COMMAND`（如 `codebuddy`）与逗号/空格拆分参数 `AI_CLI_ARGS`（例如 `-p,{{prompt}},--output-format,json`；若未包含 `{{prompt}}` 占位符则自动附加到参数末尾）；
   - 必须通过内部 `sync.Mutex` 或容量为 1 的信号量**强制单并发串行执行**，严防 1C1G–2C2G 内存溢出；
   - 兼容解析 CLI 外层 JSON 包裹（例如识别外层对象里的 `result` / `content` / `message` / `text` 字段，提取其中包含分拣字段的 JSON）或直接提取 stdout 中的 JSON 块；
   - 默认单次超时可由 `AI_TIMEOUT_SECONDS` 控制（建议 CLI 测试时可设为 `15`–`20`s）。
4. **鲁棒解析与阈值裁定**：
   - 提取 JSON 时必须兼容：纯 JSON、```` ```json ... ``` ```` 代码围栏、前后混杂说明散文；
   - 对解析结果的每个字段做规范化与范围防御（`score < 0` 夹到 `0`，`> 100` 夹到 `100`；大小写归一化；未知 `domain`/`priority` 回退安全默认值；`tags` 去重截断）；
   - 结合 `AI_KEEP_MIN_SCORE`（默认 `45`）：若模型返回 `keep == false` 或 `score < keepMinScore`（除非显式为 `high`/`critical`），映射为 `triage_status = "filtered"`；否则为 `"kept"`。

### 4. 配置环境变量（`backend/internal/ai/config.go`，不修改 `internal/config`）
全部从 `os.Getenv` 读取并提供安全默认值（同时在 `backend/internal/ai/README.md` 列出配置示例，**不要修改仓库根目录或读取 `.env`**）：
- `AI_ENABLED`（默认 `false`；若设置了 `AI_API_KEY` 且 `AI_ENABLED` 未显式设为 `false` 时可自动视为启用，或在文档写明规则并加单测覆盖 —— 建议：**未显式设置 `AI_ENABLED` 时，有 `AI_API_KEY` 或 `AI_PROVIDER=cli` 则开启，否则自动 L1-only**，最省心！）
- `AI_PROVIDER`（`openai` | `cli` | `l1`，默认 `openai`）
- `AI_BASE_URL`（默认 `https://api.openai.com/v1`）
- `AI_API_KEY`（无默认）
- `AI_MODEL`（默认 `gpt-4o-mini`）
- `AI_TOPICS`（默认聚焦三域：`投资催化剂与宏观突发,AI前沿动态与内测泄露,高时效薅羊毛与云资源优惠`）
- `AI_KEEP_MIN_SCORE`（整数 `0..100`，默认 `45`）
- `AI_TIMEOUT_SECONDS`（单条评估超时秒数，默认 `8`）
- `AI_FALLBACK_MODE`（`keep_l1` | `fail`，默认 `keep_l1`）
- `AI_OPENAI_JSON_FORMAT`（`true` | `false`，默认 `false`）
- `AI_L1_MIN_CONTENT_RUNES`（默认 `10`）
- `AI_L1_FILTER_RETWEETS`（默认 `false`）
- `AI_L1_BLOCK_KEYWORDS` / `AI_L1_ALLOW_KEYWORDS`（逗号分隔）
- `AI_CLI_COMMAND`（如 `codebuddy`） / `AI_CLI_ARGS`（逗号分隔参数列表，支持 `{{prompt}}`）
- `AI_HTTP_WRITE_TIMEOUT_SECONDS`（在 `backend/cmd/fusion/main.go` 中读取并配置 `srv.WriteTimeout`，默认放宽至 `120` 秒，防止多条推文同步评估被 30s 默认超时截断）

### 5. `backend/cmd/fusion/main.go` 最小接入
- 仅在 `run()` 中调用 `aiEvaluator := ai.NewFromEnv()`（或 `ai.NewEvaluatorFromEnv(slog.Default())`）作为第 4 参数传入 `handler.New(st, cfg, puller, aiEvaluator)`；
- 将 `srv.WriteTimeout` 调整为可由 `AI_HTTP_WRITE_TIMEOUT_SECONDS` 控制的更安全值（默认 `120 * time.Second`），保留其余所有既有逻辑不动。

---

## 文件边界（Owned / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `backend/internal/ai/**`
- `backend/cmd/fusion/main.go`（仅限上述 evaluator 装配与 `WriteTimeout` 放宽）
- *(状态回填授权：`tasks/P1-03-ai-triage-pipeline.md` 自身状态区/勾选框/回执区，及 `tasks/INDEX.md` 中 `P1-03` 对应行)*

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/**`、`tasks/README.md`、`backend/README.md`
- `backend/internal/triage/triage.go`、`backend/internal/handler/handler.go`、`backend/internal/handler/plugin.go`、`backend/internal/store/plugin.go`

### Forbidden / Out-of-Scope（严禁触碰）
- `backend/go.mod`、`backend/go.sum`（禁止引入新依赖）
- `backend/internal/handler/**`、`backend/internal/store/**`、`backend/internal/config/**`、`backend/internal/pull/**`、`backend/internal/triage/**`
- `frontend/**`、`plugins/**`、`docs/**`、`AGENTS.md`、`.gitignore`、`Dockerfile`、`LICENSE`、`NOTICE`、`.env`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）
- [x] `backend/internal/ai/` 代码结构清晰，包含 `README.md` 环境变量说明，且完全使用 Go 标准库（`go.mod`/`go.sum` 零变更）
- [x] L1 规则层与 L2 OpenAI-compatible / CLI 双 Provider 完整实现 `triage.TriageEvaluator`，所有返回值均通过 `triage.Result.Validate()`
- [x] 覆盖全场景单元测试（基于 `httptest` mock 与跨平台 Go `TestHelperProcess` fake CLI，不依赖真实外网 Key/外部工具即可跑通）：L1 噪声过滤与三域关键词保护、JSON/围栏/越界响应解析、`AI_KEEP_MIN_SCORE`、无 Key 降级、`keep_l1` vs `fail` 回退策略、CLI 单并发串行与超时控制、错误日志脱敏（不泄漏 `AI_API_KEY`）、`ctx` 取消处理
- [x] `backend/cmd/fusion/main.go` 正确注入 evaluator 且合理放宽 `WriteTimeout`
- [x] 全部代码与文档改动严格落入 `Owned Files`

建议核验命令：
```text
cd backend
go test -count=1 ./internal/ai/... ./cmd/fusion/...
go test -count=1 ./...
git diff --name-only
```
*(提示：若你所在宿主机/环境未预装 Go 1.26 工具链而无法本地运行 `go test`，严禁编造通过结果——请在 Handoff Report 如实标注阻塞/未运行原因，并在本卡列清改动清单后置为 `IN_REVIEW` 或 `BLOCKED` 交由协调端在 Linux/容器环境核验。)*

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`IN_REVIEW`
- **执行端 / 通道**：`通道 A / Qoder`
- **工作分支**：`feat/p1-03-ai-triage`（自 `main` `f11f17f` 切出；改动保留在工作区未提交，未请求 commit，交由核验端审查后合入）
- **实际改动文件清单**（全部落入 Owned Files）：
  - `backend/internal/ai/config.go`（新增）：`Config` 与 `ConfigFromEnv` 全量 `AI_*` 环境变量读取与安全默认值；`AI_ENABLED` 自动探测规则（有 Key 或 provider=cli 则启用）；`HTTPWriteTimeout()`；`redact()` 脱敏工具。
  - `backend/internal/ai/l1.go`（新增）：L1 本地规则粗筛（噪声黑名单 + deals 信号保护 + 白名单/自定义黑名单 + 转推过滤 + 短文本/`(media-only tweet)` 守卫 + 三域关键词词典与优先级提示）；ASCII 词边界 / 中文子串匹配；所有结果 `Validate()` 安全、`AIScore=nil`。
  - `backend/internal/ai/provider.go`（新增）：`CompletionProvider` 抽象接口。
  - `backend/internal/ai/openai.go`（新增）：OpenAI-compatible 主通道，`AI_BASE_URL` 智能拼接、单实例 `*http.Client` 连接池、单次超时、`response_format` 开关、Bearer 鉴权、错误脱敏。
  - `backend/internal/ai/cli.go`（新增）：受控 Headless CLI 适配器，`sync.Mutex` 强制单并发串行 + `exec.CommandContext` 超时；`{{prompt}}` 占位符解析；外层 JSON 包裹（result/content/message/text，含流式多行）解包。
  - `backend/internal/ai/parse.go`（新增）：鲁棒 JSON 提取（纯 JSON/代码围栏/散文混杂，优先含 domain/score 的对象）；verdict 字段规范化与范围防御（score 夹取 0..100、domain/priority 归一化回退、tags 去重截断 ≤6、summary 截断）；keep 阈值裁定（high/critical 例外）。
  - `backend/internal/ai/evaluator.go`（新增）：`Evaluator` 实现 `triage.TriageEvaluator`；`New`/`NewEvaluatorFromEnv`/`NewFromEnv` 装配与安全降级（disabled/l1/无 Key/无 CLI 命令 → L1-only，永不阻塞启动）；L2 失败 `keep_l1`（加 `l2-fallback` 标签、脱敏告警）vs `fail`（返回 error → 落库 failed）；`ctx.Err()` 短路；L1/L2 Prompt 构造。
  - `backend/internal/ai/README.md`（新增）：架构、全部环境变量默认值与配置示例、降级/保护规则、解析契约、测试说明。
  - `backend/internal/ai/config_test.go`、`l1_test.go`、`parse_test.go`、`openai_test.go`、`cli_test.go`（含跨平台 `TestHelperProcess`）、`evaluator_test.go`（新增）：全场景单元测试。
  - `backend/cmd/fusion/main.go`（修改，仅 3 处最小接入）：新增 `news_panel/internal/ai` import；`handler.New(st, cfg, puller, ai.NewFromEnv())`；`WriteTimeout: ai.HTTPWriteTimeout()`（默认 120s）。其余逻辑零改动。
  - `tasks/P1-03-ai-triage-pipeline.md`、`tasks/INDEX.md`（修改）：状态回填 `IN_PROGRESS`→`IN_REVIEW`、验收勾选与本回执（授权范围内）。
- **实测验证命令与观察结果**（均在 WSL Debian 13 / Go 1.26.11 亲自运行并观察到；宿主 Windows 原无 Go，经用户批准下载官方 go1.26.11 linux-amd64 tarball 并按 go.dev 公布 SHA256 `9849f4e0…710ec` 校验通过后解压至用户目录 `~/sdk/go1.26.11`，`GOTOOLCHAIN=local`）：
  - 命令：`go build ./internal/ai/...` → 结果：`BUILD_AI_OK`（纯标准库，无需第三方模块）。
  - 命令：`go vet ./internal/ai/...` → 结果：`VET_OK`（无告警）。
  - 命令：`go test -count=1 ./internal/ai/...` → 结果：`ok  news_panel/internal/ai  0.655s`。
  - 命令：`go test -count=1 -v ./internal/ai/...` → 结果：`62` 个 `--- PASS`，`NO_FAILURES`（含 httptest mock 与 TestHelperProcess fake CLI 全部子测试）。
  - 命令：`GOPROXY=https://goproxy.cn,direct go test -count=1 ./...` → 结果：`EXIT=0`；`internal/ai`、`auth`、`config`、`handler`、`pkg/httpc`、`pull`、`pullpolicy`、`store`、`triage` 全部 `ok`；`cmd/fusion`、`model`、`web` 为 `[no test files]`（`cmd/fusion` 成功编译，证明 main.go 接入正确）。
  - 命令：`git diff --name-only` → 结果：`backend/cmd/fusion/main.go`、`tasks/INDEX.md`、`tasks/P1-03-ai-triage-pipeline.md`（新增文件位于未跟踪 `backend/internal/ai/`）。
  - 命令：`git status --porcelain backend/go.mod backend/go.sum` → 结果：空（**零变更**，未引入任何新依赖）。
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - 无边界外改动需求：`handler`/`store`/`triage` 契约（P1-01）已完整支持本评估器注入与 `failed` 落库，无需联动修改。
  - 遗留风险：L1 关键词词典为启发式，真实 X 噪声/deals 精度需在 Gate 1 云端实测中调优；L2 摘要/评分质量依赖所选模型，默认 `keep_l1` 已保证信息不丢失；CLI 适配器以 fake 进程验证，真实 `codebuddy` 等 CLI 的外层包裹兼容性建议在 Linux 测试机再确认一次。
  - 环境说明：测试在 WSL Linux（非 Windows 原生）真实运行通过；Go 工具链安装于 WSL 用户目录，可随时删除，未改动系统级配置。
  - 仓库既有未跟踪目录 `secret/` 非本任务产物，全程未读取/未修改/未提交。
  - 下一步：建议核验端在云端 Linux VPS 串联 `x-twifork -> Go Core (L1+L2) -> SQLite/前端` 端到端实测，验收去噪/评分效果与容器内存占用后推进 Gate 1。

### 协调端独立复验记录（Antigravity Coordinator · 2026-11-16）
- **核验结论**：`DONE`（代码结构、边界合规、`go vet`/`go test`/`go build` 独立验证通过）。
- **复验检查与实际命令输出**：
  1. **文件边界与安全审查**：`git status --short -uall` 仅改动 `backend/internal/ai/**`、`backend/cmd/fusion/main.go`、`tasks/P1-03-ai-triage-pipeline.md` 与 `tasks/INDEX.md`（另由协调端在根 `.gitignore` 补加 `secret/` 与 `secrets/` 防误提交）。`go.mod`/`go.sum` 零变更，纯 Go 标准库实现；审查 `evaluator.go` / `config.go` / `openai.go` / `cli.go` / `parse.go` / `l1.go` 确认实现 `triage.TriageEvaluator`、凭据脱敏、无 Key 降级 L1-only、CLI 单并发互斥与超时控制。
  2. **WSL Debian Linux 独立复测（`go version go1.26.11 linux/amd64`, `CGO_ENABLED=0`, `GOTOOLCHAIN=local`）**：
     - `go vet ./...` → 退出码 `0`（无告警）；
     - `go test -count=1 ./...` → 退出码 `0`，全 9 个含测试包全部 `ok`（含 `news_panel/internal/ai 0.674s`、`handler 3.053s`、`store 0.770s`、`auth`、`config`、`pkg/httpc`、`pull`、`pullpolicy`、`triage`）；
     - `go build -o /tmp/news_panel_verify ./cmd/fusion` → 退出码 `0`。
