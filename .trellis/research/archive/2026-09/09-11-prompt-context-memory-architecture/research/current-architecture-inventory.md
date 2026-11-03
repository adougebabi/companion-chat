# Current Architecture Inventory

Date: 2026-09-11  
Branch: `codex/prompt-context-memory`, created from clean `codex/llm-capability-runtime` at `6cbc18b`

## Executive Summary

The branch already has a strong long-term Memory authority and a canonical Thin Capability runtime. The missing architecture is not basic Memory CRUD: it is the bounded projection between growing storage and Main cognition.

The most consequential current facts are:

1. Provider-facing conversation context is one large `role=user` document. Recent roles are rows inside that document, not transport-level messages.
2. The current user input is rendered twice (`current_message.content` and top-level `text`).
3. Memory retrieval is bounded to 12 results and 2400 Unicode runes, but the whole prompt is not bounded.
4. Durable Memory has lifecycle, governance and retrieval foundations, but no distinct Active Memory, automatic expiry/promotion, Conversation Summary or explicit `memory.recall`.
5. Storage is distributed across several authoritative tables. There is no single immutable Raw History ledger or unified full-history query.
6. Direct conversation has a hard one-Main-cognition/no-tool-continuation contract even though a pure QUERY execution class exists.

## Production Cognition Path

```text
Temporal ProcessCognitionActivity
  -> App.ProcessCognitionInbox
  -> App.HandleTurn / handleTurn
     -> persist current user conversation message
     -> enqueue conversation.turn cognition fact
     -> buildTurnProjection
        -> BuildContextProjectionFor
        -> authorized MemoryQueryPlan (limit 12, budget 2400 runes)
        -> DB.History(..., 12)
     -> compactCognitionContext
     -> hand-build system + one large user payload
     -> composeProviderMessages
     -> StructuredWithToolsSchema
     -> freeze decision/capability invocations
     -> generic capability execution
     -> atomically settle assistant message and effects
```

Anchors:

- Workflow entry: `apps/core-go/internal/workflow/workflow.go:552-557`
- Inbox routing: `apps/core-go/internal/core/cognition.go:52-75`
- User message first persistence: `apps/core-go/internal/core/mutations.go:519-577`
- Projection construction: `apps/core-go/internal/core/mutations.go:671-684`, `1113-1123`
- Context read model: `apps/core-go/internal/core/intelligence.go:129-292`
- Main provider request: `apps/core-go/internal/core/mutations.go:722-729`
- Single-pass settlement: `apps/core-go/internal/core/mutations.go:938-980`
- Provider payload: `apps/core-go/internal/core/provider.go:143-184`, `528-572`

## Current Storage Authorities

There is no single Raw History table. The current facts are distributed across:

| Surface | Authority | Append characteristics | Main gap |
| --- | --- | --- | --- |
| Conversation | `conversation_messages` | User/assistant text is normally appended; attachment refs may update in place | No turn/source/action/correlation columns; no tool events |
| Persona fact stream | `cognition_inbox` | New facts append with persona sequence | Also a mutable queue: claim/status and some payload fields update |
| Decisions/tools | `cognition_frozen_actions`, `autonomy_actions`, `cognition_action_outcomes` | Stable call/action identity and results retained | Invocation/result JSON aggregate is updated in place |
| State | `fluctlight_inner_states` plus revision/audit tables | Snapshot + audit/revision pattern | Some same-source revisions can merge in place |
| Life/scene/presence | `life_events`, `life_presence_overlays` plus cognition facts | New events/overlays append | Prior event lifecycle fields update; no full row revision ledger |
| Reflection | proposal/disposition tables plus mutable watermark | Proposals/dispositions append idempotently | No unified public history reader |
| Moments | `moments`, comments, reactions | Moment/comment create appends | status/media/reaction state updates; weak source provenance |
| Transport | `platform_outbox_events` | Business event envelope retained | Delivery state mutates; payload is not a full fact snapshot |
| Diagnostics | prompt/model audit tables | Useful operational evidence | Explicitly pruneable, therefore not Raw History SOT |

Key anchors:

- Conversation schema: `apps/core-go/internal/migrations/runner.go:157-163`
- Persona fact schema: `apps/core-go/internal/migrations/runner.go:164-165`
- Turn fact append: `apps/core-go/internal/core/cognition.go:208-309`
- Frozen decision records: `apps/core-go/internal/core/cognition.go:400-494`
- Assistant/effect transaction: `apps/core-go/internal/core/mutations.go:985-1046`
- Action outcomes: `apps/core-go/internal/core/action_outcome.go:28-105`, `235-260`, `375-468`
- Reflection window read: `apps/core-go/internal/core/workflow_ops.go:558-650`
- Reflection atomic persistence: `apps/core-go/internal/core/evolution_persistence.go:201-265`
- Diagnostics pruning boundary: `apps/core-go/internal/core/operations.go:1680-1701`

