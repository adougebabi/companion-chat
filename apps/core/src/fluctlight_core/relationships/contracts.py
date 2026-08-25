"""Directed Relationship state and revision contracts."""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from math import isfinite
from typing import Any
from uuid import uuid4


class RelationshipTrend(StrEnum):
    IMPROVING = "improving"
    STABLE = "stable"
    DECLINING = "declining"


METRIC_NAMES = (
    "intimacy",
    "trust",
    "familiarity",
    "attachment",
    "respect",
    "affection",
    "annoyance",
    "psychological_safety",
    "dependence",
)


def _text(value: str, name: str, limit: int = 512) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    value = value.strip()
    if len(value) > limit:
        raise ValueError(f"{name} exceeds {limit} characters")
    return value


def _metrics(values: Mapping[str, float]) -> dict[str, float]:
    result = {name: float(values.get(name, 0.0)) for name in METRIC_NAMES}
    if any(not isfinite(value) or not 0.0 <= value <= 1.0 for value in result.values()):
        raise ValueError("relationship metrics must be finite and between 0 and 1")
    unknown = set(values) - set(METRIC_NAMES)
    if unknown:
        raise ValueError(f"unknown relationship metrics: {sorted(unknown)}")
    return result


def _refs(values: Sequence[str]) -> tuple[str, ...]:
    result = tuple(_text(value, "evidence_ref", 512) for value in values)
    if not result or len(result) != len(set(result)):
        raise ValueError("relationship evidence references must be non-empty and unique")
    return result


@dataclass(frozen=True, slots=True)
class RelationshipSnapshot:
    id: str
    owner_fluctlight_id: str
    target_actor_id: str
    metrics: Mapping[str, float]
    interaction_frequency: float
    last_interaction_at: datetime | None
    last_meaningful_interaction_at: datetime | None
    trend: RelationshipTrend
    summary: str | None
    emotional_association: Mapping[str, Any]
    revision: int = 0
    updated_at: datetime = field(default_factory=lambda: datetime.now(UTC))

    def __post_init__(self) -> None:
        for name in ("id", "owner_fluctlight_id", "target_actor_id"):
            object.__setattr__(self, name, _text(getattr(self, name), name, 128))
        object.__setattr__(self, "metrics", _metrics(self.metrics))
        if not isfinite(self.interaction_frequency) or not 0.0 <= self.interaction_frequency <= 1.0:
            raise ValueError("interaction_frequency must be between 0 and 1")
        object.__setattr__(self, "trend", RelationshipTrend(self.trend))
        if self.summary is not None:
            object.__setattr__(self, "summary", _text(self.summary, "summary", 4096))
        if not isinstance(self.emotional_association, Mapping):
            raise ValueError("emotional_association must be an object")
        if self.revision < 0:
            raise ValueError("relationship revision cannot be negative")


@dataclass(frozen=True, slots=True)
class RelationshipUpdate:
    owner_fluctlight_id: str
    target_actor_id: str
    metrics: Mapping[str, float]
    evidence_refs: tuple[str, ...]
    actor_id: str
    expected_revision: int
    trend: RelationshipTrend = RelationshipTrend.STABLE
    summary: str | None = None
    emotional_association: Mapping[str, Any] = field(default_factory=dict)
    idempotency_key: str = field(default_factory=lambda: f"relationship_update_{uuid4().hex}")

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "owner_fluctlight_id", _text(self.owner_fluctlight_id, "owner_fluctlight_id", 128)
        )
        object.__setattr__(
            self, "target_actor_id", _text(self.target_actor_id, "target_actor_id", 128)
        )
        object.__setattr__(self, "actor_id", _text(self.actor_id, "actor_id", 128))
        object.__setattr__(self, "metrics", _metrics(self.metrics))
        object.__setattr__(self, "evidence_refs", _refs(self.evidence_refs))
        if self.expected_revision < 0:
            raise ValueError("expected_revision cannot be negative")
        object.__setattr__(self, "trend", RelationshipTrend(self.trend))
