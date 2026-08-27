"""Conversation responder that crosses the public cognition application port."""

from __future__ import annotations

from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any

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
    def __init__(
        self,
        cognition: CognitionService,
        fluctlight_lookup: Callable[[str], Awaitable[Any]] | None = None,
    ) -> None:
        self._cognition = cognition
        self._fluctlight_lookup = fluctlight_lookup

    async def _persona_profile(self, fluctlight_id: str) -> dict[str, Any]:
        if self._fluctlight_lookup is None:
            return {}
        fluctlight = await self._fluctlight_lookup(fluctlight_id)
        identity = fluctlight.identity.as_payload()
        # This profile becomes durable fact/action context, so bound identity prose.
        bounded_identity = {
            key: (value[:1200] if isinstance(value, str) else value)
            for key, value in identity.items()
            if key != "id"
        }
        return {
            "identity": bounded_identity,
            "personality": fluctlight.personality.as_payload(),
            "behavioral_policy": fluctlight.behavioral_policy.as_payload(),
        }

    async def _fact(self, turn: ConversationTurn, history, fluctlight_id: str) -> CognitionFact:
        return CognitionFact(
            id=turn.turn_id,
            fluctlight_id=fluctlight_id,
            event_type="conversation.message",
            payload={
                "text": turn.text,
                "conversation_id": turn.conversation_id,
                "history_message_ids": [message.id for message in history],
                "persona_profile": await self._persona_profile(fluctlight_id),
            },
            causation_id=turn.idempotency_key,
            correlation_id=turn.correlation_id,
            idempotency_key=turn.idempotency_key,
        )

    async def respond(self, turn: ConversationTurn, history) -> TurnResponse:
        if not turn.fluctlight_id:
            raise ConversationProviderError("turn requires an explicit Fluctlight participant")
        fact = await self._fact(turn, history, turn.fluctlight_id)
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
        fact = await self._fact(turn, history, turn.fluctlight_id)
        await self._cognition.enqueue(fact)
        async for chunk in self._cognition.stream_next(turn.fluctlight_id, worker_id="interaction"):
            yield chunk
