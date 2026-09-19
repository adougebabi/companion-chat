# Fluctlight Provider Contract

## Scenario: Explicit Model Roles With Capability Validation

### 1. Scope / Trigger

- Trigger: a model endpoint is configured, a role is assigned, capability preflight runs, or initialization/assessment/realization/reflection/embedding/media-prompt inference executes.
- Provider adapters normalize protocol and transport. They do not own Fluctlight semantics, numeric state, policy, or side effects.
- First delivery supports explicit role assignment without implicit fallback chains.

### 2. Signatures

```text
ProviderEndpoint
  id / kind / base_url / encrypted_api_key
  capability_status / checked_at

ModelRole
  role: initialization | cognitive_assessment | action_realization |
        reflection | embedding | media_prompt
  provider_endpoint_id / model_id
  required_capabilities
  token_budget (output reserve) / context_window_tokens / max_input_tokens
  prompt_budget_policy_version / timeout / retry_policy
```

## Scenario: Eino model foundation and ADK direct conversation

### 1. Scope / Trigger

- Trigger: a Core model task, Embedding task, or direct conversation needs to
  call an OpenAI-compatible endpoint through Eino.
- The official Eino ChatModel/Embedder owns wire protocol and message/stream
  decoding. Core owns role assignment, queue/cancellation, prompt budget,
  bounded diagnostics, capability authorization, frozen decisions and domain
  settlement.

### 2. Signatures

```go
NewEinoModelFactory(httpClient *http.Client) EinoModelFactory
EinoModelFactory.NewChatModel(ctx, EinoModelConfig) (model.ToolCallingChatModel, error)
EinoModelFactory.NewEmbedder(ctx, EinoModelConfig) (embedding.Embedder, error)
RunADKConversation(ctx, ADKConversationConfig, []*schema.Message) (ADKConversationResult, error)
NewADKCapabilityTools(definitions, ADKCapabilityInvoker) ([]tool.BaseTool, error)

type ConversationRuntime interface {
    RunMain(context.Context, ConversationMainInput) (ConversationRunResult, error)
    RunQueryContinuation(context.Context, QueryContinuationInput) (ConversationRunResult, error)
    RunTakeoverJudge(context.Context, TakeoverJudgeInput) (ConversationRunResult, error)
    RunTakeoverReply(context.Context, TakeoverReplyInput) (ConversationRunResult, error)
}
```

`ProviderClient` maps one resolved `providerAssignment` into `EinoModelConfig`;
business callers use operation-owned `ModelTask` boundaries. `ProviderCompletion`
and `CapabilityInvocation` v2 remain the only Core result/persistence contracts.

The capability definition adds one Core-only visibility bit:

```go
type CapabilityDefinition struct {
    Name        string
    Type        CapabilityType
    InternalOnly bool // executable by policy/runtime, never in model Catalog
    // ... canonical schema, surfaces and failure policy fields
}
```

### 3. Contracts

- ChatModel calls use the pinned official Eino components and the configured
  endpoint/model/secret; no project code posts or decodes `/chat/completions`
  or `/embeddings`.
- Each Generate/Stream/EmbedStrings call obtains its own local/Redis queue
  lease, timeout and cancellation watcher. An ADK Agent loop does not hold one
  lease across multiple model calls.
- `schema.ToolCall` is bounded once into `CapabilityInvocation`; tool ID/name/
  arguments are never guessed from prose. `schema.Message` multimodal parts
  preserve text plus image URL/data boundaries.
- Direct conversation builds a request-scoped ADK `ChatModelAgent` and
  `Runner` with `MaxIterations <= 2`, no automatic retry/failover, and a
  narrow `ADKCapabilityInvoker`. Query tools return the real bounded result;
  mutation/external tools return explicit `deferred` status until frozen
  Prepare/transaction settlement.
- `ADKCapabilityTrace` is request-scoped metadata only. It is merged into the
  existing invocation/result arrays and is never persisted as a second tool
  envelope or global mutable Agent state.
- Capability definitions marked `InternalOnly` remain executable through the
  Core registry for deterministic policy/maintenance actions, but are excluded
  from every model-facing `CapabilityRegistry.Catalog`; ordinary model tools
  must therefore be both Registry-defined and catalog-visible.
- Persona takeover/profile-switch policy invocations use the same capability
  identity/result contract with `Metadata.Source=policy`; they are nested policy
  audit records, never fabricated native model ToolCalls.
- `NewADKCapabilityTools` fails before Provider I/O unless the request-scoped
  invoker preserves the formal Eino tool-call ID via
  `ADKCapabilityInvokerWithID`. Missing IDs are fail-closed and are never
  derived from capability names or arguments.
- Initialization keeps JSON-object response format and its operation-owned
  budget floor. Embedding stays on the independent embedding queue. Browser
  NDJSON still publishes only after assistant settlement.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing assignment/model or unsupported Eino component | Explicit `eino_*`/role configuration error; no generic fallback |
| Invalid tool schema/message part/response schema | Fail before Provider I/O; record bounded preflight stage |
| ADK tool call is unknown, unauthorized, malformed or missing frozen identity | Return rejected/failed bounded result; no domain side effect |
| Pure query tool execution fails | ADK returns an explicit failed tool result; no empty final success |
| Mutation/external capability is requested during ADK loop | Return `deferred` result; Core later owns Prepare/CAS/transaction/intent |
| ADK exceeds two generations, is cancelled or model fails | Typed failure; no assistant publish or fabricated success |
| Embedding vector is empty, non-finite or wrong dimension | Reject vector and preserve retrieval fallback/intent retry policy |
| Provider emits hidden reasoning or raw diagnostics | Keep only bounded structured candidate/shape metadata; never expose full reasoning |

