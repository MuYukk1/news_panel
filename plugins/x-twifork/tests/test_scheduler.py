"""抖动、作息窗口与失败退避调度逻辑。"""

from datetime import datetime, time, timezone

from x_twifork.scheduler import (
    Backoff,
    ScheduleError,
    compute_delay,
    get_tz,
    in_window,
    is_quiet_now,
    parse_window,
)


def test_jitter_within_bounds():
    assert compute_delay(900, 100, quiet=False, rng=lambda bound: bound) == 1000
    assert compute_delay(900, 100, quiet=False, rng=lambda bound: 0.0) == 900


def test_quiet_hours_multiplier_and_cap():
    delay = compute_delay(900, 100, quiet=True, quiet_multiplier=3.0, max_seconds=7200, rng=lambda b: b)
    assert delay == 3000
    capped = compute_delay(900, 100, quiet=True, quiet_multiplier=10.0, max_seconds=5000, rng=lambda b: b)
    assert capped == 5000


def test_delay_minimum_is_one_second():
    assert compute_delay(0, 0, quiet=False) == 1.0


def test_parse_window_normal_and_overnight():
    assert parse_window("08:00-23:30") == (time(8, 0), time(23, 30))
    assert parse_window("23:30-08:00") == (time(23, 30), time(8, 0))


def test_parse_window_rejects_garbage():
    for bad in ("", "9am", "25:00-08:00", "08:00"):
        try:
            parse_window(bad)
            raise AssertionError(f"expected ScheduleError for {bad!r}")
        except ScheduleError:
            pass


def test_in_window_overnight_span():
    start, end = time(23, 30), time(8, 0)
    assert in_window(time(1, 0), start, end)
    assert in_window(time(23, 30), start, end)
    assert in_window(time(8, 0), start, end)
    assert not in_window(time(12, 0), start, end)


def test_in_window_normal_span():
    start, end = time(8, 0), time(23, 30)
    assert in_window(time(9, 0), start, end)
    assert not in_window(time(2, 0), start, end)


def test_is_quiet_with_quiet_hours():
    assert is_quiet_now(datetime(2026, 11, 15, 3, 0, tzinfo=timezone.utc), quiet_hours="01:00-08:30")
    assert not is_quiet_now(datetime(2026, 11, 15, 12, 0, tzinfo=timezone.utc), quiet_hours="01:00-08:30")


def test_is_quiet_with_active_hours():
    assert is_quiet_now(datetime(2026, 11, 15, 22, 0, tzinfo=timezone.utc), active_hours="08:00-21:00")
    assert not is_quiet_now(datetime(2026, 11, 15, 12, 0, tzinfo=timezone.utc), active_hours="08:00-21:00")


def test_is_quiet_defaults_false_without_windows():
    assert not is_quiet_now(datetime(2026, 11, 15, 3, 0, tzinfo=timezone.utc))


def test_get_tz_utc_without_tzdata_dependency():
    assert get_tz("UTC") is timezone.utc
    assert get_tz("") is timezone.utc


def test_backoff_exponential_escalation_and_reset():
    backoff = Backoff(initial_seconds=60, max_seconds=3600, multiplier=2.0)
    assert backoff.record_failure("rate_limit") == 120
    assert backoff.record_failure("rate_limit") == 240
    backoff.record_success()
    assert backoff.current == 60
    assert backoff.auth_failures == 0


def test_backoff_auth_escalates_longer_and_threshold():
    backoff = Backoff(
        initial_seconds=60, max_seconds=3600, multiplier=2.0,
        auth_backoff_seconds=900, auth_error_threshold=3,
    )
    delays = [backoff.record_failure("auth") for _ in range(3)]
    assert delays == [900, 1800, 2700]
    assert backoff.auth_exceeded()
    assert backoff.record_failure("rate_limit") == 120  # 两条退避通道互不影响
    backoff.record_success()
    assert not backoff.auth_exceeded()


def test_backoff_rate_limit_capped():
    backoff = Backoff(initial_seconds=1000, max_seconds=1500, multiplier=2.0)
    assert backoff.record_failure("rate_limit") == 1500
    assert backoff.record_failure("rate_limit") == 1500
