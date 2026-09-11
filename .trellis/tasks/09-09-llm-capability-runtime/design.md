# Capability Runtime Target Design

状态：in_progress；基于 `research/current-tool-inventory.md`、`research/prompt-runtime-baseline.md` 和现有 backend contracts。目标架构已进入实现与验证阶段。

## 1. 设计结论

当前代码已有 `CapabilityManifest`、`CapabilityRegistry`、`CapabilityExecutor` 和稳定 Tool Call envelope，但这些名称/对象仍被当作 Tool 执行链使用；上下文仍由 eager `ContextProjection` 或 `App` service locator 提供，主流程还有 capability-name 分支。本次不再增加一套 Legacy Tool Runtime，而是把现有雏形收敛为唯一 Capability Runtime：

```text
Provider native/sidecar shape
  -> CapabilityCodec (唯一协议归一化边界)
  -> CapabilityInvocation
  -> CapabilityRegistry
  -> ContextResolver.Resolve(required slots)
  -> Capability.Execute
  -> CapabilityResult
  -> frozen action / deferred target settlement / next turn persistence
```

普通 conversation 仍然只有一次认知 Provider 调用。CapabilityResult 在本轮持久化并由后续符合产品语义的 cognition 使用，不触发同轮 Main LLM continuation。Capability 内部为了把一个决策级输入编译成领域执行参数，可以调用既有 Provider role 或领域服务，但不得开启第二次 MainAgent 回复回合，也不得把内部规划 schema 发送给 Main LLM 的 `tools` catalog。

## 2. 核心对象与边界

### 2.1 CapabilityDefinition

`CapabilityDefinition` 是唯一对 Provider 渲染的稳定契约，替代当前执行路径中的 `CapabilityManifest` 语义。保留现有稳定名称和 `v1` JSON/schema 版本，避免 frozen action、workflow intent 和 replay 数据失效。

建议字段：

```go
type CapabilityDefinition struct {
    Name              string
    Version           string
    Type              CapabilityType // action | query | internal
    Description       string         // only what the capability does
    InputSchema       map[string]any
    OutputSchema      map[string]any
    Surfaces          []CapabilitySurface
    TargetKinds       []string
    SideEffectClass   string
    ConcurrencyClass  string
    SupportsCancel    bool
    SupportsRetry     bool
    RequiresPreflight bool
    FailurePolicy     CapabilityFailurePolicy
}
```

`Surfaces` replaces call-site name exclusion. A catalog request asks the Registry for a surface (`conversation`, `wake_up`, `autonomy`, `native_cognition`); the runtime does not know any concrete capability name. `Type` is descriptive metadata for audit/policy/discovery and never an execution switch.

### 2.2 CapabilityInvocation

The runtime receives one canonical invocation object. Provider-specific native/sidecar envelopes are decoded into it once; JSON persistence keeps the existing snake_case keys and schema version.

```go
type CapabilityInvocation struct {
    CallID            string
    CapabilityName    string
    Arguments         json.RawMessage
    Intent            string // populated for thin intent-based inputs
    SourceFactID      string
    ActionID          string
    ProviderRequestID string
    Sequence          int
    Metadata          InvocationMetadata
}
```

`InvocationMetadata` is deliberately small: correlation ID, surface, and optional output binding. It does not become a universal bag of domain fields. `ToolCallV1` remains only as a codec/persistence compatibility shape during the implementation; after cutover no executor or MainAgent consumes it directly.

### 2.3 CapabilityContext and ContextSlot

The resolver owns context acquisition. Capabilities never call `App.DB`, global services, or `BuildContextProjection` directly to obtain ambient state.

```go
type ContextSlot string

const (
    SlotCorePersona       ContextSlot = "core_persona"
    SlotCurrentState      ContextSlot = "current_state"
    SlotCurrentLife       ContextSlot = "current_life"
    SlotSchedule          ContextSlot = "schedule"
    SlotVisualIdentity    ContextSlot = "visual_identity"
    SlotAppearance        ContextSlot = "appearance"
    SlotRelationshipScope ContextSlot = "relationship_scope"
    SlotMemoryScope       ContextSlot = "memory_scope"
    SlotAgency            ContextSlot = "agency"
)
```

