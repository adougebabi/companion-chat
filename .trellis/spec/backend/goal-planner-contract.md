# Independent Goal Planner contract

## 1. Scope / Trigger

Incremental set planning after formal Goal completion or one local-day planning
episode, with real free Goal capacity required for every model invocation. Owner
requests also require capacity. Actor/relationship/profile changes remain context
hints and cannot create an additional automatic episode.
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
- Schema head `0063_goal_agent_cadence` adds operational cadence after released `0062_reply_media`. The released `0054_goal_planner` and `0055_goal_planner_cadence` bundles remain immutable; the current installer supersedes their automatic admission predicates after every Apply.

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
    attempts retain their episode admission; retry/repair cannot grant fresh automatic
    budget. No-viable output does not justify an extra same-day model call.
12. Defaults: merge 2s, retry 60s, lease 300s, max attempts 5, candidates 20.
    Owner can configure bounded recovery policy; model cannot. Existing queued deadlines
    keep their original merge window. Failed Tool decisions are retry, never no-viable.

## 4. Validation / Error Matrix

| Condition | Result |
|---|---|
| Sixth activation / full candidate storage | Domain conflict; no hidden candidate fallback |
| Stale collection/facts/claim or inactive instance | Reject mutation; bounded reread/retry |
| Closed auto planning or suggestions run | Commit denied, existing authorized execution unchanged |
| Full capacity, including Owner/review/old queue/retry | capacity_full without a model call; keep events |
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

## Scenario: Durable Goal cadence and vacancy admission (0063, 2026-10-14)

### 1. Scope / Trigger
Repeated Planner calls after Actor/profile changes or recovery bypassed the user's
completion/daily rule. Every invocation now requires active Goal count below
max_active_goals; there is no full-set Owner/review/suggestions exception.

### 2. Signatures
- `request_goal_planning(owner_id, source_id, why, data)` retains all hints.
- `goal_planner_local_day(owner_id) -> text` uses DB time and instance IANA zone.
- `goal_planner_episode_keys(owner_id, profile, watermark) -> episode_key rows`.
- `ProcessGoalPlanningIntent`, `RepairGoalPlanning` and final
  `admitGoalPlannerModel(ctx, owner, actor, profile, runRef, mode)` repeat admission.
- Migration `0063_goal_agent_cadence` is additive from `0062_reply_media`.

### 3. Contracts
`goal_planner_admissions(fluctlight_id, episode_key, run_ref, admitted_at)` records
actual native Agent admission. `daily:YYYY-MM-DD` is available only with a visible,
unprocessed captured event within the run watermark for the current local day:
schedule_accepted_daily, daily_cadence_due, initialization_completed,
auto_planning_enabled or startup_recovery. Schedule/repair use payload.local_date;
boot events use their capture time in the instance zone. A date alone, ordinary
wish, Actor/profile/relationship change, pause/cancel/abandon or context repair
cannot manufacture a daily budget.

Formal completed lifecycle events and authenticated Owner events use exact
`event:seq` keys. Completion must still name an actually completed owned Goal.
Private profile events retain individual eligibility; never replace this ledger
with a global max watermark. One run may admit several currently visible episode
keys. Failure keeps those keys spent; a new completion, local day or explicit
Owner event is needed for more model work. Same-day retries do not reset budget.

The final Agent boundary validates its processing row, claim revision, lease,
Owner actor, profile/mode and current real capacity before writing admission.
Full capacity returns `capacity_full`/model_admission=false; spent budgets return
`awaiting_trigger` without consuming events. A recovered durable commit returns
before any new admission or model call. Owner requests with free capacity keep
explicit behavior, including suggestions when automatic planning is disabled.

The existing repair lifecycle creates one stable daily_cadence_due event with
local_date when the day is unspent and capacity is free, including no-new-event
and restart cases. It never resets failed/committed history. Released 0054/0055
SQL remains unchanged; the new installer runs after old installers on every Apply.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Active count reaches capacity, any entry | no model; capacity_full and retained hints |
| Formal completion + slot | one unspent completion episode may run |
| Current captured local-day event + slot | one daily episode may run |
| Actor/profile/wish/pause/cancel only, no allowed event | no independent planning episode |
| Yesterday's schedule or repeated same-day repair | no fresh daily budget |
| Already admitted automatic episode fails | retain cause and spent ledger; retry cannot spend again |
| Pending private event activates later | exact event remains eligible, subject to capacity |
| Previously committed run is replayed | recover stored result, no model |

### 5. Good / Base / Bad Cases
Good: completing one Goal frees a slot and triggers one merged planning episode.
Base: a full set stays quiet through daily repair and Owner requests. Bad: switch
profiles repeatedly to call Planner, or let an ordinary hint borrow a date-only
budget before a legitimate daily event exists.

