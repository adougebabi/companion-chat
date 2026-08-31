# Structured Turn Contract

## Scenario: Structured control alongside streamed visible chat

### 1. Scope / Trigger

- Trigger: a chat completion must carry durable memory, affect/drives signals, or capability intent without asking application code to infer meaning from user-visible prose.
- The browser turn transport is the existing `POST` + `application/x-ndjson` `token`/`completed`/`error` contract; this spec governs the provider-to-application boundary and commit behavior. A future SSE subscription, if needed for server push, is a separate projection and is not a second turn/control stream.

### 2. Signatures

- Provider normalized completion: `{text, tokens, toolCalls, structuredTurn?, control?, parseErrors?, doneSeen}`.
- Canonical turn schema: `schemaVersion: 'companion.turn.v1'` with `control.affectEvents[]`, `control.driveSignals[]`, `control.memoryWrites[]`, `control.appraisals[]`, `control.memoryConsolidations[]`, `control.selfModelClaims[]`, `control.agencyIntentions[]`, and `control.capabilityCalls[]`.
- Appraisal candidate: `companion.appraisal.v1` with model rationale, confidence, evidence references, optional `interactionFactId`, and only allowlisted reducer candidates. The application must validate an optional fact link against the current persona and source message before persistence.
- Memory consolidation candidate: `companion.memory-consolidation.v1` with exactly one bounded `key`/`value` claim or free-form `claim`, evidence/source-fact references, revision/status, and optional `interactionFactId`. It is an auditable candidate ledger entry, not an automatic write to `companion_memories`.
- Self-model claim: `companion.self-model.v1` with LLM-owned category/claim/summary, uncertainty, evidence refs, revision/status and optional decay policy. Active claims are a separate projection and never mutate foundation.
- Agency intention: `companion.agency-intention.v1` with LLM-owned intent/topic/explanation, evidence refs and lifecycle status. Candidate persistence does not deliver a message; qualification, freeze, lease and delivery remain owned by proactive flows.
- Supported first-release drives: `social`, `exploration`, `rest`; pressure is `0..1`, where higher means more unmet need.
- Memory capability: `memory_event({memory: {operation, key, value, confidence, sourceMessageId?, idempotencyKey}})`.
- Appearance capability: `appearance_event({operation: 'set'|'clear', outfit?, reason?})`; it is persona-scoped, source-message-bound, idempotent, and persists the current outfit in the normalized state projection while retaining an auditable `appearance_change` life event.
- State tools: `affect_event({event: {type, confidence, idempotencyKey}})` and `drive_signal({signal: {drive, direction, confidence, idempotencyKey}})`; the server owns numeric deltas.
- Native capability tools are defined once by the Go Runtime capability registry. The registry exposes only installed/preflighted slots in stable order; capability-specific filtering and additional slots are additive.
- Affect persistence: `companion_persona_affect_states` materialized snapshot plus `companion_persona_affect_events` append-only events, unique on `(persona_id, idempotency_key)`.

### 3. Contracts

