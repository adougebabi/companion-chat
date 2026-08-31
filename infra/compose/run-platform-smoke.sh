#!/usr/bin/env sh
set -eu

compose_file="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-$(dirname "$compose_file")/fluctlight.env}"
project_name="fluctlight-smoke-$$"
export BFF_HOST_PORT="${BFF_HOST_PORT:-0}"
export WEB_HOST_PORT="${WEB_HOST_PORT:-0}"

if [ ! -f "$env_file" ]; then
  printf '%s\n' "Missing private environment file: $env_file" >&2
  exit 2
fi

compose() {
  docker compose --project-name "$project_name" --env-file "$env_file" -f "$compose_file" "$@"
}

assert_disposable_project() {
  case "$project_name" in
    *-[0-9]*) ;;
    *)
      printf '%s\n' "refusing non-unique disposable Compose project: $project_name" >&2
      exit 2
      ;;
  esac
  if docker ps -aq --filter "name=^${project_name}-" | grep -q .; then
    printf '%s\n' "refusing to reuse existing Compose containers for $project_name" >&2
    exit 2
  fi
  if docker volume ls -q --filter "name=^${project_name}_" | grep -q .; then
    printf '%s\n' "refusing to reuse existing Compose volumes for $project_name" >&2
    exit 2
  fi
  if docker network ls -q --filter "name=^${project_name}_" | grep -q .; then
    printf '%s\n' "refusing to reuse existing Compose networks for $project_name" >&2
    exit 2
  fi
}

compose_started=0
cleanup() {
  if [ "$compose_started" -ne 1 ]; then return; fi
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}

diagnose() {
  compose ps >&2 || true
  compose logs --no-color migrate minio-init core worker bff >&2 || true
}

check_bff_ready() {
  compose exec -T bff sh -c \
    'wget -q -O /dev/null "http://127.0.0.1:${BFF_PORT:-3000}/health/ready"'
}

check_bff_ping() {
  compose exec -T bff sh -c \
    'wget -q -O /dev/null "http://127.0.0.1:${BFF_PORT:-3000}/api/platform/ping"'
}

trap cleanup EXIT INT TERM

case "${1:-}" in
  --clean|"") ;;
  *)
    printf '%s\n' "Usage: $0 [--clean]" >&2
    exit 2
    ;;
esac

assert_disposable_project
compose_started=1
compose config >/dev/null
if ! compose up --build --detach; then
  diagnose
  exit 1
fi

attempt=0
until check_bff_ready; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    diagnose
    exit 1
  fi
  sleep 2
done

check_bff_ping
compose ps --status running
