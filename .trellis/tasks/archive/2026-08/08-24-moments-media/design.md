# T09 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

moments/media modules、feed/comment/reaction/visibility/unread、S3/MinIO assets、image/video/ComfyUI/h3、quality/progress/compensation/tombstone/orphan/Range/backup.

## Dependencies

T08 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.
- Media implementation must expose heartbeat/cancel/idempotent live crash-recovery seams with the actual adapter; T12 owns the final live correctness validation. Fixed-duration soak/resource research is excluded from both child evidence and final acceptance.
- Placeholder-only media features are excluded from positive acceptance; Required media lifecycle states such as placeholder/progress/result/failure remain T12 scenarios.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/commands/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
