# T07 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

memory/relationships modules、typed memories、pgvector/FTS retrieval、embedding workflows、directed Relationship evidence/revisions/rollback、reflection closed loops 与 prompt context.

## Dependencies

T06 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.
- This child owns implementation evidence only. T12 owns all functional acceptance and re-runs the required Memory/Relationship/reflection scenarios; no child PASS is established here.
- Candidate, pending, failed, or stale states are not positive acceptance by themselves; placeholder-only features remain excluded from acceptance.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/commands/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
