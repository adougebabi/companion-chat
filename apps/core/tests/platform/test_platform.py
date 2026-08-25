from __future__ import annotations

import asyncio
import json
from contextlib import asynccontextmanager

import pytest
from fluctlight_core.platform.configuration import PlatformSettings, RuntimeRole
from fluctlight_core.platform.outbox import OutboxEvent, event_envelope
from fluctlight_core.platform.redis_streams import (
    DURABLE_CONSUMER_GROUPS,
    EVENT_STREAM,
    AggregateGapError,
    AggregateSequenceError,
    RedisStreams,
)


def environment(role: str = "api") -> dict[str, str]:
    return {
        "FLUCTLIGHT_ENV": "test",
        "FLUCTLIGHT_ROLE": role,
        "DATABASE_URL": "postgresql://fluctlight:secret@postgres/fluctlight",
        "REDIS_URL": "redis://redis:6379/0",
        "S3_ENDPOINT": "http://minio:9000",
        "S3_REGION": "us-east-1",
        "S3_BUCKET": "fluctlight-media",
        "S3_ACCESS_KEY": "access",
        "S3_SECRET_KEY": "secret",
        "FLUCTLIGHT_CORE_SERVICE_KEY": "service-key",
        "FLUCTLIGHT_SETTINGS_KEY": "settings-key",
        "TEMPORAL_ADDRESS": "temporal:7233",
        "TEMPORAL_NAMESPACE": "default",
    }


def test_platform_settings_require_the_configured_role() -> None:
    settings = PlatformSettings.from_environ(environment())
    settings.require_role(RuntimeRole.API)
    assert settings.api_port == 8080


def test_outbox_envelope_uses_stable_json_payload() -> None:
    envelope = event_envelope(
        OutboxEvent(
            id="event-1",
            kind="platform.created",
            aggregate_type="platform",
            aggregate_id="platform-1",
            causation_id="command-1",
            correlation_id="correlation-1",
            idempotency_key="command-1",
            payload={"z": 1, "a": "value", "aggregate_sequence": 1},
            attempt_policy={"max_attempts": 3},
        )
    )
    assert json.loads(envelope["payload"]) == {"a": "value", "aggregate_sequence": 1, "z": 1}
    assert json.loads(envelope["attempt_policy"]) == {"max_attempts": 3}


def test_outbox_envelope_preserves_an_explicit_aggregate_sequence() -> None:
    envelope = event_envelope(
        OutboxEvent(
            id="event-sequence",
            kind="platform.updated",
            aggregate_type="platform",
            aggregate_id="platform-1",
            causation_id="command-1",
            correlation_id="correlation-1",
            idempotency_key="command-sequence",
            payload={"aggregate_sequence": 7},
            attempt_policy={"max_attempts": 3},
        )
    )
    assert envelope["aggregate_sequence"] == "7"


def test_outbox_envelope_rejects_a_missing_aggregate_sequence() -> None:
    with pytest.raises(ValueError, match="aggregate_sequence"):
        event_envelope(
            OutboxEvent(
                id="event-missing-sequence",
                kind="platform.updated",
                aggregate_type="platform",
                aggregate_id="platform-1",
                causation_id="command-1",
                correlation_id="correlation-1",
                idempotency_key="command-missing-sequence",
                payload={},
                attempt_policy={"max_attempts": 3},
            )
        )


def test_aggregate_sequence_requires_a_contiguous_head() -> None:
    class Result:
        def __init__(self, row):
            self.row = row

        def mappings(self):
            return self

        def one_or_none(self):
            return self.row

    class Session:
        def __init__(self, row):
            self.row = row

        async def execute(self, statement):
            return Result(self.row)

    with pytest.raises(AggregateGapError):
        asyncio.run(
            RedisStreams(FakeRedis())._validate_sequence(
                Session(None),
                "cache-projections",
                {"aggregate_type": "test", "aggregate_id": "a", "aggregate_sequence": "2"},
            )
        )
    with pytest.raises(AggregateSequenceError):
        asyncio.run(
            RedisStreams(FakeRedis())._validate_sequence(
                Session({"last_sequence": 2}),
                "cache-projections",
                {"aggregate_type": "test", "aggregate_id": "a", "aggregate_sequence": "2"},
            )
        )


