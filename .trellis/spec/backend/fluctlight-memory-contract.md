# Fluctlight Memory Contract

## Scenario: Governed Memory Lifecycle With Operation-Aware Retrieval

### 1. Scope / Trigger

- Trigger: the clean-start system records, revises, retrieves, embeds, consolidates, corrects, forgets, or injects Memory into a cognitive prompt.
- Applies to episodic, semantic, relationship, and autobiographical Memory. Working memory is a bounded read model from current Conversation/Cognition facts.
- PostgreSQL rows are authoritative. pgvector and full-text indexes are retrieval mechanisms, not a second source of truth.

### 2. Signatures

```go
applyMemoryCommandTx(ctx, tx, PreparedMemoryMutation) (MemoryApplyResult, error)
retrieveMemoryWithPlan(ctx, authorizationActorID, fluctlightID, MemoryQueryPlan) (MemoryRetrievalResult, error)
ProcessMemoryEmbeddingIntentAt(ctx, intentID, memoryID, revision, endpointID, modelID) (map[string]any, error)
```

`PreparedMemoryMutation.operation` is
`create|confirm|revise|merge|supersede|deprecate|forget|rollback`. Reflection may
propose only the first six through `MemoryCandidateV2`; authenticated Owner APIs
compile `forget` and full-snapshot/lineage `rollback` commands inside Core.

Required authoritative fields:

```text
Memory
  id
  owner_fluctlight_id
  type: episodic | semantic | relationship | autobiographical
  content
  actor_refs[]
  conversation/event/evidence refs[]
  confidence / importance / emotional_significance
  visibility_scope
  status / revision
  occurred_at / created_at / last_confirmed_at

MemoryEmbedding
  memory_id
  memory_revision
  provider_endpoint_id
  model_id
  dimensions
  embedding
  status: pending | ready | failed | stale
  embedded_at
```

`MemoryQueryPlan` includes an explicit operation, authorization actor, viewer
Actor set, `exact|allowed_set|global_only` Conversation scope, active profile,
allowed types, bounded typed cues, result/candidate limits, budget, and mode:
`salience_recent`, `lexical_salience`, or `semantic_hybrid`. Internal Scene,
Goal, Outcome, Reflection and Capability cues are local-only unless a separate
field-level egress policy explicitly sets `AllowEmbedding`.

### 3. Contracts

- `applyMemoryCommandTx` is the only production SQL authority for Memory rows,
  revisions and governance. Interactive `memory_event`, Reflection, and Owner
  governance delegate to it; non-transactional `memory_event.Execute` returns
  `caller_transaction_required`.
- Memory parent row, full immutable snapshot revision, governance disposition,
  embedding workflow intent and outbox events share the caller-owned transaction.
- `memory_event` Provider arguments contain only `type`, `content`, `confidence`,
  `importance`, and optional `emotional_significance`. Runtime Prepare freezes
  owner/profile/conversation/evidence/visibility/time/idempotency/request digest
  into `memory_plan`; replay never reconstructs them from Provider arguments.
- Reflection `MemoryCandidateV2` is closed and contains only operation-specific
  `target_ref`/`merge_refs`, semantic fields, `evidence_refs`, and
  `semantic_reason`. Core resolves opaque refs through the frozen
  `ContextReferenceIndex`, rejects overlapping targets, and binds expected revisions.
- Exact duplicate create is explicit `no_change`; approximate similarity never
  causes a Core-inferred merge. The canonical key includes type, trimmed content,
  Conversation, visibility, sorted Actor refs, and sorted Event refs.
- Owner rollback appends compensation revisions. Forgotten/deprecated rows may
  be restored; a merge restores every unchanged related row; a supersede restores
  the prior row and deprecates the created replacement. Any related row that
  evolved after the target operation causes a conflict rather than lost updates.
- Embedding failure never deletes or invalidates authoritative Memory. It remains `pending`/`failed` and may be retrieved through authorized metadata/full-text paths.
- One canonical embedding tuple is `(memory_id,memory_revision,model_id)`.
  Revision zero is an exact revision, not an “unspecified” sentinel. The frozen
  Provider endpoint/model binding is checked against the workflow intent and
  tuple; `pending|failed -> ready` updates the same row, and ready is never
  downgraded by a later failed retry.
