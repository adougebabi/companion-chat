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
  fluctlight_id?
  metrics: object
  estimated_input_tokens?
  actual_prompt_tokens?
  actual_completion_tokens?
  latency_ms?
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

- Model runs capture role, endpoint/model/version, prompt/schema/policy version,
  redacted rendered prompt layers, bounded raw/structured response,
  parse/schema diagnostics, estimated/actual token use, Provider latency,
  timeout/cancel status, correlation identity, and optional Fluctlight scope.
- `metrics.prompt_budget` contains policy/context/max-input/output-reserve/safety
  values plus estimated input and section token/count maps for `system`,
  `runtime`, `recent`, `current_input`, `tools`, and `response_schema`.
  Provider usage is normalized to `prompt_tokens`, `completion_tokens`, and
  `total_tokens`; estimator delta is actual prompt minus estimated input.
- Assembler/Working Memory/Active/Long-term/Summary selection traces remain
  Core-only. Prompt diagnostic arrays are capped at 64 entries and recursively
  redacted before persistence. Ordinary ModelRuns API rows intentionally omit
  these metrics, raw scope, token, and latency fields.
- Hidden reasoning fields are discarded or reduced to an explicitly safe bounded summary; they are never stored as full reasoning.
- Typed redaction removes settings/API keys, cookies, sessions, service credentials, auth headers, object grants, `.env` values, and other secret types before persistence/stdout/export.
- Diagnostic writes are best-effort and never participate in the business Unit
  of Work. Lifecycle/metric updates use an independent bounded context. A sink
  failure cannot replace the business result, but it must increment the
  bounded health signal and emit a rate-limited structured operational warning;
  it is never silently discarded.
- Diagnostic sink errors cannot recursively emit into the same sink. The
  operational fallback retains stage, category, safe code, retryability, and
  correlation without copying raw payloads.
- BFF submits bounded batched diagnostics through service-auth internal ingestion; it never writes PostgreSQL directly.
- Current Go retention applies 30 days and 10,000 rows independently to
  diagnostic events, model runs, turns, and workflow links. The
  `diagnostic_retention` table is not a runtime override until a producer and
  consumer are implemented.
- Lifecycle cleanup enforces age and row limits. Domain audit/revision/evidence tables are excluded.
- Owner-only UI/API supports filter, live tail, correlation chain,
  prompt/response comparison, turn state transitions, workflow links, clear,
  and redacted export. The Lifecycle timeline reads transition-only events plus
  PostgreSQL workflow-intent snapshots and filters by Fluctlight, correlation,
  intent, workflow, Run, surface, and status even when Temporal is unavailable.
- Opening the Diagnostics UI must invoke its data loader. An empty local store is
  never evidence that PostgreSQL has no diagnostics.
- Events, model runs, and optional workflow-runtime status are independent read
  operations. A Temporal runtime failure may render a bounded workflow warning,
  but must not hide successfully loaded model prompts, responses, or events or
  misreport the error as an Owner authorization failure.
- Description analysis response provenance includes both the diagnostic
  correlation ID and a separate initialization `analysis_id`. The creation
  review retains them and can open a pre-filtered diagnostic view for that
  exact analysis, while activation uses `analysis_id` only as source authority.
- Initialization model-run rows are metadata-only by default: message count,
  estimated input tokens, prompt/response byte counts and digests, model,
  timing, status, safe error, coverage counts, finish reason, structured
  framing, candidate lengths/count, delimiter balance and JSON syntax offset.
  Original character-card
  text, complete structured response, field derivations, and accepted source
  projection are excluded from ordinary Diagnostics and exist only behind the
  Owner-authorized initialization-source detail boundary.
- Foundation validation failures expose a bounded structured detail object at
  the Core/BFF boundary, including `details.validation_error` and a safe error
  type. Clients must preserve this detail; a stable top-level code alone is not
  sufficient to diagnose missing or misrouted model fields.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Record contains typed secret/credential | Redact before persistence/stdout/export. |
| Model response contains hidden reasoning field | Drop/full-reasoning deny; retain only allowed structured output/bounded diagnostic. |
| Diagnostics PostgreSQL write fails | Preserve the business result; increment the bounded failure signal and emit one rate-limited structured operational warning. |
| Best-effort diagnostic insert/update fails repeatedly | Retain first/latest safe cause and occurrence count without recursively writing or flooding logs. |
| Metric JSON is not an object or token/latency value is negative | PostgreSQL rejects the diagnostic mutation; domain result remains unaffected. |
| BFF ingestion lacks service identity or exceeds batch/schema bounds | Reject ingestion without domain effect. |
| Retention cleanup fails | Record bounded stdout/error and retry lifecycle workflow; do not delete domain audit. |
| Non-Owner queries/exports/clears | Reject before returning diagnostic content. |
| Workflow runtime is unavailable while reading diagnostics | Keep loaded events/model runs visible; show a workflow-only unavailable state. |
| Expected active WakeUp passes due plus grace with no durable progress | Emit one transition-deduped `overdue` event with the stable cycle correlation. |
| Initialization diagnostics are queried/exported | Return metadata only; never include source text or the complete Provider response. |
| Initialization content is non-empty but parse fails | Persist/log only structural metadata and the typed parse category; never collapse it into semantic-empty or expose the candidate text. |
| Owner opens diagnostics from Settings | Invoke the same loader as a filter submission; do not only mutate the active view. |

