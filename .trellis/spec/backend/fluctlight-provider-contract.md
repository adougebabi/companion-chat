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

## Scenario: Shared native Agent execution

### 1. Scope / Trigger

Every complete non-embedding task uses a registered formal Agent and the pinned
Eino v0.7.37 ChatModelAgent/Runner. Embedding retains its separate queue/API.

### 2. Signatures

```go
App.RunConversationCognitionAgent(ctx, ConversationCognitionAgentInput)
App.RunVisualIdentityAgent(ctx, VisualIdentityAgentInput)
App.ExecuteTool(ctx, ToolExecutionRequest) (ToolExecutionReceipt, error)
RunADKLoop(ctx, ADKLoopConfig, []*schema.Message)
```

### 3. Contracts

- Agent definitions own task prompt, typed context, model role, Tools and final
  decoder. Existing typed Task methods delegate to those definitions; no
  single-call schema allowlist bypass remains.
- The shared native loop consumes actual Tool receipts and continues after
  reads, writes and business rejection. MaxIterations, timeout and cancellation
  are failure guards, never successful completion conditions. No two-round or
  query-only limit and no write-Tool early stop exists.
- Tools call the same `ExecuteTool` boundary as independent callers. Mutations
  commit their domain effect, receipt and outbox in a short local transaction.
  `accepted` means a durable asynchronous task exists, not that media is ready.
- Each physical Generate/Stream obtains its own queue lease, cancellation,
  request identity, diagnostics and budget checks. No lease or transaction
  spans the decision loop.
- Native IDs are preserved. Direct operations use separate business operation
  identities and never invent native ToolCall or Provider request facts.
- Surface catalogs select default Tools; explicit canonical Tool installation
  is allowed. Resource ownership, authorization and business conditions still
  apply, but surface/source/Agent names are not execution gates.
- Final output is validated only after native loop completion. JSON-looking
  text is valid for a text-output Agent. Invalid final DTOs fail without using
  an earlier message; an empty visible field never publishes the raw DTO.
- `agent_runs` fences whole-run replay. Later model error/cancellation preserves
  committed Tool effects and partial facts; no caller re-executes trace calls.
- Production streaming uses the native streaming Runner. Publication uses the
  existing message/outbox service, including Tool publication and natural final
  output. A committed explicit reply is not published twice.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing role/model or unsupported capability | Fail before model I/O |
| Missing native ID, malformed arguments, unknown Tool | Fail closed; no invented identity |
| Tool business rejection | Return explicit receipt to the next model decision |
| Dependency error, cancellation or iteration exhaustion | Error with committed facts retained |
| Same operation with changed payload/target/profile | Conflict, no new side effect |
| Model fails after Tool commit | Failed run; replay cannot rerun the whole loop |

### 5. Good/Base/Bad Cases

- Good: write Memory, recall the committed row, then produce a final answer.
- Base: a no-tool Agent completes in one model decision.
- Bad: return deferred for every mutation, fabricate IDs or count exhaustion as success.

### 6. Tests Required

- Fixed 17-Agent and 18-Tool catalogs; every Tool direct and native adapter path.
- Multi-round, same-round multiple calls, write/read, business rejection, stream,
  cancellation, final-schema failure and committed-effect recovery.
- Real Provider Agent evidence is separate from controlled HTTP regressions.
- Disconnect feedback and skip actual writes in test-only subprocess probes;
  the corresponding assertions must fail, then pass with production behavior.

### 7. Wrong vs Correct

Wrong: freeze native calls for later caller execution and end after two models.
Correct: native Runner → `ExecuteTool` → committed receipt → next model decision.

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
  action-only response with no `conversation.reply`. Every valid native call is
  executed through the same independent Tool boundary;
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
- Thinking is operation-owned and never becomes visible output or Tool execution
  authority. Final structured output must satisfy the Agent contract; no
  reasoning/prose fallback supplies missing native calls or a failed final DTO.
- Native cognition may complete a tool-only task with an explicit legal final
  DTO and committed receipts. It does not fabricate semantic state from calls.
  The native cognition depth guard still bounds recursive life facts.
