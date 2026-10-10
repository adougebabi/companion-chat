# Fluctlight Diagnostics Contract

## Scenario: Built-In Local Debugging Without External Telemetry Stack

### 1. Scope / Trigger

- Trigger: browser boundary/Core/Worker emits a diagnostic event, a model call runs, a cognitive turn/workflow/event/media chain needs inspection, or Owner queries/exports/cleans diagnostics.
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
export(command: ExportDiagnostics) -> OwnerDiagnosticBundle
clear(command: ClearDiagnostics) -> ClearResult
```

Correlation fields include source, level/code, Fluctlight/Actor/Conversation/turn/workflow/step/event/outbox/inbox/media IDs, correlation/causation IDs, timestamps, bounded context, and outcome.

### 3. Contracts

- Model runs capture role, endpoint/model/version, prompt/schema/policy version,
  rendered prompt layers with image payloads replaced, raw/structured response,
  parse/schema diagnostics, estimated/actual token use, Provider latency,
  timeout/cancel status, correlation identity, and optional Fluctlight scope.
- `metrics.prompt_budget` contains policy/context/max-input/output-reserve/safety
  values plus estimated input and section token/count maps for `system`,
  `runtime`, `recent`, `current_input`, `tools`, and `response_schema`.
  Provider usage is normalized to `prompt_tokens`, `completion_tokens`, and
  `total_tokens`; estimator delta is actual prompt minus estimated input.
- Assembler/Working Memory/Active/Long-term/Summary selection traces remain
  Core-only. Prompt diagnostic arrays are capped at 64 entries; retained text
  and source refs keep their original values, while image payloads are replaced.
  Ordinary ModelRuns API rows intentionally omit
  these metrics, raw scope, token, and latency fields.
- Owner-only diagnostics persist available non-image Prompt, Response, Tool arguments,
  reasoning and other text verbatim, including credential-bearing values. Image
  data URLs and binary image payloads are replaced with `REDACTED_IMAGE_DATA`
  before persistence and export. Operational stdout warnings remain separately
  credential-redacted; they are not the Owner diagnostic store.
- Diagnostic writes are best-effort and never participate in the business Unit
  of Work. Lifecycle/metric updates use an independent bounded context. A sink
  failure cannot replace the business result, but it must increment the
  bounded health signal and emit a rate-limited structured operational warning;
  it is never silently discarded.
- Diagnostic sink errors cannot recursively emit into the same sink. The
  operational fallback retains stage, category, safe code, retryability, and
  correlation without copying raw payloads.
- browser boundary submits bounded batched diagnostics through service-auth internal ingestion; it never writes PostgreSQL directly.
- Current Go retention applies 30 days and 10,000 rows independently to
  diagnostic events, model runs, turns, and workflow links. The
  `diagnostic_retention` table is not a runtime override until a producer and
  consumer are implemented.
- Lifecycle cleanup enforces age and row limits. Domain audit/revision/evidence tables are excluded.
- Owner-only UI/API supports filter, live tail, correlation chain,
  prompt/response comparison, turn state transitions, workflow links, clear,
  and image-sanitized export. The Lifecycle timeline reads transition-only events plus
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
- Initialization model-run rows follow the same Owner-only non-image original-text
  policy as other scenarios. The separate initialization-source detail boundary
  still supplies accepted source authority and derivation data; it does not
  suppress the actual Provider Prompt/Response from new diagnostic rows.
- Foundation validation failures expose a bounded structured detail object at
  the Core/browser boundary, including `details.validation_error` and a safe error
  type. Clients must preserve this detail; a stable top-level code alone is not
  sufficient to diagnose missing or misrouted model fields.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Owner diagnostic contains typed secret/credential or available reasoning | Persist and return its original non-image text only through Owner-authorized diagnostics; keep operational stdout separately credential-redacted. |
| Diagnostic contains image data URL or binary image | Replace payload with `REDACTED_IMAGE_DATA` before persistence and export. |
| Diagnostics PostgreSQL write fails | Preserve the business result; increment the bounded failure signal and emit one rate-limited structured operational warning. |
| Best-effort diagnostic insert/update fails repeatedly | Retain first/latest safe cause and occurrence count without recursively writing or flooding logs. |
| Metric JSON is not an object or token/latency value is negative | PostgreSQL rejects the diagnostic mutation; domain result remains unaffected. |
| browser boundary ingestion lacks service identity or exceeds batch/schema bounds | Reject ingestion without domain effect. |
| Retention cleanup fails | Record bounded stdout/error and retry lifecycle workflow; do not delete domain audit. |
| Non-Owner queries/exports/clears | Reject before returning diagnostic content. |
| Workflow runtime is unavailable while reading diagnostics | Keep loaded events/model runs visible; show a workflow-only unavailable state. |
| Expected active WakeUp passes due plus grace with no durable progress | Emit one transition-deduped `overdue` event with the stable cycle correlation. |
| Initialization diagnostics are queried/exported | Return the same image-sanitized original Provider Prompt/Response as other Owner model runs. |
| Initialization content is non-empty but parse fails | Persist the original non-image candidate text plus typed parse category in Owner diagnostics; operational stdout remains credential-redacted. |
| Owner opens diagnostics from Settings | Invoke the same loader as a filter submission; do not only mutate the active view. |

### 5. Good / Base / Bad Cases

- Good: Owner opens one turn correlation view and sees original non-image prompt/response text, structured assessment, policy deltas, frozen action, realization, workflow attempts, and final message.
- Base: diagnostics database insert fails during a successful chat; chat
  succeeds and one rate-bounded structured warning appears on stdout/health.
- Bad: require Grafana to inspect a local prompt, log API keys, put diagnostic rows in the business transaction, or delete relationship revisions during retention cleanup.

### 6. Tests Required

- Owner-only diagnostic tests retain nested non-image text and credentials, replace
  image data in writes/reads/exports, and reject non-Owner queries.
- Model-run tests for prompt sections, bounded selection traces,
  parse errors, provenance, available reasoning, estimated/actual token
  normalization, estimator delta, latency, and collection cap.
- Sink tests for database failure, independent diagnostic context,
  rate-bounded operational warning, non-recursion, and no business rollback.
- browser boundary ingestion tests for service auth, schema/batch limits, correlation fields, and no direct database access.
- Retention tests for age/row dual limits and explicit proof that domain audit/revision/evidence remains.
- Owner authorization tests for query/tail/export/clear and no diagnostic access through ordinary product DTOs.
- Lifecycle API/UI tests filter by Fluctlight/correlation/intent/workflow/Run/
  surface/status, retain PostgreSQL snapshots during Temporal failure, render
  no-op/retry/failure/overdue distinctly, and traverse a complete correlation.
- Initialization tests assert physical and outer model-run rows consistently retain
  original non-image Prompt/Response; invalid candidates retain typed parse details.

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
diagnostics.emit(image_payload_filter.model_run(model_run_record))
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
- Owner diagnostic persistence/export replaces image payloads only; it retains
  available credentials, cookies, API keys and reasoning as non-image text.
- Owner authorization and correlation/fluctlight filters apply to reads.
- Periodic retention deletes only diagnostic tables, never domain audit or
  revision/evidence rows.
- Prompt diagnostics include final wire section counts/tokens, output reserve,
  actual Provider usage/latency, estimator delta, bounded selected/dropped refs,
  ranking reasons/components, and optional continuation phase. Retained
  non-image values, including `source_refs`, are unchanged; image data is replaced.
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
| prompt selection trace contains more than 64 array entries | persist only the first 64, preserving their non-image values |
| Provider returns usage fields outside the allowlist | discard unknown usage fields |

### 5. Good/Base/Bad Cases

- Good: a Provider failure is visible as an image-sanitized failed model run while the
  chat error remains bounded.
- Base: clearing diagnostics returns the number of deleted records.
- Bad: exposing Owner diagnostic content to a non-Owner or deleting relationship
  history during retention.

### 6. Tests Required

- Image-data replacement, Owner isolation, filters, clear counts and age/row
  retention tests against PostgreSQL.
- Provider success/failure producer tests with sink failure injection and
  bounded operational-warning assertions, including credential-redacted stdout `safe_cause`.
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
INSERT INTO diagnostic_model_runs(prompt) VALUES ($1) // raw request with image bytes
```

