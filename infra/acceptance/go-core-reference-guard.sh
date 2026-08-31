#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$root"

# Acceptance fixtures may retain Python snippets as historical or oracle tests;
# the guard only scans active runtime/build/deployment paths.
scan=(apps/gateway-go apps/web packages .github infra/compose infra/backup README.md CONTEXT.md)
patterns='(^|/)(apps/core|apps/bff)(/|$)|fluctlight_core|Python Core|Python FastAPI|PYTHONPATH=/workspace/apps/core/src|uv run|serve-api|run-worker|run-migrations|alembic upgrade|Alembic'

set +e
matches=$(grep -I -n -r -E --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=dist --exclude-dir=build --exclude-dir=acceptance --exclude-dir=archive "$patterns" "${scan[@]}" 2>/dev/null)
status=$?
set -e
if [[ "$status" == 0 ]]; then
  printf '%s\n' "$matches" >&2
  echo "Go Core reference guard: FAIL" >&2
  exit 1
fi
if [[ "$status" != 1 ]]; then
  echo "Go Core reference guard: unable to scan" >&2
  exit 1
fi
echo "Go Core reference guard: PASS"
