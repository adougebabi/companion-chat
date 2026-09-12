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

```python
preflight(role: ModelRole) -> CapabilityReport
complete_structured(role, schema, input) -> StructuredResult
stream_realization(role, input) -> AsyncIterator[ProviderChunk]
embed(role, inputs) -> VersionedEmbeddings
```

### 3. Contracts

- Generative roles may share one endpoint/model, but remain independent settings with independent budgets and provenance.
- `initialization`, `cognitive_assessment`, and `reflection` require strict structured-output/schema preflight.
- `action_realization` requires streaming, abort propagation, bounded diagnostics, and correct UTF-8/chunk handling.
- `embedding` requires an embedding endpoint and fixed dimensions recorded with each vector/index version.
- `media_prompt` requires its declared structured/text output contract and cannot execute media generation itself.
- `media_prompt` may serve both B text prompt generation and C structured multimodal acceptance using the same configured role/model; C transport, vision-capability, timeout, or schema failures are surfaced to the owning media flow as an infrastructure condition, never inferred as a content `pass` or `reject`.
- Settings cannot activate a role until preflight proves required capabilities. Health may later degrade without making Core readiness false.
- Every result records role, endpoint/model ID, capability/model version when available, prompt/schema version, timing, token usage/budget, and correlation IDs.
- No implicit role/model fallback. Failure follows explicit interaction/workflow retry/deferred/no-op/terminal rules.
- Provider adapter returns normalized transport/structured results and bounded parse diagnostics. It does not parse visible prose for semantic effects or choose domain actions.
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
| Realization role lacks streaming/abort | Preflight fails; role cannot activate. |
| Embedding dimensions change unexpectedly | Reject vectors, mark role/index mismatch, require new embedding version. |
| Timeout/token budget exceeded | Cancel/bound result and follow owning retry/terminal policy. |
| Provider/model is temporarily unavailable | Report degraded role health; request/workflow handles explicit failure. |
| API key decryption fails | Configuration error; do not use env/old-key fallback. |
| Provider returns hidden reasoning/raw diagnostics | Bound/redact and keep out of ordinary result/trace/browser contract. |

### 5. Good / Base / Bad Cases

- Good: one local chat model passes five role preflights with separate budgets; every artifact records its actual role/model/prompt version.
- Good: an embedding model upgrade creates a new dimension/model index and background rebuild without mixing distances.
- Base: reflection role is degraded while realization remains healthy; interactions continue, reflection workflows retry explicitly.
- Bad: one global model string with unknown capabilities, silently substitute realization for assessment, parse malformed structured output as prose, or hide fallback under Provider adapter logic.

### 6. Tests Required

- Endpoint/settings tests for encrypted keys, safe summaries, role assignment, shared model mapping, and atomic invalid-patch rollback.
- Role-specific preflight tests for structured schema, stream/abort/chunking, embedding dimensions, media-prompt output, timeout, and token budgets.
- Provenance tests assert every result stores role/endpoint/model/prompt/schema/correlation metadata without credentials or hidden reasoning.
- Failure tests prove one degraded role does not silently use another and follows owning interaction/workflow policy.
- Provider adapter contract suite runs against fake normalized adapters and configured OpenAI-compatible test endpoints.
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

`ProviderContext` keeps the canonical `core_persona`, `developing_self`, and
`current_state` layers plus non-empty evidence collections. It does not expose
database identifiers or state-machine bookkeeping fields.

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
- `visual_identity` in cognition context is limited to availability and the
  small renderer constraint set needed for a decision. Identity snapshots,
  workflow timeline stages, asset lists, and adapter/revision metadata stay in
  Core/media workflows. Media prompt input uses the same bounded projection;
  reference asset IDs remain only in the durable Core concept for renderer
  lookup.
- `recent_messages` keeps semantic order/kind/text/time, but omits message IDs,
  `author_actor_id`, `source`, and attachment arrays.
- Memory input keeps type/content/confidence/importance/emotional significance,
  creation time, and evidence references; storage status, revision, visibility,
  conversation foreign keys, and duplicate source/event IDs stay in Core.
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
- Assert non-empty recent messages, memories, and typed slots retain semantic
  values and evidence references.
- Assert legacy projection reconstruction does not lose Core Persona data.
- Assert every Provider system payload still has one leading system message;
  this context compaction must not alter native tools or persisted decisions.

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
- Memory `created_at` and evidence references are semantic grounding fields and
  remain; storage IDs, revision/status/FK/audit fields do not. Evidence refs
  are not removed merely because they look like IDs.
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
