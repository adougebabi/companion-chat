# T04 Design Baseline

## Authority

Parent task `08-24-python-core-architecture-refactor` owns architecture and cross-task contracts. This child may refine only implementation detail inside its approved scope after its child-specific brief is reviewed.

## Scope

fluctlights/inner_state modules：identity、personality、behavioral policy、revision/governance、affect/PAD/mood/momentum/regulation、drives、Goals/Intentions core state 与 numeric policy.

## Dependencies

T03 completed and merged.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.

## Readiness Gate

Parent decision D038 is the recorded Owner exception for this session. The child-specific brief, exact manifests, owned/forbidden paths, focused commands, rollback point, report template, and no-history dry run are now recorded in the parent research directory. T03 remains an explicit carry-forward risk rather than a PASS claim. Docker, Compose, long-running process, and full-stack runtime commands are intentionally excluded from this session's validation matrix and must remain listed as deferred.
