"""配置加载：YAML 解析 + ``${ENV_VAR}`` 展开 + 运行时校验 + Cookie 解析。

设计要点：
- 所有字符串叶子值支持 ``${ENV_VAR}`` 展开，凭证类配置建议全部走环境变量；
- Cookie 支持内联三件套（auth_token/ct0/twid）或 twifork 兼容 cookies.json；
- :func:`collect_secrets` 收集全部密钥值供 :mod:`x_twifork.redact` 脱敏使用。
"""

from __future__ import annotations

import json
import logging
import os
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml

from .scheduler import ScheduleError, parse_window

logger = logging.getLogger(__name__)

ENV_PATTERN = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)\}")

# 出现这些子串的值视为“仍是占位符”，禁止用于真实采集
PLACEHOLDER_MARKS = ("REPLACE_ME", "CHANGEME", "YOUR_", "<", "xxx")

# PLUGIN_SPEC v0.1 契约硬上限
MAX_ITEMS_PER_BATCH_LIMIT = 100

KNOWN_TOP_KEYS = {
    "source_id", "source_type", "state_file", "proxy", "impersonate", "log_level",
    "core", "cookies", "fetch", "backoff", "planner", "accounts", "queries",
}


class ConfigError(Exception):
    """配置缺失或非法。"""


@dataclass
class CoreConfig:
    base_url: str = "http://news-panel:8080"
    token: str = ""
    timeout_seconds: float = 20.0
    ingest_max_retries: int = 3


@dataclass
class CookieConfig:
    auth_token: str = ""
    ct0: str = ""
    twid: str = ""
    cookies_file: str = ""


@dataclass
class FetchConfig:
    poll_interval_seconds: int = 900
    jitter_seconds: int = 120
    max_delay_seconds: int = 7200
    quiet_hours: str = ""           # "23:30-08:00"，可跨午夜
    active_hours: str = ""          # "08:00-23:30"（与 quiet_hours 同时配置时 quiet 优先）
    quiet_hours_multiplier: float = 3.0
    timezone: str = "UTC"
    max_tweets_per_query: int = 40
    max_items_per_batch: int = 100
    search_product: str = "Latest"
    include_retweets: bool = False
    first_run_max_items: int = 20
    pace_seconds: float = 2.0


@dataclass
class BackoffConfig:
    initial_seconds: int = 60
    max_seconds: int = 3600
    multiplier: float = 2.0
    auth_backoff_seconds: int = 900
    auth_error_threshold: int = 3


@dataclass
class PlannerConfig:
    enabled: bool = False
    base_url: str = ""
    api_key: str = ""
    model: str = "gpt-4o-mini"
    topics: list[str] = field(default_factory=list)
    interval_minutes: int = 360
    merge_with_static: bool = True
    max_queries: int = 8
    max_accounts: int = 12
    timeout_seconds: float = 20.0


@dataclass
class Config:
    source_id: str = "x-twifork-main"
    source_type: str = "x"
    state_file: str = "state.json"
    proxy: str = ""
    impersonate: str = "chrome124"
    log_level: str = "INFO"
    core: CoreConfig = field(default_factory=CoreConfig)
    cookies: CookieConfig = field(default_factory=CookieConfig)
    fetch: FetchConfig = field(default_factory=FetchConfig)
    backoff: BackoffConfig = field(default_factory=BackoffConfig)
    planner: PlannerConfig = field(default_factory=PlannerConfig)
    accounts: list[str] = field(default_factory=list)
    queries: list[str] = field(default_factory=list)


def is_placeholder(value: str) -> bool:
    value = str(value or "")
    return any(mark in value for mark in PLACEHOLDER_MARKS)


