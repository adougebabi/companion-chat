# T08 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/life_world/` typed Event, full-day Schedule,
  Context authority, timezone validation and persistence service.
- `apps/core/src/fluctlight_core/autonomy/` policy snapshot, frozen action,
  stable workflow/provider IDs, governance and reconciliation service.
- Migration `0007_t08_life_world_autonomy`, metadata registration, readiness
  head and T08 contract/architecture tests.

## Implementation Evidence

```text
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests
99 passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src apps/core/tests
All checks passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline mypy --follow-imports=skip <T08 sources>
Success: no issues found in 3 source files
```

## Produced Contracts / Schema

- Alembic head `0007_t08_life_world_autonomy` with life events, immutable
  schedule versions/items, presence overlays, autonomy policies/actions and
  governance rows.
- Full local-day Schedule validation rejects gaps/overlaps and uses aware
  timezone boundaries. Context resolution is Event > accepted Schedule >
  explicit pending.
- Autonomy policy rechecks mode, allowed action, budget, cooldown and quiet
  hours before freeze; governance is append-only and retries preserve stable
  workflow/provider IDs.

## Remaining Risks / Excluded Scope

- T04 Goal/Intention workflow integration and Temporal durable timers are public
  ports/implementation evidence only; T12 must exercise them with real Worker
  restart/recovery.
- DST fold/gap, timezone replacement, real PostgreSQL CAS and cross-module
  Memory/Relationship checks remain T12-only. No occupation/weekday/clock or
  default-routine heuristic exists.
- T09+ media/moments, UI, backup and legacy deletion are excluded.

## T12 Coverage

Re-run `T08-LW-01` through `T08-LW-04` and `T08-AUT-01` through `T08-AUT-04`
from the child brief.

Rollback point: remove only T08-owned paths and migration `0007` before T09 if
the Life World/Autonomy contract gate cannot be satisfied.
