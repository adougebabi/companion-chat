from fluctlight_core.diagnostics import schema as diagnostics_schema
from fluctlight_core.diagnostics.service import DiagnosticsAuthorizationError, DiagnosticsService
from fluctlight_core.platform.persistence import metadata


def test_t05_diagnostics_tables_are_registered_and_owner_check_is_explicit() -> None:
    expected = {
        "diagnostic_events",
        "diagnostic_model_runs",
        "diagnostic_turns",
        "diagnostic_workflow_links",
        "diagnostic_retention",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    service = DiagnosticsService.__new__(DiagnosticsService)
    try:
        service._require_owner("human-a", "human-b")
    except DiagnosticsAuthorizationError:
        pass
    else:
        raise AssertionError("non-owner diagnostics access was accepted")
    assert diagnostics_schema.diagnostic_events.c.payload is not None
    assert diagnostics_schema.diagnostic_model_runs.c.prompt is not None
    assert diagnostics_schema.diagnostic_model_runs.c.response is not None
