# Exact task-scoped contract excerpts

Source: `.trellis/spec/backend/fluctlight-diagnostics-contract.md`

Source SHA-256: `f3fb96ade67c9b2e49ca744e965cefad59365501df6dcea3f60c57b20d4c2bc8`

Verbatim selected complete scenarios. Original spec remains authoritative; later credential-redaction sections supersede earlier historical credential-retention wording. Regenerate if source changes.

<!-- Original lines 3–158 -->

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


<!-- Original lines 395–533 -->

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


<!-- Original lines 591–730 -->

## Scenario: Physical termination metadata and bounded SSE observation (2026-10-11)

### 1. Scope / Trigger
Reasoning-only replies and cut JSON require actual Provider termination evidence; per-delta updates previously ignored finish/usage-only frames.

### 2. Signatures
`einoMessageRaw` adds available finish_reason/usage to the owning model-run response. `adk.model.output.response_metadata` and empty `adk.run.termination.response_metadata` contain counts, finish reason and usage only. `einoStreamObservation` observes the existing queue reader and flushes exactly once at EOF/error/cancel.

### 3. Contracts
Retain prompt/completion/total tokens, and available cached/reasoning detail. Missing optional breakdowns remain absent/unknown; do not report invented zero or infer exhaustion. Termination events do not duplicate raw reasoning; raw responses remain with the Owner model-run diagnostic.

Observation retains at most2MiB including chunk/Tool structure and terminal reserve; arbitrary ToolCall.Extra is omitted from the diagnostic projection, finish text capped256bytes, overflow marked diagnostic_output_truncated. It never changes native chunks, arguments, IDs or errors. Merge terminal metadata with accumulated content rather than replacing output with an empty terminal frame.

Pinned ACL openai0.1.17 may drop reasoning detail before exposing a separate usage-only frame after a finish-only frame. Total/cache/finish remain available; combined terminal detail is preserved. Do not add a second SSE parser or claim that an absent detail was observed.

### 4. Validation & Error Matrix
| Condition | Diagnostic |
| --- | --- |
| Empty content/Tools plus reasoning | model_output/adk_final_message_empty with real reason/counts/usage |
| finish_reason=length | record length; do not invent business success or auto-retry |
| Missing reason | unknown, not inferred stop/length |
| Metadata/content retention overflow | bounded output and explicit truncation flag |
| Provider507 | preserve actual provider_request_failed cause, separate from CAS/output errors |

### 5. Good / Base / Bad Cases
Good: finish-only+usage-only SSE preserves total/cache/reason alongside earlier content. Base: Generate metadata is stored directly. Bad: overwrite full response with usage-only empty delta or duplicate reasoning in termination events.

### 6. Tests Required
`eino_stream_observation_test.go` covers combined/split HTTP SSE, bounds/overhead, terminal events and typed errors; `runtime_recovery_test.go` covers actual repair requests and effect replay isolation. Service-backed tests remain opt-in; local pass is not real model acceptance.

### 7. Wrong vs Correct
Wrong: assume193 completion tokens exhausted a40960 budget without finish reason. Correct: preserve reported termination metadata, label uncertainty and distinguish actual model service errors.


## Scenario: Preserve malformed Provider HTTP errors before Eino conversion (2026-10-12)

### 1. Scope / Trigger
Pinned model/openai v0.1.13 converts an enclosing SDK RequestError by selecting
its inner APIError. A malformed error object (missing message or object message)
can leave that inner object empty, losing the actual HTTP status/body. This
reproduces blank provider_request_failed for Generate and initial Stream.

### 2. Signatures
`einoHeaderRoundTripper.RoundTrip` normalizes HTTP status>=400 using
`preserveProviderHTTPError(request,response) error`.
`providerHTTPResponseError{StatusCode,Detail,Truncated,readErr}` retains actual
status and a bounded sanitized error excerpt; Unwrap preserves body-read cause.

### 3. Contracts
Intercept HTTP failure before SDK conversion; return nil response and a Core
error independent of SDK APIError. Read at most4097bytes, display at most4096,
mark truncation and close the original error body once. Empty body still reports
actual HTTP status. Redact configured credential header values, both raw and
JSON-escaped full/partial echoes, recognized credential fields and image data.
Do not inspect or persist request bodies/headers, infer status, fabricate content,
retry or execute tools. Pass successful and 3xx responses unchanged so http.Client
retains CheckRedirect behavior. Network/cancel errors remain original, timeout
and request lifetime unchanged. The failure code stays provider_request_failed;
HTTP/body belongs in bounded cause, never in error_code.

