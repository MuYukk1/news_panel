"""配置加载：环境变量展开、占位符识别、Cookie 解析与运行时校验。"""

import json

import pytest

from conftest import CONFIG_EXAMPLE

from x_twifork.config import (
    collect_secrets,
    ConfigError,
    is_placeholder,
    load_config,
    resolve_cookies,
    validate_for_runtime,
)

ENV_VARS = ("NEWS_PANEL_INGEST_TOKEN", "X_AUTH_TOKEN", "X_CT0", "X_TWID", "PLANNER_API_KEY")


def _write(tmp_path, text):
    path = tmp_path / "config.yaml"
    path.write_text(text, encoding="utf-8")
    return path


def test_load_example_config_without_env(monkeypatch):
    for var in ENV_VARS:
        monkeypatch.delenv(var, raising=False)
    config = load_config(CONFIG_EXAMPLE)
    assert config.source_id == "x-twifork-main"
    assert config.source_type == "x"
    assert config.core.base_url == "http://news-panel:8080"
    assert config.core.token == ""  # env 未设置 → 展开为空串
    assert config.fetch.max_items_per_batch == 100
    assert config.fetch.poll_interval_seconds == 900
    assert config.accounts and config.queries


def test_env_expansion(monkeypatch, tmp_path):
    monkeypatch.setenv("MY_TOKEN", "tok-abc123")
    config = load_config(_write(tmp_path, "core:\n  base_url: http://c:1\n  token: ${MY_TOKEN}\n"))
    assert config.core.token == "tok-abc123"


def test_unset_env_expands_to_empty(monkeypatch, tmp_path):
    monkeypatch.delenv("NOPE_VAR", raising=False)
    config = load_config(_write(tmp_path, "core:\n  token: ${NOPE_VAR}\n"))
    assert config.core.token == ""


def test_max_items_clamped_to_contract_limit(tmp_path):
    config = load_config(_write(tmp_path, "fetch:\n  max_items_per_batch: 250\n"))
    assert config.fetch.max_items_per_batch == 100


def test_poll_interval_clamped_for_account_safety(tmp_path):
    config = load_config(_write(tmp_path, "fetch:\n  poll_interval_seconds: 2\n"))
    assert config.fetch.poll_interval_seconds == 5


def test_invalid_quiet_hours_rejected(tmp_path):
    with pytest.raises(ConfigError):
        load_config(_write(tmp_path, 'fetch:\n  quiet_hours: "25:00-09:00"\n'))


def test_load_config_none_gives_defaults():
    config = load_config(None)
    assert config.source_id == "x-twifork-main"
    assert config.accounts == [] and config.queries == []


def test_empty_source_id_rejected(tmp_path):
    with pytest.raises(ConfigError):
        load_config(_write(tmp_path, 'source_id: ""\n'))


def test_is_placeholder():
    assert is_placeholder("REPLACE_ME_WITH_REAL_TOKEN")
    assert is_placeholder("your_<token>here")
    assert not is_placeholder("a-real-looking-token-123456")


def test_resolve_cookies_inline(tmp_path):
    config = load_config(_write(tmp_path, "cookies:\n  auth_token: AAAA1111\n  ct0: CCCC2222\n"))
    cookies = resolve_cookies(config)
    assert cookies["auth_token"] == "AAAA1111"
    assert cookies["ct0"] == "CCCC2222"


def test_resolve_cookies_from_browser_export_list(tmp_path):
    cookies_file = tmp_path / "cookies.json"
    cookies_file.write_text(
        json.dumps([{"name": "auth_token", "value": "AAAA1111"}, {"name": "ct0", "value": "CCCC2222"}]),
        encoding="utf-8",
    )
    config = load_config(_write(tmp_path, "cookies:\n  cookies_file: " + json.dumps(str(cookies_file)) + "\n"))
    cookies = resolve_cookies(config)
    assert cookies["auth_token"] == "AAAA1111"


def test_resolve_cookies_requires_auth_token(tmp_path):
    config = load_config(_write(tmp_path, "cookies:\n  ct0: only\n"))
    with pytest.raises(ConfigError):
        resolve_cookies(config)


def test_resolve_cookies_missing_file(tmp_path):
    missing = json.dumps(str(tmp_path / "missing.json"))
    config = load_config(_write(tmp_path, "cookies:\n  cookies_file: " + missing + "\n"))
    with pytest.raises(ConfigError):
        resolve_cookies(config)


def test_validate_for_runtime_blocks_placeholder_token(tmp_path):
    config = load_config(_write(tmp_path, "core:\n  token: REPLACE_ME_NOW\naccounts:\n  - \"@x\"\n"))
    with pytest.raises(ConfigError):
        validate_for_runtime(config, {"auth_token": "A" * 32})


def test_validate_for_runtime_blocks_missing_cookies(tmp_path):
    config = load_config(_write(tmp_path, "core:\n  token: real-token-xyz\naccounts:\n  - \"@x\"\n"))
    with pytest.raises(ConfigError):
        validate_for_runtime(config, {})


def test_validate_for_runtime_requires_sources(tmp_path):
    config = load_config(_write(tmp_path, "core:\n  token: real-token-xyz\n"))
    with pytest.raises(ConfigError):
        validate_for_runtime(config, {"auth_token": "A" * 32})


def test_validate_for_runtime_passes(tmp_path):
    config = load_config(_write(tmp_path, "core:\n  token: real-token-xyz\naccounts:\n  - \"@someone\"\n"))
    validate_for_runtime(config, {"auth_token": "A" * 32})  # 不抛异常


def test_collect_secrets_filters_short_values():
    config = load_config(None)
    config.core.token = "short"
    config.planner.api_key = "a-very-long-api-key-value"
    secrets = collect_secrets(config, {"auth_token": "cookievalue0123456789", "tiny": "ab"})
    assert "cookievalue0123456789" in secrets
    assert "a-very-long-api-key-value" in secrets
    assert "short" not in secrets and "ab" not in secrets
