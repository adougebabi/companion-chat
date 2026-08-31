#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-redis-recovery-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$(dirname "$0")/read-compose-env.sh"
db_user="$(printenv POSTGRES_USER || true)"
if [[ -z "$db_user" ]]; then db_user="$(compose_env_value POSTGRES_USER)"; fi
db_name="$(printenv POSTGRES_DB || true)"
if [[ -z "$db_name" ]]; then db_name="$(compose_env_value POSTGRES_DB)"; fi
volume="${project}_fluctlight_redis"
event_id="redis-recovery-${RANDOM}${RANDOM}"
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
  "${compose[@]}" logs --no-color core worker redis
  exit 1
fi

"${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -v ON_ERROR_STOP=1 -c \
  "INSERT INTO platform_outbox_events
   (id, kind, aggregate_type, aggregate_id, causation_id, correlation_id,
    idempotency_key, payload, attempt_policy)
   VALUES
   ('$event_id', 't12.redis.recovery', 'recovery', 'recovery-1',
    'recovery-cause', 'recovery-correlation', '$event_id',
    '{\"aggregate_sequence\":1,\"source\":\"t12\"}'::jsonb,
    '{\"max_attempts\":3}'::jsonb)"

before_loss=0
before_effects=0
for _ in $(seq 1 60); do
  before_loss=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc \
    "SELECT count(*) FROM platform_consumer_inbox WHERE event_id = '$event_id'" | tr -d '[:space:]')
  before_effects=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc \
    "SELECT count(*) FROM platform_consumer_effects WHERE event_id = '$event_id'" | tr -d '[:space:]')
  if [[ "$before_loss" == 3 && "$before_effects" == 3 ]]; then break; fi
  sleep 1
done
if [[ "$before_loss" != 3 || "$before_effects" != 3 ]]; then
  echo "event was not consumed by all durable groups before Redis loss" >&2
  echo "inbox_count=$before_loss effect_count=$before_effects stream_length=$("${compose[@]}" exec -T redis redis-cli XLEN fluctlight:events:v1 | tr -d '[:space:]')" >&2
  "${compose[@]}" logs --no-color worker redis >&2 || true
  exit 1
fi

"${compose[@]}" stop worker redis >/dev/null
"${compose[@]}" rm --force --stop worker redis >/dev/null
docker volume rm "$volume" >/dev/null
"${compose[@]}" up --detach redis worker >/dev/null

after_loss=0
after_effects=0
stream_length=0
for _ in $(seq 1 60); do
  after_loss=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc \
    "SELECT count(*) FROM platform_consumer_inbox WHERE event_id = '$event_id'" | tr -d '[:space:]')
  after_effects=$("${compose[@]}" exec -T postgres psql -U "$db_user" -d "$db_name" -Atc \
    "SELECT count(*) FROM platform_consumer_effects WHERE event_id = '$event_id'" | tr -d '[:space:]')
  stream_length=$("${compose[@]}" exec -T redis redis-cli XLEN fluctlight:events:v1 | tr -d '[:space:]')
  if [[ "$after_loss" == "$before_loss" && "$after_effects" == "$before_effects" && "$stream_length" == 1 ]]; then break; fi
  sleep 1
done
if [[ "$after_loss" != "$before_loss" || "$after_effects" != "$before_effects" || "$stream_length" != 1 ]]; then
  echo "Redis rebuild did not restore the durable event after volume loss" >&2
  echo "expected inbox_count=$before_loss effect_count=$before_effects stream_length=1; got inbox_count=$after_loss effect_count=$after_effects stream_length=$stream_length" >&2
  "${compose[@]}" logs --no-color worker redis >&2 || true
  exit 1
fi
echo "redis-recovery-check: event=$event_id inbox_groups=$after_loss effects=$after_effects stream_length=$stream_length"
