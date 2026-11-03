# Technical design

## Target topology

```text
static Web → Go BFF → Go Core API → PostgreSQL / Redis / MinIO / Temporal
                                      ↑
                                Go Core Worker
```

Go Core is a modular monolith. `cmd/api` owns HTTP composition, `cmd/worker`
owns Temporal polling and intent dispatch, and `internal/*` packages own
domain/application capabilities. BFF remains transport-only. Middleware is
unchanged.

## Ownership and cutover

Create an explicit table/workflow ownership matrix. Each table has one Go
writer before its route is switched. During development Python may be run only
as a compatibility oracle or migration utility; it cannot be a second writer.
The cutover order is auth/config → foundation/actors → conversations → inner
state/life world → memory/relationships/reflection → media/moments → cognition
and autonomy → diagnostics/operations → workflow dispatcher/Worker.

Every port is contract-tested against the current Python behavior and shared
JSON fixtures. The BFF switches a route only after Go persistence, authorization
and failure behavior pass.

## Temporal cutover without Python-history compatibility

Freeze new Python intent starts, record active executions and their domain
intent IDs, then fence/cancel old Python workflows through the authorized
Temporal management API. PostgreSQL facts, media assets and audit records remain
authoritative. Rebuild only eligible pending intents with Go workflow IDs that
include a migration correlation; completed/failed/cancelled domain outcomes are
not replayed as new side effects. No Python and Go Worker share a queue during
the cutover, and cancellation/rebuild results are recorded for diagnostics.

## Contracts and data flow

- Core JSON remains snake_case; Browser JSON remains camelCase.
- `DecisionEffect`, `FrozenAutonomousAction`, `MediaIntent`,
  `ReflectionProposal`, `DailyReviewTrigger`, stream envelopes and error codes
  are versioned Go structs with strict validation.
- PostgreSQL transactions use short Unit-of-Work boundaries. Domain state,
  outbox rows and workflow intents commit atomically; external Provider/S3/
  Temporal calls happen outside the transaction with stable idempotency keys.
- Go Core emits the same NDJSON turn sequence and terminal semantics as Python;
  BFF translation remains unchanged.

## Provider/media portability

Provider role resolution, structured JSON schemas, streaming accumulation,
timeouts, diagnostics redaction and media prompt authority move behind Go
interfaces. Media generation retains provider job IDs, heartbeat/cancel,
download checksums, private MinIO objects and conversation/moment publication.
No semantic inference is implemented with keywords or defaults.

## Rollout / rollback

Build the Go Core/Worker images in a Go-only integration job. Run them in a
parallel disposable Compose project against a restored database snapshot for
contract/replay checks, then switch the production Compose service names after
the compatibility gate. Rollback is a service/image selection change while
preserving PostgreSQL and Temporal state; it is not a database reset.

## Resource objective

Measure RSS and startup time for Go Core/Worker after each ownership slice. Do
not claim memory improvement from an idle compatibility bridge. The acceptance
target is removal of Python Core/Worker processes from the final deployment.
