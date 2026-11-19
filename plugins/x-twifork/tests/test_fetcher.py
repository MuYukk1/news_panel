"""twifork 适配层规范化逻辑（不依赖真实 twifork 安装）。"""

import asyncio
from types import SimpleNamespace

import pytest

from x_twifork.fetcher import FATAL_ACCOUNT_KINDS, FetchError, TwiforkFetcher, _classify, tweet_to_like
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
    FakeUnauthorized = type("Unauthorized", (Exception,), {"__module__": "twikit.errors"})
    FakeAccountLocked = type("AccountLocked", (Exception,), {"__module__": "twikit.errors"})
    FakeAccountSuspended = type("AccountSuspended", (Exception,), {"__module__": "twikit.errors"})
    FakeTooManyRequests = type("TooManyRequests", (Exception,), {"__module__": "twikit.errors"})
    FakeForbidden = type("Forbidden", (Exception,), {"__module__": "twikit.errors"})
    FakeTwitterException = type("TwitterException", (Exception,), {"__module__": "twikit.errors"})

    auth_err = _classify(FakeUnauthorized())
    assert auth_err.kind == "auth"
    assert auth_err.retryable is False

    locked_err = _classify(FakeAccountLocked())
    assert locked_err.kind == "account_locked"
    assert locked_err.retryable is False
    assert "AccountLocked" in str(locked_err)

    suspended_err = _classify(FakeAccountSuspended())
    assert suspended_err.kind == "account_suspended"
    assert suspended_err.retryable is False
    assert "AccountSuspended" in str(suspended_err)

    assert _classify(FakeTooManyRequests()).kind == "rate_limit"
    assert _classify(FakeForbidden()).kind == "blocked"
    assert _classify(FakeTwitterException()).kind == "transient"
    assert _classify(OSError("net down")).kind == "network"


def test_fetch_error_kind_validated():
    assert FetchError("boom", "bogus-kind").kind == "transient"
    assert FetchError("boom", "auth").kind == "auth"
    assert FetchError("locked", "account_locked", retryable=False).kind == "account_locked"
    assert FetchError("suspended", "account_suspended", retryable=False).kind == "account_suspended"
    assert {"account_locked", "account_suspended"} == set(FATAL_ACCOUNT_KINDS)


def test_check_session_classifies_account_locked_and_drops_client():
    FakeAccountLocked = type("AccountLocked", (Exception,), {"__module__": "twikit.errors"})

    class LockedClient:
        async def is_logged_in(self):
            raise FakeAccountLocked("Account is locked")

    fetcher = TwiforkFetcher({"auth_token": "A" * 32, "ct0": "C" * 32})
    client = LockedClient()
    fetcher._client = client

    with pytest.raises(FetchError) as exc_info:
        asyncio.run(fetcher._check_session(client))

    assert exc_info.value.kind == "account_locked"
    assert exc_info.value.retryable is False
    assert fetcher._client is None
    assert fetcher._session_checked is False
