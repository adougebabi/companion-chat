"""Add durable effects for the initial Redis consumer groups.

Revision ID: 0012_t12_consumer_effects
Revises: 0011_t12_consumer_heads
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.platform import schema

revision = "0012_t12_consumer_effects"
down_revision = "0011_t12_consumer_heads"
branch_labels = None
depends_on = None


def upgrade() -> None:
    schema.consumer_effects.create(bind=op.get_bind())


def downgrade() -> None:
    schema.consumer_effects.drop(bind=op.get_bind())
