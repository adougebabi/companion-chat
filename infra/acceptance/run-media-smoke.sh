#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

project="fluctlight-t12-media-$$"
compose_file="infra/compose/fluctlight.compose.yml"
env_file="${FLUCTLIGHT_ENV_FILE:-infra/compose/fluctlight.env.example}"
compose=(docker compose --project-name "$project" --env-file "$env_file" -f "$compose_file")
source "$(dirname "$0")/read-compose-env.sh"
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
  if [[ "${KEEP_MEDIA_SMOKE:-0}" == "1" ]]; then return; fi
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
compose_started=1
"${compose[@]}" up --build --detach
for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T core python -c 'import urllib.request; urllib.request.urlopen("http://127.0.0.1:8080/health/ready")' >/dev/null 2>&1; then break; fi
  sleep 2
done

setup_token=$("${compose[@]}" exec -T core uv run --no-sync fluctlight issue-setup-token --expires-minutes 10 | tr -d '\r\n')
echo "media-smoke: setup token issued"
seed_headers=$("${compose[@]}" exec -T -e TRUSTED_ORIGIN="$trusted_origin" bff sh -c '
  curl -sS -D - -o /dev/null \
    -H "Origin: ${TRUSTED_ORIGIN}" \
    "http://127.0.0.1:${BFF_PORT:-3000}/auth/session"
')
csrf_token=$(printf '%s\n' "$seed_headers" | grep -i '^set-cookie: fluctlight_csrf=' | sed -n 's/^[^:]*: fluctlight_csrf=\([^;]*\).*/\1/p' | head -n 1)
[[ -n "$csrf_token" ]]
setup_headers=$("${compose[@]}" exec -T -e TRUSTED_ORIGIN="$trusted_origin" -e SETUP_TOKEN="$setup_token" -e CSRF_TOKEN="$csrf_token" bff sh -c '
  curl -sS --fail -D - -o /dev/null -X POST \
    -H "Origin: ${TRUSTED_ORIGIN}" \
    -H "X-CSRF-Token: ${CSRF_TOKEN}" \
    -H "Cookie: fluctlight_csrf=${CSRF_TOKEN}" \
    -H "Content-Type: application/json" \
    --data "{\"setupToken\":\"${SETUP_TOKEN}\",\"password\":\"fluctlight-smoke-password\"}" \
    "http://127.0.0.1:${BFF_PORT:-3000}/auth/setup"
')
session_cookie=$(printf '%s\n' "$setup_headers" | grep -i '^set-cookie: fluctlight_session=' | sed -n 's/^[^:]*: fluctlight_session=\([^;]*\).*/fluctlight_session=\1/p' | head -n 1)
new_csrf_token=$(printf '%s\n' "$setup_headers" | grep -i '^set-cookie: fluctlight_csrf=' | sed -n 's/^[^:]*: fluctlight_csrf=\([^;]*\).*/\1/p' | head -n 1)
if [[ -n "$new_csrf_token" ]]; then csrf_token="$new_csrf_token"; fi
[[ -n "$session_cookie" ]]
echo "media-smoke: session resolved"
fluctlight_json=$("${compose[@]}" exec -T -e TRUSTED_ORIGIN="$trusted_origin" -e CSRF_TOKEN="$csrf_token" -e SESSION_COOKIE="$session_cookie" bff sh -c '
  curl -sS --fail -X POST \
    -H "Origin: ${TRUSTED_ORIGIN}" \
    -H "X-CSRF-Token: ${CSRF_TOKEN}" \
    -H "Cookie: fluctlight_csrf=${CSRF_TOKEN}; ${SESSION_COOKIE}" \
    -H "Content-Type: application/json" \
    --data '{"name":"BFF media Fluctlight"}' \
    "http://127.0.0.1:${BFF_PORT:-3000}/api/fluctlights"
')
fluctlight_id=$(printf '%s\n' "$fluctlight_json" | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^\"]*\)".*/\1/p')
echo "media-smoke: Fluctlight ${fluctlight_id} created"

"${compose[@]}" exec -T -e FLUCTLIGHT_ID="$fluctlight_id" core uv run --no-sync python - <<'PY'
import hashlib
import os
import boto3
import psycopg

body = b"media-smoke-bytes"
key = "media/asset-smoke/v1"
client = boto3.client(
    "s3",
    endpoint_url=os.environ["S3_ENDPOINT"],
    region_name=os.environ["S3_REGION"],
    aws_access_key_id=os.environ["S3_ACCESS_KEY"],
    aws_secret_access_key=os.environ["S3_SECRET_KEY"],
)
result = client.put_object(Bucket=os.environ["S3_BUCKET"], Key=key, Body=body, ContentType="image/png")
digest = hashlib.sha256(body).hexdigest()
version = result.get("VersionId")
with psycopg.connect(os.environ["DATABASE_URL"]) as connection:
    with connection.cursor() as cursor:
        cursor.execute(
            """
            INSERT INTO media_assets
              (id, owner_fluctlight_id, version, kind, mime_type, byte_size, sha256,
               bucket, object_key, object_version, etag, provider_request_id,
               workflow_id, status, created_at, ready_at)
            VALUES
              ('asset-smoke', %s, 'v1', 'image', 'image/png', %s, %s, %s, %s, %s,
               %s, 'provider-smoke', 'workflow-smoke', 'ready', now(), now())
            """,
            (os.environ["FLUCTLIGHT_ID"], len(body), digest, os.environ["S3_BUCKET"], key, version, result.get("ETag")),
        )
PY
echo "media-smoke: object and DB asset seeded"

set +e
proxy_output=$("${compose[@]}" exec -T -e SESSION="$session_cookie" bff sh -c '
  headers=$(mktemp)
  body=$(mktemp)
  status=$(curl -sS -D "$headers" -o "$body" -w "%{http_code}" \
    -H "Cookie: ${SESSION}" \
    -H "Range: bytes=0-4" \
    http://127.0.0.1:${BFF_PORT:-3000}/api/media/asset-smoke)
  [ "$status" = 206 ]
  [ "$(cat "$body")" = media ]
  grep -qi '^content-range:' "$headers"
  printf 'media-proxy-ok %s %s %s\n' "$status" "$(cat "$body")" "$(grep -i '^content-range:' "$headers" | tr -d '\r' | sed 's/^[^:]*:[[:space:]]*//')"
  rm -f "$headers" "$body"
 ' 2>&1)
proxy_status=$?
set -e
echo "media-smoke: proxy command status=${proxy_status}"
printf '%s\n' "$proxy_output"
if [[ "$proxy_status" != 0 ]]; then exit 1; fi
echo "media-smoke: proxy response passed"