Use the smallest set of slots supported by actual current dependencies; do not add speculative persona/emotion/goal slots without a consumer. `CapabilityContext` exposes typed getters/fields for these values and serializes a bounded frozen snapshot for replay. Existing JSON/map domain shapes may remain inside the value objects because they are current Core domain representations; callers must not receive a raw `map[string]any` slot bag.

```go
type CapabilityContext struct {
    Identity ContextIdentity
    Persona  *PersonaContext
    State    *CurrentStateContext
    Life     *CurrentLifeContext
    Schedule *ScheduleContext
    Visual   *VisualIdentityContext
    Outfit   *AppearanceContext
    Relation *RelationshipScope
    Memory   *MemoryScope
    Agency   *AgencyContext
}

type ContextResolver interface {
    Resolve(context.Context, ContextRequest, []ContextSlot) (CapabilityContext, error)
}
```

The first implementation uses slot-specific loaders over the existing PostgreSQL read helpers. Shared reads (for example current life plus schedule) are deduplicated inside one resolver request. Authorization, current revision and idempotency checks remain live execution guards even when a cognition-time context snapshot is frozen.

### 2.4 Capability and deferred output extension

The normal capability seam is intentionally narrow:

```go
type Capability interface {
    Definition() CapabilityDefinition
    RequiredContext() []ContextSlot
    Execute(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
```

Capabilities with external configuration implement an optional preflight seam; output-producing asynchronous capabilities implement the deferred extension:

```go
type CapabilityPreflighter interface {
    Preflight(context.Context, CapabilityContext) error
}
```

```go
type DeferredCapability interface {
    Capability
    ExecuteDeferredTx(context.Context, pgx.Tx, CapabilityInvocation,
        CapabilityContext, OutputBinding) (CapabilityResult, error)
}
```

This preserves the existing message/Moment target ordering without making every capability implement a fake preflight or transaction method. Image preflight moves from the current name switch into the capability owner.

### 2.5 CapabilityResult and errors

`CapabilityResult` is the runtime result, converted to the existing persisted/provider-safe result shape at one boundary. It contains identity, status, bounded output, retryability, and correlation metadata; it does not contain a universal domain DTO.

Use small Go sentinels with wrapping:

```go
var (
    ErrCapabilityNotFound   = errors.New("capability not found")
    ErrInvalidArguments     = errors.New("invalid capability arguments")
    ErrContextResolve       = errors.New("capability context resolve failed")
    ErrCapabilityExecution  = errors.New("capability execution failed")
)
```

Error codes used in `CapabilityResult` remain bounded (`capability_not_found`, `invalid_arguments`, `context_resolve_failed`, `execution_failed`, plus existing domain codes). `errors.Is/As` and `%w` are the control mechanism; no exception hierarchy is introduced.

## 3. Target execution flow

1. Provider adapter normalizes native and structured-sidecar calls into `CapabilityInvocation[]`; malformed calls fail at the existing protocol boundary.
2. The scenario asks `Registry.Catalog(surface)` for installed/preflighted definitions in stable name order. No scenario excludes a concrete name.
3. Main cognition freezes the decision and invocation metadata before side effects, preserving the one-call/no-op/deferred semantics.
4. Runtime looks up the capability by invocation name, validates source fact/schema envelope, and resolves only `RequiredContext()` slots. A resolved snapshot is persisted with the frozen action for replay; no capability-specific argument mutation happens in MainAgent.
5. Runtime calls `Preflight` and `Execute` with the caller's context. It keeps sequential call order and existing exclusive/deferred semantics. The request `context.Context` is passed through every IO boundary.
6. Immediate results are validated and persisted. A failed required state-changing capability stops the visible settlement so text that may claim the state changed is not persisted. Optional internal capability failures remain bounded results. Deferred output results stay `deferred` until the caller transaction creates the conversation/Moment target, then `DeferredCapability.ExecuteDeferredTx` runs with the same frozen invocation/context snapshot.
7. Visible action selection is derived from generic output binding/result metadata (`conversation_message`, `moment`, `wake_up`), not `if call.Name == ...`. Tool results are persisted for replay and later cognition, but are not sent back as a second `role=tool` Main LLM message in the same conversation turn.

