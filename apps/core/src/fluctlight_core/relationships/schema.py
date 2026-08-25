"""Relationship-owned directed Actor state and append-only governance."""

from __future__ import annotations

from sqlalchemy import (
    Column,
    DateTime,
    Float,
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

relationships = Table(
    "relationships",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("target_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("metrics", JSONB, nullable=False),
    Column("interaction_frequency", Float, nullable=False, server_default="0"),
    Column("last_interaction_at", DateTime(timezone=True)),
    Column("last_meaningful_interaction_at", DateTime(timezone=True)),
    Column("trend", String(32), nullable=False, server_default="stable"),
    Column("summary", Text),
    Column("emotional_association", JSONB, nullable=False),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("owner_fluctlight_id", "target_actor_id", name="relationship_directed_pair"),
)


relationship_revisions = Table(
    "relationship_revisions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("relationship_id", String(128), ForeignKey("public.relationships.id"), nullable=False),
    Column("revision", Integer, nullable=False),
    Column("base_revision", Integer, nullable=False),
    Column("metrics", JSONB, nullable=False),
    Column("trend", String(32), nullable=False),
    Column("summary", Text),
    Column("emotional_association", JSONB, nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("idempotency_key", String(256), nullable=False, unique=True),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("relationship_id", "revision", name="relationship_revision_number"),
)


relationship_governance = Table(
    "relationship_governance",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("relationship_id", String(128), ForeignKey("public.relationships.id"), nullable=False),
    Column(
        "revision_id", String(128), ForeignKey("public.relationship_revisions.id"), nullable=False
    ),
    Column("action", String(32), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("reason", Text),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
