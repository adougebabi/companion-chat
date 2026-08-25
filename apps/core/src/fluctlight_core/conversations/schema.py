"""Conversation-owned PostgreSQL tables."""

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
from sqlalchemy.dialects.postgresql import JSONB

from fluctlight_core.platform.persistence import metadata

conversations = Table(
    "conversations",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("created_by_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("title", String(256)),
    Column("revision", Integer, nullable=False, server_default="0"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)


participants = Table(
    "conversation_participants",
    metadata,
    Column("conversation_id", String(128), ForeignKey("public.conversations.id"), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("role", String(32), nullable=False, server_default="member"),
    Column("status", String(32), nullable=False, server_default="active"),
    Column("joined_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("left_at", DateTime(timezone=True)),
    UniqueConstraint("conversation_id", "actor_id", name="conversation_participant_actor"),
)


conversation_heads = Table(
    "conversation_heads",
    metadata,
    Column("conversation_id", String(128), ForeignKey("public.conversations.id"), primary_key=True),
    Column("next_sequence", Integer, nullable=False, server_default="1"),
)


messages = Table(
    "conversation_messages",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("conversation_id", String(128), ForeignKey("public.conversations.id"), nullable=False),
    Column("sequence", Integer, nullable=False),
    Column("author_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("kind", String(32), nullable=False),
    Column("text", Text, nullable=False),
    Column("attachment_refs", JSONB, nullable=False, server_default="[]"),
    Column("idempotency_key", String(256), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("conversation_id", "sequence", name="conversation_message_sequence"),
    UniqueConstraint("conversation_id", "idempotency_key", name="conversation_message_idempotency"),
)


read_positions = Table(
    "conversation_read_positions",
    metadata,
    Column("conversation_id", String(128), ForeignKey("public.conversations.id"), nullable=False),
    Column("actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("last_read_sequence", Integer, nullable=False, server_default="0"),
    Column("last_delivered_sequence", Integer, nullable=False, server_default="0"),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    UniqueConstraint("conversation_id", "actor_id", name="conversation_read_actor"),
)
