#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
compose_file="$script_dir/temporal-gate.compose.yml"
api_url="${TEMPORAL_GATE_API_URL:-http://127.0.0.1:18080}"
clean=0
functional=0
soak_hours=0
require_target_nas=0

usage() {
  printf '%s\n' 'Usage: run-temporal-gate.sh [--clean] [--functional] [--soak-hours N] [--require-target-nas]'
}

while (($#)); do
  case "$1" in
    --clean) clean=1 ;;
    --functional) functional=1 ;;
    --soak-hours) shift; soak_hours="${1:?missing soak duration}" ;;
    --require-target-nas) require_target_nas=1 ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown option: %s\n' "$1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

cleanup() {
  if ((clean)); then
    docker compose -f "$compose_file" down -v --remove-orphans >/dev/null 2>&1 || true
  else
    docker compose -f "$compose_file" down --remove-orphans >/dev/null 2>&1 || true
  fi
}
on_exit() {
  local status=$?
  cleanup
  trap - EXIT INT TERM
  exit "$status"
}
trap on_exit EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

wait_ready() {
  for _ in $(seq 1 120); do
    if curl -fsS -m 1 "$api_url/readyz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  printf '%s\n' 'Temporal gate API did not become ready within 120 seconds.' >&2
  return 1
}

start_intent() {
  local intent_key=$1
  local queue=$2
  local sleep_seconds=$3
  local h3_duration_seconds=${4:-0}
  local timeout_seconds=${5:-900}
  curl -fsS -m 20 -X POST "$api_url/gate/intents" \
    -H 'content-type: application/json' \
    -d "{\"intent_key\":\"$intent_key\",\"queue\":\"$queue\",\"sleep_seconds\":$sleep_seconds,\"h3_duration_seconds\":$h3_duration_seconds,\"timeout_seconds\":$timeout_seconds}"
}

workflow_id_from_response() {
  /usr/bin/python3 -c 'import json, sys; print(json.load(sys.stdin)["workflow_id"])'
}

sample_metrics() {
  local service container_id
  local container_ids=()
  for service in temporal postgres api worker; do
    container_id="$(docker compose -f "$compose_file" ps -q "$service" 2>/dev/null || true)"
    if [[ -n "$container_id" ]]; then
      container_ids+=("$container_id")
    fi
  done
  if ((${#container_ids[@]} > 0)); then
    docker stats --no-stream --format '{{.Name}} mem={{.MemUsage}} cpu={{.CPUPerc}}' \
      "${container_ids[@]}" || true
  fi
  docker compose -f "$compose_file" exec -T postgres \
    psql -U "${POSTGRES_USER:-temporal}" -d "${POSTGRES_DB:-temporal}" -Atqc \
    "SELECT 'postgres_connections=' || count(*) FROM pg_stat_activity; SELECT 'temporal_bytes=' || pg_database_size(current_database()); SELECT 'visibility_bytes=' || pg_database_size('${VISIBILITY_DB:-temporal_visibility}');" \
    2>/dev/null || true
  local visibility_count
  visibility_count="$(curl -fsS -m 10 "$api_url/gate/workflows" 2>/dev/null \
    | /usr/bin/python3 -c 'import json, sys; print(len(json.load(sys.stdin)))' 2>/dev/null || printf 'unavailable')"
  printf 'visibility_workflow_count=%s\n' "$visibility_count"
}

run_minute_workload() {
  local minute=$1
  local queue_index queue workflow_id response
  local queues=(interaction lifecycle media)

  # Two one-hour timers per minute keep at least 100 durable timers active after
  # the initial warm-up while exercising all three application task queues.
  for queue_index in 0 1; do
    queue="${queues[$(( (minute * 2 + queue_index) % 3 ))]}"
    response="$(start_intent "temporal-soak-timer-${minute}-${queue_index}-$(date +%s)" "$queue" 3600)"
    if ((queue_index == 0)); then
      workflow_id="$(printf '%s' "$response" | workflow_id_from_response)"
    fi
  done

  # Exercise the canonical Query, Update and Signal paths on each minute's
  # primary workflow. The timer remains durable while these controls execute.
  curl -fsS -m 10 "$api_url/gate/workflows/$workflow_id/status" >/dev/null
  curl -fsS -m 10 -X POST "$api_url/gate/workflows/$workflow_id/repair" \
    -H 'content-type: application/json' \
    -d '{"reason":"target NAS soak heartbeat"}' >/dev/null
  curl -fsS -m 10 -X POST "$api_url/gate/workflows/$workflow_id/pause" >/dev/null
  curl -fsS -m 10 -X POST "$api_url/gate/workflows/$workflow_id/resume" >/dev/null

  # One long fake media Activity per hour supplies the heartbeat/recovery load.
  if ((minute % 60 == 0)); then
    start_intent "temporal-soak-h3-${minute}-$(date +%s)" media 0 900 930 >/dev/null
  fi
}

if ((clean)); then
  docker compose -f "$compose_file" down -v --remove-orphans >/dev/null 2>&1 || true
fi

if ((functional)); then
  docker compose -f "$compose_file" up -d
  docker compose -f "$compose_file" ps
  wait_ready
  curl -fsS -m 10 "$api_url/healthz"
  start_intent "temporal-runner-functional-$(date +%s)" interaction 0 0 30
  UV_CACHE_DIR="${UV_CACHE_DIR:-/tmp/fluctlight-uv-cache}" uv run pytest apps/core/tests/temporal_gate -q
fi

if ((soak_hours > 0)); then
  if ((require_target_nas)) && [[ "${TEMPORAL_GATE_TARGET_NAS:-0}" != "1" ]]; then
    printf '%s\n' '12-hour PASS requires TEMPORAL_GATE_TARGET_NAS=1 on the actual 16 GiB NAS.' >&2
    exit 3
  fi
  if ((functional && clean)); then
    docker compose -f "$compose_file" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  docker compose -f "$compose_file" up -d
  wait_ready
  for warmup in $(seq 1 100); do
    queue=(interaction lifecycle media)
    start_intent "temporal-soak-warmup-${warmup}-$(date +%s)" \
      "${queue[$(( (warmup - 1) % 3 ))]}" 3600 >/dev/null
  done
  soak_start=$SECONDS
  deadline=$((SECONDS + soak_hours * 3600))
  minute=0
  worker_restart_done=0
  temporal_restart_done=0
  postgres_restart_done=0
  while ((SECONDS < deadline)); do
    printf 'sample minute=%d timestamp=%s\n' "$minute" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    run_minute_workload "$minute"
    elapsed=$((SECONDS - soak_start))
    if ((elapsed >= 7200 && worker_restart_done == 0)); then
      docker compose -f "$compose_file" restart worker
      worker_restart_done=1
      wait_ready
    fi
    if ((elapsed >= 14400 && temporal_restart_done == 0)); then
      docker compose -f "$compose_file" restart temporal
      temporal_restart_done=1
      wait_ready
    fi
    if ((elapsed >= 28800 && postgres_restart_done == 0)); then
      docker compose -f "$compose_file" restart postgres
      postgres_restart_done=1
      wait_ready
    fi
    docker compose -f "$compose_file" ps
    sample_metrics
    minute=$((minute + 1))
    remaining=$((deadline - SECONDS))
    if ((remaining > 0)); then
      sleep $((remaining < 60 ? remaining : 60))
    fi
  done
fi

if ((!functional && soak_hours == 0)); then
  docker compose -f "$compose_file" config
fi
