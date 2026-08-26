"""Add authoritative Owner-to-Fluctlight direct conversation mapping.

Revision ID: 0013_direct_conversation
Revises: 0012_t12_consumer_effects
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.conversations import schema as conversation_schema

revision = "0013_direct_conversation"
down_revision = "0012_t12_consumer_effects"
branch_labels = None
depends_on = None


def upgrade() -> None:
    conversation_schema.direct_conversations.create(bind=op.get_bind())


def downgrade() -> None:
    conversation_schema.direct_conversations.drop(bind=op.get_bind())
