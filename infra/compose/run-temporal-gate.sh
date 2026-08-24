#!/usr/bin/env bash
set -Eeuo pipefail

compose_file="infra/compose/temporal-gate.compose.yml"
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
trap cleanup EXIT INT TERM

if ((clean)); then
  docker compose -f "$compose_file" down -v --remove-orphans >/dev/null 2>&1 || true
fi

if ((functional)); then
  docker compose -f "$compose_file" up -d
  docker compose -f "$compose_file" ps
  for _ in $(seq 1 120); do
    if curl -fsS -m 1 http://127.0.0.1:18080/readyz >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  curl -fsS -m 10 http://127.0.0.1:18080/healthz
  curl -fsS -m 20 -X POST http://127.0.0.1:18080/gate/intents \
    -H 'content-type: application/json' \
    -d '{"intent_key":"temporal-runner-functional","queue":"interaction","timeout_seconds":30}'
  uv run pytest apps/core/tests/temporal_gate -q
fi

if ((soak_hours > 0)); then
  if ((require_target_nas)) && [[ "${TEMPORAL_GATE_TARGET_NAS:-0}" != "1" ]]; then
    printf '%s\n' '12-hour PASS requires TEMPORAL_GATE_TARGET_NAS=1 on the actual 16 GiB NAS.' >&2
    exit 3
  fi
  docker compose -f "$compose_file" up -d
  deadline=$((SECONDS + soak_hours * 3600))
  while ((SECONDS < deadline)); do
    date -u +%Y-%m-%dT%H:%M:%SZ
    docker compose -f "$compose_file" ps
    docker stats --no-stream --format '{{.Name}} {{.MemUsage}} {{.CPUPerc}}' \
      temporal-gate-temporal-1 temporal-gate-postgres-1 temporal-gate-api-1 temporal-gate-worker-1 || true
    sleep 60
  done
fi

if ((!functional && soak_hours == 0)); then
  docker compose -f "$compose_file" config
fi
