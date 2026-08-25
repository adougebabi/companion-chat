"""Committed workflow intents and outbox/inbox primitives."""

from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from typing import Any

from sqlalchemy import insert, select, update
from sqlalchemy.dialects.postgresql import insert as pg_insert
from sqlalchemy.ext.asyncio import AsyncSession

from .schema import consumer_inbox, outbox_events, workflow_intents


@dataclass(frozen=True, slots=True)
class CommittedWorkflowIntent:
    intent_id: str
    workflow_id: str
    task_queue: str
    intent_type: str
    payload: dict[str, Any]


@dataclass(frozen=True, slots=True)
class OutboxEvent:
    id: str
    kind: str
    aggregate_type: str
    aggregate_id: str
    causation_id: str
    correlation_id: str
    idempotency_key: str
    payload: dict[str, Any]
    attempt_policy: dict[str, Any]
    fluctlight_id: str | None = None


async def commit_workflow_intent(
    session: AsyncSession, intent: CommittedWorkflowIntent
) -> CommittedWorkflowIntent:
    statement = (
        pg_insert(workflow_intents)
        .values(**asdict(intent))
        .on_conflict_do_nothing(index_elements=[workflow_intents.c.intent_id])
        .returning(workflow_intents)
    )
    row = (await session.execute(statement)).mappings().one_or_none()
    if row is None:
        existing = (
            (
                await session.execute(
                    select(workflow_intents).where(workflow_intents.c.intent_id == intent.intent_id)
                )
            )
            .mappings()
            .one()
        )
        return CommittedWorkflowIntent(
            intent_id=existing["intent_id"],
            workflow_id=existing["workflow_id"],
            task_queue=existing["task_queue"],
            intent_type=existing["intent_type"],
            payload=dict(existing["payload"]),
        )
    return intent


async def add_outbox_event(session: AsyncSession, event: OutboxEvent) -> None:
    sequence = event.payload.get("aggregate_sequence")
    if not isinstance(sequence, int) or sequence < 1:
        raise ValueError("outbox event requires a positive aggregate_sequence")
    await session.execute(insert(outbox_events).values(**asdict(event)))


async def claim_inbox_once(
    session: AsyncSession, *, consumer_group: str, event_id: str, result: dict[str, Any]
) -> bool:
    statement = (
        pg_insert(consumer_inbox)
        .values(consumer_group=consumer_group, event_id=event_id, result=result)
        .on_conflict_do_nothing(constraint="consumer_event")
        .returning(consumer_inbox.c.id)
    )
    return (await session.execute(statement)).scalar_one_or_none() is not None


async def begin_inbox_once(
    session: AsyncSession, *, consumer_group: str, event_id: str
) -> tuple[bool, dict[str, Any] | None]:
    """Reserve an event before applying its effect in the same transaction."""

    statement = (
        pg_insert(consumer_inbox)
        .values(
            consumer_group=consumer_group,
            event_id=event_id,
            result={"status": "processing"},
        )
        .on_conflict_do_nothing(constraint="consumer_event")
        .returning(consumer_inbox.c.id)
    )
    inserted = (await session.execute(statement)).scalar_one_or_none() is not None
    if inserted:
        return True, None
    existing = (
        (
            await session.execute(
                select(consumer_inbox.c.result)
                .where(
                    consumer_inbox.c.consumer_group == consumer_group,
                    consumer_inbox.c.event_id == event_id,
                )
                .with_for_update()
            )
        )
        .mappings()
        .one_or_none()
    )
    return False, dict(existing["result"]) if existing else None


async def complete_inbox(
    session: AsyncSession,
    *,
    consumer_group: str,
    event_id: str,
    result: dict[str, Any],
) -> None:
    await session.execute(
        update(consumer_inbox)
        .where(
            consumer_inbox.c.consumer_group == consumer_group,
            consumer_inbox.c.event_id == event_id,
        )
        .values(result=result)
    )


def event_envelope(event: OutboxEvent) -> dict[str, str]:
    sequence = event.payload.get("aggregate_sequence")
    if not isinstance(sequence, int) or sequence < 1:
        raise ValueError("outbox event requires a positive aggregate_sequence")
    return {
        "event_id": event.id,
        "event_type": event.kind,
        "schema_version": "v1",
        "aggregate_type": event.aggregate_type,
        "aggregate_id": event.aggregate_id,
        "aggregate_sequence": str(sequence),
        "fluctlight_id": event.fluctlight_id or "",
        "causation_id": event.causation_id,
        "correlation_id": event.correlation_id,
        "occurred_at": datetime.now(UTC).isoformat(),
        "attempt_policy": json.dumps(event.attempt_policy, separators=(",", ":"), sort_keys=True),
        "payload": json.dumps(event.payload, separators=(",", ":"), sort_keys=True),
    }
