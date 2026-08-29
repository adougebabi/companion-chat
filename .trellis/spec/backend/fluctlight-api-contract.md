# Fluctlight API Contract

## Scenario: Generated Core API And Cancellable Internal Stream

### 1. Scope / Trigger

- Trigger: the public BFF calls Python Core, Python exposes a command/query, or
  incremental visible output crosses the Core→BFF process boundary.
- Python uses pinned 3.13, FastAPI, Pydantic v2, and Uvicorn. These are transport/composition tools, not domain dependencies.
- Browser contracts are owned by the public BFF and are distinct from this internal contract.

### 2. Signatures

- Synchronous commands/queries: versioned HTTP/JSON described by generated OpenAPI.
- Internal stream content type: `application/x-ndjson`.
- The reference Node Core client is generated from the checked OpenAPI artifact;
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

- FastAPI routes call application interfaces only. They never receive raw PostgreSQL sessions or module repositories.
- Domain modules cannot import FastAPI, Starlette, Uvicorn, Web Pydantic DTOs, or HTTP exception types.
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
- Bad: hand-write matching Python/TypeScript DTOs, return raw ORM rows, stream hidden assessment data, or inject a database session into a route handler.

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
