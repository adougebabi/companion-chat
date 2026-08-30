"""Apply typed reflection proposals through Memory/Relationship public ports."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from hashlib import sha256

from fluctlight_core.cognition.contracts import ReflectionProposal
from fluctlight_core.memory.contracts import MemoryRecord, MemoryType, MemoryVisibility
from fluctlight_core.memory.service import MemoryService
from fluctlight_core.platform.persistence import UnitOfWork
from fluctlight_core.relationships.contracts import RelationshipTrend, RelationshipUpdate
from fluctlight_core.relationships.service import RelationshipService


class ReflectionValidationError(ValueError):
    """Raised when a Provider proposal is not a typed reflection contract."""


def _required_text(value: object, field: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ReflectionValidationError(f"reflection {field} is required")
    return value.strip()


def _bounded_number(value: object, field: str) -> float:
    if isinstance(value, bool) or not isinstance(value, int | float):
        raise ReflectionValidationError(f"reflection {field} must be numeric")
    number = float(value)
    if not 0.0 <= number <= 1.0:
        raise ReflectionValidationError(f"reflection {field} must be between 0 and 1")
    return number


def _string_tuple(value: object, field: str) -> tuple[str, ...]:
    if not isinstance(value, (list, tuple)) or not all(
        isinstance(item, str) and item.strip() for item in value
    ):
        raise ReflectionValidationError(f"reflection {field} must be an array of text")
    return tuple(str(item).strip() for item in value)


def validate_reflection_payload(payload: Mapping[str, object]) -> None:
    """Validate every candidate before any Memory/Relationship write occurs."""

    if not isinstance(payload, Mapping):
        raise ReflectionValidationError("reflection payload must be an object")
    for collection_name in ("memory_candidates", "relationship_candidates"):
        candidates = payload.get(collection_name, [])
        if not isinstance(candidates, list):
            raise ReflectionValidationError(f"reflection {collection_name} must be an array")
        for index, raw in enumerate(candidates):
            if not isinstance(raw, Mapping):
                raise ReflectionValidationError(
                    f"reflection {collection_name}[{index}] must be an object"
                )
            if collection_name == "memory_candidates":
                memory_type = _required_text(raw.get("type"), f"memory_candidates[{index}].type")
                try:
                    MemoryType(memory_type)
                except ValueError as exc:
                    raise ReflectionValidationError(
                        f"reflection memory_candidates[{index}].type is unsupported"
                    ) from exc
                _required_text(raw.get("content"), f"memory_candidates[{index}].content")
                _bounded_number(raw.get("confidence"), f"memory_candidates[{index}].confidence")
                for field in ("actor_refs", "event_refs", "evidence_refs"):
                    if field in raw:
                        _string_tuple(raw[field], f"memory_candidates[{index}].{field}")
                for field in ("importance", "emotional_significance"):
                    if field in raw:
                        _bounded_number(raw[field], f"memory_candidates[{index}].{field}")
                visibility = raw.get("visibility")
                if visibility is not None:
                    visibility_value = _required_text(
                        visibility, f"memory_candidates[{index}].visibility"
                    )
                    try:
                        MemoryVisibility(visibility_value)
                    except ValueError as exc:
                        raise ReflectionValidationError(
                            f"reflection memory_candidates[{index}].visibility is unsupported"
                        ) from exc
                occurred_at = raw.get("occurred_at")
                if occurred_at is not None:
                    if not isinstance(occurred_at, str):
                        raise ReflectionValidationError(
                            f"reflection memory_candidates[{index}].occurred_at must be text"
                        )
                    try:
                        parsed = datetime.fromisoformat(occurred_at)
                    except ValueError as exc:
                        raise ReflectionValidationError(
                            f"reflection memory_candidates[{index}].occurred_at is invalid"
                        ) from exc
                    if parsed.tzinfo is None or parsed.utcoffset() is None:
                        raise ReflectionValidationError(
                            "reflection "
                            f"memory_candidates[{index}].occurred_at must be timezone-aware"
                        )
            else:
                _required_text(
                    raw.get("target_actor_id"),
                    f"relationship_candidates[{index}].target_actor_id",
                )
                metrics = raw.get("metrics")
                if not isinstance(metrics, Mapping) or not metrics:
                    raise ReflectionValidationError(
                        f"reflection relationship_candidates[{index}].metrics must be an object"
                    )
                for metric, value in metrics.items():
                    _bounded_number(value, f"relationship_candidates[{index}].metrics.{metric}")
                expected_revision = raw.get("expected_revision", 0)
                if (
                    isinstance(expected_revision, bool)
                    or not isinstance(expected_revision, int)
                    or expected_revision < 0
                ):
                    raise ReflectionValidationError(
                        f"reflection relationship_candidates[{index}].expected_revision is invalid"
                    )
                for field in ("evidence_refs",):
                    if field in raw:
                        _string_tuple(raw[field], f"relationship_candidates[{index}].{field}")
                trend = raw.get("trend")
                if trend is not None:
                    trend_value = _required_text(
                        trend, f"relationship_candidates[{index}].trend"
                    )
                    try:
                        RelationshipTrend(trend_value)
                    except ValueError as exc:
                        raise ReflectionValidationError(
                            f"reflection relationship_candidates[{index}].trend is unsupported"
                        ) from exc


@dataclass(frozen=True, slots=True)
class ReflectionApplyResult:
    memory_ids: tuple[str, ...]
    relationship_ids: tuple[str, ...]


class ReflectionCoordinator:
    """No semantic inference: only validates and routes explicit candidates."""

    def __init__(self, memory: MemoryService, relationships: RelationshipService) -> None:
        self._memory = memory
        self._relationships = relationships

    async def apply(
        self, proposal: ReflectionProposal, *, tx: UnitOfWork | None = None
    ) -> ReflectionApplyResult:
        memory_ids: list[str] = []
        relationship_ids: list[str] = []
        payload = proposal.payload
        validate_reflection_payload(payload)
        for index, raw in enumerate(payload.get("memory_candidates", [])):
            if not isinstance(raw, Mapping):
                raise ValueError("memory candidate must be an object")
            candidate_id = raw.get("id")
            if not isinstance(candidate_id, str) or not candidate_id.strip():
                candidate_id = (
                    "memory_reflection_"
                    + sha256(f"{proposal.proposal_id}:{index}".encode()).hexdigest()[:32]
                )
            memory = MemoryRecord(
                id=candidate_id,
                owner_fluctlight_id=proposal.fluctlight_id,
                type=MemoryType(raw["type"]),
                content=str(raw["content"]),
                actor_refs=tuple(raw.get("actor_refs", ())),
                conversation_id=raw.get("conversation_id"),
                event_refs=tuple(raw.get("event_refs", ())),
                evidence_refs=tuple(raw.get("evidence_refs", proposal.evidence_refs)),
                confidence=float(raw["confidence"]),
                importance=float(raw.get("importance", 0.5)),
                emotional_significance=float(raw.get("emotional_significance", 0.0)),
                visibility=MemoryVisibility(raw.get("visibility", MemoryVisibility.PRIVATE.value)),
                occurred_at=datetime.fromisoformat(raw["occurred_at"])
                if raw.get("occurred_at")
                else None,
                created_at=datetime.now(UTC),
            )
            if tx is None:
                await self._memory.record(memory)
            else:
                await self._memory.record(memory, tx=tx)
            memory_ids.append(memory.id)
        for raw in payload.get("relationship_candidates", []):
            if not isinstance(raw, Mapping):
                raise ValueError("relationship candidate must be an object")
            update = RelationshipUpdate(
                owner_fluctlight_id=proposal.fluctlight_id,
                target_actor_id=str(raw["target_actor_id"]),
                metrics=dict(raw["metrics"]),
                evidence_refs=tuple(raw.get("evidence_refs", proposal.evidence_refs)),
                actor_id=str(raw.get("actor_id", proposal.fluctlight_id)),
                expected_revision=int(raw.get("expected_revision", 0)),
                trend=RelationshipTrend(raw.get("trend", RelationshipTrend.STABLE.value)),
                summary=raw.get("summary"),
                emotional_association=dict(raw.get("emotional_association", {})),
                idempotency_key=f"reflection:{proposal.proposal_id}:relationship:{len(relationship_ids)}",
            )
            if tx is None:
                relationship = await self._relationships.record_update(update)
            else:
                relationship = await self._relationships.record_update(update, tx=tx)
            relationship_ids.append(relationship.id)
        return ReflectionApplyResult(tuple(memory_ids), tuple(relationship_ids))
