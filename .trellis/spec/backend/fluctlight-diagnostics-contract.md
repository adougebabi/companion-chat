# Fluctlight Diagnostics Contract

## Scenario: Built-In Local Debugging Without External Telemetry Stack

### 1. Scope / Trigger

- Trigger: BFF/Core/Worker emits a diagnostic event, a model call runs, a cognitive turn/workflow/event/media chain needs inspection, or Owner queries/exports/cleans diagnostics.
- First delivery does not require OpenTelemetry, Prometheus, Grafana, Loki, Tempo, or an external collector.
- Diagnostics is operational/debug data and remains separate from authoritative domain audit/revision/evidence.

### 2. Signatures

PostgreSQL application tables:

```text
diagnostic_events
diagnostic_model_runs
diagnostic_turns
diagnostic_workflow_links
```

```python
emit(record: DiagnosticRecord) -> None
record_model_run(run: ModelRunDiagnostic) -> None
query(filter: DiagnosticFilter) -> DiagnosticPage
tail(filter: DiagnosticFilter) -> AsyncIterator[DiagnosticRecord]
export(command: ExportDiagnostics) -> RedactedBundle
clear(command: ClearDiagnostics) -> ClearResult
```

Correlation fields include source, level/code, Fluctlight/Actor/Conversation/turn/workflow/step/event/outbox/inbox/media IDs, correlation/causation IDs, timestamps, bounded context, and outcome.

### 3. Contracts

- Model runs capture role, endpoint/model/version, prompt/schema/policy version, redacted rendered prompt layers, bounded raw/structured response, parse/schema diagnostics, token/timing/timeout/cancel status, and evidence references.
- Hidden reasoning fields are discarded or reduced to an explicitly safe bounded summary; they are never stored as full reasoning.
- Typed redaction removes settings/API keys, cookies, sessions, service credentials, auth headers, object grants, `.env` values, and other secret types before persistence/stdout/export.
- Diagnostic writes are asynchronous/batched and never participate in business Unit of Work. Failure/backpressure drops or routes bounded records to structured stdout without failing business behavior.
- Diagnostic sink errors cannot recursively emit into the same sink.
- BFF submits bounded batched diagnostics through service-auth internal ingestion; it never writes PostgreSQL directly.
- Default retention: model runs/turns 30 days and 10,000 rows; structured events 14 days and 50,000 rows; workflow links 30 days. Settings may lower/raise bounded values.
- Lifecycle cleanup enforces age and row limits. Domain audit/revision/evidence tables are excluded.
- Owner-only UI/API supports filter, live tail, correlation chain, prompt/response comparison, turn state transitions, workflow links, clear, and redacted export.
- Opening the Diagnostics UI must invoke its data loader. An empty local store is
  never evidence that PostgreSQL has no diagnostics.
- Events, model runs, and optional workflow-runtime status are independent read
  operations. A Temporal runtime failure may render a bounded workflow warning,
  but must not hide successfully loaded model prompts, responses, or events or
  misreport the error as an Owner authorization failure.
- Description analysis response provenance includes the diagnostic correlation
  ID. The creation review surface retains it and can open a pre-filtered
  diagnostic view for that exact initialization run.
- Foundation validation failures expose a bounded structured detail object at
  the Core/BFF boundary, including `details.validation_error` and a safe error
  type. Clients must preserve this detail; a stable top-level code alone is not
  sufficient to diagnose missing or misrouted model fields.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Record contains typed secret/credential | Redact before persistence/stdout/export. |
