"""Conversation responder that crosses the public cognition application port."""

from __future__ import annotations

from collections.abc import AsyncIterator

from fluctlight_core.conversations.contracts import (
    ConversationProviderError,
    ConversationTurn,
    MessageDraft,
    MessageKind,
    TurnResponse,
)

from .contracts import CognitionFact, InboxStatus
from .service import CognitionService


class CognitionTurnResponder:
    def __init__(self, cognition: CognitionService) -> None:
        self._cognition = cognition

    async def respond(self, turn: ConversationTurn, history) -> TurnResponse:
        if not turn.fluctlight_id:
            raise ConversationProviderError("turn requires an explicit Fluctlight participant")
        fact = CognitionFact(
            id=turn.turn_id,
            fluctlight_id=turn.fluctlight_id,
            event_type="conversation.message",
            payload={"text": turn.text, "history_message_ids": [message.id for message in history]},
            causation_id=turn.idempotency_key,
            correlation_id=turn.correlation_id,
            idempotency_key=turn.idempotency_key,
        )
        await self._cognition.enqueue(fact)
        outcome = await self._cognition.process_next(turn.fluctlight_id, worker_id="interaction")
        if (
            outcome is None
            or outcome.status is not InboxStatus.COMPLETED
            or outcome.realization is None
        ):
            raise ConversationProviderError(
                outcome.error_code if outcome else "cognition_not_processed"
            )
        text = outcome.realization.payload.get("text")
        if not isinstance(text, str) or not text.strip():
            return TurnResponse(())
        return TurnResponse(
            (
                MessageDraft(
                    author_actor_id=turn.fluctlight_id,
                    text=text,
                    kind=MessageKind.ASSISTANT,
                    idempotency_key=f"{turn.idempotency_key}:assistant",
                ),
            )
        )

    async def stream_respond(self, turn: ConversationTurn, history) -> AsyncIterator[str]:
        if not turn.fluctlight_id:
            raise ConversationProviderError("turn requires an explicit Fluctlight participant")
        fact = CognitionFact(
            id=turn.turn_id,
            fluctlight_id=turn.fluctlight_id,
            event_type="conversation.message",
            payload={"text": turn.text, "history_message_ids": [message.id for message in history]},
            causation_id=turn.idempotency_key,
            correlation_id=turn.correlation_id,
            idempotency_key=turn.idempotency_key,
        )
        await self._cognition.enqueue(fact)
        async for chunk in self._cognition.stream_next(turn.fluctlight_id, worker_id="interaction"):
            yield chunk