- The Provider boundary emits at most one `system` message, and it must be
  the first message. Operation, context-authority, and language instructions
  are concatenated in caller order; `user`/`assistant` history keeps its order
  after that merged system message. This prevents strict chat templates such
  as mlx-serve from rejecting a late or repeated system role.
- API keys are resolved only in Go Core through the configuration secret contract and never returned to the browser boundary, browser/debug output.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Role has no endpoint/model assignment | Role unavailable with explicit configuration error; no fallback. |
| Structured role returns an empty/mismatched transport shape | Normalize only the affected fields (missing → typed empty, object ↔ array container repair), preserve native tool calls independently, and let the owning domain validator decide whether the resulting semantic payload is usable; never parse arbitrary prose. |
| One native call entry is malformed while sibling entries are valid | Keep the valid entries in the normalized completion, record a bounded diagnostic for the malformed entry, and never execute the malformed entry or discard its valid siblings. |
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
  to canonical capabilities with non-empty reply text, and that Core loads
  the committed reply exactly once without `cognition_visible_text_missing`.
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

## Scenario: Bounded native Tool diagnostics

- Only `schema.Message.ToolCalls` authorizes execution. Body, reasoning and
  structured `tool_calls` fields never create calls.
- Missing IDs fail as `tool_call_invalid`; never derive `call_derived_*`.
- Diagnostics retain bounded shape/reason metadata, physical request identity
  and formal correlation, not arguments, hidden reasoning or credentials.
- Invalid siblings never execute; previously committed valid calls remain facts.
- Cancellation/timeout terminal writes retain scenario/attempt values through
  bounded `context.WithoutCancel`; first-terminal-wins remains authoritative.
- Tests assert malformed call rejection, no argument canary leaks, physical
  request/Tool/result association and cancelled-run diagnostic persistence.

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

- Trigger: a formal cognition, Reflection, Summary, or later native Agent round sends
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
- Later native rounds retain the full assistant ToolCall/result association and
  installed Tools. Writes, mixed calls and repeated queries are legal; the
  final Agent schema applies only to the terminal assistant output.
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
| Media role reaches composer | Preserve media-specific formatter/instructions; ordinary protocol is not injected. |

### 5. Good / Base / Bad Cases

- Good: one system contains protocol plus filtered Core Persona; a separate
  Runtime Context user message contains current dynamic facts; recent messages
  keep real roles; current input appears once and last; the complete wire
  remains at or below the persisted max input.
- Good: a later native round sees every matching Tool result and can select
  another installed Tool before final output.
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
- Assert later native rounds preserve historical messages, matching Tool
  results, installed Tools and final output validation.
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

## Scenario: Task-Owned Model Operation Contracts

### 1. Scope / Trigger

- Trigger: any non-embedding domain operation needs a model call, including
  media prompt/quality, Visual Identity, conversation summary, schedule
  generation/replan, initialization, Native Cognition, Daily Review,
  Reflection, or persistent-switch assessment.
- The operation-owned formal Agent (exposed by typed Task methods) is the layer that selects prompt fragments,
  combines them with bounded business facts, selects the response schema,
  invokes Eino, and normalizes the result. Domain code owns authorization,
  semantic validation, freeze, prepare, transaction and settlement.

### 2. Signatures

```go
RunMediaPromptTask(ctx, MediaPromptTaskInput) (string, error)
RunMediaQualityTask(ctx, MediaQualityTaskInput) (MediaQualityAcceptance, error)
RunReflectionProposalTask(ctx, ReflectionProposalTaskInput) (ProjectionTaskResult, error)
RunNativeCognitionTask(ctx, NativeCognitionTaskInput) (ProjectionTaskResult, error)
RunScheduleGenerationTask(ctx, ScheduleGenerationTaskInput) (map[string]any, error)
RunFrozenEmbeddingTask(ctx, text, frozenAssignment) (modelID string, vector []float64, error)
```

