# T10 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

完整 Vue contacts/groups/create/detail/settings/governance/Moments/Diagnostics/Control Center、generated browser client、responsive/accessibility 和供 T12 使用的 capability inventory browser scenario seams.

## Dependencies

T09 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.
- This child owns implementation evidence only. T12 owns all browser, accessibility, diagnostics and cross-module acceptance; no child PASS is established here.
- Placeholder-only pages/features are excluded from positive acceptance; required lifecycle states remain T12 scenarios.

## Readiness Gate

Before `task.py start`, parent must add exact decisions/manifests/paths/commands/rollback and pass a no-history handoff dry run. Until then this document is program-level planning only.
