"""增量断点持久化（state.json）。

按跟踪账号 / 查询维度记录 ``last_seen_id`` 游标，容器重启后仅拉取新推文，
避免重复刷接口（DEC-005 风控约束）。写入采用“临时文件 + os.replace”原子替换。
"""

from __future__ import annotations

import json
import logging
import os
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

logger = logging.getLogger(__name__)

STATE_VERSION = 1
# 超过该时长未更新的游标在保存时清理，防止 planner 频繁换词导致无限增长
CURSOR_TTL_SECONDS = 30 * 24 * 3600


def id_sort_key(raw: str) -> tuple[int, int, str]:
    """推文 ID 排序键：雪花 ID 按数值比较；非数字 ID 回退字符串比较。"""
    try:
        return (1, int(raw), "")
    except (TypeError, ValueError):
        return (0, 0, str(raw))


def id_greater(a: str, b: str) -> bool:
    """推文 ID ``a`` 是否比 ``b`` 更新（数值感知）。"""
    return id_sort_key(a) > id_sort_key(b)


@dataclass
class Cursor:
    last_seen_id: str
    updated_at: float


class StateStore:
    """state.json 读写与增量选择逻辑。"""

    def __init__(
        self,
        path: str | os.PathLike,
        *,
        first_run_max_items: int = 20,
        clock: Callable[[], float] = time.time,
    ) -> None:
        self._path = Path(path)
        self._first_run_max = max(1, int(first_run_max_items))
        self._clock = clock
        self._cursors: dict[str, Cursor] = {}
        self._user_ids: dict[str, str] = {}
        self._load()

    # ---------- 持久化 ----------
    def _load(self) -> None:
        if not self._path.exists():
            return
        try:
            data = json.loads(self._path.read_text(encoding="utf-8"))
            if not isinstance(data, dict):
                raise ValueError("state root must be a JSON object")
            cursors = data.get("cursors", {})
            user_ids = data.get("user_id_cache", {})
            if not isinstance(cursors, dict) or not isinstance(user_ids, dict):
                raise ValueError("state sections must be objects")
            for key, cursor in cursors.items():
                if isinstance(cursor, dict) and "last_seen_id" in cursor:
                    self._cursors[str(key)] = Cursor(
                        str(cursor["last_seen_id"]), float(cursor.get("updated_at", 0.0))
                    )
            self._user_ids = {str(k): str(v) for k, v in user_ids.items()}
        except (ValueError, OSError, json.JSONDecodeError) as exc:
            logger.warning("state file %s unreadable (%s); starting with fresh state", self._path, exc)
            self._cursors, self._user_ids = {}, {}

    def save(self) -> None:
        """原子写入 state.json（临时文件 + os.replace）。"""
        self._prune()
        payload = {
            "version": STATE_VERSION,
            "cursors": {
                key: {"last_seen_id": cursor.last_seen_id, "updated_at": cursor.updated_at}
                for key, cursor in self._cursors.items()
            },
            "user_id_cache": dict(self._user_ids),
        }
        self._path.parent.mkdir(parents=True, exist_ok=True)
        tmp = self._path.with_name(self._path.name + ".tmp")
        tmp.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True), encoding="utf-8")
        os.replace(tmp, self._path)

    def _prune(self) -> None:
        now = self._clock()
        stale = [key for key, cursor in self._cursors.items() if now - cursor.updated_at > CURSOR_TTL_SECONDS]
        for key in stale:
            self._cursors.pop(key, None)
            logger.info("pruned stale cursor %s", key)

    # ---------- 增量游标 ----------
    def get_cursor(self, key: str) -> Cursor | None:
        return self._cursors.get(key)

    def select_new_ids(self, key: str, ids_newest_first: list[str]) -> list[str]:
        """返回该来源本轮应采集的推文 ID（入参须为新→旧排列）。

        - 首次运行（无游标）：仅回灌最近 ``first_run_max_items`` 条（种子策略，
          避免首启把历史时间线全部灌入面板）；
        - 已有游标：仅返回比 ``last_seen_id`` 更新的 ID。
        """
        cursor = self._cursors.get(key)
        if cursor is None:
            return list(ids_newest_first[: self._first_run_max])
        return [item for item in ids_newest_first if id_greater(item, cursor.last_seen_id)]

    def advance_cursor(self, key: str, newest_seen_id: str) -> None:
        current = self._cursors.get(key)
        if current is None or id_greater(newest_seen_id, current.last_seen_id):
            self._cursors[key] = Cursor(str(newest_seen_id), self._clock())

    # ---------- handle → user_id 解析缓存 ----------
    def get_user_id(self, handle: str) -> str | None:
        return self._user_ids.get(handle)

    def set_user_id(self, handle: str, user_id: str) -> None:
        self._user_ids[str(handle)] = str(user_id)
