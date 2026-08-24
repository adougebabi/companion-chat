from __future__ import annotations

from fluctlight_core.workflow_gate.ids import correlation_id, provider_request_id, workflow_id
from fluctlight_core.workflow_gate.models import GateInput, WorkflowStatus
from fluctlight_core.workflow_gate.queues import QUEUE_POLICIES


def test_stable_ids_are_derived_from_committed_intent() -> None:
    first = GateInput(intent_key="intent-001")
    second = GateInput(intent_key="intent-001")
    assert workflow_id("intent_abc") == workflow_id("intent_abc")
    assert provider_request_id(first.intent_key) == provider_request_id(second.intent_key)
    assert correlation_id("intent_abc").startswith("corr_")


def test_three_queues_have_independent_limits() -> None:
    assert set(QUEUE_POLICIES) == {"interaction", "lifecycle", "media"}
    assert len({policy.concurrency for policy in QUEUE_POLICIES.values()}) > 1
    assert all(policy.rate_limit_per_second > 0 for policy in QUEUE_POLICIES.values())


def test_duplicate_start_reuses_one_workflow(runtime) -> None:
    request = GateInput(intent_key="duplicate", h3_duration_seconds=0)
    first = runtime.start(request)
    second = runtime.start(request)
    assert first.workflow_id == second.workflow_id
    assert first.result_id == second.result_id
    assert runtime.provider.submit_count[first.provider_request_id] == 1
    assert first.status == WorkflowStatus.SUCCEEDED


def test_intent_is_committed_before_execution(runtime) -> None:
    record = runtime.start(GateInput(intent_key="queued"), execute=False)
    assert record.status == WorkflowStatus.QUEUED
    assert runtime.store.intent_exists(record.intent_id)