def test_redis_stream_contract_exposes_group_and_reclaim_boundaries() -> None:
    assert hasattr(RedisStreams, "ensure_group")
    assert hasattr(RedisStreams, "reclaim_pending")
    assert hasattr(RedisStreams, "read_pending_outbox")
    assert hasattr(RedisStreams, "publish_events")
    assert hasattr(RedisStreams, "mark_published")
    assert "integration-observers" in DURABLE_CONSUMER_GROUPS
    assert hasattr(RedisStreams, "consume_transactional")


class FakeRedis:
    def __init__(self) -> None:
        self.acks: list[str] = []
        self.events: list[dict[str, str]] = []

    async def xgroup_create(self, stream: str, group: str, *, id: str, mkstream: bool) -> None:
        assert stream == EVENT_STREAM
        assert mkstream is True

    async def xreadgroup(self, group: str, consumer: str, streams: dict[str, str], *, count: int):
        assert streams == {EVENT_STREAM: ">"}
        return [
            (
                EVENT_STREAM,
                [
                    (
                        "1-0",
                        {
                            "event_id": "event-1",
                            "aggregate_type": "test",
                            "aggregate_id": "aggregate-1",
                            "aggregate_sequence": "1",
                            "attempt_policy": '{"max_attempts":3}',
                        },
                    )
                ],
            )
        ]

    async def xautoclaim(
        self, stream: str, group: str, consumer: str, min_idle_ms: int, start_id: str, *, count: int
    ):
        assert stream == EVENT_STREAM
        return ("0-0", [])

    async def xack(self, stream: str, group: str, stream_id: str) -> None:
        self.acks.append(stream_id)

    async def xadd(self, stream: str, envelope: dict[str, str]) -> str:
        self.events.append(envelope)
        return f"{len(self.events)}-0"


class FakeSession:
    def __init__(self, events: list[str]) -> None:
        self.events = events

    async def execute(self, statement) -> None:
        self.events.append("head")


class FakeTransaction:
    def __init__(self, events: list[str]) -> None:
        self.session = FakeSession(events)
        self.events = events

    async def commit(self) -> None:
        self.events.append("commit")


class FakeUnitOfWork:
    def __init__(self, events: list[str]) -> None:
        self.transaction = FakeTransaction(events)

    @asynccontextmanager
    async def begin(self, *, command_id: str):
        yield self.transaction


def test_transactional_consumer_commits_before_ack(monkeypatch: pytest.MonkeyPatch) -> None:
    import fluctlight_core.platform.outbox as outbox

    events: list[str] = []

    async def begin_inbox(*args, **kwargs) -> tuple[bool, dict[str, str] | None]:
        events.append("inbox")
        return True, None

    async def complete_inbox(*args, **kwargs) -> None:
        events.append("complete")

    monkeypatch.setattr(outbox, "begin_inbox_once", begin_inbox)
    monkeypatch.setattr(outbox, "complete_inbox", complete_inbox)

    async def validate_sequence(session, group, fields):
        return "test", "aggregate-1", 1, None

    monkeypatch.setattr(RedisStreams, "_validate_sequence", staticmethod(validate_sequence))
    redis = FakeRedis()
    streams = RedisStreams(redis)
    unit_of_work = FakeUnitOfWork(events)

    async def handler(session, event_id: str, fields: dict[str, str]) -> dict[str, str]:
        assert session is unit_of_work.transaction.session
        assert event_id == "event-1"
        events.append("handler")
        return {"applied": "true"}

    applied = asyncio.run(
        streams.consume_transactional(
            group="cache-projections",
            consumer="worker-1",
            unit_of_work=unit_of_work,
            handler=handler,
        )
    )

    assert applied == 1
    assert events == ["inbox", "handler", "complete", "head", "commit"]
    assert redis.acks == ["1-0"]


