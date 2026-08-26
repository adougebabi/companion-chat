import asyncio

import pytest
from fluctlight_core.cognition.contracts import CognitionFact, FrozenAction, ActionType
from typing import Any
from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.runtime import ConfiguredProviderRuntime
from fluctlight_core.providers.service import ProviderEndpoint, RoleAssignment


class InvalidAssessmentAdapter:
    async def complete_structured(self, *_args, **_kwargs) -> dict[str, object]:
        return {"assessment": {}}


class DiagnosticsRecorder:
    def __init__(self) -> None:
        self.runs: list[Any] = []

    async def emit_model_run(self, run) -> None:
        self.runs.append(run)


def test_invalid_cognitive_response_records_a_redacted_failed_model_run() -> None:
    runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
    diagnostics = DiagnosticsRecorder()
    runtime._adapter = InvalidAssessmentAdapter()  # type: ignore[assignment]
    runtime._diagnostics = diagnostics  # type: ignore[assignment]
    runtime._provenance_recorder = None

    async def resolve(_role):
        return (
            RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "local", "model", 100, 30),
            ProviderEndpoint("local", "openai-compatible", "http://provider/v1", "provider:local"),
            None,
        )

    runtime._resolve = resolve  # type: ignore[method-assign]
    fact = CognitionFact(
        id="turn-1",
        fluctlight_id="fluctlight-1",
        event_type="conversation.message",
        payload={"text": "hello"},
        causation_id="cause-1",
        correlation_id="corr-1",
        idempotency_key="turn-1",
    )

    with pytest.raises(RuntimeError, match="missing decision"):
        asyncio.run(runtime.assess(fact, correlation_id="corr-1"))

    assert len(diagnostics.runs) == 1
    run = diagnostics.runs[0]
    assert run.status == "failed"
    assert run.error_code == "cognitive_provider_response_is_missing_decision"
    messages = run.prompt["messages"]
    assert messages[0]["role"] == "system"
    assert '"assessment"' in messages[0]["content"]
    assert '"social_signals":[]' in messages[0]["content"]
    assert "semantic.assessment.v1" not in messages[0]["content"]
    assert "evidence_refs" not in messages[0]["content"]
    assert "decision_id" not in messages[0]["content"]
    assert 'response_intent":{}' in messages[0]["content"]
    assert "visible reply text" in messages[0]["content"]


def test_realization_uses_the_factual_source_message_not_cognitive_payload_text() -> None:
    action = FrozenAction(
        action_id="action-1",
        decision_id="decision-1",
        inbox_id="inbox-1",
        fluctlight_id="fluctlight-1",
        action_type=ActionType.REPLY,
        payload={
            "source_text": "请告诉我现在的状态",
            "text": "this must never become the realization input",
        },
        state_revision=1,
        provider_request_id="request-1",
    )

    messages = ConfiguredProviderRuntime._realization_messages(action)

    content = str(messages[1]["content"])
    assert "请告诉我现在的状态" in content
    assert "this must never become the realization input" not in content