### 5. Good/Base/Bad Cases

- Good: the first ADK model call emits `memory.recall`; the Capability adapter
  executes the read-only runtime, the second request contains one matching
  `tool` message, and Core publishes the final reply once.
- Base: an image-generation call receives a truthful `deferred` tool result,
  freezes the invocation, and settles the external intent after the caller
  transaction commits.
- Bad: pass `*App` to an Eino tool, keep a queue lease for the whole Agent
  loop, call a raw HTTP endpoint beside Eino, or return `completed` before a
  deferred capability is persisted.

### 6. Tests Required

- Factory tests assert official ChatModel/Embedder request paths, model,
  response format, tool schema and usage mapping.
- ADK Fake model tests assert a formal tool call, real tool result in the next
  request, iteration cap, explicit failure/cancel and one final message.
- Prompt Composer tests assert explicit Slot selection, order, per-slot/total
  budget, current-input de-duplication and complete recent turns.
- Provider regression tests assert all Core model tasks use Eino and no
  production `/chat/completions` or `/embeddings` request builder remains.
- Queue tests assert two ADK model calls acquire/release two separate leases;
  diagnostics retain role/scenario/request identity and bounded usage.
- Real-provider/ComfyUI/credit/cache behavior remains an explicitly reported
  external acceptance item when credentials/services are unavailable.

### 7. Wrong vs Correct

#### Wrong

```go
for toolCall := range modelCalls {
	result := app.Execute(toolCall) // Agent loop owns a long-lived queue lease
	model.Generate(append(history, result))
}
```

#### Correct

```go
trace := &ADKCapabilityTrace{}
tools, _ := NewADKCapabilityTools(definitions, requestScopedInvoker)
result, err := RunADKConversation(ctx, ADKConversationConfig{
		Model: chatModel, Tools: tools, MaxIterations: 2,
}, assembledMessages)
// Core merges trace into CapabilityInvocation/Result v2 and settles it.
```

```python
preflight(role: ModelRole) -> CapabilityReport
complete_structured(role, schema, input) -> StructuredResult
stream_realization(role, input) -> AsyncIterator[ProviderChunk]
embed(role, inputs) -> VersionedEmbeddings
```

### 3. Contracts

- Generative roles may share one endpoint/model, but remain independent settings with independent budgets and provenance.
- `cognitive_assessment` and `reflection` require strict JSON Schema output.
  `initialization` uses JSON-object mode plus a compact canonical skeleton and
  Core-owned default completion/semantic validation; the full initialization
  schema remains a code contract but is not sent to mlx-style constrained
  decoding.
- A shared `generic_llm` binding retains its configured ordinary-call budget,
  but the `initialization` scenario applies an operation-owned minimum of 8,192
  output tokens and ten minutes. The floor is accepted only when
  `max_input_tokens + 8192 + 4096 <= context_window_tokens`; otherwise the
  request fails before Provider I/O as
  `initialization_output_reserve_unavailable`. This keeps dense-card fidelity
  independent from an Owner guessing numeric runtime settings without changing
  chat, WakeUp, or Reflection limits.
- `action_realization` requires streaming, abort propagation, bounded diagnostics, and correct UTF-8/chunk handling.
- `embedding` requires an embedding endpoint and fixed dimensions recorded with each vector/index version.
- `media_prompt` requires its declared structured/text output contract and cannot execute media generation itself.
- `media_prompt` may serve both B text prompt generation and C structured multimodal acceptance using the same configured role/model; C transport, vision-capability, timeout, or schema failures are surfaced to the owning media flow as an infrastructure condition, never inferred as a content `pass` or `reject`.
- Settings cannot activate a role until preflight proves required capabilities. Health may later degrade without making Core readiness false.
- Every result records role, endpoint/model ID, capability/model version when available, prompt/schema version, timing, token usage/budget, and correlation IDs.
- No implicit role/model fallback. Failure follows explicit interaction/workflow retry/deferred/no-op/terminal rules.
- Provider adapter returns normalized transport/structured results and bounded parse diagnostics. It does not parse visible prose for semantic effects or choose domain actions.
- A direct conversation may contain any mix of capability calls, including an
  action-only response with no `conversation.reply`. Every valid call is
  normalized and sent through its own Prepare, plan, and settlement boundary;
  missing visible text is a successful tool-only turn rather than a global
  `cognition_visible_text_missing` failure. A `conversation.reply` call still
  owns private text delivery when present. When a request advertises more than
  one capability definition, the OpenAI-compatible wire payload sets
  `parallel_tool_calls=true`; a single-capability payload omits the hint. This
  transport flag does not change Registry validation, execution ordering, or
  the one `conversation_messages` delivery path.
- Structured parsing accepts complete known transport wrappers: whole or
  embedded Markdown `json` fences, `<think>` wrappers, double-encoded JSON, and
  a short prelude followed by one terminal object. Embedded-fence extraction
  reads only the complete fenced body; it never scans arbitrary prose for an
  executable object. An unclosed/truncated fence remains invalid.
