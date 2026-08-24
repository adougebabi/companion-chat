"""Redis Streams adapter for committed events and ephemeral progress only."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

EVENT_STREAM = "fluctlight:events:v1"
PROGRESS_STREAM = "fluctlight:progress:v1"


class RedisStreams:
    def __init__(self, client: Any) -> None:
        self.client = client

    async def publish_event(self, envelope: dict[str, str]) -> str:
        required = {"event_id", "event_type", "schema_version", "aggregate_sequence"}
        if missing := sorted(key for key in required if not envelope.get(key)):
            raise ValueError(f"event envelope missing: {', '.join(missing)}")
        return await self.client.xadd(EVENT_STREAM, envelope)

    async def publish_progress(self, payload: dict[str, str], *, maxlen: int = 1_000) -> str:
        return await self.client.xadd(PROGRESS_STREAM, payload, maxlen=maxlen, approximate=True)

    async def consume_once(
        self,
        *,
        group: str,
        consumer: str,
        handler: Callable[[str, dict[str, str]], Awaitable[None]],
    ) -> int:
        messages = await self.client.xreadgroup(group, consumer, {EVENT_STREAM: ">"}, count=10)
        applied = 0
        for _, entries in messages:
            for stream_id, fields in entries:
                await handler(stream_id, fields)
                await self.client.xack(EVENT_STREAM, group, stream_id)
                applied += 1
        return applied
