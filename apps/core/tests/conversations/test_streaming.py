import asyncio
from dataclasses import replace
from datetime import UTC, datetime
from typing import Any, cast

import pytest
from fluctlight_core.cognition.contracts import (
    ActionType,
    AssessmentEnvelope,
    CognitionFact,
    DecisionEffect,
    DecisionProposal,
    FrozenAction,
    InboxClaim,
    ProviderExecutionError,
    RealizationResult,
    stable_provider_request_id,
)
from fluctlight_core.cognition.service import CognitionService
from fluctlight_core.conversations.contracts import (
    ConversationTurn,
    Message,
    MessageKind,
    TurnStreamEvent,
)
from fluctlight_core.conversations.service import ConversationService
from fluctlight_core.transport.conversations import stream_turn


def _fact() -> CognitionFact:
    return CognitionFact(
        id="turn-1",
        fluctlight_id="fluctlight-1",
        event_type="conversation.message",
        payload={"text": "hello"},
        causation_id="message-1",
        correlation_id="corr-1",
        idempotency_key="turn-1",
        occurred_at=datetime.now(UTC),
    )


def _claim() -> InboxClaim:
    return InboxClaim(_fact(), sequence=1, attempt=1, worker_id="interaction")


def _envelope() -> AssessmentEnvelope:
    from fluctlight_core.inner_state.contracts import (
        AffectDirection,
        Appraisal,
        SemanticAssessment,
        SemanticPerception,
    )
    from fluctlight_core.providers.contracts import ModelRole, ProviderProvenance

    fact = _fact()
    assessment = SemanticAssessment(
        schema_version="semantic.assessment.v1",
        perception=SemanticPerception(event_kind="message", observed_intent="inform"),
        appraisal=Appraisal(
            relevance=0.5,
            goal_congruence=0.5,
            reward=0.5,
            loss=0.1,
            social_threat=0.1,
            controllability=0.5,
            responsibility=0.5,
            relationship_significance=0.5,
            expected_effect=0.5,
        ),
        direction=AffectDirection.NEUTRAL,
        strength=0.5,
        confidence=0.9,
        evidence_refs=(fact.id,),
        model="fixture-model",
        model_version="fixture",
        prompt_version="cognition.v1",
        source_event_id=fact.id,
        idempotency_key=fact.idempotency_key,
    )
    decision = DecisionProposal(
        action_type=ActionType.REPLY,
        payload={"text": "hello"},
        confidence=0.9,
        evidence_refs=(fact.id,),
        decision_id="decision-1",
    )
    return AssessmentEnvelope(
        assessment=assessment,
        decision=decision,
        provenance=ProviderProvenance(
            role=ModelRole.COGNITIVE_ASSESSMENT,
            endpoint_id="fixture",
            model_id="fixture-model",
            prompt_version="cognition.v1",
            schema_version="semantic.assessment.v1",
            correlation_id=fact.correlation_id,
            token_budget=100,
        ),
        correlation_id=fact.correlation_id,
    )


def _action() -> FrozenAction:
    return FrozenAction(
        action_id="action-1",
        decision_id="decision-1",
        inbox_id="turn-1",
        fluctlight_id="fluctlight-1",
        action_type=ActionType.REPLY,
        payload={"text": "hello"},
        state_revision=1,
        provider_request_id=stable_provider_request_id("action-1"),
    )


class StreamingProvider:
    async def stream_realize(self, action: FrozenAction, *, correlation_id: str):
        yield "hello "
        yield "world"

    async def realize(self, action: FrozenAction, *, correlation_id: str) -> RealizationResult:
        return RealizationResult(action.provider_request_id, {"text": "fallback"})


class BlockingStreamingProvider(StreamingProvider):
    def __init__(self) -> None:
        self.released = asyncio.Event()

    async def stream_realize(self, action: FrozenAction, *, correlation_id: str):
        yield "first"
        await self.released.wait()


def _service(provider: object) -> CognitionService:
    service = CognitionService.__new__(CognitionService)
    setattr(service, "_assessment_provider", object())
    setattr(service, "_realization_provider", provider)
    setattr(service, "_diagnostics", None)
    return service


def test_stream_next_yields_provider_chunks_before_success_settlement(monkeypatch) -> None:
    service = _service(StreamingProvider())
    settled: list[tuple[str, str]] = []

    async def claim_next(*_args, **_kwargs):
        return _claim()

    async def assess(*_args, **_kwargs):
        return _envelope()

    async def freeze(*_args, **_kwargs):
        return _action()

    monkeypatch.setattr(service, "claim_next", claim_next)
    monkeypatch.setattr(
        service, "_assessment_provider", type("Assessment", (), {"assess": assess})()
    )
    monkeypatch.setattr(service, "_freeze", freeze)

    async def settle_success(*_args, **_kwargs) -> None:
        settled.append(("success", "completed"))

    monkeypatch.setattr(service, "_settle_success", settle_success)

    async def collect() -> list[str]:
        return [
            chunk async for chunk in service.stream_next("fluctlight-1", worker_id="interaction")
        ]

    events = asyncio.run(collect())
    assert events == ["hello ", "world"]
    assert settled == [("success", "completed")]


