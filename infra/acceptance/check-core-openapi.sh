#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

artifact_paths=$(mktemp)
go_paths=$(mktemp)
trap 'rm -f "$artifact_paths" "$go_paths"' EXIT

jq -r '.paths | to_entries[] | [(.key | gsub("\\{[^}]+\\}"; "{param}")), (.value | keys | sort | join(","))] | @tsv' packages/core-client/openapi.json | sort > "$artifact_paths"

node - "$go_paths" <<'NODE'
const fs = require("fs");
const out = process.argv[2];
const source = fs.readFileSync("apps/core-go/internal/httpapi/server.go", "utf8");
const methods = new Map();
for (const match of source.matchAll(/mux\.HandleFunc\("(GET|POST|PUT|DELETE|PATCH) ([^"]+)/g)) {
  const path = match[2].replace(/\{[^}]+\}/g, "{param}");
  if (!methods.has(path)) methods.set(path, new Set());
  methods.get(path).add(match[1].toLowerCase());
}
const rows = [...methods.entries()].map(([path, values]) => `${path}\t${[...values].sort().join(",")}`);
fs.writeFileSync(out, rows.sort().join("\n") + "\n");
NODE

if ! diff -u "$artifact_paths" "$go_paths"; then
  echo "core OpenAPI artifact paths/methods drift from Go Core route inventory" >&2
  exit 1
fi

artifact_version=$(jq -r '.info.version' packages/core-client/openapi.json)
if [[ -z "$artifact_version" || "$artifact_version" == "null" ]]; then
  echo "core-openapi-check: artifact version is missing" >&2
  exit 1
fi
echo "core-openapi-check: Go Core route inventory matches artifact version $artifact_version"
