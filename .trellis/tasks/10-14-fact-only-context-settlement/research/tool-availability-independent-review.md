# F18 independent read-only review closure

Two clean, one-round agents reviewed the new implementation. Neither edited code or ran external services. Main owned all follow-up choices and modifications.

## Runtime findings and resolution

1. Preclaimed native due attempts could remain running because handled suppression skipped all settlement. Removed the blanket skip; existing suppressed settlement closes a durable attempt, requeues without failure-count increment, and does not count Goal evidence. `TestToolAvailabilityHandledClosesPreclaimedDueAttempt` passed on real isolated PG.
2. Recovering typed stale errors discarded refresh and reply rebase signals. Core receipts now carry controlled refresh metadata; a stale-reply anchor requires Core-generated marker plus no-effect fields. Ordinary lease/conflict no-ops do not obtain that anchor. `TestADKStaleReplyRefreshesContextWithoutPublishingOldProse` passed including obsolete-authority rejection.
3. Canonical Execute missed a marker returned by its own fresh preparer. Canonical and Core check again after Prepare and cancellation; actual Execute stays uncalled. Canonical regression passed.
4. Final failure after an attempted handled path remains a real failed result; mixed PostgreSQL/network/transaction faults are not softened by a sibling business-conflict sentinel.

## Remaining ordinary semantic gates and resolution

Closed exact-code/error-tree recovery now covers Memory correction/active lifecycle target/time errors, generated Schedule/Activity business-plan validation, Core scene/presence context gaps, affect normalization and relationships. Real DB/Provider/cancel/owner failures remain truthful. Persona no-effect decisions use technical Tool receipts without a fabricated mutation audit ID. Duplicate wear/habit/use and existing return proof have explicit no-op handling. Domain code is never repaired by inventing a target, current fact, completed action, asset or private claim.

## Evidence limits

49 names are a fixed inventory/audit, not 49 real external executions. Default full Go and full Core/Workflow race run with PG/live-dependent skips; focused PG/race actually executes the new recovery paths. Full PG baseline/current explorations and the passing follow-up of current-only changed cases are recorded in verification/tool-availability-results.json. No live Provider, object store, ComfyUI, Redis, Temporal, push or deployment was performed. The final source snapshot excludes another live task's private-chat changes, including a mixed shared reply file reconstructed from baseline plus this task's own hunks.
