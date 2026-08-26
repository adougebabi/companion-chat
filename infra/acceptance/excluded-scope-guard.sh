#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

roots=(apps packages infra)
patterns='group-chat|multi-human|external irreversible|placeholder-only delivery|fake-only adapter'
for root in "${roots[@]}"; do
  if [[ ! -d "$root" ]]; then
    echo "excluded scope guard: FAIL (missing production scope: $root)" >&2
    exit 1
  fi
done
set +e
grep -n -i -r --exclude-dir="node_modules" --exclude-dir="dist" --exclude-dir="build" --exclude-dir=".git" --exclude-dir=".mypy_cache" --exclude-dir=".ruff_cache" --exclude-dir=".pytest_cache" --exclude-dir=".trellis" --exclude-dir="acceptance" -E "$patterns" "${roots[@]}"
status=$?
set -e
if [[ "$status" == 0 ]]; then
  echo "excluded scope guard: FAIL (excluded capability appears in production scope)" >&2
  exit 1
fi
if [[ "$status" != 1 ]]; then
  echo "excluded scope guard: FAIL (scope scan could not complete)" >&2
  exit 1
fi
echo "excluded scope guard: PASS"