- Visible text may stream from provider `content`; structured controls are accumulated and validated before any side effect is applied.
- Native tool calls, parsed provider sidecars, and legacy media/pending markers are normalized at one application boundary. New affect/memory behavior must not add text markers.
- Machine-readable argument shape belongs to the canonical capability catalog and provider `tools` payload. The model-facing system prompt contains only short behavioral guidance; it must not duplicate JSON schema bounds, dispatcher internals, or legacy marker syntax. Flow validators remain authoritative for ownership, time windows, policy, idempotency, and transactions.
- Native-capable providers receive the catalog directly. Legacy marker adapters remain compatibility fallbacks and must not be advertised in the normal prompt. A future provider-specific capability profile may filter tools, but the current base implementation sends the universal catalog unchanged.
- Scene and appearance are separate facts. When a scene transition also changes clothing, the model may issue one `scene_event` and one `appearance_event` in the same turn; an explicit clothing change in an unchanged scene may issue only `appearance_event`. Ordinary prose or transient gestures never update clothing state.
- `memory_event` is the only ordinary-chat path to long-term memory. It is persona-private, source-message-bound, idempotent, and committed with the assistant facts when the turn succeeds.
- Appraisal and memory-consolidation sidecars are LLM-owned semantic candidates. The server may reject invalid schema, missing evidence, source ownership, idempotency, or CAS state, but must not infer a replacement from visible text or a rejected candidate. A candidate's `interactionFactId`, when present, must resolve to an existing fact owned by the same persona and bound to the same source message.
- Appraisal, memory-consolidation, and affect effects are applied through the existing caller-owned chat commit transaction. They remain distinct effect capability identifiers and never create a second NDJSON/control stream or a second chat commit boundary.
- Self-model and agency intention effects use the same caller-owned transaction and distinct effect capability identifiers; they remain candidate/projection writes and cannot create a second NDJSON/control stream or bypass proactive delivery gates.
- Affect state uses persona baselines and lazy exponential decay; normal decay does not create timer events. Unknown future drive keys may be retained but are inactive until a server policy exists.
- Raw PAD values, hidden reasoning, prompts, credentials, and unbounded provider diagnostics never enter user-visible chat or ordinary API DTOs.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown schema version or oversized/unknown control field | Drop optional control effects, retain valid visible text, store a bounded diagnostic |
| Invalid memory source/persona/confidence/idempotency | Reject memory plan; do not write memory or claim `learned` success |
| Invalid affect event or arbitrary model delta | Reject the event; server reducer remains the only delta owner |
| Invalid appraisal/consolidation schema, required evidence, or interaction-fact ownership | Drop the optional semantic candidate, retain visible text, and record a bounded diagnostic; never synthesize a semantic fallback |
| Invalid self-model/agency schema, evidence, or source ownership | Drop the optional semantic candidate, retain visible text, and record a bounded diagnostic; never synthesize a self/agency fallback |
| Duplicate `(persona_id, idempotency_key)` | Replay existing result; do not duplicate rows or effects |
| Snapshot revision/CAS conflict | Refuse stale update; do not overwrite newer state |
| Provider text-only completion | Normalize with empty control channels and preserve existing chat behavior |
| Legacy marker plus supported native/structured call | Native/structured path owns the capability; matching marker cannot execute a second effect |
| Assistant/message or memory/effect transaction failure | Roll back the complete caller-owned transaction |

### 5. Good/Base/Bad Cases

- Good: a native `memory_event` call and a visible assistant sentence produce one assistant message and one persona-scoped memory row in one commit.
- Base: a plain text completion produces the same NDJSON events and no control rows.
- Bad: parsing “我有点生气” with a regex and writing PAD, trusting a model-supplied numeric delta, or exposing raw structured arguments in `token`/`done`.

### 6. Tests Required

- Contract tests for text, native tool, structured sidecar, strict JSON content, malformed sidecar, unknown fields, source scope, and duplicate idempotency.
- Provider tests for streamed/native control accumulation and JSON completion sidecar extraction without a second stream accumulator.
- Affect repository tests for lazy decay, bounded reducer deltas, event/snapshot atomicity, CAS, replay, future drive preservation, and persona isolation.
- Memory flow tests for explicit invocation, source ownership, upsert/replay, rollback, deletion compatibility, and no implicit extraction.
- Chat integration test asserting assistant message + memory + affect/drives commit and existing `token`/`completed` output.

### 7. Wrong vs Correct

#### Wrong

```js
const angry = /生气|讨厌/.test(userText);
database.prepare('UPDATE companion_persona_states SET mood = ?').run(angry ? '生气' : '平静');
```

#### Correct

```js
const turn = normalizeStructuredTurn(completion, {personaId, sourceMessageId});
const plan = affectFlow.plan(turn.control);
// Commit the validated plan with the assistant facts in the caller transaction.
```

## Scenario: Go Runtime Capability Slots And Tool Calls

### 1. Scope / Trigger

- Trigger: the Go Core receives a provider completion that requests an
  external capability, or a visible turn crosses the Core/BFF stream boundary.
- This scenario defines the active Go implementation. The retired Node
  capability catalog and browser SSE wording are historical only.

### 2. Signatures

```text
ToolCallV1 {
  id, name, arguments,
  source_fact_id, action_id,
  provider_request_id, schema_version, sequence
}

ToolResultV1 {
  tool_call_id, name, status, output?, error_code?, retryable,
  provider_request_id?, correlation_id?, schema_version
}

CapabilityExecutor.Manifest() -> CapabilityManifest
CapabilityExecutor.Execute(ctx, fluctlightID, conversationID,
                           sourceFactID, ToolCallV1) -> ToolResultV1
ProviderClient.StructuredWithTools(ctx, role, messages, manifests)
  -> ProviderCompletion{text, structured?, tool_calls, done_seen}
```

