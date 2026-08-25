"""Add durable consumer failure/quarantine records.

Revision ID: 0010_t12_event_failures
Revises: 0009_t12_vector_column
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.platform import schema

revision = "0010_t12_event_failures"
down_revision = "0009_t12_vector_column"
branch_labels = None
depends_on = None


def upgrade() -> None:
    schema.consumer_failures.create(bind=op.get_bind())


def downgrade() -> None:
    schema.consumer_failures.drop(bind=op.get_bind())
