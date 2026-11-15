# `P1-02`：外置 X (Twitter) 纯 HTTP 采集插件（`plugins/x-twifork/`）

## 元信息（Metadata）
- **Task ID**：`P1-02`
- **Phase / Milestone**：`Phase 1 (v0.1 纵向切片)`
- **前置门禁（Gate Dependency）**：`Gate 0 已于 2026-11-15 正式批准`（已允许在授权目录内编写源码与 `Dockerfile`）
- **当前状态（Status）**：`DONE`（2026-11-15，通道 A / ZCode 开发完成，协调端已在 WSL Debian Linux 独立复验通过；真实 X Cookie 上游拉取留待 `P1-01` 合入后联合真机联调）
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
- [x] `plugins/x-twifork/` 包含清晰代码结构、`Dockerfile`、`requirements.txt`、`config.example.yaml`、`README.md` 与 `tests/`
- [x] 校验无 Chromium / Playwright / Selenium 等无头浏览器依赖，无 GPL/AGPL 依赖，无任何硬编码密码登录 `login()` 调用
- [x] `--dry-run` 模式不依赖真实 X Cookie 即可跑通并输出/推送完全符合 `docs/PLUGIN_SPEC.md` Schema 的 `/api/v1/ingest` 与 `/api/v1/heartbeat` 请求
- [x] `pytest` 覆盖并全部通过：契约字段校验、`state.json` `last_seen_id` 增量断点持久化、抖动与作息调度逻辑、AI 查询规划失败降级回静态 `config.yaml`、心跳异常上报脱敏
- [x] 所有代码与文档改动严格位于 `Owned Files` 范围内

建议核验命令：
```text
cd plugins/x-twifork
python -m pytest -v
python -m <entrypoint> --dry-run --config config.example.yaml
```
*(若本地环境允许 Docker，可追加 `docker build -t news-panel-x-twifork:test .` 验证镜像构建；若宿主机无 Docker daemon，须在 Handoff Report 如实标注仅完成 Python/pytest/dry-run 实测。)*

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`IN_REVIEW`
- **执行端 / 通道**：`通道 A / ZCode`
- **实际改动文件清单**：
  - **新增（全部位于 Owned Files `plugins/x-twifork/**`）**：
    - 插件包 `x_twifork/`：`__init__.py`（版本与红线声明）、`__main__.py`（CLI 入口）、`app.py`（主循环编排：planner→fetch→增量选择→ingest→heartbeat）、`config.py`（YAML + `${ENV}` 展开 + Cookie 解析 + 运行时校验）、`fetcher.py`（twifork/twikit 兼容适配层：Cookie/代理/TLS 指纹/异常分类，惰性导入）、`transform.py`（TweetLike→契约 item 与 payload 构造/校验，唯一契约实现点）、`planner.py`（可选 AI 查询规划 + 静态无缝降级）、`poster.py`（ingest/heartbeat Bearer 投递 + 退避重试）、`scheduler.py`（抖动/作息窗口/指数退避）、`state.py`（state.json 原子持久化游标）、`redact.py`（凭证脱敏）、`samples.py`（dry-run 样例）
    - 测试 `tests/`：`conftest.py`、`test_contract.py`、`test_state.py`、`test_scheduler.py`、`test_fetcher.py`、`test_planner.py`、`test_poster.py`、`test_heartbeat.py`、`test_config.py`、`test_dryrun.py`（共 94 个用例）
    - 工具 `tools/mock_core.py`（本地 mock 核心，仅标准库，模拟 `/api/v1/ingest`+`/api/v1/heartbeat`、Token 鉴权、`--fail-mode 401/429/500`、`--dump` 流量落盘）
    - 交付物：`config.example.yaml`（全注释）、`requirements.txt`、`Dockerfile`（python:3.12-slim + 非 root）、`.dockerignore`、`pytest.ini`、`README.md`（含 Cookie 导出与风控建议）
  - **状态回填（任务卡注明的授权范围）**：`tasks/P1-02-x-source-plugin.md`（本卡状态区 + 本回执）、`tasks/INDEX.md`（`P1-02` 行状态）