def test_transactional_consumer_skips_duplicate_effect_after_inbox_commit(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    import fluctlight_core.platform.outbox as outbox

    class DuplicateRedis(FakeRedis):
        async def xreadgroup(
            self, group: str, consumer: str, streams: dict[str, str], *, count: int
        ):
            return [
                (
                    EVENT_STREAM,
                    [
                        (
                            "1-0",
                            {
                                "event_id": "event-1",
                                "aggregate_type": "test",
                                "aggregate_id": "aggregate-1",
                                "aggregate_sequence": "1",
                                "attempt_policy": '{"max_attempts":3}',
                            },
                        ),
                        (
                            "2-0",
                            {
                                "event_id": "event-1",
                                "aggregate_type": "test",
                                "aggregate_id": "aggregate-1",
                                "aggregate_sequence": "1",
                                "attempt_policy": '{"max_attempts":3}',
                            },
                        ),
                    ],
                )
            ]

    events: list[str] = []
    seen = False
    stored: dict[str, str] | None = None

    async def begin_inbox(*args, **kwargs) -> tuple[bool, dict[str, str] | None]:
        nonlocal seen
        events.append("inbox")
        if seen:
            return False, stored
        seen = True
        return True, None

    async def complete_inbox(*args, **kwargs) -> None:
        nonlocal stored
        stored = {"applied": "true"}
        events.append("complete")

    monkeypatch.setattr(outbox, "begin_inbox_once", begin_inbox)
    monkeypatch.setattr(outbox, "complete_inbox", complete_inbox)

    async def validate_sequence(session, group, fields):
        return "test", fields["aggregate_id"], 1, None

    monkeypatch.setattr(RedisStreams, "_validate_sequence", staticmethod(validate_sequence))
    redis = DuplicateRedis()
    unit_of_work = FakeUnitOfWork(events)
    handler_calls = 0

    async def handler(session, event_id: str, fields: dict[str, str]) -> dict[str, str]:
        nonlocal handler_calls
        handler_calls += 1
        events.append("handler")
        return {"applied": "true"}

    applied = asyncio.run(
        RedisStreams(redis).consume_transactional(
            group="cache-projections",
            consumer="worker-1",
            unit_of_work=unit_of_work,
            handler=handler,
        )
    )

    assert applied == 2
    assert handler_calls == 1
    assert redis.acks == ["1-0", "2-0"]


def test_transactional_consumer_quarantines_after_delivery_attempt_limit(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    import fluctlight_core.platform.outbox as outbox

    class FailingRedis(FakeRedis):
        async def xreadgroup(
            self, group: str, consumer: str, streams: dict[str, str], *, count: int
        ):
            return [
                (
                    EVENT_STREAM,
                    [
                        (
                            "1-0",
                            {
                                "event_id": "event-failing",
                                "aggregate_type": "test",
                                "aggregate_id": "aggregate-failing",
                                "aggregate_sequence": "1",
                                "attempt_policy": '{"max_attempts": 3}',
                            },
                        )
                    ],
                )
            ]

    events: list[str] = []

    async def begin_inbox(*args, **kwargs) -> tuple[bool, dict[str, str] | None]:
        return True, None

    async def complete_inbox(*args, **kwargs) -> None:
        events.append("complete")

    monkeypatch.setattr(outbox, "begin_inbox_once", begin_inbox)

    async def validate_sequence(session, group, fields):
        return "test", fields["aggregate_id"], 1, None

    monkeypatch.setattr(RedisStreams, "_validate_sequence", staticmethod(validate_sequence))
    monkeypatch.setattr(outbox, "complete_inbox", complete_inbox)
    redis = FailingRedis()
    streams = RedisStreams(redis)

    async def record_failure(*args, **kwargs) -> tuple[int, str]:
        return 3, "quarantined"

    monkeypatch.setattr(streams, "record_delivery_failure", record_failure)

    async def handler(session, event_id: str, fields: dict[str, str]) -> dict[str, str]:
        raise RuntimeError("bounded failure")

    applied = asyncio.run(
        streams.consume_transactional(
            group="cache-projections",
            consumer="worker-1",
            unit_of_work=FakeUnitOfWork(events),
            handler=handler,
        )
    )

    assert applied == 1
    assert redis.acks == ["1-0"]


def test_transactional_consumer_leaves_retryable_failure_pending(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    import fluctlight_core.platform.outbox as outbox

    class FailingRedis(FakeRedis):
        async def xreadgroup(
            self, group: str, consumer: str, streams: dict[str, str], *, count: int
        ):
            return [
                (
                    EVENT_STREAM,
                    [
                        (
                            "1-0",
                            {
                                "event_id": "event-retry",
                                "aggregate_type": "test",
                                "aggregate_id": "aggregate-retry",
                                "aggregate_sequence": "1",
                                "attempt_policy": '{"max_attempts": 3}',
                            },
                        )
                    ],
                )
            ]

    async def begin_inbox(*args, **kwargs) -> tuple[bool, dict[str, str] | None]:
        return True, None

    monkeypatch.setattr(outbox, "begin_inbox_once", begin_inbox)
    redis = FailingRedis()
    streams = RedisStreams(redis)

    async def record_failure(*args, **kwargs) -> tuple[int, str]:
        return 1, "retryable"

    monkeypatch.setattr(streams, "record_delivery_failure", record_failure)

    async def handler(session, event_id: str, fields: dict[str, str]) -> dict[str, str]:
        raise RuntimeError("retryable failure")

    applied = asyncio.run(
        streams.consume_transactional(
            group="cache-projections",
            consumer="worker-1",
            unit_of_work=FakeUnitOfWork([]),
            handler=handler,
        )
    )

    assert applied == 0
    assert redis.acks == []


def test_replay_publishes_an_outbox_snapshot_after_the_read_transaction() -> None:
    class Result:
        def __init__(self, rows):
            self.rows = rows

        def mappings(self):
            return self

        def all(self):
            return self.rows

    class Session:
        def __init__(self, rows):
            self.rows = [rows, []]

        async def execute(self, statement):
            return Result(self.rows.pop(0))

    rows = [
        {
            "id": "event-published",
            "kind": "memory.embedding.requested",
            "aggregate_type": "memory",
            "aggregate_id": "memory-1",
            "causation_id": "cause-1",
            "correlation_id": "corr-1",
            "idempotency_key": "key-1",
            "payload": {"aggregate_sequence": 1, "memory_id": "memory-1"},
            "attempt_policy": {"max_attempts": 3},
            "fluctlight_id": "fl-1",
        }
    ]
    redis = FakeRedis()
    streams = RedisStreams(redis)
    session = Session(rows)
    import asyncio

    events = asyncio.run(streams.read_outbox_page(session, limit=10))
    replayed = asyncio.run(streams.publish_events(events))

    assert replayed == 1
    assert redis.events[0]["event_id"] == "event-published"


def test_pending_outbox_read_materializes_events_without_publishing() -> None:
    class Result:
        def mappings(self):
            return self

        def all(self):
            return [
                {
                    "id": "event-pending",
                    "kind": "platform.pending",
                    "aggregate_type": "platform",
                    "aggregate_id": "platform-1",
                    "causation_id": "cause-1",
                    "correlation_id": "corr-1",
                    "idempotency_key": "key-pending",
                    "payload": {"aggregate_sequence": 1},
                    "attempt_policy": {"max_attempts": 3},
                    "fluctlight_id": None,
                }
            ]

    class Session:
        async def execute(self, statement):
            return Result()

    events = asyncio.run(RedisStreams(FakeRedis()).read_pending_outbox(Session()))

    assert [event.id for event in events] == ["event-pending"]


def test_publish_events_uses_only_the_materialized_snapshot() -> None:
    redis = FakeRedis()
    event = OutboxEvent(
        id="event-snapshot",
        kind="platform.snapshot",
        aggregate_type="platform",
        aggregate_id="platform-1",
        causation_id="cause-1",
        correlation_id="corr-1",
        idempotency_key="key-snapshot",
        payload={"aggregate_sequence": 1},
        attempt_policy={"max_attempts": 3},
    )

    published = asyncio.run(RedisStreams(redis).publish_events([event]))

    assert published == 1
    assert redis.events[0]["event_id"] == "event-snapshot"


def test_mark_published_returns_the_database_cas_row_count() -> None:
    class Result:
        rowcount = 2

    class Session:
        async def execute(self, statement):
            return Result()

    marked = asyncio.run(
        RedisStreams(FakeRedis()).mark_published(Session(), ["event-1", "event-2"])
    )

    assert marked == 2
