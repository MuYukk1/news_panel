"""契约投递 HTTP 层：Bearer 鉴权、重试退避、错误脱敏与 URL 推导。"""

import asyncio
import json

import httpx
import pytest

from x_twifork.poster import CorePoster, PosterError, derive_urls
from x_twifork.transform import build_heartbeat_payload

SECRET = "super-secret-cookie-value-123"


def _poster(handler, **kwargs):
    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    defaults = dict(
        ingest_url="https://core.invalid/api/v1/ingest",
        heartbeat_url="https://core.invalid/api/v1/heartbeat",
        token="tok-test-123456",
        max_retries=3,
        retry_backoff_base=0.0,
        secrets={SECRET},
        client=client,
    )
    defaults.update(kwargs)
    return CorePoster(**defaults)


def test_ingest_sends_bearer_and_parses_summary():
    seen = {}

    def handler(request):
        seen["auth"] = request.headers.get("Authorization")
        seen["body"] = json.loads(request.content.decode("utf-8"))
        return httpx.Response(200, json={"accepted": 1, "deduplicated": 0, "rejected": 0, "errors": []})

    poster = _poster(handler)
    payload = {
        "source_id": "s",
        "source_type": "x",
        "items": [
            {
                "external_id": "1",
                "url": "https://x.com/a/status/1",
                "content": "c",
                "published_at": "2026-11-15T10:00:00Z",
            }
        ],
    }
    data = asyncio.run(poster.send_ingest(payload))
    assert seen["auth"] == "Bearer tok-test-123456"
    assert seen["body"]["source_id"] == "s"
    assert data["accepted"] == 1


def test_ingest_401_no_retry():
    calls = {"n": 0}

    def handler(request):
        calls["n"] += 1
        return httpx.Response(401, json={"error": "unauthorized"})

    poster = _poster(handler)
    with pytest.raises(PosterError) as excinfo:
        asyncio.run(poster.send_ingest({"source_id": "s", "source_type": "x", "items": [{"external_id": "1"}]}))
    assert excinfo.value.status == 401
    assert calls["n"] == 1  # 契约错误不重试


def test_ingest_retries_on_500_then_succeeds():
    calls = {"n": 0}

    def handler(request):
        calls["n"] += 1
        if calls["n"] == 1:
            return httpx.Response(500, text=f"error token={SECRET}")
        return httpx.Response(200, json={"accepted": 2, "deduplicated": 0, "rejected": 0, "errors": []})

    poster = _poster(handler)
    data = asyncio.run(
        poster.send_ingest({"source_id": "s", "source_type": "x", "items": [{"external_id": "1"}, {"external_id": "2"}]})
    )
    assert calls["n"] == 2
    assert data["accepted"] == 2


def test_ingest_rate_limited_exhausts_retries():
    calls = {"n": 0}

    def handler(request):
        calls["n"] += 1
        return httpx.Response(429, json={"error": f"slow down {SECRET}"})

    poster = _poster(handler)
    with pytest.raises(PosterError) as excinfo:
        asyncio.run(poster.send_ingest({"source_id": "s", "source_type": "x", "items": [{"external_id": "1"}]}))
    assert calls["n"] == 3
    assert SECRET not in str(excinfo.value)
    assert SECRET not in excinfo.value.detail  # 重试耗尽后 detail 也已脱敏
    assert "REDACTED" in excinfo.value.detail


def test_ingest_400_detail_is_redacted():
    def handler(request):
        return httpx.Response(400, json={"error": f"bad request echo {SECRET}"})

    poster = _poster(handler)
    with pytest.raises(PosterError) as excinfo:
        asyncio.run(poster.send_ingest({"source_id": "s", "source_type": "x", "items": [{"external_id": "1"}]}))
    assert excinfo.value.status == 400
    assert SECRET not in excinfo.value.detail
    assert "REDACTED" in excinfo.value.detail


def test_heartbeat_payload_delivery():
    seen = {}

    def handler(request):
        seen["body"] = json.loads(request.content.decode("utf-8"))
        return httpx.Response(200, json={"status": "ok", "server_time": "2026-11-15T10:00:00Z"})

    poster = _poster(handler)
    payload = build_heartbeat_payload(source_id="s", plugin_version="0.1.0", status="ok", fetched_count=2)
    asyncio.run(poster.send_heartbeat(payload))
    assert seen["body"]["status"] == "ok"
    assert seen["body"]["fetched_count"] == 2


def test_heartbeat_failure_is_non_fatal():
    def handler(request):
        return httpx.Response(500, text="down")

    poster = _poster(handler)
    payload = build_heartbeat_payload(source_id="s", plugin_version="0.1.0", status="ok", fetched_count=0)
    asyncio.run(poster.send_heartbeat(payload))  # 不抛异常


def test_derive_urls_defaults_and_overrides():
    ingest, heartbeat = derive_urls("http://core:8080")
    assert ingest == "http://core:8080/api/v1/ingest"
    assert heartbeat == "http://core:8080/api/v1/heartbeat"

    ingest, heartbeat = derive_urls("http://core:8080/api/v1")
    assert ingest == "http://core:8080/api/v1/ingest"
    assert heartbeat == "http://core:8080/api/v1/heartbeat"

    ingest, heartbeat = derive_urls("http://core:8080", ingest_url="http://mock:1/api/v1/ingest")
    assert ingest == "http://mock:1/api/v1/ingest"
    assert heartbeat == "http://mock:1/api/v1/heartbeat"
