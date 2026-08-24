from __future__ import annotations

import asyncio

from fluctlight_core.transport.ndjson import NdjsonProducer, parse_ndjson


async def chunks(*values: bytes):
    for value in values:
        yield value


def test_ndjson_parser_handles_split_utf8_and_terminal_event() -> None:
    producer = NdjsonProducer("turn-1")
    token = producer.emit("token", {"text": "你好"})
    completed = producer.emit("completed", {"status": "ok"})

    async def collect():
        return [event async for event in parse_ndjson(chunks(token[:10], token[10:] + completed))]

    events = asyncio.run(collect())
    assert [event.type for event in events] == ["token", "completed"]
    assert [event.sequence for event in events] == [0, 1]
