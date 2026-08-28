import asyncio
from typing import Any, cast

from fluctlight_core.cognition.contracts import InboxStatus, ProcessOutcome, RealizationResult
from fluctlight_core.cognition.turn_responder import CognitionTurnResponder
from fluctlight_core.conversations.contracts import ConversationTurn


class _Cognition:
    def __init__(self, status: InboxStatus | None = None) -> None:
        self.status = status
        self.fact: Any = None
        self.process_expected_fact_id: str | None = None
        self.retry_expected_fact_id: str | None = None

    async def inbox_fact_status(self, fact_id: str, *, fluctlight_id: str):
        assert fluctlight_id == "fluctlight-1"
        assert fact_id == "turn-1"
        return self.status

    async def completed_realization(self, fact_id: str):
        assert fact_id == "turn-1"
        return RealizationResult("provider-1", {"text": "replayed"})

    async def retry_failed_fact(self, fact_id: str, *, fluctlight_id: str):
        assert fluctlight_id == "fluctlight-1"
        self.retry_expected_fact_id = fact_id
        self.status = InboxStatus.PENDING

    async def enqueue(self, fact):
        self.fact = fact

    async def process_next(self, fluctlight_id: str, *, worker_id: str, expected_fact_id: str):
        assert (fluctlight_id, worker_id) == ("fluctlight-1", "interaction")
        self.process_expected_fact_id = expected_fact_id
        return ProcessOutcome(
            InboxStatus.COMPLETED,
            realization=RealizationResult("provider-1", {"text": "processed"}),
        )

    async def stream_next(self, fluctlight_id: str, *, worker_id: str, expected_fact_id: str):
        assert (fluctlight_id, worker_id, expected_fact_id) == (
            "fluctlight-1",
            "interaction",
            "turn-1",
        )
        yield "streamed"


def _turn() -> ConversationTurn:
    return ConversationTurn(
        conversation_id="conversation-1",
        actor_id="human-1",
        text="hello",
        fluctlight_id="fluctlight-1",
        idempotency_key="turn-key-1",
        turn_id="turn-1",
        correlation_id="corr-1",
    )


def test_responder_claims_the_fact_for_the_current_turn() -> None:
    cognition = _Cognition()
    result = asyncio.run(CognitionTurnResponder(cast(Any, cognition)).respond(_turn(), ()))

    assert result.messages[0].text == "processed"
    assert cognition.fact.id == "turn-1"
    assert cognition.fact.payload["conversation_id"] == "conversation-1"
    assert cognition.process_expected_fact_id == "turn-1"


def test_responder_retries_failed_fact_in_place() -> None:
    cognition = _Cognition(InboxStatus.FAILED)
    result = asyncio.run(CognitionTurnResponder(cast(Any, cognition)).respond(_turn(), ()))

    assert result.messages[0].text == "processed"
    assert cognition.retry_expected_fact_id == "turn-1"
    assert cognition.fact is None
    assert cognition.process_expected_fact_id == "turn-1"


def test_responder_replays_completed_realization_without_reassessing() -> None:
    cognition = _Cognition(InboxStatus.COMPLETED)
    result = asyncio.run(CognitionTurnResponder(cast(Any, cognition)).respond(_turn(), ()))

    assert result.messages[0].text == "replayed"
    assert cognition.fact is None
    assert cognition.process_expected_fact_id is None


def test_stream_responder_passes_current_fact_id_to_cognition() -> None:
    cognition = _Cognition()

    async def collect() -> list[str]:
        return [
            chunk
            async for chunk in CognitionTurnResponder(cast(Any, cognition)).stream_respond(
                _turn(), ()
            )
        ]

    assert asyncio.run(collect()) == ["streamed"]
