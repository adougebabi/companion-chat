# Fluctlight API Contract

## Scenario: Generated Core API And Cancellable Internal Stream

### 1. Scope / Trigger

- Trigger: the public browser boundary calls the Core App, Go exposes a command/query, or
  incremental visible output crosses the Core/browser transport boundary.
- Go's net/http transport and generated OpenAPI types are composition tools, not domain dependencies.
- Browser contracts are owned by the public browser boundary and are distinct from this internal contract.

### 2. Signatures

- Synchronous commands/queries: versioned HTTP/JSON described by generated OpenAPI.
- Internal stream content type: `application/x-ndjson`.
- The reference Core client is generated from the checked OpenAPI artifact;
  the Go API browser boundary preserves the same contract. Hand-written
  duplicate domain DTOs are prohibited.

Canonical stream envelope:

```text
VisibleStreamEventV1
  type: token | action_result | completed | error | heartbeat
  turn_id
  sequence
  payload
```

Health endpoints:

```text
GET /health/live
GET /health/ready
```

### 3. Contracts

- HTTP routes call application interfaces only. They never receive raw PostgreSQL transactions or module repositories.
- Domain modules cannot import transport-specific request/response types or HTTP exception types.
- Pydantic validates transport/config/Provider schemas. Mapping to domain commands occurs at the adapter seam.
- OpenAPI changes and generated TypeScript client changes commit together; CI rejects ungenerated drift.
- NDJSON sequence is monotonic per turn and has exactly one terminal `completed` or `error`. Heartbeats do not change domain/action state.
- Internal stream exposes only visible text/content progress, action results, bounded errors, and terminal metadata. It never exposes perception, appraisal, hidden reasoning, raw Provider chunks, credentials, database rows, or Temporal internals.
- The public private-chat turn accepts the user message and `cognition.processing`
  intent before observing output. Browser abort/disconnect cancels only the
  observer request; the Worker owns realization and its Provider context.
- A retried conversation request reuses the original `turn_id` and `idempotency_key`. The Core responder must bind processing to that fact ID and replay or reopen it in place; it must never consume another pending fact for the request.
- `/health/live` has no dependency probes. `/health/ready` checks required configuration, PostgreSQL, and the `serve-api` role; optional Provider outage is reported separately and does not fail readiness.
- API process does not poll Temporal task queues or execute background Provider Activities.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Request/response violates OpenAPI/Pydantic schema | Return typed bounded internal error; do not call application command on invalid input. |
| Generated/reference client differs from OpenAPI artifact | CI failure; regenerate and review the artifact and clients. |
| Stream sequence repeats/skips unexpectedly | Terminate bounded error, record correlation diagnostics, never silently reorder. |
| More than one terminal event | Contract failure; the browser boundary forwards only the first terminal and records violation. |
| Browser/browser boundary disconnects after acceptance | Stop observer writes; leave the committed inbox and Worker/Provider execution untouched. |
| Provider unavailable | Return application-defined failure/status; do not map Core readiness to false unless required startup configuration is invalid. |
| PostgreSQL unavailable at readiness probe | `/health/ready` fails; liveness remains independent. |
| Domain module imports FastAPI/Pydantic Web DTO | Architecture-test failure. |

### 5. Good / Base / Bad Cases

- Good: OpenAPI keeps the browser contract stable, one turn streams ordered
  NDJSON, and a disconnected observer leaves the accepted Worker turn running.
- Base: a command returns one typed JSON result with correlation and no stream.
- Bad: hand-write matching Go/TypeScript DTOs, return raw ORM rows, stream hidden assessment data, or inject a database session into a route handler.

### 6. Tests Required

- OpenAPI snapshot/semantic-diff and generated-client no-drift tests.
- Route tests for Pydantic validation, mapping to application commands, stable errors, correlation/causation, and no raw row leakage.
- NDJSON parser/producer tests for chunking, partial frames, UTF-8, monotonic sequence, heartbeat, one terminal, error, and abort.
- End-to-end browser boundary→Core test that suppresses writes after disconnect
  and verifies the Worker still commits one reply from the accepted inbox.

