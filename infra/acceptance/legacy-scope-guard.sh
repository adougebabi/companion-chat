#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

patterns='better-sqlite3|/api/companion|companion_jobs|server/index.js|Node/SQLite|Express|companion-chat|COMPANION_DEBUG_INSPECTOR|COMPANION_|MTPLX_URL|COMFYUI_URL|fluctlight_core|@fluctlight/bff|PYTHONPATH=/workspace/apps/core/src|uv run|serve-api|run-worker|run-migrations'
scan_paths=()
for path in apps packages infra .github README.md package.json pnpm-workspace.yaml package-lock.json .env.example pyproject.toml uv.lock; do
  if [[ -e "$path" ]]; then
    scan_paths+=("$path")
  fi
done
if [[ "${#scan_paths[@]}" == 0 ]]; then
  echo "legacy scope guard: FAIL (no repository paths available for scope scan)" >&2
  exit 1
fi
set +e
grep -I -n -r --exclude-dir="__pycache__" --exclude-dir="node_modules" --exclude-dir="dist" --exclude-dir="build" --exclude-dir=".git" --exclude-dir=".mypy_cache" --exclude-dir=".ruff_cache" --exclude-dir=".pytest_cache" --exclude-dir=".gomodcache" --exclude-dir=".gocache" --exclude-dir=".gocache-review" --exclude-dir=".trellis" --exclude-dir="server" --exclude-dir="web" --exclude-dir="test" --exclude-dir="acceptance" -E "$patterns" "${scan_paths[@]}"
status=$?
set -e
if [[ "$status" == 0 ]]; then
  echo "legacy scope guard: FAIL (legacy production references remain)" >&2
  exit 1
fi
if [[ "$status" != 1 ]]; then
  echo "legacy scope guard: FAIL (scope scan could not complete)" >&2
  exit 1
fi
legacy_targets=(server web test Dockerfile compose.yaml package-lock.json .env.example .nvmrc apps/core apps/bff apps/core/Dockerfile apps/bff/Dockerfile .github/workflows/docker-publish.yml)
for target in "${legacy_targets[@]}"; do
  if [[ -e "$target" ]]; then
    echo "legacy scope guard: FAIL (legacy target remains: $target)" >&2
    exit 1
  fi
done
echo "legacy scope guard: PASS"