### 5. Good / Base / Bad Cases

- Good: Owner opens one turn correlation view and sees redacted prompt layers, structured assessment, policy deltas, frozen action, realization, workflow attempts, and final message.
- Base: diagnostics database insert fails during a successful chat; chat
  succeeds and one rate-bounded structured warning appears on stdout/health.
- Bad: require Grafana to inspect a local prompt, log API keys, put diagnostic rows in the business transaction, or delete relationship revisions during retention cleanup.

### 6. Tests Required

- Typed-redaction tests with credentials in nested request/response/header/URL/config objects and exported bundles.
- Model-run tests for prompt sections, bounded/redacted selection traces,
  parse errors, provenance, hidden-reasoning drop, estimated/actual token
  normalization, estimator delta, latency, and collection cap.
- Sink tests for database failure, independent diagnostic context,
  rate-bounded operational warning, non-recursion, and no business rollback.
- BFF ingestion tests for service auth, schema/batch limits, correlation fields, and no direct database access.
- Retention tests for age/row dual limits and explicit proof that domain audit/revision/evidence remains.
- Owner authorization tests for query/tail/export/clear and no diagnostic access through ordinary product DTOs.
- Lifecycle API/UI tests filter by Fluctlight/correlation/intent/workflow/Run/
  surface/status, retain PostgreSQL snapshots during Temporal failure, render
  no-op/retry/failure/overdue distinctly, and traverse a complete correlation.
- Initialization tests assert ordinary rows/export contain metadata/digests but
  not source text, full response, structured projection, or derivation evidence;
  truncated/invalid candidates retain finish/framing/length/balance/offset.

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

- Diagnostic writes are best-effort and never fail the domain operation, but a
  failed write produces a rate-bounded operational warning/health signal with
  component, stage, correlation ID, error type, and a 512-rune secret-redacted
  `safe_cause`. Logging only the Go error type is not sufficient.
- Recursive redaction removes credentials, cookies, API keys and hidden
  reasoning before persistence or export.
- Owner authorization and correlation/fluctlight filters apply to reads.
- Periodic retention deletes only diagnostic tables, never domain audit or
  revision/evidence rows.
- Prompt diagnostics include final wire section counts/tokens, output reserve,
  actual Provider usage/latency, estimator delta, bounded selected/dropped refs,
  ranking reasons/components, and optional continuation phase. Credentials,
  raw prompt/response keys, reasoning/perception/appraisal, image data, and
  Core-only raw IDs are redacted or omitted.
- The ordinary ModelRuns API keeps its existing small projection and does not
  expose the new prompt metrics or Fluctlight scope. Clear/prune still includes
  `diagnostic_model_runs` as operational data.
- A Provider attempt writes one coherent queued→running→terminal model-run row
  plus `provider_provenance`. PostgreSQL parameters reused by both a target
  column and subquery predicate have an explicit cast (for example
  `$2::varchar(64)` for binding role), so type inference cannot roll back the
  complete diagnostic transaction.
- Provider transport failures persist bounded tokens such as
  `request_timeout`, `request_cancelled`, or `provider_request_failed`; a URL,
  network-library sentence, response body, or arbitrary `err.Error()` never
  occupies the `error_code` column.
- The first terminal model-run state is authoritative. A late callback with a
  different terminal classification is an idempotent no-op that preserves the
  first status/error and counts as a successful state observation. A missing
  row or a terminal-to-running regression remains `state_not_written` and emits
  a bounded warning.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| non-Owner diagnostic read/clear | forbidden before content is returned |
| malformed correlation/filter or negative limit | bounded default/validation error |
| diagnostic sink unavailable | business result remains successful; bounded operational warning/health signal changes |
| provider provenance SQL cannot infer a reused parameter type | use one explicit PostgreSQL cast; do not accept an empty Model Runs timeline |
| Provider failure writes `failed`, then queue classifies the same error as `timeout` | preserve the first terminal row and treat the late terminal callback as an idempotent no-op; do not emit `diagnostic_model_run_state_not_written` |
| model-run ID is genuinely absent during a state update | emit the bounded `state_not_written` warning with ID and safe cause |
| retention cleanup fails | bounded Worker warning and retry |
| prompt selection trace contains more than 64 array entries | persist only the first 64 after recursive redaction |
| Provider returns usage fields outside the allowlist | discard unknown usage fields |

