# QUERY Continuation Contract Conflict

Date: 2026-09-11

## Finding

The user requirement permits `memory.recall` to trigger a result-dependent continuation when the Main LLM cannot answer without recalled results. The current branch does not have such a mechanism.

It has:

- `CapabilityTypeQuery`
- metadata-driven `CapabilityExecutionPureQuery`
- read-only execution through the canonical Registry/Runtime

It does not have:

- same-turn query result injection into a second LLM call
- a `role=tool` continuation
- a durable continuation phase/replay contract

Current code fails a direct turn with no visible text before capability settlement (`apps/core-go/internal/core/mutations.go:803-817`) and uses the same first cognition's visible text after capability execution (`mutations.go:938-980`).

## Existing Hard Contract

- `.trellis/spec/backend/fluctlight-cognitive-runtime.md:73-78`: exactly one Main cognition; no same-turn `role=tool` continuation or second realization.
- `.trellis/spec/backend/structured-turn-contract.md:467-475`: direct conversation always requires visible text from the same Main cognition.
- `apps/core-go/internal/core/capability_core_test.go:1020-1028`: static guard requires exactly one `StructuredWithToolsSchema` call and forbids tool role in conversation flow.
- `apps/core-go/internal/core/project_health_architecture_guard_test.go:21-27`: production-tree guard forbids any tool-role continuation.
- `README.md:41-45` and `docs/capability-architecture.md:83-88`: capability results are for replay/later cognition, not same-turn continuation.

## `memory.recall` Without Core Runtime Changes

The capability itself can be added without changing `CapabilityDefinition`, Registry, codec or generic dispatch:

```text
name              memory.recall
version           v1
type              query
surfaces          conversation
input             { intent: bounded string }
required context  memory_scope
side effect       read_only
concurrency       parallel
success boundary  query_result_available
failure policy    optional_internal
```

It should use frozen MemoryScope only for owner/viewer/conversation/profile authorization and inject a narrow recall service that builds a fresh bounded `MemoryQueryPlan` from `intent`. Returning the already-frozen automatic `memory_scope.memories` would not be deeper recall.

Relevant anchors:

- query classification: `apps/core-go/internal/core/capability_core.go:18-84`
- current query example: `relationship_capability.go:12-83`
- retrieval plan/service foundation: `memory_retrieval.go:13-88`, `147-428`
- context scope resolver: `app_capability_context.go:212-246`
- frozen snapshot behavior: `capability_core.go:1408-1416`, `1521-1526`

Provider-visible recall results must be mapped to opaque refs plus bounded semantics. Raw retrieval items contain database IDs, visibility, conversation IDs and raw evidence refs and cannot be sent directly (`memory_retrieval.go:330-337`; provider-safe allowlist at `provider_context.go:502-526`).

## Decision Options

### Option 1 — Add one generic pure-QUERY-only continuation (recommended)

Add a bounded orchestration seam outside the Capability core:

1. First Main completion explicitly produces no final visible answer and one or more pure QUERY invocations.
2. Freeze/prepare/execute those read-only queries outside a business transaction.
3. Persist bounded results and continuation phase for replay.
4. Perform at most one continuation with no tools exposed.
5. The continuation can only provide the visible answer; it cannot submit new state/appraisal/capability mutations.
6. ACTION or mixed batches remain strict one-Main-cognition flows.

This satisfies same-turn active recall but intentionally changes the current blanket continuation contract and static guards. It must be authorized explicitly.

### Option 2 — Preserve absolute one-cognition semantics

Add `memory.recall` as a pure query whose result is persisted for later cognition only. Automatic Retrieval remains the only path that can affect the current reply.

This preserves all current contracts and is cheaper/simpler, but it does not meet the user's stated “without recall result the current reply cannot be completed” experience. The user would need a subsequent turn or system-triggered later cognition.

## Recommended Decision

Choose Option 1, but define it as a narrow new conversation orchestration contract, not as an existing runtime capability. Keep canonical CapabilityDefinition/Registry/codec/dispatch untouched and replace blanket guards with guards that permit at most one no-tools continuation for pure-query-only batches.