- Memory must be `active` at both embedding read and settlement. A revision or
  lifecycle change while Provider I/O is in flight makes the tuple stale; the
  Provider call remains outside the transaction.
- Ownership, visibility, Actor, Conversation, type, status, and time constraints are mandatory hard filters. Similarity cannot bypass them.
- Empty Conversation scope is never a wildcard. SQL applies owner, status, type,
  visibility, viewer Actor and explicit Conversation mode before candidate limit,
  FTS/lexical scoring or vector lookup. Vector lookup receives only those
  authorized candidate IDs and rechecks active/current revision.
- Full-text search is lexical candidate retrieval only. It cannot infer intent, relationship meaning, importance, or other semantic state.
- Vector search compares only rows with the same model ID and dimensions.
- Exact vector search is the default. HNSW requires a recorded benchmark showing dataset threshold, latency target, recall target, filter behavior, build cost, and NAS resource impact.
- Hybrid ranking, when explicitly authorized, combines authorized FTS/vector
  candidates with recency, importance and emotional significance. Without
  embedding egress authorization the trace reports `lexical_salience`, not a
  false semantic/hybrid claim. A future LLM reranker may select only from the
  authorized candidate set.
- Provider context contains the opaque Memory `ref`, bounded semantic fields,
  semantic creation time, and only the active profile's bounded interpretation.
  Raw Memory ID, expected revision, backing evidence IDs, profile ID,
  provenance, visibility and embedding metadata remain Core-only.
- Reflection evidence keeps original observations, authoritative appraisal,
  allowlisted ActionOutcome fields, and authoritative Memory refs. An
  `autonomy.result` projection physically removes visible assistant realization
  text, raw expected/observed payloads and runtime IDs before the Provider call.
- Conversation scope for a Reflection create is derived from every cited
  sequence/Memory/Outcome evidence ref. Conflicting or unknown scopes fail;
  they never fall back to a globally visible Memory.
- Working memory is composed from current Conversation and unresolved Cognition state; do not duplicate complete recent-message history into durable Memory.
- Embedding regeneration changes only the index row. Memory content changes require a Memory revision and mark prior embeddings stale.
- The native `memory_event` capability may omit `emotional_significance`; the
  Runtime normalizes the missing optional signal to `0` (no inferred emotional
  weight) while retaining the required authoritative field on the Memory row.
- A shared Memory has one canonical fact row. Optional `personality_perspectives`
  contains at most one validated interpretation per declared personality
  profile, with bounded evidence references; retrieval selects only the active
  profile perspective and never duplicates the Memory row. Perspective evidence
  is validated against the same cognition/reflection evidence window as the
  parent Memory.
- Migration `0029_memory_lifecycle` is additive and leaves `0027`/`0028`
  immutable. Existing Memory can cross only after an explicit
  `memory.lifecycle.repair.v1` audit whose stored source snapshot exactly matches
  type/content/Conversation/visibility/sorted Actor/Event scope and whose
  canonical/request digests match the repaired row. Missing or unverifiable
  repair, duplicate active keys/revisions/embedding tuples, malformed active
  embedding intents, or durable `working` rows roll back the migration and ledger.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Memory owner/evidence/Actor scope invalid | Reject before persistence; no embedding intent. |
