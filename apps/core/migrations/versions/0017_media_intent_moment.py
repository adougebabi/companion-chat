"""Persist the Moment receiving an autonomously generated image.

Revision ID: 0017_media_intent_moment
Revises: 0016_media_intent_conversation
"""

import sqlalchemy as sa
from alembic import op

revision = "0017_media_intent_moment"
down_revision = "0016_media_intent_conversation"
branch_labels = None
depends_on = None


def _column_exists(bind: sa.Connection, table_name: str, column_name: str) -> bool:
    return any(
        column["name"] == column_name
        for column in sa.inspect(bind).get_columns(table_name, schema="public")
    )


def _moment_foreign_key_exists(bind: sa.Connection) -> bool:
    return any(
        foreign_key.get("constrained_columns") == ["moment_id"]
        and foreign_key.get("referred_table") == "moments"
        and foreign_key.get("referred_schema") == "public"
        for foreign_key in sa.inspect(bind).get_foreign_keys("media_intents", schema="public")
    )


def upgrade() -> None:
    bind = op.get_bind()
    op.execute("UPDATE public.moments SET visibility = 'participants' WHERE visibility = 'owner'")
    op.alter_column(
        "moments",
        "visibility",
        schema="public",
        existing_type=sa.String(length=32),
        server_default="participants",
    )
    if not _column_exists(bind, "media_intents", "moment_id"):
        op.add_column(
            "media_intents",
            sa.Column("moment_id", sa.String(length=128), nullable=True),
            schema="public",
        )
    if not _moment_foreign_key_exists(bind):
        op.create_foreign_key(
            "fk_media_intents_moment_id",
            "media_intents",
            "moments",
            ["moment_id"],
            ["id"],
            source_schema="public",
            referent_schema="public",
        )


def downgrade() -> None:
    op.drop_constraint("fk_media_intents_moment_id", "media_intents", schema="public")
    op.drop_column("media_intents", "moment_id", schema="public")
    op.alter_column(
        "moments",
        "visibility",
        schema="public",
        existing_type=sa.String(length=32),
        server_default="owner",
    )
