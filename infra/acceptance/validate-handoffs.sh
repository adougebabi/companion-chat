#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$BASH_SOURCE")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)
cd "$repo_root"

declare -a tasks=(
  "actors-auth-settings-providers:T03"
  "fluctlight-foundation-inner-state:T04"
  "cognition-inbox-diagnostics:T05"
  "conversations-chat-experience:T06"
  "memory-relationships-reflection:T07"
  "life-world-schedule-autonomy:T08"
  "moments-media:T09"
  "web-product-control-center:T10"
  "backup-restore-upgrade-operations:T11"
)

for entry in "${tasks[@]}"; do
  task_name="${entry%%:*}"
  task_prefix="${entry##*:}"
  handoff=".trellis/tasks/08-24-${task_name}/handoff.md"
  if [[ ! -f "$handoff" ]]; then
    echo "handoff validation: missing $handoff" >&2
    exit 1
  fi
  if ! grep -qE 'acceptance_owner=T12' "$handoff" || ! grep -qE 'acceptance=pending' "$handoff"; then
    echo "handoff validation: $handoff is missing T12 acceptance ownership" >&2
    exit 1
  fi
  if ! grep -qE '^## T12 Coverage' "$handoff" || ! grep -qE "${task_prefix}-[A-Z0-9]+-[0-9]+" "$handoff"; then
    echo "handoff validation: $handoff is missing concrete ${task_prefix} coverage IDs" >&2
    exit 1
  fi
  if ! grep -qE '^## Remaining Risks / Excluded Scope' "$handoff" || ! grep -qE '^Rollback point:' "$handoff"; then
    echo "handoff validation: $handoff is missing risk/exclusion or rollback evidence" >&2
    exit 1
  fi
done

echo "handoff validation: T03-T11 ownership, coverage, exclusions and rollback evidence passed"