| Provider supplies Memory ID/revision/profile/visibility/provenance/idempotency | Closed schema rejection before compilation; watermark unchanged. |
| Reflection target ref is foreign, wrong-kind, stale, overlapping, or cross-Conversation | Reject proposal/command; no Memory mutation or watermark advance. |
| Reflection candidate is empty, scalar, wrong-container, or has unknown fields | Reject the whole structured proposal before normalization; keep the window retryable. |
| Memory transaction commits and embedding Provider is unavailable | Keep Memory authoritative, mark embedding pending/failed, retry asynchronously. |
| Embedding workflow revision is zero | Require exact current revision zero; never embed a later revision. |
| Workflow input endpoint/model differs from persisted binding | `memory_embedding_assignment_conflict`; do not rebind the tuple. |
| Embedding dimensions/model mismatch | Reject index write; never compare incompatible vectors. |
| Query lacks authorized owner/visibility scope | Reject query; do not run FTS/vector search. |
| Similar result belongs to another Fluctlight/Conversation scope | Exclude before ranking and prompt construction. |
| HNSW has no accepted benchmark | Use exact search; do not create/enable the approximate index. |
| LLM reranker unavailable | Return the bounded deterministic authorized hybrid ranking; do not infer new semantic state. |
| Memory content revision accepted | Mark previous embedding stale and enqueue versioned rebuild. |
| Forgotten/superseded/deprecated Memory appears in an index | Filter by authoritative status; never expose it in prompt context. |
| Rollback lineage row changed after the operation being compensated | Conflict and roll back the whole compensation; never erase newer evolution. |
| `0029` repair evidence is missing/mismatched | Abort migration in the same transaction; ledger stays at `0028`. |

### 5. Good / Base / Bad Cases

- Good: a relationship Memory commits with evidence and visibility, embedding retries after an outage, then becomes searchable without changing the Memory revision.
- Good: a future group Conversation query first restricts Memory visibility to authorized participants, then performs hybrid ranking.
- Good: Reflection cites opaque refs, merges two current Memories in one proposal,
  and the next projection exposes only the new primary revision/ref.
- Good: failed embedding retry updates one tuple row to ready using the frozen
  endpoint/model; authoritative Memory is unchanged by Provider failure.
- Base: a new Memory has no embedding yet and is found by Actor/type/time/full-text filters.
- Base: internal cues have no egress authorization, so local FTS/lexical +
  salience/recency runs with a bounded Core-only trace.
- Bad: put Memory only in a vector store, search all vectors then filter ownership in application code, mix embedding models, or treat nearest-neighbor output as confirmed fact.
- Bad: duplicate every recent Message into durable Memory or create a preference from a keyword match.
- Bad: turn an unknown/conflicting Conversation scope into global, learn from
  assistant prose, or let Owner/Reflection write Memory through separate SQL paths.

### 6. Tests Required

- Persistence tests for all lifecycle operations, exact duplicate `no_change`,
  CAS/replay, full snapshots, governance, merge/supersede lineage compensation,
  terminal-state rollback and outbox atomicity.
- Embedding tests for exact revision zero, frozen assignment conflict,
  failed-to-ready same-row transition, ready non-downgrade, active-state checks,
  stale rebuild and Provider failure without Memory loss.
- Authorization tests proving hard owner/visibility/Actor/Conversation filters run before results enter ranking or prompt context.
- Retrieval tests for metadata, FTS, exact vector, hybrid rank, bounded LLM rerank, result limits, and token budget.
- Benchmark fixture defining the dataset/latency/recall threshold required before enabling HNSW; filtered-query recall must be measured.
- Working-memory tests proving it is bounded and composed without duplicating full Conversation history into durable Memory.
- Reflection tests assert closed candidate schema, opaque ref compilation,
  allowed evidence kinds/scope, assistant-prose exclusion, proposal/watermark
  rollback, all six operations and next-projection revision/ref.
- Prompt tests assert every included Memory retains opaque ref and bounded
  semantics while raw ID/revision/evidence/profile/provenance remain absent.
- Real PostgreSQL tests cover empty/repaired `0028 -> 0029`, rerun, malformed
  Memory/intent, unverifiable repair, duplicate canonical/revision/embedding
  identity, constraints and ledger rollback.

### 7. Wrong vs Correct

#### Wrong

```go
matches := vectorStore.Search(queryEmbedding)
return filterOwnerAfterRanking(matches) // too late; leaks/crowds out results
```

#### Correct

```go
plan := buildMemoryQueryPlan(operation, viewers, conversationMode,
    conversationID, allowedConversationIDs, activeProfileID, cues, 12, 2400)
result := retrieveMemoryWithPlan(ctx, authorizationActorID, fluctlightID, plan)
// SQL authorization precedes candidate limit/ranking; result.Trace is Core-only.
```
