"""CLI 入口：``python -m x_twifork [--config ...] [--dry-run] [--once] ...``"""

from __future__ import annotations

import argparse
import asyncio
import logging
import sys
from pathlib import Path

from . import PLUGIN_NAME
from .app import PluginApp
from .config import (
    Config,
    ConfigError,
    collect_secrets,
    load_config,
    resolve_cookies,
    validate_for_runtime,
)
from .fetcher import TwiforkFetcher
from .planner import AIQueryPlanner, QueryPlan
from .poster import CorePoster, derive_urls
from .state import StateStore

logger = logging.getLogger(PLUGIN_NAME)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog=PLUGIN_NAME,
        description=(
            "news_panel 外置 X (Twitter) 纯 HTTP 采集插件"
            "（twifork[impersonate]，无无头浏览器，仅 Cookie 会话复用）"
        ),
    )
    parser.add_argument(
        "--config", default="config.yaml",
        help="配置文件路径（默认 ./config.yaml；--dry-run 下允许缺失）",
    )
    parser.add_argument(
        "--dry-run", action="store_true",
        help="不连接 X 与核心服务：用内置样例推文生成合规 payload，本地打印或推送到 --ingest-url",
    )
    parser.add_argument(
        "--once", action="store_true",
        help="仅执行一轮采集后退出（默认常驻循环）",
    )
    parser.add_argument(
        "--ingest-url", default=None,
        help="覆盖 ingest 上报地址（如本地 mock: http://127.0.0.1:8099/api/v1/ingest）",
    )
    parser.add_argument(
        "--heartbeat-url", default=None,
        help="覆盖 heartbeat 上报地址（缺省由 ingest-url 同源推导）",
    )
    parser.add_argument(
        "--state-file", default=None,
        help="覆盖 state.json 增量断点文件路径",
    )
    parser.add_argument(
        "--log-level", default=None, choices=["DEBUG", "INFO", "WARNING", "ERROR"],
        help="覆盖日志级别",
    )
    return parser


def _setup_logging(level: str) -> None:
    numeric = getattr(logging, str(level).upper(), logging.INFO)
    logging.basicConfig(
        level=numeric,
        format="%(asctime)s %(levelname)-7s %(name)s: %(message)s",
        force=True,
    )


def _load_config(path_str: str, dry_run: bool) -> Config:
    path = Path(path_str)
    if not path.exists():
        if dry_run:
            logger.warning("config file %s not found — using built-in defaults (dry-run mode)", path)
            return load_config(None)
        raise ConfigError(
            f"config file {path} not found; copy config.example.yaml to config.yaml first"
        )
    return load_config(path)


async def _run_app(app: PluginApp, poster: CorePoster | None, once: bool) -> int:
    try:
        if once:
            result = await app.run_cycle()
            return 1 if result.status == "error" else 0
        await app.run_forever()
        return 0
    finally:
        if poster is not None:
            await poster.aclose()


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    dry_run = bool(args.dry_run)
    _setup_logging(args.log_level or "INFO")

    try:
        config = _load_config(args.config, dry_run)
    except ConfigError as exc:
        logger.error("%s", exc)
        return 2
    if args.log_level is None:
        _setup_logging(config.log_level)

    if args.state_file:
        config.state_file = args.state_file

    cookies: dict[str, str] = {}
    secrets: set[str] = set()
    if dry_run:
        secrets = collect_secrets(config, cookies)
    else:
        try:
            cookies = resolve_cookies(config)
            validate_for_runtime(config, cookies)
        except ConfigError as exc:
            logger.error("live mode config invalid: %s", exc)
            return 2
        secrets = collect_secrets(config, cookies)

    state = StateStore(config.state_file, first_run_max_items=config.fetch.first_run_max_items)
    ingest_url, heartbeat_url = derive_urls(config.core.base_url, args.ingest_url, args.heartbeat_url)

    fetcher: TwiforkFetcher | None = None
    planner: AIQueryPlanner | None = None
    poster: CorePoster | None = None

    if dry_run:
        if args.ingest_url:
            poster = CorePoster(
                ingest_url,
                heartbeat_url,
                token=config.core.token or "dry-run-placeholder",
                timeout_seconds=config.core.timeout_seconds,
                max_retries=config.core.ingest_max_retries,
                secrets=secrets,
            )
        app = PluginApp(config, state=state, poster=poster, dry_run=True, secrets=secrets)
    else:
        fetcher = TwiforkFetcher(
            cookies,
            proxy=config.proxy,
            impersonate=config.impersonate,
            max_tweets=config.fetch.max_tweets_per_query,
            search_product=config.fetch.search_product,
            include_retweets=config.fetch.include_retweets,
        )
        if config.planner.enabled:
            static_plan = QueryPlan(accounts=list(config.accounts), queries=list(config.queries))
            planner = AIQueryPlanner(config.planner, static_plan, secrets=secrets)
        poster = CorePoster(
            ingest_url,
            heartbeat_url,
            token=config.core.token,
            timeout_seconds=config.core.timeout_seconds,
            max_retries=config.core.ingest_max_retries,
            secrets=secrets,
        )
        app = PluginApp(
            config,
            state=state,
            fetcher=fetcher,
            planner=planner,
            poster=poster,
            dry_run=False,
            secrets=secrets,
        )

    try:
        return asyncio.run(_run_app(app, poster, once=bool(args.once) or dry_run))
    except KeyboardInterrupt:
        logger.info("interrupted by user; bye")
        return 0
    except RuntimeError as exc:  # 环境级致命错误（如 twifork 未安装）
        logger.error("%s", exc)
        return 3


if __name__ == "__main__":
    sys.exit(main())
