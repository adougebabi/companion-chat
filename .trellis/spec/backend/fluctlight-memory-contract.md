# Fluctlight Memory Contract

## Scenario: Typed Authoritative Memory With Rebuildable Hybrid Retrieval

### 1. Scope / Trigger

- Trigger: the clean-start system records, revises, retrieves, embeds, consolidates, corrects, forgets, or injects Memory into a cognitive prompt.
- Applies to episodic, semantic, relationship, and autobiographical Memory. Working memory is a bounded read model from current Conversation/Cognition facts.
- PostgreSQL rows are authoritative. pgvector and full-text indexes are retrieval mechanisms, not a second source of truth.

### 2. Signatures

```python
record(command: RecordMemory, tx: UnitOfWork) -> Memory
revise(command: ReviseMemory, tx: UnitOfWork) -> MemoryRevision
forget(command: ForgetMemory, tx: UnitOfWork) -> MemoryRevision
retrieve(query: MemoryQuery) -> MemoryContext
embed(command: EmbedMemory) -> EmbeddingResult
```

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
  model_id
  dimensions
  embedding
  status: pending | ready | failed | stale
  embedded_at
```

`MemoryQuery` includes owner Fluctlight ID, authorized Actor/Conversation scope, allowed types, time/status filters, query text/vector when available, result/token limits, and required evidence policy.

### 3. Contracts

- Memory content and revisions commit before embedding work. The same transaction writes an embedding-request outbox intent.
- Embedding failure never deletes or invalidates authoritative Memory. It remains `pending`/`failed` and may be retrieved through authorized metadata/full-text paths.
- Ownership, visibility, Actor, Conversation, type, status, and time constraints are mandatory hard filters. Similarity cannot bypass them.
- Full-text search is lexical candidate retrieval only. It cannot infer intent, relationship meaning, importance, or other semantic state.
- Vector search compares only rows with the same model ID and dimensions.
- Exact vector search is the default. HNSW requires a recorded benchmark showing dataset threshold, latency target, recall target, filter behavior, build cost, and NAS resource impact.
- Hybrid ranking combines authorized FTS/vector candidates with explicit recency, importance, and emotional-significance fields. A bounded LLM reranker may select only from that authorized candidate set.
- Prompt context includes Memory IDs, types, source/evidence references, confidence, and bounded content under a token budget. Retrieved content is evidence, not an unqualified fact.
- Working memory is composed from current Conversation and unresolved Cognition state; do not duplicate complete recent-message history into durable Memory.
- Embedding regeneration changes only the index row. Memory content changes require a Memory revision and mark prior embeddings stale.
- The native `memory_event` capability may omit `emotional_significance`; the
  Runtime normalizes the missing optional signal to `0` (no inferred emotional
  weight) while retaining the required authoritative field on the Memory row.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Memory owner/evidence/Actor scope invalid | Reject before persistence; no embedding intent. |
| Memory transaction commits and embedding Provider is unavailable | Keep Memory authoritative, mark embedding pending/failed, retry asynchronously. |
| Embedding dimensions/model mismatch | Reject index write; never compare incompatible vectors. |
| Query lacks authorized owner/visibility scope | Reject query; do not run FTS/vector search. |
| Similar result belongs to another Fluctlight/Conversation scope | Exclude before ranking and prompt construction. |
| HNSW has no accepted benchmark | Use exact search; do not create/enable the approximate index. |
| LLM reranker unavailable | Return the bounded deterministic authorized hybrid ranking; do not infer new semantic state. |
| Memory content revision accepted | Mark previous embedding stale and enqueue versioned rebuild. |
| Forgotten/superseded Memory appears in an index | Filter by authoritative status; never expose it in prompt context. |

### 5. Good / Base / Bad Cases

- Good: a relationship Memory commits with evidence and visibility, embedding retries after an outage, then becomes searchable without changing the Memory revision.
- Good: a future group Conversation query first restricts Memory visibility to authorized participants, then performs hybrid ranking.
- Base: a new Memory has no embedding yet and is found by Actor/type/time/full-text filters.
- Bad: put Memory only in a vector store, search all vectors then filter ownership in application code, mix embedding models, or treat nearest-neighbor output as confirmed fact.
- Bad: duplicate every recent Message into durable Memory or create a preference from a keyword match.

### 6. Tests Required

- Persistence tests for typed Memory, evidence ownership, revision, correction, forgetting, and rollback with outbox atomicity.
- Embedding tests for pending/retry, idempotency, model/dimensions separation, stale rebuild, and Provider failure without Memory loss.
- Authorization tests proving hard owner/visibility/Actor/Conversation filters run before results enter ranking or prompt context.
- Retrieval tests for metadata, FTS, exact vector, hybrid rank, bounded LLM rerank, result limits, and token budget.
- Benchmark fixture defining the dataset/latency/recall threshold required before enabling HNSW; filtered-query recall must be measured.
- Working-memory tests proving it is bounded and composed without duplicating full Conversation history into durable Memory.
- Prompt tests assert every included Memory retains ID, type, confidence, and evidence/source references.

### 7. Wrong vs Correct

#### Wrong

```python
matches = vector_store.search(query_embedding)
return [m for m in matches if m.metadata.get("fluctlight_id") == fluctlight_id]
```

#### Correct

```python
candidates = memory.retrieve(
    MemoryQuery(
        owner_fluctlight_id=fluctlight_id,
        authorized_actor_ids=actor_ids,
        conversation_scope=conversation_scope,
        allowed_types=allowed_types,
        query_text=query_text,
        query_embedding=versioned_embedding,
        token_budget=token_budget,
    )
)
```
