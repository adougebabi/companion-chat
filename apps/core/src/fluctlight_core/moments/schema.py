"""Moments, comments, reactions and unread markers."""

from __future__ import annotations

from sqlalchemy import Column, DateTime, ForeignKey, String, Table, Text, UniqueConstraint, func
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

moments = Table(
    "moments",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("author_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("text", Text, nullable=False),
    Column("visibility", String(32), nullable=False, server_default="participants"),
    Column("status", String(32), nullable=False, server_default="visible"),
    Column("media_asset_ids", JSONB, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


comments = Table(
    "moment_comments",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("moment_id", String(128), ForeignKey("public.moments.id"), nullable=False),
    Column("author_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("text", Text, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


reactions = Table(
    "moment_reactions",
    metadata,
    Column("moment_id", String(128), ForeignKey("public.moments.id"), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("kind", String(32), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("moment_id", "actor_id", name="moment_reaction_actor"),
)


unread_markers = Table(
    "moment_unread_markers",
    metadata,
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("last_seen_at", DateTime(timezone=True)),
    UniqueConstraint("owner_fluctlight_id", "actor_id", name="moment_unread_actor"),
)
