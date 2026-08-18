# Persistence Assessment

## Finding

The current one-row `app_state.payload` JSON document is not a viable authority for durable activities, comments, life events, supporting characters, media references, cursor pagination, or retryable jobs. It is appropriate only for low-frequency global settings and a temporary compatibility projection.

## Evidence

- `server.js:28-89` keeps personas, memories, conversations, logs, and generation jobs in one JSON payload and overwrites the complete value through `saveState()`.
- `server.js:389` truncates conversations to 60 items; `server.js:249` truncates evolution history to 30 items. Neither supports a long-lived, pageable companion history.
- `server.js:443-555` starts a chat from a state snapshot, awaits a model stream, then writes that snapshot back. Concurrent settings, job, or evolution changes can be lost.
- `server.js:620-710` treats an in-memory boolean plus JSON array as the generation queue. It has no lease, retry metadata, or durable atomic claim.
- `src/main.js:534-551` already has a five-second visible-page refresh path, which is enough for initial feed/job refresh without expanding the existing chat SSE contract.

## Recommended Boundary

Keep the current deployment footprint: one Node process, `better-sqlite3`, WAL, and one `companion.sqlite` file. Introduce versioned schema migrations and normalized tables in that same database. Do not introduce an external database, queue, or second server process.

High-growth resources require table-level ownership and time indexes:

- personas, persona foundation revisions, private memories, and persona state
- conversations and messages
- supporting characters, schedules, life events, and activities
- activity comments, reactions, visibility preferences, media assets, and media links
- durable jobs with queued/leased/running/complete/failed states, retry metadata, and leases

`app_state` may retain settings plus a storage version, but must not remain the authority for new life-domain records.

## Migration Decision

The user explicitly chose not to migrate or read legacy local data. New table storage starts clean. Existing `app_state`, old SQLite records, and files must not be deleted during upgrade.

## Consequences

- Existing database guidance that requires full-document `readState()`/`saveState()` needs revision when implementation begins.
- New job claims must use a short SQLite transaction, with provider calls outside that transaction.
- New public APIs use cursor pagination based on a stable `(created_at, id)` ordering.
- The implementation validates the new schema on a fresh temporary database; legacy data compatibility is intentionally out of scope.