- Thinking is operation-owned. Semantic cognition schemas
  (`conversation_turn_response`, `takeover_reply_response`,
  `persistent_switch_assessment`, `wake_up_response`, `daily_review_response`,
  `native_cognition_response`, and `reflection_proposal_v2`) may send
  `enable_thinking=true`; the Provider adapter may read a complete structured
  object from `reasoning_content` but must never expose reasoning as visible
  text. `query_continuation_response` and `takeover_judgement_response` omit
  the flag and remain visible/strict protocols.
- Native cognition may be capability-only: when at least one valid native tool
  call is present and the semantic sidecar is empty, Core records
  `cognitive_state_transition=not_proposed`, does not fabricate appraisal or
  state, and settles the tool call. An empty semantic sidecar without a native
  call fails closed. A life fact carries `native_cognition_depth`; both the
  dispatcher and direct native recovery entry point stop processing beyond the
  configured depth and settle a bounded cycle guard.
- The Provider boundary emits at most one `system` message, and it must be
  the first message. Operation, context-authority, and language instructions
  are concatenated in caller order; `user`/`assistant` history keeps its order
  after that merged system message. This prevents strict chat templates such
  as mlx-serve from rejecting a late or repeated system role.
- API keys are resolved only in Go Core through the configuration secret contract and never returned to BFF/browser/debug output.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Role has no endpoint/model assignment | Role unavailable with explicit configuration error; no fallback. |
| Structured role returns an empty/mismatched transport shape | Normalize only the affected fields (missing → typed empty, object ↔ array container repair), preserve native tool calls independently, and let the owning domain validator decide whether the resulting semantic payload is usable; never parse arbitrary prose. |
| One native/sidecar call entry is malformed while sibling entries are valid | Keep the valid entries in the normalized completion, record a bounded diagnostic for the malformed entry, and never execute the malformed entry or discard its valid siblings. |
| Initialization Provider omits fields or uses a known alias | Preserve returned values, fill missing defaults/typed empties, mechanically map the alias, then validate explicit values. |
| Initialization is sent through the full JSON Schema constrained decoder | Contract failure; use `response_format.type=json_object` and the canonical prompt skeleton to avoid local-provider timeout/empty fallback. |
| Realization role lacks streaming/abort | Preflight fails; role cannot activate. |
| Embedding dimensions change unexpectedly | Reject vectors, mark role/index mismatch, require new embedding version. |
| Timeout/token budget exceeded | Cancel/bound result and follow owning retry/terminal policy. |
| Initialization generic binding is 4,096 tokens / 300 seconds | Apply the 8,192-token / ten-minute initialization floor when context headroom permits; diagnostics report the effective values. |
| Initialization reaches its effective deadline | Persist one `timeout/request_timeout` model run and return `initialization_provider_timeout`; do not store a raw URL/error string as `error_code`. |
| Initialization returns non-empty content that cannot be parsed | Return `initialization_response_invalid_json` directly; do not construct an empty StructuredFallback that later appears as semantic-empty. |
| Provider reports `finish_reason=length` or delimiters are unbalanced | Record `structured_response_truncated` with framing, candidate lengths, balance and syntax offset metadata; never repair or activate the partial object. |
| Direct conversation returns valid ACTION calls without `conversation.reply` | Settle each call independently, complete cognition without fabricating assistant prose, and keep each per-call failure isolated; do not use ACTION arguments or reasoning as visible text. |
| Provider/model is temporarily unavailable | Report degraded role health; request/workflow handles explicit failure. |
| API key decryption fails | Configuration error; do not use env/old-key fallback. |
| Provider returns hidden reasoning/raw diagnostics | Bound/redact and keep out of ordinary result/trace/browser contract. |

### 5. Good / Base / Bad Cases

- Good: one local chat model passes five role preflights with separate budgets; every artifact records its actual role/model/prompt version.
- Good: the shared generative binding remains 4,096/300 for ordinary calls,
  while a dense initialization receives 8,192/600 and completes without losing
  explicit card fields.
- Base: the model wraps a complete object in a Markdown JSON fence with short
  prose around it; Core extracts the designated fence and validates the normal
  initialization semantics.
- Bad: append missing braces to a `finish_reason=length` response or turn its
  half-populated object into a default Persona.
- Good: an embedding model upgrade creates a new dimension/model index and background rebuild without mixing distances.
- Base: reflection role is degraded while realization remains healthy; interactions continue, reflection workflows retry explicitly.
- Bad: one global model string with unknown capabilities, silently substitute realization for assessment, parse malformed structured output as prose, or hide fallback under Provider adapter logic.

### 6. Tests Required

- Endpoint/settings tests for encrypted keys, safe summaries, role assignment, shared model mapping, and atomic invalid-patch rollback.
- Role-specific preflight tests for initialization JSON-object parsing,
  cognition/reflection structured schema, stream/abort/chunking, embedding
  dimensions, media-prompt output, timeout, and token budgets.
- Real initialization regression calls the configured LLM with a complex
  multi-personality description and asserts non-empty distinct raw profiles
  before Core defaults; mocks are not sufficient for this gate.
- Provenance tests assert every result stores role/endpoint/model/prompt/schema/correlation metadata without credentials or hidden reasoning.
- Failure tests prove one degraded role does not silently use another and follows owning interaction/workflow policy.
- Initialization scenario tests assert the operation floor, insufficient
  context rejection, typed timeout/cancellation codes, outer HTTP budget, and a
  configured-LLM dense multi-card completion using the same effective values.
