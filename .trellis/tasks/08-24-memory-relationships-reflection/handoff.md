# T07 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/memory/` typed Memory authority, revisions,
  rebuildable embedding lifecycle, hard-filtered exact/hybrid retrieval and
  bounded prompt context.
- `apps/core/src/fluctlight_core/relationships/` directed Actor state,
  evidence updates, CAS revisions, append-only governance and rollback.
- `apps/core/src/fluctlight_core/reflection/` explicit T05 proposal closure
  through public Memory/Relationship services.
- Migration `0006_t07_memory_relationships`, metadata registration and Core
  readiness head; T07 contract/architecture tests.

## Implementation Evidence

```text
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests
93 passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src apps/core/tests
All checks passed
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline mypy --follow-imports=skip <T07 sources>
Success: no issues found in 5 source files
```

## Produced Contracts / Schema

- Alembic head `0006_t07_memory_relationships` with Memory, Revision,
  Embedding, directed Relationship, revision and governance tables.
- PostgreSQL authority fields include owner Fluctlight, typed memory,
  Actor/Conversation/event/evidence refs, visibility, status and revision.
- Retrieval applies owner/status/type/conversation/Actor hard filters before
  lexical/exact-vector ranking and prompt budgeting; model/dimension metadata
  prevents incompatible vectors.

## Remaining Risks / Excluded Scope

- Embeddings are stored as rebuildable JSON vector rows because no pgvector
  Python type dependency is currently pinned. T12 must verify the real
  PostgreSQL extension/type path and benchmark-gated HNSW decision.
- Actor materialization and real ForeignKey behavior require the T03/T04
  integration state; T07 does not add compatibility aliases.
- Provider/Temporal failure, real PostgreSQL, cross-module reflection and full
  security acceptance remain T12-only. No keyword or default semantic path is
  introduced; no T08+ modules are implemented.

## T12 Coverage

Re-run `T07-MEM-01` through `T07-MEM-04`, `T07-REL-01`, `T07-REL-02`,
`T07-REF-01`, and `T07-REF-02` from the child brief.

Rollback point: remove only T07-owned paths and migration `0006` before T08 if
the Memory/Relationship contract gate cannot be satisfied.
