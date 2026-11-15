"""twifork（twikit 兼容 API）采集适配层。

twifork 以 ``twikit`` 模块名提供（drop-in 兼容）。为让单元测试与 --dry-run
不依赖真实安装，本模块采用惰性导入：只有真正发起采集时才 ``import twikit``。

集成点（对应 PawiX25/twifork v2.3.x，MIT）：
- ``Client(language, proxy=..., impersonate=...)``：proxy 透传 httpx/curl_cffi；
- ``client.set_cookies({...})``：注入 auth_token / ct0 / twid；
- ``client.get_user_tweets(user_id, 'Tweets', count=...)``：用户时间线；
- ``client.search_tweet(query, 'Latest', count=...)``：实时搜索；
- ``client.get_user_by_screen_name(handle)``：handle → user_id 解析。

本模块绝不调用 ``login()``（DEC-005 红线），仅复用既有会话 Cookie。
"""

from __future__ import annotations

import logging
from typing import Any

from .transform import TweetLike

logger = logging.getLogger(__name__)

FETCH_KINDS = ("auth", "rate_limit", "blocked", "transient", "network")

_AUTH_EXC = {"Unauthorized"}
_RATE_EXC = {"TooManyRequests"}
_BLOCK_EXC = {"Forbidden"}  # 常见于 Cloudflare 拦截页


class FetchError(Exception):
    """上游拉取失败；``kind`` 用于退避策略与心跳状态判定。"""

    def __init__(self, message: str, kind: str = "transient", *, retryable: bool = True) -> None:
        super().__init__(message)
        self.kind = kind if kind in FETCH_KINDS else "transient"
        self.retryable = retryable


def _classify(exc: Exception) -> FetchError:
    name = type(exc).__name__
    module = type(exc).__module__ or ""
    if "twikit" in module or "twifork" in module:
        if name in _AUTH_EXC:
            return FetchError(f"upstream session rejected ({name}) — cookies likely stale", "auth")
        if name in _RATE_EXC:
            return FetchError(f"upstream rate limit hit ({name})", "rate_limit")
        if name in _BLOCK_EXC:
            return FetchError(f"upstream blocked the request ({name}) — possible Cloudflare challenge", "blocked")
        return FetchError(f"upstream fetch failed ({name})", "transient")
    return FetchError(f"network error during fetch: {type(exc).__name__}", "network")


def _coerce_int(value: Any) -> int | None:
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _collect_str_list(source: Any, *attrs: str) -> list[str]:
    """从 twikit 对象列表 / 字符串列表中收集去重后的非空字符串。"""
    out: list[str] = []
    for entry in source or []:
        if isinstance(entry, str):
            candidate = entry
        else:
            candidate = ""
            for attr in attrs:
                value = getattr(entry, attr, None)
                if isinstance(value, str) and value.strip():
                    candidate = value
                    break
        if candidate and candidate.strip():
            stripped = candidate.strip()
            if stripped not in out:
                out.append(stripped)
    return out


def tweet_to_like(tweet: Any) -> TweetLike | None:
    """twikit Tweet → 规范化 TweetLike（全字段防御式读取，缺省安全兜底）。"""
    tweet_id = getattr(tweet, "id", None)
    if tweet_id is None or str(tweet_id) == "":
        return None
    user = getattr(tweet, "user", None)
    text = str(getattr(tweet, "text", None) or getattr(tweet, "full_text", "") or "")
    return TweetLike(
        id=str(tweet_id),
        text=text,
        created_at_raw=str(getattr(tweet, "created_at", "") or ""),
        screen_name=str(getattr(user, "screen_name", "") or "") if user is not None else "",
        user_name=str(getattr(user, "name", "") or "") if user is not None else "",
        favorite_count=_coerce_int(getattr(tweet, "favorite_count", None)),
        retweet_count=_coerce_int(getattr(tweet, "retweet_count", None)),
        reply_count=_coerce_int(getattr(tweet, "reply_count", None)),
        view_count=_coerce_int(getattr(tweet, "view_count", None)),
        urls=_collect_str_list(getattr(tweet, "urls", None), "expanded_url", "url"),
        media_urls=_collect_str_list(getattr(tweet, "media", None), "media_url", "url"),
        quoted_status_id=str(getattr(tweet, "quoted_status_id", "") or ""),
        lang=str(getattr(tweet, "lang", "") or ""),
        is_retweet=bool(getattr(tweet, "is_retweet", False)) or text.startswith("RT @"),
    )


