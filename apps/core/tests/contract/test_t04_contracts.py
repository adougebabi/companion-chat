from fluctlight_core.fluctlights import schema as fluctlight_schema
from fluctlight_core.inner_state import schema as inner_state_schema
from fluctlight_core.platform.persistence import metadata


def test_t04_tables_use_the_single_public_metadata_graph() -> None:
    expected = {
        "fluctlights",
        "fluctlight_foundation_revisions",
        "fluctlight_foundation_governance",
        "fluctlight_inner_states",
        "fluctlight_inner_state_events",
        "fluctlight_goals",
        "fluctlight_intentions",
        "fluctlight_intention_governance",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    assert all(
        table.schema == "public"
        for table in (
            fluctlight_schema.fluctlights,
            fluctlight_schema.foundation_revisions,
            fluctlight_schema.foundation_governance,
            inner_state_schema.inner_states,
            inner_state_schema.inner_state_events,
            inner_state_schema.goals,
            inner_state_schema.intentions,
            inner_state_schema.intention_governance,
        )
    )
