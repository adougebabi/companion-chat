"""Canonical IANA timezone handling for persisted local-time facts."""

from __future__ import annotations

from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

_ALIASES = {
    "UTC+8": "Asia/Shanghai",
    "UTC+08:00": "Asia/Shanghai",
    "GMT+8": "Asia/Shanghai",
    "GMT+08:00": "Asia/Shanghai",
    "China Standard Time": "Asia/Shanghai",
}


def canonical_timezone(value: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError("timezone must be an IANA timezone")
    normalized = _ALIASES.get(value.strip(), value.strip())
    try:
        ZoneInfo(normalized)
    except ZoneInfoNotFoundError as exc:
        raise ValueError("timezone must be an IANA timezone") from exc
    return normalized