`ProjectionTaskResult` contains the normalized Provider completion, the
assembled projection used by the request, and bounded prompt diagnostics. The
ADK bridge accepts one `PromptAssemblyResult`; it does not expose independent
raw `Messages`/`Schema` forwarding fields.

### 3. Contracts

- Task inputs are typed operation facts. A caller does not pass a complete
  system prompt, arbitrary provider message list, response schema or tool
  catalog to a generic forwarding wrapper.
- Each Task owns its instruction constants, selected slots, schema and output
  decoder. Shared Eino/queue/diagnostic support remains below the Task; no
  second Provider/Composer/Agent engine is introduced.
- Projection-backed Tasks select one explicit `ProviderContextSurface`, build
  the operation rules and capability catalog, and return the exact assembled
  projection for later domain validation. Installed Tools own their commits;
  callers never replay those effects.
- Media, Visual Identity and summary Tasks keep their existing multimodal or
  language-specific format and do not enter the ordinary cognition composer.
- Embedding remains an independent contract. A durable retry uses its frozen
  endpoint/model assignment and cannot silently re-resolve a role.

### 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Task input missing required business facts | Return a typed task validation error before Provider I/O. |
| Generic caller attempts to supply arbitrary messages/schema | No generic forwarding API exists; compile-time boundary prevents the path. |
| Prompt assembly violates section/total budget | Return `prompt_required_budget_exceeded`; send no request. |
| Provider structured output is malformed | Task returns the bounded provider parse/normalization error; domain retry policy decides whether to retry. |
| Task result is semantically invalid for the domain | Domain validator rejects it; Task never fabricates a fallback. |
| Frozen embedding assignment is unavailable on retry | Return the explicit assignment error; never switch models implicitly. |

### 5. Good / Base / Bad Cases

- Good: `RunReflectionProposalTask` receives evidence and a projection,
  selects the Reflection surface/schema, and returns a normalized proposal for
  Core's evolution compiler.
- Base: `RunScheduleGenerationTask` retries with the same typed facts and a
  bounded compact-output reminder; schedule continuity remains domain-owned.
- Bad: `RunStructuredTask(ctx, task, messages, schema)` lets every caller
  inject a different system prompt, schema and raw user payload while the
  supposed Task only forwards them.

### 6. Tests Required

- Assert each production model entry uses its concrete Task boundary and no
  deleted generic wrapper remains in production callers.
- Assert media/quality/Visual Identity/summary/schedule Tasks select their own
  instruction and schema and return bounded decoded values.
- Assert projection Tasks preserve surface isolation, selected slots, tool
  catalog, output budget and frozen projection identity.
- Assert ADK bridge rejects invalid assembled PromptAssemblyResult before
  Provider I/O and preserves request-scoped capability trace.
- Assert a frozen embedding retry sends the original endpoint/model tuple.

### 7. Wrong vs Correct

#### Wrong

```go
func RunStructuredTask(ctx context.Context, task ModelTask,
    messages []map[string]any, schema map[string]any) (map[string]any, error) {
    return provider.StructuredWithSchema(ctx, task.Role, messages, task.SchemaName, schema, false)
}
```

#### Correct

```go
func (a *App) RunReflectionProposalTask(ctx context.Context,
    input ReflectionProposalTaskInput) (ProjectionTaskResult, error) {
    // select reflection surface/rules/schema, assemble bounded projection,
    // invoke the shared Eino runtime, and return the normalized proposal.
}
```

## Scenario: Native Eino ToolCall Authority And Fail-Closed ADK Projection

### 1. Scope / Trigger

- Trigger: an Eino `schema.Message` contains native ToolCalls, an ADK Runner
  returns AgentEvents, or a fixed structured Task returns a JSON sidecar that
  also contains a `tool_calls` field.

### 2. Signatures

```go
normalizeEinoNativeToolCalls(*schema.Message, providerRequestID)
normalizeEinoNativeToolCallsIndependently(*schema.Message, providerRequestID)
RunADKLoop(ctx, ADKLoopConfig, []*schema.Message)
```

### 3. Contracts

- ADK and fixed-task model ToolCalls enter the Core contract only from
  `schema.Message.ToolCalls`; accepted calls retain the model's formal ID,
  name and object arguments.
