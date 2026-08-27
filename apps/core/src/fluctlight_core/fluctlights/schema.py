"""PostgreSQL tables owned by the Fluctlight foundation module."""

from __future__ import annotations

from sqlalchemy import (
    CheckConstraint,
    Column,
    DateTime,
    ForeignKey,
    Integer,
    String,
    Table,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

fluctlights = Table(
    "fluctlights",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("created_by_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("initialization_mode", String(16), nullable=False),
    Column("status", String(16), nullable=False, server_default="active"),
    Column("current_revision", Integer, nullable=False, server_default="0"),
    Column("identity", JSONB, nullable=False),
    Column("personality", JSONB, nullable=False),
    Column("behavioral_policy", JSONB, nullable=False),
    Column("life_profile", JSONB, nullable=False),
    Column("provenance", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("retired_at", DateTime(timezone=True)),
    CheckConstraint("current_revision >= 0", name="fluctlight_revision_nonnegative"),
)

foundation_revisions = Table(
    "fluctlight_foundation_revisions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("revision", Integer, nullable=False),
    Column("base_revision", Integer, nullable=False),
    Column("source", String(32), nullable=False),
    Column("status", String(16), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("initialization_mode", String(16), nullable=False),
    Column("foundation_status", String(16), nullable=False),
    Column("foundation_created_at", DateTime(timezone=True), nullable=False),
    Column("confidence", JSONB, nullable=False),
    Column("changes", JSONB, nullable=False),
    Column("identity", JSONB, nullable=False),
    Column("personality", JSONB, nullable=False),
    Column("behavioral_policy", JSONB, nullable=False),
    Column("life_profile", JSONB, nullable=False),
    Column("provenance", JSONB, nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("reason", Text),
    Column("idempotency_key", String(256), nullable=False, unique=True),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("accepted_at", DateTime(timezone=True)),
    Column("rejected_at", DateTime(timezone=True)),
    UniqueConstraint("fluctlight_id", "revision", name="uq_fluctlight_foundation_revision"),
)

foundation_governance = Table(
    "fluctlight_foundation_governance",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column(
        "revision_id",
        String(128),
        ForeignKey("public.fluctlight_foundation_revisions.id"),
        nullable=False,
    ),
    Column("action", String(32), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("reason", Text),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
