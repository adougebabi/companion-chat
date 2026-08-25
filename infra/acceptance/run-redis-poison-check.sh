#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-redis-poison-$$"
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
    def __init__(self, fields):
        self.fields = fields
        self.acks = []

    async def xgroup_create(self, stream, group, *, id, mkstream):
        assert stream == EVENT_STREAM
        assert mkstream is True

    async def xautoclaim(self, stream, group, consumer, min_idle_ms, start_id, *, count):
        return ("0-0", [])

    async def xreadgroup(self, group, consumer, streams, *, count):
        return [(EVENT_STREAM, [("1-0", self.fields)])]

    async def xack(self, stream, group, stream_id):
        self.acks.append((group, stream_id))


async def main():
    event_id = f"poison-{uuid.uuid4().hex}"
    fields = {
        "event_id": event_id,
        "event_type": "t12.poison",
        "schema_version": "v1",
        "aggregate_type": "poison",
        "aggregate_id": event_id,
        "aggregate_sequence": "1",
        "correlation_id": f"correlation-{event_id}",
        "attempt_policy": '{"max_attempts":1}',
        "payload": '{"source":"t12"}',
    }
    engine = create_engine(os.environ["DATABASE_URL"])
    unit_of_work = UnitOfWorkFactory(engine)
    redis = FakeRedis(fields)
    streams = RedisStreams(redis)

    async def fail_handler(session, event_id, fields):
        raise RuntimeError("poison fixture")

    applied = await streams.consume_transactional(
        group="integration-observers",
        consumer="poison-check",
        unit_of_work=unit_of_work,
        handler=fail_handler,
    )
    assert applied == 1
    assert redis.acks == [("integration-observers", "1-0")]
    async with unit_of_work.begin(command_id="poison-verify") as tx:
        failure = (
            await tx.session.execute(
                select(schema.consumer_failures).where(
                    schema.consumer_failures.c.event_id == event_id
                )
            )
        ).mappings().one()
        assert failure["attempt"] == 1
        assert failure["status"] == "quarantined"
        assert failure["max_attempts"] == 1
    await engine.dispose()
    print(f"redis-poison-check: event={event_id} status=quarantined attempt=1 ack_after_quarantine=true")


asyncio.run(main())
PY
