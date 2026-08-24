# T01 DBOS Runtime Gate Design

## Authority

Parent task `08-24-python-core-architecture-refactor` owns all architecture decisions. This child validates D020; it cannot replace DBOS, weaken thresholds or add another runtime.

## Gate Topology

```text
minimal gate API process
minimal gate Worker process
  -> DBOS system/application PostgreSQL
  -> interaction/lifecycle/media queues
  -> fake external h3/provider steps
  -> structured stdout correlation
```

No browser/BFF, Redis, MinIO, production domain schemas or external telemetry collector is required.

## Scenarios

- Stable committed gate intent starts one workflow exactly once.
- Durable sleep survives API/Worker/PostgreSQL restart.
- 15-minute fake h3 heartbeats, times out and cancels cooperatively.
- External success before checkpoint/result commit is recovered through stable Provider ID.
- Three queues enforce independent concurrency/rate policies.
- Admin wrapper supports list/get/pause/resume/cancel/restart/fork-from-step.
- Active workflow history survives code/schema upgrade fixture.
- Structured IDs reconstruct intent→workflow→step→provider→recovery→result.
- Quantified NAS resources stay within the parent brief thresholds.

## Failure Decision

Any required operation, resource, idempotency, recovery or upgrade gate failure yields `FAIL`, blocks T02+, and returns parent planning to Temporal evaluation. Celery/custom queue is prohibited.