class TwiforkFetcher:
    """twifork 客户端封装：Cookie 注入、代理、TLS 指纹与异常分类。"""

    def __init__(
        self,
        cookies: dict[str, str],
        *,
        proxy: str = "",
        impersonate: str = "chrome124",
        max_tweets: int = 40,
        search_product: str = "Latest",
        include_retweets: bool = False,
    ) -> None:
        self._cookies = dict(cookies)
        self._proxy = (proxy or "").strip() or None
        self._impersonate = (impersonate or "").strip() or None
        self._max_tweets = max(1, int(max_tweets))
        self._search_product = (search_product or "Latest").strip() or "Latest"
        self._include_retweets = bool(include_retweets)
        self._client: Any = None
        self._session_checked = False

    # ---------- 客户端生命周期 ----------
    async def _ensure_client(self) -> Any:
        if self._client is not None:
            return self._client
        try:
            from twikit import Client  # twifork 以 twikit 模块名提供（drop-in 兼容）
        except ImportError as exc:
            raise RuntimeError(
                "twifork is not installed; run `pip install twifork[impersonate]` "
                "(see requirements.txt)"
            ) from exc
        try:
            client = Client("en-US", proxy=self._proxy, impersonate=self._impersonate)
        except TypeError:
            # 旧版本客户端无 impersonate 参数
            client = Client("en-US", proxy=self._proxy)
        except ImportError as exc:
            raise RuntimeError(f"twifork[impersonate] extra missing (curl_cffi): {exc}") from exc
        client.set_cookies(self._cookies)
        self._client = client
        return client

    async def _check_session(self, client: Any) -> None:
        if self._session_checked:
            return
        logged_in = bool(await client.is_logged_in())
        self._session_checked = True
        if not logged_in:
            self._client = None
            self._session_checked = False
            raise FetchError(
                "upstream reports session invalid — refresh auth_token/ct0 cookies",
                "auth",
                retryable=False,
            )

    def _maybe_drop_client(self, exc: Exception) -> None:
        if type(exc).__name__ in _AUTH_EXC:
            self._client = None
            self._session_checked = False

    # ---------- 上游操作 ----------
    async def resolve_user_id(self, account: str) -> str:
        account = account.strip()
        if account.isdigit():
            return account
        client = await self._ensure_client()
        await self._check_session(client)
        try:
            user = await client.get_user_by_screen_name(account.lstrip("@"))
        except Exception as exc:
            raise _classify(exc) from exc
        user_id = str(getattr(user, "id", "") or "")
        if not user_id:
            raise FetchError(f"could not resolve user id for account {account!r}", "transient")
        return user_id

    async def fetch_user_tweets(self, user_id: str) -> list[TweetLike]:
        client = await self._ensure_client()
        await self._check_session(client)
        try:
            result = await client.get_user_tweets(str(user_id), "Tweets", count=self._max_tweets)
        except Exception as exc:
            self._maybe_drop_client(exc)
            raise _classify(exc) from exc
        return self._filter(self._to_likes(result))

    async def fetch_search(self, query: str) -> list[TweetLike]:
        client = await self._ensure_client()
        await self._check_session(client)
        try:
            result = await client.search_tweet(query, self._search_product, count=self._max_tweets)
        except Exception as exc:
            self._maybe_drop_client(exc)
            raise _classify(exc) from exc
        return self._filter(self._to_likes(result))

    # ---------- 结果规范化 ----------
    def _to_likes(self, result: Any) -> list[TweetLike]:
        likes: list[TweetLike] = []
        for tweet in list(result or []):
            like = tweet_to_like(tweet)
            if like is not None:
                likes.append(like)
        return likes

    def _filter(self, likes: list[TweetLike]) -> list[TweetLike]:
        if self._include_retweets:
            return likes
        return [like for like in likes if not like.is_retweet]