def _expand_env(value: Any) -> Any:
    if isinstance(value, str):
        def _repl(match: re.Match[str]) -> str:
            name = match.group(1)
            if name not in os.environ:
                logger.warning("config references unset env var %s; expanding to empty string", name)
                return ""
            return os.environ[name]

        return ENV_PATTERN.sub(_repl, value)
    if isinstance(value, dict):
        return {key: _expand_env(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_expand_env(item) for item in value]
    return value


def _string_list(raw: Any) -> list[str]:
    if not isinstance(raw, list):
        return []
    return [str(item).strip() for item in raw if isinstance(item, str) and str(item).strip()]


def _section(raw: dict, key: str) -> dict:
    value = raw.get(key) or {}
    if not isinstance(value, dict):
        raise ConfigError(f"config section '{key}' must be a mapping")
    return value


def _warn_unknown_keys(raw: dict, known: set[str], where: str) -> None:
    for key in raw:
        if key not in known:
            logger.warning("unknown config key %s.%s ignored", where, key)


def load_config(path: str | os.PathLike | None) -> Config:
    """从 YAML 文件加载配置；``path=None`` 或文件不存在时返回默认值。

    文件不存在是否允许由调用方决定（真实模式要求配置文件必须存在）。
    """
    raw: dict[str, Any] = {}
    if path is not None:
        config_path = Path(path)
        if config_path.exists():
            try:
                loaded = yaml.safe_load(config_path.read_text(encoding="utf-8"))
            except yaml.YAMLError as exc:
                raise ConfigError(f"config file {config_path} is not valid YAML: {exc}") from exc
            if loaded is None:
                loaded = {}
            if not isinstance(loaded, dict):
                raise ConfigError(f"config file {config_path} root must be a mapping")
            raw = loaded
    raw = _expand_env(raw)
    _warn_unknown_keys(raw, KNOWN_TOP_KEYS, "config")
    config = _build_config(raw)
    _validate_windows(config)
    return config


def _build_config(raw: dict) -> Config:
    core_raw = _section(raw, "core")
    cookies_raw = _section(raw, "cookies")
    fetch_raw = _section(raw, "fetch")
    backoff_raw = _section(raw, "backoff")
    planner_raw = _section(raw, "planner")

    fetch = FetchConfig(
        poll_interval_seconds=int(fetch_raw.get("poll_interval_seconds", 900)),
        jitter_seconds=int(fetch_raw.get("jitter_seconds", 120)),
        max_delay_seconds=int(fetch_raw.get("max_delay_seconds", 7200)),
        quiet_hours=str(fetch_raw.get("quiet_hours", "") or "").strip(),
        active_hours=str(fetch_raw.get("active_hours", "") or "").strip(),
        quiet_hours_multiplier=float(fetch_raw.get("quiet_hours_multiplier", 3.0)),
        timezone=str(fetch_raw.get("timezone", "UTC") or "UTC"),
        max_tweets_per_query=int(fetch_raw.get("max_tweets_per_query", 40)),
        max_items_per_batch=int(fetch_raw.get("max_items_per_batch", 100)),
        search_product=str(fetch_raw.get("search_product", "Latest") or "Latest"),
        include_retweets=bool(fetch_raw.get("include_retweets", False)),
        first_run_max_items=int(fetch_raw.get("first_run_max_items", 20)),
        pace_seconds=float(fetch_raw.get("pace_seconds", 2.0)),
    )
    if fetch.max_items_per_batch > MAX_ITEMS_PER_BATCH_LIMIT:
        logger.warning(
            "fetch.max_items_per_batch=%d exceeds contract limit %d; clamped",
            fetch.max_items_per_batch, MAX_ITEMS_PER_BATCH_LIMIT,
        )
        fetch.max_items_per_batch = MAX_ITEMS_PER_BATCH_LIMIT
    if fetch.max_items_per_batch < 1:
        fetch.max_items_per_batch = 1
    if fetch.poll_interval_seconds < 5:
        logger.warning("poll_interval_seconds < 5s is risky for account safety; clamped to 5")
        fetch.poll_interval_seconds = 5

    core = CoreConfig(
        base_url=str(core_raw.get("base_url", "http://news-panel:8080") or "").rstrip("/"),
        token=str(core_raw.get("token", "") or ""),
        timeout_seconds=float(core_raw.get("timeout_seconds", 20.0)),
        ingest_max_retries=max(1, int(core_raw.get("ingest_max_retries", 3))),
    )
    cookies = CookieConfig(
        auth_token=str(cookies_raw.get("auth_token", "") or ""),
        ct0=str(cookies_raw.get("ct0", "") or ""),
        twid=str(cookies_raw.get("twid", "") or ""),
        cookies_file=str(cookies_raw.get("cookies_file", "") or ""),
    )
    backoff = BackoffConfig(
        initial_seconds=int(backoff_raw.get("initial_seconds", 60)),
        max_seconds=int(backoff_raw.get("max_seconds", 3600)),
        multiplier=float(backoff_raw.get("multiplier", 2.0)),
        auth_backoff_seconds=int(backoff_raw.get("auth_backoff_seconds", 900)),
        auth_error_threshold=int(backoff_raw.get("auth_error_threshold", 3)),
    )
    planner = PlannerConfig(
        enabled=bool(planner_raw.get("enabled", False)),
        base_url=str(planner_raw.get("base_url", "") or "").rstrip("/"),
        api_key=str(planner_raw.get("api_key", "") or ""),
        model=str(planner_raw.get("model", "gpt-4o-mini") or "gpt-4o-mini"),
        topics=_string_list(planner_raw.get("topics")),
        interval_minutes=max(1, int(planner_raw.get("interval_minutes", 360))),
        merge_with_static=bool(planner_raw.get("merge_with_static", True)),
        max_queries=max(1, int(planner_raw.get("max_queries", 8))),
        max_accounts=max(1, int(planner_raw.get("max_accounts", 12))),
        timeout_seconds=float(planner_raw.get("timeout_seconds", 20.0)),
    )

    source_id = str(raw.get("source_id", "x-twifork-main") or "").strip()
    if not source_id:
        raise ConfigError("source_id must be a non-empty string")
    if len(source_id) > 128:
        raise ConfigError("source_id exceeds 128 chars (PLUGIN_SPEC limit)")
    source_type = str(raw.get("source_type", "x") or "x").strip() or "x"

    return Config(
        source_id=source_id,
        source_type=source_type,
        state_file=str(raw.get("state_file", "state.json") or "state.json"),
        proxy=str(raw.get("proxy", "") or "").strip(),
        impersonate=str(raw.get("impersonate", "chrome124") or "").strip(),
        log_level=str(raw.get("log_level", "INFO") or "INFO").upper(),
        core=core,
        cookies=cookies,
        fetch=fetch,
        backoff=backoff,
        planner=planner,
        accounts=_string_list(raw.get("accounts")),
        queries=_string_list(raw.get("queries")),
    )


def _validate_windows(config: Config) -> None:
    """尽早校验作息窗口表达式，避免运行到半夜才发现配置错误。"""
    try:
        if config.fetch.quiet_hours:
            parse_window(config.fetch.quiet_hours)
        if config.fetch.active_hours:
            parse_window(config.fetch.active_hours)
    except ScheduleError as exc:
        raise ConfigError(f"invalid quiet/active hours: {exc}") from exc


def resolve_cookies(config: Config) -> dict[str, str]:
    """解析 X 会话 Cookie：优先 cookies_file（twifork 兼容 JSON），否则内联三件套。

    仅做文件/字段级校验，绝不打印 Cookie 值本身。
    """
    cookie_cfg = config.cookies
    if cookie_cfg.cookies_file:
        path = Path(cookie_cfg.cookies_file)
        if not path.exists():
            raise ConfigError(f"cookies_file not found: {path}")
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            raise ConfigError(f"cookies_file {path} unreadable: {exc}") from exc
        if isinstance(data, list):
            # 浏览器导出格式 [{name, value}, ...]
            data = {
                str(item.get("name")): str(item.get("value"))
                for item in data
                if isinstance(item, dict) and item.get("name")
            }
        if not isinstance(data, dict):
            raise ConfigError("cookies_file must contain a JSON object or a list of {name, value}")
        cookies = {str(key): str(value) for key, value in data.items() if value}
    else:
        cookies = {
            key: value
            for key, value in (
                ("auth_token", cookie_cfg.auth_token),
                ("ct0", cookie_cfg.ct0),
                ("twid", cookie_cfg.twid),
            )
            if value
        }
    if not cookies.get("auth_token"):
        raise ConfigError(
            "auth_token cookie missing (mandatory for X session reuse); "
            "fill cookies.auth_token / cookies.cookies_file"
        )
    if not cookies.get("ct0"):
        logger.warning("ct0 cookie missing — some upstream endpoints may answer 403")
    return cookies


def validate_for_runtime(config: Config, cookies: dict[str, str]) -> None:
    """真实采集模式启动前的硬校验；dry-run 不执行。"""
    problems: list[str] = []
    if not config.core.token:
        problems.append("core.token is empty (set NEWS_PANEL_INGEST_TOKEN env or edit config)")
    elif is_placeholder(config.core.token):
        problems.append("core.token still contains a placeholder value")
    if not cookies.get("auth_token"):
        problems.append("no X cookies configured (cookies.auth_token or cookies.cookies_file)")
    elif is_placeholder(cookies["auth_token"]):
        problems.append("cookies.auth_token still contains a placeholder value")
    if not config.accounts and not config.queries:
        problems.append("both accounts and queries are empty — nothing to fetch")
    if problems:
        raise ConfigError("; ".join(problems))


def collect_secrets(config: Config, cookies: dict[str, str]) -> set[str]:
    """收集所有需要脱敏的密钥值（过滤过短值避免误伤普通文本）。"""
    values = set(cookies.values()) | {config.core.token, config.planner.api_key}
    return {value for value in values if value and len(value) >= 8}