### 6. Tests Required
`goal_agent_cadence_test.go` covers full sources/old queue/direct Agent admission,
failed day/private episodes, captured daily hints and due repair. Migration tests
restore old installers and 0062 head, then Apply twice without resetting clocks,
admissions, failed runs or private history. These are real-PG tests and SKIP without
an explicitly disposable DB; compilation is not SQL/persistence acceptance.
Retain native query/commit/reread, CAS and original completion evidence tests.

### 7. Wrong vs Correct
Wrong: full Owner/review requests still call the model, or `daily:<today>` always
exists in episode_keys. Correct: real capacity + captured legitimate event +
unspent per-event/day admission, repeated at the final native Agent boundary.


## Scenario: Native query phases and bounded held-context recovery (2026-10-11)

### 1. Scope / Trigger
A Planner may return summary before consuming queries, leaving active Goals blocked by context_review_required. Only a legitimate Planner commit may clear reviewed_goal_ids.

### 2. Signatures
`goalPlannerPhysicalRequestPolicy` and `goalPlannerQueryCoverage(trace)` inspect distinct completed native query receipts and physical ModelCallSequence. `RepairGoalPlanning` invokes native_query_v2 held-review recovery through existing normal planning events/runs.

### 3. Contracts
Following F18 availability, query coverage and post-commit reread are advisories;
installed private Tools stay callable. A query in the commit batch is still not
a consumed snapshot or post-commit reread. Semantic no_change/awaiting_facts stay
legal. No forced creation or commit, no fake query receipts and no second Runner.

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

## Scenario: Recoverable Planner commit metadata and context review (F19)

### 1. Scope / Trigger
An apply Planner submitted context_review_required as a prerequisite ID and then
reported a handled needs_input as blocked_by_policy. Fix representations that
can be corrected without inferring a semantic review or Goal completion.

### 2. Signatures
`normalizeGoalPlannerDependencies(GoalSetCommand)` applies only to private runs.
`withGoalPlannerInitialSnapshot` captures the exact initial request fence;
`bindGoalPlannerMetadata` supplements omitted technical fields.
`goalPlannerCommitFeedback` and `reconcileGoalPlannerHandledSummary` retain
actionable receipt truth in Tool feedback, diagnostics and final run result.

### 3. Contracts
dependencies is an atomic replacement map of actual Goal IDs to actual
prerequisite IDs. The exact context_review_required token is state metadata:
omit it with planner_context_review_flag_ignored. An all-metadata entry leaves
stored prerequisites intact; an explicitly empty list still clears them.
Never infer reviewed_goal_ids or clear a hold from this normalization. A genuine
review-only commit is reason + reviewed_goal_ids with consumed snapshot fences;
it needs no changes, source, new Goal or criteria. Empty initial criteria with
needs_reflection=true remain valid and unresolved after context review.

Bind technical defaults only with a real physical call identity: use a completed
earlier physical snapshot query, otherwise the immutable fence Core put in this
run's initial prompt. Never read the latest DB to upgrade intent, never consume
a same-batch query, never overwrite an explicit stale value. CAS still decides
whether that observed snapshot remains current.

Recoverable domain invalid input has specific codes and fixed safe guidance,
including goal_set_order_incomplete / goal_set_order_duplicate. ADK handled
events include warning_code, disposition and both no-effect fields; successful
Planner normalization warnings persist in the committed receipt. Final
tool_warnings preserves causes. A last handled/no-effect commit summarized as
failed/blocked_by_policy is reconciled to awaiting_facts; actual refusal or
committed business results are never converted to handled.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| context flag mixed into dependencies | warn, ignore metadata, execute valid command atomically |
| only context flag for one entry | preserve stored dependencies and hold unless explicitly reviewed |
| omitted technical fences | bind observed initial/earlier-query snapshot, then ordinary CAS |
| explicit stale fence | awaiting_state, no write; no upgrade to latest |
| incomplete/duplicate order | handled needs_input with exact code/guidance, rollback review too |
| foreign/profile/private-claim violation | actual authority refusal, no durable domain change |
| dependency SQL failure | propagate the real database cause, never mislabel unauthorized |

### 5. Good/Base/Bad Cases
Good: inspect a valid existing Goal, commit reviewed_goal_ids, persist false hold
and reread. Base: flag-only metadata preserves the hold and true prerequisites.
Bad: convert the flag into a completed semantic review or populate initial empty
criteria with Agent-invented joke/photo/relationship thresholds.

### 6. Tests Required
`goal_planner_commit_recovery_test.go`: pure metadata immutability, earlier/same
physical-call fences, explicit stale values, real PG review-only initial Goal,
dependency preservation, transactional rollback, foreign resource, warning
diagnostics, receipt replay and scripted native read→commit→reread. Re-run 49-name
registry inventory: Tool descriptions must satisfy the existing 512-rune limit.
No PG SKIP or controlled Provider result counts as real-model acceptance.

### 7. Wrong vs Correct
Wrong: a flag becomes an authorized Goal ID, then missing fields produce an
opaque no-effect success. Correct: warn on known metadata, use observed fences,
commit only explicit valid operations and expose exact unresolved causes.
