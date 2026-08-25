"""Diagnostics-owned PostgreSQL tables."""

from __future__ import annotations

from sqlalchemy import Column, DateTime, Integer, String, Table, func
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

diagnostic_events = Table(
    "diagnostic_events",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("event_type", String(128), nullable=False),
    Column("severity", String(32), nullable=False),
    Column("fluctlight_id", String(128)),
    Column("causation_id", String(128)),
    Column("correlation_id", String(128), nullable=False),
    Column("payload", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


diagnostic_model_runs = Table(
    "diagnostic_model_runs",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("role", String(64), nullable=False),
    Column("endpoint_id", String(128)),
    Column("model_id", String(256), nullable=False),
    Column("prompt", JSONB, nullable=False),
    Column("response", JSONB),
    Column("status", String(32), nullable=False),
    Column("error_code", String(128)),
    Column("correlation_id", String(128), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


diagnostic_turns = Table(
    "diagnostic_turns",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), nullable=False),
    Column("conversation_id", String(128)),
    Column("source_event_id", String(128)),
    Column("correlation_id", String(128), nullable=False),
    Column("status", String(32), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


diagnostic_workflow_links = Table(
    "diagnostic_workflow_links",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("correlation_id", String(128), nullable=False),
    Column("workflow_id", String(128), nullable=False),
    Column("intent_id", String(128)),
    Column("event_id", String(128)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


diagnostic_retention = Table(
    "diagnostic_retention",
    metadata,
    Column("id", Integer, primary_key=True, autoincrement=True),
    Column("resource", String(64), nullable=False, unique=True),
    Column("retention_days", Integer, nullable=False),
    Column("max_rows", Integer, nullable=False),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
