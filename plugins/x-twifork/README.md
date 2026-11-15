# x-twifork —— news_panel 外置 X (Twitter) 采集插件

> 任务卡：[tasks/P1-02-x-source-plugin.md](../../tasks/P1-02-x-source-plugin.md) · 契约规范：[docs/PLUGIN_SPEC.md](../../docs/PLUGIN_SPEC.md)（v0.1）
>
> 纯 HTTP 采集（`PawiX25/twifork[impersonate]`，MIT，底层 `curl_cffi` TLS 指纹伪装），
> **无无头浏览器**，适配 1C1G–2C2G 微型 Linux VPS Docker 常驻运行。

## 1. 功能总览

- **采集**：跟踪账号时间线（`@handle` 或数字 user_id）+ 关键词实时搜索（`Latest`/`Top`），仅复用用户备用小号导出的 Cookie（`auth_token` / `ct0` / `twid`），**代码中不存在也不调用任何自动化密码 `login()`**。
- **契约投递**：严格按 PLUGIN_SPEC v0.1 构造并推送
  - `POST /api/v1/ingest`（Bearer Token；`source_type: "x"`；单批 ≤ 100 条；`(source_id, external_id)` 幂等键）
  - `POST /api/v1/heartbeat`（每轮结束后上报 `ok` / `degraded` / `error` 与 `error_message`）
- **抗 Cloudflare 与低封号风控（最高优先级）**：
  - `curl_cffi` 浏览器 TLS 指纹伪装（`impersonate: chrome124`）；
  - 低频轮询（默认 15 分钟）+ 每轮随机抖动（0~120s）+ 拟人化作息静默窗口（`quiet_hours`，深夜间隔放大）；
  - 同一轮内相邻上游请求间插入拟人间隔（`pace_seconds`）；
  - `state.json` 增量断点（按账号/查询维度持久化 `last_seen_id`，重启后只拉新推文，避免重复刷接口）；
  - 首轮种子策略（默认仅回灌最近 20 条，避免首启刷屏）；
  - 403 / 429 / Cookie 失效自动分类退避：限流走指数退避，Cookie 失效走更长退避并在心跳中如实上报。
- **可选 AI 主动查询规划**（DEC-005）：OpenAI-compatible `/chat/completions` 依据关注主题动态生成/微调关键词与账号列表；**关闭、未配置 Key、超时或返回异常格式时，无缝降级回静态 `config.yaml`，绝不阻断采集主循环**。
- **`--dry-run` 独立自测**：无需真实 Cookie 与 Go 核心服务，用内置样例推文生成合规 payload，本地打印校验或推送到指定 `--ingest-url`（配合 `tools/mock_core.py` 本地 mock 核心完成端到端模拟）。
- **隐私与脱敏**：Cookie/Token 支持环境变量注入不落盘；所有日志与心跳 `error_message` 经过脱敏（`redact.py`），**严禁泄露任何凭证**。

## 2. 目录结构

```text
plugins/x-twifork/
├── x_twifork/            # 插件 Python 包
│   ├── __main__.py       # CLI 入口（python -m x_twifork）
│   ├── app.py            # 主循环编排：planner → fetch → 增量选择 → ingest → heartbeat
│   ├── config.py         # YAML 配置加载（${ENV_VAR} 展开 / Cookie 解析 / 运行时校验）
│   ├── fetcher.py        # twifork(twikit 兼容) 适配层：Cookie/代理/TLS 指纹/异常分类
│   ├── transform.py      # TweetLike → 契约 item；payload 构造与 Schema 校验（唯一契约实现点）
│   ├── planner.py        # 可选 AI 查询规划 + 静态配置无缝降级
│   ├── poster.py         # ingest/heartbeat HTTP 投递（Bearer + 退避重试）
│   ├── scheduler.py      # 抖动 / 作息窗口 / 指数退避
│   ├── state.py          # state.json 增量断点（原子写入）
│   ├── redact.py         # 凭证脱敏（日志 & 心跳 error_message）
│   └── samples.py        # --dry-run 内置样例推文
├── tests/                # pytest 单元/集成测试（无需真实 twifork 安装）
├── tools/mock_core.py    # 本地 mock 核心（仅标准库）：模拟 /api/v1/ingest + /api/v1/heartbeat
├── config.example.yaml   # 完整注释配置样例
├── requirements.txt      # 运行时依赖（MIT/BSD/Apache-2.0）+ pytest
├── Dockerfile            # python:3.12-slim，非 root 运行
└── pytest.ini
```

## 3. 快速开始

### 3.1 本地自测（无需任何凭证）

```bash
cd plugins/x-twifork
python -m pip install -r requirements.txt        # 测试机若无编译器可用 PYYAML_FORCE_LIBYAML=0 纯装 pyyaml
python -m pytest -v                              # 全量单元/集成测试

# dry-run：内置样例推文 → 打印并自检 payload 契约合规性
python -m x_twifork --dry-run --config config.example.yaml

# dry-run + 端到端推送：先起本地 mock 核心，再把 payload 真实 POST 过去
python tools/mock_core.py --port 8099 --token mock-token --dump mock_traffic.jsonl
python -m x_twifork --dry-run --config config.example.yaml \
    --ingest-url http://127.0.0.1:8099/api/v1/ingest
```

### 3.2 真实运行

1. `cp config.example.yaml config.yaml`，填写 `core.base_url` 与凭证（推荐全部走环境变量）：

