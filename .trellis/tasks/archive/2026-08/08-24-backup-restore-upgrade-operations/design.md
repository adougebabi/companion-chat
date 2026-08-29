# T11 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

PostgreSQL+object+.env backup/restore manifest、integrity、Alembic/Temporal active-workflow upgrade、diagnostics/media cleanup、NAS operator docs 与 recovery drills.

## Dependencies

T10 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.
- Operations implementation must expose active workflow resume from restored Temporal default+visibility stores; T12 owns the final proof. Fixed-duration/resource soak is excluded by parent D036 from both child evidence and final acceptance.
- Placeholder-only operational procedures are excluded from positive acceptance; Required restore/recovery paths remain T12 scenarios.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/implementation-check commands/T12 coverage IDs/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
