"""Create T04 Fluctlight foundation and inner-state tables.

Revision ID: 0003_t04_fluctlight
Revises: 0002_t03_auth
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.fluctlights import schema as fluctlight_schema
from fluctlight_core.inner_state import schema as inner_state_schema

revision = "0003_t04_fluctlight"
down_revision = "0002_t03_auth"
branch_labels = None
depends_on = None


_TABLES = (
    fluctlight_schema.fluctlights,
    fluctlight_schema.foundation_revisions,
    fluctlight_schema.foundation_governance,
    inner_state_schema.inner_states,
    inner_state_schema.inner_state_events,
    inner_state_schema.goals,
    inner_state_schema.goal_governance,
    inner_state_schema.intentions,
    inner_state_schema.intention_governance,
)


def upgrade() -> None:
    bind = op.get_bind()
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