#### Correct

```go
recordModelRun(redactDiagnostic(prompt), originalNonImageResponse, correlationID)
```

```go
// Reused role parameter has one explicit PostgreSQL type in every context.
WHERE role=$2::varchar(64)
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
- Tool-call IDs, result pairing, surface and invocation/result status remain
  structured trace data. Owner diagnostics retain available non-image reasoning,
  credentials, Prompt/Response and Tool arguments; images are replaced before storage.
- Diagnostic failure is best-effort: it cannot turn a committed domain result
  into a failure, and it cannot authorize a capability or publish an ADK
  intermediate message.

### 3. Tests Required

- Assert two ADK generations retain distinct request/attempt identities while
  sharing one logical correlation, and that failed/cancelled generations have
  terminal diagnostics without fabricated success.

## Scenario: Owner Logical Agent Failure Diagnosis

### 1. Scope / Trigger

- Trigger: a physical model call returns a ToolCall, then Tool execution or final Agent settlement fails, including a later replay reporting `agent_run_failed` without a new model call.
- `agent_runs` remains the durable replay fence and authority for the formal Agent model/Tool loop; caller-owned final-contract, publication and settlement may fail after that loop completed. `diagnostic_model_runs` remains the independent physical Provider-call record.

### 2. Signatures

```text
agent_runs += correlation_id varchar(128), failure_stage varchar(64), failure_code varchar(128)
App.AgentRunsFiltered(actorID, limit, correlationID) -> []AgentRunDiagnostic
GET /internal/diagnostics/agent-runs?limit=&correlation_id=
GET /api/diagnostics/agent-runs?limit=&correlationId=
```

Core response fields are `fluctlight_id`, `agent_id`, `run_id`, `correlation_id`, `association_status`, `status`, `started_at`, optional `finished_at`, and for failed runs `failure_stage`, `failure_code`, optional `safe_cause`. The browser maps each field explicitly to camelCase and exposes no raw `error_detail`, input digest, result JSON or Tool arguments.

### 3. Contracts

- Admission stores the Provider parent correlation alongside the stable business run ID. An old row without correlation reports `association_status=unknown`; it is not joined to a model call by timestamp or guessed to have a final response. A failure before formal admission has no `agent_runs` row, so the Owner read may use a bounded `agent.run.termination` event as a separately labelled `source=termination_event` fallback. A completed formal row does not suppress a later final-contract/publication/settlement failure event. Only a matching failed formal row suppresses its duplicate failed termination event, using DB-wide existence rather than the current page of rows.
- Tool execution failures carry typed stage/code through ADK wrapping into `finishFormalRun`. Cancellation, timeout, input-budget and Provider suppression/failure use their typed errors. An unknown failure stays `stage=agent`, `code=agent_run_failed`; never parse arbitrary `err.Error()` text into a stage.
- A non-retryable `invalid_arguments` Tool result is a recoverable business rejection returned to the next model decision, not an automatic Agent failure. Its Tool summary still displays `invalid_arguments`; if the Agent later fails its final contract, the termination event displays that later failure as a separate state.
- Owner reads render `error_detail` through the bounded cause field without changing its non-image text. The physical model row keeps its own `completed/failed/timeout` status even if the logical Agent failed later. `agent_runs` is not deleted by Diagnostics clear/retention.
- A Tool event's `model_call_id` comes from the `ADKCapabilityTrace.ModelIdentity(callID)` recorded at the physical Generate/Stream boundary. The callback's parent `context.Context` need not inherit a child Generate context; never join a Tool to the newest model row by time.
- Best-effort lifecycle failure diagnostics use an independent bounded context after cancellation. Diagnostic sink failure still cannot replace the business result and must keep its bounded warning/health signal.
- The Owner Model Runs page groups physical rows and logical Agent rows by known correlation, displays distinct status badges and a safe failure stage/cause, and labels missing old associations as unknown. Termination-event fallback uses its event timestamp as record time, not an invented Agent start time. Public chat remains unchanged.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Non-Owner queries logical run | Reject before reading `agent_runs`. |
| Correlation filter malformed or over 128 characters | `diagnostics_filter_invalid`. |
| Model call completed, Tool/Agent failed | Keep physical `completed`; display logical `failed`, typed stage/code and bounded cause. |
| Tool `invalid_arguments` is corrected in a later model round | Keep Tool rejection visible and mark the formal Agent completed only after its final contract succeeds; do not relabel the rejected Tool as an Agent crash. |
| Old Agent row has no correlation/stage | Show `association_status=unknown`, stage `unknown`; do not invent a model round. |
| Conversation fails before formal Agent admission | Persist a bounded `agent.run.termination` cause and show the event fallback; no physical model row is invented. |
| Error detail contains a credential-bearing phrase | Return the original bounded cause to the Owner; keep the separate stdout warning credential-redacted. |
| Caller context is cancelled during lifecycle failure write | Attempt persistence with detached five-second context; a sink failure remains best-effort. |

### 5. Good/Base/Bad Cases

- Good: one Tool-request model call is `completed`; its Tool summary reports `capability_prepare_failed`; the same correlation shows logical Agent `failed` with `stage=tool` and safe cause.
- Base: a pre-upgrade failed Agent row appears as a standalone logical record with unknown model association.
- Bad: turn the successful physical model row red, hide the logical failure because no new Provider request ran on replay, or expose raw diagnostic text outside the Owner boundary.

### 6. Tests Required

- PostgreSQL migration from `0040` retains `agent_runs` and adds the three columns/indexes; old rows remain readable.
- Real trace-recorded Tool callback attaches its safe summary to the correct `model_call_id` in a two-round run; manually inserted event IDs alone do not prove this.
- Owner query checks logical failed + physical completed under the same correlation, pre-admission event fallback, duplicate event suppression, legacy unknown, non-Owner rejection, bounded original non-image cause and cancelled-context lifecycle persistence.
- Browser DTO/OpenAPI/generated client and page tests check explicit mapping, grouped status, missing-response/old-row states, and no public chat trace insertion.

### 7. Wrong vs Correct

Wrong: `agentFailed := modelRun.Status == "failed"` or `tool.model_call_id = latestModelRun(correlation)`.

Correct: read `agent_runs.status` for logical outcome, keep `diagnostic_model_runs.status` per physical call, and bind Tool events to the trace-recorded physical request identity.

## Scenario: Owner ADK Physical Round Display

### 1. Scope / Trigger

- Trigger: one ADK logical run makes several physical model calls around Tool requests and results; the Owner opens Model Runs.

### 2. Signatures

```text
ModelRunsFiltered row += logical_run_id, model_call_id, sequence?, round_count,
  stage: tool_request | final_response | failed | cancelled | timeout | pending | unknown,
  tool_summaries: [{call_id, capability, status, error_code?}] // max 32
