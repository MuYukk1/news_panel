# `news_panel` 双层插件最小契约规范（v0.1）

> **状态**：**APPROVED (Gate 0, 2026-11-15)** —— v0.1 最小纵向切片开发基线  
> **定位**：定义核心 Go 服务（基于 `0x2E/fusion` 演进）与外置插件之间的最小 HTTP/JSON 契约，确保 Go 核心与任意语言外置插件（如 `plugins/x-twifork/`）可**完全并行开发与独立自测**。  
> **演进边界**：v0.1 先保通此最小契约；**Gate 1** 验收端到端切片后，再冻结正式的 v1 完整规范（如外置 Processor Webhook 钩子、事件回调等）。

---

## 1. 通用约定与鉴权

- **传输协议**：HTTP/1.1（或 HTTPS）+ `application/json; charset=utf-8`
- **接口前缀**：`/api/v1`
- **鉴权方式**：所有插件入站请求均须携带 Bearer Token：
  ```http
  Authorization: Bearer <INGEST_TOKEN>
  ```
  - Token 缺失、格式错误或校验不通过时，核心服务返回 `401 Unauthorized`。
- **时间格式**：统一使用 **RFC 3339 / ISO 8601 UTC** 时间字符串（例如 `2026-11-15T11:20:00Z`）。
- **大小与幂等护栏**：单次 `/api/v1/ingest` 建议最多携带 `100` 条 `items`；以 `(source_id, external_id)` 作为核心服务**幂等去重联合唯一键**，插件重复投递同一条目时核心必须安全去重，不得报错或产生重复记录。

---

## 2. `POST /api/v1/ingest`（外置信源投递接口）

外置 Source 插件采集并清洗基础字段后，通过该接口向核心服务推送标准化条目；核心服务完成幂等去重后入库，并触发内置 L1/L2 AI 处理管线。

### 2.1 请求体（Request JSON Schema）

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["source_id", "source_type", "items"],
  "additionalProperties": false,
  "properties": {
    "source_id": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "description": "信源插件实例唯一标识，例如 x-twifork-main"
    },
    "source_type": {
      "type": "string",
      "minLength": 1,
      "maxLength": 64,
      "description": "信源类型标识，例如 x / telegram / forum / qq"
    },
    "items": {
      "type": "array",
      "minItems": 1,
      "maxItems": 100,
      "items": {
        "type": "object",
        "required": ["external_id", "url", "content", "published_at"],
        "additionalProperties": false,
        "properties": {
          "external_id": {
            "type": "string",
            "minLength": 1,
            "maxLength": 256,
            "description": "信源内唯一幂等键（如推文 ID），与 source_id 联合保证幂等"
          },
          "url": {
            "type": "string",
            "format": "uri",
            "minLength": 1,
            "description": "原始条目规范链接"
          },
          "title": {
            "type": "string",
            "description": "可选标题；若短内容平台（如 X）无原生标题，可传空字符串或由插件截取首行摘要"
          },
          "content": {
            "type": "string",
            "minLength": 1,
            "description": "正文内容（纯文本或轻 Markdown）"
          },
          "author": {
            "type": "string",
            "description": "可选作者标识或展示名（如 @OpenAI）"
          },
          "published_at": {
            "type": "string",
            "format": "date-time",
            "description": "RFC 3339 UTC 发布时间"
          },
          "raw_meta": {
            "type": "object",
            "additionalProperties": true,
            "description": "可选原始补充元数据（如转赞评计数、媒体链接、命中关键词/列表名）"
          }
        }
      }
    }
  }
}
```

### 2.2 响应体与状态码（Response）

- **`200 OK`**（整批请求语法合法且已完成幂等处理）：
  ```json
  {
    "accepted": 3,
    "deduplicated": 2,
    "rejected": 0,
    "errors": []
  }
  ```
  - `accepted` (`integer`)：本次新入库条目数。
  - `deduplicated` (`integer`)：因 `(source_id, external_id)` 已存在而安全跳过的重复条目数。
  - `rejected` (`integer`)：批内单条字段校验失败被拒绝的数量（存在单条失败时，`errors` 返回 `[{"external_id": "...", "reason": "..."}]`）。
- **错误状态码**：
  - `400 Bad Request`：JSON 非法、缺少顶层必填字段或 `items` 为空/超限。
  - `401 Unauthorized`：Bearer Token 缺失或无效。
  - `413 Payload Too Large`：单次请求体超过服务上限。
  - `429 Too Many Requests` / `5xx`：限流或服务异常，插件端应退避重试。

---

## 3. `POST /api/v1/heartbeat`（外置插件心跳上报接口）

外置插件在每轮拉取周期结束后（或异常降级时）上报运行健康状态，便于核心面板监控静默断流、Cookie 失效或 Cloudflare 拦截。

### 3.1 请求体（Request JSON Schema）

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["source_id", "plugin_version", "status", "fetched_count", "timestamp"],
  "additionalProperties": false,
  "properties": {
    "source_id": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128
    },
    "plugin_version": {
      "type": "string",
      "minLength": 1,
      "maxLength": 64
    },
    "status": {
      "type": "string",
      "enum": ["ok", "degraded", "error"],
      "description": "ok=正常；degraded=部分查询受限/触发退避但仍可用；error=Cookie失效或持续拦截无法采集"
    },
    "fetched_count": {
      "type": "integer",
      "minimum": 0,
      "description": "本轮采集到的条目数量"
    },
    "error_message": {
      "type": ["string", "null"],
      "description": "非 ok 状态时的可读诊断信息（严禁包含 Cookie/Token 等敏感凭证）"
    },
    "timestamp": {
      "type": "string",
      "format": "date-time",
      "description": "RFC 3339 UTC 心跳时间戳"
    }
  }
}
```

