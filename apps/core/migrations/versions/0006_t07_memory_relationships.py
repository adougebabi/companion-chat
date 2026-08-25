"""Create T07 Memory, Embedding, Relationship and governance tables.

Revision ID: 0006_t07_memory_relationships
Revises: 0005_t06_conversations
"""

from __future__ import annotations

from alembic import op
from fluctlight_core.memory import schema as memory_schema
from fluctlight_core.relationships import schema as relationship_schema

revision = "0006_t07_memory_relationships"
down_revision = "0005_t06_conversations"
branch_labels = None
depends_on = None


_TABLES = (
    memory_schema.memories,
    memory_schema.memory_revisions,
    memory_schema.memory_embeddings,
    relationship_schema.relationships,
    relationship_schema.relationship_revisions,
    relationship_schema.relationship_governance,
)


def upgrade() -> None:
    bind = op.get_bind()
    # The vector extension is infrastructure-owned and must exist before the
    # rebuildable embedding authority is created. The current JSON payload
    # remains a portable fallback until the vector column benchmark gate is
    # accepted.
    op.execute("CREATE EXTENSION IF NOT EXISTS vector")
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