## Existing Memory Foundation

### Durable model

The production authority already supports:

- types: `episodic | semantic | relationship | autobiographical`
- parent statuses: `active | superseded | deprecated | forgotten`
- operations: create, confirm, revise, merge, supersede, deprecate, forget, rollback
- revision CAS, request digests, idempotent replay, immutable snapshot revisions
- governance dispositions and lineage
- owner/visibility/actor/conversation hard filters before ranking
- FTS, versioned embeddings, stale revision protection, bounded hybrid retrieval
- opaque revision/scope/snapshot-bound refs for Provider-visible evidence

Anchors:

- Tables and constraints: `apps/core-go/internal/migrations/runner.go:197-201`, `1090-1162`
- Single write authority: `apps/core-go/internal/core/memory_lifecycle.go:253-329`
- Lifecycle implementations: `apps/core-go/internal/core/memory_lifecycle.go:584-956`
- Static SQL authority guard: `apps/core-go/internal/core/memory_lifecycle_test.go:625-644`
- Reflection candidate compiler: `apps/core-go/internal/core/reflection_memory.go:13-233`
- Embedding workflow: `apps/core-go/internal/core/memory_embedding.go:13-225`
- Retrieval plan/filter/rank/budget: `apps/core-go/internal/core/memory_retrieval.go:13-88`, `147-428`

### Automatic retrieval

Automatic retrieval already runs while building `ContextProjection` for conversation, wake-up, daily review, native cognition, reflection and capability MemoryScope. It combines operation cues with current input, life context, goals, intentions, outcomes and hypotheses.

It does not currently use recent conversation topic/summary, and production cue construction disables embedding egress, so automatic retrieval is presently lexical/salience rather than vector-hybrid.

Anchors:

- Cue construction: `apps/core-go/internal/core/memory_retrieval.go:103-145`
- Query plan in projection: `apps/core-go/internal/core/intelligence.go:224-234`
- Production automatic call sites: `apps/core-go/internal/core/mutations.go:1113-1123`, `wakeup.go:278-304`, `autonomy.go:63-85`, `cognition_growth.go:323-336`

### Missing semantics

The durable `status='active'` flag means “current long-term authority,” not “temporary fact that remains behaviorally relevant.” Missing pieces are:

- Active Memory tier and lifecycle
- `valid_from`, `valid_until`, original/resolved temporal expressions and timezone semantics
- completion/expiry/promotion workflow
- temporal-urgency/current-relevance/last-relevant ranking
- subject-level ABA conflict prevention beyond explicit LLM-proposed supersede
- Conversation Summary with source boundaries and rebuild
- explicit `memory.recall`
- per-candidate selection/drop explanation

## Context and Budget Gaps

Current local bounds:

- recent messages: 12 rows (`apps/core-go/internal/core/intelligence.go:239-248`)
- recent outcomes: 12 rows (`intelligence.go:220`)
- Memory: 12 results / 2400 serialized semantic runes (`intelligence.go:224-230`, `memory_retrieval.go:406-427`)
- Memory cues: at most 32, each at most 1000 runes
- Memory SQL candidate pool: at most 200 recent rows before Go ranking
- Provider output: `model_roles.token_budget`, default 4096, emitted as `max_tokens`

There is no total input budget covering System, Persona, Runtime Context, Recent, Active, Retrieved, Summary, Tools, Response Schema, Current Input and Output Reserve. `max_tokens` controls only completion output.

Known unbounded or high-cardinality context sources include goals, intentions, relationships, developing-self entries, typed drive/preference/trigger slots, schedule expansion and individual message length.

## Existing Tests Worth Reusing

- Full conversation request capture: `apps/core-go/internal/core/life_context_e2e_test.go:68-128`
- Prompt composer baseline: `apps/core-go/internal/core/provider_prompt_composer_test.go`
- Current-input/history dedupe: `apps/core-go/internal/core/provider_context_test.go:383-398`
- Recent semantic-role projection: `provider_context_test.go:483-518`
- Memory retrieval authorization/budget: `memory_retrieval_test.go`
- Memory lifecycle and transaction semantics: `memory_lifecycle_test.go`
- Conversation delivery/recovery: `conversation_delivery_regression_test.go`
- Cross-domain transaction rollback: `project_health_transaction_integration_test.go:316-460`

## Inventory Implications

- Do not replace the existing Memory lifecycle authority; extend or compose it.
- Do not equate the existing durable `active` status with the requested Active Memory tier.
- Do not promote outbox or diagnostics to Raw History authority.
- The first production seam to consolidate is Main prompt/context assembly, not the generic Capability runtime.
- Raw History can be a logically unified read/source contract over existing authoritative stores in the first iteration; a new giant event-sourcing framework is neither required nor allowed.
- Any new schema must improve source linkage and projection semantics without destructively rewriting existing data.
