"""Framework-free typed Memory and retrieval contracts."""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from math import isfinite
from uuid import uuid4


class MemoryType(StrEnum):
    EPISODIC = "episodic"
    SEMANTIC = "semantic"
    RELATIONSHIP = "relationship"
    AUTOBIOGRAPHICAL = "autobiographical"


class MemoryVisibility(StrEnum):
    PRIVATE = "private"
    OWNER = "owner"
    PARTICIPANTS = "participants"


class MemoryStatus(StrEnum):
    ACTIVE = "active"
    SUPERSEDED = "superseded"
    FORGOTTEN = "forgotten"


class EmbeddingStatus(StrEnum):
    PENDING = "pending"
    READY = "ready"
    FAILED = "failed"
    STALE = "stale"


def _text(value: str, name: str, limit: int = 4096) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    value = value.strip()
    if len(value) > limit:
        raise ValueError(f"{name} exceeds {limit} characters")
    return value


def _refs(values: Sequence[str], name: str, required: bool = False) -> tuple[str, ...]:
    refs = tuple(_text(value, name, 512) for value in values)
    if required and not refs:
        raise ValueError(f"{name} requires at least one reference")
    if len(refs) != len(set(refs)):
        raise ValueError(f"{name} contains duplicate references")
    return refs


def _aware(value: datetime, name: str) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError(f"{name} must be timezone-aware")
    return value


def _bounded(value: float, name: str) -> float:
    if not isfinite(value) or not 0.0 <= value <= 1.0:
        raise ValueError(f"{name} must be finite and between 0 and 1")
    return value


@dataclass(frozen=True, slots=True)
class MemoryRecord:
    id: str
    owner_fluctlight_id: str
    type: MemoryType
    content: str
    actor_refs: tuple[str, ...]
    conversation_id: str | None
    event_refs: tuple[str, ...]
    evidence_refs: tuple[str, ...]
    confidence: float
    importance: float
    emotional_significance: float
    visibility: MemoryVisibility
    status: MemoryStatus = MemoryStatus.ACTIVE
    revision: int = 0
    occurred_at: datetime | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))
    last_confirmed_at: datetime | None = None

    def __post_init__(self) -> None:
        for name in ("id", "owner_fluctlight_id", "content"):
            object.__setattr__(self, name, _text(getattr(self, name), name))
        object.__setattr__(self, "type", MemoryType(self.type))
        object.__setattr__(self, "actor_refs", _refs(self.actor_refs, "actor_refs"))
        object.__setattr__(self, "event_refs", _refs(self.event_refs, "event_refs"))
        object.__setattr__(
            self, "evidence_refs", _refs(self.evidence_refs, "evidence_refs", required=True)
        )
        for name in ("confidence", "importance", "emotional_significance"):
            object.__setattr__(self, name, _bounded(getattr(self, name), name))
        object.__setattr__(self, "visibility", MemoryVisibility(self.visibility))
        object.__setattr__(self, "status", MemoryStatus(self.status))
        if self.revision < 0:
            raise ValueError("memory revision cannot be negative")
        for name in ("created_at", "occurred_at", "last_confirmed_at"):
            value = getattr(self, name)
            if value is not None:
                object.__setattr__(self, name, _aware(value, name))


@dataclass(frozen=True, slots=True)
class MemoryRevision:
    memory_id: str
    revision: int
    base_revision: int
    content: str
    status: MemoryStatus
    actor_id: str
    evidence_refs: tuple[str, ...]
    idempotency_key: str = field(default_factory=lambda: f"memory_revision_{uuid4().hex}")
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


@dataclass(frozen=True, slots=True)
class EmbeddingResult:
    memory_id: str
    revision: int
    model_id: str
    vector: tuple[float, ...]
    status: EmbeddingStatus = EmbeddingStatus.READY
    error_code: str | None = None

    def __post_init__(self) -> None:
        if self.revision < 0 or not self.model_id.strip() or not self.vector:
            raise ValueError("embedding identity and vector are required")
        if any(not isfinite(value) for value in self.vector):
            raise ValueError("embedding vector must contain finite values")
        object.__setattr__(self, "status", EmbeddingStatus(self.status))


@dataclass(frozen=True, slots=True)
class MemoryQuery:
    owner_fluctlight_id: str
    authorized_actor_ids: tuple[str, ...]
    conversation_scope: str | None = None
    allowed_types: tuple[MemoryType, ...] = ()
    statuses: tuple[MemoryStatus, ...] = (MemoryStatus.ACTIVE,)
    query_text: str | None = None
    query_embedding: tuple[float, ...] | None = None
    embedding_model_id: str | None = None
    limit: int = 20
    token_budget: int = 1600

    def __post_init__(self) -> None:
        object.__setattr__(
            self, "owner_fluctlight_id", _text(self.owner_fluctlight_id, "owner_fluctlight_id", 128)
        )
        object.__setattr__(
            self,
            "authorized_actor_ids",
            _refs(self.authorized_actor_ids, "authorized_actor_ids", required=True),
        )
        object.__setattr__(
            self, "allowed_types", tuple(MemoryType(item) for item in self.allowed_types)
        )
        object.__setattr__(self, "statuses", tuple(MemoryStatus(item) for item in self.statuses))
        if self.conversation_scope is not None:
            object.__setattr__(
                self,
                "conversation_scope",
                _text(self.conversation_scope, "conversation_scope", 128),
            )
        if self.query_text is not None:
            object.__setattr__(self, "query_text", _text(self.query_text, "query_text", 4096))
        if self.query_embedding is not None:
            if any(not isfinite(value) for value in self.query_embedding):
                raise ValueError("query embedding must contain finite values")
            if not self.embedding_model_id:
                raise ValueError("embedding_model_id is required with query_embedding")
        if self.limit < 1 or self.limit > 200 or self.token_budget < 1:
            raise ValueError("memory query limits are invalid")


@dataclass(frozen=True, slots=True)
class MemoryContextItem:
    id: str
    type: MemoryType
    content: str
    confidence: float
    evidence_refs: tuple[str, ...]
    source_refs: tuple[str, ...]
    revision: int


def cosine_similarity(left: Sequence[float], right: Sequence[float]) -> float:
    if len(left) != len(right) or not left:
        return 0.0
    dot = sum(a * b for a, b in zip(left, right, strict=True))
    left_norm = sum(a * a for a in left) ** 0.5
    right_norm = sum(b * b for b in right) ** 0.5
    if left_norm == 0 or right_norm == 0:
        return 0.0
    return dot / (left_norm * right_norm)
