"""Serializable contracts shared by the API, Worker, and local gate tests."""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
from enum import StrEnum
from typing import Any

QUEUES = ("interaction", "lifecycle", "media")


class WorkflowStatus(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    SLEEPING = "sleeping"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELED = "canceled"
    TERMINATED = "terminated"
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
        if self.name not in QUEUES:
            raise ValueError(f"unknown task queue: {self.name}")
        if self.concurrency < 1 or self.rate_limit_per_second <= 0:
            raise ValueError("queue limits must be positive")


@dataclass(frozen=True, slots=True)
class FailureInjection:
    """Bounded faults used by the reproducible recovery fixtures."""

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
    continue_after: int = 0
    failure: FailureInjection = field(default_factory=FailureInjection)

    def __post_init__(self) -> None:
        if not self.intent_key.strip():
            raise ValueError("intent_key must not be empty")
        if self.queue not in QUEUES:
            raise ValueError(f"queue must be one of {', '.join(QUEUES)}")
        if self.sleep_seconds < 0 or self.h3_duration_seconds < 0:
            raise ValueError("durations must be non-negative")
        if self.heartbeat_interval_seconds <= 0 or self.timeout_seconds <= 0:
            raise ValueError("heartbeat interval and timeout must be positive")
        if self.continue_after < 0:
            raise ValueError("continue_after must be non-negative")

    def as_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, payload: dict[str, Any]) -> GateInput:
        failure = payload.get("failure", {})
        fields = {
            "intent_key",
            "queue",
            "sleep_seconds",
            "h3_duration_seconds",
            "heartbeat_interval_seconds",
            "timeout_seconds",
            "decision_version",
            "continue_after",
        }
        return cls(
            **{
                **{key: payload[key] for key in fields if key in payload},
                "failure": FailureInjection(**failure),
            }
        )


@dataclass(frozen=True, slots=True)
class ProviderResult:
    request_id: str
    output: str
    effect_count: int = 1


@dataclass(frozen=True, slots=True)
class GateResult:
    intent_id: str
    workflow_id: str
    provider_request_id: str
    result_id: str
    status: WorkflowStatus
    output: str | None = None
    recovery_count: int = 0

    def as_dict(self) -> dict[str, Any]:
        return asdict(self)


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


@dataclass(frozen=True, slots=True)
class ManagementAudit:
    action: str
    workflow_id: str
    actor: str
    authorized: bool
    audit_id: str
    details: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True, slots=True)
class RepairCommand:
    reason: str
    expected_version: str | None = None


@dataclass(frozen=True, slots=True)
class RepairResult:
    accepted: bool
    workflow_id: str
    reason: str
    acknowledged_version: str


@dataclass(frozen=True, slots=True)
class WorkerDeploymentVersion:
    """Current deployment-version fixture; no deprecated Build ID redirect."""

    deployment_name: str = "fluctlight-gate"
    version: str = "gate-v1"
    compatible_history_versions: tuple[str, ...] = ("gate-v1",)

    def can_replay(self, history_version: str) -> bool:
        return history_version in self.compatible_history_versions
