# Current Tool Inventory (2026-09-09)

Scope: read-only scan of `apps/core-go/internal/core`. The authoritative production catalog is the 11 executor-backed manifests registered in `app.go:72-84` and fallback-registered in `capability_runtime.go:13-30`. `ExternalCapabilityManifests()` at `tool_contract.go:195-201` is test-only and contains four entries; it must not be treated as the production catalog.

## Provider-facing catalog and execution

| Current Tool | Purpose | Current Args | Description Complexity | Current Executor | Context Dependencies |
|---|---|---|---|---|---|
| `conversation.reply` | Deliver final user-visible conversation text | `text` | Low, one capability sentence | `conversationReplyCapabilityExecutor`; deferred target settlement | Text in call; caller-owned conversation message binding |
| `moment.publish` | Publish final Moment text to shared feed | `text` | Low, one capability sentence | `momentPublishCapabilityExecutor`; deferred target settlement | Text in call; caller-owned Moment binding; omitted from ordinary conversation catalog |
| `media.image.generate` | Request one configured image generation | Open `concept` object (`additionalProperties: true`) | Low top-level sentence, but concept schema delegates a large visual DTO | `imageCapabilityExecutor`; preflight + deferred media intent | Cognition `ContextProjection`; visual identity, scene, appearance, state; frozen concept binding; runtime media config; MinIO/ComfyUI in worker |
| `visual_identity.initialize` | Start durable visual identity workflow | Optional `reason` | Medium: includes WakeUp lifecycle restriction | `visualIdentityCapabilityExecutor` | Core persona identity/life profile; active visual identity session; WakeUp source fact |
| `scene_event` | Start/switch/end evidence-backed scene/activity/location | `operation`, scene/activity/location, time, evidence, confidence, idempotency | High: trigger rules, prose prohibition and schedule coordination | `sceneCapabilityExecutor` -> `applySceneCapability` | Live active scene, source fact, current time, PostgreSQL transaction |
| `presence_event` | Record temporary presence overlay | user presence/task/expiry, evidence, confidence, idempotency | Low | `presenceCapabilityExecutor` -> `applyPresenceCapability` | Current time, source fact, PostgreSQL transaction |
| `schedule.replan` | Replace current/future local-day schedule after interruption | date/timezone/revision/completed boundary/full item array/reason/evidence/idempotency | High: completed-history and full-replacement rules | `scheduleReplanCapabilityExecutor` -> `applyScheduleReplanCapability` | Live accepted schedule, local timezone/date, revision/CAS, current clock |
| `memory_event` | Record explicit evidence-backed memory candidate | operation/type/content/confidence/importance/visibility/perspectives/actor/event/evidence/idempotency | Low sentence, large domain schema | `memoryCapabilityExecutor` -> `applyMemoryCapability` | Source fact/conversation, owner/participants, current core persona profiles, existing memory/revision |
| `affect_event` | Record semantic affect event; Core owns numeric reducer | nested event type/confidence/evidence/idempotency | Medium: description mentions reducer ownership | `affectEventCapabilityExecutor` -> `applyAffectEvent` | Live inner state/revision and static reducer table |
| `relationship.lookup` | Read one authorized direct relationship | `target_actor_id` | Low | `relationshipLookupCapabilityExecutor` | Owner, conversation participant scope, active profile, live relationship row |
| `capability.request` | Record missing capability request for Owner review | key/title/description/rationale/desired contract/side effect/priority/evidence/idempotency | Medium: includes non-side-effect boundary | `capabilityRequestExecutor` -> `persistCapabilityRequest` | Source fact, Fluctlight; no domain snapshot |

## Exposure by cognition scenario

| Scenario | Catalog selection |
|---|---|
| Ordinary conversation | All registry manifests except `moment.publish` (`mutations.go:568-575`), 10 tools |
| WakeUp | All 11 (`wakeup.go:295-300`) |
| Daily review/autonomy | All 11 (`autonomy.go:68-73`) |
| Native cognition | All except `affect_event`, `moment.publish`, `conversation.reply` (`cognition_growth.go:257-260`), 8 tools |
| Reflection | No tools (`workflow_ops.go:481-489`) |

## Provider and runtime path

`NewApp` registers executors -> `CapabilityRegistry.Manifests()` returns stable name order -> scenario selects manifests -> `Provider.StructuredWithToolsSchema` -> `providerChatPayloadWithSchema` adds `tools`, `tool_choice=auto`, strict response format -> `ToolCallPayload` renders OpenAI-compatible function definitions -> provider response native calls or structured sidecar -> `NormalizeProviderToolCalls` -> frozen decision -> `ExecuteToolCalls` -> deferred settlement after output target exists.

Evidence: `tool_contract.go:23-38,120-193,306-417`; `provider.go:143-181,528-550`; `mutations.go:568-591,630-680,697-828`; `capability_runtime.go:50-154,334-410`.

## Current boundary defects relevant to this task

- `CapabilityExecutor.Execute` receives raw `ToolCallV1` plus IDs, not a declared context snapshot (`tool_contract.go:112-118`).
- No `ContextSlot`, `RequiredContext`, `ContextResolver`, or `CapabilityContext` exists. `ContextProjection` is an eager cognition read model (`intelligence.go:30-69,118-245`), not executor DI.
- `ExecuteToolCalls` is already registry-based but has name-specific preflight (`capability_runtime.go:413-447`) and `conversation.reply` action detection (`capability_runtime.go:607-623`).
- `mutations.go:1131-1155` rewrites `capability.request` into `conversation.reply`; `media_context.go:14-47` has a name-specific media context binder.
- Runtime executes calls serially; `ConcurrencyClass:"parallel"` is metadata only (`capability_runtime.go:53-153`).
- Tool result is persisted into frozen actions; current main conversation path does not construct `role=tool` messages or perform a second LLM call (`mutations.go:746-860`).
- Provider usage tokens and Tool schema byte sizes are not recorded (`provider.go:213-301`; `diagnostics.go:251-289`).
- Runtime has no unified per-call lifecycle log; current logs are provider shape and turn-level warnings.
