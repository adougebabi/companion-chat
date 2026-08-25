# T05 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/cognition/` contracts, schema, service and
  Temporal workflow adapter.
- `apps/core/src/fluctlight_core/diagnostics/` contracts, redaction, schema and
  isolated persistence service.
- `apps/core/src/fluctlight_core/platform/cognition_events.py` stable event
  names and `apps/core/src/fluctlight_core/entrypoints/cognition_worker.py` the
  one-item worker adapter.
- `apps/core/src/fluctlight_core/entrypoints/worker.py` registers the cognition
  workflow; `apps/core/src/fluctlight_core/transport/api.py` exposes the
  Owner-authenticated diagnostics query/clear routes.
- `apps/core/src/fluctlight_core/transport/diagnostics.py` owns the redacted
  diagnostics transport projection.
- `apps/core/migrations/versions/0004_t05_cognition_diagnostics.py` and
  `apps/core/migrations/env.py` register the T05 tables and linear head.
- T05 contract, architecture and package tests under `apps/core/tests/`.

## Implementation Evidence

```text
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check <T05 paths>
All checks passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff format --check <T05 paths>
24 files already formatted
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests
80 passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline mypy --follow-imports=skip <T05 sources>
Success: no issues found in 5 source files
```

## Produced Contracts / Schema

- One sequence head and one lease owner per Fluctlight, with idempotent fact
  enqueue, ordered claim, CAS-style freeze and terminal action result.
- Typed assessment/decision/frozen-action/realization/reflection ports with
  stable action and Provider request IDs.
- Diagnostics event/model-run/turn/workflow-link tables with recursive secret
  and hidden-reasoning redaction, Owner-only query/export/clear and bounded
  retention.
- Alembic head `0004_t05_cognition_diagnostics`.

## Remaining Risks / Excluded Scope

- T02's shared platform inbox constraint name, aggregate sequence and Redis
  commit-before-ack/reclaim implementation remain carry-forward work; T05 did
  not silently change those shared primitives.
- Provider network adapters are injected ports. The local evidence does not
  prove real Provider, Redis, PostgreSQL, or Temporal restart behavior.
- T06 owns Conversation/NDJSON/BFF transport; T07-T11 own their domain and
  operations modules. No future-only or placeholder-only capability is claimed.
- Full capability, security/redaction, failure, cross-module, Compose and
  backup/restore validation remains T12-only.

## T12 Coverage

`T05-CGN-01`, `T05-CGN-02`, `T05-CGN-03`, `T05-CGN-04`, `T05-DIA-01`,
`T05-DIA-02`, `T05-DIA-03`, and `T05-EVT-01` from the child brief must be
re-run by T12.

Rollback point: remove only T05-owned paths and migration `0004` before T06 if
the integration gate cannot be satisfied; preserve unrelated T03/T04 worktree
changes.
