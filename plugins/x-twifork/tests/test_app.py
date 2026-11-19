"""app 层账号锁定/封禁熔断单元测试：立即中断、明确心跳、常驻静默挂起与 --once 退出码。"""

import asyncio
import json

import pytest
from conftest import FakeFetcher, FakePoster, make_config, make_tweet

from x_twifork.__main__ import _run_app
from x_twifork.app import PluginApp
from x_twifork.fetcher import FetchError
from x_twifork.state import StateStore

COOKIE_SECRET = "LOCKEDSECRETVALUE0123456789"


def test_account_locked_aborts_cycle_immediately_without_hitting_remaining_sources(tmp_path):
    fetcher = FakeFetcher(
        account_results={
            "@first_account": FetchError(
                f"upstream account locked (AccountLocked) echo={COOKIE_SECRET}",
                "account_locked",
                retryable=False,
            ),
            "@second_account": [make_tweet(id="201", text="should never be fetched")],
        },
        query_results={
            "ai breaking": [make_tweet(id="202", text="should never be searched")],
        },
    )
    config = make_config(tmp_path)
    config.accounts = ["@first_account", "@second_account"]
    config.queries = ["ai breaking"]

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    poster = FakePoster()
    app = PluginApp(
        config,
        state=state,
        fetcher=fetcher,
        poster=poster,
        dry_run=False,
        secrets={COOKIE_SECRET},
    )

    result = asyncio.run(app.run_cycle())

    # 首个来源命中 AccountLocked 后立刻 break，不得再请求后续账号或搜索词
    assert fetcher.calls == [("resolve", "first_account")]
    assert result.status == "error"
    assert result.halt_reason == "account_locked"
    assert result.fetched_count == 0
    assert result.failures == [("account:@first_account", "account_locked")]

    assert len(poster.heartbeats) == 1
    hb = poster.heartbeats[0]
    assert hb["status"] == "error"
    assert "AccountLocked" in (hb.get("error_message") or "")
    assert "failed transiently" not in (hb.get("error_message") or "")
    assert COOKIE_SECRET not in json.dumps(hb, ensure_ascii=False)


def test_account_suspended_mid_cycle_still_aborts_and_reports_error(tmp_path):
    fetcher = FakeFetcher(
        account_results={
            "@first_account": [make_tweet(id="301", text="collected before suspension")],
            "@second_account": FetchError(
                "upstream account suspended (AccountSuspended)",
                "account_suspended",
                retryable=False,
            ),
            "@third_account": [make_tweet(id="302", text="should never be fetched")],
        },
        query_results={
            "deals": [make_tweet(id="303", text="should never be searched")],
        },
    )
    config = make_config(tmp_path)
    config.accounts = ["@first_account", "@second_account", "@third_account"]
    config.queries = ["deals"]

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    poster = FakePoster()
    app = PluginApp(config, state=state, fetcher=fetcher, poster=poster, dry_run=False)

    result = asyncio.run(app.run_cycle())

    # 第 1 个源正常完成，第 2 个源命中 AccountSuspended 后立刻中断，不触碰第 3 个账号与搜索词
    assert ("resolve", "third_account") not in fetcher.calls
    assert ("search", "deals") not in fetcher.calls
    assert result.fetched_count == 1
    assert len(poster.ingests) == 1
    # 即使前序源已成功，账号被封禁仍须报 error 并触发熔断
    assert result.status == "error"
    assert result.halt_reason == "account_suspended"

    hb = poster.heartbeats[-1]
    assert hb["status"] == "error"
    assert "AccountSuspended" in (hb.get("error_message") or "")
    assert "failed transiently" not in (hb.get("error_message") or "")


@pytest.mark.parametrize(
    ("kind", "expected_marker"),
    [
        ("account_locked", "AccountLocked"),
        ("account_suspended", "AccountSuspended"),
    ],
)
def test_run_forever_halts_silently_without_retrying_after_fatal_account_error(
    tmp_path, monkeypatch, kind, expected_marker
):
    fetcher = FakeFetcher(
        account_results={
            "@first_account": FetchError(f"fatal {kind}", kind, retryable=False),
            "@second_account": [make_tweet(id="401")],
        },
        query_results={"q": [make_tweet(id="402")]},
    )
    config = make_config(tmp_path)
    config.accounts = ["@first_account", "@second_account"]
    config.queries = ["q"]
    config.fetch.poll_interval_seconds = 5

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    poster = FakePoster()
    sleep_calls: list[float] = []
    real_sleep = asyncio.sleep

    async def _fail_if_retry_sleep(delay: float) -> None:
        sleep_calls.append(delay)

    monkeypatch.setattr("x_twifork.app.asyncio.sleep", _fail_if_retry_sleep)

    async def _exercise() -> None:
        stop_event = asyncio.Event()
        app = PluginApp(
            config,
            state=state,
            fetcher=fetcher,
            poster=poster,
            dry_run=False,
            stop_event=stop_event,
        )
        task = asyncio.create_task(app.run_forever())
        # 让事件循环运行多轮调度，确认 run_forever 进入静默挂起而不退出、不重试
        for _ in range(5):
            await real_sleep(0)
        assert not task.done(), "run_forever must stay suspended instead of exiting (restart: unless-stopped)"
        assert fetcher.calls == [("resolve", "first_account")]
        assert sleep_calls == []
        assert len(poster.heartbeats) == 1
        assert poster.heartbeats[0]["status"] == "error"
        assert expected_marker in (poster.heartbeats[0].get("error_message") or "")

        stop_event.set()
        await asyncio.wait_for(task, timeout=1.0)
        # 退出前依然只请求过首轮第一个源
        assert fetcher.calls == [("resolve", "first_account")]
        assert len(poster.heartbeats) == 1

    asyncio.run(_exercise())


def test_run_forever_default_suspension_waits_until_cancelled(tmp_path):
    fetcher = FakeFetcher(
        account_results={
            "@first_account": FetchError("account locked", "account_locked", retryable=False),
        }
    )
    config = make_config(tmp_path)
    config.accounts = ["@first_account"]
    config.queries = []

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    poster = FakePoster()

    async def _exercise() -> None:
        app = PluginApp(config, state=state, fetcher=fetcher, poster=poster, dry_run=False)
        task = asyncio.create_task(app.run_forever())
        for _ in range(5):
            await asyncio.sleep(0)
        assert not task.done()
        assert len(poster.heartbeats) == 1
        assert poster.heartbeats[0]["status"] == "error"
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task

    asyncio.run(_exercise())


@pytest.mark.parametrize("kind", ["account_locked", "account_suspended"])
def test_once_mode_returns_exit_code_1_on_fatal_account_error(tmp_path, kind):
    fetcher = FakeFetcher(
        account_results={
            "@first_account": FetchError(f"fatal {kind}", kind, retryable=False),
            "@second_account": [make_tweet(id="501")],
        }
    )
    config = make_config(tmp_path)
    config.accounts = ["@first_account", "@second_account"]
    config.queries = ["q"]

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    poster = FakePoster()
    app = PluginApp(config, state=state, fetcher=fetcher, poster=poster, dry_run=False)

    code = asyncio.run(_run_app(app, poster, once=True))

    assert code == 1
    assert fetcher.calls == [("resolve", "first_account")]
    assert len(poster.heartbeats) == 1
    assert poster.heartbeats[0]["status"] == "error"
