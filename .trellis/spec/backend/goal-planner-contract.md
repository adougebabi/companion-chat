# Independent Goal Planner contract

## 1. Scope / Trigger

Incremental set planning after Goal resolution, released capacity, Actor/relationship
context changes, explicit Owner requests, profile activation or bounded recovery.
Preserve the Eino Runner, GoalEvaluation, Stage/Commitment/Intention and Schedule.

## 2. Signatures

- `ApplyOwnerGoalCommand(ctx, actor, owner, goalID, GoalOwnerCommand)`.
- `ApplyGoalSetCommand(ctx, actor, owner, GoalSetCommand)`.
- `RunGoalPlannerAgent(ctx, owner, actor, profile, runRef, mode)`.
- `ProcessGoalPlanningIntent(ctx, runID)`; `RepairGoalPlanning(ctx, limit)`.
- `GoalSetCommand`: expected_version, expected_facts_revision, idempotency_key,
  reason, changes, order, dependencies, reviewed_goal_ids, source_reviews and
  Owner-only recovery_policy / automatic policy settings.
- Native Tools: `goal_planner.query` and `goal_planner.commit`, private surface
  `goal_planner`; FormalAgent `goal_planner`, schema `goal_planner_v1`, 12 cycles.
- Internal/browser resources: `goal-set`, `goal-planning`, `actor-context/:actorId`.
- Schema head `0054_goal_planner`, additive from `0053_goal_reconciliation`.

## 3. Contracts

1. Count all instance active Goals, including blocked/waiting execution. Never count
   Stage/Commitment/Intention. Policies only tighten maximum 5; candidate bound defaults 20.
2. Every Goal write shares `lockLifeContextTx` and a PostgreSQL trigger guard. Model
   calls happen after claim transaction commit. Count-then-unlocked-insert is forbidden.
3. Acquire Life lock before reading idempotency ledger. Same-key concurrent replay
   returns the same stored IDs and final result; immutable input maps must not be mutated.
4. Apply a final set atomically. Dependencies are same-instance AND prerequisites;
   reject self/cycle and private-to-shared edges. `derived_from` is only provenance.
5. Preserve manual relative order. Scoped automatic reorder reuses visible rank
   positions and never changes hidden Goal rank. New/resumed Goals append by default.
6. Owner protected/pause/cancel cannot be undone by Planner. Legal unprotected system
   pauses can resume; new actions still recheck lifecycle, dependencies and current facts.
7. Source imports link existing terminal/paused/cancelled IDs. Legacy desires/goals
   leave current persona projections; preserve original source and require semantic
   review, explicit source association or non-active candidate. Never regex-delete prose.
8. Query Tool has no mutation seam. Commit uses trusted run target and current
   mode/Owner/profile/policy/facts/claim/lease. Model parameters cannot grant authorization,
   change progress/evidence/terminal state or widen policy. Only one accepted commit per run.
9. `commit_succeeded` recovers even pure sort/dependency/review commits with empty
   applied[]; do not infer persistence from model text or array length.
10. Store processed_run_id per event. Consume only current/shared-profile events
    captured before the claim watermark. Private events stay pending until activation.
    Request insertion uses the Life lock; sequence allocation cannot outrun its commit.
11. Policy-blocked work stays durable. Recovery checks current permission; failed
    attempts cap retries; unchanged/no-viable snapshots do not create fixed model calls.
12. Defaults: merge 2s, retry 60s, lease 300s, max attempts 5, candidates 20.
    Owner can configure bounded recovery policy; model cannot. Existing queued deadlines
    keep their original merge window. Failed Tool decisions are retry, never no-viable.

## 4. Validation / Error Matrix

| Condition | Result |
|---|---|
| Sixth activation / full candidate storage | Domain conflict; no hidden candidate fallback |
| Stale collection/facts/claim or inactive instance | Reject mutation; bounded reread/retry |
| Closed auto planning or suggestions run | Commit denied, existing authorized execution unchanged |
| Missing meaningful no-candidate reason/review condition | Invalid Agent contract |
| Invalid dependency/order/source decision | Whole transaction rollback |
| Committed run repeats another command | Reject; replay original key returns original result |
| Owner Actor context stale version | HTTP 409; keep browser draft |
| Old over-capacity stock | Preserve rows, show violation, block growth until Owner governance |

## 5. Good / Base / Bad Cases

Good: actual expression completes three Goals, merged Planner reads real tools,
creates one finite new Goal, original evidence chain completes it. Base: no viable
candidate preserves an empty set and review condition. Bad: re-inject desires on
persona compilation, fabricate Actor acceptance, or call new Goal active while only
queued a wish. Owner location/timezone edits conservatively review untargeted Goals;
precise per-fact dependency selection is a remaining refinement.

## 6. Required tests

`TestGoalPlanner*`, actual PostgreSQL migrations/HTTP, native scripted Provider query→
commit→reread, same-key/capacity/cycle races, Source review replay, pure commit recovery,
private profile pending events, old due dependency gates, actual original closure.
`TestGoalJointPostgresRedisTemporalWorkerRestart` uses task-owned PG/Redis/Temporal.
`TestFormalAgentE2E/goal_planner` is real Provider, never equate its parent SKIP with PASS.
Update both OpenAPI artifacts/generators and run route inventory, Web typecheck/test/build.

## 7. Wrong / Correct

Wrong: `count(active); insert sixth; retry model after response loss; clear all events`.
Correct: `Life lock → ledger replay → version/fact/claim checks → final set transaction
→ durable commit marker → consume captured eligible event IDs → existing progression`.

### Operational recovery policy

`GoalSetCommand.recovery_policy` is Owner-only, with merge_window_seconds 1..60,
retry_backoff_seconds 5..3600, max_attempts 1..10, lease_seconds 60..600 and
candidate_limit 1..20. All are stored on goal_set_policies and CAS-protected.
Use database time for request availability and lease expiry; comparing the host
clock with a database timestamp introduces millisecond/VM-drift flakes.
Model scheduled wishes without active goal_ref return planning_requested and do
not call Schedule generation. Ordinary cognition reads a compact goal_policy
capacity summary, retaining the same physical model-call count.
