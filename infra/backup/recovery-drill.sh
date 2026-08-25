#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${FLUCTLIGHT_RECOVERY_WORKDIR:-}" ]]; then
  echo "Set FLUCTLIGHT_RECOVERY_WORKDIR to a disposable recovery directory." >&2
  exit 2
fi

case "$FLUCTLIGHT_RECOVERY_WORKDIR" in
  /tmp/*|/private/tmp/*) ;;
  *) echo "Recovery drill refuses non-temporary targets: $FLUCTLIGHT_RECOVERY_WORKDIR" >&2; exit 2 ;;
esac

echo "1. Restore PostgreSQL application, temporal and temporal_visibility snapshots into disposable volumes."
echo "2. Restore the private object bucket and verify manifest counts/SHA-256 samples."
echo "3. Run explicit Alembic upgrade to the manifest application head."
echo "4. Start Temporal, Core and Worker; query the active workflow ID from the manifest."
echo "5. Prove one active workflow resumes with its stable workflow/provider IDs."
echo "6. Record evidence under $FLUCTLIGHT_RECOVERY_WORKDIR and destroy the disposable volumes."
