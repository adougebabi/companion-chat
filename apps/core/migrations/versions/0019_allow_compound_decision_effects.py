"""Allow more than one decision proposal for a single assessment.

Compound cognition decisions persist one assessment and one proposal per effect.
"""

from alembic import op

revision = "0019_allow_compound_decision_effects"
down_revision = "0018_foundation_v2_life_profile"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.drop_constraint(
        "cognition_decision_assessment",
        "cognition_decision_proposals",
        schema="public",
        type_="unique",
    )


def downgrade() -> None:
    op.create_unique_constraint(
        "cognition_decision_assessment",
        "cognition_decision_proposals",
        ["assessment_id"],
        schema="public",
    )
