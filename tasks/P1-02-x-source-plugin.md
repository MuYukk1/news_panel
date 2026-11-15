# `P1-02`：外置 X (Twitter) 纯 HTTP 采集插件（`plugins/x-twifork/`）

## 元信息（Metadata）
- **Task ID**：`P1-02`
- **Phase / Milestone**：`Phase 1 (v0.1 纵向切片)`
- **前置门禁（Gate Dependency）**：`Gate 0 已于 2026-11-15 正式批准`（已允许在授权目录内编写源码与 `Dockerfile`）
- **当前状态（Status）**：`IN_PROGRESS`（2026-11-15，通道 A / ZCode 认领开发中）
- **调度通道与目标执行端（Channel / Runner）**：`通道 A：ZCode`（ZCode 额度充足且适合边界干净的独立任务）
- **工作区 / 分支策略（Worktree / Branch）**：`独立目录隔离（仅修改 plugins/x-twifork/**，可独立开发与自测，不依赖 Go 核心服务先就绪）`

---

## 目标与上下文（Objective & Context）
- **核心目标**：基于 `PawiX25/twifork[impersonate]`（MIT，底层采用 `curl_cffi` TLS 指纹伪装）构建纯 HTTP、低内存占用的外置 X (Twitter) 采集插件容器，将抓取到的推文严格按 [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md) 的 `POST /api/v1/ingest` 与 `POST /api/v1/heartbeat` 契约推送给 `news_panel` 核心服务。
- **前置输入 / 依赖任务**：契约规范已冻结于 [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)，本任务与核心服务开发（`P1-01`）完全解耦并行，通过 `--dry-run` 与本地 mock HTTP 测试即可独立完成验收。

---

## 关联文档与约束（References & Constraints）
开始前必读：
1. [AGENTS.md](../AGENTS.md)（注：根目录 `AGENTS.md` 若仍保留 Gate 0 提示语，以 [docs/HANDOFF.md](../docs/HANDOFF.md) 记载的 **Gate 0 已于 2026-11-15 批准** 为准）
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)（重点核对 `DEC-003`、`DEC-005`）
4. [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md)（v0.1 `/api/v1/ingest` 与 `/api/v1/heartbeat` 唯一请求/响应格式规范）
5. [docs/PRD.md](../docs/PRD.md) & [docs/ROADMAP.md](../docs/ROADMAP.md)

本任务硬性约束：
- [x] **严禁引入无头浏览器**（禁止 Chromium / Playwright / Selenium 等高内存组件），必须保持纯 HTTP 低内存占用（适配 1C1G–2C2G 小型 Linux VPS）
- [x] **严格遵守宽松开源协议**：核心依赖 `twifork[impersonate]`（MIT）等仅限 MIT / Apache-2.0 / BSD，严禁引入 GPL / AGPL 依赖
- [x] **最高优先级：抗 Cloudflare 拦截与低封号风控**（仅使用用户备用 X 账号导出 Cookie：`auth_token` / `ct0` / `twid`，严禁在代码中调用自动化账号密码 `login()`）
- [x] **严格限定 `Owned Files`**：所有源码、测试、配置样例与 `Dockerfile` 必须全部落于 `plugins/x-twifork/` 内

---

## 功能与工程细则要求

1. **采集与反风控引擎（`twifork[impersonate]`）**：
   - 使用 `twifork[impersonate]`（`curl_cffi`）读取用户提供的 Cookie（支持 `auth_token`、`ct0`、`twid` 配置或 `cookies.json` 挂载）；
   - 支持可选代理配置（HTTP / SOCKS5 proxy URL 透传给采集客户端）；
   - **低频拟人化调度**：支持基础轮询间隔 + 随机抖动（jitter）+ 拟人化作息静默窗口（active hours / quiet hours，降低深夜高频机械请求特征）；
   - **增量断点持久化（`state.json`）**：按跟踪账号/查询维度持久化 `last_seen_id` 等游标状态，容器重启后仅拉取新推文，避免重复刷接口；
   - **退避保护**：遇到 `403` / `429` / Cookie 失效异常时立即退避降级，并通过 `/api/v1/heartbeat` 上报 `status: "degraded"` 或 `"error"`（`error_message` 严禁泄露任何 Cookie/Token 凭证）。
