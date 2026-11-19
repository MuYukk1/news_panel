"""pytest 共享夹具与测试工厂（无真实网络、无真实 twifork 依赖）。"""

import sys
from pathlib import Path

PLUGIN_ROOT = Path(__file__).resolve().parents[1]
TOOLS_DIR = PLUGIN_ROOT / "tools"
if str(TOOLS_DIR) not in sys.path:
    sys.path.insert(0, str(TOOLS_DIR))  # 使测试可导入 tools/mock_core.py

from x_twifork.config import (  # noqa: E402
    BackoffConfig,
    Config,
    CoreConfig,
    CookieConfig,
    FetchConfig,
    PlannerConfig,
)
from x_twifork.transform import TweetLike  # noqa: E402

CONFIG_EXAMPLE = PLUGIN_ROOT / "config.example.yaml"


def make_tweet(**overrides) -> TweetLike:
    """TweetLike 测试工厂：字段可逐项覆盖。"""
    fields = dict(
        id="1900000000000000200",
        text="Hello world from the test factory",
        created_at_raw="Sun Nov 15 10:00:00 +0000 2026",
        screen_name="test_user",
        user_name="Test User",
        favorite_count=10,
        retweet_count=2,
        reply_count=1,
        view_count=500,
        urls=[],
        media_urls=[],
        quoted_status_id="",
        lang="en",
        is_retweet=False,
    )
    fields.update(overrides)
    return TweetLike(**fields)


def make_config(tmp_path=None, **section_overrides) -> Config:
    """构建合理默认的 Config（隔离 tmp_path，无外部副作用）。"""
    state_file = str(tmp_path / "state.json") if tmp_path else "state.json"
    config = Config(
        source_id="x-twifork-test",
        source_type="x",
        state_file=state_file,
        core=CoreConfig(base_url="http://mock-core.invalid", token="test-token-0123456789"),
        cookies=CookieConfig(auth_token="A" * 32, ct0="C" * 32),
        fetch=FetchConfig(
            poll_interval_seconds=900,
            jitter_seconds=30,
            quiet_hours="",
            active_hours="",
            pace_seconds=0.0,
        ),
        backoff=BackoffConfig(),
        planner=PlannerConfig(enabled=False),
        accounts=["@good_account"],
        queries=["test query"],
    )
    for section, values in section_overrides.items():
        target = getattr(config, section)
        for key, value in values.items():
            setattr(target, key, value)
    return config


class FakeFetcher:
    """可编程假采集器：ref → TweetLike 列表 / FetchError 异常。

    account_results 的键兼容 "@handle" 与 "handle"（app 层会先清洗掉 @ 再传入）；
    每个 handle 解析为独立 user_id，保证 fetch_user_tweets 能正确回映射到来源。
    """

    def __init__(self, account_results=None, query_results=None, fixed_user_id="111") -> None:
        self.account_results = {key.lstrip("@"): value for key, value in (account_results or {}).items()}
        self.query_results = query_results or {}
        self.fixed_user_id = fixed_user_id
        self.resolved: dict[str, str] = {}
        self.calls: list[tuple[str, str]] = []

    async def resolve_user_id(self, account: str) -> str:
        clean = account.lstrip("@")
        self.calls.append(("resolve", clean))
        value = self.account_results.get(clean)
        if isinstance(value, Exception):
            raise value
        uid = f"{self.fixed_user_id}{len(self.resolved):02d}"
        self.resolved[clean] = uid
        return uid

    async def fetch_user_tweets(self, user_id: str) -> list:
        self.calls.append(("user", user_id))
        ref = next((r for r, uid in self.resolved.items() if uid == user_id), user_id)
        return self._result(ref, self.account_results)

    async def fetch_search(self, query: str) -> list:
        self.calls.append(("search", query))
        return self._result(query, self.query_results)

    def _result(self, ref: str, table: dict) -> list:
        if ref in table:
            value = table[ref]
            if isinstance(value, Exception):
                raise value
            return list(value)
        return []


class FakePoster:
    """可编程假投递器：记录 ingest/heartbeat 载荷，可注入异常。"""

    def __init__(self) -> None:
        self.ingests: list[dict] = []
        self.heartbeats: list[dict] = []
        self.ingest_error: Exception | None = None

    async def send_ingest(self, payload: dict) -> dict:
        if self.ingest_error is not None:
            raise self.ingest_error
        self.ingests.append(payload)
        return {"accepted": len(payload["items"]), "deduplicated": 0, "rejected": 0, "errors": []}

    async def send_heartbeat(self, payload: dict) -> None:
        self.heartbeats.append(payload)

    async def aclose(self) -> None:
        return None
