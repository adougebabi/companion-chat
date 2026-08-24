"""Application service for Fluctlight lifecycle and revision governance."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import replace
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import insert, select, update

from fluctlight_core.platform.persistence import UnitOfWork, UnitOfWorkFactory

from . import schema
from .contracts import (
    PERSONALITY_FIELDS,
    BehavioralPolicy,
    CreateFluctlight,
    FluctlightSnapshot,
    FluctlightStatus,
    FoundationRevision,
    FoundationRevisionRequest,
    Identity,
    InitializationMode,
    Personality,
    PersonalityUpdatePolicy,
    RevisionSource,
    RevisionStatus,
)
from .policy import RevisionConflictError, apply_changes, validate_revision


class FluctlightNotFoundError(LookupError):
    """Raised when a Fluctlight or revision does not exist."""


class FluctlightLifecycleError(RuntimeError):
    """Raised when a lifecycle transition is not allowed."""


def _parse_datetime(value: datetime | str | None, field_name: str) -> datetime:
    if value is None:
        raise FluctlightLifecycleError(f"{field_name} is missing from a persisted row")
    if isinstance(value, datetime):
        return value
    return datetime.fromisoformat(value)


def _parse_optional_datetime(value: datetime | str | None) -> datetime | None:
    if value is None:
        return None
    return value if isinstance(value, datetime) else datetime.fromisoformat(value)


def _identity_from_payload(payload: dict[str, Any]) -> Identity:
    return Identity(
        id=payload["id"],
        name=payload.get("name"),
        age=payload.get("age"),
        gender=payload.get("gender"),
        occupation=payload.get("occupation"),
        residence=payload.get("residence"),
        timezone=payload.get("timezone"),
        birthday=payload.get("birthday"),
        background=payload.get("background"),
        biography=payload.get("biography"),
        core_values=tuple(payload.get("core_values", ())),
        worldview=payload.get("worldview"),
        notes=payload.get("notes"),
    )


def _personality_from_payload(payload: dict[str, Any]) -> Personality:
    values = dict(payload)
    update_policy = values.pop("update_policy", {})
    values["update_policy"] = PersonalityUpdatePolicy(**update_policy)
    return Personality(**values)


def _snapshot_from_row(row: Any) -> FluctlightSnapshot:
    return FluctlightSnapshot(
        id=row["id"],
        initialization_mode=InitializationMode(row["initialization_mode"]),
        status=FluctlightStatus(row["status"]),
        identity=_identity_from_payload(dict(row["identity"])),
        personality=_personality_from_payload(dict(row["personality"])),
        behavioral_policy=BehavioralPolicy(**dict(row["behavioral_policy"])),
        current_revision=row["current_revision"],
        created_at=_parse_datetime(row["created_at"], "created_at"),
        updated_at=_parse_datetime(row["updated_at"], "updated_at"),
        retired_at=(
            _parse_datetime(row["retired_at"], "retired_at")
            if row["retired_at"] is not None
            else None
        ),
    )


class FluctlightService:
    """Own lifecycle persistence while exposing only domain value objects."""

    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    @asynccontextmanager
    async def _transaction(
        self, tx: UnitOfWork | None, command_id: str
    ) -> AsyncIterator[UnitOfWork]:
        if tx is not None:
            yield tx
            return
        async with self._unit_of_work.begin(command_id=command_id) as owned:
            yield owned
            await owned.commit()

    async def create(
        self, command: CreateFluctlight, *, tx: UnitOfWork | None = None
    ) -> FluctlightSnapshot:
        now = datetime.now(UTC)
        snapshot = FluctlightSnapshot(
            id=command.id,
            initialization_mode=command.initialization_mode,
            status=FluctlightStatus.ACTIVE,
            identity=command.identity or Identity(id=command.id),
            personality=command.personality,
            behavioral_policy=command.behavioral_policy,
            current_revision=0,
            created_at=now,
            updated_at=now,
        )
        async with self._transaction(tx, f"fluctlight-create:{uuid4()}") as tx:
            existing = await tx.session.scalar(
                select(schema.fluctlights.c.id).where(schema.fluctlights.c.id == snapshot.id)
            )
            if existing is not None:
                raise FluctlightLifecycleError("fluctlight already exists")
            await tx.session.execute(
                insert(schema.fluctlights).values(
                    id=snapshot.id,
                    created_by_actor_id=command.actor_id,
                    initialization_mode=snapshot.initialization_mode.value,
                    status=snapshot.status.value,
                    current_revision=0,
                    identity=snapshot.identity.as_payload(),
                    personality=snapshot.personality.as_payload(),
                    behavioral_policy=snapshot.behavioral_policy.as_payload(),
                    created_at=now,
                    updated_at=now,
                )
            )
            await tx.session.execute(
                insert(schema.foundation_revisions).values(
                    id=f"foundation_revision_{uuid4().hex}",
                    fluctlight_id=snapshot.id,
                    revision=0,
                    base_revision=0,
                    source=RevisionSource.INITIALIZATION.value,
                    status=RevisionStatus.ACCEPTED.value,
                    actor_id=command.actor_id,
                    initialization_mode=snapshot.initialization_mode.value,
                    foundation_status=snapshot.status.value,
                    foundation_created_at=snapshot.created_at,
                    confidence=1.0,
                    changes={},
                    identity=snapshot.identity.as_payload(),
                    personality=snapshot.personality.as_payload(),
                    behavioral_policy=snapshot.behavioral_policy.as_payload(),
                    evidence_refs=[],
                    idempotency_key=f"fluctlight-create:{snapshot.id}",
                    created_at=now,
                    accepted_at=now,
                )
            )
        return snapshot

    async def get(self, fluctlight_id: str, *, tx: UnitOfWork | None = None) -> FluctlightSnapshot:
        async with self._transaction(tx, f"fluctlight-read:{uuid4()}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.fluctlights).where(schema.fluctlights.c.id == fluctlight_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
        if row is None:
            raise FluctlightNotFoundError(fluctlight_id)
        return _snapshot_from_row(row)

    async def submit_revision(
        self, request: FoundationRevisionRequest, *, tx: UnitOfWork | None = None
    ) -> FoundationRevision:
        now = request.requested_at
        async with self._transaction(tx, f"fluctlight-submit:{request.idempotency_key}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.fluctlights)
                        .where(schema.fluctlights.c.id == request.fluctlight_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                raise FluctlightNotFoundError(request.fluctlight_id)
            snapshot = _snapshot_from_row(row)
            existing = (
                (
                    await tx.session.execute(
                        select(schema.foundation_revisions).where(
                            schema.foundation_revisions.c.idempotency_key == request.idempotency_key
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                if (
                    existing["fluctlight_id"] != request.fluctlight_id
                    or existing["actor_id"] != request.actor_id
                    or existing["changes"] != dict(request.changes)
                ):
                    raise FluctlightLifecycleError(
                        "idempotency key was reused for another revision"
                    )
                return _revision_from_row(existing)
            personality_rows = (
                await tx.session.execute(
                    select(
                        schema.foundation_revisions.c.created_at,
                        schema.foundation_revisions.c.changes,
                    ).where(
                        schema.foundation_revisions.c.fluctlight_id == request.fluctlight_id,
                        schema.foundation_revisions.c.status == RevisionStatus.ACCEPTED.value,
                    )
                )
            ).mappings()
            last_personality_revision_at = None
            for previous in personality_rows:
                if set(dict(previous["changes"])) & PERSONALITY_FIELDS:
                    candidate_time = _parse_datetime(previous["created_at"], "created_at")
                    if (
                        last_personality_revision_at is None
                        or candidate_time > last_personality_revision_at
                    ):
                        last_personality_revision_at = candidate_time
            validate_revision(
                snapshot,
                request,
                now=now,
                evidence_event_count=len(request.evidence_refs),
                last_personality_revision_at=last_personality_revision_at,
            )
            candidate = apply_changes(
                snapshot,
                request.changes,
                revision=snapshot.current_revision + 1,
                now=now,
            )
            revision_id = f"foundation_revision_{uuid4().hex}"
            await tx.session.execute(
                insert(schema.foundation_revisions).values(
                    id=revision_id,
                    fluctlight_id=request.fluctlight_id,
                    revision=candidate.current_revision,
                    base_revision=snapshot.current_revision,
                    source=request.source.value,
                    status=RevisionStatus.PROPOSED.value,
                    actor_id=request.actor_id,
                    initialization_mode=snapshot.initialization_mode.value,
                    foundation_status=snapshot.status.value,
                    foundation_created_at=snapshot.created_at,
                    confidence=request.confidence,
                    changes=dict(request.changes),
                    identity=candidate.identity.as_payload(),
                    personality=candidate.personality.as_payload(),
                    behavioral_policy=candidate.behavioral_policy.as_payload(),
                    evidence_refs=list(request.evidence_refs),
                    idempotency_key=request.idempotency_key,
                    created_at=now,
                )
            )
        return FoundationRevision(
            id=revision_id,
            fluctlight_id=request.fluctlight_id,
            revision=candidate.current_revision,
            base_revision=snapshot.current_revision,
            source=request.source,
            status=RevisionStatus.PROPOSED,
            actor_id=request.actor_id,
            changes=request.changes,
            evidence_refs=request.evidence_refs,
            candidate=candidate,
            idempotency_key=request.idempotency_key,
            created_at=now,
            confidence=request.confidence,
        )

    async def accept_revision(
        self,
        *,
        revision_id: str,
        actor_id: str,
        expected_revision: int,
        tx: UnitOfWork | None = None,
    ) -> FluctlightSnapshot:
        now = datetime.now(UTC)
        async with self._transaction(tx, f"fluctlight-accept:{revision_id}") as tx:
            revision_row = await self._revision_row(tx, revision_id, for_update=True)
            if revision_row["status"] == RevisionStatus.ACCEPTED.value:
                return _snapshot_from_revision_row(revision_row)
            if revision_row["status"] != RevisionStatus.PROPOSED.value:
                raise FluctlightLifecycleError("only a proposed revision can be accepted")
            if revision_row["base_revision"] != expected_revision:
                raise RevisionConflictError("revision acceptance expected revision is stale")
            current_row = await self._fluctlight_row(
                tx, revision_row["fluctlight_id"], for_update=True
            )
            if current_row["current_revision"] != expected_revision:
                raise RevisionConflictError("foundation revision changed before acceptance")
            if actor_id != revision_row["actor_id"]:
                raise FluctlightLifecycleError("only the revision actor may accept this revision")
            candidate = _snapshot_from_revision_row(revision_row)
            result = await tx.session.execute(
                update(schema.fluctlights)
                .where(
                    schema.fluctlights.c.id == revision_row["fluctlight_id"],
                    schema.fluctlights.c.current_revision == expected_revision,
                )
                .values(
                    current_revision=candidate.current_revision,
                    identity=candidate.identity.as_payload(),
                    personality=candidate.personality.as_payload(),
                    behavioral_policy=candidate.behavioral_policy.as_payload(),
                    updated_at=now,
                )
            )
            if result.rowcount != 1:
                raise RevisionConflictError("foundation acceptance lost compare-and-set")
            await tx.session.execute(
                update(schema.foundation_revisions)
                .where(schema.foundation_revisions.c.id == revision_id)
                .values(status=RevisionStatus.ACCEPTED.value, accepted_at=now)
            )
            await tx.session.execute(
                insert(schema.foundation_governance).values(
                    id=f"foundation_governance_{uuid4().hex}",
                    fluctlight_id=revision_row["fluctlight_id"],
                    revision_id=revision_id,
                    action="accept",
                    actor_id=actor_id,
                    created_at=now,
                )
            )
        return replace(candidate, updated_at=now)

    async def reject_revision(
        self,
        *,
        revision_id: str,
        actor_id: str,
        reason: str,
        tx: UnitOfWork | None = None,
    ) -> FoundationRevision:
        now = datetime.now(UTC)
        async with self._transaction(tx, f"fluctlight-reject:{revision_id}") as tx:
            row = await self._revision_row(tx, revision_id, for_update=True)
            if row["status"] != RevisionStatus.PROPOSED.value:
                raise FluctlightLifecycleError("only a proposed revision can be rejected")
            if actor_id != row["actor_id"]:
                raise FluctlightLifecycleError("only the revision actor may reject this revision")
            await tx.session.execute(
                update(schema.foundation_revisions)
                .where(schema.foundation_revisions.c.id == revision_id)
                .values(status=RevisionStatus.REJECTED.value, rejected_at=now)
            )
            await tx.session.execute(
                insert(schema.foundation_governance).values(
                    id=f"foundation_governance_{uuid4().hex}",
                    fluctlight_id=row["fluctlight_id"],
                    revision_id=revision_id,
                    action="reject",
                    actor_id=actor_id,
                    reason=reason[:1024],
                    created_at=now,
                )
            )
        return replace(_revision_from_row(row), status=RevisionStatus.REJECTED)

    async def rollback_revision(
        self,
        *,
        fluctlight_id: str,
        target_revision: int,
        actor_id: str,
        expected_revision: int,
        reason: str,
        tx: UnitOfWork | None = None,
    ) -> FluctlightSnapshot:
        now = datetime.now(UTC)
        async with self._transaction(
            tx, f"fluctlight-rollback:{fluctlight_id}:{target_revision}"
        ) as tx:
            current_row = await self._fluctlight_row(tx, fluctlight_id, for_update=True)
            if current_row["current_revision"] != expected_revision:
                raise RevisionConflictError("rollback expected revision is stale")
            target_row = (
                (
                    await tx.session.execute(
                        select(schema.foundation_revisions).where(
                            schema.foundation_revisions.c.fluctlight_id == fluctlight_id,
                            schema.foundation_revisions.c.revision == target_revision,
                            schema.foundation_revisions.c.status == RevisionStatus.ACCEPTED.value,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if target_row is None:
                raise FluctlightNotFoundError(f"accepted revision {target_revision}")
            current = _snapshot_from_row(current_row)
            target = _snapshot_from_revision_row(target_row)
            candidate = replace(
                target,
                id=fluctlight_id,
                initialization_mode=current.initialization_mode,
                status=current.status,
                current_revision=expected_revision + 1,
                created_at=current.created_at,
                updated_at=now,
                retired_at=current.retired_at,
            )
            revision_id = f"foundation_revision_{uuid4().hex}"
            await tx.session.execute(
                update(schema.fluctlights)
                .where(
                    schema.fluctlights.c.id == fluctlight_id,
                    schema.fluctlights.c.current_revision == expected_revision,
                )
                .values(
                    current_revision=candidate.current_revision,
                    identity=candidate.identity.as_payload(),
                    personality=candidate.personality.as_payload(),
                    behavioral_policy=candidate.behavioral_policy.as_payload(),
                    updated_at=now,
                )
            )
            await tx.session.execute(
                insert(schema.foundation_revisions).values(
                    id=revision_id,
                    fluctlight_id=fluctlight_id,
                    revision=candidate.current_revision,
                    base_revision=expected_revision,
                    source=RevisionSource.ROLLBACK.value,
                    status=RevisionStatus.ACCEPTED.value,
                    actor_id=actor_id,
                    initialization_mode=candidate.initialization_mode.value,
                    foundation_status=candidate.status.value,
                    foundation_created_at=candidate.created_at,
                    confidence=1.0,
                    changes={"rollback_to": target_revision},
                    identity=candidate.identity.as_payload(),
                    personality=candidate.personality.as_payload(),
                    behavioral_policy=candidate.behavioral_policy.as_payload(),
                    evidence_refs=[f"revision:{target_revision}"],
                    idempotency_key=f"rollback:{fluctlight_id}:{expected_revision}:{target_revision}",
                    created_at=now,
                    accepted_at=now,
                )
            )
            await tx.session.execute(
                insert(schema.foundation_governance).values(
                    id=f"foundation_governance_{uuid4().hex}",
                    fluctlight_id=fluctlight_id,
                    revision_id=revision_id,
                    action="rollback",
                    actor_id=actor_id,
                    reason=reason[:1024],
                    created_at=now,
                )
            )
        return candidate

    async def retire(
        self,
        *,
        fluctlight_id: str,
        actor_id: str,
        expected_revision: int,
        reason: str,
        tx: UnitOfWork | None = None,
    ) -> FluctlightSnapshot:
        now = datetime.now(UTC)
        async with self._transaction(tx, f"fluctlight-retire:{fluctlight_id}") as tx:
            current = await self._fluctlight_row(tx, fluctlight_id, for_update=True)
            if current["status"] == FluctlightStatus.RETIRED.value:
                return _snapshot_from_row(current)
            if current["current_revision"] != expected_revision:
                raise RevisionConflictError("retirement expected revision is stale")
            accepted_revision_id = await tx.session.scalar(
                select(schema.foundation_revisions.c.id).where(
                    schema.foundation_revisions.c.fluctlight_id == fluctlight_id,
                    schema.foundation_revisions.c.revision == expected_revision,
                    schema.foundation_revisions.c.status == RevisionStatus.ACCEPTED.value,
                )
            )
            if accepted_revision_id is None:
                raise FluctlightLifecycleError("current foundation revision is not auditable")
            result = await tx.session.execute(
                update(schema.fluctlights)
                .where(
                    schema.fluctlights.c.id == fluctlight_id,
                    schema.fluctlights.c.current_revision == expected_revision,
                )
                .values(status=FluctlightStatus.RETIRED.value, retired_at=now, updated_at=now)
            )
            if result.rowcount != 1:
                raise RevisionConflictError("retirement lost compare-and-set")
            await tx.session.execute(
                insert(schema.foundation_governance).values(
                    id=f"foundation_governance_{uuid4().hex}",
                    fluctlight_id=fluctlight_id,
                    revision_id=accepted_revision_id,
                    action="retire",
                    actor_id=actor_id,
                    reason=reason[:1024],
                    created_at=now,
                )
            )
        return replace(
            _snapshot_from_row(current),
            status=FluctlightStatus.RETIRED,
            retired_at=now,
            updated_at=now,
        )

    async def _fluctlight_row(self, tx, fluctlight_id: str, *, for_update: bool = False):
        statement = select(schema.fluctlights).where(schema.fluctlights.c.id == fluctlight_id)
        if for_update:
            statement = statement.with_for_update()
        row = (await tx.session.execute(statement)).mappings().one_or_none()
        if row is None:
            raise FluctlightNotFoundError(fluctlight_id)
        return row

    async def _revision_row(self, tx, revision_id: str, *, for_update: bool = False):
        statement = select(schema.foundation_revisions).where(
            schema.foundation_revisions.c.id == revision_id
        )
        if for_update:
            statement = statement.with_for_update()
        row = (await tx.session.execute(statement)).mappings().one_or_none()
        if row is None:
            raise FluctlightNotFoundError(revision_id)
        return row


def _snapshot_from_revision_row(row: Any) -> FluctlightSnapshot:
    return FluctlightSnapshot(
        id=row["fluctlight_id"],
        initialization_mode=InitializationMode(row["initialization_mode"]),
        status=FluctlightStatus(row["foundation_status"]),
        identity=_identity_from_payload(dict(row["identity"])),
        personality=_personality_from_payload(dict(row["personality"])),
        behavioral_policy=BehavioralPolicy(**dict(row["behavioral_policy"])),
        current_revision=row["revision"],
        created_at=_parse_datetime(row["foundation_created_at"], "foundation_created_at"),
        updated_at=_parse_datetime(row.get("accepted_at") or row["created_at"], "updated_at"),
    )


def _revision_from_row(row: Any) -> FoundationRevision:
    candidate = _snapshot_from_revision_row(row)
    return FoundationRevision(
        id=row["id"],
        fluctlight_id=row["fluctlight_id"],
        revision=row["revision"],
        base_revision=row["base_revision"],
        source=RevisionSource(row["source"]),
        status=RevisionStatus(row["status"]),
        actor_id=row["actor_id"],
        changes=dict(row["changes"]),
        evidence_refs=tuple(row["evidence_refs"]),
        candidate=candidate,
        idempotency_key=row["idempotency_key"],
        created_at=_parse_datetime(row["created_at"], "created_at"),
        accepted_at=_parse_optional_datetime(row.get("accepted_at")),
        confidence=float(row.get("confidence", 1.0)),
    )
