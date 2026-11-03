# T07 No-History Handoff Dry Run

Date: 2026-08-25

T07 consumes the T06 Conversation/Actor public boundary and owns the Memory,
Relationship and reflection-closure modules. Its only shared integration files
are the migration metadata import and readiness head listed in `implement.md`.

## Execution

1. Define typed Memory/Embedding/Relationship/revision contracts with evidence
   and visibility fields.
2. Add authoritative PostgreSQL tables and migration `0006`.
3. Implement hard authorization filters before lexical/exact-vector ranking,
   bounded prompt context and versioned embedding status transitions.
4. Implement directed Relationship CAS, append-only governance and rollback.
5. Consume T05 reflection proposals through public service ports and apply
   memory/relationship candidates without foreign-table access.
6. Run focused Python checks; hand real pgvector, PostgreSQL, provider failure
   and cross-module acceptance to T12.

## Exclusions / Risks

HNSW and a real pgvector SQL type remain disabled until T12 has a benchmark and
runtime extension gate. T07 does not implement Life World, Media, UI, backup or
legacy deletion, and does not use keyword/heuristic semantic inference.

Conclusion: T06 handoff plus the assigned contracts resolve the planning
boundary required to start this child.