- Parser tests cover a greater-than-12,000-character embedded complete fence,
  double encoding, thinking wrappers, malformed JSON, unbalanced/truncated
  output, non-object root, typed failure, and metadata-only diagnostics.
- Provider adapter contract suite runs against fake normalized adapters and configured OpenAI-compatible test endpoints.
- The opt-in live conversation regression asserts that a media ACTION is
  returned together with `conversation.reply`, that both native calls normalize
  to canonical capabilities with non-empty reply text, and that Core freezes
  the reply fallback as visible output without `cognition_visible_text_missing`.
- Assert every real payload has exactly one leading system message and that
  merging preserves every operation/context/language instruction; media-prompt
  calls may omit the language instruction but follow the same single-system
  invariant.
- Architecture tests prevent Provider adapters from importing domain policy/repositories or implementing semantic regex/keyword fallbacks.

### 7. Wrong vs Correct

#### Wrong

```python
model = settings.default_model
try:
    return await provider.complete(model, prompt)
except Exception:
    return await provider.complete(settings.fallback_model, prompt)
```

#### Correct

```python
role = model_roles.require("cognitive_assessment")
role.require_capability("structured_output", schema_version)
return await provider.complete_structured(
    role=role,
    schema=SemanticAssessmentV1,
    input=assessment_input,
)

## Scenario: Go Provider Preflight And Explicit Media Failure

### 1. Scope / Trigger

- Trigger: Go Core configures a model role or a media workflow receives an
  endpoint/model capability error.

### 2. Signatures

- `ConfigureProviderRole(ctx, actorID, payload)` persists a preflight only
  after the endpoint model list contains the selected model.
- Provider calls carry deterministic idempotency/request headers.
- `ProviderModels(ctx, actorID, endpointID)` normalizes common model-list
  envelopes: OpenAI-style `data[]` and Ollama-style `models[]`, accepting
  string entries and object `id`/`name`/`model` fields. Unknown envelopes stay
  empty and cannot activate a role.

### 3. Contracts

- Endpoint reconfiguration invalidates bound roles until the next preflight.
- A missing ComfyUI model is a bounded failure; no alternate model is chosen.
- Successful and failed model runs are recorded with redacted diagnostics.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| endpoint/model missing or unavailable | reject role; return `provider_endpoint_not_found`, `provider_models_unavailable`, or `provider_model_not_available`; no role row |
| model list uses a supported `data[]`/`models[]` envelope | normalize IDs, deduplicate and sort before matching |
| model list is empty or has an unsupported envelope | reject role with `provider_model_not_available`; no role row |
| configured media model absent | retry, then mark media intent `failed` |
| diagnostic contains credentials/hidden reasoning | redact/drop before persistence |

### 5. Good/Base/Bad Cases

- Good: preflight passes and retries reuse one request ID.
- Base: a previously healthy endpoint degrades and returns a bounded Provider
  error while Core readiness remains healthy.
- Bad: silently selecting the first available transformer after a 400.

### 6. Tests Required

- Fake `/models` preflight success/unknown-model tests.
- Header idempotency and recursive diagnostic-redaction tests.
- Real ComfyUI model-not-found test asserting failed durable media state.

### 7. Wrong vs Correct

#### Wrong

```go
workflow["transformer"] = firstAvailableModel()
```

#### Correct

```go
markMediaIntentFailed("provider_model_not_available")
return err
```

## Scenario: Bounded Tool-Call Normalization Diagnostics

### 1. Scope / Trigger

- Trigger: an OpenAI-compatible Provider returns malformed native
  `message.tool_calls` or a malformed structured JSON `tool_calls` sidecar.
- The Provider boundary must explain why a call could not become a
  `CapabilityInvocation` without persisting model arguments, user text, or the
  raw response.

### 2. Signatures

```go
NormalizeProviderToolCalls(value, sourceFactID, providerRequestID)
  ([]CapabilityInvocation, error)
providerToolCallNormalizationDiagnostic(value, source, err)
  map[string]any