### 3. Contracts

- `CapabilityRegistry` is a generic slot registry. Runtime owns lookup,
  authorization/resource scope, revision/idempotency checks, persistence,
  timeout/retry/cancel and result settlement; an executor owns only its
  external provider operation.
- Tool names use `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`. Arguments are bounded JSON
  objects (64 KiB maximum) and are validated once at the provider-to-runtime
  boundary. Native provider entries and JSON sidecars normalize to the same
  `ToolCallV1`.
- The first registered slot is `media.image.generate` with an object `concept`
  argument. Video/audio/search slots are not advertised until an executor is
  installed and preflighted; adding a slot does not change cognition schemas.
- A tool call is a structured proposal, not direct SQL or arbitrary domain
  access. Memory, Reflection, affect, evolution, and other native Fluctlight
  capabilities remain authority services even if a narrow intent is exposed to
  the model.
- No human approval/HITL state is introduced. Installed and preflighted
  capabilities are callable subject to deterministic schema, ownership,
  revision, budget, timeout, cancellation and idempotency checks.
- Interactive turns use `POST` + `application/x-ndjson`. Provider SSE is an
  adapter detail. A future authenticated SSE subscription for committed
  server-push projections is separate from the turn command and cannot be a
  second state source.
- Tool results are persisted with the frozen action before realization and may
  be included in the next provider prompt. Raw tool arguments and provider
  internals never cross the browser boundary.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/duplicate ID or invalid name | Reject the call; no executor invocation |
| Arguments missing, non-object, invalid JSON, or over 64 KiB | `tool_arguments_invalid`; no side effect |
| Unknown schema version or capability slot | `tool_call_rejected` / `tool_capability_unavailable`; no side effect |
| Source fact differs from current Fluctlight fact | `tool_call_source_invalid`; no side effect |
| Plugin operation fails before durable result | `failed` result with explicit retryable flag; frozen action remains auditable |
| External request result is ambiguous | `result_unknown`; reconcile by stable request ID, never resubmit blindly |
| Provider has no native tool call | Accept only the same structured JSON sidecar; never parse prose markers |
| Browser disconnects after action result | Stop later writes and cancel reads; keep committed action/effect settlement independent |

### 5. Good / Base / Bad Cases

- Good: a native `media.image.generate` call is normalized, frozen against the
  source fact, creates one idempotent media intent, persists one `ToolResultV1`,
  and the realization prompt receives that result.
- Base: a provider returns the canonical JSON sidecar for a reply; the existing
  turn behavior remains unchanged and no external effect is created.
- Bad: parse “请画一张图” with a keyword branch, write a media row directly,
  or execute a second effect when a legacy marker accompanies a native call.

### 6. Tests Required

- Unit tests for native/sidecar normalization, strict name/argument bounds,
  duplicate IDs, unknown capabilities, stable manifest ordering, and result
  identity/status validation.
- Runtime tests for registry injection, source-fact ownership, idempotent
  media intent creation, tool-result persistence, and frozen replay without a
  second plugin call.
- Provider tests for `tools` request payloads, native tool-call responses,
  JSON sidecar responses, malformed calls, and stable request headers.
- Core/BFF stream tests for provider chunk → Core NDJSON → browser frames,
  early token delivery, abort, one terminal frame, and hidden payload
  redaction. No test should require an SSE turn endpoint.

### 7. Wrong vs Correct

#### Wrong

```go
if strings.Contains(userText, "画") {
    return createMediaIntent(ctx, fluctlightID, conversationID, map[string]any{
        "prompt": userText,
    })
}
```

#### Correct

```go
completion, err := provider.StructuredWithTools(ctx, "cognitive_assessment", messages, registry.Manifests())
calls := completion.ToolCalls // native and sidecar forms are already normalized
proposal := validateAndFreeze(calls, sourceFactID, stateRevision)
executor, _ := registry.Lookup(proposal.Name)
result, _ := executor.Execute(ctx, fluctlightID, conversationID, sourceFactID, proposal)
persistToolResult(result)
```
