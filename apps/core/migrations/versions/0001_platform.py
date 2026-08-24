"""Create platform persistence tables.

Revision ID: 0001_platform
Revises:
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.platform import schema  # noqa: F401

revision = "0001_platform"
down_revision = None
branch_labels = None
depends_on = None


def upgrade() -> None:
    bind = op.get_bind()
    for table in (
        schema.workflow_intents,
        schema.outbox_events,
        schema.consumer_inbox,
        schema.workflow_management_audit,
        schema.platform_object_grants,
    ):
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in (
        schema.platform_object_grants,
        schema.workflow_management_audit,
        schema.consumer_inbox,
        schema.outbox_events,
        schema.workflow_intents,
    ):
        table.drop(bind=bind)
