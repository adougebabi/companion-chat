#!/usr/bin/env sh
set -eu

compose_file="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-$(dirname "$compose_file")/fluctlight.env}"
project_name="fluctlight-smoke-$$"

if [ ! -f "$env_file" ]; then
  printf '%s\n' "Missing private environment file: $env_file" >&2
  exit 2
fi

compose() {
  docker compose --project-name "$project_name" --env-file "$env_file" -f "$compose_file" "$@"
}

cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}

diagnose() {
  compose ps >&2 || true
  compose logs --no-color migrate minio-init core worker bff >&2 || true
}

trap cleanup EXIT INT TERM

case "${1:-}" in
  --clean|"") ;;
  *)
    printf '%s\n' "Usage: $0 [--clean]" >&2
    exit 2
    ;;
esac

cleanup
compose config >/dev/null
if ! compose up --build --detach; then
  diagnose
  exit 1
fi

attempt=0
until curl --fail --silent --show-error http://127.0.0.1:13000/health/ready >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    diagnose
    exit 1
  fi
  sleep 2
done

curl --fail --silent --show-error http://127.0.0.1:13000/api/platform/ping >/dev/null
compose ps --status running
