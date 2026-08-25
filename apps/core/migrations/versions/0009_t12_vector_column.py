"""Add the rebuildable pgvector column to existing T09 deployments.

Revision ID: 0009_t12_vector_column
Revises: 0008_t09_moments_media
"""

from __future__ import annotations

from alembic import op

revision = "0009_t12_vector_column"
down_revision = "0008_t09_moments_media"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.execute(
        "ALTER TABLE public.memory_embeddings "
        "ADD COLUMN IF NOT EXISTS embedding_vector vector"
    )
    op.execute(
        "ALTER TABLE public.memories "
        "ADD COLUMN IF NOT EXISTS search_document tsvector "
        "GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED"
    )
    op.execute(
        "CREATE INDEX IF NOT EXISTS ix_memories_search_document "
        "ON public.memories USING gin (search_document)"
    )


def downgrade() -> None:
    op.execute(
        "ALTER TABLE public.memory_embeddings "
        "DROP COLUMN IF EXISTS embedding_vector"
    )
    op.execute("DROP INDEX IF EXISTS public.ix_memories_search_document")
    op.execute("ALTER TABLE public.memories DROP COLUMN IF EXISTS search_document")
