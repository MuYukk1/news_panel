"""可选 AI 查询规划：成功解析与一切失败场景的无缝静态降级（DEC-005）。"""

import asyncio

import httpx
import pytest

from conftest import make_config

from x_twifork.planner import AIQueryPlanner, QueryPlan, extract_plan_json

STATIC = QueryPlan(accounts=["@static_one"], queries=["static query"])


def _make_planner(handler, enabled=True, merge=True, **cfg_overrides):
    config = make_config()
    config.planner.enabled = enabled
    config.planner.base_url = "https://llm.invalid/v1"
    config.planner.api_key = "sk-test-1234567890"
    config.planner.merge_with_static = merge
    for key, value in cfg_overrides.items():
        setattr(config.planner, key, value)
    return AIQueryPlanner(config.planner, STATIC, secrets=set(), transport=httpx.MockTransport(handler))


def test_disabled_planner_returns_static_without_http():
    calls = {"n": 0}

    def handler(request):
        calls["n"] += 1
        return httpx.Response(200, json={"choices": [{"message": {"content": "{}"}}]})

    planner = _make_planner(handler, enabled=False)
    plan = asyncio.run(planner.refresh(1000.0))
    assert plan.source == "static"
    assert plan.queries == STATIC.queries
    assert plan.accounts == STATIC.accounts
    assert calls["n"] == 0  # 未发任何 HTTP 请求


def test_missing_api_key_falls_back():
    def handler(request):
        raise AssertionError("should not be called")

    planner = _make_planner(handler)
    planner._cfg.api_key = ""
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.source == "static"
    assert plan.queries == STATIC.queries


def test_success_parses_fenced_json_and_merges():
    content = (
        "Here you go:\n```json\n"
        '{"queries": ["LLM pricing"], "accounts": ["@EvalWatch"]}\n'
        "```\nHope this helps."
    )

    def handler(request):
        assert request.headers["Authorization"] == "Bearer sk-test-1234567890"
        assert request.url.path.endswith("/chat/completions")
        return httpx.Response(200, json={"choices": [{"message": {"content": content}}]})

    planner = _make_planner(handler)
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.source == "ai"
    assert plan.queries == ["static query", "LLM pricing"]
    assert plan.accounts == ["@static_one", "evalwatch"]


def test_replace_mode_without_merge():
    content = '{"queries": ["fresh"], "accounts": ["handle1"]}'

    def handler(request):
        return httpx.Response(200, json={"choices": [{"message": {"content": content}}]})

    planner = _make_planner(handler, merge=False)
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.queries == ["fresh"]
    assert plan.accounts == ["handle1"]


def test_http_500_falls_back_to_static():
    def handler(request):
        return httpx.Response(500, text="boom")

    planner = _make_planner(handler)
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.source == "static"
    assert plan.queries == STATIC.queries
    assert "planner failed" in plan.note


def test_malformed_json_falls_back_to_static():
    def handler(request):
        return httpx.Response(
            200, json={"choices": [{"message": {"content": "I suggest {queries: [broken] and more"}}]}
        )

    planner = _make_planner(handler)
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.source == "static"


def test_empty_plan_falls_back_to_static():
    def handler(request):
        return httpx.Response(
            200, json={"choices": [{"message": {"content": '{"queries": [], "accounts": []}'}}]}
        )

    planner = _make_planner(handler)
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.source == "static"


def test_non_string_entries_ignored():
    content = '{"queries": ["ok", 42, null], "accounts": ["ok2"]}'

    def handler(request):
        return httpx.Response(200, json={"choices": [{"message": {"content": content}}]})

    planner = _make_planner(handler)
    plan = asyncio.run(planner.refresh(0.0))
    assert plan.queries == ["static query", "ok"]
    assert plan.accounts == ["@static_one", "ok2"]


def test_due_respects_interval():
    planner = _make_planner(lambda request: httpx.Response(500))
    assert planner.due(0.0)          # 初始即可运行
    asyncio.run(planner.refresh(0.0))
    assert not planner.due(60.0)     # 间隔内不再触发
    assert planner.due(360 * 60 + 1)


def test_extract_plan_json_code_fence_and_prose():
    assert extract_plan_json('Sure!\n```json\n{"queries": ["a"]}\n```\nHope this helps.') == {
        "queries": ["a"]
    }
    assert extract_plan_json('prefix {"queries": ["b"]} suffix') == {"queries": ["b"]}
    with pytest.raises(ValueError):
        extract_plan_json("no json at all")
