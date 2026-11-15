# news_panel 外置 X 采集插件（x-twifork）
# 纯 HTTP 采集：twifork[impersonate]（curl_cffi TLS 指纹伪装），无无头浏览器依赖，
# 适配 1C1G–2C2G 微型 Linux VPS（DEC-005 / ADR-004）。
FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

# 先装依赖再拷代码，最大化镜像层缓存命中
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY x_twifork/ ./x_twifork/

# 运行期挂载点：
#   /config/config.yaml        配置文件（必须）
#   /data/cookies.json         可选 Cookie 文件（cookies_file 指向时）
#   /data/state.json           增量断点（由 --state-file 指定）
RUN mkdir -p /config /data \
    && chown -R 65532:65532 /config /data /app

USER 65532:65532

CMD ["python", "-m", "x_twifork", "--config", "/config/config.yaml", "--state-file", "/data/state.json"]
