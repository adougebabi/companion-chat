"""Autonomy policy snapshots, frozen actions and immutable governance."""

from __future__ import annotations

from sqlalchemy import Column, DateTime, ForeignKey, Integer, String, Table, Text, func
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

policies = Table(
    "autonomy_policies",
    metadata,
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), primary_key=True),
    Column("mode", String(32), nullable=False, server_default="active"),
    Column("allowed_actions", JSONB, nullable=False),
    Column("budget_remaining", String(64), nullable=False),
    Column("quiet_hours", JSONB, nullable=False),
    Column("cooldown_until", DateTime(timezone=True)),
    Column("concurrency_limit", Integer, nullable=False, server_default="1"),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


actions = Table(
    "autonomy_actions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("action_type", String(64), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("policy_snapshot", JSONB, nullable=False),
    Column("expected_revisions", JSONB, nullable=False),
    Column("status", String(32), nullable=False, server_default="frozen"),
    Column("workflow_id", String(128), nullable=False, unique=True),
    Column("provider_request_id", String(128), nullable=False, unique=True),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("settled_at", DateTime(timezone=True)),
    Column("error_code", String(128)),
)


governance = Table(
    "autonomy_governance",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("action_id", String(128), ForeignKey("public.autonomy_actions.id"), nullable=False),
    Column("from_status", String(32), nullable=False),
    Column("to_status", String(32), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("reason", Text),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
