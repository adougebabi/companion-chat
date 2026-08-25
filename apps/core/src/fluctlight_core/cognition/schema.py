"""Cognition-owned PostgreSQL tables."""

from __future__ import annotations

from sqlalchemy import (
    Column,
    DateTime,
    Integer,
    String,
    Table,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

inbox_heads = Table(
    "cognition_inbox_heads",
    metadata,
    Column("fluctlight_id", String(128), primary_key=True),
    Column("next_sequence", Integer, nullable=False, server_default="1"),
    Column("last_processed_sequence", Integer, nullable=False, server_default="0"),
    Column("writer_owner", String(128)),
    Column("writer_lease_until", DateTime(timezone=True)),
)


inbox = Table(
    "cognition_inbox",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), nullable=False),
    Column("sequence", Integer, nullable=False),
    Column("event_type", String(128), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("causation_id", String(128), nullable=False),
    Column("correlation_id", String(128), nullable=False),
    Column("idempotency_key", String(256), nullable=False),
    Column("occurred_at", DateTime(timezone=True), nullable=False),
    Column("status", String(32), nullable=False, server_default="pending"),
    Column("attempt_count", Integer, nullable=False, server_default="0"),
    Column("claimed_by", String(128)),
    Column("claimed_at", DateTime(timezone=True)),
    Column("processed_at", DateTime(timezone=True)),
    Column("error_code", String(128)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("fluctlight_id", "sequence", name="cognition_inbox_fluctlight_sequence"),
    UniqueConstraint("idempotency_key", name="cognition_inbox_idempotency"),
)


assessments = Table(
    "cognition_assessments",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("inbox_id", String(128), nullable=False),
    Column("fluctlight_id", String(128), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("schema_version", String(64), nullable=False),
    Column("model", String(256), nullable=False),
    Column("model_version", String(256), nullable=False),
    Column("prompt_version", String(256), nullable=False),
    Column("correlation_id", String(128), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("inbox_id", name="cognition_assessment_inbox"),
)


decision_proposals = Table(
    "cognition_decision_proposals",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("assessment_id", String(128), nullable=False),
    Column("fluctlight_id", String(128), nullable=False),
    Column("action_type", String(64), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("confidence", Text, nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("expires_at", DateTime(timezone=True)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("assessment_id", name="cognition_decision_assessment"),
)


frozen_actions = Table(
    "cognition_frozen_actions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("decision_id", String(128), nullable=False),
    Column("inbox_id", String(128), nullable=False),
    Column("fluctlight_id", String(128), nullable=False),
    Column("action_type", String(64), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("state_revision", Integer, nullable=False),
    Column("provider_request_id", String(128), nullable=False),
    Column("status", String(32), nullable=False, server_default="frozen"),
    Column("realization_payload", JSONB),
    Column("error_code", String(128)),
    Column("frozen_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("completed_at", DateTime(timezone=True)),
    UniqueConstraint("decision_id", name="cognition_action_decision"),
    UniqueConstraint("provider_request_id", name="cognition_action_provider_request"),
)


reflection_windows = Table(
    "cognition_reflection_windows",
    metadata,
    Column("fluctlight_id", String(128), primary_key=True),
    Column("watermark", Integer, nullable=False, server_default="0"),
    Column("state_revision", Integer, nullable=False, server_default="0"),
    Column("status", String(32), nullable=False, server_default="idle"),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


reflection_proposals = Table(
    "cognition_reflection_proposals",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), nullable=False),
    Column("from_sequence", Integer, nullable=False),
    Column("to_sequence", Integer, nullable=False),
    Column("base_state_revision", Integer, nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("correlation_id", String(128), nullable=False),
    Column("status", String(32), nullable=False, server_default="proposed"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint(
        "fluctlight_id",
        "from_sequence",
        "to_sequence",
        name="cognition_reflection_window_range",
    ),
)