### 5. Good/Base/Bad Cases

- Good: a Provider failure is visible as a redacted failed model run while the
  chat error remains bounded.
- Base: clearing diagnostics returns the number of deleted records.
- Bad: persisting raw authorization headers or deleting relationship history
  during retention.

### 6. Tests Required

- Recursive redaction, Owner isolation, filters, clear counts and age/row
  retention tests against PostgreSQL.
- Provider success/failure producer tests with sink failure injection and
  bounded operational-warning assertions, including redacted `safe_cause`.
- PostgreSQL/Compose test asserts a live WakeUp or Reflection attempt persists
  model run and provenance with the same correlation and attempt identity.
- PostgreSQL regression writes one terminal row, delivers a different late
  terminal callback, and asserts one unchanged row plus no persistence warning;
  a separate test asserts Provider timeout creates exactly one
  `timeout/request_timeout` row.
- Provider wire-budget tests assert Tools/schema are counted separately,
  `max_tokens` remains output reserve, usage/latency/delta are normalized, and
  ordinary product APIs cannot read prompt metrics.

### 7. Wrong vs Correct

#### Wrong

```go
INSERT INTO diagnostic_model_runs(prompt) VALUES ($1) // raw request
```

#### Correct

```go
recordModelRun(redactDiagnostic(prompt), boundedResponse, correlationID)
```

```go
// Reused role parameter has one explicit PostgreSQL type in every context.
WHERE role=$2::varchar(64)
```

## Scenario: Per-call Eino/ADK model-run diagnostics

### 1. Scope / Trigger

- Trigger: one request-scoped ADK conversation makes multiple physical Eino
  ChatModel calls around tool execution.

### 2. Signatures

```go
recordQueuedModelRun(ctx, role, endpointID, modelID, correlationID, scenario, priority, prompt) string
updateModelRunPromptMetrics(ctx, modelRunID, usage, latency)
```

### 3. Contracts

- The parent turn correlation may be shared, but each physical ChatModel
  Generate/Stream call creates its own queued→running→terminal
  `diagnostic_model_runs` row, attempt identity and Provider request ID.
- Prompt/response/arguments remain recursively redacted and bounded. Tool-call
  IDs may be retained only as bounded identity metadata; raw arguments and
  hidden reasoning do not enter ordinary diagnostics.
- Queue cancellation, timeout, tool failure and iteration-limit errors update
  the affected row with a stable bounded error code without changing the
  business settlement result.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Two ADK model generations | Two model-run rows with distinct attempt/request IDs and one parent correlation |
| First model call fails | First row terminalizes; no second call or fabricated final row |
| Diagnostics sink unavailable | Core result remains governed by domain outcome; bounded persistence warning only |
| Raw prompt/reasoning/tool args supplied | Redact/drop before persistence |

### 5. Good/Base/Bad Cases

- Good: rows show `conversation` parent correlation, `adk:1`/`adk:2`
  attempts, per-call usage/latency and bounded terminal state.
- Base: a one-call text task produces one row with the existing role/scenario.
- Bad: update one shared row twice so the first HTTP call appears to have the
  second call's usage or terminal status.

### 6. Tests Required

- Assert two ADK calls create two rows, distinct request IDs, bounded usage and
  one shared parent correlation.
- Assert cancellation/timeout terminalization and queue release per call.
- Assert redaction of image data, credentials, raw arguments and reasoning in
  both rows.

### 7. Wrong vs Correct

#### Wrong

```go
diagnosticID := recordQueuedModelRun(parentCorrelation)
runner.Run(ctx) // all Generate calls reuse one row
```

#### Correct

```go
// Each queuedToolCallingChatModel.Generate creates its own row.
callID := recordQueuedModelRun(callCorrelation)
runProviderQueued(ctx, callID, generateOneModelCall)
```

## Scenario: ADK Generation-Level Diagnostics

### 1. Scope / Trigger

- Trigger: an ADK structured task performs a model generation or executes a
  registered capability.

### 2. Contracts

- Diagnostics retain the logical WakeUp/Conversation correlation and record
  each physical Provider generation with its own attempt/request identity,
  queue lifecycle, timing and normalized usage when the diagnostic store is
  available.
- Tool-call IDs, result pairing, surface and bounded invocation/result status
  are safe structured trace data. Hidden reasoning, credentials, raw prompts,
  complete provider responses and unbounded arguments remain redacted or
  omitted.
- Diagnostic failure is best-effort: it cannot turn a committed domain result
  into a failure, and it cannot authorize a capability or publish an ADK
  intermediate message.

### 3. Tests Required

- Assert two ADK generations retain distinct request/attempt identities while
  sharing one logical correlation, and that failed/cancelled generations have
  terminal diagnostics without fabricated success.
