#!/usr/bin/env bash
set -euo pipefail

: "${BFF_ORIGIN:?set BFF_ORIGIN to the public Go BFF origin}"
: "${COOKIE_JAR:?set COOKIE_JAR to an authenticated cookie jar}"
: "${MOMENT_FLUCTLIGHT_ID:?set MOMENT_FLUCTLIGHT_ID}"
: "${PROACTIVE_FLUCTLIGHT_ID:?set PROACTIVE_FLUCTLIGHT_ID}"

moment_json=$(mktemp)
action_json=$(mktemp)
conversation_json=$(mktemp)
trap 'rm -f "$moment_json" "$action_json" "$conversation_json"' EXIT

curl --fail-with-body --max-time 30 -sS -b "$COOKIE_JAR" \
  "$BFF_ORIGIN/api/fluctlights/$MOMENT_FLUCTLIGHT_ID/moments" >"$moment_json"
curl --fail-with-body --max-time 30 -sS -b "$COOKIE_JAR" \
  "$BFF_ORIGIN/api/fluctlights/$MOMENT_FLUCTLIGHT_ID/autonomy-actions" >"$action_json"

jq -e 'any(.[]; .status == "visible" and (.text | strings | length > 0))' "$moment_json" >/dev/null
jq -e 'any(.[]; .action_type == "moment" and .status == "completed" and ((.workflow_id | strings) | startswith("go-autonomy:")))' "$action_json" >/dev/null

curl --fail-with-body --max-time 30 -sS -b "$COOKIE_JAR" \
  "$BFF_ORIGIN/api/fluctlights/$PROACTIVE_FLUCTLIGHT_ID/conversation" >"$conversation_json"
curl --fail-with-body --max-time 30 -sS -b "$COOKIE_JAR" \
  "$BFF_ORIGIN/api/fluctlights/$PROACTIVE_FLUCTLIGHT_ID/autonomy-actions" >"$action_json"
jq -e 'any(.[]; .action_type == "proactive_message" and .status == "completed" and ((.workflow_id | strings) | startswith("go-autonomy:")))' "$action_json" >/dev/null
jq -e 'any(.messages[]; .kind == "assistant" and (.text | strings | length > 0))' "$conversation_json" >/dev/null

echo "Go projection checks: PASS"
