# T08 Life World / Schedule / Autonomy Implementation Brief

## Status

Parent-authorized implementation brief for the fourth executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T07 Memory/Relationship public contracts and handoff are available. T08 uses
T04 Goal/Intention services and T07 public Relationship/Memory ports; it does
not recreate their tables or read their repositories.

## Owned Paths

- `apps/core/src/fluctlight_core/life_world/**`
- `apps/core/src/fluctlight_core/autonomy/**`
- `apps/core/migrations/versions/0007_t08_life_world_autonomy.py`
- `apps/core/migrations/env.py` (T08 schema import only)
- `apps/core/src/fluctlight_core/transport/api.py` (readiness head only)
- `apps/core/src/fluctlight_core/entrypoints/worker.py` (workflow registration only)
- `apps/core/tests/life_world/**`, `apps/core/tests/autonomy/**`,
  `apps/core/tests/contract/test_t08_*.py`, `apps/core/tests/architecture/test_t08_*.py`

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**` and root legacy runtime files.
- T09-T12 modules/migrations; T08 emits typed action intents for future media/
  moments instead of importing those modules.
- Recreating T04 Goal/Intention tables, reading T07 foreign tables, importing
  FastAPI/Provider/Redis/S3 SDKs into domain contracts, or adding another queue.
- Occupation/weekday/clock/default-routine keyword heuristics for Context or
  Schedule state.

## Decisions And Contracts

Implement without changing D002, D007, D012-D016, D020, D026-D033 and D039.
The assigned contracts are `fluctlight-life-world-contract.md`,
`fluctlight-autonomy-contract.md`, `fluctlight-workflow-contract.md`,
`fluctlight-cognitive-runtime.md`, and `fluctlight-persistence-contract.md`.
Schedule versions are immutable and cover a complete local day; accepted
Context authority is Event > Schedule > explicit pending. Frozen actions reuse
stable IDs and are rechecked against permission, budget, quiet hours, cooldown,
Context, Schedule and revisions before execution.

## Implementation Checklist

1. Add typed Event, ScheduleItem/Version, Context and timezone contracts with
   full-day/overlap/gap/DST validation.
2. Add Life World Event/Schedule/Item and Autonomy policy/action/governance
   tables plus migration `0007`.
3. Implement proposal/accept/replan/future-only context resolution and
   timezone supersession without rewriting completed history.
4. Implement Goal/Intention workflow ports, typed triggers, permission/budget/
   quiet-hour/cooldown checks, frozen action and immutable governance.
5. Add focused contract/architecture/unit checks; Temporal restart/DST/real
   PostgreSQL and cross-module acceptance remains T12.

## Implementation Checks

```bash
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src/fluctlight_core/life_world apps/core/src/fluctlight_core/autonomy apps/core/tests/life_world apps/core/tests/autonomy apps/core/tests/contract/test_t08_*.py
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests/life_world apps/core/tests/autonomy apps/core/tests/contract/test_t08_*.py
```

## T12 Coverage IDs

`T08-LW-01` full-day immutable Schedule validation/acceptance;
`T08-LW-02` future-only replan and completed history; `T08-LW-03` Event >
Schedule > pending Context with Presence overlay; `T08-LW-04` timezone/DST
change and future replacement; `T08-AUT-01` Goal/Intention typed trigger and
workflow closure; `T08-AUT-02` permission/budget/quiet-hours/cooldown/frozen
action; `T08-AUT-03` pause/resume/cancel/reconciliation governance;
`T08-AUT-04` stable retry IDs and no heuristic semantic trigger.

## Rollback Point

Before T09 starts, revert only T08-owned paths and migration `0007` if the
Life World/Autonomy contract gate cannot be satisfied. Preserve T05-T07 and
prior unrelated edits.

## Implementation Evidence Handoff

Record changed paths, contract/schema artifacts, implementation-check
commands/results, remaining risks, excluded scope, T12 coverage IDs and the
rollback point. State `acceptance_owner=T12` and `acceptance=pending`; no child
PASS, production readiness or cutover is established here.
