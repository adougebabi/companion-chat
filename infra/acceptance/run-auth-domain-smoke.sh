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
session_cookie=$("${compose[@]}" exec -T -e SETUP_TOKEN="$setup_token" -e TRUSTED_ORIGIN="$trusted_origin" bff node -e '
const origin = process.env.TRUSTED_ORIGIN;
const seed = await fetch("http://127.0.0.1:3000/auth/session", {headers: {origin}});
const seedCookies = seed.headers.getSetCookie ? seed.headers.getSetCookie() : [seed.headers.get("set-cookie") ?? ""];
const csrfCookie = seedCookies.find((value) => value.startsWith("fluctlight_csrf="))?.split(";")[0] ?? "";
const csrfToken = csrfCookie.split("=")[1] ?? "";
if (!csrfToken) { console.error("session seed did not include CSRF cookie"); process.exit(1); }
const response = await fetch("http://127.0.0.1:3000/auth/setup", {method: "POST", headers: {origin, "x-csrf-token": csrfToken, cookie: csrfCookie, "content-type": "application/json"}, body: JSON.stringify({setupToken: process.env.SETUP_TOKEN, password: "fluctlight-smoke-password"})});
if (!response.ok) { console.error("setup status", response.status, await response.text()); process.exit(1); }
const cookies = response.headers.getSetCookie ? response.headers.getSetCookie() : [response.headers.get("set-cookie") ?? ""];
const session = cookies.find((value) => value.startsWith("fluctlight_session="))?.split(";")[0] ?? "";
if (!session) { console.error("setup response did not include a session cookie"); process.exit(1); }
console.log(session);
')
session_status=$?
set -e
echo "auth-domain-smoke: setup request status=${session_status}"
[[ "$session_status" == 0 ]]
session_cookie=$("${compose[@]}" exec -T -e SERVICE_KEY="$service_key" bff node -e '
const response = await fetch("http://core:8080/internal/auth/login", {method: "POST", headers: {"x-fluctlight-service-key": process.env.SERVICE_KEY, "content-type": "application/json"}, body: JSON.stringify({password: "fluctlight-smoke-password"})});
if (!response.ok) { console.error("login status", response.status, await response.text()); process.exit(1); }
console.log((await response.json()).session_token);
')
[[ -n "$session_cookie" ]]
echo "auth-domain-smoke: Core login session resolved"

fluctlight_id=$("${compose[@]}" exec -T -e SESSION_COOKIE="$session_cookie" -e SERVICE_KEY="$service_key" bff node -e '
const response = await fetch("http://core:8080/internal/fluctlights", {method: "POST", headers: {"x-fluctlight-service-key": process.env.SERVICE_KEY, "x-fluctlight-human-session": process.env.SESSION_COOKIE, "content-type": "application/json"}, body: JSON.stringify({name: "T12 smoke Fluctlight"})});
if (!response.ok) process.exit(1);
console.log((await response.json()).id);
')

conversation_id=$("${compose[@]}" exec -T -e SESSION_COOKIE="$session_cookie" -e SERVICE_KEY="$service_key" -e FLUCTLIGHT_ID="$fluctlight_id" bff node -e '
const response = await fetch("http://core:8080/internal/conversations", {method: "POST", headers: {"x-fluctlight-service-key": process.env.SERVICE_KEY, "x-fluctlight-human-session": process.env.SESSION_COOKIE, "content-type": "application/json"}, body: JSON.stringify({participant_actor_ids: [process.env.FLUCTLIGHT_ID], title: "T12 smoke conversation"})});
if (!response.ok) process.exit(1);
console.log((await response.json()).conversation.id);
')

stream=$("${compose[@]}" exec -T -e SESSION_COOKIE="$session_cookie" -e SERVICE_KEY="$service_key" -e FLUCTLIGHT_ID="$fluctlight_id" -e CONVERSATION_ID="$conversation_id" bff node -e '
const response = await fetch(`http://core:8080/internal/conversations/${process.env.CONVERSATION_ID}/turn`, {method: "POST", headers: {"x-fluctlight-service-key": process.env.SERVICE_KEY, "x-fluctlight-human-session": process.env.SESSION_COOKIE, "content-type": "application/json"}, body: JSON.stringify({fluctlight_id: process.env.FLUCTLIGHT_ID, text: "Provider is intentionally unconfigured", idempotency_key: "t12-smoke-turn"})});
console.log(await response.text());
')
printf '%s\n' "$stream" | grep -E '"type":"error"' >/dev/null

actor_count=$("${compose[@]}" exec -T postgres psql -U "$postgres_user" -d "$postgres_db" -Atc "SELECT count(*) FROM actors WHERE actor_type = 'fluctlight'")
actor_match=$("${compose[@]}" exec -T postgres psql -U "$postgres_user" -d "$postgres_db" -Atc "SELECT count(*) FROM actors WHERE id = '$fluctlight_id' AND actor_type = 'fluctlight'" | tr -d '[:space:]')
participant_match=$("${compose[@]}" exec -T postgres psql -U "$postgres_user" -d "$postgres_db" -Atc "SELECT count(*) FROM conversation_participants WHERE conversation_id = '$conversation_id' AND actor_id = '$fluctlight_id'" | tr -d '[:space:]')
[[ "${actor_count//[[:space:]]/}" -ge 1 && "$actor_match" == 1 && "$participant_match" == 1 ]]
echo "auth-domain-smoke: setup/session/fluctlight=${fluctlight_id}/conversation=${conversation_id}; actor_rows=${actor_count//[[:space:]]/}; participant_rows=$participant_match; explicit_provider_error=pass"
