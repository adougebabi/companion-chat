# Background trigger correction — user instructions 2026-10-08

This supplements the existing PRD/design/implementation scope for the confirmed live queue flood. User explicitly requests implementation and technical fixes; live environment remains read-only.

## Required behavior

- WakeUp: actual last conversation publication resets one per-instance Redis key to10min. Expiry releases one durable cycle; after that trigger repeat every10min. Worker startup with no key triggers once; existing key prevents startup duplication. Keep stable PG intent/cycle and lost-notification recovery, but do not overwrite a valid key on startup or catch up missed ticks with bursts.
- Reflection: once30min after last chat, reset by newer chat. Run only on new unreflected real observations/outcomes; unchanged periodic wakeup/no_op/inspection audit alone must not call Reflection model. Coalesce pending triggers per instance; old superseded/completed epochs cannot revive on generic workflow reconciliation. A genuine new external outcome can contribute evidence, but cannot cause an unbounded recurring timer or make Reflection run inside the last-chat quiet period.
- New cognition job enqueued for the same instance preempts queued and running WakeUp/Reflection: durable supersede + Provider context cancellation + Temporal cancellation where available; late result must not commit. Do this at enqueue boundary (not only after waiting/at processing). Do not cancel unrelated instances. Existing cancellation helpers should be reused and races tested.
- Goal Evaluation is evidence/explicit review driven, with2s coalescing, not a free-running clock. Inspection/control/no_op audit must not continually invalidate Goal review watermarks or auto-trigger assessment. Preserve actual domain queries/actions and source revocation/criteria/Owner reassess and finite omitted-source remainder. Reflection keeps memory/relationship/self learning; existing Goal standard/progress writer is the sole authority. No broad merger or deletion of either Agent.

## Live evidence (sanitized)

GET settings: product.wakeup enabled=true interval_seconds=300; generated_concurrency=1. Latest100 model runs: reflection51, wake_up35, goal_evaluation6, cognitive_assessment7, conversation_summary1. Count is recent model calls, not distinct triggers; ADK/retries may generate several calls per trigger.

Current code: wakeup.go default1800s, user idle slots10m/30m thenconfigured; redis_triggers.go reflectionQuietPeriod10m; agent_result_adapter.persistCommittedWakeUp unconditionally inserts reflection intent and arms timer; recordGoalOutcomeSourceTx records all nonpending receipts, linked ones auto-evaluate; queueGoalReviewsTx uses all source max(id), so inspection audit can reopen same-day Review. Existing CancelLifecycleForCognition marks pending/retry/started/running superseded and requests Provider+Temporal cancellation, but enqueue coverage/startup/generic recovery must be verified.

## Ownership / verification

Production changes limited to Core trigger lifecycle / source qualification and Worker integration as needed, with owning tests. No public API/schema change unless unavoidable. Keep Eino Tool registry, short transactions, source/CAS, outbox/Temporal; no increased model concurrency/input limits, keyword semantic guessing, secret logging, forced Goal completion or production writes. Meaningful tests:10min expiry/startup idempotency; last-chat30min Reflection coalescing/no-change suppression; same-instance queued/running Provider cancellation and late-commit refusal; repeated silent inspections create zero Reflection/Goal model requests while genuine evidence still closes Goal. Use disposable PG/Redis for final integration; SKIP is not PASS.
