"""Structured stdout diagnostics for the T01 correlation gate."""

from __future__ import annotations

import json
import sys
import time
from collections.abc import Iterable
from dataclasses import dataclass
from typing import Any, TextIO


@dataclass(frozen=True, slots=True)
class DiagnosticRecord:
    event: str
    intent_id: str
    workflow_id: str
    step_id: str | None = None
    provider_request_id: str | None = None
    attempt: int | None = None
    correlation_id: str | None = None
    result_id: str | None = None
    details: dict[str, Any] | None = None
    timestamp: float = 0.0

    def as_dict(self) -> dict[str, Any]:
        value = {
            "event": self.event,
            "intent_id": self.intent_id,
            "workflow_id": self.workflow_id,
            "step_id": self.step_id,
            "provider_request_id": self.provider_request_id,
            "attempt": self.attempt,
            "correlation_id": self.correlation_id,
            "result_id": self.result_id,
            "details": self.details or {},
            "timestamp": self.timestamp,
        }
        return {key: item for key, item in value.items() if item is not None}


class Diagnostics:
    """Write one JSON object per line and retain a queryable in-memory view."""

    def __init__(self, stream: TextIO | None = None) -> None:
        self.stream = stream or sys.stdout
        self.records: list[DiagnosticRecord] = []

    def emit(
        self, event: str, *, intent_id: str, workflow_id: str, **kwargs: Any
    ) -> DiagnosticRecord:
        record = DiagnosticRecord(
            event=event,
            intent_id=intent_id,
            workflow_id=workflow_id,
            timestamp=time.time(),
            **kwargs,
        )
        self.records.append(record)
        self.stream.write(json.dumps(record.as_dict(), sort_keys=True) + "\n")
        self.stream.flush()
        return record

    def chain(self, workflow_id: str) -> list[DiagnosticRecord]:
        return [record for record in self.records if record.workflow_id == workflow_id]

    def correlated(self, workflow_id: str) -> Iterable[dict[str, Any]]:
        return (record.as_dict() for record in self.chain(workflow_id))