2. **可选 AI 主动查询规划接口（带静态配置降级）**：
   - 提供轻量可选的 OpenAI-compatible 规划接口/模块，可根据配置的关注主题动态生成或微调待搜索关键词 / 重点账号列表；
   - 当 AI 规划关闭、未配置 API Key、网络超时或返回异常格式时，**必须无缝降级回静态 `config.yaml`** 中的账号与关键词列表，不得阻断采集主循环。
3. **契约投递与 `--dry-run` 独立自测模式**：
   - 严格按照 [docs/PLUGIN_SPEC.md](../docs/PLUGIN_SPEC.md) 构造：
     - `POST /api/v1/ingest`（Header 带 `Authorization: Bearer <token>`，`source_type: "x"`，每条包含 `external_id`（推文 ID）、`url`、`title`、`content`、`author`、`published_at`（RFC 3339 UTC）、`raw_meta`）；
     - `POST /api/v1/heartbeat`（包含 `source_id`、`plugin_version`、`status`、`fetched_count`、`error_message`、`timestamp`）。
   - 支持 `--dry-run` 启动模式：无需真实 X Cookie 与真实 Go 核心服务，加载内置样例推文数据生成合规 payload，支持本地打印校验及向指定 `--ingest-url`（如本地临时 mock HTTP 服务）完成端到端模拟推送。
4. **交付物清单（全部位于 `plugins/x-twifork/` 下）**：
   - 插件 Python 模块代码与单元测试（`tests/`）
   - `requirements.txt`（精简依赖，含 `twifork[impersonate]`、`pyyaml`、`httpx`/`requests`、`pytest` 等，无重型依赖）
   - `Dockerfile`（基于轻量 Python 镜像如 `python:3.12-slim`，兼容 `curl_cffi` 运行时）
   - `config.example.yaml`（完整注释：核心 ingest URL/Token、X Cookie 占位、代理、抖动/作息时段、静态账号/关键词、可选 AI planner 配置）
   - `README.md`（使用说明、备用小号 Cookie 导出与风控建议、`--dry-run` 及 Docker 运行命令）

---

## 文件边界（Owned / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `plugins/x-twifork/**`
- *(注：开始与结束时允许仅更新 `tasks/P1-02-x-source-plugin.md` 自身状态/回执区及 `tasks/INDEX.md` 中 `P1-02` 对应行)*

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/PLUGIN_SPEC.md`、`docs/DECISIONS.md`、`docs/PRD.md`、`docs/ROADMAP.md`、`docs/HANDOFF.md`

### Forbidden / Out-of-Scope（严禁触碰）
- 核心仓库代码与前端代码（如 `backend/**`、`frontend/**`、`src/**`、根目录 `Dockerfile` / `docker-compose.yml`）
- `docs/**`、`AGENTS.md`、`tasks/README.md`、`tasks/_TEMPLATE.md`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）
- [ ] `plugins/x-twifork/` 包含清晰代码结构、`Dockerfile`、`requirements.txt`、`config.example.yaml`、`README.md` 与 `tests/`
- [ ] 校验无 Chromium / Playwright / Selenium 等无头浏览器依赖，无 GPL/AGPL 依赖，无任何硬编码密码登录 `login()` 调用
- [ ] `--dry-run` 模式不依赖真实 X Cookie 即可跑通并输出/推送完全符合 `docs/PLUGIN_SPEC.md` Schema 的 `/api/v1/ingest` 与 `/api/v1/heartbeat` 请求
- [ ] `pytest` 覆盖并全部通过：契约字段校验、`state.json` `last_seen_id` 增量断点持久化、抖动与作息调度逻辑、AI 查询规划失败降级回静态 `config.yaml`、心跳异常上报脱敏
- [ ] 所有代码与文档改动严格位于 `Owned Files` 范围内

建议核验命令：
```text
cd plugins/x-twifork
python -m pytest -v
python -m <entrypoint> --dry-run --config config.example.yaml
```
*(若本地环境允许 Docker，可追加 `docker build -t news-panel-x-twifork:test .` 验证镜像构建；若宿主机无 Docker daemon，须在 Handoff Report 如实标注仅完成 Python/pytest/dry-run 实测。)*

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`IN_REVIEW` | `BLOCKED`
- **执行端 / 通道**：`通道 A / ZCode`
- **实际改动文件清单**：
  - `<待填写>`
- **实测验证命令与观察结果**（仅填写最终修改后亲自跑过并观察到的真实结果）：
  - 命令：`...` → 结果：`...`
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - `<待填写>`