```

### 3. Contracts

- Both native and structured-sidecar failures use the bounded
  `tool_call_invalid` model-run error code and retain the original fail-closed
  normalization behavior. The strict fixture helper rejects a missing call ID;
  the production Provider adapter may derive a stable ID when the endpoint
  supplies a provider request ID, using only request identity, sequence, tool
  name and canonical arguments. It never trusts model text as the identity.
- `diagnostic_model_runs.response` may contain only shape metadata: `source`
  (`native` or `structured`), `value_shape`, `call_count`,
  `failed_item_index`, `normalization_reason`, and bounded item fields such as
  `id_present`, `id_type`, `type_value`, `name_present`, `name_length`,
  `name_valid`, `arguments_present`, `arguments_shape`, and
  `arguments_length`.
- Lengths are byte/serialized-shape measurements, and reasons are stable codes
  such as `id_required`, `name_invalid`, `duplicate_id`,
  `arguments_invalid_json`, `arguments_not_object`, and
  `arguments_oversized`.
- No call ID/name value, argument value, reasoning, user text, or complete
  Provider response may enter this diagnostic response. Unknown capability
  names remain a later registry/semantic validation error and are not reported
  as normalization failures.
- Worker/activity logs may expose the stable `tool_call_invalid` code and one
  allowlisted normalization reason through `core.ProviderErrorInfo`; they must
  omit the original error string so model-controlled IDs, names, and arguments
  cannot leak through the log path.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| native/structured call item is missing an ID but has a provider request identity | derive a deterministic `call_derived_<digest>` ID before registry validation; retries of the same request reuse it |
| call item is missing an ID and no provider request identity is available | return `tool_call_invalid`; persist the bounded `id_required` reason |
| arguments are missing, invalid JSON, non-object, or oversized | return `tool_call_invalid` with the corresponding stable reason |
| duplicate IDs or invalid names/types | return `tool_call_invalid` with the failing item index; do not execute any call |
| diagnostic contains model-controlled content | omit it; retain only bounded shape fields |

### 5. Good / Base / Bad Cases

- Good: a Provider that omits native IDs still gets deterministic call identity
  from the request boundary, while malformed arguments remain a bounded
  `tool_call_invalid` diagnostic with no tool payload.
- Base: a valid native or sidecar call continues through the existing
  normalization path unchanged.
- Bad: log the full arguments, response body, or model reasoning to explain a
  `tool_call_invalid`, or derive an ID from an unbounded/model-controlled field
  without the stable provider request identity.

### 6. Tests Required

- Unit tests cover each stable reason and assert source, call count, failing
  index, field shapes, and absence of argument canaries.
- PostgreSQL Provider integration tests exercise native and structured-sidecar
  failures and assert one failed model run with `error_code=tool_call_invalid`
  plus the safe response metadata.
- Regression tests verify successful native/sidecar normalization and confirm
  malformed calls never reach Capability execution.

### 7. Wrong vs Correct

#### Wrong

```go
p.recordProviderFailure(ctx, assignment, role, correlationID, messages, "tool_call_invalid")
```

#### Correct

```go
diagnostic := providerToolCallNormalizationDiagnostic(rawCalls, "native", err)
p.recordProviderFailure(ctx, assignment, role, correlationID, messages,
    "tool_call_invalid", diagnostic)
```

## Scenario: Compact Provider Cognition Context

### 1. Scope / Trigger

- Trigger: a cognition, wake-up, daily-review, reflection, native-cognition,
  or action-realization call serializes a `ContextProjection` for the Provider.
- The full projection remains an internal/replay value; the Provider receives
  a role-facing semantic projection only.

### 2. Signatures

```text
compactCognitionContext(ContextProjection) -> ProviderContext
```

`compactCognitionContext` is an internal/replay-safe DTO. The final Provider
input is produced by `ProviderContextSurface` and is narrower than that DTO;
it does not expose database identifiers or state-machine bookkeeping fields.

### 3. Contracts

- `core_persona` is the only place for identity, personality,
  behavioral-policy, and life-profile data; parallel aliases are omitted.
- `current_state.data` is the only place for `inner_state` and `life_context`;
  revision, persistence timestamps, and numeric decay-control parameters stay
  in Core.
- `life_context.current_time` is the current local wall-clock time formatted
  with the Fluctlight's canonical IANA `timezone`; it is the semantic time fact
  used by wake-up, cognition, reply, daily-review, and reflection decisions.
  The raw RFC3339 `instant` used for Core snapshots is not Provider input.
- `visual_identity` in ordinary conversation/wake-up cognition is limited to
  availability/missing state. Renderer constraints, identity snapshots,
  workflow timeline stages, asset lists, and adapter/revision metadata stay in
  Core/media workflows. Media prompt input uses its separate bounded projection;
  reference asset IDs remain only in the durable Core concept for renderer
  lookup.
- `recent_messages` keeps semantic order/kind/text/time, but omits message IDs,
  `author_actor_id`, `source`, and attachment arrays.
- Memory input keeps type/content/confidence/importance/emotional significance,
  creation time, and an opaque `ContextReference` `ref` only when it resolves
  in the frozen index. Raw Memory IDs, evidence IDs, storage status, revision,
  visibility, conversation foreign keys, and source/event IDs stay in Core.
- Empty optional collections are omitted. Native Provider `tools` remains the
  sole complete capability schema; `context.capabilities` is never duplicated.
- The current conversation message is carried once by its owning operation;
  `recent_messages` excludes that newest user entry when it is already present
  as the operation input.
- The full `ContextProjection` may still be persisted inside a frozen decision
  for replay; compaction only affects Provider-facing user content.

Action realization, reflection, and native cognition also use operation-owned
semantic projections: realization receives a compact response plan and only
tool outcome fields, reflection evidence uses a short `sequence:N`
`evidence_ref` plus a metadata-free payload, and native cognition receives a
metadata-free fact. Frozen Core records retain the complete protocol objects.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| canonical Core Persona envelope is missing but legacy parallel fields exist | reconstruct one canonical envelope before serialization; do not duplicate aliases |
| optional evidence collection is empty | omit the field from Provider content |
| optional evidence collection is non-empty | preserve its semantic values and required evidence references |
| native tools are present | omit full capability manifests from user context |
| persisted/replay projection contains old full fields | continue deserializing it; compact only at the Provider boundary |

### 5. Good/Base/Bad Cases

- Good: wake-up receives one three-layer context and native tools once, with no
  random database IDs or duplicate persona envelope.
- Base: a legacy instance with only parallel identity fields is reconstructed
  into the canonical Core Persona before the model call.
- Bad: send `persona_profile` beside `context`, repeat tools under
  `context.capabilities`, or let model input expose `message_<random-id>` and
  state revision bookkeeping.

### 6. Tests Required

- Assert compact context contains one canonical three-layer shape and omits
  all parallel aliases, database IDs, revisions, timestamps, and empty lists.
- Assert the Provider projection keeps the local `current_time` and canonical
  `timezone` while omitting the raw Core `instant`.
- Assert Visual Identity workflow timeline/asset metadata is absent from
  cognition and media prompt projections while renderer constraints remain.
- Assert realization plans/results and reflection/native evidence facts keep
  semantic outcomes while omitting protocol IDs, schemas, revisions, and
  transport metadata.
- Assert non-empty recent messages and memories retain semantic values; typed
  evolution slots are available only on surfaces that explicitly need them,
  and no raw evidence/identifier field crosses the ordinary conversation
  boundary.
- Assert legacy projection reconstruction does not lose Core Persona data.
- Assert every Provider system payload still has one leading system message;
  this context compaction must not alter native tools or persisted decisions.

## Scenario: Operation-Owned Provider Context Surfaces

### 1. Scope / Trigger

- Trigger: the same `ContextProjection` is used by Main conversation,
  takeover reply, wake-up, daily review, native cognition, persistent-switch
  assessment, and Reflection.
- A broad internal projection must not be promoted wholesale into every
  `[RUNTIME CONTEXT]` message. This boundary also prevents invalid database
  IDs, revisions, authority/persistence metadata, and duplicate actor/drive/
  presence copies from becoming model input.

### 2. Signatures

```go
type ProviderContextSurface string

