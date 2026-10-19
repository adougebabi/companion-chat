#!/usr/bin/env bash
set -euo pipefail
: "${FLUCTLIGHT_RECOVERY_WORKDIR:?Set an explicitly disposable recovery work directory}"
: "${FLUCTLIGHT_RECOVERY_JOB:?Create an actual isolated restore job from the completed backup first}"
: "${FLUCTLIGHT_RECOVERY_ENV_FILE:?Set the source deployment private env file}"
case "$FLUCTLIGHT_RECOVERY_WORKDIR" in
  /tmp/*|/private/tmp/*) ;;
  *) echo 'Recovery drill requires an explicit temporary work directory.' >&2; exit 2 ;;
esac
case "$FLUCTLIGHT_RECOVERY_JOB" in
  restore_*) ;;
  *) echo 'Recovery drill accepts only a restore job, never a source backup operation.' >&2; exit 2 ;;
esac
task_backup_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$task_backup_dir/maintenance.py" \
  --job "$FLUCTLIGHT_RECOVERY_JOB" \
  --compose "${FLUCTLIGHT_RECOVERY_COMPOSE:-$task_backup_dir/../compose/fluctlight.compose.yml}" \
  --env-file "$FLUCTLIGHT_RECOVERY_ENV_FILE" \
  --workdir "$FLUCTLIGHT_RECOVERY_WORKDIR" \
  --maintenance-confirm