### 3.2 响应体（Response）

- **`200 OK`**：
  ```json
  {
    "status": "ok",
    "server_time": "2026-11-15T11:25:00Z"
  }
  ```
- 错误状态码：`400 Bad Request` / `401 Unauthorized` / `5xx`。

---

## 4. 核心 `items` 表扩展 AI / 分拣字段规范

在 `0x2E/fusion` 原有 `items` 持久化模型（`guid` / `url` / `title` / `content` / `pub_date` / `unread` / `bookmark` 等）基础上，为支持高信噪比情报分诊，核心库与面板读取模型新增以下扩展字段：

| 字段名 | 类型 / 约束 | 默认值 / 可选枚举 | 说明 |
| :--- | :--- | :--- | :--- |
| `domain` | `string` | `"other"`（如 `"investing"` \| `"ai"` \| `"deals"` \| `"other"`） | 所属情报域分类 |
| `ai_score` | `integer \| null` | `null`（处理后为 `0–100`） | AI 相关性与价值评分 |
| `ai_tags` | `string[]`（SQLite 存 JSON 数组） | `[]` | 结构化主题/实体标签 |
| `ai_summary` | `string` | `""` | L2 LLM 提炼的一句话高密度摘要 |
| `priority` | `string` | `"normal"`（枚举：`"critical"` \| `"high"` \| `"normal"` \| `"low"`） | 优先级路由等级 |
| `triage_status` | `string` | `"pending"`（枚举：`"pending"` \| `"kept"` \| `"filtered"` \| `"failed"`） | 管线分拣状态（面板默认展示 `kept`，`filtered` 保留在库供审计不扰民） |

---

## 5. 内置插件 vs 外置 HTTP 插件的等价内部模型

`news_panel` 采用**内置 + 外置混合双层插件架构**，两者在核心服务内部收敛到同一套数据信封与处理流水线：

```text
[内置 Go Source 拉取器] ──┐
                           ├──▶ 统一内部信封 (Normalized IngestItem, 键: source_id + external_id)
[外置 HTTP Source 插件] ──┘         │
     (POST /api/v1/ingest)          ▼
                         [SQLite 幂等落库 (triage_status = pending)]
                                    │
                                    ▼
                         [双层 Processor Pipeline]
                           ├─ L1: 内置轻量规则 / 关键词 / 频控粗筛
                           ├─ L2: 内置云端 OpenAI-compatible LLM 评分/打标/摘要
                           └─ (Gate 1 后扩展) 外置 Processor Webhook 钩子
                                    │
                                    ▼
                         [更新 items AI/分拣字段 -> React 19 面板呈现]
```

- **设计要点**：
  1. **内置插件**（如 v0.1 默认的 L1 规则过滤、L2 OpenAI-compatible 评分器、以及后续通用 RSS 拉取器）直接实现核心 Go 接口，进程内零网络开销调用，对 1C1G VPS 内存最友好；
  2. **外置插件**（如 v0.1 的 `plugins/x-twifork/`）运行在独立容器或远端微型 VPS 上，可使用 Python/Node/Go/Shell 任意语言编写，通过本规范的 `POST /api/v1/ingest` 与 `POST /api/v1/heartbeat` 投递与核心内置源完全等价的内部条目；
  3. **演进预留**：v0.1 仅实现并冻结上述最小 HTTP Ingest + Heartbeat 契约；待 Gate 1 验收通过后，再按实际需求冻结外置 Processor Webhook 契约及完整的 v1 插件规范。
