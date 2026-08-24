from __future__ import annotations

from fluctlight_core.workflow_gate.models import FailureInjection, GateInput, WorkflowStatus


def test_durable_sleep_can_resume_after_worker_restart(runtime) -> None:
    request = GateInput(intent_key="sleep", sleep_seconds=30)
    sleeping = runtime.start(request)
    assert sleeping.status == WorkflowStatus.SLEEPING
    resumed = runtime.execute(sleeping.workflow_id)
    assert resumed.status == WorkflowStatus.SUCCEEDED


def test_recovery_does_not_skip_an_unexpired_durable_sleep() -> None:
    now = [100.0]
    from fluctlight_core.workflow_gate.runtime import GateRuntime

    runtime = GateRuntime(clock=lambda: now[0])
    sleeping = runtime.start(GateInput(intent_key="sleep-before-due", sleep_seconds=30))
    restarted = GateRuntime(store=runtime.store, provider=runtime.provider, clock=lambda: now[0])

    assert restarted.recover(sleeping.workflow_id).status == WorkflowStatus.SLEEPING
    now[0] += 30
    assert restarted.recover(sleeping.workflow_id).status == WorkflowStatus.SUCCEEDED


def test_provider_success_before_checkpoint_recovers_one_effect(runtime) -> None:
    request = GateInput(
        intent_key="provider-window",
        failure=FailureInjection(provider_success_before_checkpoint=True),
    )
    crashed = runtime.start(request)
    assert crashed.status == WorkflowStatus.CRASHED
    recovered = runtime.recover(crashed.workflow_id)
    assert recovered.status == WorkflowStatus.SUCCEEDED
    assert runtime.provider.submit_count[recovered.provider_request_id] == 1
    assert recovered.result_id == runtime.recover(recovered.workflow_id).result_id


def test_process_death_after_provider_checkpoint_commits_result_once(runtime) -> None:
    request = GateInput(
        intent_key="checkpoint-window",
        failure=FailureInjection(crash_after_provider_checkpoint=True),
    )
    crashed = runtime.start(request)
    assert crashed.status == WorkflowStatus.CRASHED
    recovered = runtime.recover(crashed.workflow_id)
    assert recovered.status == WorkflowStatus.SUCCEEDED
    assert runtime.provider.submit_count[recovered.provider_request_id] == 1


def test_recovery_rehydrates_committed_input_after_worker_restart(runtime) -> None:
    request = GateInput(
        intent_key="rehydrate-input",
        failure=FailureInjection(provider_success_before_checkpoint=True),
    )
    crashed = runtime.start(request)
    assert crashed.status == WorkflowStatus.CRASHED

    restarted = type(runtime)(store=runtime.store, provider=runtime.provider)
    recovered = restarted.recover(crashed.workflow_id)

    assert recovered.status == WorkflowStatus.SUCCEEDED
    assert recovered.result_id == restarted.recover(crashed.workflow_id).result_id
    assert runtime.provider.submit_count[recovered.provider_request_id] == 1


def test_heartbeat_timeout_and_cooperative_cancel(runtime) -> None:
    timeout = runtime.start(
        GateInput(
            intent_key="timeout",
            h3_duration_seconds=10,
            failure=FailureInjection(timeout=True),
        )
    )
    canceled = runtime.start(
        GateInput(
            intent_key="cancel",
            h3_duration_seconds=10,
            failure=FailureInjection(cancel=True),
        )
    )
    assert timeout.status == WorkflowStatus.FAILED
    assert canceled.status == WorkflowStatus.CANCELED
