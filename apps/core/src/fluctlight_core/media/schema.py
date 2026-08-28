"""Media identity/lifecycle, references and generation intent tables."""

from __future__ import annotations

from sqlalchemy import (
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

from fluctlight_core.platform.persistence import metadata

intents = Table(
    "media_intents",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("kind", String(32), nullable=False),
    Column("mime_type", String(128), nullable=False),
    Column("prompt", Text, nullable=False),
    Column("provider_request_id", String(128), nullable=False, unique=True),
    Column("provider_job_id", String(256), unique=True),
    Column("workflow_id", String(128), nullable=False, unique=True),
    Column("conversation_id", String(128), ForeignKey("public.conversations.id")),
    Column("moment_id", String(128), ForeignKey("public.moments.id")),
    Column("status", String(32), nullable=False, server_default="pending"),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


assets = Table(
    "media_assets",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("version", String(128), nullable=False),
    Column("kind", String(32), nullable=False),
    Column("mime_type", String(128), nullable=False),
    Column("byte_size", Integer, nullable=False),
    Column("sha256", String(128), nullable=False),
    Column("bucket", String(256), nullable=False),
    Column("object_key", Text, nullable=False),
    Column("object_version", String(256)),
    Column("etag", String(256)),
    Column("provider_request_id", String(128), nullable=False),
    Column("workflow_id", String(128), nullable=False),
    Column("status", String(32), nullable=False, server_default="pending"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("ready_at", DateTime(timezone=True)),
    Column("tombstoned_at", DateTime(timezone=True)),
    Column("deleted_at", DateTime(timezone=True)),
    UniqueConstraint("id", "version", name="media_asset_version"),
)


references = Table(
    "media_references",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("asset_id", String(128), ForeignKey("public.media_assets.id"), nullable=False),
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("target_type", String(64), nullable=False),
    Column("target_id", String(128), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


tombstones = Table(
    "media_tombstones",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("asset_id", String(128), ForeignKey("public.media_assets.id"), nullable=False),
    Column("reason", Text, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
