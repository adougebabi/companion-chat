<!-- Verbatim planning context snapshot from .trellis/spec/backend/fluctlight-autonomy-contract.md; part 2/2. Read original source before modifying its contract. -->

- A qualified Intention creates one revision-scoped `intention.trigger`
  workflow intent. A time trigger sleeps in Temporal history until `due_at`;
  event triggers compare typed event identity; semantic triggers only react to
  the existence of a new processed fact and never inspect keywords.
- Trigger maturity writes one stable `agency.intention_due` fact and attempt
  identity. That fact re-enters the ordinary Main cognition/Capability path;
  it never executes an action directly.
- A decision serving the due Intention must cite both Goal and Intention opaque
  refs. The primary completed/failed/cancelled/suppressed ActionOutcome
  mechanically settles exactly one attempt. An accepted virtual activity stays
  pending until its later confirmed result settles the external-ref Outcome;
  start alone cannot complete the Intention. Only a completed, Goal-bound
  Outcome may support a Reflection V2 Goal progress proposal.
- `intention.schedule` is the controlled path from a future virtual action to
  an accepted Schedule item. It commits or reuses the Goal and Intention in
  the same transaction as the versioned item and time trigger. A typed action
  link, not Schedule prose, grants execution. At due time Core rechecks the
  accepted version, action plan, time and Intention status before starting.
  `agency.intention_due` without such a link still enters Native Cognition.
- The scheduled activity's confirmed completed Outcome can settle its linked
  Intention and a sole-criterion Goal. Failure pauses the scheduled Intention
  for reassessment. Cancellation of the accepted Schedule cancels open linked
  intentions and a Goal with no other open intentions. A stale link or a
  previously cancelled run cannot be replayed into a new activity.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Free-form trigger/code/keyword expression | Reject; only time/event/semantic typed shapes are valid. |
| Same trigger revision is replayed | Return the same due fact, inbox and attempt IDs. |
| Intention expires before trigger apply | Append `expired`; create no due fact/action. |
| Due decision omits Goal/Intention influence refs | Fail before freeze; do not execute a Capability. |
| Failed/cancelled/suppressed Outcome | Requalify or cancel according to mechanical policy; never advance Goal progress. |
| Same primary Outcome is replayed | Reuse the stored attempt settlement; no second revision. |
| Planner fails or its selected slot is invalid | Commit no Goal, Intention, accepted Schedule or trigger; report a Tool failure. |
| Scheduled link is stale or the accepted version was cancelled | Do not start or complete an activity from that link. |

### 5. Good / Base / Bad Cases

- Good: a time Intention sleeps durably, emits one due fact, executes one
  Capability, settles one attempt and later advances Goal progress from its
  completed Outcome.
- Base: a due cognition defers/no-ops; the attempt settles suppressed and the
  Intention becomes eligible for later reassessment.
- Bad: poll message text for intent keywords, execute from the timer callback,
  or mark Goal complete because assistant prose says it succeeded.

### 6. Tests Required

- `TestIntentionTriggerWorkflowUsesDurableTemporalTimerBeforeActivity`.
- `TestIntentionTriggerProductionFlowCreatesDueFactAndSettlesFromOutcome`.
- Goal progress tests must cover completed versus failed/suppressed Outcomes,
  criterion indexes, Goal binding, CAS and replay.
- Scheduled activity PostgreSQL tests assert one atomic Goal/Intention/item
  commit, duplicate-plan reuse, due start once, cancellation/replan rejection,
  and confirmed result settlement of the sole-criterion Goal.

### 7. Wrong vs Correct

#### Wrong

```go
if strings.Contains(message, "remind me") { executeAction() }
```

#### Correct

```go
due := ProcessIntentionTrigger(ctx, intentionID)
// due writes a cognition fact; the normal Main cognition chooses the action.
```

## Scenario: WakeUp formal Agent boundary

### 1. Scope / Trigger

