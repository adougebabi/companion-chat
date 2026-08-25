#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"
default_output="$repo_root/infra/acceptance/deletion-manifest.txt"
if [[ $# -gt 0 ]]; then
  requested_output="$1"
else
  requested_output="$default_output"
fi
case "$requested_output" in
  /*) output="$requested_output" ;;
  *) output="$repo_root/$requested_output" ;;
esac
case "$output" in
  "$repo_root"/*) ;;
  *) echo "refusing to write outside the repository: $output" >&2; exit 2 ;;
esac
mkdir -p "$(dirname "$output")"
temporary_output=$(mktemp "$repo_root/.t12-deletion-manifest.XXXXXX")
trap 'rm -f "$temporary_output"' EXIT
{
  echo "# T12 conditional legacy deletion manifest"
  echo "# Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  for target in server web test Dockerfile compose.yaml package-lock.json .env.example .nvmrc .github/workflows/docker-publish.yml; do
    if [[ -e "$target" ]]; then
      echo "PRESENT $target"
    else
      echo "ABSENT $target"
    fi
  done
  echo "# Deletion is permitted only after all Required gates pass."
} > "$temporary_output"
mv "$temporary_output" "$output"
trap - EXIT
echo "$output"
