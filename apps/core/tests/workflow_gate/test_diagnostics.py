from __future__ import annotations

import json

from fluctlight_core.workflow_gate.models import GateInput, FailureInjection


def test_stdout_records_reconstruct_recovery_chain(runtime, diagnostics) -> None:
    crashed = runtime.start(
        GateInput(
            intent_key="diagnostics-chain",
            failure=FailureInjection(provider_success_before_checkpoint=True),
        )
    )
    runtime.recover(crashed.workflow_id)
    events = [record.event for record in diagnostics.chain(crashed.workflow_id)]
    assert events == [
        "intent_committed",
        "workflow_started",
        "provider_succeeded",
        "worker_crashed",
        "workflow_recovery_started",
        "provider_result_recovered",
        "result_committed",
    ]
    lines = diagnostics.stream.getvalue().splitlines()
    assert all(json.loads(line)["workflow_id"] == crashed.workflow_id for line in lines)
