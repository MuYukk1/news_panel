"""state.json 增量断点持久化（last_seen_id 游标、原子写入、缓存与清理）。"""

import json

from x_twifork.state import StateStore, id_greater


def test_first_run_seeds_only_recent_n(tmp_path):
    store = StateStore(tmp_path / "state.json", first_run_max_items=3)
    ids_newest_first = ["104", "103", "102", "101", "100"]
    assert store.select_new_ids("account:@a", ids_newest_first) == ["104", "103", "102"]


def test_incremental_after_save_and_reload(tmp_path):
    path = tmp_path / "state.json"
    store = StateStore(path, first_run_max_items=3)
    assert store.select_new_ids("account:@a", ["104", "103", "102", "101", "100"]) == ["104", "103", "102"]
    store.advance_cursor("account:@a", "104")
    store.save()

    reloaded = StateStore(path, first_run_max_items=3)
    assert reloaded.get_cursor("account:@a").last_seen_id == "104"
    assert reloaded.select_new_ids("account:@a", ["106", "105", "104", "103"]) == ["106", "105"]


def test_no_replay_of_older_tweets(tmp_path):
    store = StateStore(tmp_path / "state.json", first_run_max_items=5)
    store.advance_cursor("account:@a", "150")
    store.save()
    reloaded = StateStore(tmp_path / "state.json")
    assert reloaded.select_new_ids("account:@a", ["150", "149", "140"]) == []


def test_cursor_isolated_between_origins(tmp_path):
    store = StateStore(tmp_path / "state.json")
    store.advance_cursor("account:@a", "200")
    # 另一来源（query）无游标：首轮种子策略返回全部
    assert store.select_new_ids("query:AI", ["300", "200", "100"]) == ["300", "200", "100"]
    # 账号来源只取更新的
    assert store.select_new_ids("account:@a", ["201", "200"]) == ["201"]


def test_corrupt_state_file_starts_fresh(tmp_path):
    path = tmp_path / "state.json"
    path.write_text("{{{not json", encoding="utf-8")
    store = StateStore(path)
    assert store.get_cursor("any") is None
    store.advance_cursor("any", "9")
    store.save()
    assert json.loads(path.read_text(encoding="utf-8"))["cursors"]["any"]["last_seen_id"] == "9"


def test_atomic_save_leaves_no_tmp_file(tmp_path):
    path = tmp_path / "state.json"
    store = StateStore(path)
    store.advance_cursor("account:@a", "42")
    store.save()
    assert path.exists()
    assert not (tmp_path / "state.json.tmp").exists()
    data = json.loads(path.read_text(encoding="utf-8"))
    assert data["version"] == 1


def test_user_id_cache_roundtrip(tmp_path):
    path = tmp_path / "state.json"
    store = StateStore(path)
    store.set_user_id("openai", "12345")
    store.save()
    reloaded = StateStore(path)
    assert reloaded.get_user_id("openai") == "12345"


def test_stale_cursors_pruned_on_save(tmp_path):
    class FakeClock:
        def __init__(self) -> None:
            self.now = 1000.0

        def __call__(self) -> float:
            return self.now

    clock = FakeClock()
    path = tmp_path / "state.json"
    store = StateStore(path, clock=clock)
    store.advance_cursor("old", "1")        # updated_at = 1000
    clock.now += 40 * 24 * 3600             # 前进 40 天（> 30 天 TTL）
    store.advance_cursor("new", "2")
    store.save()
    data = json.loads(path.read_text(encoding="utf-8"))
    assert "old" not in data["cursors"]
    assert "new" in data["cursors"]


def test_id_greater_numeric_awareness():
    assert id_greater("200", "100")
    assert not id_greater("100", "200")
    assert not id_greater("100", "100")
    # 超过 float 精度的 19 位雪花 ID 也能正确比较（按 int 而非字符串）
    assert id_greater("1900000000000000107", "1900000000000000107"[:-1] + "6")
    assert id_greater("b", "a")  # 非数字回退字符串比较
