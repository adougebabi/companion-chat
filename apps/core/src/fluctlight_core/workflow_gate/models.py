"""Serializable data contracts for the runtime gate."""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
from enum import StrEnum
from typing import Any


class WorkflowStatus(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    SLEEPING = "sleeping"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELED = "canceled"
    CRASHED = "crashed"
    PAUSED = "paused"


class StepStatus(StrEnum):
    PENDING = "pending"
    RUNNING = "running"
    HEARTBEAT = "heartbeat"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELED = "canceled"


@dataclass(frozen=True, slots=True)
class QueuePolicy:
    name: str
    concurrency: int
    rate_limit_per_second: float

    def __post_init__(self) -> None:
        if self.concurrency < 1 or self.rate_limit_per_second <= 0:
            raise ValueError("queue limits must be positive")


@dataclass(frozen=True, slots=True)
class FailureInjection:
    """Failure points used by tests and the reproducible report."""

    provider_success_before_checkpoint: bool = False
    crash_after_provider_checkpoint: bool = False
    crash_before_result_commit: bool = False
    timeout: bool = False
    cancel: bool = False
    worker_restart: bool = False
    database_restart: bool = False


@dataclass(frozen=True, slots=True)
class GateInput:
    intent_key: str
    queue: str = "media"
    sleep_seconds: float = 0.0
    h3_duration_seconds: float = 0.0
    heartbeat_interval_seconds: float = 5.0
    timeout_seconds: float = 900.0
    decision_version: str = "gate-v1"
    failure: FailureInjection = field(default_factory=FailureInjection)


@dataclass(frozen=True, slots=True)
class ProviderResult:
    request_id: str
    output: str
    effect_count: int


@dataclass(frozen=True, slots=True)
class GateResult:
    intent_id: str
    workflow_id: str
    provider_request_id: str
    result_id: str
    status: WorkflowStatus
    output: str | None = None
    recovery_count: int = 0


@dataclass(frozen=True, slots=True)
class ManagementAudit:
    action: str
    workflow_id: str
    actor: str
    authorized: bool
    audit_id: str
    details: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True, slots=True)
class WorkflowRecord:
    intent_id: str
    workflow_id: str
    queue: str
    status: WorkflowStatus
    step_status: dict[str, StepStatus] = field(default_factory=dict)
    provider_request_id: str | None = None
    provider_result: ProviderResult | None = None
    result: GateResult | None = None
    recovery_count: int = 0
    checkpoint: str | None = None
    error: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    def as_dict(self) -> dict[str, Any]:
        return asdict(self)
