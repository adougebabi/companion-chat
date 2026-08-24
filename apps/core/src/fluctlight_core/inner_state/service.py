"""Application persistence service for inner state, goals, and intentions."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import insert, select, update

from fluctlight_core.platform.persistence import UnitOfWork, UnitOfWorkFactory

from . import schema
from .contracts import (
    PAD,
    DriveConflict,
    DriveState,
    EventTrigger,
    Goal,
    GoalEvidence,
    GoalSource,
    GoalStatus,
    InnerStateSnapshot,
    InnerStateTransition,
    Intention,
    IntentionEvidence,
    IntentionGovernance,
    IntentionStatus,
    Momentum,
    Mood,
    Regulation,
    SemanticAssessment,
    SemanticTrigger,
    TimeTrigger,
    TriggerType,
)
from .policy import (
    NumericPolicyError,
    NumericStatePolicy,
    govern_intention,
    propose_goal,
    propose_intention,
    qualify_intention,
    transition_goal,
)


class InnerStateNotFoundError(LookupError):
    """Raised when an inner-state, goal, or intention does not exist."""


def _dt(value: datetime | str | None) -> datetime | None:
    if value is None:
        return None
    return value if isinstance(value, datetime) else datetime.fromisoformat(value)


def _snapshot_from_payload(payload: dict[str, Any]) -> InnerStateSnapshot:
    mood_payload = payload.get("mood", {})
    return InnerStateSnapshot(
        fluctlight_id=payload["fluctlight_id"],
        pad=PAD(**payload.get("pad", {})),
        mood=Mood(
            label=mood_payload.get("label"),
            intensity=mood_payload.get("intensity", 0.0),
            source=mood_payload.get("source", "regulation"),
            started_at=_dt(mood_payload.get("started_at")),
            expected_decay_at=_dt(mood_payload.get("expected_decay_at")),
        ),
        momentum=Momentum(**payload.get("momentum", {})),
        regulation=Regulation(**payload.get("regulation", {})),
        drives=tuple(DriveState(**item) for item in payload.get("drives", ())),
        conflicts=tuple(DriveConflict(**item) for item in payload.get("conflicts", ())),
        revision=payload.get("revision", 0),
        last_updated_at=_dt(payload["last_updated_at"]) or datetime.now(UTC),
    )


def _snapshot_from_row(row: Any) -> InnerStateSnapshot:
    return InnerStateSnapshot(
        fluctlight_id=row["fluctlight_id"],
        pad=PAD(**dict(row["pad"])),
        mood=Mood(
            label=row["mood"].get("label"),
            intensity=row["mood"].get("intensity", 0.0),
            source=row["mood"].get("source", "regulation"),
            started_at=_dt(row["mood"].get("started_at")),
            expected_decay_at=_dt(row["mood"].get("expected_decay_at")),
        ),
        momentum=Momentum(**dict(row["momentum"])),
        regulation=Regulation(**dict(row["regulation"])),
        drives=tuple(DriveState(**item) for item in row["drives"]),
        conflicts=tuple(DriveConflict(**item) for item in row["conflicts"]),
        revision=row["revision"],
        last_updated_at=row["last_updated_at"],
    )


def _goal_from_row(row: Any) -> Goal:
    return Goal(
        id=row["id"],
        fluctlight_id=row["fluctlight_id"],
        source=GoalSource(row["source"]),
        description=row["description"],
        importance=float(row["importance"]),
        urgency=float(row["urgency"]),
        progress=float(row["progress"]),
        status=GoalStatus(row["status"]),
        evidence_refs=tuple(row["evidence_refs"]),
        revision=row["revision"],
        deadline=row["deadline"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
    )


def _trigger_from_payload(payload: dict[str, Any]):
    trigger_type = TriggerType(payload["type"])
    if trigger_type == TriggerType.TIME:
        return TimeTrigger(at=datetime.fromisoformat(payload["at"]))
    if trigger_type == TriggerType.EVENT:
        return EventTrigger(event_type=payload["event_type"], event_id=payload.get("event_id"))
    return SemanticTrigger(
        schema_version=payload["schema_version"], evidence_refs=tuple(payload["evidence_refs"])
    )


def _intention_from_row(row: Any) -> Intention:
    return Intention(
        id=row["id"],
        fluctlight_id=row["fluctlight_id"],
        goal_id=row["goal_id"],
        action=row["action"],
        preferred_time=row["preferred_time"],
        trigger=_trigger_from_payload(dict(row["trigger"])),
        confidence=float(row["confidence"]),
        expiration=row["expiration"],
        evidence_refs=tuple(row["evidence_refs"]),
        permission_snapshot=dict(row["permission_snapshot"]),
        budget_snapshot=dict(row["budget_snapshot"]),
        status=IntentionStatus(row["status"]),
        revision=row["revision"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
    )


def _transition_from_row(row: Any) -> InnerStateTransition:
    return InnerStateTransition(
        previous=_snapshot_from_payload(dict(row["previous_state"])),
        current=_snapshot_from_payload(dict(row["resulting_state"])),
        result=row["result"],
        reason_code=row["reason_code"],
        policy_version=row["policy_version"],
        model_version=row["model_version"],
        requested_delta=dict(row["requested_delta"]),
        applied_delta=dict(row["applied_delta"]),
        idempotency_key=row["idempotency_key"],
        source_event_id=row["source_event_id"],
        evidence_refs=tuple(row["evidence_refs"]),
    )


class InnerStateService:
    """Persist state transitions without exposing SQLAlchemy rows."""

    def __init__(
        self, unit_of_work: UnitOfWorkFactory, *, policy: NumericStatePolicy | None = None
    ) -> None:
        self._unit_of_work = unit_of_work
        self._policy = policy or NumericStatePolicy()

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

    async def initialize(
        self,
        fluctlight_id: str,
        *,
        snapshot: InnerStateSnapshot | None = None,
        tx: UnitOfWork | None = None,
    ) -> InnerStateSnapshot:
        current = snapshot or InnerStateSnapshot(fluctlight_id=fluctlight_id)
        if current.fluctlight_id != fluctlight_id:
            raise NumericPolicyError("snapshot targets a different Fluctlight")
        async with self._transaction(tx, f"inner-state-init:{fluctlight_id}") as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.inner_states).where(
                            schema.inner_states.c.fluctlight_id == fluctlight_id
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                return _snapshot_from_row(existing)
            await tx.session.execute(
                insert(schema.inner_states).values(
                    fluctlight_id=fluctlight_id,
                    revision=current.revision,
                    pad=current.pad.as_payload(),
                    mood=current.mood.as_payload(),
                    momentum=current.momentum.as_payload(),
                    regulation=current.regulation.as_payload(),
                    drives=[item.as_payload() for item in current.drives],
                    conflicts=[item.as_payload() for item in current.conflicts],
                    last_updated_at=current.last_updated_at,
                )
            )
        return current

    async def read(self, fluctlight_id: str, *, tx: UnitOfWork | None = None) -> InnerStateSnapshot:
        async with self._transaction(tx, f"inner-state-read:{fluctlight_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.inner_states).where(
                            schema.inner_states.c.fluctlight_id == fluctlight_id
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
        if row is None:
            raise InnerStateNotFoundError(fluctlight_id)
        return _snapshot_from_row(row)

    async def apply_assessment(
        self,
        fluctlight_id: str,
        assessment: SemanticAssessment,
        *,
        expected_revision: int,
        tx: UnitOfWork | None = None,
    ) -> InnerStateTransition:
        async with self._transaction(tx, f"inner-state-apply:{assessment.idempotency_key}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.inner_states)
                        .where(schema.inner_states.c.fluctlight_id == fluctlight_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                raise InnerStateNotFoundError(fluctlight_id)
            snapshot = _snapshot_from_row(row)
            existing = (
                (
                    await tx.session.execute(
                        select(schema.inner_state_events).where(
                            schema.inner_state_events.c.idempotency_key
                            == assessment.idempotency_key
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                if (
                    existing["fluctlight_id"] != fluctlight_id
                    or existing["source_event_id"] != assessment.source_event_id
                ):
                    raise NumericPolicyError("idempotency key was reused for another state event")
                return _transition_from_row(existing)
            transition = self._policy.apply_semantic_assessment(
                snapshot,
                assessment,
                expected_revision=expected_revision,
            )
            if transition.current.fluctlight_id != fluctlight_id:
                raise NumericPolicyError("assessment changed the target Fluctlight")
            result = await tx.session.execute(
                update(schema.inner_states)
                .where(
                    schema.inner_states.c.fluctlight_id == fluctlight_id,
                    schema.inner_states.c.revision == snapshot.revision,
                )
                .values(
                    revision=transition.current.revision,
                    pad=transition.current.pad.as_payload(),
                    mood=transition.current.mood.as_payload(),
                    momentum=transition.current.momentum.as_payload(),
                    regulation=transition.current.regulation.as_payload(),
                    drives=[item.as_payload() for item in transition.current.drives],
                    conflicts=[item.as_payload() for item in transition.current.conflicts],
                    last_updated_at=transition.current.last_updated_at,
                )
            )
            if result.rowcount != 1:
                raise NumericPolicyError("inner-state compare-and-set failed")
            await tx.session.execute(
                insert(schema.inner_state_events).values(
                    id=f"inner_state_event_{uuid4().hex}",
                    fluctlight_id=fluctlight_id,
                    source_event_id=transition.source_event_id,
                    expected_revision=snapshot.revision,
                    resulting_revision=transition.current.revision,
                    previous_state=snapshot.as_payload(),
                    resulting_state=transition.current.as_payload(),
                    requested_delta=dict(transition.requested_delta),
                    applied_delta=dict(transition.applied_delta),
                    result=transition.result,
                    reason_code=transition.reason_code,
                    policy_version=transition.policy_version,
                    model_version=transition.model_version,
                    evidence_refs=list(transition.evidence_refs),
                    idempotency_key=transition.idempotency_key,
                )
            )
        return transition

    async def create_goal(self, evidence: GoalEvidence, *, tx: UnitOfWork | None = None) -> Goal:
        goal = propose_goal(evidence)
        async with self._transaction(tx, f"goal-create:{goal.id}") as tx:
            await tx.session.execute(
                insert(schema.goals).values(
                    id=goal.id,
                    fluctlight_id=goal.fluctlight_id,
                    source=goal.source.value,
                    description=goal.description,
                    importance=goal.importance,
                    urgency=goal.urgency,
                    progress=goal.progress,
                    deadline=goal.deadline,
                    status=goal.status.value,
                    evidence_refs=list(goal.evidence_refs),
                    revision=goal.revision,
                    created_at=goal.created_at,
                    updated_at=goal.updated_at,
                )
            )
        return goal

    async def transition_goal(
        self,
        goal_id: str,
        *,
        target: GoalStatus,
        expected_revision: int,
        actor_id: str = "system",
        reason: str | None = None,
        tx: UnitOfWork | None = None,
    ) -> Goal:
        now = datetime.now(UTC)
        async with self._transaction(tx, f"goal-transition:{goal_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.goals).where(schema.goals.c.id == goal_id).with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                raise InnerStateNotFoundError(goal_id)
            goal = _goal_from_row(row)
            if goal.revision != expected_revision:
                raise NumericPolicyError("goal revision is stale")
            next_goal = transition_goal(goal, target, now=now)
            await tx.session.execute(
                update(schema.goals)
                .where(schema.goals.c.id == goal_id, schema.goals.c.revision == expected_revision)
                .values(status=next_goal.status.value, revision=next_goal.revision, updated_at=now)
            )
            await tx.session.execute(
                insert(schema.goal_governance).values(
                    id=f"goal_governance_{uuid4().hex}",
                    goal_id=goal.id,
                    fluctlight_id=goal.fluctlight_id,
                    from_status=goal.status.value,
                    to_status=next_goal.status.value,
                    actor_id=actor_id,
                    reason=reason,
                )
            )
        return next_goal

    async def create_intention(
        self, evidence: IntentionEvidence, *, tx: UnitOfWork | None = None
    ) -> Intention:
        intention = propose_intention(evidence)
        async with self._transaction(tx, f"intention-create:{intention.id}") as tx:
            if intention.goal_id is not None:
                goal_owner = await tx.session.scalar(
                    select(schema.goals.c.fluctlight_id).where(
                        schema.goals.c.id == intention.goal_id,
                        schema.goals.c.fluctlight_id == intention.fluctlight_id,
                    )
                )
                if goal_owner is None:
                    raise InnerStateNotFoundError("intention goal is not owned by this Fluctlight")
            await tx.session.execute(
                insert(schema.intentions).values(
                    id=intention.id,
                    fluctlight_id=intention.fluctlight_id,
                    goal_id=intention.goal_id,
                    action=intention.action,
                    preferred_time=intention.preferred_time,
                    trigger=intention.trigger.as_payload(),
                    confidence=intention.confidence,
                    expiration=intention.expiration,
                    evidence_refs=list(intention.evidence_refs),
                    permission_snapshot=dict(intention.permission_snapshot),
                    budget_snapshot=dict(intention.budget_snapshot),
                    status=intention.status.value,
                    revision=intention.revision,
                    created_at=intention.created_at,
                    updated_at=intention.updated_at,
                )
            )
        return intention

    async def qualify_intention(
        self,
        intention_id: str,
        *,
        expected_revision: int,
        actor_id: str = "system",
        reason: str | None = None,
        tx: UnitOfWork | None = None,
    ) -> Intention:
        async with self._transaction(tx, f"intention-qualify:{intention_id}") as tx:
            row = await self._intention_row(tx, intention_id, for_update=True)
            intention = _intention_from_row(row)
            if intention.revision != expected_revision:
                raise NumericPolicyError("intention revision is stale")
            next_intention = qualify_intention(intention)
            await self._update_intention(tx, next_intention, expected_revision)
            await tx.session.execute(
                insert(schema.intention_governance).values(
                    id=f"intention_governance_{uuid4().hex}",
                    intention_id=intention.id,
                    fluctlight_id=intention.fluctlight_id,
                    from_status=intention.status.value,
                    to_status=next_intention.status.value,
                    actor_id=actor_id,
                    reason=reason,
                )
            )
        return next_intention

    async def govern_intention(
        self, command: IntentionGovernance, *, tx: UnitOfWork | None = None
    ) -> Intention:
        async with self._transaction(tx, f"intention-govern:{command.intention_id}") as tx:
            row = await self._intention_row(tx, command.intention_id, for_update=True)
            intention = _intention_from_row(row)
            if intention.revision != command.expected_revision:
                raise NumericPolicyError("intention revision is stale")
            next_intention = govern_intention(intention, command.action)
            await self._update_intention(tx, next_intention, command.expected_revision)
            await tx.session.execute(
                insert(schema.intention_governance).values(
                    id=f"intention_governance_{uuid4().hex}",
                    intention_id=intention.id,
                    fluctlight_id=intention.fluctlight_id,
                    from_status=intention.status.value,
                    to_status=next_intention.status.value,
                    actor_id=command.actor_id,
                    reason=command.reason,
                )
            )
        return next_intention

    async def _intention_row(self, tx, intention_id: str, *, for_update: bool = False):
        statement = select(schema.intentions).where(schema.intentions.c.id == intention_id)
        if for_update:
            statement = statement.with_for_update()
        row = (await tx.session.execute(statement)).mappings().one_or_none()
        if row is None:
            raise InnerStateNotFoundError(intention_id)
        return row

    async def _update_intention(self, tx, intention: Intention, expected_revision: int) -> None:
        result = await tx.session.execute(
            update(schema.intentions)
            .where(
                schema.intentions.c.id == intention.id,
                schema.intentions.c.revision == expected_revision,
            )
            .values(
                status=intention.status.value,
                revision=intention.revision,
                updated_at=intention.updated_at,
            )
        )
        if result.rowcount != 1:
            raise NumericPolicyError("intention compare-and-set failed")
