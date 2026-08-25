# T09 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/moments/` Moment/feed/comment/reaction/
  visibility/unread contracts, schema and authorization service.
- `apps/core/src/fluctlight_core/media/` intent/asset/reference/tombstone/grant
  contracts, schema, lifecycle service and stable Provider workflow adapter.
- `apps/core/src/fluctlight_core/platform/object_storage.py` range/TTL grant
  validation; migration `0008_t09_moments_media`, metadata registration,
  readiness head and T09 tests.

## Implementation Evidence

```text
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests
105 passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src apps/core/tests
All checks passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline mypy --follow-imports=skip <T09 sources>
Success: no issues found in 4 source files
```

## Produced Contracts / Schema

- Alembic head `0008_t09_moments_media` with Moment interaction and Media
  intent/asset/reference/tombstone tables.
- Private S3 object identity is generated as `media/{asset_id}/{version}`;
  checksum/byte-size/version/provider/workflow IDs are persisted before ready.
- Python authorization is required before short internal grants; tombstone
  removes references before retryable physical deletion. Provider workflow
  seam exposes stable submit/poll/cancel/heartbeat behavior.

## Remaining Risks / Excluded Scope

- Actual MinIO upload/Range proxy, Provider crash after success before result
  commit, Redis outbox/rebuild, and object backup/restore remain T12-only.
- T10 owns BFF media proxy and UI; T11 owns backup tooling. No public bucket,
  local absolute Provider path, second queue or semantic fallback was added.
- Fixed-duration resource soak is excluded per parent decision.

## T12 Coverage

Re-run `T09-MOM-01`, `T09-MOM-02`, `T09-MED-01` through `T09-MED-05`, and
`T09-EVT-01` from the child brief.

Rollback point: remove only T09-owned paths and migration `0008` before T10 if
the Moments/Media contract gate cannot be satisfied.
