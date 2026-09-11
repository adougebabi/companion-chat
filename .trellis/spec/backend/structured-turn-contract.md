# Structured Turn Contract

## Scenario: Structured control alongside streamed visible chat

### 1. Scope / Trigger

- Trigger: a chat completion must carry durable memory, affect/drives signals, or capability intent without asking application code to infer meaning from user-visible prose.
- The browser turn transport is `POST` + `application/x-ndjson` with checked
  `token | message | media | completed | error | heartbeat` frames. Core
  `action_result.message` carries committed authoritative rows and BFF maps it
  to `message`/`media`; this spec governs the provider-to-application boundary
  and commit behavior.

### 2. Signatures

- Provider normalized completion: `{text, tokens, toolCalls, structuredTurn?, control?, parseErrors?, doneSeen}`.
- A cognitive response with an accepted `visible_text` is already the reply
  realization and may be emitted directly. Missing/omitted visible text fails
  closed unless a declared visible-output CapabilityInvocation supplies it;
  the turn never opens a second Main LLM realization call. Compatibility action
  names such as `respond` normalize to the canonical `reply` action before this
  decision.
- Direct conversation has no successful “silent reply” state. The same Main
  cognition must return concrete visible assistant text (directly or through a
  declared conversation-output Capability); otherwise Core returns
  `cognition_visible_text_missing`, preserves the already committed user row
  and retry identity, and commits no completed frozen action/native effect. A
  deferred output invocation without a concrete assistant/Moment target
  remains a bounded `deferred` result; background surfaces may still settle an
  explicit tool-only `no_op`.
- When a newer turn is accepted for the same conversation, older pending or
  claimed conversation cognition facts are marked superseded. Completion locks
  the inbox and requires the current claim/status; an old settlement can never
  restore `processed` or persist a late assistant after supersession.
- Canonical turn schema: `schemaVersion: 'companion.turn.v1'` with `control.affectEvents[]`, `control.driveSignals[]`, `control.memoryWrites[]`, `control.appraisals[]`, `control.memoryConsolidations[]`, `control.selfModelClaims[]`, `control.agencyIntentions[]`, and `control.capabilityCalls[]`.
- Appraisal candidate: `companion.appraisal.v1` with model rationale, confidence, evidence references, optional `interactionFactId`, and only allowlisted reducer candidates. The application must validate an optional fact link against the current persona and source message before persistence.
- Memory consolidation candidate: `companion.memory-consolidation.v1` with exactly one bounded `key`/`value` claim or free-form `claim`, evidence/source-fact references, revision/status, and optional `interactionFactId`. It is an auditable candidate ledger entry, not an automatic write to `companion_memories`.
- Self-model claim: `companion.self-model.v1` with LLM-owned category/claim/summary, uncertainty, evidence refs, revision/status and optional decay policy. Active claims are a separate projection and never mutate foundation.
- Agency intention: `companion.agency-intention.v1` with LLM-owned intent/topic/explanation, evidence refs and lifecycle status. Candidate persistence does not deliver a message; qualification, freeze, lease and delivery remain owned by proactive flows.
- Supported first-release drives: `social`, `exploration`, `rest`; pressure is `0..1`, where higher means more unmet need.
- Memory capability: `memory_event({type, content, confidence, importance,
  emotional_significance?})`. Owner/profile/Conversation/evidence/visibility/
  time/idempotency/revision and embedding data are Runtime-owned and frozen in
  the separate `memory_plan` PreparedPayload.
- Appearance capability: `appearance_event({operation: 'set'|'clear', outfit?, reason?})`; it is persona-scoped, source-message-bound, idempotent, and persists the current outfit in the normalized state projection while retaining an auditable `appearance_change` life event.
- State tools: `affect_event({event: {type, confidence, idempotencyKey}})` and `drive_signal({signal: {drive, direction, confidence, idempotencyKey}})`; the server owns numeric deltas.
- Native capability tools are defined once by the Go Runtime capability registry. The registry exposes only installed/preflighted slots in stable order; capability-specific filtering and additional slots are additive.
- Affect persistence: `companion_persona_affect_states` materialized snapshot plus `companion_persona_affect_events` append-only events, unique on `(persona_id, idempotency_key)`.

