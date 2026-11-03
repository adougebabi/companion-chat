# Technical design

## Boundaries

The public topology remains:

```text
static Web → Go BFF → Go Core API → PostgreSQL / Redis / MinIO / Temporal
                                      ↑
                                Go Core Worker
```

This task owns only the durable platform path and Worker runtime. Existing
product handlers remain behaviorally frozen unless a platform seam requires a
small adapter change. The Go Core remains the only domain writer.

## Outbox and Redis

`platform_outbox_events` is the authoritative event ledger. A Worker-owned
publisher claims rows with a short lease-like status transition, publishes one
bounded JSON envelope to a versioned Redis Stream, then marks `published_at`.
The event ID, idempotency key, correlation ID and causation ID are preserved.
Provider/Temporal calls never run inside the PostgreSQL transaction that writes
the outbox row.

Each durable consumer group uses Redis Streams for delivery and PostgreSQL for
idempotency:

```text
XREADGROUP/XAUTOCLAIM
        ↓
consumer inbox (event_id + group)
        ↓
consumer effect/head transaction
        ↓
XACK
```

Duplicate event IDs return the prior inbox/effect result. Handler failures
increment a bounded attempt count; after the maximum, a poison record is
written and the stream entry is acknowledged so it cannot spin forever.

## Temporal Worker

The Worker starts exactly one poller per canonical queue. All workflow starts
use the normalized `go:` namespace. The workflow registry includes the
existing lifecycle/media/autonomy/reflection/memory workflows plus explicit
cognition processing and platform control wrappers. Activities call public
Core application methods; workflow bodies remain deterministic.

The dispatcher updates durable intent state only after Temporal accepts a start.
If that update fails, the next reconciliation pass describes both pending and
started intents and repairs the ledger. A terminal Temporal execution maps to
an explicit completed/cancelled/failed intent status.

## Compatibility and rollback

Migrations are additive and preserve released IDs and all business volumes.
Redis stream names and consumer groups are versioned constants. Rollback is a
Worker image selection change; PostgreSQL facts, outbox rows, inbox rows and
Temporal histories are never deleted.

## Observability

Every publisher/consumer/workflow transition emits bounded diagnostics with
correlation and causation IDs. Secrets, raw Provider responses and hidden
reasoning are redacted before persistence. Diagnostics failure cannot fail the
business transaction.
