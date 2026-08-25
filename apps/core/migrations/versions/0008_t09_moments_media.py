"""Create T09 Moments and Media lifecycle tables.

Revision ID: 0008_t09_moments_media
Revises: 0007_t08_life_world_autonomy
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.media import schema as media_schema
from fluctlight_core.moments import schema as moments_schema

revision = "0008_t09_moments_media"
down_revision = "0007_t08_life_world_autonomy"
branch_labels = None
depends_on = None


_TABLES = (
    moments_schema.moments,
    moments_schema.comments,
    moments_schema.reactions,
    moments_schema.unread_markers,
    media_schema.intents,
    media_schema.assets,
    media_schema.references,
    media_schema.tombstones,
)


def upgrade() -> None:
    bind = op.get_bind()
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
