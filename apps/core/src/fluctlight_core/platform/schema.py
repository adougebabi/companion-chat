"""Platform-owned PostgreSQL tables. Domain modules add their own tables later."""

from __future__ import annotations

from sqlalchemy import Column, DateTime, Integer, String, Table, Text, UniqueConstraint, func
from sqlalchemy.dialects.postgresql import JSONB

from .persistence import metadata

workflow_intents = Table(
    "platform_workflow_intents",
    metadata,
    Column("intent_id", String(128), primary_key=True),
    Column("workflow_id", String(128), nullable=False, unique=True),
    Column("task_queue", String(32), nullable=False),
    Column("intent_type", String(96), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)

outbox_events = Table(
    "platform_outbox_events",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("kind", String(128), nullable=False),
    Column("aggregate_type", String(96), nullable=False),
    Column("aggregate_id", String(128), nullable=False),
    Column("fluctlight_id", String(128)),
    Column("causation_id", String(128), nullable=False),
    Column("correlation_id", String(128), nullable=False),
    Column("idempotency_key", String(256), nullable=False, unique=True),
    Column("payload", JSONB, nullable=False),
    Column("occurred_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("available_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("attempt_policy", JSONB, nullable=False),
    Column("published_at", DateTime(timezone=True)),
    Column("completed_at", DateTime(timezone=True)),
    Column("failed_at", DateTime(timezone=True)),
)

consumer_inbox = Table(
    "platform_consumer_inbox",
    metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column("consumer_group", String(96), nullable=False),
    Column("event_id", String(128), nullable=False),
    Column("result", JSONB, nullable=False),
    Column("applied_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("consumer_group", "event_id", name="consumer_event"),
)

consumer_failures = Table(
    "platform_consumer_failures",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("consumer_group", String(96), nullable=False),
    Column("event_id", String(128), nullable=False),
    Column("stream_id", String(128), nullable=False),
    Column("attempt", Integer, nullable=False),
    Column("max_attempts", Integer, nullable=False),
    Column("status", String(32), nullable=False),
    Column("error_code", String(128), nullable=False),
    Column("details", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("consumer_group", "event_id", "attempt", name="consumer_failure_attempt"),
)

consumer_heads = Table(
    "platform_consumer_heads",
    metadata,
    Column("consumer_group", String(96), nullable=False),
    Column("aggregate_type", String(96), nullable=False),
    Column("aggregate_id", String(128), nullable=False),
    Column("last_sequence", Integer, nullable=False),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint(
        "consumer_group",
        "aggregate_type",
        "aggregate_id",
        name="consumer_aggregate_head",
    ),
)

consumer_effects = Table(
    "platform_consumer_effects",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("consumer_group", String(96), nullable=False),
    Column("event_id", String(128), nullable=False),
    Column("effect_type", String(64), nullable=False),
    Column("aggregate_type", String(96), nullable=False),
    Column("aggregate_id", String(128), nullable=False),
    Column("aggregate_sequence", Integer, nullable=False),
    Column("correlation_id", String(128), nullable=False),
    Column("fluctlight_id", String(128)),
    Column("payload_digest", String(64), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("consumer_group", "event_id", name="consumer_effect_once"),
)

workflow_management_audit = Table(
    "platform_workflow_management_audit",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("action", String(32), nullable=False),
    Column("workflow_id", String(128), nullable=False),
    Column("actor_id", String(128), nullable=False),
    Column("authorized", String(8), nullable=False),
    Column("details", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)

platform_object_grants = Table(
    "platform_object_grants",
    metadata,
    Column("grant_id", String(128), primary_key=True),
    Column("object_key", Text, nullable=False),
    Column("object_version", String(256)),
    Column("expires_at", DateTime(timezone=True), nullable=False),
    Column("range_policy", String(64), nullable=False),
)
