"""Provider tables owned by the providers module."""

from sqlalchemy import Column, DateTime, Integer, String, Table, Text, func

from fluctlight_core.platform.persistence import metadata

provider_endpoints = Table(
    "provider_endpoints",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("kind", String(64), nullable=False),
    Column("base_url", Text, nullable=False),
    Column("secret_purpose", String(128), nullable=False),
    Column("capability_status", String(32), nullable=False, server_default="unknown"),
    Column("checked_at", DateTime(timezone=True)),
)
model_roles = Table(
    "model_roles",
    metadata,
    Column("role", String(64), primary_key=True),
    Column("provider_endpoint_id", String(128), nullable=False),
    Column("model_id", String(256), nullable=False),
    Column("required_capabilities", Text, nullable=False),
    Column("token_budget", Integer, nullable=False),
    Column("timeout_seconds", Integer, nullable=False),
    Column("retry_policy", Text, nullable=False),
)
provider_preflights = Table(
    "provider_preflights",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("role", String(64), nullable=False),
    Column("result", String(32), nullable=False),
    Column("capability_version", String(128)),
    Column("checked_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
provider_provenance = Table(
    "provider_provenance",
    metadata,
    Column("id", String(128), primary_key=True),
    Column("role", String(64), nullable=False),
    Column("endpoint_id", String(128), nullable=False),
    Column("model_id", String(256), nullable=False),
    Column("prompt_version", String(128), nullable=False),
    Column("schema_version", String(128), nullable=False),
    Column("correlation_id", String(128), nullable=False),
    Column("token_budget", Integer, nullable=False),
    Column("created_at", DateTime(timezone=True), nullable=False, server_default=func.now()),
)
