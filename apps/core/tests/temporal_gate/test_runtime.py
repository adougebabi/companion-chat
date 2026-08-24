from __future__ import annotations

import pytest
from fluctlight_core.temporal_gate.models import (
    FailureInjection,
    GateInput,
    RepairCommand,
    WorkflowStatus,
)
from fluctlight_core.temporal_gate.runtime import TemporalGateRuntime


def test_duplicate_start_reuses_one_workflow_and_one_provider_effect(runtime) -> None:
    request = GateInput(intent_key="duplicate", h3_duration_seconds=0)

    first = runtime.start(request)
    second = runtime.start(request)

    assert first.workflow_id == second.workflow_id
    assert first.result_id == second.result_id
    assert first.provider_request_id == second.provider_request_id
    assert runtime.provider.submit_count[first.provider_request_id] == 1
    assert first.status is WorkflowStatus.SUCCEEDED


def test_intent_is_committed_before_execution(runtime) -> None:
    record = runtime.start(GateInput(intent_key="queued"), execute=False)

    assert record.status is WorkflowStatus.QUEUED
    assert runtime.store.intent_exists(record.intent_id)
    assert record.provider_request_id is not None


def test_unexpired_durable_timer_survives_runtime_reconstruction(runtime) -> None:
    now = [100.0]
    clocked = TemporalGateRuntime(
        store=runtime.store,
        provider=runtime.provider,
        diagnostics=runtime.diagnostics,
        queue_limits=runtime.queue_limits,
        clock=lambda: now[0],
    )

    sleeping = clocked.start(GateInput(intent_key="timer", sleep_seconds=30))
    restarted = TemporalGateRuntime(
        store=runtime.store,
        provider=runtime.provider,
        diagnostics=runtime.diagnostics,
        queue_limits=runtime.queue_limits,
        clock=lambda: now[0],
    )

    assert sleeping.status is WorkflowStatus.SLEEPING
    assert restarted.recover(sleeping.workflow_id).status is WorkflowStatus.SLEEPING

    now[0] += 30
    resumed = restarted.recover(sleeping.workflow_id)
    assert resumed.status is WorkflowStatus.SUCCEEDED


@pytest.mark.parametrize(
    "failure",
    [
        FailureInjection(provider_success_before_checkpoint=True),
        FailureInjection(crash_after_provider_checkpoint=True),
        FailureInjection(crash_before_result_commit=True),
    ],
)
def test_provider_and_result_boundaries_recover_idempotently(runtime, failure) -> None:
    crashed = runtime.start(GateInput(intent_key="recovery", failure=failure))
    assert crashed.status is WorkflowStatus.CRASHED

    recovered = runtime.recover(crashed.workflow_id)
    again = runtime.recover(crashed.workflow_id)

    assert recovered.status is WorkflowStatus.SUCCEEDED
    assert again.result_id == recovered.result_id
    assert runtime.provider.submit_count[recovered.provider_request_id] == 1


def test_heartbeat_timeout_and_cooperative_cancel_are_distinct_outcomes(runtime) -> None:
    healthy = runtime.start(
        GateInput(
            intent_key="heartbeat",
            h3_duration_seconds=3,
            heartbeat_interval_seconds=1,
        )
    )
    timeout = runtime.start(
        GateInput(
            intent_key="timeout",
            h3_duration_seconds=10,
            heartbeat_interval_seconds=1,
            failure=FailureInjection(timeout=True),
        )
    )
    canceled = runtime.start(
        GateInput(
            intent_key="cancel",
            h3_duration_seconds=10,
            heartbeat_interval_seconds=1,
            failure=FailureInjection(cancel=True),
        )
    )

    assert healthy.status is WorkflowStatus.SUCCEEDED
    assert runtime.provider.heartbeat_count[healthy.provider_request_id] == 3
    assert timeout.status is WorkflowStatus.FAILED
    assert canceled.status is WorkflowStatus.CANCELED


def test_pause_resume_query_repair_reset_and_continue_as_new(runtime) -> None:
    record = runtime.start(GateInput(intent_key="management"), execute=False)

    runtime.pause(record.workflow_id)
    assert runtime.query(record.workflow_id).status is WorkflowStatus.PAUSED
    assert runtime.execute(record.workflow_id).status is WorkflowStatus.PAUSED

    queued = runtime.resume(record.workflow_id)
    assert queued.status is WorkflowStatus.QUEUED
    result = runtime.execute(record.workflow_id)
    assert result.status is WorkflowStatus.SUCCEEDED

    accepted = runtime.repair(
        record.workflow_id,
        command=RepairCommand("owner-approved repair", expected_version="gate-v1"),
    )
    conflict = runtime.repair(
        record.workflow_id,
        command=RepairCommand("stale repair", expected_version="gate-v0"),
    )
    assert accepted.accepted
    assert not conflict.accepted

    reset = runtime.reset(record.workflow_id, history_point=4)
    assert reset.status is WorkflowStatus.QUEUED
    assert reset.metadata["reset_from_event"] == 4
    continued = runtime.continue_as_new(record.workflow_id, ("pause", "resume"))
    assert continued.metadata["continue_as_new"] is True
    assert continued.metadata["pending_signals"] == ("pause", "resume")


def test_management_operations_reject_invalid_history_points(runtime) -> None:
    record = runtime.start(GateInput(intent_key="bad-reset"), execute=False)

    with pytest.raises(ValueError, match="non-negative"):
        runtime.reset(record.workflow_id, history_point=-1)

    with pytest.raises(ValueError, match="audit reason"):
        runtime.repair(record.workflow_id, command=RepairCommand(" "))
