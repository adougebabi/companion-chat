"""Create T05 cognition inbox and diagnostics tables.

Revision ID: 0004_t05_cognition_diagnostics
Revises: 0003_t04_fluctlight
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.cognition import schema as cognition_schema
from fluctlight_core.diagnostics import schema as diagnostics_schema

revision = "0004_t05_cognition_diagnostics"
down_revision = "0003_t04_fluctlight"
branch_labels = None
depends_on = None


_TABLES = (
    cognition_schema.inbox_heads,
    cognition_schema.inbox,
    cognition_schema.assessments,
    cognition_schema.decision_proposals,
    cognition_schema.frozen_actions,
    cognition_schema.reflection_windows,
    cognition_schema.reflection_proposals,
    diagnostics_schema.diagnostic_events,
    diagnostics_schema.diagnostic_model_runs,
    diagnostics_schema.diagnostic_turns,
    diagnostics_schema.diagnostic_workflow_links,
    diagnostics_schema.diagnostic_retention,
)


def upgrade() -> None:
    bind = op.get_bind()
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
