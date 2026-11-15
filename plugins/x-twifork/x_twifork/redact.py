"""敏感信息脱敏。

任何可能进入日志、心跳 error_message 或异常详情的文本都应先经过
:func:`redact_text`，确保 Cookie / Token 等凭证不会外泄（PLUGIN_SPEC
明确要求 error_message 严禁包含敏感凭证）。
"""

from __future__ import annotations

import re

REDACTED = "***REDACTED***"

# 形如 auth_token=xxx / ct0: xxx / Bearer xxx / api_key=xxx 的键值对
_KEY_VALUE_RE = re.compile(
    r"(?i)\b(auth[_-]?token|ct0|twid|bearer|api[_-]?key|access[_-]?token|password|cookies?)\b"
    r"(\s*[:=]\s*|\s+)([^\s\"',;}{]{4,})"
)

# 短于该长度的“密钥”不做整串替换，避免误伤普通单词
MIN_SECRET_LEN = 8


def redact_text(text: str, secrets: set[str] | None = None) -> str:
    """按已知密钥集合与通用键值模式双重脱敏。

    1. 整串替换：``secrets`` 中出现过的值（长度 >= 8）全部替换为占位符；
    2. 模式替换：``auth_token=...`` / ``Bearer ...`` 等键值对一律打码。
    """
    if not text:
        return text or ""
    out = str(text)
    for secret in sorted(secrets or set(), key=len, reverse=True):
        if secret and len(secret) >= MIN_SECRET_LEN and secret in out:
            out = out.replace(secret, REDACTED)
    out = _KEY_VALUE_RE.sub(lambda m: f"{m.group(1)}{m.group(2)}{REDACTED}", out)
    return out
