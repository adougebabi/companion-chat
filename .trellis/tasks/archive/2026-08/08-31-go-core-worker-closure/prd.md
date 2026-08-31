# Close Go Core and Worker platform capabilities

## Goal

Close the remaining Go platform/Worker capabilities without changing the
already accepted product semantics for chat, media, creation, detail,
Moments, or proactive contact. The task must make the durable event path and
Temporal Worker ownership complete enough to satisfy the migration contract;
it is not complete when the process is merely healthy.

## Requirements

- Implement one Go outbox publisher from PostgreSQL to Redis Streams with
  bounded retry, stable event IDs, and published/completed/failed bookkeeping.
- Implement durable Go consumer processing for the configured event groups,
  including inbox idempotency, effect/head persistence, reclaim/poison failure
  records, and bounded retention. Redis is transport only; PostgreSQL remains
  authoritative.
- Register and exercise the remaining migrated Temporal workflow boundaries,
  including cognition processing and platform control, while keeping one
  Worker owner per canonical queue and stable `go:` workflow IDs.
- Move durable cognition processing triggers behind Worker activities without
  changing the public conversation payload or the accepted product behavior.
- Make dispatcher/reconciliation resilient to the crash window after Temporal
  accepts a start but before the intent status update commits.
- Add focused tests for event publication/consumption, duplicate delivery,
  reclaim/poison, workflow registration/replay, crash reconciliation and
  queue ownership. Do not rerun the baseline product suite for unrelated
  changes; rerun only if shared routes or persistence are touched.
- Do not restore Python Core, add a second domain writer, reset business
  volumes, or silently alter Provider/media semantics.

## Acceptance Criteria

- [ ] PostgreSQL outbox rows are published to Redis with stable IDs and a
      bounded retry/terminal-failure path; no eligible row remains permanently
      unpublished after a Worker restart.
- [ ] Each durable consumer group records inbox/effect/head state exactly once
      for duplicate deliveries and records poison failures without looping
      forever; domain facts are not duplicated.
- [ ] All required Go workflows/activities are registered on their canonical
      queue, replay in tests, and no queue has a second implementation owner.
- [ ] A committed cognition/event intent survives Worker restart and is either
      processed or reconciled to an explicit terminal state.
- [ ] Core/Gateway Go race tests, vet, build, OpenAPI/reference guards and
      focused platform integration tests pass.
- [ ] Existing baseline product evidence remains recorded; if shared Core,
      Provider, media, persistence or deployment code changes, the affected
      baseline cases are rerun before closure.
- [ ] Task is archived only after the full capability matrix and final
      acceptance review are green.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
