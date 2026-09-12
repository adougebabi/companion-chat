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

## Scenario: Raw, Active, Summary, and Working Memory Separation

### 1. Scope / Trigger

- Trigger: a conversation turn, cognition, wake-up, daily review, Reflection,
  Automatic Retrieval, or `memory.recall` needs historical context without
  making Provider input grow with storage.
- Raw History and durable/Active authorities may grow. Working Memory and the
  final prompt are bounded read models; Summary is a rebuildable projection.

### 2. Signatures

```go
type RawHistoryReader interface {
	Recent(context.Context, RawHistoryQuery) ([]RawHistoryEvent, error)
	Search(context.Context, RawHistorySearchQuery) ([]RawHistoryEvent, error)
	ReadSources(context.Context, string, []string) ([]RawHistoryEvent, error)
}

applyActiveMemoryCommandTx(ctx, tx, PreparedActiveMemoryMutation) (ActiveMemoryApplyResult, error)
ResolveWorkingMemory(WorkingMemoryInput, WorkingMemoryPolicy) (WorkingMemory, error)
MemoryRecallService.Recall(ctx, MemoryRecallRequest) ([]map[string]any, bool, error)
```

Persistence roles:

```text
conversation_messages / cognition_inbox / cognition_action_outcomes
  owning Raw History rows; no copied raw_history table

active_memories / active_memory_revisions / active_memory_commands
  current short-lived authority + immutable lifecycle/command audit

conversation_summaries
  source-range projection with source refs/digest and superseding revision
```

Active operations are `create|confirm|revise|complete|expire|supersede`;
kinds are `future_event|commitment|temporary_context`; statuses are
`active|completed|expired|superseded`. `memory.recall/v1` is conversation-only,
requires `memory_scope`, and accepts only `{intent: string[1..1000]}`.

### 3. Contracts

- `RawHistoryEvent` is a read-only envelope over owning tables. Source refs use
  `message:<id>`, `fact:<id>`, or `outcome:<id>`; the reader never writes a
  second event ledger. Recent defaults to 50, caps at 200, source re-read caps
  at 64, and conversation search is currently message FTS only.
- Active Memory is not `memories.status='active'`. Reads require owner,
  conversation scope, `status='active'`, reached `valid_from`, and future
  `valid_until`; expiry therefore removes an item before cleanup writes the
  terminal revision. `applyActiveMemoryCommandTx` is its only lifecycle SQL
  authority.
- Active time keeps both the original expression and validated absolute bounds.
  `unknown` precision has no invented bounds; named timezone and supplied
  offset must agree. Every create supplies `original_time_expression` and
  `time_precision`; a `future_event` additionally requires nonempty original
  expression, non-unknown precision, and `valid_until`. Its optional
  `valid_from` is when the fact becomes relevant, not automatically the event
  start; omit it or use current/source time when it must be visible beforehand.
  Provider input never supplies owner, database ID,
  revision, canonical key, evidence storage, or idempotency.
- Summary work retains the latest 24 messages. An older chunk becomes eligible
  at 20 completed assistant turns, about 6000 estimated tokens, or 40 messages,
  and is cut on an assistant boundary. Settlement re-reads the contiguous Raw
  range and requires the same ordered refs and digest; old summaries are never
  summary input.
- Working Memory receives already-authorized fragments and performs no SQL,
  embedding, extraction, or LLM call. Default section caps are runtime `6144`,
  Active `2048`, Recent `8192`, retrieved Long-term `3072`, Summary `2048`.
  It selects whole fragments and whole recent turns, restores chronological
  order, and deduplicates shared source refs without deleting source rows.
- Automatic Retrieval uses bounded cues from current input, recent topic,
  current state, Active Memory, goals/intentions/outcomes/hypotheses. At most 32
  cues, 1000 runes each, and 1024 estimated query tokens are allowed; projection
  cues set `AllowEmbedding=false` unless a separate field-level policy exists.