```

### 3. Contracts

- Each physical `diagnostic_model_runs` row keeps its own Prompt/Response. `metrics.run_id` groups rows and `metrics.model_call_id` joins bounded `adk.model.input/output` and `adk.tool.*` events. The Core reader derives sequence and stage; the browser only groups and sorts. A completed output with ToolCall IDs is an intermediate request; only a completed output with zero ToolCalls is a final model answer.
- ADK rows use their recorded model-call events for Tool/final stage. New non-ADK rows with explicit Provider attempt identity derive a stable physical sequence from queued-at/id and status-derived stage; genuinely old rows without these markers remain `unknown`. A page cut within a logical run reports its full round count.
- ADK `queuedToolCallingChatModel` writes one row per actual physical request. The outer Agent completion must not write a second model-run summary. Owner reads exclude historical no-`model_call_id` outer summary rows when the same logical run has ADK physical rows; preflight failure rows without a Provider request remain diagnosable but do not receive a fabricated physical sequence.
- Tool summaries expose call identity, capability, status and error code; the correlated Owner event retains original arguments. Model Prompt/Response retain available non-image `reasoning_content` and `tool_calls[].function.arguments`, while image data is replaced on write and read. Diagnostics timestamps use the Owner's current viewing IANA zone and show its name; chat messages keep their own sender-time provenance.
- Public chat NDJSON continues to emit only committed visible messages, never physical intermediate responses.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Output contains ToolCall IDs | Stage `tool_request`, not final reply. |
| Physical row is failed/cancelled/timeout or lacks Response | Display its actual status and no copied Response. |
| Old row has no model-call event | Stage/sequence `unknown`. |
| Tool payload contains arguments/reasoning | Retain original non-image values in Owner diagnostics; summary may remain compact. |

### 5. Good/Base/Bad Cases

- Good: a two-call run displays “第 1/2 次 · 请求 Tool” then its safe Tool result, followed by “第 2/2 次 · 最终模型回答.”
- Base: a genuinely old row says “轮次未知”; a new non-ADK row has a sequence and status-derived stage.
- Bad: flattening both rows under identical “认知判断” titles or dropping available multimodal text from the physical Prompt.

### 6. Tests Required

- PostgreSQL: two physical rows and Tool events preserve distinct responses, sequence/stage, bounded safe summary, and full group count; failure and old-row cases do not fabricate final output.
- BFF: explicit snake_case↔camelCase mapping; raw Tool arguments remain in the Owner event payload, not public chat. Browser: group ordering, missing Response, time-zone labels and no public-chat trace insertion.

### 7. Wrong vs Correct

Wrong: treat every `status='completed'` model run as the final answer.
Correct: derive `tool_request` from that call's output ToolCall IDs and `final_response` only from its completed no-Tool output event.

## Scenario: ADK Run/Tool/Input Correlation Export

### 1. Scope / Trigger

- Trigger: a Conversation, WakeUp or Takeover ADK run needs to explain whether
  a model requested a Tool, whether it was authorized/dispatched, and whether
  the next physical model input actually contained the matching result.

### 2. Signatures

```text
adk.model.input
adk.model.output
adk.tool.requested / adk.tool.rejected / adk.tool.dispatched / adk.tool.result
adk.run.termination
DiagnosticsExportFiltered(filter.RunID)
```

### 3. Contracts

- All events share the parent correlation and a bounded `run_id`; physical
  model runs retain distinct attempt/request/model-run identities.
- `adk.model.input` records message count, formal tool-result IDs and matched
  assistant/tool pair count. “Model received the result” is true only when
  this actual next-input event contains the pair; direct return is `n/a`.
- Tool events retain call ID, capability, surface, status, error code, original
  non-image arguments and an arguments digest. Physical model rows retain
  Prompt/Response and available reasoning under Owner authorization.
- Diagnostic writes are best-effort and do not authorize, settle or publish a
  Capability. Owner-authorized export may filter ordinary events and model
  runs by `run_id` while keeping the small ordinary ModelRuns projection.

### 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Unknown tool fails before `InvokableRun` | model output/termination remains correlated; no dispatched event is fabricated |
| Tool result is not in next model input | pair count is zero; trace reports the missing-result stage |
| Diagnostic sink is unavailable | domain result remains unchanged; bounded persistence warning only |
| Export uses a non-owner or unsafe run filter | reject before returning diagnostic content |

### 5. Good/Base/Bad Cases

- Good: one run shows two physical model calls, one formal call ID, a matching
  next-input pair, typed tool result and final termination.
- Base: a legal direct return records `tool_result_pair_status=n/a` because
  there is no next model call.
- Bad: infer “model received result” from a successful tool log, expose the
  arguments outside Owner diagnostics, or reuse one model-run row for two calls.

### 6. Tests Required

- Assert image-sanitized event payloads, separate model input/output/termination
  events, run ID filtering, owner authorization and no diagnostic write in the
  business transaction.
- Use controlled Conversation/WakeUp success, Tool failure and model parse
  failure fixtures when no real incident run is available.

### 7. Wrong vs Correct

#### Wrong

```go
receivedByModel := toolResultLoggedSuccessfully
```

#### Correct

```go
receivedByModel := nextInput.MatchedAssistantToolPairCount > 0
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
- Prompt/response/arguments retain original non-image text. Tool-call IDs remain
  bounded identity metadata; available reasoning is retained in Owner diagnostics.
