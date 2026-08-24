"""Framework-neutral value objects for numeric inner state and autonomy."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field, fields
from datetime import UTC, datetime
from enum import StrEnum
from types import MappingProxyType
from typing import Any
from uuid import uuid4

from fluctlight_core.fluctlights.contracts import bounded, signed_bounded


class InnerStateValidationError(ValueError):
    """Raised when typed inner-state input is malformed."""


class NumericPolicyError(ValueError):
    """Raised when a semantic result cannot be accepted by numeric policy."""


def _text(value: str | None, name: str, *, required: bool = False, limit: int = 4096) -> str | None:
    if value is None:
        if required:
            raise InnerStateValidationError(f"{name} is required")
        return None
    if not isinstance(value, str):
        raise InnerStateValidationError(f"{name} must be text")
    value = value.strip()
    if required and not value:
        raise InnerStateValidationError(f"{name} is required")
    if len(value) > limit:
        raise InnerStateValidationError(f"{name} exceeds {limit} characters")
    return value or None


def _aware(value: datetime, name: str) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise InnerStateValidationError(f"{name} must be timezone-aware")
    return value


def _refs(
    values: tuple[str, ...] | list[str], name: str, *, required: bool = False
) -> tuple[str, ...]:
    if not isinstance(values, tuple | list):
        raise InnerStateValidationError(f"{name} must be a list of references")
    normalized: list[str] = []
    for item in values:
        value = _text(item, name, required=True, limit=256)
        if value is None:
            raise InnerStateValidationError(f"{name} contains an empty reference")
        normalized.append(value)
    result = tuple(normalized)
    if required and not result:
        raise InnerStateValidationError(f"{name} requires at least one reference")
    if len(result) != len(set(result)):
        raise InnerStateValidationError(f"{name} must not contain duplicates")
    return result


class AffectDirection(StrEnum):
    POSITIVE = "positive"
    NEGATIVE = "negative"
    MIXED = "mixed"
    NEUTRAL = "neutral"


class DriveName(StrEnum):
    SOCIAL = "social"
    EXPLORATION = "exploration"
    REST = "rest"
    AUTONOMY = "autonomy"
    INTIMACY = "intimacy"


class GoalSource(StrEnum):
    DRIVE = "drive"
    EVENT = "event"
    HUMAN = "human"
    USER = "human"
    SELF = "self"


class GoalStatus(StrEnum):
    CANDIDATE = "candidate"
    ACTIVE = "active"
    PAUSED = "paused"
    COMPLETED = "completed"
    ABANDONED = "abandoned"
    CANCELLED = "cancelled"


class IntentionStatus(StrEnum):
    CANDIDATE = "candidate"
    PENDING = "pending"
    PAUSED = "paused"
    COMPLETED = "completed"
    EXPIRED = "expired"
    CANCELLED = "cancelled"


class TriggerType(StrEnum):
    TIME = "time"
    EVENT = "event"
    SEMANTIC = "semantic"


@dataclass(frozen=True, slots=True)
class PAD:
    pleasure: float = 0.0
    arousal: float = 0.0
    dominance: float = 0.0

    def __post_init__(self) -> None:
        for item in fields(self):
            signed_bounded(getattr(self, item.name), f"pad.{item.name}")

    def as_payload(self) -> dict[str, float]:
        return {item.name: getattr(self, item.name) for item in fields(self)}


@dataclass(frozen=True, slots=True)
class Mood:
    label: str | None = None
    intensity: float = 0.0
    source: str = "regulation"
    started_at: datetime | None = None
    expected_decay_at: datetime | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "label", _text(self.label, "mood.label"))
        object.__setattr__(
            self, "source", _text(self.source, "mood.source", required=True, limit=128)
        )
        bounded(self.intensity, "mood.intensity")
        if self.started_at is not None:
            _aware(self.started_at, "mood.started_at")
        if self.expected_decay_at is not None:
            _aware(self.expected_decay_at, "mood.expected_decay_at")

    def as_payload(self) -> dict[str, Any]:
        return {
            "label": self.label,
            "intensity": self.intensity,
            "source": self.source,
            "started_at": self.started_at.isoformat() if self.started_at else None,
            "expected_decay_at": self.expected_decay_at.isoformat()
            if self.expected_decay_at
            else None,
        }


@dataclass(frozen=True, slots=True)
class Momentum:
    pleasure_momentum: float = 0.0
    arousal_momentum: float = 0.0
    dominance_momentum: float = 0.0
    decay_rate: float = 0.25

    def __post_init__(self) -> None:
        for name in ("pleasure_momentum", "arousal_momentum", "dominance_momentum"):
            signed_bounded(getattr(self, name), f"momentum.{name}")
        bounded(self.decay_rate, "momentum.decay_rate")

    def as_payload(self) -> dict[str, float]:
        return {item.name: getattr(self, item.name) for item in fields(self)}


@dataclass(frozen=True, slots=True)
class Regulation:
    natural_decay_rate: float = 0.25
    sleep_recovery: float = 0.2
    positive_event_recovery: float = 0.1
    negative_event_amplification: float = 0.1
    emotional_stability: float = 0.5

    def __post_init__(self) -> None:
        for item in fields(self):
            bounded(getattr(self, item.name), f"regulation.{item.name}")

    def as_payload(self) -> dict[str, float]:
        return {item.name: getattr(self, item.name) for item in fields(self)}


@dataclass(frozen=True, slots=True)
class DriveState:
    name: DriveName | str
    level: float = 0.0
    baseline: float = 0.0
    growth_rate: float = 0.1
    satisfaction_rate: float = 0.1
    urgency_threshold: float = 0.7

    def __post_init__(self) -> None:
        normalized_drive: DriveName | str = (
            DriveName(self.name)
            if self.name in {item.value for item in DriveName}
            else str(self.name)
        )
        object.__setattr__(
            self,
            "name",
            normalized_drive,
        )
        object.__setattr__(
            self, "name", _text(str(self.name), "drive.name", required=True, limit=64)
        )
        for item in fields(self):
            if item.name != "name":
                bounded(getattr(self, item.name), f"drive.{item.name}")

    def as_payload(self) -> dict[str, Any]:
        return {
            "name": str(self.name),
            **{item.name: getattr(self, item.name) for item in fields(self) if item.name != "name"},
        }


@dataclass(frozen=True, slots=True)
class DriveConflict:
    first: str
    second: str
    intensity: float
    resolution: str | None = None

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "first", _text(self.first, "drive_conflict.first", required=True, limit=64)
        )
        object.__setattr__(
            self, "second", _text(self.second, "drive_conflict.second", required=True, limit=64)
        )
        if self.first == self.second:
            raise InnerStateValidationError("drive conflict requires two different drives")
        bounded(self.intensity, "drive_conflict.intensity")
        object.__setattr__(self, "resolution", _text(self.resolution, "drive_conflict.resolution"))

    def as_payload(self) -> dict[str, Any]:
        return {
            "first": self.first,
            "second": self.second,
            "intensity": self.intensity,
            "resolution": self.resolution,
        }


@dataclass(frozen=True, slots=True)
class SemanticPerception:
    event_kind: str
    observed_intent: str | None = None
    sentiment: str | None = None
    social_signals: tuple[str, ...] = ()
    environment_meaning: str | None = None

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "event_kind",
            _text(self.event_kind, "perception.event_kind", required=True, limit=128),
        )
        for name in ("observed_intent", "sentiment", "environment_meaning"):
            object.__setattr__(self, name, _text(getattr(self, name), f"perception.{name}"))
        object.__setattr__(
            self, "social_signals", _refs(self.social_signals, "perception.social_signals")
        )

    def as_payload(self) -> dict[str, Any]:
        return {
            "event_kind": self.event_kind,
            "observed_intent": self.observed_intent,
            "sentiment": self.sentiment,
            "social_signals": list(self.social_signals),
            "environment_meaning": self.environment_meaning,
        }


@dataclass(frozen=True, slots=True)
class Appraisal:
    relevance: float
    goal_congruence: float
    reward: float
    loss: float
    social_threat: float
    controllability: float
    responsibility: float
    relationship_significance: float
    expected_effect: float

    def __post_init__(self) -> None:
        for item in fields(self):
            bounded(getattr(self, item.name), f"appraisal.{item.name}")

    def as_payload(self) -> dict[str, float]:
        return {item.name: getattr(self, item.name) for item in fields(self)}


@dataclass(frozen=True, slots=True)
class DriveAssessment:
    drive: DriveName | str
    direction: AffectDirection
    strength: float
    confidence: float
    evidence_refs: tuple[str, ...]

    def __post_init__(self) -> None:
        normalized_drive: DriveName | str
        try:
            normalized_drive = DriveName(self.drive)
        except ValueError:
            normalized_drive = str(self.drive)
        object.__setattr__(
            self,
            "drive",
            normalized_drive,
        )
        object.__setattr__(self, "direction", AffectDirection(self.direction))
        object.__setattr__(
            self, "drive", _text(str(self.drive), "drive_assessment.drive", required=True, limit=64)
        )
        bounded(self.strength, "drive_assessment.strength")
        bounded(self.confidence, "drive_assessment.confidence")
        object.__setattr__(
            self,
            "evidence_refs",
            _refs(self.evidence_refs, "drive_assessment.evidence_refs", required=True),
        )


@dataclass(frozen=True, slots=True)
class SemanticAssessment:
    schema_version: str
    perception: SemanticPerception
    appraisal: Appraisal
    direction: AffectDirection
    strength: float
    confidence: float
    evidence_refs: tuple[str, ...]
    model: str
    model_version: str
    prompt_version: str
    source_event_id: str
    idempotency_key: str
    drive_assessments: tuple[DriveAssessment, ...] = ()
    raw_numeric_delta: Mapping[str, Any] | None = None

    def __post_init__(self) -> None:
        if not isinstance(self.perception, SemanticPerception) or not isinstance(
            self.appraisal, Appraisal
        ):
            raise NumericPolicyError("assessment perception and appraisal must be typed values")
        object.__setattr__(
            self,
            "schema_version",
            _text(self.schema_version, "schema_version", required=True, limit=64),
        )
        if self.schema_version != "semantic.assessment.v1":
            raise NumericPolicyError("unknown semantic assessment schema")
        bounded(self.strength, "assessment.strength")
        bounded(self.confidence, "assessment.confidence")
        object.__setattr__(self, "direction", AffectDirection(self.direction))
        object.__setattr__(
            self,
            "evidence_refs",
            _refs(self.evidence_refs, "assessment.evidence_refs", required=True),
        )
        for name in (
            "model",
            "model_version",
            "prompt_version",
            "source_event_id",
            "idempotency_key",
        ):
            object.__setattr__(
                self,
                name,
                _text(getattr(self, name), f"assessment.{name}", required=True, limit=256),
            )
        if self.raw_numeric_delta is not None:
            raise NumericPolicyError("model-provided numeric deltas are forbidden")
        if not isinstance(self.drive_assessments, tuple) or any(
            not isinstance(item, DriveAssessment) for item in self.drive_assessments
        ):
            raise NumericPolicyError("drive assessments must be typed values")

    def as_payload(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "perception": self.perception.as_payload(),
            "appraisal": self.appraisal.as_payload(),
            "direction": self.direction.value,
            "strength": self.strength,
            "confidence": self.confidence,
            "evidence_refs": list(self.evidence_refs),
            "model": self.model,
            "model_version": self.model_version,
            "prompt_version": self.prompt_version,
            "source_event_id": self.source_event_id,
            "idempotency_key": self.idempotency_key,
            "drive_assessments": [
                {
                    "drive": str(item.drive),
                    "direction": item.direction.value,
                    "strength": item.strength,
                    "confidence": item.confidence,
                    "evidence_refs": list(item.evidence_refs),
                }
                for item in self.drive_assessments
            ],
        }


@dataclass(frozen=True, slots=True)
class InnerStateSnapshot:
    fluctlight_id: str
    pad: PAD = field(default_factory=PAD)
    mood: Mood = field(default_factory=Mood)
    momentum: Momentum = field(default_factory=Momentum)
    regulation: Regulation = field(default_factory=Regulation)
    drives: tuple[DriveState, ...] = ()
    conflicts: tuple[DriveConflict, ...] = ()
    revision: int = 0
    last_updated_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "fluctlight_id",
            _text(self.fluctlight_id, "fluctlight_id", required=True, limit=128),
        )
        if self.revision < 0:
            raise InnerStateValidationError("inner state revision cannot be negative")
        _aware(self.last_updated_at, "last_updated_at")
        names = [str(item.name) for item in self.drives]
        if len(names) != len(set(names)):
            raise InnerStateValidationError("drive names must be unique")

    def as_payload(self) -> dict[str, Any]:
        return {
            "fluctlight_id": self.fluctlight_id,
            "pad": self.pad.as_payload(),
            "mood": self.mood.as_payload(),
            "momentum": self.momentum.as_payload(),
            "regulation": self.regulation.as_payload(),
            "drives": [item.as_payload() for item in self.drives],
            "conflicts": [item.as_payload() for item in self.conflicts],
            "revision": self.revision,
            "last_updated_at": self.last_updated_at.isoformat(),
        }


@dataclass(frozen=True, slots=True)
class InnerStateTransition:
    """Auditable result of one validated numeric policy application."""

    previous: InnerStateSnapshot
    current: InnerStateSnapshot
    result: str
    reason_code: str
    policy_version: str
    model_version: str
    requested_delta: Mapping[str, float]
    applied_delta: Mapping[str, float]
    idempotency_key: str
    source_event_id: str
    evidence_refs: tuple[str, ...]

    def __post_init__(self) -> None:
        if self.result not in {"accepted", "rejected", "deferred", "no_op"}:
            raise InnerStateValidationError("unknown inner-state transition result")
        object.__setattr__(
            self,
            "reason_code",
            _text(self.reason_code, "transition.reason_code", required=True, limit=128),
        )
        object.__setattr__(
            self,
            "policy_version",
            _text(self.policy_version, "transition.policy_version", required=True, limit=128),
        )
        object.__setattr__(
            self,
            "model_version",
            _text(self.model_version, "transition.model_version", required=True, limit=128),
        )
        object.__setattr__(
            self,
            "idempotency_key",
            _text(self.idempotency_key, "transition.idempotency_key", required=True, limit=256),
        )
        object.__setattr__(
            self,
            "source_event_id",
            _text(self.source_event_id, "transition.source_event_id", required=True, limit=256),
        )
        object.__setattr__(
            self,
            "evidence_refs",
            _refs(self.evidence_refs, "transition.evidence_refs", required=True),
        )
        object.__setattr__(
            self,
            "requested_delta",
            MappingProxyType({key: float(value) for key, value in self.requested_delta.items()}),
        )
        object.__setattr__(
            self,
            "applied_delta",
            MappingProxyType({key: float(value) for key, value in self.applied_delta.items()}),
        )


@dataclass(frozen=True, slots=True)
class GoalEvidence:
    fluctlight_id: str
    source: GoalSource
    description: str
    evidence_refs: tuple[str, ...]
    importance: float
    urgency: float
    goal_id: str = field(default_factory=lambda: f"goal_{uuid4().hex}")
    deadline: datetime | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "source", GoalSource(self.source))
        object.__setattr__(
            self,
            "fluctlight_id",
            _text(self.fluctlight_id, "goal.fluctlight_id", required=True, limit=128),
        )
        object.__setattr__(
            self,
            "description",
            _text(self.description, "goal.description", required=True, limit=4096),
        )
        object.__setattr__(
            self, "goal_id", _text(self.goal_id, "goal.id", required=True, limit=128)
        )
        object.__setattr__(
            self, "evidence_refs", _refs(self.evidence_refs, "goal.evidence_refs", required=True)
        )
        bounded(self.importance, "goal.importance")
        bounded(self.urgency, "goal.urgency")
        if self.deadline is not None:
            _aware(self.deadline, "goal.deadline")


@dataclass(frozen=True, slots=True)
class Goal:
    id: str
    fluctlight_id: str
    source: GoalSource
    description: str
    importance: float
    urgency: float
    progress: float
    status: GoalStatus
    evidence_refs: tuple[str, ...]
    revision: int = 0
    deadline: datetime | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    updated_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        object.__setattr__(self, "source", GoalSource(self.source))
        object.__setattr__(self, "status", GoalStatus(self.status))
        object.__setattr__(self, "id", _text(self.id, "goal.id", required=True, limit=128))
        object.__setattr__(
            self,
            "fluctlight_id",
            _text(self.fluctlight_id, "goal.fluctlight_id", required=True, limit=128),
        )
        object.__setattr__(
            self,
            "description",
            _text(self.description, "goal.description", required=True, limit=4096),
        )
        object.__setattr__(
            self, "evidence_refs", _refs(self.evidence_refs, "goal.evidence_refs", required=True)
        )
        bounded(self.importance, "goal.importance")
        bounded(self.urgency, "goal.urgency")
        bounded(self.progress, "goal.progress")
        if self.revision < 0:
            raise InnerStateValidationError("goal revision cannot be negative")
        if self.deadline is not None:
            _aware(self.deadline, "goal.deadline")
        _aware(self.created_at, "goal.created_at")
        _aware(self.updated_at, "goal.updated_at")


@dataclass(frozen=True, slots=True)
class GoalGovernance:
    goal_id: str
    action: GoalStatus
    actor_id: str
    expected_revision: int
    reason: str | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "action", GoalStatus(self.action))
        object.__setattr__(
            self, "goal_id", _text(self.goal_id, "goal_id", required=True, limit=128)
        )
        object.__setattr__(
            self, "actor_id", _text(self.actor_id, "actor_id", required=True, limit=128)
        )
        object.__setattr__(self, "reason", _text(self.reason, "governance.reason", limit=1024))
        if self.expected_revision < 0:
            raise InnerStateValidationError("expected_revision cannot be negative")


@dataclass(frozen=True, slots=True)
class TimeTrigger:
    at: datetime
    type: TriggerType = field(default=TriggerType.TIME, init=False)

    def __post_init__(self) -> None:
        _aware(self.at, "time_trigger.at")

    def as_payload(self) -> dict[str, Any]:
        return {"type": self.type.value, "at": self.at.isoformat()}


@dataclass(frozen=True, slots=True)
class EventTrigger:
    event_type: str
    event_id: str | None = None
    type: TriggerType = field(default=TriggerType.EVENT, init=False)

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "event_type",
            _text(self.event_type, "event_trigger.event_type", required=True, limit=128),
        )
        object.__setattr__(
            self, "event_id", _text(self.event_id, "event_trigger.event_id", limit=128)
        )

    def as_payload(self) -> dict[str, Any]:
        return {"type": self.type.value, "event_type": self.event_type, "event_id": self.event_id}


@dataclass(frozen=True, slots=True)
class SemanticTrigger:
    schema_version: str
    evidence_refs: tuple[str, ...]
    type: TriggerType = field(default=TriggerType.SEMANTIC, init=False)

    def __post_init__(self) -> None:
        if self.schema_version != "semantic.trigger.v1":
            raise InnerStateValidationError("unknown semantic trigger schema")
        object.__setattr__(
            self,
            "evidence_refs",
            _refs(self.evidence_refs, "semantic_trigger.evidence_refs", required=True),
        )

    def as_payload(self) -> dict[str, Any]:
        return {
            "type": self.type.value,
            "schema_version": self.schema_version,
            "evidence_refs": list(self.evidence_refs),
        }


@dataclass(frozen=True, slots=True)
class IntentionEvidence:
    fluctlight_id: str
    goal_id: str | None
    action: str
    trigger: TimeTrigger | EventTrigger | SemanticTrigger
    confidence: float
    expiration: datetime
    evidence_refs: tuple[str, ...]
    intention_id: str = field(default_factory=lambda: f"intention_{uuid4().hex}")
    preferred_time: datetime | None = None
    permission_snapshot: Mapping[str, Any] = field(default_factory=dict)
    budget_snapshot: Mapping[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        if not isinstance(self.trigger, TimeTrigger | EventTrigger | SemanticTrigger):
            raise InnerStateValidationError("intention trigger must be a typed trigger")
        object.__setattr__(
            self,
            "fluctlight_id",
            _text(self.fluctlight_id, "intention.fluctlight_id", required=True, limit=128),
        )
        object.__setattr__(self, "goal_id", _text(self.goal_id, "intention.goal_id", limit=128))
        object.__setattr__(
            self, "action", _text(self.action, "intention.action", required=True, limit=256)
        )
        object.__setattr__(
            self, "intention_id", _text(self.intention_id, "intention.id", required=True, limit=128)
        )
        bounded(self.confidence, "intention.confidence")
        object.__setattr__(
            self,
            "evidence_refs",
            _refs(self.evidence_refs, "intention.evidence_refs", required=True),
        )
        _aware(self.expiration, "intention.expiration")
        if self.preferred_time is not None:
            _aware(self.preferred_time, "intention.preferred_time")
        if self.preferred_time is not None and self.expiration <= self.preferred_time:
            raise InnerStateValidationError("intention expiration must follow preferred time")
        object.__setattr__(
            self, "permission_snapshot", MappingProxyType(dict(self.permission_snapshot))
        )
        object.__setattr__(self, "budget_snapshot", MappingProxyType(dict(self.budget_snapshot)))


@dataclass(frozen=True, slots=True)
class Intention:
    id: str
    fluctlight_id: str
    goal_id: str | None
    action: str
    preferred_time: datetime | None
    trigger: TimeTrigger | EventTrigger | SemanticTrigger
    confidence: float
    expiration: datetime
    evidence_refs: tuple[str, ...]
    permission_snapshot: Mapping[str, Any]
    budget_snapshot: Mapping[str, Any]
    status: IntentionStatus = IntentionStatus.CANDIDATE
    revision: int = 0
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    updated_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        if not isinstance(self.trigger, TimeTrigger | EventTrigger | SemanticTrigger):
            raise InnerStateValidationError("intention trigger must be a typed trigger")
        object.__setattr__(self, "status", IntentionStatus(self.status))
        object.__setattr__(self, "id", _text(self.id, "intention.id", required=True, limit=128))
        object.__setattr__(
            self,
            "fluctlight_id",
            _text(self.fluctlight_id, "intention.fluctlight_id", required=True, limit=128),
        )
        object.__setattr__(self, "goal_id", _text(self.goal_id, "intention.goal_id", limit=128))
        object.__setattr__(
            self, "action", _text(self.action, "intention.action", required=True, limit=256)
        )
        bounded(self.confidence, "intention.confidence")
        _aware(self.expiration, "intention.expiration")
        if self.preferred_time is not None:
            _aware(self.preferred_time, "intention.preferred_time")
        if self.preferred_time is not None and self.expiration <= self.preferred_time:
            raise InnerStateValidationError("intention expiration must follow preferred time")
        object.__setattr__(
            self,
            "evidence_refs",
            _refs(self.evidence_refs, "intention.evidence_refs", required=True),
        )
        object.__setattr__(
            self, "permission_snapshot", MappingProxyType(dict(self.permission_snapshot))
        )
        object.__setattr__(self, "budget_snapshot", MappingProxyType(dict(self.budget_snapshot)))
        if self.revision < 0:
            raise InnerStateValidationError("intention revision cannot be negative")
        _aware(self.created_at, "intention.created_at")
        _aware(self.updated_at, "intention.updated_at")


@dataclass(frozen=True, slots=True)
class IntentionGovernance:
    intention_id: str
    action: IntentionStatus
    actor_id: str
    expected_revision: int
    reason: str | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "action", IntentionStatus(self.action))
        object.__setattr__(
            self, "intention_id", _text(self.intention_id, "intention_id", required=True, limit=128)
        )
        object.__setattr__(
            self, "actor_id", _text(self.actor_id, "actor_id", required=True, limit=128)
        )
        object.__setattr__(self, "reason", _text(self.reason, "governance.reason", limit=1024))
        if self.expected_revision < 0:
            raise InnerStateValidationError("expected_revision cannot be negative")