def test_stream_next_processes_secondary_media_after_primary_no_op(monkeypatch) -> None:
    service = _service(StreamingProvider())
    envelope = _envelope()
    secondary = DecisionEffect(
        "media",
        ActionType.MEDIA_REQUEST,
        {"media_request": {"scene": "室内"}, "conversation_id": "conversation-1"},
    )
    no_op = DecisionProposal(
        action_type=ActionType.NO_OP,
        payload={},
        confidence=0.9,
        evidence_refs=("turn-1",),
        decision_id="decision-1",
        effects=(DecisionEffect("no-op", ActionType.NO_OP, {}), secondary),
    )
    envelope = replace(envelope, decision=no_op)
    processed: list[str] = []

    async def claim_next(*_args, **_kwargs):
        return _claim()

    async def assess(*_args, **_kwargs):
        return envelope

    async def freeze(*_args, **_kwargs):
        return replace(_action(), action_type=ActionType.NO_OP, payload={})

    async def secondary_effects(*_args, **_kwargs):
        processed.append("media")
        return ()

    async def settle_success(*_args, **_kwargs):
        processed.append("settled")

    monkeypatch.setattr(service, "claim_next", claim_next)
    monkeypatch.setattr(
        service, "_assessment_provider", type("Assessment", (), {"assess": assess})()
    )
    monkeypatch.setattr(service, "_freeze", freeze)
    monkeypatch.setattr(service, "_process_secondary_effects", secondary_effects)
    monkeypatch.setattr(service, "_settle_success", settle_success)

    async def collect() -> list[str]:
        return [
            chunk
            async for chunk in service.stream_next("fluctlight-1", worker_id="interaction")
        ]

    assert asyncio.run(collect()) == []
    assert processed == ["media", "settled"]


def test_stream_next_marks_realization_cancelled_and_propagates_cancel(monkeypatch) -> None:
    provider = BlockingStreamingProvider()
    service = _service(provider)
    failures: list[str] = []

    async def claim_next(*_args, **_kwargs):
        return _claim()

    async def assess(*_args, **_kwargs):
        return _envelope()

    async def freeze(*_args, **_kwargs):
        return _action()

    monkeypatch.setattr(service, "claim_next", claim_next)
    monkeypatch.setattr(
        service, "_assessment_provider", type("Assessment", (), {"assess": assess})()
    )
    monkeypatch.setattr(service, "_freeze", freeze)

    async def settle_failure(*_args, **kwargs) -> None:
        failures.append(str(kwargs.get("error_code", _args[1] if len(_args) > 1 else "")))

    monkeypatch.setattr(service, "_settle_failure", settle_failure)

    async def consume() -> list[str]:
        return [
            chunk async for chunk in service.stream_next("fluctlight-1", worker_id="interaction")
        ]

    async def run() -> None:
        task = asyncio.create_task(consume())
        await asyncio.sleep(0)
        await asyncio.sleep(0)
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task

    asyncio.run(run())
    assert failures == ["realization_cancelled"]


def test_stream_next_rejects_when_the_requested_turn_is_not_the_inbox_head(monkeypatch) -> None:
    service = _service(StreamingProvider())

    async def claim_next(*_args, **_kwargs):
        return None

    monkeypatch.setattr(service, "claim_next", claim_next)

    async def collect() -> list[str]:
        return [
            chunk
            async for chunk in service.stream_next(
                "fluctlight-1",
                worker_id="interaction",
                expected_fact_id="turn-1",
            )
        ]

    with pytest.raises(ProviderExecutionError, match="turn_not_ready"):
        asyncio.run(collect())


def test_transport_stream_preserves_order_and_single_terminal() -> None:
    user = Message(
        id="message-1",
        conversation_id="conversation-1",
        sequence=1,
        author_actor_id="human-1",
        text="hello",
        kind=MessageKind.USER,
    )

    class Service:
        async def stream_accept_turn(self, _turn: ConversationTurn):
            yield TurnStreamEvent("action_result", message=user)
            yield TurnStreamEvent("token", text="hello ")
            yield TurnStreamEvent("token", text="world")
            yield TurnStreamEvent("completed", message_ids=("assistant-1",))

    async def collect() -> list[bytes]:
        return [
            chunk
            async for chunk in stream_turn(
                cast(
                    ConversationService,
                    cast(Any, Service()),
                ),
                ConversationTurn("conversation-1", "human-1", "hello", turn_id="turn-1"),
            )
        ]

    import json

    chunks = asyncio.run(collect())
    events = [json.loads(chunk) for chunk in chunks]
    assert [event["type"] for event in events] == ["action_result", "token", "token", "completed"]
    assert [event["sequence"] for event in events] == [0, 1, 2, 3]