### 3. Contracts

- Visible text may stream from provider `content`; structured controls are accumulated and validated before any side effect is applied.
- If a text-only realization provider returns a structured action wrapper,
  treat it as transport/control data and expose only its user-facing
  `content`/`text` value; `action_type`, arguments, and the wrapper object must
  never be persisted or emitted as conversation text. This applies to bare
  JSON, fenced Markdown JSON, and equivalent transport wrappers.
- Native tool calls and parsed provider sidecars are normalized at one application boundary. New affect/memory behavior must not add text markers.
- Machine-readable argument shape belongs to the canonical capability catalog and provider `tools` payload. The model-facing system prompt contains only short behavioral guidance; it must not duplicate JSON schema bounds, dispatcher internals, or legacy marker syntax. Flow validators remain authoritative for ownership, time windows, policy, idempotency, and transactions.
- Native-capable providers receive the catalog directly. Provider-native calls
  and the single root JSON sidecar normalize into the same
  `CapabilityInvocation`; no active marker adapter or second execution path is
  advertised. A future provider-specific capability profile may filter tools,
  but the current base implementation selects definitions by surface metadata.
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
| Duplicate native and root-sidecar call for one provider response | Native call owns the capability; the matching sidecar cannot execute a second effect |
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

## Historical Scenario: Go Runtime Capability Slots And Tool Calls (pre-cutover)

This section records the released pre-cutover envelope for audit and migration
reference only. It is not an active contract. The active Go contract is the
unified Capability Runtime scenario below; production replay and workflow code
does not consume the released pre-cutover shapes.

### 1. Scope / Trigger

- Trigger: the Go Core receives a provider completion that requests an
  external capability, or a visible turn crosses the Core/BFF stream boundary.
- The signatures and examples below are historical migration inputs only.

### 2. Signatures

```text
Released invocation envelope {
  id, name, arguments,
  source_fact_id, action_id,
  provider_request_id, schema_version, sequence
}

Released result envelope {
  tool_call_id, name, status, output?, error_code?, retryable,
  provider_request_id?, correlation_id?, schema_version
}

Released composite envelope {
  schema_version, kind, action_type, response_intent,
  tool_calls[], output_bindings[]
}

OutputBindingV1 {
  tool_call_id, target_kind, target_ref
}

ProviderClient.StructuredWithTools(ctx, role, messages, released definitions)
  -> ProviderCompletion{text, structured?, tool_calls, done_seen}
```

### 3. Contracts

- `CapabilityRegistry` was a generic slot registry. Runtime owned lookup,
  authorization/resource scope, revision/idempotency checks, persistence,
  timeout/retry/cancel and result settlement; an executor owns only its
  external provider operation.
- A user-visible conversation reply or Moment is a `CompositeActionV1`, not a
  Tool. The same target-neutral capability slot may be bound to either output
  through `OutputBindingV1`; target-specific names such as `message_media` or
  `moment_media` are not part of the protocol.
- A released definition with `side_effect_class=external_async` and non-empty
  `target_kinds` is a deferred output slot. Runtime records a bounded
  `deferred` ToolResult while the action is being realized, persists the
  message/Moment, then invokes the executor's `ExecuteDeferredTx` in the same
  caller-owned transaction. The executor creates the durable external intent
  with the concrete target ID. This ordering prevents a conversation-level
  compatibility message from being created for a message-targeted result.
- The released definition was the typed slot contract. New code uses the
  direct `CapabilityDefinition` contract documented in the active scenario.
- Structured Provider calls always send a strict `response_format` using a
  named `json_schema`. The `cognitive_assessment` role additionally sends
  `enable_thinking: true`; other structured roles keep thinking disabled. Some
  thinking-capable Providers place the structured JSON in
  `reasoning_content` while leaving `content` empty; the adapter reads that
  field as control data only and never exposes it as visible text. The adapter
  may remove complete, known transport wrappers (`<think>`, a JSON Markdown
  fence, one JSON-string encoding, or a terminal object after a transport
  prelude), but it must not infer semantics from arbitrary prose. A rejected
  structured response is normalized field-by-field: missing values use the
  schema's empty value, an object supplied for an array field is wrapped as a
  one-item array, and an array supplied for an object field uses its first
  object. Correctly typed sibling fields are left untouched. Native tool calls
  are normalized independently and remain usable when the structured sidecar
  is empty or malformed. The adapter records only bounded channel-shape
  diagnostics (such as presence and length), never hidden reasoning/raw
  provider output. The visible `action_realization` stream remains ordinary
  text and does not use JSON response formatting.
