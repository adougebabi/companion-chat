# Fluctlight API Contract

## Scenario: Generated Core API And Cancellable Internal Stream

### 1. Scope / Trigger

- Trigger: the public BFF calls Go Core, Go exposes a command/query, or
  incremental visible output crosses the Core→BFF process boundary.
- Go's net/http transport and generated OpenAPI types are composition tools, not domain dependencies.
- Browser contracts are owned by the public BFF and are distinct from this internal contract.

### 2. Signatures

- Synchronous commands/queries: versioned HTTP/JSON described by generated OpenAPI.
- Internal stream content type: `application/x-ndjson`.
- The reference Core client is generated from the checked OpenAPI artifact;
  the Go BFF's HTTP Core client preserves the same contract. Hand-written
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
- Public BFF response abort/disconnect propagates to the Core ASGI request and
  realization cancellation. Committed assessment/state/frozen decision is not
  rolled back by transport disconnect.
- A retried conversation request reuses the original `turn_id` and `idempotency_key`. The Core responder must bind processing to that fact ID and replay or reopen it in place; it must never consume another pending fact for the request.
- `/health/live` has no dependency probes. `/health/ready` checks required configuration, PostgreSQL, and the `serve-api` role; optional Provider outage is reported separately and does not fail readiness.
- API process does not poll Temporal task queues or execute background Provider Activities.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Request/response violates OpenAPI/Pydantic schema | Return typed bounded internal error; do not call application command on invalid input. |
| Generated/reference client differs from OpenAPI artifact | CI failure; regenerate and review the artifact and clients. |
| Stream sequence repeats/skips unexpectedly | Terminate bounded error, record correlation diagnostics, never silently reorder. |
| More than one terminal event | Contract failure; the BFF forwards only the first terminal and records violation. |
| Browser/BFF disconnects during realization | Propagate cancellation; suppress later writes; settle frozen action per workflow policy. |
| Provider unavailable | Return application-defined failure/status; do not map Core readiness to false unless required startup configuration is invalid. |
| PostgreSQL unavailable at readiness probe | `/health/ready` fails; liveness remains independent. |
| Domain module imports FastAPI/Pydantic Web DTO | Architecture-test failure. |

### 5. Good / Base / Bad Cases

- Good: OpenAPI keeps the browser contract stable, one turn streams ordered
  NDJSON, the BFF translates it, and disconnect cancels realization without
  reverting the frozen decision.
- Base: a command returns one typed JSON result with correlation and no stream.
- Bad: hand-write matching Go/TypeScript DTOs, return raw ORM rows, stream hidden assessment data, or inject a database session into a route handler.

### 6. Tests Required

- OpenAPI snapshot/semantic-diff and generated-client no-drift tests.
- Route tests for Pydantic validation, mapping to application commands, stable errors, correlation/causation, and no raw row leakage.
- NDJSON parser/producer tests for chunking, partial frames, UTF-8, monotonic sequence, heartbeat, one terminal, error, and abort.
- End-to-end BFF→Core streaming cancellation test with suppression of writes after disconnect.
- Liveness/readiness tests for PostgreSQL/config/optional Provider states and API-vs-Worker role separation.
- Architecture tests preventing FastAPI/Starlette/Uvicorn/Web DTO imports in domain modules and Temporal task-queue polling in API runtime.
- Real PostgreSQL ASGI integration tests plus in-process application-interface tests.

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
  API, or an NDJSON completion crosses the BFF boundary.

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

- Trigger: actor groups cross the Go Core/BFF/browser boundary or the browser
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
- BFF remains a transport pass-through; Core owns the domain response shape.

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
- Base: an old BFF returns `{id, name, members: []}` and the normalized group
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

- Trigger: relationship state crosses Core, BFF, browser detail/governance, or Provider context boundaries.
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
| Missing evidence refs or reason | BFF rejects before Core; no mutation |
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
- BFF tests assert camelCase↔snake_case mapping, validation and route/OpenAPI parity.
- Browser tests assert current-user labeling, safe relationship rendering, editor fields and conflict recovery after refresh.
- Provider projection tests assert mixed Human/Fluctlight sender aliases and no raw Actor IDs in model-facing context.

### 7. Wrong vs Correct

Wrong: infer the social role owner from created_by_actor_id.
Correct: compute is_current_user from the authenticated session Actor, and read the social role from the persisted Relationship.