### 4. Validation & Error Matrix
| Condition | Outcome |
| --- | --- |
| HTTP400 {"error":{}} or invalid message type | real HTTP400 and bounded payload survive Eino/ADK |
| HTTP400 empty body | actual HTTP400 plus empty-body explanation |
| HTTP507 ordinary error | preserve actual status and sanitized reason |
| oversized/error-body read failure | bounded excerpt/truncation; typed read cause |
| successful JSON/SSE | original response/body/stream untouched |
| redirect | existing following/CheckRedirect policy retained |
| partial escaped secret at read bound | no credential fragment disclosed |

### 5. Good/Base/Bad Cases
Good: malformed error stays an explicit Provider failure with HTTP status.
Base: existing success/native Tool path is unchanged. Bad: fill an empty error
with guessed token/GPU cause or capture unbounded error body.

### 6. Tests Required
TestEinoProviderHTTPError* exercises pinned SDK/Eino/queued/ADK Generate/Stream,
one physical request and no fabricated final; status/body/typed cause/sentinel,
body bounds/close/read error, cancellation, successful JSON/SSE, redirect policy,
raw/escaped/partial credential redaction. Production actual error remains unknown
until captured; controlled HTTP400 is not evidence of production HTTP400.

### 7. Wrong vs Correct
Wrong: errors.As selects an empty nested APIError and discards RequestError.
Correct: normalize the real failed HTTP response at the existing transport
boundary, before the lossy dependency conversion, while preserving normal HTTP.

## Successful Provider credential echoes (2026-10-13)
Secret-key fields (api_key, Authorization, passwords, tokens, cookies and complete assignment objects) are redacted in all diagnostics. Physical and non-ADK model calls carry their known credential only in a private runtime context; prompts, responses, output events and JSON-encoded content scrub exact credential echoes before persistence. Successful responses require the same privacy boundary as failures. Raw Owner diagnostic content remains useful for ordinary text/tool results, while image payloads and credentials are removed. This scrub never mutates the Provider result used for domain parsing. A Writer response containing its actual credential is rejected before publication.

## Scenario: Effective physical request configuration (2026-10-14)

### 1. Scope / Trigger
Each queued Eino Generate/Stream needs to distinguish effective request policy
from the logical Composer's schema/budget and from Provider usage.

### 2. Signatures
`observePhysicalRequest(ctx, body)` emits `adk.model.effective_request` through the
existing best-effort diagnostic owner. `physicalRequestConfiguration` allowlists
serialized parameters; `physicalRequestModuleEstimates` labels logical estimates.

### 3. Contracts
Observe inside the existing final normal/repair payload modifier. The pinned
WithRequestPayloadModifier setter replaces, rather than composes, callbacks;
adding a competing modifier can remove tool-choice/schema/repair policy.
Keep only model, temperature/top_p, max token fields, thinking/stream flags,
string tool_choice, response-format presence/type and tool count/schema digest.
Never copy messages, schema text, headers, credentials or reasoning into this
new event. Existing typed secret/image redaction remains authoritative.
Keep physical model_call_id, role/phase and parent run; reply, settlement and
final_repair remain distinguishable. Context selection/version metadata is
Core-only. Module estimates are disjoint logical sections, explicitly estimated;
missing section data is available=false, never measured zero. Existing Provider
response usage/latency records remain separate and cannot be synthesized from
fixture estimates or summed into a single request size.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| effective native tools present | observed format absent, matching final wire |
| final repair | no tools/choice/parallel, original schema and thinking=false |
| absent logical budget | available=false |
| arbitrary private payload/extra fields | excluded by allowlist before event |
| sink failure | existing bounded best-effort behavior; no business outcome change |

### 5. Good / Base / Bad Cases
Good: SDK HTTP/SSE payload and observed config match for each physical call.
Base: Provider usage missing remains unknown. Bad: a UI prompt proves schema was
sent, a second modifier overwrites repair policy, or raw body is logged wholesale.

### 6. Tests Required
TestToolFinalSchemaSeparationPhysicalRequests compares observation with actual
captured Generate/Stream payloads, including malformed final and invalid repair.
TestPhysicalRequestConfigurationExcludesPrivatePayload checks private strings
and unknown fields remain absent and missing estimates are unavailable.

### 7. Wrong vs Correct
Wrong: append a second payload modifier and log its whole JSON. Correct: observe
an allowlist after the existing modifier's policy edits and verify against wire.
