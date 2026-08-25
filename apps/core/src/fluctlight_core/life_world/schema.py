"""Life World-owned Event, Schedule and Context tables."""

from __future__ import annotations

from sqlalchemy import (
    Column,
    Date,
    DateTime,
    ForeignKey,
    Integer,
    String,
    Table,
    UniqueConstraint,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

events = Table(
    "life_events",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("kind", String(128), nullable=False),
    Column("start_at", DateTime(timezone=True), nullable=False),
    Column("end_at", DateTime(timezone=True), nullable=False),
    Column("scene", String(512)),
    Column("activity", String(512)),
    Column("location", String(512)),
    Column("status", String(32), nullable=False, server_default="confirmed"),
    Column("evidence_refs", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


schedules = Table(
    "life_schedules",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("local_date", Date, nullable=False),
    Column("timezone", String(128), nullable=False),
    Column("status", String(32), nullable=False, server_default="proposed"),
    Column("generated_from", String(128), nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("previous_version_id", String(128), ForeignKey("public.life_schedules.id")),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("generated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("reschedule_policy", JSONB, nullable=False),
    UniqueConstraint("fluctlight_id", "local_date", "revision", name="life_schedule_revision"),
)


schedule_items = Table(
    "life_schedule_items",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("schedule_id", String(128), ForeignKey("public.life_schedules.id"), nullable=False),
    Column("start_at", DateTime(timezone=True), nullable=False),
    Column("end_at", DateTime(timezone=True), nullable=False),
    Column("activity", String(512), nullable=False),
    Column("scene", String(512), nullable=False),
    Column("item_type", String(64), nullable=False),
    Column("status", String(32), nullable=False),
    Column("priority", String(32), nullable=False),
    Column("flexibility", String(32), nullable=False),
    Column("interruption_cost", String(32), nullable=False),
)


presence_overlays = Table(
    "life_presence_overlays",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("current_task", String(512)),
    Column("user_presence", String(128)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
