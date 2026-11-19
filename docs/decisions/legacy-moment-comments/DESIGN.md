# Legacy Moment comment operation identity

2026-11-19. Scope: the compatible `/api/moments/:momentId/comments` and
`/internal/moments/:momentID/comments` endpoints. The current Moment feed remains
on its established revision/reason command contract.

## Decision and compatibility

Accept an optional `idempotencyKey` (Core: `idempotency_key`). Callers retain the
same key across retries of an unknown outcome. Omission retains legacy behavior:
two requests, including identical text, intentionally create two comments. The
server cannot distinguish a retry without operation identity and does not derive
identity from text or invent a new random request key.

A keyed command binds the authenticated Actor, target Moment, operation and exact
request text. Reuse with changed input conflicts. Keep the original four-field
comment response and 32000-code-point text limit; do not add audit/replay fields
or force the newer feed's 2000-character/reason/revision requirements.

## Atomicity and lock order

Reuse `owner_surface_idempotency`; no second receipt store or migration is added.
Comment, Moment revision, outbox and original response commit in one transaction.
A failed outbox write rolls back the receipt as well, allowing the same key to be
retried. No model, HTTP, renderer or other external work occurs in this transaction.

Keyed comments first acquire the common Owner-surface receipt lock, then the
existing Owner/world/lifecycle coordinator, operation lock and Moment row lock.
The common lock serializes commands such as new-direct-conversation that use the
same receipt table without the world coordinator. The world coordinator also
serializes the existing feed's command ledger. Recheck target ownership before
returning an existing receipt; a key is never an authorization credential.

## Security boundary and accepted limitations

Browser session, CSRF and Core service/Human authentication remain mandatory.
The browser maps only the declared text/key; errors retain bounded public status
and codes. Another Actor cannot retrieve the first Actor's receipt. A committed,
authorized operation replays after plugin disable or hiding; new operations are
still rejected. Replay returns a historical receipt, not permission for new work.

No business data or old comments are rewritten. Unkeyed legacy callers do not
receive an exactly-once retry guarantee; upgraded clients can supply the key.

## Validation

Real HTTP plus disposable PostgreSQL covers response loss, concurrent retry,
changed target/text/operation, cross-entry key contention with a controlled row
lock, ownership/session/CSRF, hidden/plugin admission, old response/long-text
compatibility, Unicode keys and transaction rollback. Generated contract/client
checks cover the optional key and original response. These checks do not claim
real model, media or production-environment validation.
