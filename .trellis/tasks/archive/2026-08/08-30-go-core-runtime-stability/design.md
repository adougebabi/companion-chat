# Technical design

## Migration boundary

`apps/core-go` is a new module with `cmd/api` and `internal/*` packages. The
first slice owns only transport/auth/lifecycle/conversation-target persistence
and reads/writes through a narrow PostgreSQL repository interface. The existing
Python Core remains the sole owner of cognition, media, reflection, autonomy
and Temporal workflow tables until a later handoff. The Go API is introduced
behind a separate Compose service and is not allowed to write a Python-owned
row. The BFF can be pointed at Go Core only for endpoints whose ownership is
explicitly transferred.

The Go module mirrors the stable Core wire contract (snake_case) and keeps
browser DTO mapping in `apps/gateway-go`. Both modules use explicit config and
health checks; no generic proxy or domain logic is added to the BFF.

## Contract freeze

Export the live FastAPI OpenAPI from the running/source Python app, reconcile
the committed artifact and generated client, then add JSON fixtures for the
cross-language effect contracts. Fixtures are versioned and consumed by both
Python tests and Go tests; they are not used as fake provider responses in
Docker acceptance.

## Compound-effect settlement

Normalize and validate the complete effect list before realization. The
validator enforces the primary `reply`/`no_op` position and the allowed
autonomous side-effect set. Only after validation succeeds does the service
freeze/realize the primary effect and independently settle secondary effects,
each with a stable effect idempotency key. `process_next` and `stream_next`
settle one terminal result on failure; replay resolves existing user/assistant
messages and media intents rather than inserting duplicates.

## Lifecycle and daily review

Compose a `DailyLifeReviewRegistrar` in the API root and give it an optional
transaction handle. Activation/create writes inner-state baseline, schedule
intent, direct-conversation target, and the current-local-date review intent in
one transaction. A failed registration aborts activation. Worker startup scans
only to repair historical active rows and uses durable intent state to avoid
restarting already-running workflows.

## Reflection and Worker recovery

Reflection provider output is parsed into a strict discriminated schema before
any persistence. Candidate and relationship fields are validated with typed
errors; apply and watermark advance share one transaction. A malformed item is
deferred/failed with bounded diagnostics and can be retried without losing its
watermark.

The dispatcher claims at most `limit` durable intents per pass, records
dispatch/terminal state, and treats an existing Temporal execution as reuse.
Reflection intents are coalesced per Fluctlight/evidence watermark. Media,
reflection, and lifecycle/daily-review have independent concurrency budgets so
old work cannot starve a newly activated Fluctlight.

## Rollout and rollback

The first Go Core service is opt-in in Compose and CI. During rollout, the BFF
continues to call Python for endpoints without transferred ownership. A failed
Go health/contract check leaves Python active; rollback is a config/service
selection change, not a database rollback or dual-write period.

## Acceptance evidence

Code-level evidence includes focused tests plus Go/Python build checks. Runtime
evidence is collected from the real Docker stack through the public BFF and
Core endpoints for cases 1–7, with request timeout <= 600 seconds and no
manual Worker restart between activation and proactive-review assertions.
