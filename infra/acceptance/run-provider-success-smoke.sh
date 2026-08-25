#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-provider-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$(dirname "$0")/read-compose-env.sh"
service_key="$(printenv FLUCTLIGHT_CORE_SERVICE_KEY || true)"
if [[ -z "$service_key" ]]; then service_key="$(compose_env_value FLUCTLIGHT_CORE_SERVICE_KEY)"; fi
db_user="$(printenv POSTGRES_USER || true)"
if [[ -z "$db_user" ]]; then db_user="$(compose_env_value POSTGRES_USER)"; fi
db_name="$(printenv POSTGRES_DB || true)"
if [[ -z "$db_name" ]]; then db_name="$(compose_env_value POSTGRES_DB)"; fi
provider_port="${FLUCTLIGHT_PROVIDER_PORT:-18081}"
provider_container=""
assert_disposable_compose_project "$project"
compose_started=0

cleanup() {
  if [[ -n "$provider_container" ]]; then docker rm -f "$provider_container" >/dev/null 2>&1 || true; fi
  if [[ "$compose_started" != 1 ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

provider_container="$project-provider"
compose_started=1
"${compose[@]}" up --build --detach
docker run --detach --rm --name "$provider_container" \
  --network "${project}_platform" --network-alias provider-fixture \
  -v "$PWD/infra/acceptance/provider-fixture.py:/fixture.py:ro" \
  python:3.13.7-slim-bookworm python /fixture.py --host 0.0.0.0 --port "$provider_port" >/dev/null
for _ in $(seq 1 30); do
  if docker exec "$provider_container" python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:$provider_port/models')" >/dev/null 2>&1; then break; fi
  sleep 1
done
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
  "${compose[@]}" logs --no-color core worker
  exit 1
fi

setup_token=$("${compose[@]}" exec -T core uv run --no-sync fluctlight issue-setup-token --expires-minutes 10 | tr -d '\r\n')
"${compose[@]}" exec -T \
  -e SETUP_TOKEN="$setup_token" \
  -e SERVICE_KEY="$service_key" \
  -e PROVIDER_URL="http://provider-fixture:$provider_port" \
  core uv run --no-sync python - <<'PY'
import asyncio
import json
import os
import time
import urllib.error
import urllib.request

base = "http://127.0.0.1:8080"
service_key = os.environ["SERVICE_KEY"]
provider_url = os.environ["PROVIDER_URL"]


def call(path, method="GET", body=None, session=None):
    headers = {"x-fluctlight-service-key": service_key}
    if session:
        headers["x-fluctlight-human-session"] = session
    data = None
    if body is not None:
        headers["content-type"] = "application/json"
        data = json.dumps(body).encode()
    request = urllib.request.Request(base + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            raw = response.read()
            return response.status, raw
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise SystemExit(f"{method} {path} failed with {error.code}: {detail}") from error


def stream_call(path, body, session):
    headers = {"x-fluctlight-service-key": service_key, "content-type": "application/json"}
    headers["x-fluctlight-human-session"] = session
    request = urllib.request.Request(
        base + path, data=json.dumps(body).encode(), headers=headers, method="POST"
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            frames = []
            token_times = []
            token_text = ""
            completed_at = None
            while True:
                line = response.readline()
                if not line:
                    break
                event = json.loads(line)
                frames.append(line)
                if event["type"] == "token":
                    token_times.append(time.monotonic())
                    token_text += str(event.get("payload", {}).get("text", ""))
                if event["type"] == "completed":
                    completed_at = time.monotonic()
            return response.status, b"".join(frames), token_times, completed_at, token_text
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise SystemExit(f"POST {path} failed with {error.code}: {detail}") from error


_, setup_raw = call(
    "/internal/auth/setup",
    "POST",
    {"setup_token": os.environ["SETUP_TOKEN"], "password": "provider-smoke-password"},
)
session = json.loads(setup_raw)["session_token"]
call(
    "/internal/settings",
    "PUT",
    {"values": {}, "secrets": {"provider:fixture": "fixture-secret"}},
    session,
)
call(
    "/internal/providers/endpoints",
    "PUT",
    {
        "endpoint_id": "fixture",
        "kind": "openai-compatible",
        "base_url": provider_url,
        "secret_purpose": "provider:fixture",
    },
    session,
)
for role in (
    "initialization",
    "cognitive_assessment",
    "action_realization",
    "reflection",
    "embedding",
    "media_prompt",
):
    status, raw = call(
        "/internal/providers/roles",
        "PUT",
        {
            "role": role,
            "endpoint_id": "fixture",
            "model_id": "fixture-model",
            "token_budget": 1000,
            "timeout_seconds": 20,
        },
        session,
    )
    if status != 200 or not json.loads(raw).get("available"):
        raise SystemExit(f"Provider role preflight failed: {role}")

_, fluctlight_raw = call("/internal/fluctlights", "POST", {"name": "Provider smoke"}, session)
fluctlight_id = json.loads(fluctlight_raw)["id"]
_, conversation_raw = call(
    "/internal/conversations",
    "POST",
    {"participant_actor_ids": [fluctlight_id], "title": "Provider smoke"},
    session,
)
conversation_id = json.loads(conversation_raw)["conversation"]["id"]
_, stream, token_times, completed_at, token_text = stream_call(
    f"/internal/conversations/{conversation_id}/turn",
    {
        "fluctlight_id": fluctlight_id,
        "text": "prove configured execution",
        "idempotency_key": "provider-success-smoke",
    },
    session,
)
if token_text != "configured Provider reply" or len(token_times) < 2 or completed_at is None:
    raise SystemExit(f"configured Provider reply was not observed: {stream!r}")

from datetime import UTC, datetime

from fluctlight_core.memory.contracts import MemoryRecord, MemoryType, MemoryVisibility
from fluctlight_core.memory.service import MemoryService
from fluctlight_core.platform.persistence import UnitOfWorkFactory, create_engine


async def create_memory() -> str:
    engine = create_engine(os.environ["DATABASE_URL"])
    service = MemoryService(UnitOfWorkFactory(engine))
    memory_id = "memory_provider_smoke"
    await service.record(
        MemoryRecord(
            id=memory_id,
            owner_fluctlight_id=fluctlight_id,
            type=MemoryType.EPISODIC,
            content="configured Provider embedding smoke",
            actor_refs=(fluctlight_id,),
            conversation_id=conversation_id,
            event_refs=(conversation_id,),
            evidence_refs=("provider-success-smoke",),
            confidence=0.9,
            importance=0.5,
            emotional_significance=0.2,
            visibility=MemoryVisibility.OWNER,
            created_at=datetime.now(UTC),
        )
    )
    await engine.dispose()
    return memory_id


memory_id = asyncio.run(create_memory())
print(
    json.dumps(
        {
            "fluctlight_id": fluctlight_id,
            "conversation_id": conversation_id,
            "provider_success": True,
            "stream_contains_reply": True,
        }
    )
)
PY
embedding_status=""
for _ in $(seq 1 60); do
  embedding_status=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT status FROM memory_embeddings WHERE memory_id = 'memory_provider_smoke' AND model_id = 'fixture-model' ORDER BY created_at DESC LIMIT 1" | tr -d '[:space:]')
  if [[ "$embedding_status" == "ready" ]]; then break; fi
  sleep 1
done
if [[ "$embedding_status" != "ready" ]]; then
  echo "provider-success-smoke: embedding workflow did not reach ready: $embedding_status" >&2
  "${compose[@]}" logs --no-color worker core >&2 || true
  exit 1
fi
provenance_count=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc "SELECT count(*) FROM provider_provenance WHERE endpoint_id = 'fixture'" | tr -d '[:space:]')
if [[ "$provenance_count" -lt 2 ]]; then
  echo "provider-success-smoke: expected assessment/realization provenance rows, got $provenance_count" >&2
  exit 1
fi
echo "provider-success-smoke: configured assessment, streaming realization, conversation reply, embedding=$embedding_status and provenance rows=$provenance_count passed"
