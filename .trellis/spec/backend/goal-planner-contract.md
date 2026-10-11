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
- Schema head `0059_fact_generation_provenance` includes the additive Kev adapter and logical-run coordination. Planner cadence remains `0055_goal_planner_cadence`, additive from released `0054_goal_planner`; the original Planner bundle remains additive from `0053_goal_reconciliation`.

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

### Automatic cadence and evaluation failures (0055)

`request_goal_planning(owner_id, source_id, why, payload)` persists hints but only
Owner requests, real Actor/profile review, or primary triggers enqueue work.
Primary triggers are lifecycle capacity release, accepted daily Schedule, initial
startup and authorized re-enable/recovery; they require instance active count
below capacity. Claim repeats the same gate for old queued work. Full Owner
requests use suggestions; real Actor/profile changes may review an existing full
set. Ordinary cognition/reflection wishes do not dispatch an independent run.

`acceptScheduleWithTx` requests `schedule_accepted_daily` with source identity
`schedule:<IANA timezone>:<local date>` after formal acceptance, in the same
transaction. Replay/replan uses the same daily event. Do not use schedule revision
or random ID as the daily planning identity. Recovery retains late/private events
and recovers stranded review events directly; do not convert a full-set Actor
review into a capacity-blocked startup event or generate new events every minute.

| State/error | Durable behavior |
|---|---|
| Full automatic supplementation | No model request; old queued run becomes capacity_full, trigger remains durable |
| Ordinary wish only | Retain hint; no automatic dispatch, old queued run awaits a primary trigger |
| Missing assessment goal coverage | One explicit replacement-object correction via the existing formal task |
| Correction still lacks coverage or enumerated fixed wire/scope error | failed with original cause; no success memo or evidence consumption |
| Planner final without consumed query / unbacked commit claim | failed with precise contract error; replay makes no new model call |
| Transient Provider failure / stale authoritative facts | Existing bounded retry and backoff; never write success memo |
| Successful memo with same semantic authority/sources/conditions | succeeded / assessment_memo_match; no physical model call |

Required tests: `TestGoalPlannerCadenceFullAutomaticAndWishAreQuietButOwnerCanSuggest`
(full repair twice creates no events; stranded full Actor review recovers),
`TestGoalPlannerFinalWithoutQueryStopsInsteadOfRetryingSameRun`,
`TestGoalAssessmentMissingCoverageGetsOneTypedCorrection`,
`TestGoalAssessmentRepeatedMissingCoverageFailsTerminally`,
`TestGoalAssessmentFailureNeverCreatesMemoAndRetriesModel`,
`TestAcceptScheduleUsesDatabaseIdempotencyBoundary`, and true 0054 function restore
then 0055 migration upgrade/reentry. Keep original authority/source rollback
assertions in malformed-output tests even when request status changes to failed.

Wrong: full set + every wish/replan -> new model call; bad coverage -> five blind
identical attempts -> retry_exhausted.
Correct: durable trigger + capacity/semantic admission -> bounded formal task ->
coverage correction or precise failure; new facts/Owner action may start new work.
`provider_request_failed` is a physical request error, not duplicate skip. An Agent
final_message/completed diagnostic is not proof of accepted Goal authority.


## Scenario: Native query phases and bounded held-context recovery (2026-10-11)

### 1. Scope / Trigger
A Planner may return summary before consuming queries, leaving active Goals blocked by context_review_required. Only a legitimate Planner commit may clear reviewed_goal_ids.

### 2. Signatures
`goalPlannerPhysicalRequestPolicy` and `goalPlannerQueryCoverage(trace)` inspect distinct completed native query receipts and physical ModelCallSequence. `RepairGoalPlanning` invokes native_query_v2 held-review recovery through existing normal planning events/runs.

### 3. Contracts
Until two distinct successful queries exist, expose only installed goal_planner.query, force its actual native call and omit final grammar. Once query coverage exists, permit the full private catalog and final output; semantic no_change/awaiting_facts stay legal. An accepted commit requires a real snapshot query in a later physical call before final; a query in the commit batch is insufficient. No forced creation or commit, no fake query receipts and no second Runner.

Recovery creates one stable versioned trigger per held semantic Goal/context signature, not evaluation revision. Signature includes genuine Actor/profile event sequence and excludes recovery's own event. Current auto-planning/permissions/profile/locks/backoff/max_attempts apply. Existing pending work, committed runs and unrelated failures remain untouched; old query_results_missing failure history is immutable. A semantically justified reviewed_goal_ids commit clears the hold through the original sole path.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| Failed or duplicate native query | does not increase coverage |
| Query beside commit | does not count as post-commit reread |
| Current query coverage, no viable direction | normal semantic final allowed |
| Only assessment revision changes | no new recovery budget |
| Existing commit / different failure / permission off | no blind restart |

### 5. Good / Base / Bad Cases
Good: query actual constraints → commit reviewed IDs if valid → later snapshot → autonomous handoff becomes eligible. Base: unresolved changed context stays held. Bad: clear review from GoalEvaluation prose or reset the old failed run.

### 6. Tests Required
`goal_planner_request_policy_test.go` actual HTTP Generate/Stream covers rejected query, repeated IDs, same-batch rejection, later snapshot and identity headers. PG recovery tests assert stable semantic recovery, revision-only non-requeue, backoff/policy and immutable failed/committed history.

### 7. Wrong vs Correct
Wrong: always use summary grammar, then fail query_results_missing; restart after every Goal revision. Correct: require native query evidence and use one bounded recovery per real held context.
