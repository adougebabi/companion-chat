from fluctlight_core.autonomy import schema as autonomy_schema  # noqa: F401
from fluctlight_core.life_world import schema as life_world_schema
from fluctlight_core.platform.persistence import metadata


def test_t08_tables_share_public_metadata() -> None:
    expected = {
        "life_events",
        "life_schedules",
        "life_schedule_items",
        "life_presence_overlays",
        "autonomy_policies",
        "autonomy_actions",
        "autonomy_governance",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    assert life_world_schema.schedules.c.local_date is not None
