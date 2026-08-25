#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

live_schema=$(mktemp)
live_paths=$(mktemp)
artifact_paths=$(mktemp)
trap 'rm -f "$live_schema" "$live_paths" "$artifact_paths"' EXIT

.venv/bin/python -c 'import json; from fluctlight_core.transport.api import create_app; print(json.dumps(create_app().openapi()))' > "$live_schema"

jq -r '.paths | to_entries[] | [(.key | gsub("\\{[^}]+\\}"; "{param}")), (.value | keys | sort | join(","))] | @tsv' "$live_schema" | sort > "$live_paths"
jq -r '.paths | to_entries[] | [(.key | gsub("\\{[^}]+\\}"; "{param}")), (.value | keys | sort | join(","))] | @tsv' packages/core-client/openapi.json | sort > "$artifact_paths"

if ! diff -u "$artifact_paths" "$live_paths"; then
  echo "core OpenAPI artifact paths/methods drift from FastAPI schema" >&2
  exit 1
fi

live_version=$(jq -r '.info.version' "$live_schema")
artifact_version=$(jq -r '.info.version' packages/core-client/openapi.json)
if [[ "$live_version" != "$artifact_version" ]]; then
  echo "core OpenAPI version drift: live=$live_version artifact=$artifact_version" >&2
  exit 1
fi

echo "core-openapi-check: live FastAPI paths/methods and version match artifact"
