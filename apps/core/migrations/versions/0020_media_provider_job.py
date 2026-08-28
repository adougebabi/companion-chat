"""Persist the external Provider job ID used for media workflow recovery.

Revision ID: 0020_media_provider_job
Revises: 0019_compound_effects
"""

import sqlalchemy as sa
from alembic import op

revision = "0020_media_provider_job"
down_revision = "0019_compound_effects"
branch_labels = None
depends_on = None


def upgrade() -> None:
    bind = op.get_bind()
    columns = sa.inspect(bind).get_columns("media_intents", schema="public")
    if not any(column["name"] == "provider_job_id" for column in columns):
        op.add_column(
            "media_intents",
            sa.Column("provider_job_id", sa.String(length=256), nullable=True),
            schema="public",
        )
    constraints = sa.inspect(bind).get_unique_constraints("media_intents", schema="public")
    if not any(
        constraint.get("name") == "uq_media_intents_provider_job_id"
        for constraint in constraints
    ):
        op.create_unique_constraint(
            "uq_media_intents_provider_job_id",
            "media_intents",
            ["provider_job_id"],
            schema="public",
        )


def downgrade() -> None:
    op.drop_constraint(
        "uq_media_intents_provider_job_id", "media_intents", schema="public", type_="unique"
    )
    op.drop_column("media_intents", "provider_job_id", schema="public")
