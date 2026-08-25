from fluctlight_core.operations.backup import CleanupPlan, TemporalRestorePlan


def test_t11_restore_plan_separates_temporal_databases() -> None:
    plan = TemporalRestorePlan("temporal", "temporal_visibility", "default", ("workflow-1",))
    assert plan.default_database != plan.visibility_database
    assert CleanupPlan(None, (), (), dry_run=True).dry_run is True
