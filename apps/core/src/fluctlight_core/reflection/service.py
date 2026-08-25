"""Apply typed reflection proposals through Memory/Relationship public ports."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from datetime import UTC, datetime

from fluctlight_core.cognition.contracts import ReflectionProposal
from fluctlight_core.memory.contracts import MemoryRecord, MemoryType, MemoryVisibility
from fluctlight_core.memory.service import MemoryService
from fluctlight_core.relationships.contracts import RelationshipTrend, RelationshipUpdate
from fluctlight_core.relationships.service import RelationshipService


@dataclass(frozen=True, slots=True)
class ReflectionApplyResult:
    memory_ids: tuple[str, ...]
    relationship_ids: tuple[str, ...]


class ReflectionCoordinator:
    """No semantic inference: only validates and routes explicit candidates."""

    def __init__(self, memory: MemoryService, relationships: RelationshipService) -> None:
        self._memory = memory
        self._relationships = relationships

    async def apply(self, proposal: ReflectionProposal) -> ReflectionApplyResult:
        memory_ids: list[str] = []
        relationship_ids: list[str] = []
        payload = proposal.payload
        for raw in payload.get("memory_candidates", []):
            if not isinstance(raw, Mapping):
                raise ValueError("memory candidate must be an object")
            memory = MemoryRecord(
                id=str(raw["id"]),
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
            await self._memory.record(memory)
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
            )
            relationship = await self._relationships.record_update(update)
            relationship_ids.append(relationship.id)
        return ReflectionApplyResult(tuple(memory_ids), tuple(relationship_ids))