- **实测验证命令与观察结果**（仅填写最终修改后亲自跑过并观察到的真实结果）：
  - 命令：`cd plugins/x-twifork && python -m pytest -v`（本地临时 venv：Python 3.15.1 + pytest 9.2.0 / pyyaml 6.0.3 / httpx 0.28.1；未安装 twifork，适配层测试全部走 mock/注入）→ 结果：**`94 passed in 1.31s`**，覆盖验收清单全部五项：契约字段校验、`state.json` `last_seen_id` 增量断点（含损坏恢复/原子写/TTL 清理）、抖动与作息调度（跨午夜窗口/倍率/上限/退避）、AI 规划失败降级静态配置（禁用/无 Key/500/畸形 JSON/空计划均回退）、心跳异常上报脱敏（Cookie 值与 `Bearer xxx` 模式均被替换为 `***REDACTED***`）
  - 命令：`python -m x_twifork --dry-run --config config.example.yaml` → 结果：退出码 `0`；打印 7 条样例 ingest payload + 1 条 heartbeat，末行 `--- VALIDATION OK: payloads conform to PLUGIN_SPEC v0.1 ---`；`state.json` 未被创建（dry-run 幂等不落盘）
  - 命令：`python tools/mock_core.py --port 8099 --token mock-token --dump mock_traffic.jsonl` + 两次 `python -m x_twifork --dry-run --config config.example.yaml --ingest-url http://127.0.0.1:8099/api/v1/ingest` → 结果：两次退出码均 `0`；第 1 轮日志 `ingest accepted=7 deduplicated=0 rejected=0`，第 2 轮 `ingest accepted=0 deduplicated=7 rejected=0`（核心侧幂等去重生效）；mock 流量记录确认 ingest/heartbeat 共 4 次请求均携带 `Bearer mock-token`
  - 命令：`python -m x_twifork --config config.example.yaml`（live 模式、未配置 Cookie/Token）→ 结果：按设计拒绝启动，`ERROR x-twifork: live mode config invalid: auth_token cookie missing (mandatory for X session reuse)...`，退出码 `2`
  - 命令：`grep -rniE "chromium|playwright|selenium|headless" x_twifork/ tests/ tools/ requirements.txt Dockerfile` 与 `grep -rnE "\.login\(|login\(" x_twifork/` → 结果：仅命中 docstring 中"严禁无头浏览器/绝不调用 login()"的红线声明本身，无任何实际引入或调用；`grep -inE "gpl|agpl" requirements.txt` 仅命中"无 GPL/AGPL"注释，依赖为 twifork(MIT)/pyyaml(MIT)/httpx(BSD-3)/pytest(MIT)
  - 命令：`docker build -t news-panel-x-twifork:test .`（Docker Desktop 引擎 29.7.2，经用户同意临时启动）→ 结果：退出码 `0`；`python:3.12-slim` 内成功安装全依赖（twifork 2.4.0、curl_cffi 0.7.4、lxml 6.1.3 等，`pyjsparser` 由 sdist 纯 Python 构建，无需编译器）
  - 容器冒烟：`docker run --rm news-panel-x-twifork:test python -c "import twikit, curl_cffi, yaml, httpx"` → 结果：全部可导入（twifork 以 `twikit` 模块名提供）；`docker run --rm news-panel-x-twifork:test python -m x_twifork --dry-run --state-file /data/state.json` → 结果：`VALIDATION OK`；非 root(uid 65532) 对 `/data/state.json` 写入成功；镜像 272MB
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - **无边界变更申请**：全部改动严格位于 `plugins/x-twifork/**` + 本卡状态区/回执 + `INDEX.md` 的 `P1-02` 行；未触碰 `docs/**`、核心代码或任何 Forbidden Files。
  - **遗留风险 1（需 P1-01 联调确认）**：`fetcher.py` 按 twifork v2.3.x/2.4 公开 API（GitHub README 与 `client.py` 源码核对：`Client(language, proxy=, impersonate=)`、`set_cookies`、`get_user_tweets`、`search_tweet`、`get_user_by_screen_name`、`is_logged_in`）实现，但本环境无真实 X Cookie，**真实上游拉取未实测**；首次真机联调建议以 `--once` 单轮运行并观察退避与心跳表现。
  - **遗留风险 2**：可选 AI planner 仅完成降级/解析路径实测（mock transport），未接真实 OpenAI-compatible 服务验证 prompt 效果。
  - **实测环境说明**：宿主机仅 Python 3.15 且无 MSVC，本地 venv 中 pyyaml 以 `PYYAML_FORCE_LIBYAML=0` 纯 Python 模式安装（仅影响本机测试环境，不影响交付物）；Dockerfile 目标 3.12 的完整依赖已通过真实 `docker build` 验证。跨平台 `pip download --only-binary` 预检曾报 `pyjsparser` 无 wheel（PyPI 仅 sdist），经真实容器构建证实为误报。
  - **下一步建议**：① 核验端按"验收标准"逐项复核（重点：契约 Schema 镜像一致性、Cookie/Token 不落盘）；② `P1-01` 核心就绪后做端到端真机联调（含 Cloudflare/风控表现与内存占用观测，对接 Gate 1 验收）；③ 后续可评估镜像瘦身（twifork 自带 lxml/js2py 等间接依赖占体积，运行内存影响很小）。另：本机 Docker Desktop 为本次实测临时启动，测试镜像 `news-panel-x-twifork:test` 保留在本地 docker 中供核验，可随时 `docker rmi` 清理并关闭 Docker Desktop。

### 协调端独立复验记录（Antigravity Coordinator · 2026-11-15）
- **核验结论**：`DONE`（按任务卡独立离线/契约验收标准通过；真机 X Cookie 拉取与 AI 规划实测延后至 `P1-01` + `P1-03` 联调阶段）。
- **复验检查与实际命令输出**：
  1. **文件边界复核**：`git status --short -uall` 仅包含 `plugins/x-twifork/**`、`tasks/P1-02-x-source-plugin.md`、`tasks/INDEX.md`，无越界改动。
  2. **合规与代码审查**：核查 `requirements.txt`（`twifork[impersonate]` MIT / `pyyaml` MIT / `httpx` BSD-3-Clause / `pytest` MIT）、`Dockerfile`（`python:3.12-slim` 非 root 运行）、`x_twifork/{fetcher.py,transform.py,poster.py,planner.py,redact.py,scheduler.py,state.py,config.py}` 与 `rg` 全量扫描确认无 `login()` 调用、无无头浏览器组件、无 GPL/AGPL 依赖，契约字段与 `docs/PLUGIN_SPEC.md` 一致。
  3. **WSL Debian Linux 实测（Python 3.13.5 + 临时 `/tmp/xtwifork-venv` 安装 `requirements.txt` 全量依赖）**：
     - `pytest -q`：**`94 passed in 2.19s`**
     - `python -m x_twifork --dry-run --once`：退出码 `0`，输出 7 条 `ingest` items 与 1 条 `heartbeat`，末行 `--- VALIDATION OK: payloads conform to PLUGIN_SPEC v0.1 ---`
     - `docker images news-panel-x-twifork`：本地存在 `news-panel-x-twifork:test`（`98cf274322ed`，272MB）。