- Tool names use `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`. Arguments are bounded JSON
  objects (64 KiB maximum) and are validated once at the provider-to-runtime
  boundary. Native provider entries and JSON sidecars normalize to the same
  the released call envelope.
- The released external slot used a thick media argument. Future external
  video/audio/search slots and native
  Fluctlight slots such as `scene_event`, `presence_event`,
  `memory_event`, or `relationship_signal` are additive executors; a slot is
  advertised only when its implementation is installed and preflighted, and
  adding a slot does not change cognition schemas.
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
- Released tool results were persisted with the frozen action before
  realization and may
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
  source fact, recorded as a deferred result, then bound to the persisted
  assistant message (or Moment) and creates one idempotent media intent.
- Base: a provider returns the canonical JSON sidecar for a reply; the existing
  turn behavior remains unchanged and no external effect is created.
- Bad: parse “请画一张图” with a keyword branch, write a media row directly,
  or execute a second effect when duplicate provider shapes accompany a native call.

### 6. Tests Required

- Unit tests for native/sidecar normalization, strict name/argument bounds,
  duplicate IDs, unknown capabilities, stable manifest ordering, and result
  identity/status validation.
- Runtime tests for registry injection, source-fact ownership, idempotent
  media intent creation, target binding, deferred settlement, tool-result
  persistence, and frozen replay without a second plugin call.
- Provider tests for `tools` request payloads, native tool-call responses,
  JSON sidecar responses, malformed calls, and stable request headers.
- Core/BFF stream tests for provider chunk → Core NDJSON → browser frames,
  post-settlement token delivery, abort, one terminal frame, and hidden payload
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
completion, err := provider.StructuredWithTools(ctx, "cognitive_assessment", messages, registry.Catalog(surface))
invocations := completion.ToolCalls // native and sidecar forms are already normalized
frozen := validateAndFreeze(invocations, sourceFactID, stateRevision)
output := persistAssistantOrMoment(...)
result, _ := runtime.ExecuteDeferred(ctx, tx, frozen, OutputBindingV1{TargetKind: output.Kind, TargetRef: output.ID})
persistCapabilityResult(result)
```

## Scenario: P1 Context Projection And Self-Evaluated Expression

### 1. Scope / Trigger

- Trigger: a Fluctlight response or native capability candidate needs current
  life context, authorized Memory, evidence-bound claims, or self-model
  evolution.

### 2. Signatures

```text
BuildContextProjection(ctx, actorID, fluctlightID, conversationID,
                       sourceFactID, currentUserText) -> ContextProjection
normalizeResponsePlan(decision, sourceFactID, context) -> ResponsePlanV1
RetrieveMemoryContext(ctx, actorID, fluctlightID, conversationID,
                      query, limit, tokenBudget) -> MemoryContextV1
