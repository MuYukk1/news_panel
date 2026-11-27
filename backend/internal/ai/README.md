# 内置三级 AI 分拣管线

插件投递和启用 AI 的原生 RSS 共用一个评估器、CLI 冷却锁和统计实例。各层按以下顺序执行。

1. **L1 本地规则**拦截明显噪声。若主模型关闭、选用 `l1` 或缺少主模型配置，直接返回本地结果。
2. **L2 可选决策门控**给出放行或拦截、优先级及粗领域。缺少门控配置、`AI_DECISION_ENABLED=false` 或 provider 为 `off` 时，L1 放行条目直达 L3。L2 拦截及未达 L3 优先级的条目均保留空分数。默认保留低优先级条目到「未打分」。
3. **L3 主模型**通过 OpenAI-compatible HTTP 或受控 CLI 生成评分、中文摘要、标签、最终优先级及裁定。若经过 L2 放行，提示词附带初判优先级和领域。L3 失败时沿用原有 `AI_FALLBACK_MODE` 和 `l2-fallback` 兼容标签。

RSS 单源开启 `ai_enabled` 后，按发布时间从新到旧评估本轮新入库条目。数据库按源和 GUID 去重，已存在条目不占额度；超额或关闭 AI 的条目保留为未打分。单条评估失败会记为 `failed`，其余条目继续处理。

## L2 与 RSS 配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `AI_DECISION_ENABLED` | 空 | 留空自动探测，false 强制关闭 |
| `AI_DECISION_PROVIDER` | `openai` | `openai`、`typesafe`（TypeSafe AI Jev System One）、`http_json`、`off`（或 `none`） |
| `AI_DECISION_BASE_URL` | 空 | `openai` 缺省 `https://api.openai.com/v1`，`typesafe` 缺省 `https://api.typesafe.ai/v1/systemone`，`http_json` 填完整 POST 地址 |
| `AI_DECISION_API_KEY` | 空 | 独立门控密钥，`openai`/`typesafe` 必填，`http_json` 可选 |
| `AI_DECISION_AUTH_HEADER` | `Authorization` | http_json 鉴权头名。Authorization 自动为裸密钥添加 Bearer 前缀，自定义头原样发送 |
| `AI_DECISION_MODEL` | `gpt-4o-mini` / `jev-latest` | L2 请求中的 model（`typesafe` 缺省 `jev-latest`） |
| `AI_DECISION_JSON_FORMAT` | `false` | 在 L2 OpenAI-compatible 请求中指定 json_object |
| `AI_DECISION_TIMEOUT_SECONDS` | `6` | L2 单次 HTTP 调用超时秒数 |
| `AI_DECISION_L3_PRIORITIES` | `critical,high,normal` | 放行后进入 L3 的优先级 |
| `AI_DECISION_LOW_ACTION` | `keep_unscored` | 未达 L3 门槛时保留为未打分，设 filter 则拦截 |
| `AI_DECISION_FALLBACK_MODE` | `skip_to_l3` | L2 异常时直通 L3，也可选 keep_l1 或 fail |
| `AI_RSS_MAX_ITEMS_PER_PULL` | `20` | 每个开启 AI 的 RSS 源单轮最多评估的新条目数，0 为不限 |

`openai` 使用与主模型相同的路径拼接规则。门控提示词要求单个 JSON 对象，包括 `action`、`priority`、`domain` 和可选简短原因 `reason`。

`http_json` 向配置地址 POST JSON，携带 `model`、`topics`、`source_type`、`source_id`、`author`、`title`、`content`（最多 2000 个字符）、`url`、`published_at`（UTC RFC3339）、`l1_domain` 和 `l1_priority`。支持顶层或 `data`、`decision`、`result` 包装的响应，例如 `{"decision":{"keep":true,"priority":"high","domain":"ai"}}`。缺少领域时沿用 L1，缺少优先级时放行取 normal、拦截取 low。无有效裁定、超时和响应超过 1 MiB 会触发门控失败策略。

## 运行统计

使用面板读者会话请求 `GET /api/v1/ai/stats`。返回的 `data` 包含实际启用状态和 provider 类型，以及自进程启动以来 RSS 和插件合计的计数。重启后清零。

| 计数 | 含义 |
| --- | --- |
| `l1_filtered` | 本地规则拦截 |
| `l2_skipped` | 有可用 L3 时跳过未启用的门控 |
| `l2_filtered`、`l2_passed` | 云端门控拦截或放行 |
| `l2_bypassed_l3` | L2 放行后，因优先级未达门槛而省下的 L3 调用 |
| `l2_errors` | 门控失败并进入配置的降级策略 |
| `l3_called`、`l3_kept`、`l3_filtered`、`l3_fallback` | 主模型调用、最终保留、最终拦截及失败降级 |

## 环境变量（全部从进程环境读取，含安全默认值）

| 变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `AI_ENABLED` | 自动探测 | `true`/`false`。**未显式设置时**：有 `AI_API_KEY` 或 `AI_PROVIDER=cli` 则启用，否则 L1-only。显式 `false` 强制 L1-only |
| `AI_PROVIDER` | `openai` | `openai` \| `cli` \| `l1`。未知值告警并退化 `l1` |
| `AI_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible 基址；智能拼接 `/chat/completions`（见下） |
| `AI_API_KEY` | 无 | Bearer Token；为空时 openai 通道自动降级 L1-only（不报错） |
| `AI_MODEL` | `gpt-4o-mini` | 模型名 |
| `AI_TOPICS` | `投资催化剂与宏观突发,AI前沿动态与内测泄露,高时效薅羊毛与云资源优惠` | 注入 L2 和 L3 Prompt 的自定义关注主题 |
| `AI_KEEP_MIN_SCORE` | `45` | `0..100`；`keep=false` 或 `score<阈值`（除非 priority 为 high/critical）→ `filtered` |
| `AI_TIMEOUT_SECONDS` | `8` | 单条 L3 评估超时（CLI 测试建议 `15`–`20`） |
| `AI_FALLBACK_MODE` | `keep_l1` | L3 失败策略：`keep_l1` 降级放行并加 `l2-fallback` 标签；`fail` 返回 error → 落库 `failed` |
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

## L3 输出契约与鲁棒解析

Prompt 要求模型严格输出单段 JSON：

```json
{"domain":"investing|ai|deals|other","score":0,"tags":["..."],"summary":"一句话高密度中文摘要","priority":"critical|high|normal|low","keep":true}
```

解析兼容纯 JSON、```` ```json ```` 围栏、前后混杂散文；CLI 外层包裹（`result`/`content`/`message`/`text`，含流式多行 JSON）自动解包。字段防御：score 四舍五入并夹取 `0..100`；domain/priority 大小写归一化、未知值回退安全默认；tags 去重截断（≤6 个、每个 ≤32 字符）；summary 最多保留 500 个字符并附截断标记。

## 测试

```bash
cd backend
go test -count=1 ./internal/ai/...
```

全部基于 `net/http/httptest` mock 与跨平台 Go `TestHelperProcess` fake CLI，不依赖真实外网 Key 或外部工具。
