"""Directed Relationship persistence, CAS revisions and append-only governance."""

from __future__ import annotations

from collections.abc import Callable
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import insert, select, update

from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .contracts import RelationshipSnapshot, RelationshipTrend, RelationshipUpdate


class RelationshipService:
    def __init__(
        self, unit_of_work: UnitOfWorkFactory, *, clock: Callable[[], datetime] | None = None
    ) -> None:
        self._unit_of_work = unit_of_work
        self._clock = clock or (lambda: datetime.now(UTC))

    async def read(
        self, owner_fluctlight_id: str, target_actor_id: str
    ) -> RelationshipSnapshot | None:
        async with self._unit_of_work.begin(
            command_id=f"relationship-read:{owner_fluctlight_id}:{target_actor_id}"
        ) as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.relationships).where(
                            schema.relationships.c.owner_fluctlight_id == owner_fluctlight_id,
                            schema.relationships.c.target_actor_id == target_actor_id,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
        return self._from_row(row) if row else None

    async def list_for_fluctlight(self, owner_fluctlight_id: str) -> list[RelationshipSnapshot]:
        async with self._unit_of_work.begin(
            command_id=f"relationship-list:{owner_fluctlight_id}"
        ) as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.relationships)
                        .where(schema.relationships.c.owner_fluctlight_id == owner_fluctlight_id)
                        .order_by(schema.relationships.c.updated_at.desc())
                    )
                )
                .mappings()
                .all()
            )
        return [self._from_row(row) for row in rows]

    async def record_update(self, command: RelationshipUpdate) -> RelationshipSnapshot:
        now = self._clock()
        async with self._unit_of_work.begin(
            command_id=f"relationship-update:{command.idempotency_key}"
        ) as tx:
            prior_revision = (
                (
                    await tx.session.execute(
                        select(schema.relationship_revisions).where(
                            schema.relationship_revisions.c.idempotency_key
                            == command.idempotency_key
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if prior_revision is not None:
                prior = (
                    (
                        await tx.session.execute(
                            select(schema.relationships).where(
                                schema.relationships.c.id == prior_revision["relationship_id"]
                            )
                        )
                    )
                    .mappings()
                    .one()
                )
                if (
                    prior["owner_fluctlight_id"] != command.owner_fluctlight_id
                    or prior["target_actor_id"] != command.target_actor_id
                ):
                    raise ValueError("relationship idempotency key targets another relationship")
                return self._from_row(prior)
            row = (
                (
                    await tx.session.execute(
                        select(schema.relationships)
                        .where(
                            schema.relationships.c.owner_fluctlight_id
                            == command.owner_fluctlight_id,
                            schema.relationships.c.target_actor_id == command.target_actor_id,
                        )
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                relationship_id = f"relationship_{uuid4().hex}"
                await tx.session.execute(
                    insert(schema.relationships).values(
                        id=relationship_id,
                        owner_fluctlight_id=command.owner_fluctlight_id,
                        target_actor_id=command.target_actor_id,
                        metrics=dict(command.metrics),
                        interaction_frequency=0.0,
                        trend=RelationshipTrend.STABLE.value,
                        summary=None,
                        emotional_association={},
                        revision=0,
                        updated_at=now,
                    )
                )
                row = {
                    "id": relationship_id,
                    "owner_fluctlight_id": command.owner_fluctlight_id,
                    "target_actor_id": command.target_actor_id,
                    "metrics": dict(command.metrics),
                    "interaction_frequency": 0.0,
                    "last_interaction_at": None,
                    "last_meaningful_interaction_at": None,
                    "trend": RelationshipTrend.STABLE.value,
                    "summary": None,
                    "emotional_association": {},
                    "revision": 0,
                    "updated_at": now,
                }
            elif int(row["revision"]) != command.expected_revision:
                raise ValueError("relationship revision is stale")
            next_revision = int(row["revision"]) + 1
            await tx.session.execute(
                update(schema.relationships)
                .where(
                    schema.relationships.c.id == row["id"],
                    schema.relationships.c.revision == int(row["revision"]),
                )
                .values(
                    metrics=dict(command.metrics),
                    trend=command.trend.value,
                    summary=command.summary,
                    emotional_association=dict(command.emotional_association),
                    revision=next_revision,
                    last_interaction_at=now,
                    updated_at=now,
                )
            )
            revision_id = f"relationship_revision_{uuid4().hex}"
            await tx.session.execute(
                insert(schema.relationship_revisions).values(
                    id=revision_id,
                    relationship_id=row["id"],
                    revision=next_revision,
                    base_revision=int(row["revision"]),
                    metrics=dict(command.metrics),
                    trend=command.trend.value,
                    summary=command.summary,
                    emotional_association=dict(command.emotional_association),
                    evidence_refs=list(command.evidence_refs),
                    actor_id=command.actor_id,
                    idempotency_key=command.idempotency_key,
                    created_at=now,
                )
            )
            await tx.session.execute(
                insert(schema.relationship_governance).values(
                    id=f"relationship_governance_{uuid4().hex}",
                    relationship_id=row["id"],
                    revision_id=revision_id,
                    action="updated",
                    actor_id=command.actor_id,
                    reason=command.summary,
                    created_at=now,
                )
            )
            await tx.commit()
        return RelationshipSnapshot(
            id=row["id"],
            owner_fluctlight_id=command.owner_fluctlight_id,
            target_actor_id=command.target_actor_id,
            metrics=command.metrics,
            interaction_frequency=float(row["interaction_frequency"]),
            last_interaction_at=now,
            last_meaningful_interaction_at=row["last_meaningful_interaction_at"],
            trend=command.trend,
            summary=command.summary,
            emotional_association=command.emotional_association,
            revision=next_revision,
            updated_at=now,
        )

    async def rollback(
        self,
        owner_fluctlight_id: str,
        target_actor_id: str,
        *,
        target_revision: int,
        expected_revision: int,
        actor_id: str,
        evidence_refs: tuple[str, ...],
    ) -> RelationshipSnapshot:
        async with self._unit_of_work.begin(
            command_id=f"relationship-rollback:{owner_fluctlight_id}:{target_revision}"
        ) as tx:
            relationship = (
                (
                    await tx.session.execute(
                        select(schema.relationships)
                        .where(
                            schema.relationships.c.owner_fluctlight_id == owner_fluctlight_id,
                            schema.relationships.c.target_actor_id == target_actor_id,
                        )
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if relationship is None or int(relationship["revision"]) != expected_revision:
                raise ValueError("relationship revision is stale")
            target = (
                (
                    await tx.session.execute(
                        select(schema.relationship_revisions).where(
                            schema.relationship_revisions.c.relationship_id == relationship["id"],
                            schema.relationship_revisions.c.revision == target_revision,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if target is None:
                raise KeyError(target_revision)
            command = RelationshipUpdate(
                owner_fluctlight_id,
                target_actor_id,
                dict(target["metrics"]),
                tuple(evidence_refs) + (f"rollback:{target_revision}",),
                actor_id,
                expected_revision,
                RelationshipTrend(target["trend"]),
                target["summary"],
                dict(target["emotional_association"]),
            )
        return await self.record_update(command)

    @staticmethod
    def _from_row(row: Any) -> RelationshipSnapshot:
        return RelationshipSnapshot(
            id=row["id"],
            owner_fluctlight_id=row["owner_fluctlight_id"],
            target_actor_id=row["target_actor_id"],
            metrics=dict(row["metrics"]),
            interaction_frequency=float(row["interaction_frequency"]),
            last_interaction_at=row["last_interaction_at"],
            last_meaningful_interaction_at=row["last_meaningful_interaction_at"],
            trend=RelationshipTrend(row["trend"]),
            summary=row["summary"],
            emotional_association=dict(row["emotional_association"]),
            revision=int(row["revision"]),
            updated_at=row["updated_at"],
        )