- `memory.recall` uses frozen authorization/viewer/conversation/profile scope
  but executes a fresh bounded query over Active, Long-term, authorized older
  conversation messages, and Summary. Final output is deduplicated, at most 12
  whole items and 3072 estimated tokens, and contains only opaque refs plus
  bounded semantic fields. It has no database write or workflow intent.
- Durable Memory remains the four-type `memory.lifecycle.v2` authority. Active
  completion/expiry does not automatically promote an item; Reflection may
  create a durable candidate from the same Raw evidence and close Active in its
  caller-owned transaction.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Raw query lacks owner or authorized conversation scope | Reject before reading sources. |
| Existing message/fact sequence duplicates block `0032` | Roll back schema and ledger; never renumber history. |
| Active create omits original expression/precision, future event omits expiry, time is unknown but bounds are supplied, timezone offset disagrees, or range is reversed | Reject the command; append no current row/revision/command result. |
| Active target revision is stale or idempotency payload differs | Conflict; replay only an identical prior command. |
| Active row is expired at read time but cleanup has not run | Exclude immediately; cleanup may later append `expire`. |
| Summary source has a gap, changed ref/digest, wrong author boundary, or foreign owner | Reject settlement; keep Raw rows and retryable intent. |
| Required Working Memory fragment exceeds its section | `working_memory_required_budget_exceeded`; do not truncate it. |
| Recall result exceeds item/token bounds | Drop whole lower-ranked items and return `truncated=true`. |
| Recall source is unauthorized or has no provider-safe content | Exclude before combined ranking; expose no raw identifier. |

### 5. Good / Base / Bad Cases

- Good: “明早 7 点赶飞机” leaves Recent, remains selected Active Memory with
  original expression/timezone, and disappears immediately after `valid_until`;
  the source message remains readable and Reflection decides separately whether
  the experience deserves durable episodic Memory.
- Good: 1000 Raw events, 100 durable Memories, and 30 Active items still produce
  bounded whole-item Working Memory; the trace explains selected, section-cap,
  total-cap, deduplicated, expired, and unauthorized outcomes.
- Base: no summary or recall match exists; current facts and recent complete
  turns still assemble without manufacturing Memory.
- Bad: copy every message into `memories`, call a summary the source of truth,
  keep an expired flight because cleanup has not run, or return raw retrieval
  rows from `memory.recall`.

### 6. Tests Required

- Raw pagination/source/scope tests, 1000+ row growth, atomic user/fact/intent/
  outbox rollback, sequence-duplicate migration failure, and unchanged Raw
  rows after Summary/Working Memory selection.
- Active create/confirm/revise/complete/expire/supersede, timezone/unknown-time,
  read-time expiry, CAS, idempotency/replay, provenance, and one-SQL-authority
  guards.
- Summary threshold/range/digest/source-drift/rebuild/supersede tests and
  Provider/restart replay with no recursive summary input.
- Working Memory whole-fragment/whole-turn, source-dedupe, required overflow,
  fixed-cap stress, real-role order, and dropped-reason trace assertions.
- Automatic Retrieval tests for authorization-before-limit, old relevant rows,
  irrelevant bulk, lexical fallback honesty, cue cap, and ABA current lineage.
- `memory.recall` definition/scope/deep-query/opaque-output/item-token-bound tests;
  continuation behavior is tested by the Structured Turn contract.
- Real PostgreSQL migration and lifecycle cases remain mandatory at the final
  acceptance gate; unit/compile evidence is not a substitute.

### 7. Wrong vs Correct

#### Wrong

```go
prompt.Memories = loadAllMemories(fluctlightID)
prompt.History = loadAllMessages(conversationID)
```

#### Correct

```go
raw := rawHistory.Recent(ctx, boundedQuery)
active := retrieveActiveMemories(ctx, scopedQuery)
retrieved := retrieveMemoryWithPlan(ctx, actorID, fluctlightID, plan)
working, err := ResolveWorkingMemory(WorkingMemoryInput{
	RecentMessages: recentFragments(raw),
	ActiveCandidates: activeFragments(active),
	RetrievedMemories: durableFragments(retrieved),
	Summaries: sourceBoundSummaries,
}, DefaultWorkingMemoryPolicy())
```
