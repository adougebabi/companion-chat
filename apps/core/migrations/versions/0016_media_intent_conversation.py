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


def _column_exists(bind: sa.Connection, table_name: str, column_name: str) -> bool:
    columns = sa.inspect(bind).get_columns(table_name, schema="public")
    return any(column["name"] == column_name for column in columns)


def _conversation_foreign_key_exists(bind: sa.Connection) -> bool:
    for foreign_key in sa.inspect(bind).get_foreign_keys("media_intents", schema="public"):
        if (
            foreign_key.get("constrained_columns") == ["conversation_id"]
            and foreign_key.get("referred_table") == "conversations"
            and foreign_key.get("referred_schema") == "public"
        ):
            return True
    return False


def upgrade() -> None:
    bind = op.get_bind()
    if not _column_exists(bind, "media_intents", "conversation_id"):
        op.add_column(
            "media_intents",
            sa.Column("conversation_id", sa.String(length=128), nullable=True),
            schema="public",
        )
    if not _conversation_foreign_key_exists(bind):
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