ProcessReflection(ctx, fluctlightID, correlationID) -> ReflectionOutcome
```

### 3. Contracts

- `ContextProjection` is the sole current-context reader for cognition,
  realization, Memory retrieval, Reflection and scene/presence slots. It
  includes source/revision/confidence/expiry metadata and bounded recent
  messages; full transcript is a record, not a truth source.
- Claims are classified as `confirmed_fact`, `observed_fact`,
  `supported_hypothesis`, `uncertain_hypothesis`, or `unsupported_self_claim`.
  Unsupported self-claims are omitted or downgraded and are never promoted to
  long-term Memory/Personality merely because an assistant message contains
  them.
- The cognition response schema is closed for persisted claims: each claim
  uses `kind`, `content`, `confidence`, and `evidence_refs` (with optional
  `repetition_key`). Older provider aliases such as `claim` are normalized to
  `content` only at the cognition application boundary. Semantic references such as
  `current_message.content` or `life_context.*` are bound to the current
  source fact before persistence; arbitrary provider-supplied IDs are not
  trusted as evidence.
- Repetition of the same normalized claim/topic without new evidence is a
  deterministic no-op: it does not raise confidence, create another Memory or
  Life World row, or re-enter the same context section.
- Ordinary replies may use a one-call fast path. Effects and native candidates
  use `assessment → self-evaluation → freeze → effect → optional realization`;
  realization renders the frozen plan and cannot add semantic effects.
- `scene_event` and `presence_event` are replaceable native slots. Life World
  owns canonical Event persistence; Presence can overlay only
  `user_presence/current_task` and never replace scene/activity/location.
- Memory retrieval filters owner/visibility/actor/conversation before lexical,
  FTS/vector or hybrid ranking and token budgeting. New Memory records create
  their revision, embedding intent and outbox atomically.
- Reflection claims a Fluctlight evidence window with watermark/CAS, validates
  refs against that window, and applies Memory/Relationship/Self-model/
  Personality revisions through authority ports. Slow fields require multiple
  evidence-bearing facts and every revision is auditable/rollbackable.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Claim has unknown kind, foreign evidence, or invalid confidence | Reject the plan; no semantic write |
| Unsupported self-claim or repeated claim without new evidence | Store bounded rejected/expired provenance; omit from normal context |
| Scene/presence candidate has invalid temporal bounds or source fact | Reject candidate; no Event/Presence mutation |
| Memory visibility/owner filter fails | Exclude before ranking; do not leak to provider |
| Reflection candidate references an outside-window fact | Reject candidate and keep the window retryable |
| Personality/Self-model evidence is below its threshold | Defer candidate; do not mutate slow state |
| Realization adds a claim/effect not in frozen plan | One bounded rewrite; then omit/uncertain/deferred |

### 5. Good / Base / Bad Cases

- Good: a user fact creates one evidence-bound Memory, later retrieval injects
  it into ContextProjection, and a Reflection window promotes a recurring
  preference only after enough evidence.
- Base: a provider emits no claims; the Runtime returns a normal grounded reply
  without inventing a Memory or scene.
- Bad: feed the complete transcript to every call, treat the last assistant
  sentence as a new fact, create a scene event on every turn, or let realization
  re-decide the action.

### 6. Tests Required

- Projection tests for current-input priority, Event > Schedule > pending,
  Presence overlay, authorized Memory and hypothesis expiry.
- Claim tests for unsupported self-claim omission, repeated no-op,
  correction/supersede, confidence/evidence bounds and one bounded rewrite.
- Native slot tests for scene/presence idempotency, temporal bounds and ordered
  cognition re-entry without duplicate events.
- Memory tests for authorization-before-ranking, FTS/vector/hybrid scoring,
  token budget, embedding failure, revise/forget and rollback.
- Reflection tests for producer, window lease/CAS, evidence ownership,
  candidate apply, fast/medium/slow evolution and future projection use.

### 7. Wrong vs Correct

#### Wrong

```go
history := fullConversationTranscript()
prompt := append(history, "You previously said ...")
return provider.Generate(prompt) // old assistant prose becomes a new fact
```

#### Correct

```go
projection := BuildContextProjection(...)
plan := normalizeResponsePlan(assessment, factID, projection)
gate := selfEvaluateAndValidate(plan, projection)
frozen := freeze(gate)
return renderFrozenPlan(frozen, projection)
```

## Scenario: Unified Capability Runtime And Thin Provider Catalog

### 1. Scope / Trigger

- Trigger: a Go Core provider call advertises local capabilities, a native or
  structured-sidecar invocation is frozen, or a frozen/action payload is
  replayed after the Capability Runtime cutover.
- The Registry/Definition/Invocation/Result/ContextResolver chain is the only
  active execution model. Provider-native/root-sidecar envelopes are decoded
  once at ingress and are not accepted by Runtime/replay as a second API.

### 2. Signatures

```go
type Capability interface {
    Definition() CapabilityDefinition
    RequiredContext() []ContextSlot
    Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}

type CapabilityPreparer interface {
    Prepare(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error)
}

type TransactionalCapability interface {
    ExecuteTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}

