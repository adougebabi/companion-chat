"""Framework-free contracts for the Fluctlight cognitive runtime.

The cognition package owns ordering, validation, idempotency and freeze policy.
Provider implementations only supply typed semantic proposals and realization
content through the ports declared here.
"""

from __future__ import annotations

from collections.abc import AsyncIterator, Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime, timedelta
from enum import StrEnum
from hashlib import sha256
from typing import Any, Protocol
from uuid import uuid4

from fluctlight_core.inner_state.contracts import SemanticAssessment
from fluctlight_core.providers.contracts import ProviderProvenance


class CognitionError(RuntimeError):
    """Base error for explicit cognitive failures."""


class CognitionConflictError(CognitionError):
    """Raised when a state or reflection revision is stale."""


class ProviderExecutionError(CognitionError):
    """Raised when an injected Provider cannot return a typed result."""


class InboxStatus(StrEnum):
    PENDING = "pending"
    CLAIMED = "claimed"
    FROZEN = "frozen"
    COMPLETED = "completed"
    FAILED = "failed"


class ActionStatus(StrEnum):
    FROZEN = "frozen"
    REALIZING = "realizing"
    COMPLETED = "completed"
    FAILED = "failed"


class ActionType(StrEnum):
    REPLY = "reply"
    PROACTIVE_MESSAGE = "proactive_message"
    NO_OP = "no_op"
    MEMORY_CANDIDATE = "memory_candidate"
    RELATIONSHIP_CANDIDATE = "relationship_candidate"
    MEDIA_REQUEST = "media_request"
    SCHEDULE_PROPOSAL = "schedule_proposal"


def _required_text(value: str, name: str, limit: int = 512) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    value = value.strip()
    if len(value) > limit:
        raise ValueError(f"{name} exceeds {limit} characters")
    return value


def _aware(value: datetime, name: str) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError(f"{name} must be timezone-aware")
    return value


def _refs(values: Sequence[str], name: str) -> tuple[str, ...]:
    result = tuple(_required_text(item, name, 256) for item in values)
    if len(result) != len(set(result)):
        raise ValueError(f"{name} contains duplicate references")
    return result


@dataclass(frozen=True, slots=True)
class CognitionFact:
    """An observed fact that enters one Fluctlight's ordered inbox."""

    id: str
    fluctlight_id: str
    event_type: str
    payload: Mapping[str, Any]
    causation_id: str
    correlation_id: str
    idempotency_key: str
    occurred_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        for name in (
            "id",
            "fluctlight_id",
            "event_type",
            "causation_id",
            "correlation_id",
            "idempotency_key",
        ):
            object.__setattr__(self, name, _required_text(getattr(self, name), name))
        if not isinstance(self.payload, Mapping):
            raise ValueError("fact.payload must be an object")
        object.__setattr__(self, "payload", dict(self.payload))
        object.__setattr__(self, "occurred_at", _aware(self.occurred_at, "occurred_at"))


@dataclass(frozen=True, slots=True)
class EnqueuedFact:
    fact: CognitionFact
    sequence: int
    status: InboxStatus = InboxStatus.PENDING

    def __post_init__(self) -> None:
        if self.sequence < 1:
            raise ValueError("inbox sequence must be positive")


@dataclass(frozen=True, slots=True)
class InboxClaim:
    fact: CognitionFact
    sequence: int
    attempt: int
    worker_id: str


@dataclass(frozen=True, slots=True)
class DecisionProposal:
    """LLM-owned candidate action; Python validates and freezes it."""

    action_type: ActionType
    payload: Mapping[str, Any]
    confidence: float
    evidence_refs: tuple[str, ...]
    decision_id: str = field(default_factory=lambda: f"decision_{uuid4().hex}")
    expires_at: datetime | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "action_type", ActionType(self.action_type))
        if not isinstance(self.payload, Mapping):
            raise ValueError("decision.payload must be an object")
        object.__setattr__(self, "payload", dict(self.payload))
        if not 0.0 <= self.confidence <= 1.0:
            raise ValueError("decision.confidence must be between 0 and 1")
        object.__setattr__(
            self, "evidence_refs", _refs(self.evidence_refs, "decision.evidence_refs")
        )
        object.__setattr__(self, "decision_id", _required_text(self.decision_id, "decision_id"))
        if self.expires_at is not None:
            object.__setattr__(self, "expires_at", _aware(self.expires_at, "expires_at"))


@dataclass(frozen=True, slots=True)
class AssessmentEnvelope:
    assessment: SemanticAssessment
    decision: DecisionProposal
    provenance: ProviderProvenance
    correlation_id: str

    def __post_init__(self) -> None:
        if not isinstance(self.assessment, SemanticAssessment):
            raise ValueError("assessment must be a typed SemanticAssessment")
        if not isinstance(self.decision, DecisionProposal):
            raise ValueError("decision must be a typed DecisionProposal")
        object.__setattr__(
            self, "correlation_id", _required_text(self.correlation_id, "correlation_id")
        )


