"""Add versioned Foundation V2 life-profile and field provenance.

Revision ID: 0018_foundation_v2_life_profile
Revises: 0017_media_intent_moment
"""

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects.postgresql import JSONB

revision = "0018_foundation_v2_life_profile"
down_revision = "0017_media_intent_moment"
branch_labels = None
depends_on = None


def upgrade() -> None:
    for table_name in ("fluctlights", "fluctlight_foundation_revisions"):
        op.add_column(
            table_name,
            sa.Column("life_profile", JSONB, nullable=False, server_default=sa.text("'{}'::jsonb")),
            schema="public",
        )
        op.add_column(
            table_name,
            sa.Column("provenance", JSONB, nullable=False, server_default=sa.text("'{}'::jsonb")),
            schema="public",
        )


def downgrade() -> None:
    for table_name in ("fluctlight_foundation_revisions", "fluctlights"):
        op.drop_column(table_name, "provenance", schema="public")
        op.drop_column(table_name, "life_profile", schema="public")
