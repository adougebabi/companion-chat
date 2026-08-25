"""Redis Streams adapter for committed events and ephemeral progress only."""

from __future__ import annotations

import json
from collections.abc import Awaitable, Callable, Iterable
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4

from sqlalchemy import and_, func, insert, select, update
from sqlalchemy.ext.asyncio import AsyncSession

EVENT_STREAM = "fluctlight:events:v1"
PROGRESS_STREAM = "fluctlight:progress:v1"
DURABLE_CONSUMER_GROUPS = ("bff-notifications", "cache-projections", "integration-observers")


class AggregateGapError(RuntimeError):
    """An event arrived before the next authoritative aggregate sequence."""


class AggregateSequenceError(RuntimeError):
    """An event sequence conflicts with an already-applied aggregate head."""


class RedisStreams:
    def __init__(self, client: Any) -> None:
        self.client = client

    async def publish_event(self, envelope: dict[str, str]) -> str:
        required = {"event_id", "event_type", "schema_version", "aggregate_sequence"}
        if missing := sorted(key for key in required if not envelope.get(key)):
            raise ValueError(f"event envelope missing: {', '.join(missing)}")
        return await self.client.xadd(EVENT_STREAM, envelope)

    async def ensure_group(self, group: str, *, start_id: str = "0") -> None:
        if not group.strip():
            raise ValueError("consumer group is required")
        try:
            await self.client.xgroup_create(EVENT_STREAM, group, id=start_id, mkstream=True)
        except Exception as exc:
            if "BUSYGROUP" not in str(exc):
                raise

    async def bootstrap_groups(self) -> None:
        for group in DURABLE_CONSUMER_GROUPS:
            await self.ensure_group(group)

    async def reclaim_pending(
        self, *, group: str, consumer: str, min_idle_ms: int = 30_000, count: int = 100
    ) -> list[tuple[str, dict[str, str]]]:
        if min_idle_ms < 0 or count < 1:
            raise ValueError("reclaim bounds are invalid")
        result = await self.client.xautoclaim(
            EVENT_STREAM,
            group,
            consumer,
            min_idle_ms,
            "0-0",
            count=count,
        )
        entries = result[1] if isinstance(result, tuple | list) and len(result) > 1 else []
        return [(stream_id, fields) for stream_id, fields in entries]

    async def read_pending_outbox(self, session: AsyncSession, *, limit: int = 100) -> list[Any]:
        """Read a committed outbox snapshot without performing external I/O."""

        from .schema import outbox_events

        if limit < 1:
            raise ValueError("outbox batch limit must be positive")
        rows = (
            (
                await session.execute(
                    select(outbox_events)
                    .where(
                        outbox_events.c.published_at.is_(None),
                        outbox_events.c.available_at <= datetime.now(UTC),
                    )
                    .order_by(outbox_events.c.occurred_at, outbox_events.c.id)
                    .limit(limit)
                )
            )
            .mappings()
            .all()
        )
        return [self._event_from_row(row) for row in rows]

    async def read_outbox_page(
        self, session: AsyncSession, *, offset: int = 0, limit: int = 500
    ) -> list[Any]:
        """Read a bounded PostgreSQL event-journal page for Redis replay."""

        from .schema import outbox_events

        if offset < 0 or limit < 1:
            raise ValueError("outbox page bounds are invalid")
        rows = (
            (
                await session.execute(
                    select(outbox_events)
                    .order_by(outbox_events.c.occurred_at, outbox_events.c.id)
                    .offset(offset)
                    .limit(limit)
                )
            )
            .mappings()
            .all()
        )
        return [self._event_from_row(row) for row in rows]

    async def publish_events(self, events: Iterable[Any]) -> int:
        """Publish an outbox snapshot after its database transaction is closed."""

        from .outbox import event_envelope

        published = 0
        for event in events:
            await self.publish_event(event_envelope(event))
            published += 1
        return published

    async def mark_published(self, session: AsyncSession, event_ids: Iterable[str]) -> int:
        """CAS-mark events after their Redis XADD calls have completed."""

        from .schema import outbox_events

        ids = tuple(event_ids)
        if not ids:
            return 0
        result = await session.execute(
            update(outbox_events)
            .where(outbox_events.c.id.in_(ids), outbox_events.c.published_at.is_(None))
            .values(published_at=datetime.now(UTC))
        )
        return int(result.rowcount or 0)

    @staticmethod
    def _event_from_row(row: Any) -> Any:
        from .outbox import OutboxEvent

        return OutboxEvent(
            id=row["id"],
            kind=row["kind"],
            aggregate_type=row["aggregate_type"],
            aggregate_id=row["aggregate_id"],
            causation_id=row["causation_id"],
            correlation_id=row["correlation_id"],
            idempotency_key=row["idempotency_key"],
            payload=dict(row["payload"]),
            attempt_policy=dict(row["attempt_policy"]),
            fluctlight_id=row["fluctlight_id"],
        )

    @staticmethod
    def _max_attempts(fields: dict[str, str]) -> int:
        try:
            policy = json.loads(fields.get("attempt_policy", "{}"))
        except json.JSONDecodeError:
            policy = {}
        value = policy.get("max_attempts", 3) if isinstance(policy, dict) else 3
        return max(1, int(value)) if isinstance(value, int | float) else 3

    async def record_delivery_failure(
        self,
        unit_of_work: Any,
        *,
        group: str,
        event_id: str,
        stream_id: str,
        fields: dict[str, str],
        error: BaseException,
    ) -> tuple[int, str]:
        """Persist a bounded attempt record outside the failed business transaction."""

        from .schema import consumer_failures

        max_attempts = self._max_attempts(fields)
        error_code = type(error).__name__[:128] or "consumer_failure"
        async with unit_of_work.begin(command_id=f"redis-failure:{group}:{event_id}") as tx:
            previous_row = (
                (
                    await tx.session.execute(
                        select(consumer_failures.c.attempt, consumer_failures.c.status)
                        .where(
                            consumer_failures.c.consumer_group == group,
                            consumer_failures.c.event_id == event_id,
                        )
                        .order_by(consumer_failures.c.attempt.desc())
                        .limit(1)
                    )
                )
                .mappings()
                .one_or_none()
            )
            previous_value = previous_row["attempt"] if previous_row else None
            previous: int | None = int(previous_value) if previous_value is not None else None
            if previous_row and previous_row["status"] == "gap":
                assert previous is not None
                return previous, "gap"
            # The persisted attempt number is authoritative across consumer replacement.
            if previous is None:
                previous = await tx.session.scalar(
                    select(func.max(consumer_failures.c.attempt)).where(
                        consumer_failures.c.consumer_group == group,
                        consumer_failures.c.event_id == event_id,
                    )
                )
            attempt = int(previous or 0) + 1
            status = (
                "gap"
                if isinstance(error, AggregateGapError)
                else "quarantined"
                if attempt >= max_attempts
                else "retryable"
            )
            await tx.session.execute(
                insert(consumer_failures).values(
                    id=f"consumer_failure_{uuid4().hex}",
                    consumer_group=group,
                    event_id=event_id,
                    stream_id=stream_id,
                    attempt=attempt,
                    max_attempts=max_attempts,
                    status=status,
                    error_code=error_code,
                    details={"exception": error_code},
                )
            )
            await tx.commit()
        return attempt, status

    async def consume_transactional(
        self,
        *,
        group: str,
        consumer: str,
        unit_of_work: Any,
        handler: Callable[[AsyncSession, str, dict[str, str]], Awaitable[dict[str, Any]]],
        reclaim_idle_ms: int = 30_000,
    ) -> int:
        """Apply inbox/effect in PostgreSQL before acknowledging Redis delivery."""

        from .outbox import begin_inbox_once, complete_inbox
        from .schema import consumer_heads

        await self.ensure_group(group)
        reclaimed = await self.reclaim_pending(
            group=group,
            consumer=consumer,
            min_idle_ms=reclaim_idle_ms,
        )
        messages = [(EVENT_STREAM, reclaimed)]
        messages.extend(
            await self.client.xreadgroup(group, consumer, {EVENT_STREAM: ">"}, count=10)
        )
        applied = 0
        for _, entries in messages:
            for stream_id, fields in entries:
                event_id = fields.get("event_id", stream_id)
                try:
                    async with unit_of_work.begin(command_id=f"redis-event:{event_id}") as tx:
                        claimed, existing = await begin_inbox_once(
                            tx.session,
                            consumer_group=group,
                            event_id=event_id,
                        )
                        if claimed:
                            (
                                aggregate_type,
                                aggregate_id,
                                sequence,
                                head,
                            ) = await self._validate_sequence(tx.session, group, fields)
                            result = await handler(tx.session, event_id, fields)
                            await complete_inbox(
                                tx.session,
                                consumer_group=group,
                                event_id=event_id,
                                result=result,
                            )
                            if head is None:
                                await tx.session.execute(
                                    insert(consumer_heads).values(
                                        consumer_group=group,
                                        aggregate_type=aggregate_type,
                                        aggregate_id=aggregate_id,
                                        last_sequence=sequence,
                                    )
                                )
                            else:
                                await tx.session.execute(
                                    update(consumer_heads)
                                    .where(
                                        and_(
                                            consumer_heads.c.consumer_group == group,
                                            consumer_heads.c.aggregate_type == aggregate_type,
                                            consumer_heads.c.aggregate_id == aggregate_id,
                                        )
                                    )
                                    .values(
                                        last_sequence=sequence,
                                        updated_at=datetime.now(UTC),
                                    )
                                )
                        elif existing and existing.get("status") != "processing":
                            result = existing
                        else:
                            raise RuntimeError("consumer inbox entry is still processing")
                        await tx.commit()
                except Exception as exc:
                    _, status = await self.record_delivery_failure(
                        unit_of_work,
                        group=group,
                        event_id=event_id,
                        stream_id=stream_id,
                        fields=fields,
                        error=exc,
                    )
                    if status != "quarantined":
                        continue
                    await self.client.xack(EVENT_STREAM, group, stream_id)
                    applied += 1
                    continue
                await self.client.xack(EVENT_STREAM, group, stream_id)
                applied += 1
        return applied

    @staticmethod
    async def _validate_sequence(
        session: AsyncSession, group: str, fields: dict[str, str]
    ) -> tuple[str, str, int, Any | None]:
        from .schema import consumer_heads

        aggregate_type = fields.get("aggregate_type", "").strip()
        aggregate_id = fields.get("aggregate_id", "").strip()
        try:
            sequence = int(fields.get("aggregate_sequence", "0"))
        except ValueError as exc:
            raise AggregateSequenceError("aggregate sequence is not an integer") from exc
        if not aggregate_type or not aggregate_id or sequence < 1:
            raise AggregateSequenceError("aggregate identity or sequence is invalid")
        head = (
            (
                await session.execute(
                    select(consumer_heads)
                    .where(
                        consumer_heads.c.consumer_group == group,
                        consumer_heads.c.aggregate_type == aggregate_type,
                        consumer_heads.c.aggregate_id == aggregate_id,
                    )
                    .with_for_update()
                )
            )
            .mappings()
            .one_or_none()
        )
        if head is None:
            if sequence != 1:
                raise AggregateGapError(f"aggregate sequence gap: expected 1, received {sequence}")
            return aggregate_type, aggregate_id, sequence, None
        expected = int(head["last_sequence"]) + 1
        if sequence > expected:
            raise AggregateGapError(
                f"aggregate sequence gap: expected {expected}, received {sequence}"
            )
        if sequence < expected:
            raise AggregateSequenceError(
                f"aggregate sequence conflict: expected {expected}, received {sequence}"
            )
        return aggregate_type, aggregate_id, sequence, head

    async def publish_progress(self, payload: dict[str, str], *, maxlen: int = 1_000) -> str:
        return await self.client.xadd(PROGRESS_STREAM, payload, maxlen=maxlen, approximate=True)

    async def consume_once(
        self,
        *,
        group: str,
        consumer: str,
        handler: Callable[[str, dict[str, str]], Awaitable[None]],
    ) -> int:
        messages = await self.client.xreadgroup(group, consumer, {EVENT_STREAM: ">"}, count=10)
        applied = 0
        for _, entries in messages:
            for stream_id, fields in entries:
                await handler(stream_id, fields)
                await self.client.xack(EVENT_STREAM, group, stream_id)
                applied += 1
        return applied
