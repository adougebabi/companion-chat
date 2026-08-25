"""Transport projection for already-redacted diagnostics records."""

from __future__ import annotations

from typing import Any


def diagnostic_event_response(row: dict[str, Any]) -> dict[str, Any]:
    """Keep route serialization independent from diagnostics table rows."""

    return {
        "id": row["id"],
        "event_type": row["event_type"],
        "severity": row["severity"],
        "fluctlight_id": row.get("fluctlight_id"),
        "causation_id": row.get("causation_id"),
        "correlation_id": row["correlation_id"],
        "payload": dict(row.get("payload") or {}),
        "created_at": row.get("created_at"),
    }
