"""心跳状态判定与异常上报脱敏（app 层集成测试）。"""

import asyncio
import json
from pathlib import Path

from conftest import FakeFetcher, FakePoster, make_config, make_tweet

from x_twifork.app import PluginApp
from x_twifork.fetcher import FetchError
from x_twifork.poster import PosterError
from x_twifork.state import StateStore

COOKIE_SECRET = "AUTHTOKENVALUE0123456789"


def _run_cycle(tmp_path, fetcher, config=None):
    cfg = config or make_config(tmp_path)
    state = StateStore(cfg.state_file, first_run_max_items=cfg.fetch.first_run_max_items)
    poster = FakePoster()
    app = PluginApp(cfg, state=state, fetcher=fetcher, poster=poster, dry_run=False, secrets={COOKIE_SECRET})
    result = asyncio.run(app.run_cycle())
    return result, poster, state


def test_degraded_when_one_source_fails(tmp_path):
    fetcher = FakeFetcher(
        account_results={
            "@good_account": [make_tweet(id="200", text="fresh tweet")],
            "@bad_account": FetchError("upstream rate limit hit", "rate_limit"),
        }
    )
    config = make_config(tmp_path)
    config.accounts = ["@good_account", "@bad_account"]
    result, poster, _ = _run_cycle(tmp_path, fetcher, config)

    assert result.status == "degraded"
    heartbeat = poster.heartbeats[-1]
    assert heartbeat["status"] == "degraded"
    assert "rate limit" in heartbeat["error_message"]
    assert heartbeat["fetched_count"] == 1


def test_error_when_all_sources_auth_fail(tmp_path):
    fetcher = FakeFetcher(
        account_results={"@good_account": FetchError("upstream session rejected", "auth")}
    )
    config = make_config(tmp_path)
    config.accounts = ["@good_account"]
    config.queries = []
    result, poster, _ = _run_cycle(tmp_path, fetcher, config)

    assert result.status == "error"
    heartbeat = poster.heartbeats[-1]
    assert heartbeat["status"] == "error"
    assert "refresh auth_token/ct0" in heartbeat["error_message"]
    assert COOKIE_SECRET not in json.dumps(heartbeat, ensure_ascii=False)


def test_heartbeat_never_leaks_cookie_values(tmp_path):
    fetcher = FakeFetcher(
        account_results={
            "@good_account": FetchError(f"weird upstream error mentioning {COOKIE_SECRET}", "transient")
        }
    )
    config = make_config(tmp_path)
    config.accounts = ["@good_account"]
    config.queries = []
    result, poster, _ = _run_cycle(tmp_path, fetcher, config)

    assert result.status == "error"  # 仅一个来源且失败
    serialized = json.dumps(poster.heartbeats, ensure_ascii=False)
    assert COOKIE_SECRET not in serialized


def test_ingest_error_redacted_in_heartbeat(tmp_path):
    fetcher = FakeFetcher(account_results={"@good_account": [make_tweet(id="300")]})
    config = make_config(tmp_path)
    config.accounts = ["@good_account"]
    config.queries = []

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    poster = FakePoster()
    poster.ingest_error = PosterError(f"ingest rejected: echo {COOKIE_SECRET}", status=400)
    app = PluginApp(
        config, state=state, fetcher=fetcher, poster=poster, dry_run=False, secrets={COOKIE_SECRET}
    )
    result = asyncio.run(app.run_cycle())

    assert result.status == "degraded"
    heartbeat = poster.heartbeats[-1]
    assert COOKIE_SECRET not in json.dumps(heartbeat, ensure_ascii=False)
    assert "REDACTED" in (heartbeat.get("error_message") or "")
    # 投递失败 → 游标不推进，state.json 不落盘（下轮自动重发，核心幂等去重）
    assert not Path(config.state_file).exists()


def test_ok_when_all_sources_healthy(tmp_path):
    fetcher = FakeFetcher(
        account_results={"@good_account": [make_tweet(id="201")]},
        query_results={"test query": [make_tweet(id="202", screen_name="searcher")]},
    )
    result, poster, _ = _run_cycle(tmp_path, fetcher)
    assert result.status == "ok"
    heartbeat = poster.heartbeats[-1]
    assert heartbeat["status"] == "ok"
    assert "error_message" not in heartbeat or heartbeat["error_message"] is None
    assert heartbeat["fetched_count"] == 2
    assert len(poster.ingests) == 1  # 2 条 < 100 → 单批


def test_incremental_no_repeat_across_cycles(tmp_path):
    tweets = [make_tweet(id=str(300 + i)) for i in range(5)]
    fetcher = FakeFetcher(account_results={"@good_account": list(tweets)})
    config = make_config(tmp_path)
    config.accounts = ["@good_account"]
    config.queries = []
    cfg = config
    state = StateStore(cfg.state_file, first_run_max_items=cfg.fetch.first_run_max_items)
    poster = FakePoster()
    app = PluginApp(cfg, state=state, fetcher=fetcher, poster=poster, dry_run=False, secrets=set())

    first = asyncio.run(app.run_cycle())
    assert first.fetched_count == 5          # 首轮种子回灌
    second = asyncio.run(app.run_cycle())
    assert second.fetched_count == 0         # 同一批推文不再重复投递（增量断点生效）
    assert len(poster.ingests) == 1
