#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-redis-gap-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$script_dir/read-compose-env.sh"
assert_disposable_compose_project "$project"
compose_started=0

cleanup() {
  if [[ "$compose_started" != 1 ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

compose_started=1
"${compose[@]}" up --build --detach
ready=0
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core python -c 'import urllib.request; urllib.request.urlopen("http://127.0.0.1:8080/health/ready")' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != 1 ]]; then
  "${compose[@]}" ps
  "${compose[@]}" logs --no-color core worker >&2 || true
  exit 1
fi

"${compose[@]}" exec -T core uv run --no-sync python - <<'PY'
import asyncio
import os
import uuid

from sqlalchemy import select

from fluctlight_core.platform import schema
from fluctlight_core.platform.persistence import UnitOfWorkFactory, create_engine
from fluctlight_core.platform.redis_streams import EVENT_STREAM, RedisStreams


class FakeRedis:
    def __init__(self, entries):
        self.entries = list(entries)
        self.acks = []

    async def xgroup_create(self, stream, group, *, id, mkstream):
        assert stream == EVENT_STREAM

    async def xautoclaim(self, stream, group, consumer, min_idle_ms, start_id, *, count):
        return ("0-0", [])

    async def xreadgroup(self, group, consumer, streams, *, count):
        entries, self.entries = self.entries, []
        return [(EVENT_STREAM, entries)]

    async def xack(self, stream, group, stream_id):
        self.acks.append(stream_id)


async def main():
    aggregate_id = f"gap-{uuid.uuid4().hex}"

    def event(stream_id, sequence):
        return (
            stream_id,
            {
                "event_id": f"event-{stream_id}",
                "event_type": "t12.gap",
                "schema_version": "v1",
                "aggregate_type": "gap",
                "aggregate_id": aggregate_id,
                "aggregate_sequence": str(sequence),
                "correlation_id": aggregate_id,
                "attempt_policy": '{"max_attempts":3}',
                "payload": '{"source":"t12"}',
            },
        )

    engine = create_engine(os.environ["DATABASE_URL"])
    unit_of_work = UnitOfWorkFactory(engine)
    redis = FakeRedis([event("2-0", 2), event("1-0", 1)])
    streams = RedisStreams(redis)

    async def handler(session, event_id, fields):
        return {"status": "applied", "event_id": event_id}

    first = await streams.consume_transactional(
        group="integration-observers",
        consumer="gap-check",
        unit_of_work=unit_of_work,
        handler=handler,
    )
    assert first == 1
    assert redis.acks == ["1-0"]
    async with unit_of_work.begin(command_id="gap-verify") as tx:
        gap_row = (
            await tx.session.execute(
                select(schema.consumer_failures).where(
                    schema.consumer_failures.c.event_id == "event-2-0"
                )
            )
        ).mappings().one()
        assert gap_row["status"] == "gap"

    redis.entries = [event("2-0", 2)]
    second = await streams.consume_transactional(
        group="integration-observers",
        consumer="gap-check",
        unit_of_work=unit_of_work,
        handler=handler,
    )
    assert second == 1
    assert redis.acks == ["1-0", "2-0"]
    await engine.dispose()
    print(f"redis-gap-check: aggregate={aggregate_id} first_gap_pending=true replay_after_head=true")


asyncio.run(main())
PY
