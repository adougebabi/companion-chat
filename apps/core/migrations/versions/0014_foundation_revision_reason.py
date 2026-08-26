"""Persist Owner/revision governance reasons.

Revision ID: 0014_foundation_reason
Revises: 0013_direct_conversation
"""

import sqlalchemy as sa
from alembic import op

revision = "0014_foundation_reason"
down_revision = "0013_direct_conversation"
branch_labels = None
depends_on = None


def upgrade() -> None:
    bind = op.get_bind()
    columns = {column["name"] for column in sa.inspect(bind).get_columns("fluctlight_foundation_revisions")}
    if "reason" not in columns:
        op.add_column(
            "fluctlight_foundation_revisions", sa.Column("reason", sa.Text(), nullable=True)
        )


def downgrade() -> None:
    bind = op.get_bind()
    columns = {column["name"] for column in sa.inspect(bind).get_columns("fluctlight_foundation_revisions")}
    if "reason" in columns:
        op.drop_column("fluctlight_foundation_revisions", "reason")