- Queue cancellation, timeout, tool failure and iteration-limit errors update
  the affected row with a stable bounded error code without changing the
  business settlement result.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Two ADK model generations | Two model-run rows with distinct attempt/request IDs and one parent correlation |
| First model call fails | First row terminalizes; no second call or fabricated final row |
| Diagnostics sink unavailable | Core result remains governed by domain outcome; bounded persistence warning only |
| Raw prompt/reasoning/tool args supplied | Persist original non-image values for Owner reads; replace image data before storage. |

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
- Assert image data is replaced and non-image credentials, arguments and
  available reasoning survive in both rows under Owner authorization.

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
  remain structured trace data. Owner diagnostics retain available non-image
  reasoning, credentials, Prompt/Response and Tool arguments; images are replaced.
- Diagnostic failure is best-effort: it cannot turn a committed domain result
  into a failure, and it cannot authorize a capability or publish an ADK
  intermediate message.

### 3. Tests Required

- Assert two ADK generations retain distinct request/attempt identities while
  sharing one logical correlation, and that failed/cancelled generations have
  terminal diagnostics without fabricated success.


## Scenario: Post-run business failure and current-facts mismatch

### 1. Scope / Trigger
A finished formal model run may fail later at publication or cognition settlement. Classifying only its inner cause previously collapsed known business boundaries to agent_run_failed.

