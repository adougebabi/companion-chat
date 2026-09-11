# Fluctlight BFF Contract

## Scenario: Typed Browser Boundary Without Domain Ownership

### 1. Scope / Trigger

- Trigger: a browser command/query/stream/media request crosses the public
  BFF, or BFF calls Go Core.
- The active and only BFF uses Go's standard library HTTP server and client.
  Browser uses Vue 3/Vite/Pinia; no Node BFF runtime remains.
- BFF owns browser transport/session/DTOs but no Fluctlight domain state, persistence, workflow, or semantic policy.

### 2. Signatures

- `packages/core-client`: generated from the Go Core OpenAPI contract.
- `packages/browser-client`: generated from the checked browser OpenAPI artifact;
  the Go BFF must preserve this contract even though it does not generate the
  artifact itself.
- Browser turn transport: POST `fetch()` with `application/x-ndjson` response.
- BFF package interfaces: browser API, stream translator, media proxy, health,
  and session transport.

Browser stream envelope:

```text
BrowserTurnEventV1
  type: token | message | media | completed | error | heartbeat
  turn_id
  sequence
  payload
```

### 3. Contracts

- Go routes validate body/query/params/headers and call the HTTP Core client or
  BFF transport modules only.
- BFF cannot import PostgreSQL/Redis/Temporal clients, Core module internals, domain repositories, or semantic rule modules.
- Go Core and browser OpenAPI artifacts and their generated clients are
  committed/reviewed together; hand-written duplicate DTOs are prohibited.
- Internal Core NDJSON is parsed incrementally across arbitrary byte/chunk boundaries, schema-validated, redacted, and mapped to browser events.
- One browser turn has monotonic sequence and exactly one terminal event. BFF never forwards hidden assessment, Provider chunks, credentials, database rows, or workflow internals.
- Optimistic/queued/stream drafts use a local message/turn/idempotency identity
  and never guess an authoritative server sequence. An authoritative assistant
  may replace only the draft bound to its own turn; it cannot match an unrelated
  queued user bubble merely because a sequence number or text happens to match.
- Post-stream history is merged monotonically under the request epoch. A stale
  history response cannot erase stream-confirmed/queued messages or overwrite a
  later selected conversation; refresh/crash recovery keeps queued text bound
  to its original conversation until durable submission.
- Browser retry/queued state is provisional client state. During clean-start,
  server reset, persona deletion or conversation recreation, an identity that
  no longer belongs to an available Fluctlight/current direct conversation is
  pruned before send. Before a retry from another available conversation can
  block a new send, the browser reconciles that conversation's authoritative
  history; a retry whose user message already has a later assistant message is
  cleared, while an unresolved retry remains recoverable and is never mistaken
  for the current turn.
- Browser disconnect/abort cancels BFF upstream read and Core request. BFF suppresses later browser writes while Core settles committed work independently.
- BFF media route obtains a Go Core authorization grant and proxies only the granted object/version/range with bounded headers.
- Go package boundaries organize transport/config lifecycle; the BFF is not a
  location for Fluctlight business behavior.
- BFF errors use stable browser codes/messages and correlation IDs mapped from Core errors without leaking stack/provider bodies.
- Core error `details` are untrusted internal data: before they cross the
  browser boundary, retain only bounded JSON values and drop credential,
  secret, token, prompt, reasoning, stack, and raw provider-response keys.
- A successful Core media status with a missing or empty response body is a
  bounded `media_unavailable` failure; the BFF never defers `Close` on a nil
  body or forwards an empty object-storage response as a successful asset.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Browser input fails the browser schema | Return stable 400 browser error; do not call Core. |
| Generated client is stale against OpenAPI | CI failure; regenerate and review. |
| Core stream has invalid JSON/schema/sequence | Emit one bounded browser error, abort upstream, record correlation diagnostic. |
| Core emits hidden/internal fields | Reject/redact contract violation; never forward them. |
| Core error details contain sensitive or oversized values | Drop those fields and cap nested collections/strings before returning the browser error. |
| Browser aborts | Abort Core fetch/read, stop browser writes, preserve Core settlement semantics. |
| Core returns typed domain error | Map by error code/status table; do not parse message text. |
| Media grant expired/range mismatched | Stop proxy and return bounded media error; do not mint another grant implicitly. |
| Core returns a successful media status without a body | Return bounded `media_unavailable`; do not panic or emit a false successful response. |
| Older retry assistant arrives while a newer user message is queued | Reconcile by local turn identity; preserve the queued user bubble and send it under its original conversation binding. |
| Post-stream history or previous-conversation request returns late | Epoch check and monotonic merge ignore stale regression; newer selection/messages remain authoritative. |
| Persisted retry references a deleted Fluctlight or recreated conversation | Prune the orphaned retry/queued identity and allow a new current-conversation send; do not block with “another conversation pending”. |
| Persisted retry's authoritative assistant already exists | Clear the provisional retry after history reconciliation and allow the current-conversation send; do not ask the user to retry a completed turn. |
| BFF code imports storage/workflow/domain internals | Architecture-test failure. |

### 5. Good / Base / Bad Cases

- Good: a browser turn uses generated client types, BFF validates input, maps ordered Core NDJSON, and aborts cleanly on navigation.
- Good: a video range request is authorized by Go Core and proxied without exposing bucket/key/credentials.
- Base: a typed query returns one BFF DTO composed from Core application results.
- Bad: hand-write matching DTOs, directly proxy raw Core JSON, parse Core error text, query Redis for domain state, or add a relationship rule in a transport handler.

### 6. Tests Required

- Semantic OpenAPI diff and generated-client no-drift tests for Core and browser contracts.
- `net/http/httptest` tests for schema-equivalent validation, stable errors,
  status codes, headers, session context, and server lifecycle.
- Incremental NDJSON tests for split/multiple frames, UTF-8 boundaries, invalid schema, redaction, sequence, heartbeat, terminal uniqueness, backpressure, and abort.
- End-to-end browser→BFF→Core cancellation tests with no writes after disconnect.
- Media proxy tests for authorization grant, expiry, Range, ETag, MIME, stream failure, and no storage detail leakage.
- Browser OpenAPI method/path artifact versus Go route inventory parity, plus
  a real HTTP browser→BFF→Core auth/conversation/NDJSON smoke and downstream
  disconnect cancellation test.
- Public error-detail sanitization tests for safe validation fields, sensitive
  keys, nesting depth, collection size, and string limits; media nil-body and
  header allow-list tests.
- Architecture tests rejecting BFF imports of PostgreSQL, Redis, Temporal, Core internals, domain repositories, and semantic heuristic modules.
- Browser tests consume generated client types and do not duplicate wire DTO definitions.
- Conversation-delivery regressions cover successful old retry plus queued new
  user turn, stale post-stream history, repeated text, conversation switching,
  and authoritative Core→BFF message fields.

### 7. Wrong vs Correct

#### Wrong

```typescript
fastify.post('/turn', async (request) => {
  const mood = request.body.text.includes('sorry') ? 'better' : 'same';
  await redis.set(`mood:${request.body.fluctlightId}`, mood);
  return core.rawTurn(request.body);
});
```

#### Correct

```typescript
fastify.post('/turn', {schema: turnRouteSchema}, async (request, reply) => {
  const upstream = await coreClient.acceptTurn(mapBrowserTurn(request));
  return translateCoreNdjson(upstream, reply, request.signal);
});
```
