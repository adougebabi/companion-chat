# T12 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

全 capability matrix e2e/failure/security/backup 验收、一次性切换、旧 server/web/test/SQLite/jobs/routes/dependencies/docs/CI 删除、唯一生产实现确认.

## Dependencies

T01B and T02-T11 completed, merged and parent integration review approved.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12 final acceptance passes; the one-time cutover occurs in T12 after all required gates and scope guards pass.
- Future-only, reserved, and placeholder-only capabilities are excluded from positive acceptance; T12 runs only a negative scope guard for them.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/commands/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