### 2. Signatures
`classifyAgentPostRunFailure(stage,code,cause)` enriches generic cause classification with caller-owned boundaries. `currentFactsMismatch{Expected,Actual,Boundary}` unwraps ErrCurrentFactsStale and retains Error()=current_facts_stale.

### 3. Contracts
Replace only generic agent_run_failed/agent classification; typed tool/provider/cancellation/timeout classifications remain specific. Keep the inner safe_cause. Publication uses agent_output_publication_failed/output_publication, settlement uses agent_cognition_settlement_failed/settlement. Facts conflicts add expected_current_facts_revision, actual_current_facts_revision and authority_boundary=settlement|tool_receipt to the Owner diagnostic and readable safe_cause. This does not loosen source/version checks or replay effects. Model/formal-run success is distinct from post-run business outcome.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| missing reply under publication boundary | business failure code + inner cognition_visible_text_missing |
| facts mismatch under settlement boundary | business settlement code + actual/expected facts revisions |
| typed provider/tool failure | preserve specific classified code |
| parent cancelled | preserve request_cancelled |

### 5. Good / Base / Bad Cases
Good: diagnose phase and cause independently. Base: normal run remains completed. Bad: label a publication failure as generic agent_run_failed, or suppress current_facts_stale to hide conflicts.

### 6. Tests Required
TestPostRunFailureRetainsOuterBoundaryAndSpecificCauses covers publication/settlement/provider/cancel. TestCurrentFactsMismatchPreservesStaleContract checks sentinel/label compatibility. PostgreSQL TestPublicationFailureDiagnosticRetainsBusinessCode validates persisted payload (SKIP without isolated DB).

### 7. Wrong vs Correct
Wrong: classify only causes[0] and discard the code already known to the caller.
Correct: preserve the business boundary on generic fallback and retain the specific cause with bounded diagnostics.
