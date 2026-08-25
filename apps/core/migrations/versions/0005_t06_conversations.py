"""Create T06 Conversation, Participant, Message and read-position tables.

Revision ID: 0005_t06_conversations
Revises: 0004_t05_cognition_diagnostics
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.conversations import schema as conversation_schema

revision = "0005_t06_conversations"
down_revision = "0004_t05_cognition_diagnostics"
branch_labels = None
depends_on = None


_TABLES = (
    conversation_schema.conversations,
    conversation_schema.participants,
    conversation_schema.conversation_heads,
    conversation_schema.messages,
    conversation_schema.read_positions,
)


def upgrade() -> None:
    bind = op.get_bind()
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
