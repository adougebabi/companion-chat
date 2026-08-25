from fluctlight_core.cognition import schema as cognition_schema
from fluctlight_core.platform.persistence import metadata


def test_t05_cognition_tables_share_the_public_metadata_graph() -> None:
    expected = {
        "cognition_inbox_heads",
        "cognition_inbox",
        "cognition_assessments",
        "cognition_decision_proposals",
        "cognition_frozen_actions",
        "cognition_reflection_windows",
        "cognition_reflection_proposals",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    assert cognition_schema.inbox.c.fluctlight_id is not None
    assert any(
        constraint.name == "cognition_inbox_fluctlight_sequence"
        for constraint in cognition_schema.inbox.constraints
    )
