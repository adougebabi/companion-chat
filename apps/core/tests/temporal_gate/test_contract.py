from __future__ import annotations

import pytest
from fluctlight_core.temporal_gate.ids import (
    audit_id,
    correlation_id,
    intent_id,
    provider_request_id,
    result_id,
    slug,
    workflow_id,
)
from fluctlight_core.temporal_gate.models import (
    QUEUES,
    FailureInjection,
    GateInput,
    QueuePolicy,
)
from fluctlight_core.temporal_gate.queues import QUEUE_POLICIES


def test_stable_ids_form_one_deterministic_identity_chain() -> None:
    committed_intent = intent_id("intent-001")
    execution_id = workflow_id(committed_intent)

    assert committed_intent == intent_id("intent-001")
    assert execution_id == workflow_id(committed_intent)
    assert provider_request_id(committed_intent).startswith("provider_")
    assert correlation_id(committed_intent).startswith("corr_")
    assert result_id(execution_id).startswith("result_")
    assert audit_id("owner:local", "pause", execution_id) == audit_id(
        "owner:local", "pause", execution_id
    )
    assert execution_id != workflow_id(intent_id("intent-002"))


def test_slug_normalizes_human_input_and_has_an_empty_fallback() -> None:
    assert slug("Gate / H3 Result 01") == "gate-h3-result-01"
    assert slug("!!!") == "unknown"


def test_gate_input_round_trips_nested_failure_contract() -> None:
    request = GateInput(
        intent_key="contract-round-trip",
        queue="interaction",
        sleep_seconds=2.5,
        h3_duration_seconds=4.0,
        heartbeat_interval_seconds=1.0,
        timeout_seconds=8.0,
        decision_version="gate-v2",
        continue_after=3,
        failure=FailureInjection(
            provider_success_before_checkpoint=True,
            crash_before_result_commit=True,
        ),
    )

    assert GateInput.from_dict(request.as_dict()) == request


def test_gate_input_ignores_continue_as_new_control_fields() -> None:
    restored = GateInput.from_dict(
        {
            "intent_key": "continue",
            "continue_iteration": 2,
            "pending_signals": ["pause"],
        }
    )
    assert restored.intent_key == "continue"
    assert restored.continue_after == 0


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("intent_key", "   ", "intent_key"),
        ("queue", "unknown", "queue"),
        ("sleep_seconds", -1.0, "durations"),
        ("h3_duration_seconds", -1.0, "durations"),
        ("heartbeat_interval_seconds", 0.0, "heartbeat interval"),
        ("timeout_seconds", 0.0, "heartbeat interval"),
        ("continue_after", -1, "continue_after"),
    ],
)
def test_gate_input_rejects_invalid_values(field: str, value: object, message: str) -> None:
    kwargs: dict[str, object] = {"intent_key": "valid"}
    kwargs[field] = value

    with pytest.raises(ValueError, match=message):
        GateInput(**kwargs)  # type: ignore[arg-type]


def test_queue_contract_has_three_named_policies() -> None:
    assert set(QUEUE_POLICIES) == set(QUEUES) == {"interaction", "lifecycle", "media"}
    assert len({policy.concurrency for policy in QUEUE_POLICIES.values()}) > 1
    assert all(policy.rate_limit_per_second > 0 for policy in QUEUE_POLICIES.values())

    with pytest.raises(ValueError, match="unknown task queue"):
        QueuePolicy("other", concurrency=1, rate_limit_per_second=1.0)