- Missing/conflicting/duplicate IDs, invalid names, malformed arguments and
  unknown tools fail closed. No ID may be derived from request ID, position,
  capability name, prose, Markdown or reasoning.
- A malformed sibling may produce a bounded diagnostic while valid typed
  siblings remain available; the malformed sibling never executes.
- A final structured body can be decoded for the Agent DTO, but
  its `tool_calls` field is never a second execution authority on an ADK
  completion. The Provider must not normalize the same ADK call twice.
- `RunADKLoop` propagates `ErrExceedMaxIterations`, cancellation and model/tool
  errors. It may project AgentEvents and request-scoped trace, but it may not
  swallow an upper-limit error as success or run another model/tool loop.

### 4. Validation & Error Matrix

| Condition | Result |
|---|---|
| Native call has no ID or conflicting identity fields | bounded `tool_call_invalid`; no invocation |
| Native arguments are not a JSON object | bounded `arguments_not_object`/`arguments_invalid_json`; no invocation |
| Valid and malformed native siblings share a response | valid sibling retained; malformed sibling rejected and diagnosed |
| Sidecar repeats a native ADK call | native Eino call remains sole authority; sidecar tool call ignored |
| ADK reaches max iterations | typed failure; no fabricated success |

### 5. Good/Base/Bad Cases

- Good: `call-7` appears unchanged in the assistant ToolCall, invoker trace,
  tool result and next model input.
- Base: a fixed Task parses a strict DTO while a sidecar `tool_calls` field is
  ignored for execution.
- Bad: derive `call_derived_*`, scan visible prose for a tool, or use the last
  non-empty assistant text after a failed/empty final event.

### 6. Tests Required

- Native formal identity, malformed sibling, sidecar conflict, tool feedback,
  model/tool failure, cancellation and iteration-limit tests.
- Production Conversation/WakeUp/Takeover Runner tests must assert the actual
  next input contains the matching assistant/tool pair and that only the
  existing settlement boundary publishes.

### 7. Wrong vs Correct

#### Wrong

```go
id := "call_derived_" + stableDigest(providerRequestID+name+arguments)
```

#### Correct

```go
id := toolCall.ID // copied from Eino schema.Message.ToolCalls
```

### Formal request projection boundary (0037)

Every formal task reaches a read-only source-map and physical-budget check immediately before Provider egress. Main, WakeUp, Native, and Daily reproject authorized current System/Runtime context after a successful mutating Tool; the outbound message copy changes, while original Eino messages, multimodal parts, ToolCall IDs, and matching ToolResults remain intact. The final settlement follows the first context generation through committed Tool receipts. Diagnostics identify source versions, budget parts, omitted categories, and the final physical request mapping. The current budget estimator is a rune heuristic and must be labeled as an estimate, not exact tokenizer usage.

The native Tool adapter also passes the Core-owned snapshot from the same projection to `ExecuteTool` for contextful calls. A Tool that resolves an opaque reference must see the exact map exposed on its preceding physical model request. `ExecuteTool` validates snapshot identity and accepts this path only for a native ToolCall with a Provider request ID; independent direct Tools continue without a caller-supplied snapshot.

Provider claim `evidence_refs` use only the opaque references exposed in Runtime Context. Before persistence, Core resolves an exact entry through its frozen `ContextReferenceIndex`; current life/state references become the turn's source fact anchor, and durable entity references become authorized internal IDs. Each normalized claim must carry Core's private proof that every submitted ref came from an exact visible token. An unknown token, raw entity ID, or model-supplied imitation of that proof remains invalid. Do not validate a model-visible `kind:ctx_...` token against an allowlist of raw database IDs.

Current body and wearing also carry an `appearance:ctx_...` reference in the compact Runtime view. This token is generated from the same effective snapshot without its read timestamp and is valid for claims and influences about current appearance; the underlying body and wardrobe tables remain authoritative. Wardrobe item IDs are business Tool arguments, not context references, and must never be joined to a `wardrobe:ctx_` prefix.