`WakeUpWorkflow → ProcessWakeUpActivity → App.ProcessWakeUp` starts the formal
WakeUp Agent with its stable cycle/run identity.

### 2. Signatures

`RunWakeUpTask` delegates to the registered Agent; `ExecuteTool` owns every
native business call. `agent_runs` and `tool_executions` retain recovery facts.

### 3. Contracts

The shared Eino loop owns feedback. Catalogs select defaults, not execution
permissions. Autonomy policy is explicit and rechecked per operation. Later
policy revocation blocks new operations without erasing prior receipts.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Valid no-op | Complete lifecycle without fabricated output |
| Business rejection | Feed reason back to Agent |
| Cancellation/invalid final after commit | Failed run, retained effects, no whole-run replay |
| Repeated completed cycle | Read existing result; no new Provider or publication |

### 5. Good/Base/Bad Cases

Good: committed reply receipt is consumed by the next decision and handler.
Base: quiet hours reject external publication while internal cognition proceeds.
Bad: return deferred for all writes or execute trace calls again in the worker.

### 6. Tests Required

Real PostgreSQL tests cover policy denial/revocation, same-run budget reservation,
no-op, duplicate suppression, committed-effect recovery and run replay. Controlled
model evidence is separate from strictly serial real Provider acceptance.

### 7. Wrong vs Correct

Wrong: caller freezes native ToolCalls for a second executor.
Correct: native Runner executes Tools; caller records committed outcome.

## Scenario: Linked Goal Execution Admission And Durable Attempts (0050)

### 1. Scope / Trigger
A due cognition is claimed, a direct/scheduled Tool starts a linked activity,
Goal lifecycle/standards change, or an Attempt lease/callback deadline elapses.

### 2. Signatures
`prepareNativeDueAttempt(ctx,inboxID,fluctlightID)`,
`admitGoalLinkedToolTx(ctx,tx,ToolExecutionRequest)`,
`ReconcileGoalAttempts(ctx,limit<=100)`; migration `0050_goal_execution`.
Goal rows add `criteria_version`, persisted `criterion_ids`, `deadline_policy`.
Attempt rows retain running/waiting/needs_reconciliation and terminal states,
operation identity, lease/deadline, wait_ref, reconciliation count and result.
Intention rows retain next_attempt_at/retry_count/retry_reason/goal_hold_status.

### 3. Contracts
- Local linked admission and governance share Life -> inbox/domain-row lock order.
  Provider work stays outside transactions; this is not external atomicity.
- Claim precedes projection/Provider preparation. Failure records Attempt and
  failed inbox together. New retry has a new Agent/Attempt, retaining committed
  receipts; never replay the failed native Agent wholesale.
- Active Goal, matching instance/profile, live revisions, permissions, window,
  typed time maturity and next_attempt_at gate new effects. Replayed receipts
  remain historical facts and do not constitute a new execution.
- Only completed ACTION receipts settle synchronous action success. QUERY/no_op
  do not prove business success; async acceptance binds an actual operation.
  Goal standards require their own versioned evaluation, not action success.
- Queued intentions pause/cancel with their Goal; standard changes pause them
  for reassessment. Resume checks expiration. Started operations retain facts.
- Callback reconciliation reads actual activity/media/visual identity state and
  requires ready owned assets for media success. Unknown operations are not
  resubmitted. Six unresolved checks halt automatic checking and hold execution
  as needs_reconciliation; real late callbacks may still record results.
- product.goal_retry config accepts max_attempts 1..20, base_seconds 30..3600,
  max_seconds >=base and <=86400. Defaults are 5/60/3600 with stable <=25% jitter.
  Explicit denial/refusal does not automatically retry. Suppression does not
  count as a failed effort. next_attempt_at is also the durable workflow due time.
- Standard IDs persist across reorder/copy edits; versioned Goal snapshots retain
  mappings. Mixed standards revision/evaluation is rejected. Standard edits reset
  old progress and require reevaluation; progress never gains fixed action points.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Inactive Goal/stale due/foreign scope | Suppress/reject before new Tool effect |
