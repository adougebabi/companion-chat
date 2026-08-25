# T05 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

cognition module、durable sequenced inbox、single cognitive writer、two-stage assessment/freeze/realization、reflection framework、diagnostic tables/query/redaction/retention/correlation.

## Dependencies

T04 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.
- This child owns implementation evidence only. T12 owns all functional acceptance and re-runs the required cognition/diagnostics scenarios; no child PASS is established here.
- Future-only, reserved, and placeholder-only capabilities are excluded from positive acceptance; real Required lifecycle states remain T12 scenarios.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/commands/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