compactCognitionContextForSurface(ContextProjection, ProviderContextSurface) map[string]any
workingMemoryInputFromProjectionForSurface(ContextProjection, ProviderContextSurface, active, summaries) WorkingMemoryInput
App.assembleProjectionPromptForSurface(ctx, surface, projection, role, rules, input, tools, schemaName, schema)
```

### 3. Contracts

- `ContextProjection` remains the complete Core read model. Surface filtering is
  applied only before Working Memory fragments are built; capability snapshots,
  frozen actions, and CAS validation keep the full projection.
- `conversation_main`, `takeover_reply`, and
  `persistent_switch_assessment` retain semantic current state, local time and
  timezone, current speaker, relationship, schedule, agency, outcome, memory,
  and recent conversation facts. They omit actor rosters, top-level presence,
  duplicate drive slots, preference/trigger slots, raw hypotheses, and visual
  renderer constraints.
- `native_cognition` and `reflection` do not receive interactive recent
  history or conversation summaries unless a future surface explicitly opts
  in. Wake-up and daily review retain their operation-owned schedule/memory
  context.
- Current state is projected as mood/PAD/momentum/drive/regulation and
  scene/activity/location/current local time/timezone. `ref`, `*_id`, revision,
  `authority`, `source`, persistence timestamps, and schedule/presence linkage
  fields are Core-only in the ordinary surface.
- Actor objects use display/type/role/label. An arbitrary actor/database ID is
  never a Provider fact; only the explicit `actor_user`/`actor_self` aliases
  may be used as display aliases.
- Runtime summaries contain semantic `summary` text only. `source_digest`,
  source kind, sequence bounds, completion timestamps, and summary row IDs are
  retrieval metadata and do not enter the wire context.
- Memory and capability target refs are retained only when they resolve in the
  frozen `ContextReferenceIndex`; an opaque-looking but unresolved `ref` is
  dropped. Approximate content similarity is not used as a Core-side merge or
  prompt dedupe rule.
- Conversation retrieval uses current input, recent topic, and active-memory
  cues. Foundation/state/goal cues remain available in the prompt but do not
  make every old episodic row a conversation search hit. Interactive retrieval
  is capped at six durable results; duplicate lexical query tokens count once.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown operation surface | Use the compatibility/default projection only at internal test seams; production callers pass an explicit surface. |
| `id`, `*_id`, revision, persistence, source, authority, or unresolved `ref` appears in ordinary Runtime Context | Drop the field; never stringify it into a semantic fact. |
| Current speaker lacks a display name and has an arbitrary ref | Keep only type/role/label; do not expose the arbitrary ref. |
| Summary has no non-empty semantic text | Omit the summary fragment. |
| Native cognition/Reflection projection has interactive history or summaries | Suppress those fragments unless the surface policy changes explicitly. |
| Conversation memory query repeats the same lexical token | Score the token once; do not amplify relevance by repetition. |

### 5. Good / Base / Bad Cases

- Good: a conversation prompt retains “工作室 / 剪假发 / Asia/Shanghai / 当前
  关系” and a few relevant memories, while the same prompt has no
  `actor_id`, `event_ref`, `expected_revision`, renderer adapter data, or
  repeated drive/presence roster.
- Base: Core still keeps the full projection and reference index for a tool
  call; only the Provider-facing Runtime Context is reduced.
- Bad: loop over every key in `compactCognitionContext` and turn it into a
  Runtime Fact, or assume a field is valid merely because its name is `ref` or
  `*_id`.

### 6. Tests Required

- Surface fixture with current state, life context, actors, duplicate drives,
  top-level/nested presence, visual renderer constraints, invalid IDs, and
  summary metadata; assert field paths in the final `[RUNTIME CONTEXT]` JSON.
- Assert valid `ContextReference` refs survive only on surfaces that need them,
  while unresolved refs and raw entity IDs are absent.
- Assert native cognition/Reflection omit recent messages/summaries and Main
  retains current time, scene, relationship and bounded memory facts.
- Assert conversation retrieval limits durable results and repeated query
  tokens do not increase lexical overlap scoring.
- Run a Provider wire smoke with a real `ContextProjection` when a live
  Provider is configured; do not treat a fake payload as live-token evidence.

### 7. Wrong vs Correct

#### Wrong

```go
for key, value := range compactCognitionContext(projection) {
	input.RuntimeFacts = append(input.RuntimeFacts, fact(key, value))
}
```

#### Correct

```go
compact := compactCognitionContextForSurface(projection, surface)
input := workingMemoryInputFromProjectionForSurface(projection, surface, active, summaries)
```

## Scenario: B-Layout Prompt Assembly and Final Wire Budget

### 1. Scope / Trigger

- Trigger: a Main cognition, Reflection, Summary, or query continuation sends
  non-media messages to an OpenAI-compatible Provider.
- The assembler changes only Provider-facing selection, roles, formatting, and
  budget enforcement. Retrieval, semantic ranking, Tool execution, workflow
  state, and frozen replay remain outside it.

### 2. Signatures

```go
ResolveWorkingMemory(input, policy) (WorkingMemory, error)
AssemblePromptContext(PromptAssemblyInput) (PromptAssemblyResult, error)
App.assembleProjectionPrompt(ctx, role, rules, projection, input, tools, schema)
ProviderClient.StructuredAssembledWithToolsSchema(ctx, role, messages, tools, schema)
```

`model_roles` persists `token_budget` as output reserve plus
`context_window_tokens`, `max_input_tokens`, and
`prompt_budget_policy_version`.

### 3. Contracts

- Ordinary system content contains exactly two top-level sections: `# 运行协议`
  and `# 人格设定`. Role-specific operation rules are nested under the run
  protocol; they are never represented as Core Persona fields.
