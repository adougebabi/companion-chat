"""Memory-owned PostgreSQL authority and rebuildable embedding rows."""

from __future__ import annotations

import json

from sqlalchemy import (
    Column,
    Computed,
    DateTime,
    Float,
    ForeignKey,
    Index,
    Integer,
    String,
    Table,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB, TSVECTOR
from sqlalchemy.types import UserDefinedType

from fluctlight_core.platform.persistence import metadata


class PgVector(UserDefinedType):
    """Portable SQLAlchemy binding for the pgvector extension's vector type."""

    cache_ok = True

    def get_col_spec(self, **kwargs: object) -> str:
        return "vector"

    def bind_processor(self, dialect: object):
        def process(value: tuple[float, ...] | list[float] | None) -> str | None:
            if value is None:
                return None
            return "[" + ",".join(str(float(item)) for item in value) + "]"

        return process

    def result_processor(self, dialect: object, coltype: object):
        def process(value: str | None) -> tuple[float, ...] | None:
            if value is None:
                return None
            return tuple(float(item) for item in json.loads(value))

        return process


memories = Table(
    "memories",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("owner_fluctlight_id", String(128), ForeignKey("public.fluctlights.id"), nullable=False),
    Column("type", String(32), nullable=False),
    Column("content", Text, nullable=False),
    Column(
        "search_document",
        TSVECTOR,
        Computed("to_tsvector('simple', content)", persisted=True),
        nullable=False,
    ),
    Column("actor_refs", JSONB, nullable=False),
    Column("conversation_id", String(128)),
    Column("event_refs", JSONB, nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("confidence", Float, nullable=False),
    Column("importance", Float, nullable=False),
    Column("emotional_significance", Float, nullable=False),
    Column("visibility", String(32), nullable=False),
    Column("status", String(32), nullable=False, server_default="active"),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("occurred_at", DateTime(timezone=True)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("last_confirmed_at", DateTime(timezone=True)),
)

Index("ix_memories_search_document", memories.c.search_document, postgresql_using="gin")


memory_revisions = Table(
    "memory_revisions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("memory_id", String(128), ForeignKey("public.memories.id"), nullable=False),
    Column("revision", Integer, nullable=False),
    Column("base_revision", Integer, nullable=False),
    Column("content", Text, nullable=False),
    Column("status", String(32), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("evidence_refs", JSONB, nullable=False),
    Column("idempotency_key", String(256), nullable=False, unique=True),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("memory_id", "revision", name="memory_revision_number"),
)


memory_embeddings = Table(
    "memory_embeddings",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("memory_id", String(128), ForeignKey("public.memories.id"), nullable=False),
    Column("memory_revision", Integer, nullable=False),
    Column("model_id", String(256), nullable=False),
    Column("dimensions", Integer, nullable=False),
    Column("embedding", JSONB, nullable=False),
    Column("embedding_vector", PgVector(), nullable=True),
    Column("status", String(32), nullable=False, server_default="pending"),
    Column("error_code", String(128)),
    Column("embedded_at", DateTime(timezone=True)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("memory_id", "memory_revision", "model_id", name="memory_embedding_identity"),
)
