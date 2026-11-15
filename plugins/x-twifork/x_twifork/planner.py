"""可选 AI 主动查询规划（OpenAI-compatible）。

DEC-005 要求：AI 规划为可选项，当关闭、未配置 API Key、网络超时或返回异常
格式时，必须无缝降级回静态 ``config.yaml`` 中的账号与关键词列表，
绝不阻断采集主循环 —— 因此 :meth:`AIQueryPlanner.refresh` 保证不抛异常。
"""

from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass, field
from typing import Any

import httpx

from .config import PlannerConfig
from .redact import redact_text

logger = logging.getLogger(__name__)

_FENCE_RE = re.compile(r"```(?:json)?\s*(.*?)```", re.DOTALL)


@dataclass
class QueryPlan:
    """一轮采集的输入计划：accounts + queries（``source`` 标记来源）。"""

    accounts: list[str] = field(default_factory=list)
    queries: list[str] = field(default_factory=list)
    source: str = "static"      # "static" | "ai"
    note: str = ""              # 最近一次规划说明（如回退原因），仅供日志/调试


def _clean_str_list(values: Any, *, max_items: int, lowercase: bool = False) -> list[str]:
    out: list[str] = []
    if not isinstance(values, list):
        return out
    for value in values:
        if not isinstance(value, str):
            continue
        value = value.strip()
        if not value:
            continue
        if lowercase:
            value = value.lstrip("@").lower()
        if value not in out:
            out.append(value)
        if len(out) >= max_items:
            break
    return out


def _merge(base: list[str], extra: list[str]) -> list[str]:
    out = list(base)
    for item in extra:
        if item not in out:
            out.append(item)
    return out


def extract_plan_json(content: str) -> dict:
    """从 LLM 文本中鲁棒地提取 plan JSON（容忍代码围栏与前后缀散文）。"""
    if not isinstance(content, str):
        raise ValueError("content must be a string")
    fenced = [m for m in _FENCE_RE.findall(content) if m.strip()]
    candidates = fenced or [content]
    last_error: Exception | None = None
    for candidate in candidates:
        start, end = candidate.find("{"), candidate.rfind("}")
        if start == -1 or end <= start:
            last_error = ValueError("no JSON object found in planner reply")
            continue
        try:
            data = json.loads(candidate[start : end + 1])
        except json.JSONDecodeError as exc:
            last_error = exc
            continue
        if isinstance(data, dict):
            return data
        last_error = ValueError("planner JSON is not an object")
    raise last_error or ValueError("empty planner reply")


class AIQueryPlanner:
    """调用 OpenAI-compatible /chat/completions 生成/微调查询计划。"""

    def __init__(
        self,
        config: PlannerConfig,
        static_plan: QueryPlan,
        *,
        secrets: set[str] | None = None,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        self._cfg = config
        self._static = static_plan
        self._secrets = secrets or set()
        self._transport = transport
        self._last_run = float("-inf")

    @property
    def static_plan(self) -> QueryPlan:
        return self._static

    def due(self, now: float) -> bool:
        return self._cfg.enabled and (now - self._last_run) >= self._cfg.interval_minutes * 60

    async def refresh(self, now: float) -> QueryPlan:
        """返回最新 QueryPlan；任何失败均回退静态配置，绝不抛异常。"""
        self._last_run = now
        if not self._cfg.enabled:
            return self._fallback("planner disabled")
        if not self._cfg.base_url or not self._cfg.api_key:
            return self._fallback("planner base_url/api_key not configured")

        url = self._cfg.base_url.rstrip("/") + "/chat/completions"
        headers = {"Authorization": f"Bearer {self._cfg.api_key}", "Content-Type": "application/json"}
        body = {
            "model": self._cfg.model,
            "messages": [
                {
                    "role": "system",
                    "content": (
                        "You are a query planner for an X (Twitter) intelligence feed. "
                        "Reply with strict JSON only, no prose."
                    ),
                },
                {"role": "user", "content": self._build_prompt()},
            ],
            "temperature": 0.2,
        }
        try:
            async with httpx.AsyncClient(timeout=self._cfg.timeout_seconds, transport=self._transport) as client:
                response = await client.post(url, json=body, headers=headers)
                response.raise_for_status()
                content = response.json()["choices"][0]["message"]["content"]
            data = extract_plan_json(content)
            queries = _clean_str_list(data.get("queries"), max_items=self._cfg.max_queries)
            accounts = _clean_str_list(data.get("accounts"), max_items=self._cfg.max_accounts, lowercase=True)
            if not queries and not accounts:
                return self._fallback("planner returned an empty plan")
            if self._cfg.merge_with_static:
                queries = _merge(self._static.queries, queries)
                accounts = _merge(self._static.accounts, accounts)
            logger.info(
                "AI planner refreshed plan (source=ai): %d queries, %d accounts",
                len(queries), len(accounts),
            )
            return QueryPlan(accounts=accounts, queries=queries, source="ai")
        except Exception as exc:  # noqa: BLE001 —— 任何异常都必须降级，不得阻断采集
            return self._fallback(
                f"planner failed: {type(exc).__name__}: {redact_text(str(exc), self._secrets)}"
            )

    def _fallback(self, reason: str) -> QueryPlan:
        logger.warning("AI planner falling back to static config: %s", reason)
        return QueryPlan(
            accounts=list(self._static.accounts),
            queries=list(self._static.queries),
            source="static",
            note=reason,
        )

    def _build_prompt(self) -> str:
        topics = "; ".join(self._cfg.topics) or "(no topics configured)"
        return (
            f"Monitoring topics: {topics}.\n"
            f"Current search queries: {self._static.queries or []}\n"
            f"Current tracked accounts: {self._static.accounts or []}\n"
            "Propose refined search queries and high-signal accounts worth tracking. "
            'Reply with JSON only: {"queries": ["..."], "accounts": ["handle1", "handle2"]} '
            f"(max {self._cfg.max_queries} queries, max {self._cfg.max_accounts} accounts, no leading @)."
        )