| Future time trigger | intention_trigger_not_due |
| Hard deadline | Expire unstarted intention; no permanent due/retry |
| Backoff not elapsed | intention_retry_not_due; preserve queued evidence |
| Standard revision plus outcome evaluation | goal_standard_revision_and_evaluation_mixed |
| Stale Goal/criteria evaluation version | goal_evaluation_version_conflict |
| Resume while operation unresolved | intention_operation_reconciliation_required |

### 5. Good/Base/Bad Cases
Good: pre-Tool Provider fails; Attempt fails, bounded retry survives App restart.
Base: original async operation is unresolved; report/check its identity then hold.
Bad: future Tool bypasses timer, re-send after unknown timeout, or increase Goal
progress on repeated expressions without evidence of another standard.

### 6. Tests Required
Goal execution correctness suite covers pre-Tool failure/restart/lease expiry,
pause/cancel/provider race, future direct start, hard deadline, bounded unknown
operation, stable criterion identities and stale-version/repeated-points rejection.
PostgresGoalExecution tests cover upgrade/history/rerun/ledger rollback. Existing
formal due/start/failure/late-result tests preserve held logical lifecycle and
independent physical attempt facts. Scripted Provider is not live acceptance.

### 7. Wrong vs Correct
Wrong: any Tool invocation -> pending; failed inbox alone recovers a due intention.
Correct: durable claim -> classified actual receipt/operation -> atomic Attempt
settlement or explicit waiting -> domain backoff/reconciliation -> durable work.

## Scenario: Goal Evaluation Source Admission Under Accumulated Tool History

Goal evaluation retains the complete durable source/CAS snapshot and offers a
bounded Provider view. Repeated Goal inspection snapshots must not be embedded
as proof; selected Goal standards are already provided separately. Actor facts
retain subject, value, epistemic kind, status and effective interval, supporting
both database (`subject_actor_id/value_json/valid_from`) and Tool receipt
(`actor_id/value/effective_at`) shapes. Domain query results and published
message text retain their full semantic payload.

Existing judgment evidence is mandatory. Admit recent messages before other
sources and sort equal-priority sources by recorded time then ID descending.
Keep the Provider source view below the existing required-input ceiling; the
current implementation reserves headroom with a 12000 estimated-token budget.
Oversized standards/mandatory proof fail visibly; never truncate proof or raise
global limits to hide overflow. The normal physical prompt gate still applies.

Only offered references may be accepted or marked processed. Unoffered sources
remain durable and unprocessed; queue a source-remainder evaluation for selected
active/paused Goals. Preserve existing deferred-Goal batching and claim CAS.

Required regression: accumulated legal inspection receipts and genuine domain
queries must still admit a recent preference/recommendation, close once without
fabricated Attempts, preserve a specific unoffered source and consume it in the
next batch. Actor Tool projection tests use the actual output field names and
unique subject/value/time assertions, rather than database-shaped mock receipts.

## Scenario: Durable Automatic Assessment Memo (2026-10-08)

### 1. Scope / Trigger

An automatic Goal request is queued despite unchanged already-assessed evidence.

### 2. Signatures

`goalAssessmentMemo` persists in successful goal_evaluation_requests.result;
`goalAssessmentEligible` removes only covered targets; owner_reassess retains
forced_goal_ids through pending coalescing. No schema migration/local cache.

### 3. Contracts

- Memo only after complete successful domain transaction; use post-commit Goal,
  Stage/Commitment authority. Never memo failed/truncated/partial results, apply
  old output again, or fabricate Evaluation/Attempt/Resolution on a cache hit.
- Signature includes actual standards/optional policy, target/profile, deadlines,
  admitted source identity/version/validity, slow relevant persona/permission/
  Life/schedule/relationship/Actor constraints and effective working profile.
  Exclude own Goal revision/judgment/Plan IDs, clock instant/as_of/current_time,
  and mood/drive decay. Coarse deadline/window phases still invalidate on real
  boundary crossing. A legitimate Foundation revision0 remains valid by CAS.
