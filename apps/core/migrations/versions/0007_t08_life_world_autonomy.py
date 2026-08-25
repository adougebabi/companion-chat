"""Create T08 Life World and Autonomy tables.

Revision ID: 0007_t08_life_world_autonomy
Revises: 0006_t07_memory_relationships
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.autonomy import schema as autonomy_schema
from fluctlight_core.life_world import schema as life_world_schema

revision = "0007_t08_life_world_autonomy"
down_revision = "0006_t07_memory_relationships"
branch_labels = None
depends_on = None


_TABLES = (
    life_world_schema.events,
    life_world_schema.schedules,
    life_world_schema.schedule_items,
    life_world_schema.presence_overlays,
    autonomy_schema.policies,
    autonomy_schema.actions,
    autonomy_schema.governance,
)


def upgrade() -> None:
    bind = op.get_bind()
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
