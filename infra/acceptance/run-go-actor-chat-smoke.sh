#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-actor-smoke-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.local.env}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
cookie_file=$(mktemp)
session_body=$(mktemp)
setup_body=$(mktemp)
turn_body=$(mktemp)
cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -f "$cookie_file" "$session_body" "$setup_body" "$turn_body"
}
trap cleanup EXIT INT TERM

export BFF_HOST_PORT=0
export WEB_HOST_PORT=0
"${compose[@]}" up --build --detach --wait --wait-timeout 240 >/dev/null
bff_port=$("${compose[@]}" port bff 3000 | sed -E 's/.*:([0-9]+)$/\1/')
trusted_origin=$(grep '^FLUCTLIGHT_TRUSTED_ORIGIN=' "$env_file" | cut -d= -f2-)
token=$("${compose[@]}" exec -T core /usr/local/bin/fluctlight-setup-token-go --expires-minutes 10 | tr -d '\r\n')
test -n "$token"

session_headers=$(curl -sS -D - -o "$session_body" -H "Origin: $trusted_origin" "http://127.0.0.1:$bff_port/auth/session")
csrf=$(printf '%s\n' "$session_headers" | awk -F'[=;]' 'tolower($1) ~ /set-cookie: fluctlight_csrf/ {print $2; exit}')
test -n "$csrf"
setup_headers=$(curl -sS -D - -o "$setup_body" -c "$cookie_file" -b "fluctlight_csrf=$csrf" -H "Origin: $trusted_origin" -H "X-CSRF-Token: $csrf" -H "Cookie: fluctlight_csrf=$csrf" -H "Content-Type: application/json" --data "{\"setupToken\":\"$token\",\"password\":\"fluctlight-actor-smoke-password\"}" "http://127.0.0.1:$bff_port/auth/setup")
session_cookie=$(printf '%s\n' "$setup_headers" | awk -F'[=;]' 'tolower($1) ~ /set-cookie: fluctlight_session/ {print $2; exit}')
new_csrf=$(printf '%s\n' "$setup_headers" | awk -F'[=;]' 'tolower($1) ~ /set-cookie: fluctlight_csrf/ {print $2; exit}')
if [[ -n "$new_csrf" ]]; then csrf="$new_csrf"; fi
test -n "$session_cookie"
cookie="fluctlight_session=$session_cookie; fluctlight_csrf=$csrf"

create_fluctlight() {
  curl -sS -f -H "Origin: $trusted_origin" -H "X-CSRF-Token: $csrf" -H "Cookie: $cookie" -H "Content-Type: application/json" --data "{\"name\":\"$1\"}" "http://127.0.0.1:$bff_port/api/fluctlights"
}
first=$(create_fluctlight "群聊主摇光")
second=$(create_fluctlight "群聊副摇光")
fluctlight_one=$(printf '%s' "$first" | jq -r '.id')
fluctlight_two=$(printf '%s' "$second" | jq -r '.id')
test -n "$fluctlight_one" -a -n "$fluctlight_two"

group=$(curl -sS -f -H "Origin: $trusted_origin" -H "X-CSRF-Token: $csrf" -H "Cookie: $cookie" -H "Content-Type: application/json" --data "{\"title\":\"双摇光群聊\",\"participantActorIds\":[\"$fluctlight_one\",\"$fluctlight_two\"]}" "http://127.0.0.1:$bff_port/api/conversations")
conversation_id=$(printf '%s' "$group" | jq -r '.conversation.id')
test -n "$conversation_id"

# Provider configuration is intentionally optional in this smoke. The turn
# may produce a bounded provider error, but the user fact must still be
# persisted with the selected Fluctlight sender as author_actor_id.
curl -sS -o "$turn_body" -H "Origin: $trusted_origin" -H "X-CSRF-Token: $csrf" -H "Cookie: $cookie" -H "Accept: application/x-ndjson" -H "Content-Type: application/json" --data "{\"fluctlightId\":\"$fluctlight_one\",\"senderActorId\":\"$fluctlight_two\",\"text\":\"由副摇光发言\",\"idempotencyKey\":\"actor-smoke-turn-1\"}" "http://127.0.0.1:$bff_port/api/conversations/$conversation_id/turn" || true

sender_messages=$("${compose[@]}" exec -T postgres psql -U "${POSTGRES_USER:-fluctlight}" -d "${POSTGRES_DB:-fluctlight}" -Atc "SELECT count(*) FROM conversation_messages WHERE conversation_id='$conversation_id' AND author_actor_id='$fluctlight_two' AND text='由副摇光发言'" | tr -d '[:space:]')
participants=$("${compose[@]}" exec -T postgres psql -U "${POSTGRES_USER:-fluctlight}" -d "${POSTGRES_DB:-fluctlight}" -Atc "SELECT count(*) FROM conversation_participants WHERE conversation_id='$conversation_id' AND actor_id IN ('$fluctlight_one','$fluctlight_two')" | tr -d '[:space:]')
printf 'go-actor-chat-smoke: conversation=%s sender=%s target=%s sender_messages=%s participants=%s\n' "$conversation_id" "$fluctlight_two" "$fluctlight_one" "$sender_messages" "$participants"
test "$sender_messages" = 1
test "$participants" = 2
