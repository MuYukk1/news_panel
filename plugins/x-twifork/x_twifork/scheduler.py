"""低频拟人化调度：随机抖动、作息静默窗口（quiet/active hours）与失败退避。

所有函数均为纯逻辑（时间与随机源可注入），便于单元测试与复用。
调度目标（DEC-005）：降低深夜高频机械请求特征，优先抗 Cloudflare 拦截与低封号风控。
"""

from __future__ import annotations

import logging
import random
from datetime import datetime, time, timezone, tzinfo
from typing import Callable

from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

logger = logging.getLogger(__name__)

RngFunction = Callable[[float], float]


class ScheduleError(ValueError):
    """作息窗口表达式非法时抛出。"""


def parse_hhmm(text: str) -> time:
    parts = str(text).strip().split(":")
    if len(parts) != 2:
        raise ScheduleError(f"invalid HH:MM value: {text!r}")
    try:
        hour, minute = int(parts[0]), int(parts[1])
    except ValueError as exc:
        raise ScheduleError(f"invalid HH:MM value: {text!r}") from exc
    if not (0 <= hour <= 23 and 0 <= minute <= 59):
        raise ScheduleError(f"HH:MM out of range: {text!r}")
    return time(hour, minute)


def parse_window(text: str) -> tuple[time, time]:
    """解析 "HH:MM-HH:MM" 作息窗口（允许跨午夜，如 "23:30-08:00"）。"""
    text = str(text or "").strip()
    if not text or "-" not in text:
        raise ScheduleError(f"invalid window {text!r}, expected 'HH:MM-HH:MM'")
    start_text, end_text = text.split("-", 1)
    return parse_hhmm(start_text), parse_hhmm(end_text)


def in_window(now: time, start: time, end: time) -> bool:
    """判断时刻是否落在 [start, end] 窗口内；窗口允许跨午夜（start > end）。"""
    if start == end:
        return False
    if start < end:
        return start <= now <= end
    return now >= start or now <= end


def get_tz(name: str) -> tzinfo:
    """按名称取时区；'UTC' 永远可用，未知名称或缺失 tzdata 时安全回退 UTC。"""
    name = str(name or "").strip() or "UTC"
    if name.upper() in ("UTC", "Z", "GMT"):
        return timezone.utc
    try:
        return ZoneInfo(name)
    except (ZoneInfoNotFoundError, ValueError, OSError, KeyError) as exc:
        logger.warning("unknown timezone %r (%s); falling back to UTC", name, exc)
        return timezone.utc


def is_quiet_now(now: datetime, quiet_hours: str = "", active_hours: str = "") -> bool:
    """当前时刻是否处于低频静默窗口（quiet 优先于 active）。"""
    quiet_hours = str(quiet_hours or "").strip()
    active_hours = str(active_hours or "").strip()
    if quiet_hours:
        start, end = parse_window(quiet_hours)
        return in_window(now.time(), start, end)
    if active_hours:
        start, end = parse_window(active_hours)
        return not in_window(now.time(), start, end)
    return False


def compute_delay(
    base_seconds: float,
    jitter_seconds: float,
    quiet: bool,
    *,
    quiet_multiplier: float = 3.0,
    max_seconds: float = 7200.0,
    rng: RngFunction | None = None,
) -> float:
    """计算下一轮等待：基础间隔 + 随机抖动；静默窗口内放大倍数；夹在 [1, max]。"""
    rng = rng or (lambda bound: random.uniform(0.0, bound))
    delay = max(0.0, float(base_seconds)) + rng(max(0.0, float(jitter_seconds)))
    if quiet:
        delay *= max(1.0, float(quiet_multiplier))
    return max(1.0, min(delay, float(max_seconds)))


class Backoff:
    """失败退避：限流/瞬时错误走指数退避，Cookie 失效类走更长线性退避。"""

    def __init__(
        self,
        initial_seconds: float = 60.0,
        max_seconds: float = 3600.0,
        multiplier: float = 2.0,
        auth_backoff_seconds: float = 900.0,
        auth_error_threshold: int = 3,
    ) -> None:
        self._initial = max(1.0, float(initial_seconds))
        self._max = max(self._initial, float(max_seconds))
        self._multiplier = max(1.0, float(multiplier))
        self._auth_base = max(1.0, float(auth_backoff_seconds))
        self._auth_threshold = max(1, int(auth_error_threshold))
        self._current = self._initial
        self._auth_failures = 0

    def record_failure(self, kind: str) -> float:
        """记录一次失败并返回建议的额外等待秒数。"""
        if kind == "auth":
            self._auth_failures += 1
            delay = self._auth_base * min(self._auth_failures, 4)
            return min(delay, self._max * 4)
        self._current = min(self._current * self._multiplier, self._max)
        return self._current

    def record_success(self) -> None:
        self._current = self._initial
        self._auth_failures = 0

    @property
    def current(self) -> float:
        return self._current

    @property
    def auth_failures(self) -> int:
        return self._auth_failures

    @property
    def auth_error_threshold(self) -> int:
        return self._auth_threshold

    def auth_exceeded(self) -> bool:
        """连续 auth 失败是否已达阈值（达到后心跳应上报 error）。"""
        return self._auth_failures >= self._auth_threshold
