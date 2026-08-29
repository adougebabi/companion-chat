"""Conversation responder that crosses the public cognition application port."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any

from fluctlight_core.conversations.contracts import (
    ConversationProviderError,
    ConversationTurn,
    MessageDraft,
    MessageKind,
    TurnResponse,
)

from .contracts import CognitionFact, InboxStatus, RealizationResult
from .service import CognitionService


class CognitionTurnResponder:
    _HISTORY_LIMIT = 12
    _HISTORY_TEXT_LIMIT = 800
    _TURN_READY_ATTEMPTS = 120
    _TURN_READY_INTERVAL_SECONDS = 0.25

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
            "life_profile": fluctlight.life_profile.as_payload(),
        }

    async def _fact(self, turn: ConversationTurn, history, fluctlight_id: str) -> CognitionFact:
        history_context = [
            {
                "sequence": message.sequence,
                "speaker": "fluctlight"
                if message.author_actor_id == fluctlight_id
                else "user",
                "kind": message.kind.value,
                "text": message.text[: self._HISTORY_TEXT_LIMIT],
                "truncated": len(message.text) > self._HISTORY_TEXT_LIMIT,
            }
            for message in history[-self._HISTORY_LIMIT :]
        ]
        return CognitionFact(
            id=turn.turn_id,
            fluctlight_id=fluctlight_id,
            event_type="conversation.message",
            payload={
                "text": turn.text,
                "conversation_id": turn.conversation_id,
                "conversation_history": history_context,
                "persona_profile": await self._persona_profile(fluctlight_id),
            },
            causation_id=turn.idempotency_key,
            correlation_id=turn.correlation_id,
            idempotency_key=turn.idempotency_key,
        )

    async def respond(self, turn: ConversationTurn, history) -> TurnResponse:
        if not turn.fluctlight_id:
            raise ConversationProviderError("turn requires an explicit Fluctlight participant")
        handled, replay = await self._prepare_turn(turn, history)
        if handled:
            return self._response_from_realization(replay, turn)
        outcome = await self._process_current_turn(turn)
        if (
            outcome is None
            or outcome.status is not InboxStatus.COMPLETED
            or outcome.realization is None
        ):
            raise ConversationProviderError(
                outcome.error_code if outcome else "turn_not_ready"
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
        handled, replay = await self._prepare_turn(turn, history)
        if handled:
            text = self._text_from_realization(replay)
            if text:
                yield text
            return
        for attempt in range(self._TURN_READY_ATTEMPTS):
            try:
                async for chunk in self._cognition.stream_next(
                    turn.fluctlight_id,
                    worker_id="interaction",
                    expected_fact_id=turn.turn_id,
                ):
                    yield chunk
                return
            except Exception as exc:
                if str(exc) != "turn_not_ready" or attempt == self._TURN_READY_ATTEMPTS - 1:
                    raise
                await asyncio.sleep(self._TURN_READY_INTERVAL_SECONDS)

    async def _prepare_turn(
        self, turn: ConversationTurn, history
    ) -> tuple[bool, RealizationResult | None]:
        """Prepare a new turn or reopen/replay its existing immutable fact."""

        if not turn.fluctlight_id:
            raise ConversationProviderError("turn requires an explicit Fluctlight participant")
        status = await self._cognition.inbox_fact_status(
            turn.turn_id, fluctlight_id=turn.fluctlight_id
        )
        if status is InboxStatus.COMPLETED:
            return True, await self._cognition.completed_realization(turn.turn_id)
        if status is InboxStatus.FAILED:
            await self._cognition.retry_failed_fact(
                turn.turn_id, fluctlight_id=turn.fluctlight_id
            )
        elif status in {InboxStatus.CLAIMED, InboxStatus.FROZEN}:
            raise ConversationProviderError("turn_in_progress")
        elif status is None:
            fact = await self._fact(turn, history, turn.fluctlight_id)
            await self._cognition.enqueue(fact)
        return False, None

    async def _process_current_turn(self, turn: ConversationTurn):
        if not turn.fluctlight_id:
            raise ConversationProviderError("turn requires an explicit Fluctlight participant")
        for attempt in range(self._TURN_READY_ATTEMPTS):
            outcome = await self._cognition.process_next(
                turn.fluctlight_id,
                worker_id="interaction",
                expected_fact_id=turn.turn_id,
            )
            if outcome is not None or attempt == self._TURN_READY_ATTEMPTS - 1:
                return outcome
            await asyncio.sleep(self._TURN_READY_INTERVAL_SECONDS)
        return None

    @staticmethod
    def _text_from_realization(realization: RealizationResult | None) -> str:
        if realization is None:
            return ""
        text = realization.payload.get("text")
        return text.strip() if isinstance(text, str) else ""

    @classmethod
    def _response_from_realization(
        cls, realization: RealizationResult | None, turn: ConversationTurn
    ) -> TurnResponse:
        text = cls._text_from_realization(realization)
        if not text:
            return TurnResponse(())
        return TurnResponse(
            (
                MessageDraft(
                    author_actor_id=turn.fluctlight_id or turn.actor_id,
                    text=text,
                    kind=MessageKind.ASSISTANT,
                    idempotency_key=f"{turn.idempotency_key}:assistant",
                ),
            )
        )
