from fluctlight_core.fluctlights import schema as fluctlight_schema
from fluctlight_core.inner_state import schema as inner_state_schema
from sqlalchemy import ForeignKeyConstraint


def test_t04_schema_keeps_actor_audit_and_goal_ownership_constraints() -> None:
    created_by = next(iter(fluctlight_schema.fluctlights.c.created_by_actor_id.foreign_keys))
    revision_actor = next(iter(fluctlight_schema.foundation_revisions.c.actor_id.foreign_keys))
    assert created_by.target_fullname == "public.actors.id"
    assert revision_actor.target_fullname == "public.actors.id"
    assert any(
        isinstance(constraint, ForeignKeyConstraint)
        and constraint.name == "fk_fluctlight_intention_goal_owner"
        for constraint in inner_state_schema.intentions.constraints
    )