## 4. Current Tool -> new Capability mapping

Names below remain stable unless the current name is already the canonical product contract. “Thin input” describes the Provider-facing schema after migration, not the internal planner/assembler payload.

| Current Tool | New Capability | Type | Thin Input | Required Context | Internal Planner / Executor |
|---|---|---|---|---|---|
| `conversation.reply` | `conversation.reply` | ACTION | `{text}` | none; target binding is runtime metadata | Reply output binder; deferred message settlement |
| `moment.publish` | `moment.publish` | ACTION | `{text}` | none; Moment target is runtime metadata | Moment output binder; deferred feed settlement |
| `media.image.generate` | `media.image.generate` | ACTION | `{intent}` | `visual_identity`, `current_life`, `appearance`, `current_state` | Image capability compiles intent + frozen context into the existing concept/prompt/workflow; ComfyUI/MinIO remain behind media workflow |
| `visual_identity.initialize` | `visual_identity.initialize` | INTERNAL | `{}` | `core_persona`, `visual_identity` | WakeUp-only lifecycle capability delegates to existing initialization workflow; no ordinary conversation exposure |
| `scene_event` | `scene_event` | ACTION | `{operation, scene, activity, location?}` | `current_life`, `schedule` | Semantic scene fields remain because they are the model's actual decision; evidence/source/time/idempotency are attached by Runtime; executor keeps transaction/CAS rules |
| `presence_event` | `presence_event` | ACTION | `{user_presence?, current_task?, expires_at?}` | `current_life` | Presence overlay assembler; executor owns bounded expiry and persistence |
| `schedule.replan` | `schedule.replan` | ACTION | `{intent}` | `schedule`, `current_life`, `agency` | Schedule planner compiles a replacement against live revision; completed-history protection and CAS remain Core-owned; planner failure is an execution failure, never a fallback to the old thick schema |
| `memory_event` | `memory_event` | ACTION | `{content, type?, importance?}` | `core_persona`, `memory_scope` | Memory assembler adds source/evidence/visibility/profile perspective and calls existing atomic recorder; no IDs or storage metadata in schema |
| `affect_event` | `affect_event` | ACTION | `{event: {type, confidence?}}` | `current_state` | Semantic event stays model-owned; Core reducer calculates numeric transition and owns evidence/idempotency |
| `relationship.lookup` | `relationship.lookup` | QUERY | `{target_actor_id}` | `relationship_scope` | Read-only authorized query; live scope check remains executor guard |
| `capability.request` | `capability.request` | INTERNAL | `{capability_key, title, description, rationale, desired_contract?, priority?}` | none beyond source fact | Owner-review request recorder; evidence/idempotency/security checks stay in Core |

### Mapping decisions

- Image and schedule are the two current domains where the thick payload is mostly implementation/assembly detail and a capability-local planner is justified. The local planner is not a second MainAgent response and its schema is never advertised.
- Scene operation and semantic scene/activity are still decisions the Main LLM must make; they are retained as a small explicit set instead of forcing a hidden parser to infer them.
- Relationship target is inherently a query decision and remains explicit. Capability request contract fields describe the missing capability the model wants, so collapsing them to an opaque string would lose product value.
- Evidence references, source facts, actor scope, revision/CAS, idempotency keys, workflow/provider IDs, renderer/workflow settings and database identifiers are always runtime/context-owned.

## 5. Catalog and schema rendering

`CapabilityRegistry.Catalog(surface)` returns definitions, filters by generic surface metadata, validates preflight status when required, and sorts by stable name. `RenderCapabilityTools([]CapabilityDefinition)` is the only provider renderer and reuses the existing OpenAI-compatible map shape. The renderer never sees executor internals or domain DTOs.

Thin-schema tests must assert:

- image has only bounded `intent`, not `concept`, workflow/model/sampler/camera/renderer/database fields;
- schedule does not expose revision, completion boundary, evidence, or idempotency fields;
- memory does not expose actor/event IDs, visibility internals, profile persistence metadata, or embedding controls;
- scene/presence keep only semantic action fields and bounded user-visible state fields;
- `conversation.reply`, `moment.publish`, and relationship query remain explicit because their fields are actual decisions.

