"""twifork 适配层规范化逻辑（不依赖真实 twifork 安装）。"""

from types import SimpleNamespace

from x_twifork.fetcher import FetchError, _classify, tweet_to_like
from x_twifork.transform import TweetLike


def test_tweet_to_like_normalizes_fields():
    tweet = SimpleNamespace(
        id=123456789,
        text="RT @someone: look at this",
        created_at="Sun Nov 15 07:58:12 +0000 2026",
        user=SimpleNamespace(screen_name="alice", name="Alice A"),
        favorite_count="7",
        retweet_count=None,
        reply_count=0,
        view_count="1234",
        urls=[SimpleNamespace(expanded_url="https://example.com/x"), "https://plain.example/y"],
        media=[SimpleNamespace(media_url="https://media.example/1.jpg")],
        quoted_status_id=999,
        lang="en",
        is_retweet=False,
    )
    like = tweet_to_like(tweet)
    assert isinstance(like, TweetLike)
    assert like.id == "123456789"
    assert like.screen_name == "alice"
    assert like.user_name == "Alice A"
    assert like.favorite_count == 7
    assert like.retweet_count is None
    assert like.view_count == 1234
    assert like.urls == ["https://example.com/x", "https://plain.example/y"]
    assert like.media_urls == ["https://media.example/1.jpg"]
    assert like.quoted_status_id == "999"
    assert like.is_retweet  # RT @ 前缀兜底识别


def test_tweet_to_like_missing_id_returns_none():
    assert tweet_to_like(SimpleNamespace(text="no id here")) is None


def test_classify_maps_twikit_exceptions():
    FakeUnauthorized = type("Unauthorized", (), {"__module__": "twikit.errors"})
    FakeTooManyRequests = type("TooManyRequests", (), {"__module__": "twikit.errors"})
    FakeForbidden = type("Forbidden", (), {"__module__": "twikit.errors"})
    FakeTwitterException = type("TwitterException", (), {"__module__": "twikit.errors"})
    assert _classify(FakeUnauthorized()).kind == "auth"
    assert _classify(FakeTooManyRequests()).kind == "rate_limit"
    assert _classify(FakeForbidden()).kind == "blocked"
    assert _classify(FakeTwitterException()).kind == "transient"
    assert _classify(OSError("net down")).kind == "network"


def test_fetch_error_kind_validated():
    assert FetchError("boom", "bogus-kind").kind == "transient"
    assert FetchError("boom", "auth").kind == "auth"
