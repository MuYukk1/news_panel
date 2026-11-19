"""采集主循环编排：planner → fetch → transform → 增量选择 → ingest → heartbeat。

职责边界：
- 上游常规拉取失败永不打断主循环：按 ``FetchError.kind`` 记入退避并在心跳中如实上报；
- 命中账号锁定/封禁（``AccountLocked`` / ``AccountSuspended``）时：单轮立即中断剩余来源、上报明确 ``status=error`` 诊断心跳，常驻模式进入静默挂起等待人工解锁并重启容器（避免在 ``restart: unless-stopped`` 下无限重启循环），``--once`` 模式返回退出码 1；
- ingest 投递失败时游标不推进，下一轮自然重发（核心按 (source_id, external_id) 幂等去重）；
- dry-run 模式不写 state.json、不触碰真实上游，输出/推送完全合规的契约 payload。
"""

from __future__ import annotations

import asyncio
import json
import logging
import random
import time
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

from . import PLUGIN_NAME, __version__
from .config import Config
from .fetcher import FATAL_ACCOUNT_KINDS, FetchError
from .planner import AIQueryPlanner, QueryPlan
from .redact import redact_text
from .samples import SAMPLE_TWEETS
from .scheduler import Backoff, compute_delay, get_tz, is_quiet_now
from .state import StateStore, id_sort_key
from .transform import (
    TweetLike,
    build_heartbeat_payload,
    build_ingest_payload,
    chunk_items,
    tweet_to_item,
    validate_ingest_payload,
    validate_heartbeat_payload,
)

logger = logging.getLogger(__name__)

SAMPLE_ORIGIN_KEY = "account:@x-twifork-sample"
SAMPLE_ORIGIN_LABEL = "built-in sample feed"
MAX_ERROR_MESSAGE_CHARS = 500


@dataclass
class CycleResult:
    """单轮采集结果摘要（供 CLI 退出码与日志使用）。"""

    status: str = "ok"
    fetched_count: int = 0
    error_message: str | None = None
    failure_delay: float = 0.0
    failures: list[tuple[str, str]] = field(default_factory=list)  # (origin_key, kind)
    halt_reason: str | None = None  # 命中账号锁定/封禁时置位，触发常驻静默熔断