- `# 人格设定` contains only semantic Core Persona groups:
  `identity`, `personality`, `behavioral_policy`, and `life_profile`.
  `schema_version`, any internal ID/revision, persistence timestamps,
  foreign keys, transport metadata, and automatic-evolution control fields do
  not enter the prompt.
- The production B-layout is one leading system message, an optional delimited
  `[RUNTIME CONTEXT]` user message containing dynamic facts, selected real-role
  recent user/assistant messages in chronological order, and the current input
  exactly once as the final user message. Dynamic relationship, state, scene,
  Active/Long-term Memory, and Summary content never becomes system policy.
- Runtime Context has distinct `facts`, `active_memory`, `retrieved_memory`,
  and `conversation_summaries` keys. Recent selection uses complete messages/
  turns; it is not rendered as one synthetic user-history table.
- Memory `created_at` and a valid opaque `ContextReference` are semantic
  grounding fields and may remain; raw storage IDs, evidence IDs, revision,
  status/FK/audit fields do not. An arbitrary `ref` is not provider-safe just
  because it looks like an identifier.
- Recent messages retain role/kind, semantic time, content, and order. The
  current operation input is not duplicated in recent history.
- The conservative estimator is
  `ceil(max(ceil(utf8_bytes/3), unicode_runes) * 1.25)` plus message/final-wire
  overhead. Tools and response schema count toward input.
- Defaults are context `65536`, max input `49152`, output reserve `4096`, safety
  margin `4096`, and policy `prompt-budget.v1`, leaving `8192` headroom. System
  and current input each cap at `8192`/`16384`; Tools plus response schema cap at
  `16384`. Required overflow returns `prompt_required_budget_exceeded` before
  network I/O; optional items are dropped whole by priority.
- Every non-media structured and streaming request executes a final wire
  estimate. `max_tokens` receives output reserve only; it is not input budget.
- A continuation reuses frozen B-layout messages, appends exactly one assistant
  tool-call envelope and 1–2 matching `role=tool` results, sends no Tools, and
  accepts only closed `{visible_text}` output. Normal historical assistant
  messages remain valid before that terminal envelope. A native-tool response
  that omits the structured sidecar may normalize its missing response mode only
  when the adapter marked `StructuredFallback`, visible text is empty, and all
  1–2 calls pass the generic pure-query registry gate. Non-fallback schema
  omission, ACTION, mixed, visible, empty, and over-limit results remain final/
  invalid; reasoning content is never treated as visible text.
- `media_prompt`, `media_quality_acceptance`, and Visual Identity media calls
  retain their existing English/YAML/multimodal path and do not enter this
  ordinary composer.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Core Persona exists in `context` or `context_projection` | Lift it once into system and remove the dynamic duplicate. |