## 6. Prompt and Policy split

- Keep `providerRuntimeProtocol` as the single global rule that external side effects must use a real capability call and prose cannot claim an unperformed state change.
- Add one concise generic rule in the Core Policy section: if the response says a state-changing behavior happened/is happening/immediately will happen and a registered capability exists, issue that capability call; never replace the real effect with prose.
- Remove per-capability trigger, prohibition, implementation-field and cross-tool coordination text from `conversationAssessmentInstruction`, `wakeUpAssessmentInstruction`, and `dailyReviewInstruction`. Keep only operation-level decisions that are not expressible in a schema.
- Keep capability descriptions to one short “can do” sentence. Move scene/schedule safety, media assembly, memory visibility, and WakeUp restrictions into Capability implementation/policy validators.
- Do not redesign unrelated response-format fields. Keep exactly one root structured `tool_calls` sidecar as a provider compatibility channel, remove the duplicated nested `response_plan.tool_calls`, preserve native-call precedence, and normalize both inputs once into the same CapabilityInvocation path. The sidecar is a codec shape, not a second executor/schema/runtime.

## 7. Persistence, replay, and compatibility

- Add the new canonical invocation/context/result fields to the existing frozen payload rather than creating a second action table. A database migration structurally rewrites still-executable v1 frozen calls into the v2 invocation/prepared-payload form; completed history remains audit data and is not re-executed.
- Before deploying the cutover, scan `cognition_frozen_actions`, `platform_workflow_intents`, and active workflow histories for old call shapes. The implementation plan includes a fail-closed migration/reconciliation step; active histories must be drained or migrated before the incompatible worker cutover. The final Runtime does not retain a v1 compat codec.
- Existing stable capability names, provider request IDs, call IDs, target kinds and idempotency keys remain unchanged. This preserves external provider job recovery and duplicate suppression.
- No `LegacyAdapter`, `CompatExecutor`, old registry, parallel schema builder, or v1 runtime codec remains after cutover. Released database migration SQL may retain the structural data migration, as required by the project's linear migration contract, but it is not an executable legacy path.

## 8. Concurrency, cancellation, and logging

- Keep the current sequential batch order and deferred two-phase ordering. Do not make `parallel` metadata execute concurrently in this migration.
- Runtime checks `ctx.Err()` before each call and passes the same context to resolver, preflight, executor, database, provider, storage and workflow APIs. Cleanup unlocks use a short derived context only where independent cleanup is required and are explicitly tested.
- Emit one bounded runtime lifecycle record per invocation with capability, call ID, type, surface, required slots, duration, status, retryable/error code. Intent text is length-bounded/redacted; full prompts, secrets and context snapshots stay out of normal logs.

## 9. One-cognition failure semantics

- Main cognition may return visible text and one or more capability invocations together. Runtime executes/finalizes required state-changing capabilities before committing that visible output.
- Definition metadata declares generic failure policy (`required_for_visible_claim` or `optional_internal`); the Runtime branches only on this policy, never a capability name.
- A required state-changing failure leaves the frozen decision and bounded CapabilityResult auditable, fails the conversation settlement, and persists no assistant reply that could falsely claim completion.
- An optional internal failure (for example a memory candidate rejected independently of the requested visible action) does not erase an otherwise valid reply, but its result must remain failed and must never be described as successful by Core.
- Image delivery is asynchronous: successful creation of the durable media intent is the capability success boundary for the conversation. The visible reply may promise/request generation, but cannot claim that the final asset already exists.
- No deterministic code inspects natural-language reply text to decide whether it lied; the LLM's capability invocation plus generic Definition failure policy is the structured contract.

## 10. Future discovery boundary

This delivery registers local capabilities explicitly in the application composition root. It does not introduce `CapabilityProvider`, MCP, HTTP discovery, embeddings or lazy selection. The future seam is `CapabilityRegistry.Catalog(surface)` plus Definition/Invocation: a later catalog source can contribute definitions and executors before registration without changing MainAgent or business Capability implementations. If lazy discovery is introduced later, it wraps catalog selection and still returns the same definitions; it does not create another execution model.