@dataclass(frozen=True, slots=True)
class FrozenAction:
    action_id: str
    decision_id: str
    inbox_id: str
    fluctlight_id: str
    action_type: ActionType
    payload: Mapping[str, Any]
    state_revision: int
    provider_request_id: str
    status: ActionStatus = ActionStatus.FROZEN

    def __post_init__(self) -> None:
        for name in (
            "action_id",
            "decision_id",
            "inbox_id",
            "fluctlight_id",
            "provider_request_id",
        ):
            object.__setattr__(self, name, _required_text(getattr(self, name), name))
        object.__setattr__(self, "action_type", ActionType(self.action_type))
        if not isinstance(self.payload, Mapping):
            raise ValueError("action.payload must be an object")
        object.__setattr__(self, "payload", dict(self.payload))
        if self.state_revision < 0:
            raise ValueError("state_revision cannot be negative")
        object.__setattr__(self, "status", ActionStatus(self.status))


@dataclass(frozen=True, slots=True)
class RealizationResult:
    provider_request_id: str
    payload: Mapping[str, Any]
    status: str = "completed"

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "provider_request_id",
            _required_text(self.provider_request_id, "provider_request_id"),
        )
        if not isinstance(self.payload, Mapping):
            raise ValueError("realization.payload must be an object")
        object.__setattr__(self, "payload", dict(self.payload))
        object.__setattr__(self, "status", _required_text(self.status, "status", 64))


@dataclass(frozen=True, slots=True)
class ProcessOutcome:
    status: InboxStatus
    action: FrozenAction | None = None
    realization: RealizationResult | None = None
    error_code: str | None = None


@dataclass(frozen=True, slots=True)
class ReflectionWindow:
    fluctlight_id: str
    from_sequence: int
    to_sequence: int
    base_state_revision: int
    watermark: int

    def __post_init__(self) -> None:
        if self.to_sequence < self.from_sequence or self.from_sequence < 0:
            raise ValueError("reflection window sequence range is invalid")
        if self.base_state_revision < 0 or self.watermark < 0:
            raise ValueError("reflection revisions must be non-negative")


@dataclass(frozen=True, slots=True)
class ReflectionProposal:
    proposal_id: str
    fluctlight_id: str
    from_sequence: int
    to_sequence: int
    base_state_revision: int
    payload: Mapping[str, Any]
    evidence_refs: tuple[str, ...]
    provenance: ProviderProvenance

    def __post_init__(self) -> None:
        for name in ("proposal_id", "fluctlight_id"):
            object.__setattr__(self, name, _required_text(getattr(self, name), name))
        if self.to_sequence < self.from_sequence or self.from_sequence < 0:
            raise ValueError("reflection proposal sequence range is invalid")
        if self.base_state_revision < 0:
            raise ValueError("base_state_revision must be non-negative")
        if not isinstance(self.payload, Mapping):
            raise ValueError("reflection payload must be an object")
        object.__setattr__(self, "payload", dict(self.payload))
        object.__setattr__(self, "evidence_refs", _refs(self.evidence_refs, "evidence_refs"))


class AssessmentProvider(Protocol):
    async def assess(self, fact: CognitionFact, *, correlation_id: str) -> AssessmentEnvelope: ...


class RealizationProvider(Protocol):
    async def realize(self, action: FrozenAction, *, correlation_id: str) -> RealizationResult: ...


class StreamingRealizationProvider(Protocol):
    def stream_realize(
        self, action: FrozenAction, *, correlation_id: str
    ) -> AsyncIterator[str]: ...


class ReflectionProvider(Protocol):
    async def reflect(
        self, window: ReflectionWindow, *, correlation_id: str
    ) -> ReflectionProposal: ...


class ReflectionApplier(Protocol):
    async def apply(self, proposal: ReflectionProposal) -> Any: ...


class StateApplier(Protocol):
    async def apply_assessment(
        self, fluctlight_id: str, assessment: SemanticAssessment, *, tx: Any
    ) -> int: ...


def stable_action_id(source_event_id: str, decision_id: str) -> str:
    digest = sha256(f"{source_event_id}:{decision_id}".encode()).hexdigest()[:32]
    return f"action_{digest}"


def stable_provider_request_id(action_id: str) -> str:
    return f"provider_{sha256(action_id.encode('utf-8')).hexdigest()[:32]}"


def new_correlation_id(prefix: str = "cog") -> str:
    return f"{prefix}_{uuid4().hex}"


def lease_expiry(seconds: int = 30) -> datetime:
    return datetime.now(UTC) + timedelta(seconds=seconds)
