"""PLUGIN_SPEC v0.1 契约投递：ingest 与 heartbeat 的 HTTP 发送（含退避重试）。

- 所有请求携带 ``Authorization: Bearer <token>``；
- 429 / 5xx / 网络错误按指数退避重试；400 / 401 / 413 属契约错误，立即失败；
- 错误详情经 :mod:`x_twifork.redact` 脱敏后才进入日志与异常信息。
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any

import httpx

from .redact import redact_text

logger = logging.getLogger(__name__)

RETRYABLE_STATUS = {429, 500, 502, 503, 504}


class PosterError(Exception):
    """契约投递失败；``status`` 为 HTTP 状态码（网络错误时为 None）。"""

    def __init__(self, message: str, *, status: int | None = None, detail: str = "") -> None:
        super().__init__(message)
        self.status = status
        self.detail = detail


def derive_urls(
    base_url: str,
    ingest_url: str | None = None,
    heartbeat_url: str | None = None,
) -> tuple[str, str]:
    """由 base_url 推导契约端点；显式传入的 URL 优先（供 --dry-run 指向 mock）。

    base_url 以 ``/api/v1`` 结尾时不再重复拼接；仅给 ingest_url 时，
    heartbeat_url 默认取其同源下的 ``/api/v1/heartbeat``。
    """
    base = (base_url or "").rstrip("/")
    if base.endswith("/api/v1"):
        ingest_default = base + "/ingest"
        heartbeat_default = base + "/heartbeat"
    else:
        ingest_default = base + "/api/v1/ingest"
        heartbeat_default = base + "/api/v1/heartbeat"

    ingest = ingest_url or ingest_default
    if heartbeat_url:
        heartbeat = heartbeat_url
    elif ingest_url:
        parts = httpx.URL(ingest_url)
        heartbeat = str(parts.copy_with(path="/api/v1/heartbeat", query=None, fragment=None))
    else:
        heartbeat = heartbeat_default
    return ingest, heartbeat


class CorePoster:
    """news_panel 核心服务 HTTP 客户端（Bearer 鉴权 + 重试退避）。"""

    def __init__(
        self,
        ingest_url: str,
        heartbeat_url: str,
        token: str,
        *,
        timeout_seconds: float = 20.0,
        max_retries: int = 3,
        retry_backoff_base: float = 2.0,
        secrets: set[str] | None = None,
        client: httpx.AsyncClient | None = None,
    ) -> None:
        self._ingest_url = ingest_url
        self._heartbeat_url = heartbeat_url
        self._token = token
        self._timeout = timeout_seconds
        self._max_retries = max(1, int(max_retries))
        self._retry_base = max(0.0, float(retry_backoff_base))
        self._secrets = secrets or set()
        self._client = client
        self._owns_client = client is None

    async def _ensure_client(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = httpx.AsyncClient(timeout=self._timeout)
        return self._client

    async def aclose(self) -> None:
        if self._owns_client and self._client is not None:
            await self._client.aclose()
            self._client = None

    def _headers(self) -> dict[str, str]:
        return {"Authorization": f"Bearer {self._token}", "Content-Type": "application/json"}

    async def send_ingest(self, payload: dict[str, Any]) -> dict[str, Any]:
        """投递一批 items；返回核心服务响应（accepted/deduplicated/rejected/errors）。"""
        data = await self._post_with_retry(self._ingest_url, payload, "ingest")
        logger.info(
            "ingest accepted=%s deduplicated=%s rejected=%s",
            data.get("accepted"), data.get("deduplicated"), data.get("rejected"),
        )
        return data

    async def send_heartbeat(self, payload: dict[str, Any]) -> None:
        """心跳上报为尽力而为：失败只记日志，绝不影响采集主循环。"""
        try:
            await self._post_with_retry(self._heartbeat_url, payload, "heartbeat")
        except PosterError as exc:
            logger.warning("heartbeat delivery failed (non-fatal): %s", exc)

    async def _post_with_retry(self, url: str, payload: dict[str, Any], label: str) -> dict[str, Any]:
        client = await self._ensure_client()
        last_detail = ""
        for attempt in range(1, self._max_retries + 1):
            try:
                response = await client.post(url, json=payload, headers=self._headers())
            except httpx.HTTPError as exc:
                last_detail = f"network error: {type(exc).__name__}: {exc}"
                logger.warning("%s attempt %d/%d failed: %s", label, attempt, self._max_retries, last_detail)
            else:
                if response.status_code == 200:
                    try:
                        return response.json()
                    except ValueError as exc:
                        raise PosterError(
                            f"{label} response is not valid JSON", status=200, detail=str(exc)
                        ) from exc
                if response.status_code in RETRYABLE_STATUS:
                    last_detail = f"HTTP {response.status_code}: {redact_text(response.text[:300], self._secrets)}"
                    logger.warning(
                        "%s attempt %d/%d got retryable status %d",
                        label, attempt, self._max_retries, response.status_code,
                    )
                else:
                    raise PosterError(
                        f"{label} rejected with HTTP {response.status_code}",
                        status=response.status_code,
                        detail=redact_text(response.text[:300], self._secrets),
                    )
            if attempt < self._max_retries:
                await asyncio.sleep(min(self._retry_base * (2 ** (attempt - 1)), 8.0))
        raise PosterError(
            f"{label} unreachable after {self._max_retries} attempts", detail=last_detail
        )
