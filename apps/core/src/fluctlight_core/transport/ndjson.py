"""Strict internal Core-to-BFF NDJSON framing."""

from __future__ import annotations

import codecs
import json
from collections.abc import AsyncIterator
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field

TerminalType = Literal["completed", "error"]


class VisibleStreamEventV1(BaseModel):
    model_config = ConfigDict(extra="forbid")

    type: Literal["token", "action_result", "completed", "error", "heartbeat"]
    turn_id: str = Field(min_length=1, max_length=128)
    sequence: int = Field(ge=0)
    payload: dict[str, Any]


class StreamContractError(ValueError):
    pass


class NdjsonProducer:
    def __init__(self, turn_id: str) -> None:
        self.turn_id = turn_id
        self._next_sequence = 0
        self._terminal = False

    def emit(
        self,
        event_type: Literal["token", "action_result", "completed", "error", "heartbeat"],
        payload: dict[str, Any],
    ) -> bytes:
        if self._terminal:
            raise StreamContractError("stream has already emitted a terminal event")
        event = VisibleStreamEventV1(
            type=event_type,
            turn_id=self.turn_id,
            sequence=self._next_sequence,
            payload=payload,
        )
        self._next_sequence += 1
        if event_type in {"completed", "error"}:
            self._terminal = True
        return event.model_dump_json().encode("utf-8") + b"\n"


async def parse_ndjson(chunks: AsyncIterator[bytes]) -> AsyncIterator[VisibleStreamEventV1]:
    decoder = codecs.getincrementaldecoder("utf-8")()
    buffer = ""
    turn_id: str | None = None
    expected_sequence = 0
    terminal = False
    async for chunk in chunks:
        buffer += decoder.decode(chunk)
        while "\n" in buffer:
            line, buffer = buffer.split("\n", 1)
            if not line:
                continue
            if terminal:
                raise StreamContractError("data after terminal event")
            try:
                event = VisibleStreamEventV1.model_validate_json(line)
            except ValueError as exc:
                raise StreamContractError("invalid Core NDJSON event") from exc
            if turn_id is None:
                turn_id = event.turn_id
            if event.turn_id != turn_id or event.sequence != expected_sequence:
                raise StreamContractError("Core NDJSON sequence is not monotonic")
            expected_sequence += 1
            terminal = event.type in {"completed", "error"}
            yield event
    buffer += decoder.decode(b"", final=True)
    if buffer.strip():
        raise StreamContractError("incomplete NDJSON frame")
    if not terminal:
        raise StreamContractError("Core NDJSON stream ended without a terminal event")


def encode_event(event: VisibleStreamEventV1) -> bytes:
    return json.dumps(
        event.model_dump(), ensure_ascii=False, separators=(",", ":")
    ).encode("utf-8") + b"\n"
