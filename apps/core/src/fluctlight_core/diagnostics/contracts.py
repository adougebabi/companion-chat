"""Typed, redacted diagnostics contracts."""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any


class DiagnosticSeverity(StrEnum):
    DEBUG = "debug"
    INFO = "info"
    WARNING = "warning"
    ERROR = "error"


_SECRET_KEYS = {
    "api_key",
    "authorization",
    "cookie",
    "credential",
    "password",
    "secret",
    "session_token",
    "token",
}
_HIDDEN_KEYS = {"hidden_reasoning", "reasoning", "chain_of_thought", "thoughts"}


def _key_class(key: str) -> str | None:
    normalized = key.strip().lower().replace("-", "_")
    if normalized in _HIDDEN_KEYS or normalized.endswith("_reasoning"):
        return "hidden"
    if normalized in _SECRET_KEYS or any(
        part in normalized for part in ("api_key", "password", "token")
    ):
        return "secret"
    return None


def redact(value: Any, *, max_string_length: int = 12_000) -> Any:
    """Recursively redact secrets and hidden reasoning before persistence/export."""

    if isinstance(value, Mapping):
        result: dict[str, Any] = {}
        for raw_key, raw_value in value.items():
            key = str(raw_key)
            kind = _key_class(key)
            if kind == "hidden":
                continue
            if kind == "secret":
                result[key] = "[REDACTED]"
            else:
                result[key] = redact(raw_value, max_string_length=max_string_length)
        return result
    if isinstance(value, Sequence) and not isinstance(value, str | bytes | bytearray):
        return [redact(item, max_string_length=max_string_length) for item in value]
    if isinstance(value, str):
        return (
            value
            if len(value) <= max_string_length
            else value[:max_string_length] + "...[truncated]"
        )
    if isinstance(value, bytes | bytearray):
        return "[BINARY]"
    return value


@dataclass(frozen=True, slots=True)
class DiagnosticEvent:
    event_type: str
    payload: Mapping[str, Any]
    correlation_id: str
    severity: DiagnosticSeverity = DiagnosticSeverity.INFO
    fluctlight_id: str | None = None
    causation_id: str | None = None
    event_id: str | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        if not self.event_type.strip() or not self.correlation_id.strip():
            raise ValueError("diagnostic event type and correlation_id are required")
        if not isinstance(self.payload, Mapping):
            raise ValueError("diagnostic event payload must be an object")
        object.__setattr__(self, "payload", redact(self.payload))
        object.__setattr__(self, "severity", DiagnosticSeverity(self.severity))
        if self.created_at.tzinfo is None or self.created_at.utcoffset() is None:
            raise ValueError("created_at must be timezone-aware")


@dataclass(frozen=True, slots=True)
class DiagnosticModelRun:
    role: str
    model_id: str
    prompt: Mapping[str, Any]
    correlation_id: str
    status: str
    endpoint_id: str | None = None
    response: Mapping[str, Any] | None = None
    error_code: str | None = None
    run_id: str | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        for name in ("role", "model_id", "correlation_id", "status"):
            if not getattr(self, name).strip():
                raise ValueError(f"{name} is required")
        object.__setattr__(self, "prompt", redact(self.prompt))
        if self.response is not None:
            object.__setattr__(self, "response", redact(self.response))


@dataclass(frozen=True, slots=True)
class DiagnosticTurn:
    fluctlight_id: str
    correlation_id: str
    status: str
    source_event_id: str | None = None
    conversation_id: str | None = None
    turn_id: str | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class DiagnosticWorkflowLink:
    correlation_id: str
    workflow_id: str
    intent_id: str | None = None
    event_id: str | None = None
    link_id: str | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