```bash
export NEWS_PANEL_INGEST_TOKEN="<核心服务下发的 INGEST_TOKEN>"
export X_AUTH_TOKEN="<备用小号 auth_token>"
export X_CT0="<备用小号 ct0>"
export X_TWID="<备用小号 twid>"        # 可选
```

2. 单轮试跑或常驻：

```bash
python -m x_twifork --config config.yaml --once   # 单轮采集后退出
python -m x_twifork --config config.yaml          # 常驻循环（默认）
```

### 3.3 Docker 部署（微型 VPS）

```bash
docker build -t news-panel-x-twifork:test .

docker run -d --name x-twifork \
    -v ./config.yaml:/config/config.yaml:ro \
    -v x-twifork-data:/data \
    -e NEWS_PANEL_INGEST_TOKEN=... -e X_AUTH_TOKEN=... -e X_CT0=... \
    news-panel-x-twifork:test
```

容器内 `/data/state.json` 持久化增量断点；插件无监听端口，健康状态通过
`POST /api/v1/heartbeat` 上报核心面板（`status: error` 即 Cookie 失效或持续被拦截）。

## 4. 备用小号 Cookie 导出与风控建议

> 严禁使用主力账号；仅使用专用**备用小号**，并接受该账号可能被限制的风险。

1. 浏览器登录备用小号 → DevTools（F12）→ Application → Cookies → `https://x.com`；
2. 复制 `auth_token`（必填）、`ct0`（强烈建议）、`twid`（可选）三个 Cookie 值；
3. 通过环境变量或挂载的 `cookies.json`（twifork 兼容格式，对象或 `[{name,value}]` 数组均可）注入；
4. **绝不将 Cookie 明文提交进仓库；本插件不会也不会尝试自动刷新登录态**。

风控建议：

- 保持默认低频（≥ 15 分钟）+ 抖动 + 静默窗口配置，不要压低 `poll_interval_seconds`（代码已强制 ≥ 5s 下限，但请自觉 ≥ 300s）；
- 若插件部署机 IP 属数据中心段，建议配置 `proxy`（HTTP/SOCKS5）走住宅/干净出口；
- 心跳出现 `error`（"session invalid"）说明 Cookie 已失效，请重新导出；
- 触发 403/429 时插件自动退避，请勿多实例共用同一账号 Cookie。

## 5. 配置速查

| 键 | 默认 | 说明 |
| :--- | :--- | :--- |
| `source_id` / `source_type` | `x-twifork-main` / `x` | 信源标识（幂等键与心跳监控维度） |
| `core.base_url` / `core.token` | — | 核心服务地址与 Bearer Token（支持 `${ENV}`） |
| `cookies.auth_token/ct0/twid` 或 `cookies.cookies_file` | — | 备用小号 Cookie（二选一） |
| `proxy` | 空 | 可选 HTTP/SOCKS5 代理，透传给 twifork |
| `impersonate` | `chrome124` | curl_cffi TLS 指纹 |
| `fetch.poll_interval_seconds` / `jitter_seconds` | 900 / 120 | 基础轮询间隔与随机抖动 |
| `fetch.quiet_hours` / `quiet_hours_multiplier` / `timezone` | `01:00-08:30` / 4.0 / `UTC` | 拟人化作息静默窗口（可跨午夜） |
| `fetch.max_tweets_per_query` / `max_items_per_batch` | 40 / 100 | 单轮拉取上限 / 单批契约上限（≤100） |
| `fetch.search_product` / `include_retweets` | `Latest` / false | 搜索模式 / 是否含转推 |
| `fetch.first_run_max_items` / `pace_seconds` | 20 / 2.0 | 首轮种子条数 / 请求间拟人间隔 |
| `backoff.*` | 60/3600/2.0/900/3 | 指数退避与 auth 长退避、error 阈值 |
| `accounts` / `queries` | — | 静态跟踪账号（`@handle` 或数字 id）与关键词 |
| `planner.enabled` / `base_url` / `api_key` / `model` / `topics` | false / — / — / `gpt-4o-mini` / — | 可选 AI 规划；失败自动回退静态列表 |

心跳 `status` 语义（PLUGIN_SPEC）：`ok`=正常；`degraded`=部分查询受限/触发退避但仍可用；`error`=Cookie 失效或持续拦截无法采集。

## 6. 开发与测试说明

- 测试**不需要**安装 `twifork`（适配层惰性导入，测试全部使用 mock/注入）；
- `tools/mock_core.py` 支持模拟鉴权失败（`--fail-mode 401|429|500`）与流量落盘（`--dump`），可用于验证插件退避与心跳脱敏；
- 契约唯一实现点在 `x_twifork/transform.py`（构造 + 镜像 Schema 的校验器），Go 核心侧联调以 [docs/PLUGIN_SPEC.md](../../docs/PLUGIN_SPEC.md) 为准。

## 7. 合规声明

- 依赖均为宽松协议：`twifork`（MIT）、`pyyaml`（MIT）、`httpx`（BSD-3-Clause）；无 GPL/AGPL 依赖；
- 无 Chromium / Playwright / Selenium 等无头浏览器组件，纯 HTTP 低内存占用；
- 无自动化密码登录 `login()` 调用，仅 Cookie 会话复用；
- 心跳与日志输出经过凭证脱敏，不回传任何 Cookie/Token。
