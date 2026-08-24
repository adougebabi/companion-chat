"""PostgreSQL tables owned by the inner-state module."""

from __future__ import annotations

from sqlalchemy import (
    CheckConstraint,
    Column,
    DateTime,
    ForeignKey,
    ForeignKeyConstraint,
    Integer,
    String,
    Table,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

inner_states = Table(
    "fluctlight_inner_states",
    metadata,
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), primary_key=True),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("pad", JSONB, nullable=False),
    Column("mood", JSONB, nullable=False),
    Column("momentum", JSONB, nullable=False),
    Column("regulation", JSONB, nullable=False),
    Column("drives", JSONB, nullable=False),
    Column("conflicts", JSONB, nullable=False),
    Column("last_updated_at", DateTime(timezone=True), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    CheckConstraint("revision >= 0", name="inner_state_revision_nonnegative"),
    CheckConstraint(
        "(pad->>'pleasure')::double precision BETWEEN -1 AND 1",
        name="inner_state_pad_pleasure_range",
    ),
    CheckConstraint(
        "(pad->>'arousal')::double precision BETWEEN -1 AND 1",
        name="inner_state_pad_arousal_range",
    ),
    CheckConstraint(
        "(pad->>'dominance')::double precision BETWEEN -1 AND 1",
        name="inner_state_pad_dominance_range",
    ),
)

inner_state_events = Table(
    "fluctlight_inner_state_events",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("source_event_id", String(256), nullable=False),
    Column("expected_revision", Integer, nullable=False),
    Column("resulting_revision", Integer, nullable=False),
    Column("previous_state", JSONB, nullable=False),
    Column("resulting_state", JSONB, nullable=False),
    Column("requested_delta", JSONB, nullable=False),
    Column("applied_delta", JSONB, nullable=False),
    Column("result", String(16), nullable=False),
    Column("reason_code", String(128), nullable=False),
    Column("policy_version", String(128), nullable=False),
    Column("model_version", String(128), nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("idempotency_key", String(256), nullable=False, unique=True),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("fluctlight_id", "source_event_id", name="uq_fluctlight_state_source_event"),
)

goals = Table(
    "fluctlight_goals",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("source", String(16), nullable=False),
    Column("description", Text, nullable=False),
    Column("importance", JSONB, nullable=False),
    Column("urgency", JSONB, nullable=False),
    Column("progress", JSONB, nullable=False),
    Column("deadline", DateTime(timezone=True)),
    Column("status", String(16), nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("fluctlight_id", "id", name="uq_fluctlight_goal_owner"),
)

goal_governance = Table(
    "fluctlight_goal_governance",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("goal_id", String(128), ForeignKey("public.fluctlight_goals.id"), nullable=False),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("from_status", String(16), nullable=False),
    Column("to_status", String(16), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("reason", Text),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)

intentions = Table(
    "fluctlight_intentions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("goal_id", String(128)),
    Column("action", String(256), nullable=False),
    Column("preferred_time", DateTime(timezone=True)),
    Column("trigger", JSONB, nullable=False),
    Column("confidence", JSONB, nullable=False),
    Column("expiration", DateTime(timezone=True), nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("permission_snapshot", JSONB, nullable=False),
    Column("budget_snapshot", JSONB, nullable=False),
    Column("status", String(16), nullable=False),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    ForeignKeyConstraint(
        ["fluctlight_id", "goal_id"],
        ["public.fluctlight_goals.fluctlight_id", "public.fluctlight_goals.id"],
        name="fk_fluctlight_intention_goal_owner",
    ),
)

intention_governance = Table(
    "fluctlight_intention_governance",
    metadata,
    Column("id", String(128), primary_key=True),
    Column(
        "intention_id", String(128), ForeignKey("public.fluctlight_intentions.id"), nullable=False
    ),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("from_status", String(16), nullable=False),
    Column("to_status", String(16), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("reason", Text),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
