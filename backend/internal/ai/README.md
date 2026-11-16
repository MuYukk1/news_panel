# `internal/ai` — v0.1 内置 L1+L2 AI 分拣管线

实现 [`triage.TriageEvaluator`](../triage/triage.go) 契约的两段式信息分拣器（`DEC-004`）：

```text
triage.Input ──▶ L1 本地规则粗筛（<1ms，零网络）
                   ├─ 命中噪声 ──▶ filtered（priority=low，不消耗 L2 Token）
                   └─ 放行 ──▶ L2 CompletionProvider（openai | cli）
                                 ├─ 成功 ──▶ 评分/打标/摘要/优先级 + keep 阈值裁定
                                 └─ 失败 ──▶ AI_FALLBACK_MODE（keep_l1 降级放行 | fail 落库 failed）
```

- **纯 Go 标准库**，零新增第三方依赖（不修改 `go.mod` / `go.sum`）。
- **资源红线**：复用单个 `*http.Client` 连接池；CLI 适配器以 `sync.Mutex` 强制单并发串行 + 超时保护，适配 1C1G–2C2G 微型 VPS。
- **安全红线**：不读取/修改 `.env`；`AI_API_KEY` 等凭证经 `redact()` 脱敏后才可进入日志或错误；所有返回值均通过 `triage.Result.Validate()`。
- **不伪造分数**：未走 L2 模型评分的兜底放行条目 `AIScore = nil`。

## 装配入口

```go
// backend/cmd/fusion/main.go
h, err := handler.New(st, cfg, puller, ai.NewFromEnv())
srv := &http.Server{ /* ... */ WriteTimeout: ai.HTTPWriteTimeout() }
```

`NewFromEnv()` 永不返回 nil、永不阻塞启动：未启用/无 Key/配置缺失时自动退化为 L1-only。

## 环境变量（全部从进程环境读取，含安全默认值）

| 变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `AI_ENABLED` | 自动探测 | `true`/`false`。**未显式设置时**：有 `AI_API_KEY` 或 `AI_PROVIDER=cli` 则启用，否则 L1-only。显式 `false` 强制 L1-only |
| `AI_PROVIDER` | `openai` | `openai` \| `cli` \| `l1`。未知值告警并退化 `l1` |
| `AI_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible 基址；智能拼接 `/chat/completions`（见下） |
| `AI_API_KEY` | 无 | Bearer Token；为空时 openai 通道自动降级 L1-only（不报错） |
| `AI_MODEL` | `gpt-4o-mini` | 模型名 |
| `AI_TOPICS` | `投资催化剂与宏观突发,AI前沿动态与内测泄露,高时效薅羊毛与云资源优惠` | 注入 L2 Prompt 的自定义关注主题 |
| `AI_KEEP_MIN_SCORE` | `45` | `0..100`；`keep=false` 或 `score<阈值`（除非 priority 为 high/critical）→ `filtered` |
| `AI_TIMEOUT_SECONDS` | `8` | 单条 L2 评估超时（CLI 测试建议 `15`–`20`） |
| `AI_FALLBACK_MODE` | `keep_l1` | L2 失败策略：`keep_l1` 降级放行并加 `l2-fallback` 标签；`fail` 返回 error → 落库 `failed` |
| `AI_OPENAI_JSON_FORMAT` | `false` | `true` 时请求附 `response_format:{"type":"json_object"}`；不支持该字段的中转/开源模型保持 `false` |
| `AI_L1_MIN_CONTENT_RUNES` | `10` | 去 URL/空白/@mention/`(media-only tweet)` 后正文有效字符下限；`0` 关闭短文本过滤 |
| `AI_L1_FILTER_RETWEETS` | `false` | `true` 时按 `raw_meta.is_retweet` 过滤转推 |
| `AI_L1_BLOCK_KEYWORDS` | 空 | 逗号分隔自定义黑名单（命中即 filtered，优先级高于 deals 保护） |
| `AI_L1_ALLOW_KEYWORDS` | 空 | 逗号分隔白名单（强制不被 L1 拦截，直通 L2/放行） |
| `AI_CLI_COMMAND` | 空 | CLI 可执行名/路径（如 `codebuddy`）；为空时 cli 通道降级 L1-only |
| `AI_CLI_ARGS` | 空 | 逗号分隔参数（无逗号时按空白拆分），支持 `{{prompt}}` 占位符；未含占位符则自动附加到末尾 |
| `AI_HTTP_WRITE_TIMEOUT_SECONDS` | `120` | 由 `cmd/fusion` 读取以放宽 `srv.WriteTimeout`，防止多条同步评估被 30s 默认截断 |

### `AI_BASE_URL` 拼接规则

- 已以 `/chat/completions` 结尾 → 原样使用；
- 以 `/v1` 结尾 → 补 `/chat/completions`；
- 其他 → 补 `/v1/chat/completions`。

### L1 噪声过滤与 deals 保护

默认黑名单拦截互粉/互关/抽奖引流/纯广告/网赚刷单等水帖；但**命中 deals 信号词（薅羊毛、限免、VPS 优惠、折扣码、空投等）时跳过默认黑名单**，严禁误杀真实线报。优先级：`AI_L1_ALLOW_KEYWORDS` 白名单 > 自定义黑名单 > 转推过滤 > deals 保护下的默认黑名单 > 短文本过滤。短文本过滤仅在「无标题且无非社交高价值外链」时生效。

ASCII 关键词按词边界匹配（`ai` 不会误命中 `email`），中文关键词按子串匹配。

## 配置示例（仅示意，请通过容器环境/编排注入，勿写入仓库 `.env`）

```bash
# 云端 OpenAI-compatible（默认主通道）
AI_ENABLED=true
AI_PROVIDER=openai
AI_BASE_URL=https://api.openai.com/v1
AI_API_KEY=sk-xxxx
AI_MODEL=gpt-4o-mini
AI_KEEP_MIN_SCORE=45
AI_TIMEOUT_SECONDS=8
AI_FALLBACK_MODE=keep_l1
AI_OPENAI_JSON_FORMAT=false

# 或：Linux 测试机受控 Headless CLI（单并发串行）
AI_PROVIDER=cli
AI_CLI_COMMAND=codebuddy
AI_CLI_ARGS=-p,{{prompt}},--output-format,json
AI_TIMEOUT_SECONDS=20

# 或：完全离线，仅 L1 规则
AI_PROVIDER=l1
```

## L2 输出契约与鲁棒解析

Prompt 要求模型严格输出单段 JSON：

```json
{"domain":"investing|ai|deals|other","score":0,"tags":["..."],"summary":"一句话高密度中文摘要","priority":"critical|high|normal|low","keep":true}
```

解析兼容纯 JSON、```` ```json ```` 围栏、前后混杂散文；CLI 外层包裹（`result`/`content`/`message`/`text`，含流式多行 JSON）自动解包。字段防御：score 四舍五入并夹取 `0..100`；domain/priority 大小写归一化、未知值回退安全默认；tags 去重截断（≤6 个、每个 ≤32 字符）；summary 截断 200 字符。

## 测试

```bash
cd backend
go test -count=1 ./internal/ai/...
```

全部基于 `net/http/httptest` mock 与跨平台 Go `TestHelperProcess` fake CLI，不依赖真实外网 Key 或外部工具。