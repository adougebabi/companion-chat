# 第三阶段技术设计：后台主动行为共享 ADK 接入

## 1. Boundary

```text
WakeUp Workflow/Activity
  -> App.ProcessWakeUp (existing lifecycle and domain use case)
  -> shared ADK structured-task boundary
  -> Eino ChatModel + ADK Runner
  -> WakeUp-scoped CapabilityRegistry adapter
  -> bounded completion/trace
  -> existing WakeUp policy/freeze/intent/outbox/settlement
```

The shared boundary owns only model/tool protocol execution. `ProcessWakeUp`,
`ProcessAutonomyAction`, `ProcessCapabilityAction`, PostgreSQL transactions,
Temporal retry, Redis hints, recurrence clocks and output publication remain
Core/workflow responsibilities.

## 2. Generic ADK loop contract

The conversation-only `ConversationRuntime` bridge is generalized without
copying its loop:

```go
type ADKCapabilityRequest struct {
    FluctlightID   string
    ConversationID string
    SourceFactID   string
    ActionID       string
    CorrelationID  string
    Surface        CapabilitySurface
    Projection     ContextProjection
}

type ADKStructuredTaskInput struct {
    Role           string
    Scenario       string
    Messages       []map[string]any
    Definitions    []CapabilityDefinition
    SchemaName     string
    Schema         map[string]any
    EnableThinking bool
    Capability     *ADKCapabilityRequest
}

type ADKStructuredTaskResult struct {
    Completion ProviderCompletion
    Trace      *ADKCapabilityTrace
}

func (a *App) RunADKStructuredTask(
    context.Context, ADKStructuredTaskInput,
) (ADKStructuredTaskResult, error)
```

The task boundary validates definitions against the Registry and the requested
surface before Provider I/O, creates a request-scoped trace/invoker, and calls
the existing `ProviderClient.StructuredAssembledWithToolsSchema`. Provider
schemas explicitly allowed for ADK include conversation Main/B and WakeUp;
Daily Review, Native Cognition and Reflection remain outside the loop unless a
later requirement adds a real feedback contract.

`RunADKLoop` (renamed from the phase-two conversation-specific helper) remains
the only Eino ADK model/tool event iterator. Its hard limits are:

- `MaxIterations <= 2`;
- no framework/provider retry or model fallback;
- formal Eino tool-call ID required;
- each physical Generate/Stream gets an independent queue/diagnostic attempt;
- event/tool/model errors return typed failure and never fabricate a final result.

## 3. Surface-aware capability bridge

The former invoker's hard-coded Conversation surface is replaced with the
request's `Surface`. The bridge still receives only narrow request identity and
projection data; it does not expose `*App`, a repository or a transaction to
the Eino tool.

For WakeUp:

- `Surface = CapabilitySurfaceWakeUp`;
- `SourceFactID = wakeID`;
- `ActionID = autonomy_wake_<digest(wakeID)>`;
- `CorrelationID = wake_up:<fluctlight_id>:cycle:<cycle>`;
- definitions = `CapabilityRegistry.Catalog(CapabilitySurfaceWakeUp)`;
- context snapshot and candidate validation use WakeUp scope;
- pure query tools can return bounded results to ADK;
- mutation/deferred/external tools return `deferred` and remain owned by the
  existing frozen action/intent worker.

The bridge must never expose `CapabilitySurfaceConversation` tools merely
because a direct conversation exists for the Fluctlight. `InternalOnly`
capabilities remain executable by deterministic Core policy only and are not
model-visible on any surface.

## 4. WakeUp prompt and result ownership

`ProcessWakeUp` keeps its existing projection and Composer call. Only the
Provider invocation changes from a single `RunStructuredToolsTask` call to the
shared ADK structured-task boundary. The current `wake_up_response` schema,
scenario, thinking setting, capability catalog and output fields remain the
same.

The existing post-model code remains authoritative for:

- action type and no-op normalization;
- evidence/influence validation;
- autonomy policy;
- target/recipient validation;
- Capability bind/prepare/freeze;
- `persistWakeUp` transaction;
- `autonomy.action`/`capability.action` intent and outbox creation;
- next WakeUp clock and Reflection/WakeUp Redis hints.

Intermediate ADK messages and tool observations are never published as user
messages or dynamic posts. A model/tool error, cancellation or iteration cap
must exit through the existing WakeUp failure/lifecycle path instead of being
normalized to no-op.

## 5. Task classification guard

The implementation adds explicit regression assertions for the classification:

| Schema / task | ADK loop | Reason |
| --- | --- | --- |
| `wake_up_response` | yes | required phase-three background entry |
| `conversation_turn_response` | yes | phase-two Main |
| `takeover_reply_response` | yes | phase-two B |
| `daily_review_response` | no | frozen action declaration; no model feedback |
| `native_cognition_response` | no | post-freeze capability settlement |
| `reflection_proposal_v2` | no | provider contract rejects ToolCall |
| summary/schedule/media/visual/embedding tasks | no | single Task or fixed workflow |

This is a schema/operation policy, not a global task-type switch used to
override domain boundaries.

## 6. Compatibility and rollback

- No Workflow/Activity names, queue names, retry budgets, recurrence keys,
  intent IDs or database schema fields change.
- If WakeUp ADK execution fails before a frozen action is persisted, existing
  WakeUp Activity/intent retry handles the bounded failure; no fallback to the
  old single Provider path is retained.
- Existing ConversationRuntime behavior and its tests must remain unchanged.
- The only new durable evidence is the existing per-call provider diagnostics
  and CapabilityInvocation/Result trace envelope; no new checkpoint or agent
  state table is introduced.
