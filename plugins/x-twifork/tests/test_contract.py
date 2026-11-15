"""PLUGIN_SPEC v0.1 契约字段校验（ingest / heartbeat / 时间与标题规范化）。"""

import json

from conftest import make_tweet

from x_twifork.samples import SAMPLE_TWEETS
from x_twifork.transform import (
    MAX_ITEMS_PER_INGEST,
    build_heartbeat_payload,
    build_ingest_payload,
    chunk_items,
    derive_title,
    tweet_to_item,
    twitter_date_to_rfc3339,
    validate_ingest_payload,
    validate_item,
    validate_heartbeat_payload,
)


def _sample_payload():
    items = [tweet_to_item(t, origin_key="account:@demo", origin_label="demo") for t in SAMPLE_TWEETS]
    return build_ingest_payload("x-twifork-main", "x", items)


def test_dry_run_sample_payload_conforms_to_contract():
    assert validate_ingest_payload(_sample_payload()) == []


def test_ingest_payload_shape():
    payload = _sample_payload()
    assert set(payload) == {"source_id", "source_type", "items"}
    assert payload["source_type"] == "x"
    for item in payload["items"]:
        assert set(item) <= {"external_id", "url", "title", "content", "author", "published_at", "raw_meta"}
        for key in ("external_id", "url", "content", "published_at"):
            assert key in item


def test_item_missing_required_field_is_rejected():
    item = tweet_to_item(make_tweet(), origin_key="account:@x", origin_label="x")
    del item["content"]
    errors = validate_ingest_payload(build_ingest_payload("s", "x", [item]))
    assert any("content" in err for err in errors)


def test_item_additional_property_is_rejected():
    item = tweet_to_item(make_tweet(), origin_key="account:@x", origin_label="x")
    item["sneaky"] = "value"
    assert any("sneaky" in err for err in validate_item(item))


def test_item_bad_url_scheme_is_rejected():
    item = tweet_to_item(make_tweet(), origin_key="k", origin_label="k")
    item["url"] = "ftp://example.com/file"
    assert any("url" in err for err in validate_item(item))


def test_item_bad_timestamp_is_rejected():
    item = tweet_to_item(make_tweet(), origin_key="k", origin_label="k")
    item["published_at"] = "2026-11-15 10:00:00"  # 缺 T 与时区，非 RFC 3339
    assert any("published_at" in err for err in validate_item(item))


def test_build_ingest_payload_rejects_over_limit():
    items = [
        {
            "external_id": str(i),
            "url": "https://x.com/i/status/1",
            "content": "c",
            "published_at": "2026-11-15T10:00:00Z",
        }
        for i in range(MAX_ITEMS_PER_INGEST + 1)
    ]
    try:
        build_ingest_payload("s", "x", items)
        raise AssertionError("expected ValueError")
    except ValueError as exc:
        assert str(MAX_ITEMS_PER_INGEST) in str(exc)


def test_chunk_items_splits_at_contract_limit():
    items = [{"external_id": str(i)} for i in range(150)]
    chunks = chunk_items(items, 100)
    assert [len(chunk) for chunk in chunks] == [100, 50]
    assert all(len(chunk) <= MAX_ITEMS_PER_INGEST for chunk in chunks)


def test_heartbeat_payload_conforms():
    payload = build_heartbeat_payload(
        source_id="x-twifork-main",
        plugin_version="0.1.0",
        status="degraded",
        fetched_count=3,
        error_message="upstream rate limit hit",
    )
    assert validate_heartbeat_payload(payload) == []
    assert payload["timestamp"].endswith("Z")


def test_heartbeat_bad_status_rejected():
    payload = build_heartbeat_payload(source_id="s", plugin_version="0.1.0", status="ok", fetched_count=0)
    payload["status"] = "bogus"
    assert any("status" in err for err in validate_heartbeat_payload(payload))


def test_heartbeat_missing_required_rejected():
    errors = validate_heartbeat_payload({"source_id": "s"})
    for key in ("plugin_version", "status", "fetched_count", "timestamp"):
        assert any(key in err for err in errors)


def test_heartbeat_fetched_count_must_be_non_negative_int():
    payload = build_heartbeat_payload(source_id="s", plugin_version="v", status="ok", fetched_count=0)
    payload["fetched_count"] = -1
    assert validate_heartbeat_payload(payload)
    payload["fetched_count"] = True  # bool 是 int 子类，须被拒绝
    assert validate_heartbeat_payload(payload)


def test_heartbeat_extra_property_rejected():
    payload = build_heartbeat_payload(source_id="s", plugin_version="v", status="ok", fetched_count=0)
    payload["extra"] = 1
    assert any("additional property" in err for err in validate_heartbeat_payload(payload))


def test_twitter_date_parses_to_rfc3339_utc():
    assert twitter_date_to_rfc3339("Sun Nov 15 07:58:12 +0000 2026") == "2026-11-15T07:58:12Z"


def test_twitter_date_handles_non_utc_offset():
    assert twitter_date_to_rfc3339("Sun Nov 15 09:58:12 +0200 2026") == "2026-11-15T07:58:12Z"


def test_twitter_date_iso_fallback():
    assert twitter_date_to_rfc3339("2026-11-15T07:58:12+00:00") == "2026-11-15T07:58:12Z"


def test_twitter_date_fallback_to_now_on_garbage():
    value = twitter_date_to_rfc3339("not-a-date")
    assert value.endswith("Z") and "T" in value


def test_derive_title_first_line_and_truncation():
    assert derive_title("first line\nsecond line") == "first line"
    title = derive_title("x" * 200)
    assert len(title) <= 80 and title.endswith("…")


def test_tweet_to_item_author_and_url():
    item = tweet_to_item(
        make_tweet(screen_name="alice", id="123"),
        origin_key="account:@alice",
        origin_label="alice timeline",
    )
    assert item["author"] == "@alice"
    assert item["url"] == "https://x.com/alice/status/123"
    assert item["external_id"] == "123"


def test_raw_meta_carries_metrics_not_secrets():
    item = tweet_to_item(make_tweet(favorite_count=42), origin_key="k", origin_label="l")
    assert item["raw_meta"]["metrics"]["likes"] == 42
    serialized = json.dumps(item, ensure_ascii=False)
    assert "auth_token" not in serialized and "ct0" not in serialized
