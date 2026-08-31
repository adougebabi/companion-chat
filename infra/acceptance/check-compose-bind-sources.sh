#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
compose_file="${1:-$script_dir/../compose/fluctlight.compose.yml}"
compose_dir=$(cd -- "$(dirname -- "$compose_file")" && pwd)

required_files=(
  "$compose_dir/../postgres/10-temporal-databases.sql"
  "$compose_dir/../redis/redis.conf"
)

for path in "${required_files[@]}"; do
  if [[ ! -f "$path" ]]; then
    echo "Compose bind source is missing or not a regular file: $path" >&2
    exit 1
  fi
done

echo "Compose bind sources: PASS"
