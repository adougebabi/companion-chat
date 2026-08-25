"""Actor and Owner-auth tables owned by the actors module."""

from sqlalchemy import Column, DateTime, ForeignKey, String, Table, Text, func

from fluctlight_core.platform.persistence import metadata

actors = Table(
    "actors",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("actor_type", String(16), nullable=False),
    Column("status", String(16), nullable=False, server_default="active"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
owner_accounts = Table(
    "owner_accounts",
    metadata,
    Column("human_actor_id", String(128), ForeignKey("public.actors.id"), primary_key=True),
    Column("owner_key", String(16), nullable=False, unique=True, server_default="owner"),
    Column("credential_hash", Text, nullable=False),
    Column("algorithm", String(32), nullable=False, server_default="argon2id"),
    Column("parameters", String(256), nullable=False),
    Column("credential_revision", String(64), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
auth_sessions = Table(
    "auth_sessions",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("token_hash", String(64), nullable=False, unique=True),
    Column("human_actor_id", String(128), ForeignKey("public.actors.id"), nullable=False),
    Column("expires_at", DateTime(timezone=True), nullable=False),
    Column("last_seen_at", DateTime(timezone=True)),
    Column("revoked_at", DateTime(timezone=True)),
    Column("user_agent_hash", String(64)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
owner_setup_tokens = Table(
    "owner_setup_tokens",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("token_hash", String(64), nullable=False, unique=True),
    Column("expires_at", DateTime(timezone=True), nullable=False),
    Column("consumed_at", DateTime(timezone=True)),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
auth_audit = Table(
    "auth_audit",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("action", String(64), nullable=False),
    Column("actor_id", String(128)),
    Column("result", String(16), nullable=False),
    Column("details", Text, nullable=False, server_default="{}"),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
