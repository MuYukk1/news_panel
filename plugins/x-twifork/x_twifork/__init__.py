"""news_panel 外置 X (Twitter) 纯 HTTP 采集插件（twifork[impersonate] 驱动）。

契约遵循 docs/PLUGIN_SPEC.md（v0.1）：
- POST /api/v1/ingest    推送标准化条目（(source_id, external_id) 幂等键）
- POST /api/v1/heartbeat 上报运行健康状态（ok / degraded / error）

硬性约束（tasks/P1-02）：
- 纯 HTTP，严禁无头浏览器（Chromium / Playwright / Selenium）；
- 严禁自动化密码登录 login()，仅使用用户备用小号导出的 Cookie；
- 低频拟人化调度 + state.json 增量断点 + 失败退避。
"""

__version__ = "0.1.0"
PLUGIN_NAME = "x-twifork"
