# T09 Moments / Media Implementation Brief

## Status

Parent-authorized implementation brief for the fifth executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T08 Life World/Autonomy public contracts and handoff are available. T09
consumes typed action intents and does not read Life World or Cognition tables.

## Owned Paths

- `apps/core/src/fluctlight_core/moments/**`
- `apps/core/src/fluctlight_core/media/**`
- `apps/core/src/fluctlight_core/platform/object_storage.py` (generic adapter
  extensions only)
- `apps/core/migrations/versions/0008_t09_moments_media.py`
- `apps/core/migrations/env.py` and `apps/core/src/fluctlight_core/transport/api.py`
  (schema import/readiness head only)
- `apps/core/tests/moments/**`, `apps/core/tests/media/**`,
  `apps/core/tests/contract/test_t09_*.py`, `apps/core/tests/architecture/test_t09_*.py`

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**` and root legacy runtime files.
- T10-T12 UI/operations/cutover modules; T09 exposes media grant values for a
  later BFF proxy but does not implement browser UI or backup scripts.
- Public buckets, user-controlled object locators, local absolute Provider
  paths, direct Redis domain state, or a second workflow/queue runtime.

Carry-forward from T01B: T12 final acceptance must cover actual media Activity heartbeat/cancel and live Provider-success-before-completion crash recovery with stable IDs; this child only prepares implementation seams and evidence. No fixed-duration gate.

## Decisions And Contracts

Implement without changing D002, D005-D007, D018-D020, D023-D025, D031-D033,
and D039. The assigned contracts are `fluctlight-media-contract.md`,
`fluctlight-event-contract.md`, `fluctlight-workflow-contract.md`,
`fluctlight-autonomy-contract.md`, and `fluctlight-persistence-contract.md`.
PostgreSQL owns media identity/lifecycle; S3-compatible storage owns bytes;
Progress is ephemeral, durable events are outbox-driven, and object grants are
short-lived/internal.

## Implementation Checklist

1. Add Moment/feed/comment/reaction/visibility/unread contracts and tables.
2. Add Media intent/asset/reference/tombstone/grant contracts and tables with
   checksum/size/version/provider/workflow identity.
3. Implement private-object upload/authorize/attach/tombstone/delete/orphan
   seams and idempotent Provider heartbeat/cancel/progress adapter ports.
4. Add quality/progress/compensation state transitions without exposing local
   paths or provider internals.
5. Add focused contract/architecture/unit checks; real MinIO, Range proxy,
   crash/recovery and backup acceptance remains T12.

## Implementation Checks

```bash
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src/fluctlight_core/moments apps/core/src/fluctlight_core/media apps/core/tests/moments apps/core/tests/media apps/core/tests/contract/test_t09_*.py
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests/moments apps/core/tests/media apps/core/tests/contract/test_t09_*.py
```

## T12 Coverage IDs

`T09-MOM-01` Moment visibility/feed/unread; `T09-MOM-02` comment/reaction
authorization; `T09-MED-01` committed intent→upload→checksum/version→ready;
`T09-MED-02` private grant/reference authorization and Range;
`T09-MED-03` Provider stable ID/heartbeat/cancel/compensation;
`T09-MED-04` tombstone/delete/orphan retry; `T09-EVT-01` durable event and
ephemeral progress recovery; `T09-MED-05` object backup manifest linkage.

## Rollback Point

Before T10 starts, revert only T09-owned paths and migration `0008` if the
Moments/Media contract gate cannot be satisfied. Preserve T05-T08 and prior
unrelated edits.

## Implementation Evidence Handoff

Record changed paths, contract/schema artifacts, implementation-check
commands/results, remaining media/provider/restore risks, excluded scope, T12
coverage IDs and rollback point. State `acceptance_owner=T12` and
`acceptance=pending`; no child PASS, production readiness or cutover is
established here.