type ContextResolver interface {
    Resolve(context.Context, ContextRequest, []ContextSlot) (CapabilityContext, error)
}

CapabilityRegistry.Catalog(surface) []CapabilityDefinition
RenderCapabilityTools(definitions) []map[string]any
CapabilityRuntime.Execute(ctx, invocation) (CapabilityResult, error)
```

### 3. Contracts

- `CapabilityDefinition.InputSchema` is the only Provider-facing schema. Thin
  inputs contain model-owned decisions; renderer/workflow/database/evidence,
  revision, and idempotency fields stay Core-owned. Planner schemas are never
  sent in `tools`.
- Provider `Arguments` are immutable and validated with the complete bounded
  Definition schema, including additional-properties, conditional alternatives,
  nested types, enum, pattern, length and numeric bounds. Capability-local plans
  and Runtime provenance live only in the separately versioned
  `PreparedPayload`; a Provider cannot supply or override that payload through
  Arguments.
- `CapabilitySurface` selects conversation, WakeUp, autonomy, or native
  cognition catalogs. Callers do not maintain concrete-name exclusion lists.
- A Capability declares `ContextSlot` dependencies. The resolver loads only
  those slots and exposes typed value objects plus a bounded replay snapshot;
  live authorization, revision, and idempotency guards remain in execution.
- Runtime order is invocation validation → Registry lookup → context resolve →
  optional preflight/Prepare → persist frozen PreparedPayload → sequential
  execute or caller-owned transactional apply → provider-safe result. Native
  and sidecar calls normalize once and native calls remain authoritative.
- Runtime Prepare is mandatory for every invocation, even when a Capability has
  no capability-local planner. It freezes Runtime-owned provenance, an explicit
  PreparedPayload envelope, and the declared ContextSnapshot before apply;
  replay validates that envelope and never regenerates it.
- Interactive Memory/Affect mutations execute in per-Capability savepoints
  inside the assistant transaction. Optional failure rolls back the savepoint
  and persists a failed result; required failure rolls back the complete visible
  settlement. Provider/Redis/object/workflow I/O is forbidden in this phase.
- `memory_event` uses `required_for_visible_claim`: a failed explicit Memory
  write rolls back the visible settlement instead of allowing “I remembered”
  prose to commit without the Memory. Its non-transactional executor returns
  `caller_transaction_required`.
- Reflection Memory output is the closed operation-aware candidate shape
  (`create|confirm|revise|merge|supersede|deprecate`, opaque target/merge refs,
  semantic fields/evidence/reason). Core compiles it to the same Memory
  lifecycle authority used by chat and Owner governance; malformed candidates
  invalidate the proposal before watermark advancement.
- `required_for_visible_claim` failures roll back a visible assistant/Moment
  settlement; `optional_internal` failures remain structured and auditable.
  Conversation never sends a same-turn `role=tool` continuation.
- A tool-only result without a structured appraisal may settle as a
  capability-only `no_op` only on explicitly non-visible/background surfaces.
  Direct conversation always requires visible assistant text from the same
  Main cognition; tool-only/no-visible output fails with
  `cognition_visible_text_missing` before Capability settlement. Core never
  creates a synthetic neutral/default appraisal, and Provider output cannot set
  `cognitive_state_transition=not_proposed`.
- Appraisal is optional independently of the visible-output channel. A valid
  direct reply supplied through `visible_text` or `conversation.reply` with no
  appraisal commits both authoritative messages and records Core-owned
  `cognitive_state_transition=not_proposed`; it writes no appraisal/state
  revision. A present malformed appraisal is not equivalent to absence and
  still fails closed.
- Appraisal is closed and ref-bound. Optional Drive signals contain only an
  opaque Drive ref, increase/decrease direction, bounded strength/confidence,
  and frozen context evidence refs. Core owns pressure/conflict numbers and
  records the post-transition state ref in the frozen action and ActionOutcome.
- Active frozen/action payloads use `capability_runtime_version`,
  `capability_invocations`, `capability_results`, and bounded per-invocation
  context snapshots. Migration head `0026_capability_runtime` converts only
  active released rows, moves legacy prepared/provenance fields out of
  Arguments, converts legacy results, and validates workflow authority.
  Completed audit rows are not rewritten; completed per-call results are not
  re-executed. Missing active provenance fails closed and IDs are never fabricated.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Nil, duplicate, invalid, or unknown capability | Deterministic registry error or `capability_not_found`; no executor call |
| Invalid invocation JSON/identity/source/provider ID | `invalid_arguments`; no side effect |
| Declared slot missing, loader failure, or cancelled context | `context_resolve_failed`; no side effect |
| Capability returns invalid status/output or execution error | `execution_failed`; required policies fail closed |
| Provider Arguments contain an undeclared prepared/runtime field | `invalid_arguments`; Prepare and executor are not called |
| Frozen PreparedPayload is malformed or conflicts with thin intent/context | fail closed; do not re-plan or repair it |
| Invocation has no Capability-local preparer | Runtime still freezes provenance, declared ContextSnapshot, and an explicit empty PreparedPayload envelope before apply |
| Direct conversation omits visible assistant text | `cognition_visible_text_missing`; preserve committed user row and same retry identity; commit no completed action/effect |
| Direct conversation has valid visible text/`conversation.reply` but omits appraisal | Commit user + assistant and Capability settlement; write no appraisal/state revision and do not synthesize neutral state |
| Background tool-only result omits appraisal | Settle the Capability-only `no_op`; write no appraisal or state revision |
| Appraisal contains unknown/raw numeric fields or foreign context evidence | Reject before freeze; do not infer or append a replacement appraisal |
| Frozen State or AffectProfile revision changed before apply | Terminal conflict/re-assessment boundary; no mutation and no blind retry |
| Thin schedule intent without a configured internal planner | `schedule_replan_planner_failed`; never fall back to thick schema |
| Active replay payload lacks stable provenance | Migration/replay fails closed; completed audit data remains readable |

### 5. Good / Base / Bad Cases

- Good: a dummy Capability is registered once, appears in a surface catalog,
  renders through `RenderCapabilityTools`, resolves its declared slot, and
  executes without a MainAgent/schema switch.
- Base: a native and root sidecar call normalize to one Invocation; a deferred
  output binds after its durable target exists and retries the same IDs.
- Bad: add a `switch call.Name` to MainAgent, expose the old image concept or
  schedule revision fields, infer a missing semantic field, or call Main LLM a
  second time with ToolResults.
- Bad: manufacture a “neutral” appraisal for a Tool-only result, or execute a
  transactional autonomy sibling before the message/Moment/action transaction.

### 6. Tests Required

- Registry tests cover registration, nil/invalid/duplicate behavior, stable
  ordering, lookup, and surface catalogs.
- Context tests cover slot declarations, cancellation, missing loaders,
  snapshot round-trip, deduplicated requests, and no unrelated reads.
- Schema tests cover thin forbidden fields, one root sidecar, and byte/char
  reports; Runtime tests cover taxonomy, preflight, sequential ordering,
  deferred binding, and required/optional failure policies.
- Replay tests cover canonical payload enrichment, stable IDs, malformed active
  fail-closed behavior, and untouched completed audit rows.
- Affect/Drive tests cover foreign appraisal refs, raw numeric rejection,
  non-finite values, profile/state CAS, same-source coalescing, elapsed-time
  partition invariance, typed-slot deactivation, and later state-ref citation.
- Transaction tests make one sibling mutate and a required sibling fail, then
  assert state/action/output/outcome/outbox authorities all roll back together.
- Conversation delivery tests cover the Provider-native shape with empty
  content/reasoning, one `conversation.reply` Tool call and no appraisal. They
  assert committed user before Provider completion, committed assistant after
  settlement, zero state revisions, and ordered user/token/assistant frames.
  Browser tests assert a terminal error never erases that committed user row or
  its retry identity.

### 7. Wrong vs Correct

#### Wrong

```go
if call.Name == "media.image.generate" {
    createMediaIntent(ctx, call.Arguments)
}
```

#### Correct

```go
definition := registry.Definition(invocation.CapabilityName)
context := resolver.Resolve(ctx, request, definition.RequiredContext)
result := runtime.Execute(ctx, invocation)
```
