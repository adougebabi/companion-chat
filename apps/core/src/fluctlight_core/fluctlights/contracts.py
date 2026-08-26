"""Transport-neutral Fluctlight foundation contracts.

The module deliberately contains no persistence or framework imports.  A
Fluctlight is represented by immutable value objects at the domain boundary;
the application service is responsible for mapping them to PostgreSQL rows.
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field, fields
from datetime import UTC, datetime
from enum import StrEnum
from math import isfinite
from types import MappingProxyType
from typing import Any
from uuid import uuid4


class FoundationValidationError(ValueError):
    """Raised when a foundation value violates a deterministic contract."""


class InitializationMode(StrEnum):
    LLM_DEFINED = "llm_defined"
    BLANK_SLATE = "blank_slate"


class FluctlightStatus(StrEnum):
    ACTIVE = "active"
    PAUSED = "paused"
    RETIRED = "retired"


class MutabilityClass(StrEnum):
    IMMUTABLE = "immutable"
    HUMAN_GOVERNED = "human_governed"
    LIVED = "lived"


class RevisionSource(StrEnum):
    INITIALIZATION = "initialization"
    HUMAN = "human"
    LIVED_FACT = "lived_fact"
    REFLECTION = "reflection"
    ROLLBACK = "rollback"


class RevisionStatus(StrEnum):
    PROPOSED = "proposed"
    ACCEPTED = "accepted"
    REJECTED = "rejected"


def _text(
    value: str | None, field_name: str, *, required: bool = False, limit: int = 4096
) -> str | None:
    if value is None:
        if required:
            raise FoundationValidationError(f"{field_name} is required")
        return None
    if not isinstance(value, str):
        raise FoundationValidationError(f"{field_name} must be text")
    value = value.strip()
    if required and not value:
        raise FoundationValidationError(f"{field_name} is required")
    if len(value) > limit:
        raise FoundationValidationError(f"{field_name} exceeds {limit} characters")
    return value or None


def _aware(value: datetime, field_name: str) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise FoundationValidationError(f"{field_name} must be timezone-aware")
    return value


def bounded(value: float, field_name: str, lower: float = 0.0, upper: float = 1.0) -> float:
    if not isinstance(value, int | float) or isinstance(value, bool):
        raise FoundationValidationError(f"{field_name} must be numeric")
    numeric = float(value)
    if not isfinite(numeric) or numeric < lower or numeric > upper:
        raise FoundationValidationError(f"{field_name} must be between {lower} and {upper}")
    return numeric


def signed_bounded(value: float, field_name: str) -> float:
    return bounded(value, field_name, -1.0, 1.0)


def _string_tuple(values: tuple[str, ...] | list[str] | None, field_name: str) -> tuple[str, ...]:
    if values is None:
        return ()
    if not isinstance(values, tuple | list):
        raise FoundationValidationError(f"{field_name} must be a list of text")
    result: list[str] = []
    for item in values:
        normalized = _text(item, field_name, required=True, limit=512)
        if normalized is None:
            raise FoundationValidationError(f"{field_name} contains an empty value")
        result.append(normalized)
    return tuple(result)


@dataclass(frozen=True, slots=True)
class Identity:
    """Stable identity anchors and lived biography fields."""

    id: str
    name: str | None = None
    age: int | None = None
    gender: str | None = None
    occupation: str | None = None
    residence: str | None = None
    timezone: str | None = None
    birthday: str | None = None
    background: str | None = None
    biography: str | None = None
    core_values: tuple[str, ...] = ()
    worldview: str | None = None
    notes: str | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "id", _text(self.id, "identity.id", required=True, limit=128))
        for name in (
            "name",
            "gender",
            "occupation",
            "residence",
            "timezone",
            "birthday",
            "background",
            "biography",
            "worldview",
            "notes",
        ):
            object.__setattr__(self, name, _text(getattr(self, name), f"identity.{name}"))
        if self.age is not None and (
            isinstance(self.age, bool) or not isinstance(self.age, int) or not 0 <= self.age <= 200
        ):
            raise FoundationValidationError("identity.age must be between 0 and 200")
        object.__setattr__(
            self, "core_values", _string_tuple(self.core_values, "identity.core_values")
        )

    def as_payload(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "name": self.name,
            "age": self.age,
            "gender": self.gender,
            "occupation": self.occupation,
            "residence": self.residence,
            "timezone": self.timezone,
            "birthday": self.birthday,
            "background": self.background,
            "biography": self.biography,
            "core_values": list(self.core_values),
            "worldview": self.worldview,
            "notes": self.notes,
        }


@dataclass(frozen=True, slots=True)
class PersonalityUpdatePolicy:
    """Server-owned limits for slow evidence-backed personality change."""

    evidence_window_events: int = 3
    max_delta: float = 0.05
    cooldown_seconds: int = 86_400
    minimum_confidence: float = 0.7

    def __post_init__(self) -> None:
        if self.evidence_window_events <= 0:
            raise FoundationValidationError("personality evidence window must be positive")
        if self.cooldown_seconds < 0:
            raise FoundationValidationError("personality cooldown cannot be negative")
        bounded(self.max_delta, "personality.max_delta")
        bounded(self.minimum_confidence, "personality.minimum_confidence")

    def as_payload(self) -> dict[str, Any]:
        return {
            "evidence_window_events": self.evidence_window_events,
            "max_delta": self.max_delta,
            "cooldown_seconds": self.cooldown_seconds,
            "minimum_confidence": self.minimum_confidence,
        }


@dataclass(frozen=True, slots=True)
class Personality:
    """Long-lived normalized traits; changes require reflection evidence."""

    openness: float = 0.5
    conscientiousness: float = 0.5
    extraversion: float = 0.5
    agreeableness: float = 0.5
    neuroticism: float = 0.5
    curiosity: float = 0.5
    independence: float = 0.5
    patience: float = 0.5
    empathy: float = 0.5
    assertiveness: float = 0.5
    humor: float = 0.5
    sociability: float = 0.5
    risk_tolerance: float = 0.5
    update_policy: PersonalityUpdatePolicy = field(default_factory=PersonalityUpdatePolicy)

    def __post_init__(self) -> None:
        for item in fields(self):
            if item.name == "update_policy":
                continue
            bounded(getattr(self, item.name), f"personality.{item.name}")

    @classmethod
    def neutral(cls) -> Personality:
        return cls()

    def as_payload(self) -> dict[str, Any]:
        payload = {
            item.name: getattr(self, item.name)
            for item in fields(self)
            if item.name != "update_policy"
        }
        payload["update_policy"] = self.update_policy.as_payload()
        return payload


@dataclass(frozen=True, slots=True)
class BehavioralPolicy:
    """Habitual outward expression, separate from semantic decision making."""

    response_style: str | None = None
    message_length: str | None = None
    emoji_frequency: float = 0.0
    punctuation_style: str | None = None
    humor_style: str | None = None
    sarcasm_tendency: float = 0.0
    directness: float = 0.5
    initiative: float = 0.5
    topic_initiation: float = 0.5
    silence_tolerance: float = 0.5
    response_delay: float = 0.0
    emotional_expression: float = 0.5
    conflict_style: str | None = None
    refusal_style: str | None = None
    intimacy_expression: str | None = None

    def __post_init__(self) -> None:
        for name in (
            "response_style",
            "message_length",
            "punctuation_style",
            "humor_style",
            "conflict_style",
            "refusal_style",
            "intimacy_expression",
        ):
            object.__setattr__(self, name, _text(getattr(self, name), f"behavioral_policy.{name}"))
        for name in (
            "emoji_frequency",
            "sarcasm_tendency",
            "directness",
            "initiative",
            "topic_initiation",
            "silence_tolerance",
            "emotional_expression",
        ):
            bounded(getattr(self, name), f"behavioral_policy.{name}")
        if not isinstance(self.response_delay, int | float) or isinstance(
            self.response_delay, bool
        ):
            raise FoundationValidationError("behavioral_policy.response_delay must be numeric")
        if not isfinite(float(self.response_delay)) or self.response_delay < 0:
            raise FoundationValidationError("behavioral_policy.response_delay cannot be negative")

    def as_payload(self) -> dict[str, Any]:
        return {item.name: getattr(self, item.name) for item in fields(self)}


@dataclass(frozen=True, slots=True)
class FluctlightSnapshot:
    id: str
    initialization_mode: InitializationMode
    status: FluctlightStatus
    identity: Identity
    personality: Personality
    behavioral_policy: BehavioralPolicy
    current_revision: int = 0
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    updated_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    retired_at: datetime | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "id", _text(self.id, "fluctlight.id", required=True, limit=128))
        object.__setattr__(
            self, "initialization_mode", InitializationMode(self.initialization_mode)
        )
        object.__setattr__(self, "status", FluctlightStatus(self.status))
        if self.identity.id != self.id:
            raise FoundationValidationError("identity.id must equal fluctlight.id")
        if self.current_revision < 0:
            raise FoundationValidationError("current_revision cannot be negative")
        _aware(self.created_at, "created_at")
        _aware(self.updated_at, "updated_at")
        if self.retired_at is not None:
            _aware(self.retired_at, "retired_at")
            if self.status != FluctlightStatus.RETIRED:
                raise FoundationValidationError("retired_at requires retired status")

    def as_payload(self) -> dict[str, Any]:
        return {
            "id": self.id,
            "initialization_mode": self.initialization_mode.value,
            "status": self.status.value,
            "identity": self.identity.as_payload(),
            "personality": self.personality.as_payload(),
            "behavioral_policy": self.behavioral_policy.as_payload(),
            "current_revision": self.current_revision,
            "created_at": self.created_at.isoformat(),
            "updated_at": self.updated_at.isoformat(),
            "retired_at": self.retired_at.isoformat() if self.retired_at else None,
        }


@dataclass(frozen=True, slots=True)
class CreateFluctlight:
    actor_id: str
    id: str = field(default_factory=lambda: f"fluctlight_{uuid4().hex}")
    initialization_mode: InitializationMode = InitializationMode.BLANK_SLATE
    identity: Identity | None = None
    personality: Personality = field(default_factory=Personality.neutral)
    behavioral_policy: BehavioralPolicy = field(default_factory=BehavioralPolicy)

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "actor_id", _text(self.actor_id, "actor_id", required=True, limit=128)
        )
        object.__setattr__(self, "id", _text(self.id, "fluctlight.id", required=True, limit=128))
        object.__setattr__(
            self, "initialization_mode", InitializationMode(self.initialization_mode)
        )
        if self.identity is None:
            object.__setattr__(self, "identity", Identity(id=self.id))
        elif self.identity.id != self.id:
            raise FoundationValidationError("identity.id must equal create id")


@dataclass(frozen=True, slots=True)
class FoundationRevisionRequest:
    fluctlight_id: str
    actor_id: str
    source: RevisionSource
    changes: Mapping[str, Any]
    evidence_refs: tuple[str, ...]
    expected_revision: int
    idempotency_key: str
    confidence: float = 1.0
    requested_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    reason: str | None = None

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "fluctlight_id",
            _text(self.fluctlight_id, "fluctlight_id", required=True, limit=128),
        )
        object.__setattr__(
            self, "actor_id", _text(self.actor_id, "actor_id", required=True, limit=128)
        )
        object.__setattr__(
            self,
            "idempotency_key",
            _text(self.idempotency_key, "idempotency_key", required=True, limit=256),
        )
        object.__setattr__(self, "source", RevisionSource(self.source))
        if self.expected_revision < 0:
            raise FoundationValidationError("expected_revision cannot be negative")
        bounded(self.confidence, "revision.confidence")
        refs = _string_tuple(self.evidence_refs, "evidence_refs")
        if len(set(refs)) != len(refs):
            raise FoundationValidationError("evidence_refs must be unique")
        object.__setattr__(self, "evidence_refs", refs)
        object.__setattr__(self, "changes", MappingProxyType(dict(self.changes)))
        object.__setattr__(self, "reason", _text(self.reason, "revision.reason", limit=1024))
        _aware(self.requested_at, "requested_at")


@dataclass(frozen=True, slots=True)
class FoundationRevision:
    id: str
    fluctlight_id: str
    revision: int
    base_revision: int
    source: RevisionSource
    status: RevisionStatus
    actor_id: str
    changes: Mapping[str, Any]
    evidence_refs: tuple[str, ...]
    candidate: FluctlightSnapshot
    idempotency_key: str
    created_at: datetime
    accepted_at: datetime | None = None
    confidence: float = 1.0
    reason: str | None = None

    def __post_init__(self) -> None:
        if self.revision < self.base_revision or (
            self.revision == self.base_revision and self.revision != 0
        ):
            raise FoundationValidationError("revision must be greater than base_revision")
        if self.candidate.current_revision != self.revision:
            raise FoundationValidationError("candidate revision does not match revision record")
        object.__setattr__(self, "changes", MappingProxyType(dict(self.changes)))
        object.__setattr__(
            self, "evidence_refs", _string_tuple(self.evidence_refs, "evidence_refs")
        )
        _aware(self.created_at, "created_at")
        if self.accepted_at is not None:
            _aware(self.accepted_at, "accepted_at")
        object.__setattr__(self, "source", RevisionSource(self.source))
        object.__setattr__(self, "status", RevisionStatus(self.status))
        bounded(self.confidence, "revision.confidence")
        object.__setattr__(self, "reason", _text(self.reason, "revision.reason", limit=1024))


IDENTITY_FIELDS = frozenset(
    {
        "name",
        "age",
        "gender",
        "occupation",
        "residence",
        "timezone",
        "birthday",
        "background",
        "biography",
        "core_values",
        "worldview",
        "notes",
    }
)
PERSONALITY_FIELDS = frozenset(
    {
        "openness",
        "conscientiousness",
        "extraversion",
        "agreeableness",
        "neuroticism",
        "curiosity",
        "independence",
        "patience",
        "empathy",
        "assertiveness",
        "humor",
        "sociability",
        "risk_tolerance",
    }
)
BEHAVIORAL_POLICY_FIELDS = frozenset(
    {
        "response_style",
        "message_length",
        "emoji_frequency",
        "punctuation_style",
        "humor_style",
        "sarcasm_tendency",
        "directness",
        "initiative",
        "topic_initiation",
        "silence_tolerance",
        "response_delay",
        "emotional_expression",
        "conflict_style",
        "refusal_style",
        "intimacy_expression",
    }
)
