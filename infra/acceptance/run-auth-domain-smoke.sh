#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-auth-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$(dirname "$0")/read-compose-env.sh"
service_key="$(printenv FLUCTLIGHT_CORE_SERVICE_KEY || true)"
if [[ -z "$service_key" ]]; then service_key="$(compose_env_value FLUCTLIGHT_CORE_SERVICE_KEY)"; fi
postgres_user="$(printenv POSTGRES_USER || true)"
if [[ -z "$postgres_user" ]]; then postgres_user="$(compose_env_value POSTGRES_USER)"; fi
postgres_db="$(printenv POSTGRES_DB || true)"
if [[ -z "$postgres_db" ]]; then postgres_db="$(compose_env_value POSTGRES_DB)"; fi
trusted_origin="$(printenv FLUCTLIGHT_TRUSTED_ORIGIN || true)"
if [[ -z "$trusted_origin" ]]; then trusted_origin="$(compose_env_value FLUCTLIGHT_TRUSTED_ORIGIN)"; fi
assert_disposable_compose_project "$project"
compose_started=0

cleanup() {
  if [[ "$compose_started" != 1 ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

compose_started=1
"${compose[@]}" up --build --detach
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core python -c 'import urllib.request; urllib.request.urlopen("http://127.0.0.1:8080/health/ready")' >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

setup_token=$("${compose[@]}" exec -T core uv run --no-sync fluctlight issue-setup-token --expires-minutes 10 | tr -d '\r\n')
[[ -n "$setup_token" ]]
echo "auth-domain-smoke: setup token issued"
set +e
seed_headers=$("${compose[@]}" exec -T -e TRUSTED_ORIGIN="$trusted_origin" bff sh -c '
  curl -sS -D - -o /dev/null -H "Origin: ${TRUSTED_ORIGIN}" "http://127.0.0.1:${BFF_PORT:-3000}/auth/session"
')
csrf_token=$(printf '%s\n' "$seed_headers" | grep -i '^set-cookie: fluctlight_csrf=' | sed -n 's/^[^:]*: fluctlight_csrf=\([^;]*\).*/\1/p' | head -n 1)
if [[ -z "$csrf_token" ]]; then
  echo "session seed did not include CSRF cookie" >&2
  session_status=1
else
  setup_headers=$("${compose[@]}" exec -T -e SETUP_TOKEN="$setup_token" -e TRUSTED_ORIGIN="$trusted_origin" -e CSRF_TOKEN="$csrf_token" bff sh -c '
    curl -sS -D - -o /dev/null -X POST \
      -H "Origin: ${TRUSTED_ORIGIN}" \
      -H "X-CSRF-Token: ${CSRF_TOKEN}" \
      -H "Cookie: fluctlight_csrf=${CSRF_TOKEN}" \
      -H "Content-Type: application/json" \
      --data "{\"setupToken\":\"${SETUP_TOKEN}\",\"password\":\"fluctlight-smoke-password\"}" \
      "http://127.0.0.1:${BFF_PORT:-3000}/auth/setup"
  ')
  session_cookie=$(printf '%s\n' "$setup_headers" | grep -i '^set-cookie: fluctlight_session=' | sed -n 's/^[^:]*: fluctlight_session=\([^;]*\).*/fluctlight_session=\1/p' | head -n 1)
  if [[ -z "$session_cookie" ]]; then
    echo "setup response did not include a session cookie" >&2
    session_status=1
  else
    printf '%s\n' "$session_cookie"
    session_status=0
  fi
fi
set -e
echo "auth-domain-smoke: setup request status=${session_status}"
[[ "$session_status" == 0 ]]
login_json=$("${compose[@]}" exec -T -e SERVICE_KEY="$service_key" bff sh -c '
  curl -sS --fail -X POST \
    -H "X-Fluctlight-Service-Key: ${SERVICE_KEY}" \
    -H "Content-Type: application/json" \
    --data '{"password":"fluctlight-smoke-password"}' \
    http://core:8080/internal/auth/login
')
session_cookie=$(printf '%s\n' "$login_json" | sed -n 's/.*"session_token"[[:space:]]*:[[:space:]]*"\([^\"]*\)".*/\1/p')
[[ -n "$session_cookie" ]]
echo "auth-domain-smoke: Core login session resolved"

fluctlight_json=$("${compose[@]}" exec -T -e SESSION_COOKIE="$session_cookie" -e SERVICE_KEY="$service_key" bff sh -c '
  curl -sS --fail -X POST \
    -H "X-Fluctlight-Service-Key: ${SERVICE_KEY}" \
    -H "X-Fluctlight-Human-Session: ${SESSION_COOKIE}" \
    -H "Content-Type: application/json" \
    --data '{"name":"T12 smoke Fluctlight"}' \
    http://core:8080/internal/fluctlights
')
fluctlight_id=$(printf '%s\n' "$fluctlight_json" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^\"]*\)".*/\1/p')
[[ -n "$fluctlight_id" ]]

conversation_json=$("${compose[@]}" exec -T -e SESSION_COOKIE="$session_cookie" -e SERVICE_KEY="$service_key" -e FLUCTLIGHT_ID="$fluctlight_id" bff sh -c '
  curl -sS --fail -X POST \
    -H "X-Fluctlight-Service-Key: ${SERVICE_KEY}" \
    -H "X-Fluctlight-Human-Session: ${SESSION_COOKIE}" \
    -H "Content-Type: application/json" \
    --data "{\"participant_actor_ids\":[\"${FLUCTLIGHT_ID}\"],\"title\":\"T12 smoke conversation\"}" \
    http://core:8080/internal/conversations
')
conversation_id=$(printf '%s\n' "$conversation_json" | sed -n 's/.*"conversation"[[:space:]]*:[[:space:]]*{[^}]*"id"[[:space:]]*:[[:space:]]*"\([^\"]*\)".*/\1/p')
[[ -n "$conversation_id" ]]

stream=$("${compose[@]}" exec -T -e SESSION_COOKIE="$session_cookie" -e SERVICE_KEY="$service_key" -e FLUCTLIGHT_ID="$fluctlight_id" -e CONVERSATION_ID="$conversation_id" bff sh -c '
  curl -sS -X POST \
    -H "X-Fluctlight-Service-Key: ${SERVICE_KEY}" \
    -H "X-Fluctlight-Human-Session: ${SESSION_COOKIE}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/x-ndjson" \
    --data "{\"fluctlight_id\":\"${FLUCTLIGHT_ID}\",\"text\":\"Provider is intentionally unconfigured\",\"idempotency_key\":\"t12-smoke-turn\"}" \
    "http://core:8080/internal/conversations/${CONVERSATION_ID}/turn"
')
printf '%s\n' "$stream" | grep -E '"type":"error"' >/dev/null

actor_count=$("${compose[@]}" exec -T postgres psql -U "$postgres_user" -d "$postgres_db" -Atc "SELECT count(*) FROM actors WHERE actor_type = 'fluctlight'")
actor_match=$("${compose[@]}" exec -T postgres psql -U "$postgres_user" -d "$postgres_db" -Atc "SELECT count(*) FROM actors WHERE id = '$fluctlight_id' AND actor_type = 'fluctlight'" | tr -d '[:space:]')
participant_match=$("${compose[@]}" exec -T postgres psql -U "$postgres_user" -d "$postgres_db" -Atc "SELECT count(*) FROM conversation_participants WHERE conversation_id = '$conversation_id' AND actor_id = '$fluctlight_id'" | tr -d '[:space:]')
[[ "${actor_count//[[:space:]]/}" -ge 1 && "$actor_match" == 1 && "$participant_match" == 1 ]]
echo "auth-domain-smoke: setup/session/fluctlight=${fluctlight_id}/conversation=${conversation_id}; actor_rows=${actor_count//[[:space:]]/}; participant_rows=$participant_match; explicit_provider_error=pass"