| Model response contains hidden reasoning field | Drop/full-reasoning deny; retain only allowed structured output/bounded diagnostic. |
| Diagnostics PostgreSQL write fails | Do not affect business result; emit bounded structured stdout if possible. |
| Diagnostic queue is full | Apply bounded drop/backpressure policy and counters; never block long business path indefinitely. |
| Sink failure tries to emit a diagnostic | Use non-recursive stderr/stdout fallback only. |
| BFF ingestion lacks service identity or exceeds batch/schema bounds | Reject ingestion without domain effect. |
| Retention cleanup fails | Record bounded stdout/error and retry lifecycle workflow; do not delete domain audit. |
| Non-Owner queries/exports/clears | Reject before returning diagnostic content. |
| Workflow runtime is unavailable while reading diagnostics | Keep loaded events/model runs visible; show a workflow-only unavailable state. |
| Owner opens diagnostics from Settings | Invoke the same loader as a filter submission; do not only mutate the active view. |

### 5. Good / Base / Bad Cases

- Good: Owner opens one turn correlation view and sees redacted prompt layers, structured assessment, policy deltas, frozen action, realization, workflow attempts, and final message.
- Base: diagnostics database insert fails during a successful chat; chat succeeds and a bounded JSON record appears on stdout.
- Bad: require Grafana to inspect a local prompt, log API keys, put diagnostic rows in the business transaction, or delete relationship revisions during retention cleanup.

### 6. Tests Required

- Typed-redaction tests with credentials in nested request/response/header/URL/config objects and exported bundles.
- Model-run tests for full prompt layers, bounded raw/structured response, parse errors, provenance, hidden-reasoning drop, and token/timing data.
- Sink tests for batching, queue bounds, database outage, stdout fallback, non-recursion, and no business rollback/latency lock.
- BFF ingestion tests for service auth, schema/batch limits, correlation fields, and no direct database access.
- Retention tests for age/row dual limits and explicit proof that domain audit/revision/evidence remains.
- Owner authorization tests for query/tail/export/clear and no diagnostic access through ordinary product DTOs.
- UI/e2e test filters by IDs/role/status/time and traverses a complete correlation chain.

### 7. Wrong vs Correct

#### Wrong

```python
async with business_uow.begin() as tx:
    await diagnostics.save(full_request_with_api_key, tx=tx)
    await conversations.commit_turn(turn, tx=tx)
```

#### Correct

```python
result = await conversations.commit_turn(turn)
diagnostics.emit(redactor.model_run(model_run_record))
return result
```

## Scenario: Go Diagnostic Producers And Retention

### 1. Scope / Trigger

- Trigger: Go Provider calls, Core mutations or Worker failures need Owner-only
  inspection without making diagnostics part of a business transaction.

### 2. Signatures

- Provider calls record `diagnostic_model_runs` and `provider_provenance` with
  role, endpoint/model, correlation ID, prompt and bounded response.
- `ClearDiagnosticsCount` deletes diagnostic tables and returns `{cleared:n}`.

### 3. Contracts

- Diagnostic writes are best-effort and never fail the domain operation.
- Recursive redaction removes credentials, cookies, API keys and hidden
  reasoning before persistence or export.
- Owner authorization and correlation/fluctlight filters apply to reads.
- Periodic retention deletes only diagnostic tables, never domain audit or
  revision/evidence rows.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| non-Owner diagnostic read/clear | forbidden before content is returned |
| malformed correlation/filter or negative limit | bounded default/validation error |
| diagnostic sink unavailable | business result remains successful |
| retention cleanup fails | bounded Worker warning and retry |

### 5. Good/Base/Bad Cases

- Good: a Provider failure is visible as a redacted failed model run while the
  chat error remains bounded.
- Base: clearing diagnostics returns the number of deleted records.
- Bad: persisting raw authorization headers or deleting relationship history
  during retention.

### 6. Tests Required

- Recursive redaction, Owner isolation, filters, clear counts and age/row
  retention tests against PostgreSQL.
- Provider success/failure producer tests with sink failure injection.

### 7. Wrong vs Correct

#### Wrong

```go
INSERT INTO diagnostic_model_runs(prompt) VALUES ($1) // raw request
```

#### Correct

```go
recordModelRun(redactDiagnostic(prompt), boundedResponse, correlationID)
```
