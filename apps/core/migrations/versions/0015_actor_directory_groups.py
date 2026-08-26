"""Create Owner-organized Actor directory groups.

Revision ID: 0015_actor_groups
Revises: 0014_foundation_reason
"""

from alembic import op
from fluctlight_core.actors import schema as actors

revision = "0015_actor_groups"
down_revision = "0014_foundation_reason"
branch_labels = None
depends_on = None


def upgrade() -> None:
    bind = op.get_bind()
    actors.actor_groups.create(bind=bind)
    actors.actor_group_members.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    actors.actor_group_members.drop(bind=bind)
    actors.actor_groups.drop(bind=bind)