- Query persisted stamps by instance/Goal, then match effective profile and
  constraints; a shared request's empty profile and the same effective profile
  must not cause repeat calls. Source subsets may reuse a prior stamp, but new
  proof, withdrawal, standards/constraints changes, due review and Owner force
  remain eligible. goal_revision is not a meaningful review source watermark.
- Parenthesize JSONB force-array extraction during coalescing; preserve Owner
  reason/targets when an automatic request already exists. Never lose force.
- Both model-backed and skipped settlement revalidate claim, Goal/source and
  Foundation/effective-Life stamps under lifecycle locks. Preserve finite
  omitted-source and six-Goal remainder. Mandatory historical proof filling the
  entire budget without admitting a new pending source fails visibly/boundedly;
  do not create an infinite skip/remainder loop or consume unseen proof.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Same successful proof after restart | assessment_memo_match; zero model calls |
| Explicit Owner force merged into pending | fresh model assessment |
| New/revoked source or changed criteria/window | memo miss |
| Failed assessment | no stamp; bounded real retry |
| Authority changes before settlement | conflict; no stale skip/commit |
| No new proof fits beside mandatory proof | goal_evaluation_new_source_budget_blocked |

### 5. Good / Base / Bad Cases

Good: completed sibling disappears, unchanged remaining Goal uses its persisted
stamp. Base: ordinary seconds pass with no fact/window change, no new model call.
Bad: own evaluation increments revision and becomes another assessment reason.

### 6. Tests Required

Real PG repeat/restart, merged Owner force, new proof, criteria revision, failed
retry, goal_revision review-watermark stability and Stage replay model-call count.
Unit temporal boundary/clock/mood/no-progress/source-subset tests. Retain all
source revocation, real query completion, evidence batching and CAS regressions.

### 7. Wrong vs Correct

Wrong: hash the full Projection including instant or mark an unsuccessful run as
assessed. Correct: persist validated semantic-success stamps and recheck current
authority before a model-free settlement.


## Scenario: Completion Impact Consistency (2026-10-08)

### 1. Scope / Trigger
A model declares success in prose or satisfies every mandatory criterion while
returning `impact=progressed`. Do not persist or memoize an active Goal at 100%.

### 2. Signatures
`ApplyGoalEvaluation(goal,candidate,sources,at)` validates actual sources and
all/any/optional policy before checking `candidate.Impact`.

### 3. Contracts
Completion requires validated completion criteria and `impact=completed`.
Both directions are checked. No prose keyword parsing or automatic field repair.
Paused Goals retain lifecycle state and existing ready-for-settlement handling;
relationship confirmation and CAS checks remain mandatory. Core policy is
`goal.evaluation.v2`; memo `evaluation_policy_version` must match current policy
(older or missing policy misses cache). This version stays out of model prompts.
A failed candidate
uses bounded retry and cannot produce a successful assessment memo.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| complete criteria + non-completed impact | goal_evaluation_completion_impact_mismatch |
| incomplete criteria + completed impact | goal_completion_criteria_incomplete |
| duplicate criterion_ref | goal_evaluation_wire_criterion_ref_invalid |
| missing any claimed Goal evaluation, even with a plan | goal_assessment_coverage_missing |

### 5. Good / Base / Bad Cases
Good: valid sci-fi recommendation proof satisfies the supplied standard and
returns completed. Base: incomplete standards return progressed/needs_evidence.
Bad: invent a book text-percentage threshold or declare completion only in prose.

### 6. Tests Required
Unit all/any/optional-policy consistency and paused lifecycle; real PG malformed
output, duplicate judgments and partial coverage preserve Goal revision, source,
Evaluation and Resolution counts with request retry. Keep relationship guards.

### 7. Wrong vs Correct
Wrong: cache all-satisfied progressed as a successful assessment.
Correct: reject the contradiction before any domain write; only commit valid
completed candidates through the existing sole writer.
