"""Add per-consumer aggregate sequence heads.

Revision ID: 0011_t12_consumer_heads
Revises: 0010_t12_event_failures
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.platform import schema

revision = "0011_t12_consumer_heads"
down_revision = "0010_t12_event_failures"
branch_labels = None
depends_on = None


def upgrade() -> None:
    schema.consumer_heads.create(bind=op.get_bind())


def downgrade() -> None:
    schema.consumer_heads.drop(bind=op.get_bind())
