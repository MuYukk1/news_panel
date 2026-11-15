"""推文规范化与 PLUGIN_SPEC v0.1 契约 payload 构造 / 校验（唯一契约实现点）。

构造函数产出的 payload 与 docs/PLUGIN_SPEC.md 的 JSON Schema 逐字段对齐；
配套的 ``validate_*`` 校验器镜像同一 Schema（required / additionalProperties=false /
minLength / maxLength / format），供 dry-run 自检与单元测试复用。
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any

logger = logging.getLogger(__name__)

TWITTER_DATE_FORMAT = "%a %b %d %H:%M:%S %z %Y"   # X API 原生时间格式
RFC3339_FORMAT = "%Y-%m-%dT%H:%M:%SZ"
MAX_ITEMS_PER_INGEST = 100                        # PLUGIN_SPEC: items 1..100
TITLE_MAX_CHARS = 80

RFC3339_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$")
HTTP_URL_RE = re.compile(r"^https?://[^\s]+$")

ITEM_REQUIRED_KEYS = ("external_id", "url", "content", "published_at")
ITEM_ALLOWED_KEYS = {"external_id", "url", "title", "content", "author", "published_at", "raw_meta"}
HEARTBEAT_REQUIRED_KEYS = ("source_id", "plugin_version", "status", "fetched_count", "timestamp")
HEARTBEAT_ALLOWED_KEYS = {"source_id", "plugin_version", "status", "fetched_count", "error_message", "timestamp"}
HEARTBEAT_STATUSES = ("ok", "degraded", "error")


@dataclass
class TweetLike:
    """与 twikit Tweet 解耦的规范化推文视图（真实采集与 dry-run 样例共用）。"""

    id: str
    text: str
    created_at_raw: str
    screen_name: str = ""
    user_name: str = ""
    favorite_count: int | None = None
    retweet_count: int | None = None
    reply_count: int | None = None
    view_count: int | None = None
    urls: list[str] = field(default_factory=list)
    media_urls: list[str] = field(default_factory=list)
    quoted_status_id: str = ""
    lang: str = ""
    is_retweet: bool = False


# ---------- 时间规范化 ----------

def twitter_date_to_rfc3339(raw: str, *, now: datetime | None = None) -> str:
    """X 原生时间 / ISO 字符串 → RFC 3339 UTC；无法解析时回退当前时间。"""
    now = now or datetime.now(timezone.utc)
    raw = str(raw or "").strip()
    if raw:
        try:
            parsed = datetime.strptime(raw, TWITTER_DATE_FORMAT)
            return parsed.astimezone(timezone.utc).strftime(RFC3339_FORMAT)
        except ValueError:
            pass
        try:
            parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
            if parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=timezone.utc)
            return parsed.astimezone(timezone.utc).strftime(RFC3339_FORMAT)
        except ValueError:
            logger.warning("unrecognized timestamp %r; falling back to current time", raw)
    return now.strftime(RFC3339_FORMAT)


# ---------- 字段规范化 ----------

def derive_title(text: str) -> str:
    """X 无原生标题：按契约建议截取首行摘要（截断至 80 字符）。"""
    title = ""
    for line in str(text or "").splitlines():
        collapsed = " ".join(line.split())
        if collapsed:
            title = collapsed
            break
    if len(title) > TITLE_MAX_CHARS:
        title = title[: TITLE_MAX_CHARS - 1].rstrip() + "…"
    return title


def tweet_to_item(tweet: TweetLike, *, origin_key: str, origin_label: str) -> dict:
    """TweetLike → PLUGIN_SPEC item（不含敏感信息，raw_meta 仅含指标/链接等公开元数据）。"""
    tweet_id = str(tweet.id)
    screen = (tweet.screen_name or "").lstrip("@")
    if screen:
        url = f"https://x.com/{screen}/status/{tweet_id}"
    elif tweet.urls:
        url = tweet.urls[0]
    else:
        url = f"https://x.com/i/status/{tweet_id}"
    author = f"@{screen}" if screen else (tweet.user_name or "unknown")
    content = str(tweet.text or "").strip() or "(media-only tweet)"
    raw_meta = {
        "origin_key": origin_key,
        "origin_label": origin_label,
        "via": "x-twifork",
        "lang": tweet.lang or None,
        "is_retweet": bool(tweet.is_retweet),
        "quoted_status_id": tweet.quoted_status_id or None,
        "metrics": {
            "likes": tweet.favorite_count,
            "retweets": tweet.retweet_count,
            "replies": tweet.reply_count,
            "views": tweet.view_count,
        },
        "outgoing_urls": list(tweet.urls),
        "media_urls": list(tweet.media_urls),
    }
    raw_meta = {key: value for key, value in raw_meta.items() if value is not None}
    return {
        "external_id": tweet_id,
        "url": url,
        "title": derive_title(tweet.text),
        "content": content,
        "author": author,
        "published_at": twitter_date_to_rfc3339(tweet.created_at_raw),
        "raw_meta": raw_meta,
    }


# ---------- payload 构造 ----------

def build_ingest_payload(source_id: str, source_type: str, items: list[dict]) -> dict:
    if not items:
        raise ValueError("ingest payload requires at least 1 item")
    if len(items) > MAX_ITEMS_PER_INGEST:
        raise ValueError(
            f"ingest payload supports at most {MAX_ITEMS_PER_INGEST} items per call (got {len(items)})"
        )
    return {"source_id": source_id, "source_type": source_type, "items": list(items)}


def chunk_items(items: list[dict], size: int) -> list[list[dict]]:
    size = max(1, min(int(size), MAX_ITEMS_PER_INGEST))
    return [items[i : i + size] for i in range(0, len(items), size)]


def build_heartbeat_payload(
    *,
    source_id: str,
    plugin_version: str,
    status: str,
    fetched_count: int,
    error_message: str | None = None,
    now: datetime | None = None,
) -> dict:
    if status not in HEARTBEAT_STATUSES:
        raise ValueError(f"invalid heartbeat status: {status!r}")
    if isinstance(fetched_count, bool) or not isinstance(fetched_count, int) or fetched_count < 0:
        raise ValueError("fetched_count must be a non-negative integer")
    payload: dict[str, Any] = {
        "source_id": source_id,
        "plugin_version": plugin_version,
        "status": status,
        "fetched_count": int(fetched_count),
        "timestamp": (now or datetime.now(timezone.utc)).strftime(RFC3339_FORMAT),
    }
    if error_message:
        payload["error_message"] = str(error_message)
    return payload


# ---------- 契约校验（镜像 docs/PLUGIN_SPEC.md JSON Schema） ----------

def validate_item(item: Any, path: str = "items[*]") -> list[str]:
    errors: list[str] = []
    if not isinstance(item, dict):
        return [f"{path}: must be a JSON object"]
    for key in item:
        if key not in ITEM_ALLOWED_KEYS:
            errors.append(f"{path}.{key}: additional property not allowed (additionalProperties=false)")
    for key in ITEM_REQUIRED_KEYS:
        if item.get(key) is None:
            errors.append(f"{path}.{key}: required field missing")

    external_id = item.get("external_id")
    if external_id is not None:
        if not isinstance(external_id, str):
            errors.append(f"{path}.external_id: must be a string")
        elif not 1 <= len(external_id) <= 256:
            errors.append(f"{path}.external_id: length must be 1..256")

    url = item.get("url")
    if url is not None:
        if not isinstance(url, str) or len(url) < 1:
            errors.append(f"{path}.url: must be a non-empty string")
        elif not HTTP_URL_RE.match(url):
            errors.append(f"{path}.url: must be an http(s) URI")

    for optional_text in ("title", "author"):
        value = item.get(optional_text)
        if value is not None and not isinstance(value, str):
            errors.append(f"{path}.{optional_text}: must be a string")

    content = item.get("content")
    if content is not None:
        if not isinstance(content, str) or len(content) < 1:
            errors.append(f"{path}.content: must be a non-empty string")

    published_at = item.get("published_at")
    if published_at is not None:
        if not isinstance(published_at, str) or not RFC3339_RE.match(published_at):
            errors.append(f"{path}.published_at: must be RFC 3339 UTC date-time (e.g. 2026-11-15T11:20:00Z)")

    raw_meta = item.get("raw_meta")
    if raw_meta is not None and not isinstance(raw_meta, dict):
        errors.append(f"{path}.raw_meta: must be an object")
    return errors


def validate_ingest_payload(payload: Any) -> list[str]:
    errors: list[str] = []
    if not isinstance(payload, dict):
        return ["payload: must be a JSON object"]
    for key in payload:
        if key not in ("source_id", "source_type", "items"):
            errors.append(f"{key}: additional property not allowed (additionalProperties=false)")
    for key in ("source_id", "source_type", "items"):
        if payload.get(key) is None:
            errors.append(f"{key}: required field missing")

    source_id = payload.get("source_id")
    if source_id is not None:
        if not isinstance(source_id, str) or not 1 <= len(source_id) <= 128:
            errors.append("source_id: must be a string of length 1..128")

    source_type = payload.get("source_type")
    if source_type is not None:
        if not isinstance(source_type, str) or not 1 <= len(source_type) <= 64:
            errors.append("source_type: must be a string of length 1..64")

    items = payload.get("items")
    if items is not None:
        if not isinstance(items, list) or not 1 <= len(items) <= MAX_ITEMS_PER_INGEST:
            errors.append(f"items: must be an array of 1..{MAX_ITEMS_PER_INGEST}")
        else:
            for index, item in enumerate(items):
                errors.extend(validate_item(item, path=f"items[{index}]"))
    return errors


def validate_heartbeat_payload(payload: Any) -> list[str]:
    errors: list[str] = []
    if not isinstance(payload, dict):
        return ["payload: must be a JSON object"]
    for key in payload:
        if key not in HEARTBEAT_ALLOWED_KEYS:
            errors.append(f"{key}: additional property not allowed (additionalProperties=false)")
    for key in HEARTBEAT_REQUIRED_KEYS:
        if payload.get(key) is None:
            errors.append(f"{key}: required field missing")

    status = payload.get("status")
    if status is not None and status not in HEARTBEAT_STATUSES:
        errors.append(f"status: must be one of {HEARTBEAT_STATUSES}")

    fetched_count = payload.get("fetched_count")
    if fetched_count is not None:
        if isinstance(fetched_count, bool) or not isinstance(fetched_count, int) or fetched_count < 0:
            errors.append("fetched_count: must be an integer >= 0")

    timestamp = payload.get("timestamp")
    if timestamp is not None:
        if not isinstance(timestamp, str) or not RFC3339_RE.match(timestamp):
            errors.append("timestamp: must be RFC 3339 UTC date-time")

    error_message = payload.get("error_message")
    if error_message is not None and not isinstance(error_message, str):
        errors.append("error_message: must be a string or null")
    return errors