## Scenario: Durable Private Chat Turn And Explicit Cancellation

### 1. Scope / Trigger

- Trigger: `POST /api/conversations/{conversationId}/turn` accepts a private
  message, the browser disconnects, or the Owner clicks the explicit cancel
  button. The legacy internal synchronous `HandleTurn` caller is separate.

### 2. Signatures

```text
App.AcceptTurn(ctx, actorID, conversationID, payload) -> TurnResult{UserMessage, TurnID, InboxID}
App.StreamDurableTurn(ctx, writer, actorID, conversationID, payload) -> NDJSON observer
POST /api/conversations/{conversationId}/turn/{turnId}/cancel -> 204
History user message: turnId, idempotencyKey, turnStatus,
  turnErrorCode? and turnRetryable? (failed/cancelled only)
turnStatus: pending | running | completed | failed | cancelled
```

### 3. Contracts

- Acceptance writes `conversation_messages`, `cognition_inbox`, and
  `cognition.processing` intent in one short transaction; only Worker claims
  and runs the model for the public private-chat path. The observer emits only
  committed user/assistant frames and terminal state. Its context must not
  reach the Worker.
- The inbox payload preserves both `actor_id` (speaker) and
  `authorization_actor_id` (Owner). Worker replay calls `HandleActorTurn` when
  they differ; legacy rows without the latter recover the Owner from the
  Fluctlight record. A Worker must not authorize a Fluctlight sender as Owner.
- The authenticated cancel command marks the inbox `failed/user_cancelled`,
  stops Provider and active Temporal work, and fences late reply publication.
  A claimed inbox keeps the workflow intent in `cancel_requested` until the
  execution terminates; history sets `turnRetryable=false` during that window.
- A retry reuses the original turn/idempotency and user message. It reopens the
  failed inbox after the prior workflow is terminal, clears its cancellation
  marker, and gives the intent a new workflow ID. The Agent's attempt RunID may
  change, while its conversation publication correlation remains `turn:<turnId>`.
- A committed assistant reply takes precedence over a later projection failure
  in the history/stream visible result. Only a real failed or cancelled turn
  without a committed reply offers retry.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Browser disconnects after acceptance | Observer stops; inbox remains pending/claimed and Worker continues. |
| Cancel is requested by a non-participant or for an unknown turn | 404/unauthorized boundary; no mutation. |
| Cancel arrives after processed/failed terminal state | Idempotent no-op; do not mark a completed reply cancelled. |
| Retry arrives while cancellation is still settling | Conflict until the old workflow is terminal; history exposes `turnRetryable=false`. |
| Provider fails before reply | Bounded error code, failed history state, original identity eligible for retry. |
| Provider result is late after explicit cancellation | Inbox/publication guard rejects the write. |

### 5. Good / Base / Bad Cases

- Good: close the browser after acceptance, let Worker commit one reply, then
  reload the conversation and read `completed` from history.
- Base: cancel a pending turn before Worker dispatch; no Provider call occurs,
  and the same user message can be retried.
- Bad: pass the browser request context into `RunConversationCognitionAgent`
  or infer cancellation from an `AbortError` in Pinia.

### 6. Tests Required

- Isolated PostgreSQL tests assert disconnect leaves inbox pending, Worker
  commits one reply, cancel stops a running Provider, retry preserves one user
  message, Actor sender authorization survives Worker dispatch, and stale
  reply publication is fenced.
- Browser route/OpenAPI matrix, generated-client drift and frontend reload
  tests assert the cancel path and status fields agree across layers.

### 7. Wrong vs Correct

#### Wrong

```go
result, err := app.HandleTurn(request.Context(), actor, conversation, body)
// request disconnect cancels the model and marks the accepted fact failed
```

#### Correct