class PluginApp:
    """组装各组件并驱动「拉取 - 选择 - 投递 - 心跳」主循环（支持 dry-run 注入）。"""

    def __init__(
        self,
        config: Config,
        *,
        state: StateStore,
        fetcher: Any | None = None,
        planner: AIQueryPlanner | None = None,
        poster: Any | None = None,
        dry_run: bool = False,
        secrets: set[str] | None = None,
        clock: Any = time.time,
        rng: Any = random.random,
        sample_tweets: list[TweetLike] | None = None,
        stop_event: asyncio.Event | None = None,
    ) -> None:
        self._config = config
        self._state = state
        self._fetcher = fetcher
        self._planner = planner
        self._poster = poster
        self._dry_run = bool(dry_run)
        self._secrets = secrets or set()
        self._clock = clock
        self._rng = rng
        self._stop_event = stop_event
        self._backoff = Backoff(
            config.backoff.initial_seconds,
            config.backoff.max_seconds,
            config.backoff.multiplier,
            config.backoff.auth_backoff_seconds,
            config.backoff.auth_error_threshold,
        )
        self._plan = QueryPlan(accounts=list(config.accounts), queries=list(config.queries))
        self._sample_tweets = SAMPLE_TWEETS if sample_tweets is None else sample_tweets
        if not self._dry_run and (self._fetcher is None or self._poster is None):
            raise ValueError("live mode requires both fetcher and poster")

    # ---------- 主流程 ----------

    async def run_cycle(self) -> CycleResult:
        result = CycleResult()

        # 1) 可选 AI 查询规划刷新（关闭/失败自动回退静态配置，绝不阻断）
        if self._planner is not None and not self._dry_run and self._planner.due(self._clock()):
            self._plan = await self._planner.refresh(self._clock())
            if self._plan.note:
                logger.info("planner note: %s", self._plan.note)

        # 2) 上游拉取（账号时间线 + 关键词搜索）
        grouped = await self._collect_tweets(result)

        # 3) 跨源去重（账号优先于查询）→ 新→旧排序 → 增量选择
        selected: list[dict] = []
        newest_by_origin: dict[str, str] = {}
        seen_ids: set[str] = set()
        for origin_key, group in grouped.items():
            tweets: list[TweetLike] = []
            for tweet in group["tweets"]:
                if tweet.id in seen_ids:
                    continue
                seen_ids.add(tweet.id)
                tweets.append(tweet)
            tweets.sort(key=lambda t: id_sort_key(t.id), reverse=True)
            ids_desc = [tweet.id for tweet in tweets]
            chosen = set(self._state.select_new_ids(origin_key, ids_desc))
            group["selected"] = [
                tweet_to_item(tweet, origin_key=origin_key, origin_label=group["label"])
                for tweet in tweets
                if tweet.id in chosen
            ]
            selected.extend(group["selected"])
            if ids_desc:
                newest_by_origin[origin_key] = ids_desc[0]
        result.fetched_count = len(selected)

        # 4) 构造契约 payload（分批 ≤100）并投递
        batches = chunk_items(selected, self._config.fetch.max_items_per_batch) if selected else []
        batch_payloads = [
            build_ingest_payload(self._config.source_id, self._config.source_type, batch)
            for batch in batches
        ]
        payload_errors: list[str] = []
        for payload in batch_payloads:
            payload_errors.extend(validate_ingest_payload(payload))

        ingest_ok = True
        ingest_error = ""
        fetch_successes = len(grouped)  # grouped 仅收录成功来源；失败已记入 result.failures

        if payload_errors:
            ingest_ok = False
            ingest_error = "ingest payload failed contract validation: " + "; ".join(payload_errors[:5])
            logger.error("%s", ingest_error)
        elif not self._dry_run:
            for payload in batch_payloads:
                try:
                    await self._poster.send_ingest(payload)
                except Exception as exc:  # noqa: BLE001 —— 投递失败：游标不推进，下轮重发（核心幂等）
                    ingest_ok = False
                    ingest_error = redact_text(f"ingest delivery failed: {type(exc).__name__}: {exc}", self._secrets)
                    logger.warning("%s", ingest_error)
                    break

        # 5) 游标推进与断点持久化（dry-run 不落盘，保持可重复执行）
        if ingest_ok and not self._dry_run:
            for origin_key, newest_id in newest_by_origin.items():
                self._state.advance_cursor(origin_key, newest_id)
            try:
                self._state.save()
            except OSError as exc:
                logger.warning("state save failed: %s", exc)

        # 6) 状态判定（heartbeat 契约枚举：ok / degraded / error）
        self._finalize_status(result, ingest_ok, payload_errors, fetch_successes, ingest_error)

        heartbeat_payload = build_heartbeat_payload(
            source_id=self._config.source_id,
            plugin_version=__version__,
            status=result.status,
            fetched_count=result.fetched_count,
            error_message=result.error_message,
        )
        heartbeat_errors = validate_heartbeat_payload(heartbeat_payload)
        if heartbeat_errors:  # 防御性：正常不应发生
            logger.error("heartbeat payload failed validation: %s", heartbeat_errors)

        # 7) dry-run 输出 / 正式心跳上报
        if self._dry_run:
            self._print_dry_run(batch_payloads, heartbeat_payload, payload_errors + heartbeat_errors)
            if self._poster is not None and not payload_errors and not heartbeat_errors:
                for payload in batch_payloads:
                    try:
                        await self._poster.send_ingest(payload)
                    except Exception as exc:  # noqa: BLE001
                        logger.warning("dry-run ingest push failed: %s", redact_text(str(exc), self._secrets))
                try:
                    await self._poster.send_heartbeat(heartbeat_payload)
                except Exception as exc:  # noqa: BLE001
                    logger.warning("dry-run heartbeat push failed: %s", redact_text(str(exc), self._secrets))
        else:
            try:
                await self._poster.send_heartbeat(heartbeat_payload)
            except Exception as exc:  # noqa: BLE001 —— 心跳异常不影响主循环
                logger.warning("heartbeat crashed unexpectedly: %s", redact_text(str(exc), self._secrets))

        return result

    async def run_forever(self) -> None:
        """常驻循环：每轮结束按「作息窗口 + 抖动 + 失败退避」计算下一次等待。"""
        if self._dry_run:
            await self.run_cycle()
            return
        config = self._config
        tz = get_tz(config.fetch.timezone)
        while True:
            result = await self.run_cycle()
            if result.halt_reason:
                logger.error(
                    "%s circuit-breaker engaged (%s): %s; staying idle without further X requests "
                    "until manual unlock and container restart",
                    PLUGIN_NAME,
                    result.halt_reason,
                    result.error_message or "manual intervention required",
                )
                await self._wait_until_stopped()
                return
            if not result.failures:
                self._backoff.record_success()
            quiet = is_quiet_now(datetime.now(tz), config.fetch.quiet_hours, config.fetch.active_hours)
            delay = compute_delay(
                config.fetch.poll_interval_seconds,
                config.fetch.jitter_seconds,
                quiet,
                quiet_multiplier=config.fetch.quiet_hours_multiplier,
                max_seconds=config.fetch.max_delay_seconds,
                rng=lambda bound: bound * float(self._rng()),
            )
            delay = max(delay, result.failure_delay)
            logger.info(
                "%s cycle done: status=%s fetched=%d failures=%d; next poll in %.0fs%s",
                PLUGIN_NAME, result.status, result.fetched_count, len(result.failures), delay,
                " (quiet hours)" if quiet else "",
            )
            await asyncio.sleep(delay)

    async def _wait_until_stopped(self) -> None:
        """静默挂起等待外部停止/重启信号，避免 Docker restart: unless-stopped 陷入重启循环。"""
        event = self._stop_event if self._stop_event is not None else asyncio.Event()
        await event.wait()

    # ---------- 内部组件 ----------

    async def _collect_tweets(self, result: CycleResult) -> dict[str, dict]:
        """按当前计划逐源拉取；普通单源失败只记录，账号锁定/封禁立即熔断本轮。"""
        grouped: dict[str, dict] = {}
        if self._dry_run:
            grouped[SAMPLE_ORIGIN_KEY] = {"label": SAMPLE_ORIGIN_LABEL, "tweets": list(self._sample_tweets)}
            return grouped

        sources: list[tuple[str, str]] = [("account", ref) for ref in self._plan.accounts]
        sources += [("query", ref) for ref in self._plan.queries]
        if not sources:
            result.failures.append(("plan", "config"))
            logger.error("no accounts or queries configured — nothing to fetch")
            return grouped

        for index, (kind, ref) in enumerate(sources):
            if index:
                await self._politeness_sleep()
            origin_key = f"{kind}:{ref}"
            try:
                if kind == "account":
                    tweets = await self._fetch_account(ref)
                else:
                    tweets = await self._fetcher.fetch_search(ref)
            except FetchError as exc:
                if exc.kind in FATAL_ACCOUNT_KINDS:
                    result.failures.append((origin_key, exc.kind))
                    result.halt_reason = exc.kind
                    logger.error(
                        "fetch %s hit fatal account status (%s): %s — aborting remaining sources",
                        origin_key,
                        exc.kind,
                        redact_text(str(exc), self._secrets),
                    )
                    break
                delay = self._backoff.record_failure(exc.kind)
                result.failure_delay = max(result.failure_delay, delay)
                result.failures.append((origin_key, exc.kind))
                logger.warning(
                    "fetch %s failed (%s): %s", origin_key, exc.kind, redact_text(str(exc), self._secrets)
                )
                continue
            except RuntimeError:
                raise  # 环境级致命错误（如 twifork 未安装）：终止而非退避重试
            except Exception as exc:  # noqa: BLE001 —— 未预期异常一律降级，绝不打断主循环
                delay = self._backoff.record_failure("transient")
                result.failure_delay = max(result.failure_delay, delay)
                result.failures.append((origin_key, "transient"))
                logger.warning(
                    "fetch %s crashed unexpectedly: %s: %s",
                    origin_key, type(exc).__name__, redact_text(str(exc), self._secrets),
                )
                continue
            label = f"@{ref.lstrip('@')} timeline" if kind == "account" else f"search: {ref}"
            grouped[origin_key] = {"label": label, "tweets": list(tweets)}
        return grouped

    async def _fetch_account(self, ref: str) -> list[TweetLike]:
        """解析并拉取单个账号时间线；handle → user_id 结果缓存在 state.json。"""
        clean = ref.strip().lstrip("@")
        if clean.isdigit():
            return await self._fetcher.fetch_user_tweets(clean)
        cached = self._state.get_user_id(clean.lower())
        if cached:
            return await self._fetcher.fetch_user_tweets(cached)
        user_id = await self._fetcher.resolve_user_id(clean)
        self._state.set_user_id(clean.lower(), user_id)
        return await self._fetcher.fetch_user_tweets(user_id)

    async def _politeness_sleep(self) -> None:
        pace = max(0.0, float(self._config.fetch.pace_seconds))
        if pace <= 0:
            return
        factor = 0.7 + 0.6 * float(self._rng())
        await asyncio.sleep(min(pace * factor, 10.0))

    def _finalize_status(
        self,
        result: CycleResult,
        ingest_ok: bool,
        payload_errors: list[str],
        fetch_successes: int,
        ingest_error: str = "",
    ) -> None:
        if result.halt_reason:
            result.status = "error"
        elif result.failures and fetch_successes == 0:
            result.status = "error"
        elif self._backoff.auth_exceeded():
            result.status = "error"
        elif result.failures or not ingest_ok:
            result.status = "degraded"
        else:
            result.status = "ok"

        messages: list[str] = []
        kinds = {kind for _, kind in result.failures}
        if "account_locked" in kinds:
            messages.append(
                "X account locked (AccountLocked / challenge required) — "
                "halting fetches; unlock account in browser, refresh cookies if needed, and restart container"
            )
        if "account_suspended" in kinds:
            messages.append(
                "X account suspended (AccountSuspended) — "
                "halting fetches; switch backup account/cookies and restart container"
            )
        if "auth" in kinds:
            messages.append("upstream rejected session cookies — refresh auth_token/ct0")
        if "blocked" in kinds:
            messages.append("upstream blocked requests (possible Cloudflare challenge)")
        if "rate_limit" in kinds:
            messages.append("upstream rate limit hit; backing off")
        known_kinds = {"account_locked", "account_suspended", "auth", "blocked", "rate_limit"}
        transient_count = len(result.failures) - sum(
            1 for _, kind in result.failures if kind in known_kinds
        )
        if transient_count:
            messages.append(f"{transient_count} source(s) failed transiently")
        if not ingest_ok and payload_errors:
            messages.append("payload contract validation failed")
        elif not ingest_ok:
            # ingest_error 已在上游经 redact_text 脱敏，可安全进入心跳诊断信息
            messages.append(ingest_error or "ingest delivery failed; will retry next cycle")
        if self._backoff.auth_exceeded():
            messages.append(
                f"consecutive auth failures {self._backoff.auth_failures}/{self._backoff.auth_error_threshold}"
            )
        if messages:
            result.error_message = redact_text("; ".join(messages), self._secrets)[:MAX_ERROR_MESSAGE_CHARS]

    # ---------- dry-run 输出 ----------

    def _print_dry_run(self, batch_payloads: list[dict], heartbeat_payload: dict, errors: list[str]) -> None:
        total = sum(len(payload["items"]) for payload in batch_payloads)
        print(f"--- x-twifork dry-run (source_id={self._config.source_id}, new_items={total}) ---")
        if not batch_payloads:
            print("--- (no new items selected this cycle) ---")
        for index, payload in enumerate(batch_payloads, 1):
            print(f"--- INGEST {index}/{len(batch_payloads)} (items={len(payload['items'])}) ---")
            print(json.dumps(payload, ensure_ascii=False, indent=2))
        print("--- HEARTBEAT ---")
        print(json.dumps(heartbeat_payload, ensure_ascii=False, indent=2))
        if errors:
            print("--- VALIDATION FAILED ---")
            for error in errors:
                print(f"  ! {error}")
        else:
            print("--- VALIDATION OK: payloads conform to PLUGIN_SPEC v0.1 ---")
