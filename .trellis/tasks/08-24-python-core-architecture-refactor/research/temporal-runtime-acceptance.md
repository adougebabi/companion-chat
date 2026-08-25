# Temporal Runtime Acceptance

## Decision

On 2026-08-24 the user approved removing resource-duration hard gates. Parent D020/D036 accept Temporal as the sole workflow runtime despite the archived T01B report's historical `FAIL` status.

The report failed because its old contract required unrun long-duration/resource tests, not because Temporal core semantics failed. The historical report remains unchanged.

## Accepted Evidence

- Grouped non-HA Server with PostgreSQL default+visibility and no ES/UI/metrics.
- API/Worker separation and `interaction`/`lifecycle`/`media` task queues.
- Stable Workflow/Provider IDs and one final result.
- Durable timer across Worker/Temporal/PostgreSQL restarts.
- Signal pause/resume, Query status and validated Update/repair.
- Bounded heartbeat/timeout/cancellation.
- Saved Event History Replayer v1→v2.
- Current Worker Deployment Versioning coexist/current/rollback.
- Continue-as-new state continuity.
- Bounded backup rows, diagnostics correlation and clean DBOS removal.
- Measured approximately 139 MiB Temporal RSS and 425 MiB complete gate-stack RSS; sufficient for the 16 GiB target NAS.

## Removed As Hard Gates

- 12-hour target NAS soak or any fixed-duration stability run.
- Strict RSS/CPU maxima and leak-slope criteria.
- 30-day history/disk resource projection.
- Full 900-second fake h3 solely to prove duration.

These may be observed during normal implementation but cannot block T02 or release solely because a duration/resource study was not run.

## Functional Carry-Forward

| Owner | Required functional check |
| --- | --- |
| T02 | Live reset/restart/history-point replay; complete management operation integration and authorization/audit; normal health/readiness/cleanup. |
| T09 | Media Activity heartbeat/cancel/idempotency and live Provider-success-before-completion crash behavior using the actual media adapter. No fixed 15-minute duration requirement. |
| T11 | Start Temporal against restored default+visibility databases and prove active workflow resume. |
| T12 | Final product workflow e2e and recovery smoke, without fixed-duration/resource soak. |

## T02 Unblock Rule

T02 may start after its child-specific brief/manifests/owned paths/commands pass a no-history handoff dry run and the user explicitly starts the child. It does not wait for resource-duration tests.
