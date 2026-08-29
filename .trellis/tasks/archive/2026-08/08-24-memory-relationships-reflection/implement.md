# T07 Memory / Relationship / Reflection Implementation Brief

## Status

Parent-authorized implementation brief for the third executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T06 Conversation public contracts and handoff are available. T07 consumes
Conversation/Actor references through values and does not query conversation
repositories directly.

## Owned Paths

- `apps/core/src/fluctlight_core/memory/**`
- `apps/core/src/fluctlight_core/relationships/**`
- `apps/core/src/fluctlight_core/reflection/**`
- `apps/core/migrations/versions/0006_t07_memory_relationships.py`
- `apps/core/src/fluctlight_core/migrations/**` only if a T07 helper is needed
- `apps/core/src/fluctlight_core/migrations/env.py` (T07 schema import only)
- `apps/core/src/fluctlight_core/transport/api.py` (readiness head only)
- `apps/core/tests/memory/**`, `apps/core/tests/relationships/**`,
  `apps/core/tests/reflection/**`, `apps/core/tests/contract/test_t07_*.py`,
  `apps/core/tests/architecture/test_t07_*.py`

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**` and root legacy runtime files.
- T08-T12 domain modules and migrations; T07 exposes public ports only.
- Direct imports of FastAPI, Temporal, Redis, S3, Provider SDKs or Conversation
  repositories from domain contracts/services.
- A second vector database, semantic keyword classifier, or HNSW index without
  an accepted benchmark.

## Decisions And Contracts

Implement without changing D002, D009-D017, D020-D022, D029-D033, and D039.
The assigned contracts are `fluctlight-memory-contract.md`,
`fluctlight-cognitive-runtime.md`, `fluctlight-provider-contract.md`, and
`fluctlight-persistence-contract.md`. PostgreSQL rows remain authoritative;
embedding rows are rebuildable and only rows with matching model/dimensions are
compared. Authorization owner/visibility/Actor/Conversation/status/time filters
must be applied before lexical/vector ranking and prompt construction.

## Implementation Checklist

1. Add typed Memory and Relationship value objects with evidence, visibility,
   revision and directed Actor references.
2. Add authoritative Memory/Embedding/Revision and Relationship/Revision/
   Governance tables plus linear migration `0006`.
3. Implement record/revise/forget/embed/retrieve/prompt-context APIs, exact
   bounded hybrid ranking and explicit pending/failed/stale embedding states.
4. Implement directed Relationship evidence updates, revision CAS, append-only
   governance and rollback as a new revision.
5. Add reflection coordinator ports that consume T05 evidence windows and apply
   typed memory/relationship candidates without reading foreign tables.
6. Add contract/architecture/unit checks; real pgvector/PostgreSQL and
   cross-module reflection acceptance remains T12.

## Implementation Checks

```bash
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src/fluctlight_core/memory apps/core/src/fluctlight_core/relationships apps/core/src/fluctlight_core/reflection apps/core/tests/memory apps/core/tests/relationships apps/core/tests/reflection apps/core/tests/contract/test_t07_*.py
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests/memory apps/core/tests/relationships apps/core/tests/reflection apps/core/tests/contract/test_t07_*.py
```

## T12 Coverage IDs

`T07-MEM-01` typed record/revision/forget with evidence and outbox;
`T07-MEM-02` hard authorization filters before FTS/vector ranking;
`T07-MEM-03` embedding pending/failure/model-dimension/stale rebuild;
`T07-MEM-04` bounded hybrid ranking and prompt context provenance;
`T07-REL-01` directed Actor relationship and evidence trend;
`T07-REL-02` revision CAS/rollback/governance; `T07-REF-01` reflection
watermark plus memory/relationship proposal closure; `T07-REF-02` stale
proposal rejection.

## Rollback Point

Before T08 starts, revert only T07-owned paths and migration `0006` if the
Memory/Relationship contract gate cannot be satisfied. Preserve T05/T06 and
prior unrelated worktree edits.

## Implementation Evidence Handoff

Record changed paths, contract/schema artifacts, implementation-check
commands/results, remaining risks, excluded scope, T12 coverage IDs and the
rollback point. State `acceptance_owner=T12` and `acceptance=pending`; no child
PASS, production readiness or cutover is established here.