```go
accepted, err := app.AcceptTurn(acceptContext, actor, conversation, body)
// The response context only observes accepted.InboxID; Worker processes it.
```
- Liveness/readiness tests for PostgreSQL/config/optional Provider states and API-vs-Worker role separation.
- Architecture tests preventing FastAPI/Starlette/Uvicorn/Web DTO imports in domain modules and Temporal task-queue polling in API runtime.
- Real PostgreSQL HTTP integration tests plus in-process application-interface tests.

### 7. Wrong vs Correct

#### Wrong

```python
@app.post("/turn")
async def turn(dto: TurnDTO, session: AsyncSession = Depends(get_session)):
    row = await session.execute(text("select * from inner_state"))
    return dict(row.first())
```

#### Correct

```python
@app.post("/turn", response_model=TurnAcceptedDTO)
async def turn(dto: TurnRequestDTO, commands: TurnCommandsDep):
    command = map_turn_request(dto)
    result = await commands.accept_turn(command)
    return map_turn_result(result)
```

## Scenario: Strict Go Core Boundary And Workflow Namespaces

### 1. Scope / Trigger

- Trigger: a Core mutation receives JSON, a workflow is managed through the
  API, or an NDJSON completion crosses the browser boundary.

### 2. Signatures

- Core request bodies decode as exactly one JSON object with no trailing value.
- Temporal management normalizes durable intent IDs to the `go:` namespace for
  status, history, signal, cancel, reset and restart.

### 3. Contracts

- CAS conflicts produce no mutation and an explicit conflict result.
- Reset accepts only a real `WorkflowTaskCompleted` event ID.
- Completed stream payloads expose browser-visible message IDs only; workflow,
  Provider and media-intent internals stay inside Core.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| concatenated/trailing JSON | bounded request-validation error |
| stale expected revision | no mutation and explicit conflict |
| unknown workflow ID | not found; no Temporal call |
| reset point is not a completed workflow task | reject before reset |

### 5. Good/Base/Bad Cases

- Good: raw and `go:` workflow IDs address one execution and one audit row.
- Base: an empty body is accepted only for bodyless pause/resume/cancel.
- Bad: accepting a second JSON value or forwarding `media_intent_id` to the
  browser.

### 6. Tests Required

- Route tests for malformed/trailing JSON, missing CAS fields, unauthorized
  resources and conflict no-mutation behavior.
- Temporal adapter tests for ID normalization, history-point validation and
  command idempotency.
- NDJSON tests for frame bounds and completion payload allow-listing.

### 7. Wrong vs Correct

#### Wrong

```go
decoder.Decode(&body) // ignore a second JSON value
client.DescribeWorkflowExecution(ctx, rawWorkflowID, "")
```

#### Correct

```go
body, ok := decodeObjectBody(limitedBody)
workflowID = normalizedWorkflowID(rawWorkflowID)
```

## Scenario: Actor Group Response Compatibility

### 1. Scope / Trigger

- Trigger: actor groups cross the Go Core/browser boundary or the browser
  filters the Fluctlight directory by group.
- This contract preserves the established browser field name while allowing a
  rolling deployment to read the previous `members` response.

### 2. Signatures

- `GET /internal/actor-groups` and `GET /api/actor-groups` return an array of
  group objects.
- `POST /internal/actor-groups` and `POST /api/actor-groups` return the created
  group object.

### 3. Contracts

- The authoritative member field is `actor_ids: string[]`; `owner_actor_id`
  and `created_at` remain additive metadata.
- Browser normalization accepts `actor_ids` or legacy `members`, filters out
  non-string entries, and always stores an array (empty when absent).
- browser boundary remains a transport pass-through; Core owns the domain response shape.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Core returns `actor_ids` | Browser uses the string array unchanged after filtering. |
| Rolling deployment returns `members` | Browser maps it to `actor_ids` before any `includes` call. |
| Member field is missing, null, or not an array | Browser treats the group as having zero members; no render exception. |
| Group object has no string `id`/`name` | Browser drops the malformed group and keeps other groups usable. |

### 5. Good/Base/Bad Cases

