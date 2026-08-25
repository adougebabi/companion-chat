from __future__ import annotations

from alembic import context
from fluctlight_core.actors import schema as actors_schema  # noqa: F401
from fluctlight_core.cognition import schema as cognition_schema  # noqa: F401
from fluctlight_core.conversations import schema as conversations_schema  # noqa: F401
from fluctlight_core.diagnostics import schema as diagnostics_schema  # noqa: F401
from fluctlight_core.fluctlights import schema as fluctlights_schema  # noqa: F401
from fluctlight_core.inner_state import schema as inner_state_schema  # noqa: F401
from fluctlight_core.life_world import schema as life_world_schema  # noqa: F401
from fluctlight_core.media import schema as media_schema  # noqa: F401
from fluctlight_core.memory import schema as memory_schema  # noqa: F401
from fluctlight_core.moments import schema as moments_schema  # noqa: F401
from fluctlight_core.platform import schema  # noqa: F401
from fluctlight_core.platform.persistence import metadata
from fluctlight_core.providers import schema as providers_schema  # noqa: F401
from fluctlight_core.relationships import schema as relationships_schema  # noqa: F401
from fluctlight_core.settings import schema as settings_schema  # noqa: F401
from sqlalchemy import engine_from_config, pool

config = context.config
target_metadata = metadata


def run_migrations_offline() -> None:
    context.configure(
        url=config.get_main_option("sqlalchemy.url"),
        target_metadata=target_metadata,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
    )
    with context.begin_transaction():
        context.run_migrations()


def run_migrations_online() -> None:
    connectable = engine_from_config(
        config.get_section(config.config_ini_section, {}),
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )
    with connectable.connect() as connection:
        context.configure(connection=connection, target_metadata=target_metadata)
        with context.begin_transaction():
            context.run_migrations()


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
