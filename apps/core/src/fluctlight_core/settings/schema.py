"""Runtime settings and encrypted-secret tables owned by settings."""

from sqlalchemy import Column, DateTime, LargeBinary, String, Table, Text, func

from fluctlight_core.platform.persistence import metadata

runtime_settings = Table(
    "runtime_settings",
    metadata,
    Column("key", String(128), primary_key=True),
    Column("value_json", Text, nullable=False),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
setting_secrets = Table(
    "setting_secrets",
    metadata,
    Column("purpose", String(128), primary_key=True),
    Column("ciphertext", LargeBinary, nullable=False),
    Column("nonce", LargeBinary, nullable=False),
    Column("updated_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
settings_audit = Table(
    "settings_audit",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("actor_id", String(128), nullable=False),
    Column("field", String(128), nullable=False),
    Column("result", String(16), nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