- Good: Core returns `{id, name, actor_ids: ["fl-1"]}` and desktop/mobile
  filters show only `fl-1`.
- Base: an old browser boundary returns `{id, name, members: []}` and the normalized group
  remains selectable without a crash.
- Bad: a view reads `group.actor_ids.includes(...)` directly from an untrusted
  API payload before normalization.

### 6. Tests Required

- Core response tests assert list/create payloads contain `actor_ids`.
- Browser normalizer tests cover `actor_ids`, legacy `members`, empty/missing
  fields, and non-string members.
- Desktop and mobile directory regression tests assert group filtering never
  invokes `includes` on `undefined`.

### 7. Wrong vs Correct

#### Wrong

```ts
this.actorGroups = await client.listActorGroups();
group.actor_ids.includes(fluctlightId);
```

#### Correct

```ts
this.actorGroups = normalizeActorGroups(await client.listActorGroups());
group.actor_ids.includes(fluctlightId); // always a string[]
```

## Scenario: Actor Relationship Projection And Governance

### 1. Scope / Trigger

- Trigger: relationship state crosses Core, browser boundary, browser detail/governance, or Provider context boundaries.
- Human and Fluctlight are both Actors. created_by_actor_id and Owner account remain authorization metadata; they do not automatically create a social Relationship.

### 2. Signatures

    GET /api/fluctlights/{fluctlightId}/detail
    PUT /api/fluctlights/{fluctlightId}/relationships/{targetActorId}
    POST /api/fluctlights/{fluctlightId}/relationships/rollback
    relationship.lookup({target_actor_id}) -> read-only direct relationship

Edit request fields include expectedRevision, role, metrics, trend, summary, emotionalAssociation, evidenceRefs, and reason.

### 3. Contracts

- Detail relationship rows include target_actor_id, target_actor_type, is_current_user, role, metrics, trend, summary, provenance and revision.
- is_current_user is computed by Core from the authenticated Human Actor ID; a browser field cannot establish authority or override the marker.
- targetActorId is path identity and is immutable during an edit. A successful edit appends relationship_revisions and relationship_governance rows with action=edit.
- Relationship provenance distinguishes initialization, reflection, and manual; authorization metadata is separate from social relationship data.
- Provider context uses a leading system relationship snapshot for self and the current speaker. Transport role=user is not the domain sender identity.
- Reflection may autonomously create/update relationship-scoped goals and intentions. Conversation cognition reads the snapshot but does not mutate long-lived relationship or agency rows.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/negative expectedRevision | 422; no mutation |
| Missing evidence refs or reason | browser boundary rejects before Core; no mutation |
| Unknown role code, invalid trend, or metric outside 0..1 | 422; no revision |
| Stale expected revision | 409; no mutation |
| Browser submits a different Actor as current user | Ignore/reject; session Actor remains authoritative |
| Relationship lookup targets a non-participant Actor | Read-only lookup rejected; no relationship data returned |

### 5. Good/Base/Bad Cases

- Good: the UI labels a Human relationship as “当前用户”, edits its role with revision 3, and detail reload shows revision 4 plus an audit row.
- Base: a Fluctlight target is shown as Fluctlight with the same relationship editor; no User-only code path is used.
- Bad: treating created_by_actor_id as the social role, allowing target Actor replacement in a PUT, or returning the full relationship list from a lookup.

### 6. Tests Required

- Core detail tests assert actor type and authenticated-current-user marker.
- Relationship edit tests assert role/metrics/trend/summary persistence, CAS, immutable target, provenance and governance audit.
- browser boundary tests assert camelCase↔snake_case mapping, validation and route/OpenAPI parity.
- Browser tests assert current-user labeling, safe relationship rendering, editor fields and conflict recovery after refresh.
- Provider projection tests assert mixed Human/Fluctlight sender aliases and no raw Actor IDs in model-facing context.

### 7. Wrong vs Correct

Wrong: infer the social role owner from created_by_actor_id.
Correct: compute is_current_user from the authenticated session Actor, and read the social role from the persisted Relationship.
