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
service_key="$(printenv FLUCTLIGHT_CORE_SERVICE_KEY || true)"
if [[ -z "$service_key" ]]; then service_key="$(compose_env_value FLUCTLIGHT_CORE_SERVICE_KEY)"; fi
postgres_user="$(printenv POSTGRES_USER || true)"
if [[ -z "$postgres_user" ]]; then postgres_user="$(compose_env_value POSTGRES_USER)"; fi
postgres_db="$(printenv POSTGRES_DB || true)"
if [[ -z "$postgres_db" ]]; then postgres_db="$(compose_env_value POSTGRES_DB)"; fi
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
session=$("${compose[@]}" exec -T -e SERVICE_KEY="$service_key" -e SETUP_TOKEN="$setup_token" bff node -e '
const setup = await fetch("http://core:8080/internal/auth/setup", {method: "POST", headers: {"x-fluctlight-service-key": process.env.SERVICE_KEY, "content-type": "application/json"}, body: JSON.stringify({setup_token: process.env.SETUP_TOKEN, password: "fluctlight-smoke-password"})});
if (!setup.ok) process.exit(1);
console.log((await setup.json()).session_token);
')
echo "media-smoke: session resolved"
fluctlight_id=$("${compose[@]}" exec -T -e SERVICE_KEY="$service_key" -e SESSION="$session" bff node -e '
const response = await fetch("http://core:8080/internal/fluctlights", {method: "POST", headers: {"x-fluctlight-service-key": process.env.SERVICE_KEY, "x-fluctlight-human-session": process.env.SESSION, "content-type": "application/json"}, body: JSON.stringify({name: "T12 media Fluctlight"})});
if (!response.ok) process.exit(1);
console.log((await response.json()).id);
')
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
proxy_output=$("${compose[@]}" exec -T -e SERVICE_KEY="$service_key" -e SESSION="$session" bff node -e '
try {
  const response = await fetch("http://bff:3000/api/media/asset-smoke", {headers: {cookie: `fluctlight_session=${process.env.SESSION}`, range: "bytes=0-4"}});
  if (response.status !== 206) { console.error("status", response.status, Object.fromEntries(response.headers), await response.text()); process.exit(1); }
  const body = await response.text();
  if (body !== "media") { console.error("body", body); process.exit(1); }
  if (!response.headers.get("content-range")) process.exit(1);
  console.log("media-proxy-ok", response.status, body, response.headers.get("content-range"));
} catch (error) {
  console.error("proxy exception", error);
  process.exit(1);
}
 ' 2>&1)
proxy_status=$?
set -e
echo "media-smoke: proxy command status=${proxy_status}"
printf '%s\n' "$proxy_output"
if [[ "$proxy_status" != 0 ]]; then exit 1; fi
echo "media-smoke: proxy response passed"
