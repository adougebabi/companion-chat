"""Persist the conversation receiving a generated media result.

Revision ID: 0016_media_intent_conversation
Revises: 0015_actor_groups
"""

import sqlalchemy as sa
from alembic import op

revision = "0016_media_intent_conversation"
down_revision = "0015_actor_groups"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column(
        "media_intents",
        sa.Column("conversation_id", sa.String(length=128), nullable=True),
        schema="public",
    )
    op.create_foreign_key(
        "fk_media_intents_conversation_id",
        "media_intents",
        "conversations",
        ["conversation_id"],
        ["id"],
        source_schema="public",
        referent_schema="public",
    )


def downgrade() -> None:
    op.drop_constraint("fk_media_intents_conversation_id", "media_intents", schema="public")
    op.drop_column("media_intents", "conversation_id", schema="public")