| No Core Persona exists (initialization/legacy payload) | Keep protocol and explicit empty initialization/persona state; never invent fixed traits. |
| Metadata field is an internal ID/revision/FK/status | Omit from Provider content. |
| Memory/developing-self evidence reference or memory semantic creation time | Preserve in the relevant section. |
| Current time is missing/malformed | Core supplies the canonical local-time fallback; raw `instant` remains internal. |
| Required system/current/tools/schema cost exceeds section or total input cap | Return `prompt_required_budget_exceeded`; send no Provider request and never tail-truncate. |
| Optional whole item exceeds section/total/final-wire budget | Drop it with `section_cap`, `total_cap`, or `total_cap_final_wire` trace reason. |
| Role budget violates `max_input + output_reserve + 4096 <= context_window` or uses an unknown policy | Reject configuration as `provider_prompt_budget_invalid` or `prompt_budget_policy_unknown`. |
| Preassembled messages have multiple/late system roles, empty content, or non-user final input | Reject as `provider_assembled_messages_invalid`. |
| Continuation contains ACTION/mixed calls, multiple assistant tool envelopes, unmatched results, Tools, or a second tool request | Reject the continuation; perform no assistant settlement. |
| Native tool response omits structured sidecar | Infer continuation only for `StructuredFallback` + empty visible text + 1–2 generic pure queries; otherwise preserve final/fail-closed behavior. |
| Media role reaches composer | Preserve media-specific formatter/instructions; ordinary protocol is not injected. |

### 5. Good / Base / Bad Cases

- Good: one system contains protocol plus filtered Core Persona; a separate
  Runtime Context user message contains current dynamic facts; recent messages
  keep real roles; current input appears once and last; the complete wire
  remains at or below the persisted max input.
- Good: a pure-query continuation with ordinary assistant history appends one
  canonical tool-call assistant message and matching bounded results, without a
  Tools catalog or another mutation schema.
- Base: a legacy projection has parallel identity fields; the composer rebuilds
  one Core Persona envelope and keeps the dynamic context readable.
- Bad: put `schema_version` or persona IDs in the system, delete all
  `evidence_refs`, duplicate current input, exclude Tools/schema from budget,
  truncate half a turn, or treat every assistant history message as tool use.

### 6. Tests Required

- Assert every ordinary role emits one leading system with the fixed section
  order and operation rules inside `# 运行协议`.
- Assert Core Persona filtering removes internal metadata and retains all four
  initialized semantic groups; initialization without a persona does not get
  synthetic traits.
- Assert Runtime Context delimiters/fact keys, current time/timezone,
  current-user deduplication, real recent roles/order, Memory opaque refs, and
  Developing Self evidence refs.
- Assert whole-fragment/whole-turn selection, cross-source dedupe, exact default
  role budgets, estimator formula, section/final-wire drops, required overflow,
  and output reserve mapped to `max_tokens`.
- Assert continuation accepts ordinary historical assistant messages but only
  one terminal assistant tool-call envelope, 1–2 matching results, no Tools, and
  visible-text-only output.
- Assert media prompt/quality/Visual Identity payloads remain outside the
  ordinary composer and preserve their language/format behavior.
- Assert frozen realization still uses its captured projection and provider
  schema/tool payloads remain unchanged.

### 7. Wrong vs Correct

#### Wrong

```go
messages := []ProviderMessage{{Role: "system", Content: rules + dynamicState},
	{Role: "user", Content: allHistory + currentInput}}
provider.Send(messages, tools) // no whole-wire budget; current input duplicated
```

#### Correct

```go
working, err := ResolveWorkingMemory(input, policy)
assembled, err := AssemblePromptContext(PromptAssemblyInput{
	OperationRules: rules, CorePersona: filteredPersona,
	WorkingMemory: working, CurrentInput: currentInput,
	Tools: tools, ResponseSchema: schema, Budget: roleBudget,
})
completion, err := provider.StructuredAssembledWithToolsSchema(
	ctx, role, assembled.Messages, tools, schema)
```

## Scenario: Surface-Aware Bounded ADK Structured Tasks

### 1. Scope / Trigger

- Trigger: a conversation Main/Takeover-B turn or the WakeUp model decision
  needs the existing Eino ADK model→Capability→ToolResult→model protocol.
- The request is operation-scoped. It carries only identity, correlation,
  surface and a read-only context projection; it never exposes `*App`, a
  repository or a transaction to the ADK tool adapter.

### 2. Contracts

- The explicit ADK loop allowlist is `conversation_turn_response`,
  `takeover_reply_response`, and `wake_up_response`.
- The loop has at most two model generations, has no hidden retry/fallback,
  preserves the Provider tool-call ID, and returns typed errors for model,
  tool, cancellation, empty-final and iteration-limit failures.
- WakeUp definitions are taken from
  `CapabilityRegistry.Catalog(CapabilitySurfaceWakeUp)` and are checked against
  the canonical Registry definition before Provider I/O. Unknown, duplicate,
  mismatched, `InternalOnly`, and cross-surface definitions fail closed.
- Pure query capabilities may return a bounded result to the next ADK request.
  Deferred, mutation and external capabilities return a pending/deferred
  result only; the ADK callback never commits a transaction or publishes an
  output. Existing freeze, policy, intent/outbox and action-worker boundaries
  remain authoritative.
- Daily Review, Native Cognition, Reflection and query/Judge schemas remain
  single-task/no-feedback paths until an explicit feedback contract is added;
  a capability catalog alone does not opt a schema into ADK.

### 3. Tests Required

- Assert WakeUp enters the same Eino ADK loop used by conversation, preserves
  formal call/result pairing and correlation, and bounds the loop to two
  generations.
- Assert model/tool failure and cancellation do not fabricate a final result;
  WakeUp no-op and deferred output remain valid only after the owning Core
  lifecycle path settles them.
