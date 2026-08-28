"""FastAPI transport models and NDJSON projection for conversations."""

from __future__ import annotations

import logging
import re
from collections.abc import AsyncIterator
from typing import Any

from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from fluctlight_core.conversations.contracts import (
    ConversationPage,
    ConversationTurn,
    Message,
    TurnResult,
)
from fluctlight_core.conversations.service import ConversationService
from fluctlight_core.transport.ndjson import NdjsonProducer

logger = logging.getLogger(__name__)


class ConversationCreateRequest(BaseModel):
    title: str | None = Field(default=None, max_length=256)
    participant_actor_ids: list[str] = Field(min_length=1, max_length=1)


class ConversationTurnRequest(BaseModel):
    text: str = Field(min_length=1, max_length=32_000)
    fluctlight_id: str = Field(min_length=1, max_length=128)
    attachment_refs: list[str] = Field(default_factory=list, max_length=16)
    idempotency_key: str = Field(min_length=1, max_length=256)
    turn_id: str | None = Field(default=None, min_length=1, max_length=256)


class ReadPositionRequest(BaseModel):
    read_sequence: int = Field(ge=0)
    delivered_sequence: int | None = Field(default=None, ge=0)


def message_response(message: Message) -> dict[str, Any]:
    return {
        "id": message.id,
        "conversation_id": message.conversation_id,
        "sequence": message.sequence,
        "author_actor_id": message.author_actor_id,
        "kind": message.kind.value,
        "text": message.text,
        "attachment_refs": list(message.attachment_refs),
        "created_at": message.created_at.isoformat(),
    }


def page_response(page: ConversationPage) -> dict[str, Any]:
    return {
        "conversation": {
            "id": page.conversation.id,
            "created_by_actor_id": page.conversation.created_by_actor_id,
            "title": page.conversation.title,
            "revision": page.conversation.revision,
            "created_at": page.conversation.created_at.isoformat(),
            "updated_at": page.conversation.updated_at.isoformat(),
        },
        "participants": [
            {
                "conversation_id": participant.conversation_id,
                "actor_id": participant.actor_id,
                "role": participant.role.value,
                "status": participant.status.value,
                "joined_at": participant.joined_at.isoformat(),
                "left_at": participant.left_at.isoformat() if participant.left_at else None,
            }
            for participant in page.participants
        ],
        "messages": [message_response(message) for message in page.messages],
        "next_before_sequence": page.next_before_sequence,
    }


def turn_response(result: TurnResult) -> dict[str, Any]:
    return {
        "turn_id": result.turn.turn_id,
        "correlation_id": result.turn.correlation_id,
        "user_message": message_response(result.user_message),
        "messages": [message_response(message) for message in result.assistant_messages],
    }


async def stream_turn(service: ConversationService, turn: ConversationTurn) -> AsyncIterator[bytes]:
    producer = NdjsonProducer(turn.turn_id)
    try:
        async for event in service.stream_accept_turn(turn):
            if event.type == "action_result" and event.message is not None:
                yield producer.emit(
                    "action_result",
                    {
                        "message": message_response(event.message),
                        "correlation_id": turn.correlation_id,
                    },
                )
            elif event.type == "token" and event.text:
                yield producer.emit("token", {"text": event.text})
            elif event.type == "completed":
                yield producer.emit(
                    "completed", {"turn_id": turn.turn_id, "message_ids": list(event.message_ids)}
                )
    except Exception as exc:
        detail = str(exc).strip()
        error_code = re.sub(r"[^a-z0-9_.-]+", "_", detail.lower()).strip("_")[:120]
        logger.error(
            "conversation.stream.failed turn_id=%s correlation_id=%s error_code=%s "
            "error_type=%s",
            turn.turn_id,
            turn.correlation_id,
            error_code or "turn_unavailable",
            type(exc).__name__,
        )
        yield producer.emit(
            "error",
            {
                "code": error_code or "turn_unavailable",
                "message": "The turn could not be completed",
                "detail": detail or type(exc).__name__,
            },
        )


def turn_stream_response(service: ConversationService, turn: ConversationTurn) -> StreamingResponse:
    return StreamingResponse(stream_turn(service, turn), media_type="application/x-ndjson")
