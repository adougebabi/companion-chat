# Fluctlight Autonomy Contract

## Scenario: Pre-Authorized Goals, Intentions, And Audited Actions

### 1. Scope / Trigger

- Trigger: a Drive/Event/Human request/Reflection proposes a Goal, a Goal produces an Intention, a trigger becomes due, or an autonomous Action is executed/governed.
- LLM owns semantic goal/intention/action proposals. Go Core owns lifecycle, permissions, budgets, timing facts, workflow, freeze, execution authorization, and audit.
- Owner grants policy in advance; normal allowed Actions do not require per-action confirmation.

### 2. Signatures

```python
propose_goal(command: GoalEvidence) -> GoalCandidate
accept_goal(command: AcceptGoal, tx: UnitOfWork) -> Goal
propose_intention(command: IntentionEvidence) -> IntentionCandidate
qualify_intention(command: QualifyIntention, tx: UnitOfWork) -> Intention
freeze_action(command: FreezeAutonomousAction, tx: UnitOfWork) -> FrozenAction
govern(command: PauseResumeCancel, tx: UnitOfWork) -> GovernanceResult
```

Goal status: `candidate | active | paused | completed | abandoned | cancelled`.

Intention includes Goal/one-shot Event reference, action, preferred time, typed trigger, confidence, expiration, evidence, permission/budget snapshot, status, and revision.

### 3. Contracts

- Goal source is `drive | event | human | self`. A Human request is evidence, not automatic forced execution.
- Goal has no side effect. A concrete qualified Intention and frozen final decision are required before Action.
- Time triggers use Temporal durable timers; Event triggers use inbox facts; semantic triggers re-enter LLM assessment and are not keyword listeners.
- A ready current-local-day Schedule is one typed lifecycle fact. The Worker may
  enqueue a stable `life_world.daily_review` inbox fact containing the accepted
  Schedule, active Goals/Intentions, existing direct-conversation target when
  present, and the frozen Foundation profile. The model decides `no_op`,
  `proactive_message`, or `moment`; Go Core never derives an action from idle
  duration or Schedule text.
- Execution rechecks current permission, per-action budget, quiet hours, cooldown, concurrency, Context, Schedule, Relationship, state revisions, and expiration.
- Go Core freezes the accepted final decision. Retry reuses it and stable IDs rather than re-assessing implicitly.
- Allowed pre-authorized Actions: any installed, preflighted Capability slot plus
  internal Memory/Relationship/Goal/Intention candidates. Product code does not
  restrict the semantic Action type; the capability manifest and Core hard
  safety/authorization boundary remain authoritative.
- Forbidden autonomous Actions: identity-anchor/safety/Owner permission change, Provider/infrastructure setting change, destructive other-Actor/Fluctlight data action, budget bypass, or external irreversible action without a future explicit authorization model.
- Owner may inspect/pause/resume/cancel pending Goal/Intention/workflow and set `autonomy_mode` plus per-action policy. Governance appends history and audit; it does not erase facts.
- Paused mode blocks new autonomous external Actions but allows time, Context, Schedule, affect/drive decay, inbox facts, and explicit Human interaction.
- Screen/mute blocks proactive visibility/delivery according to policy without punishing Relationship or stopping internal life.
- A proactive message requires an already-existing direct Conversation ID at
  freeze time. A model proposal without that factual target is rejected; code
  never creates a conversation merely to make a proactive message possible.
- A Moment is shared-feed content by default (`participants`). The current
  product exposes the Owner's authorized instances only; future Fluctlight
  cross-feed/group readers extend consumption without changing Moment history.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Goal/Intention lacks evidence, owner, source, or bounded fields | Reject candidate; no fallback Goal. |
| Trigger condition is free-form code/keyword matcher | Reject; use typed time/Event or semantic reassessment. |
| Permission disabled, budget exhausted, quiet hours/cooldown active | Deny/defer by explicit policy and record reason; do not reinterpret semantics. |
| State/Schedule/Relationship revision changed before freeze | Requalify/re-assess explicitly; do not execute stale action. |
| Frozen Action retry | Reuse same decision/workflow/Provider IDs; no duplicate action. |
| Owner pauses/cancels | Append lifecycle/audit transition, cancel cooperative workflow, preserve history. |
| Paused Fluctlight receives direct Human message | Process explicit interaction; do not create unrelated autonomous external Actions. |
| Action targets forbidden infrastructure/destructive capability | Hard reject regardless of LLM confidence. |
| Proactive proposal has no direct Conversation target | Reject before autonomy freeze; do not create a conversation or fallback message. |
| Daily review is retried/replayed | Reuse the stable local-date fact ID; do not create duplicate Moment or proactive Action. |

When a daily-review activity returns `schedule_pending`, the workflow waits a
bounded interval and continues as new with an empty `local_date`; the next
activity recalculates the Fluctlight's current local date and rechecks the
accepted Schedule before asking the model for an action. Carrying the original
activation date can leave a permanently pending review after a local-day
boundary.

### 5. Good / Base / Bad Cases

- Good: intimacy Drive and relationship evidence produce a Goal, a timed Intention, a fresh assessment, one frozen proactive message, and an audited delivery under budget.
- Good: Owner pauses autonomy; Schedule and affect decay continue while pending external intentions remain paused.
- Base: a due Intention is denied by quiet hours and explicitly deferred without changing its semantic meaning.
- Bad: “no message for 10 minutes” directly sends a message, regex creates a Goal, cancellation deletes history, or LLM changes Provider settings.

### 6. Tests Required

- Goal/Intention contract tests for every source/status, evidence, revisions, expiration, and typed trigger.
- Workflow tests for durable time/Event triggers, semantic reassessment, frozen retry, idempotent Action, and restart recovery.
- Policy tests for permission, budget, quiet hours, cooldown, concurrency, screen/mute, and forbidden Actions.
- Governance tests for inspect/pause/resume/cancel, cooperative workflow cancellation, immutable history, audit, and active/paused mode.
- Concurrency tests with direct interaction, lifecycle Event, and due Intention for one Fluctlight inbox.
- Anti-heuristic tests prove time/engagement facts are LLM inputs and code does not infer relationship/action meaning.

### T04 Ownership And Governance Persistence

Goal and Intention rows are owned by a Fluctlight. An Intention may reference a
Goal only when `(fluctlight_id, goal_id)` matches the Goal owner. Qualification,
pause/resume, completion, expiration, and cancellation append immutable
governance rows containing actor, old status, new status, reason, and revision;
they never delete or rewrite prior lifecycle history. A paused Intention cannot
resume after its expiration and must transition through `expired` instead.

### 7. Wrong vs Correct

#### Wrong

```python
if now - last_human_message > timedelta(minutes=10):
    send_proactive_message("I miss you")
```

#### Correct

```python
inbox.enqueue(IntentionDueFact(intention_id=intention.id))
decision = cognition.process_next(fluctlight_id)
autonomy_policy.freeze_and_schedule(decision, current_policy, tx=tx)
```

## Scenario: Periodic Wake-Up And Internal-Life Loop

### 1. Scope / Trigger

- Trigger: an active Fluctlight reaches the next durable wake-up boundary, or a
  Worker resumes its long-lived `wake_up.current` workflow after restart.
- Purpose: give the Fluctlight a bounded internal cycle even when no human or
  life-world event arrived, while keeping external autonomy governed.

### 2. Signatures

```text
WakeUpWorkflow(ctx, {fluctlight_id, cycle}) -> ContinueAsNew(cycle + 1)
ProcessWakeUp(ctx, fluctlight_id, cycle) -> WakeUpResult
```

`WakeUpResult` contains `wake_up_id`, `cycle`, `status`, `action_type`,
`action_id`, `reflection_intent_id`, and `interval_seconds`. The persisted
`cognition_wakeups` row records `attention`, `thought`, `desire`, `agency`, an
`internal_dynamics` snapshot, the requested/actual action type, and the bounded
action result. The corresponding `internal.wake_up` cognition fact is assigned
the next per-Fluctlight sequence and is marked processed only after its row and
the `reflection.run` intent are committed.

### 3. Contracts

- Activation writes one stable `wake_up.current` intent with workflow ID
  `wake_up:<fluctlight_id>` and cycle `0`; each cycle ID is derived from the
  Fluctlight ID and cycle number, so retries and Worker restarts are idempotent.
- The default interval is 1800 seconds. Core clamps configured
  `product.wakeup.interval_seconds` to 300–86400 seconds and accepts
  `product.wakeup.enabled=false` as an explicit pause of the internal timer.
- The model owns the semantic stage summaries (`attention`, `thought`,
  `desire`, `agency`) and may propose `no_op`, a legacy visible action, or any
  installed Capability slot through a tool call. Core stores concise summaries,
  never hidden chain-of-thought.
- A proposed external action is frozen only after its Capability manifest,
  arguments, source fact, Owner authorization, hard safety, resource and
  idempotency checks pass. There is no product-type allowlist; visible text is
  produced once by `action_realization` where needed and delivery remains
  workflow-owned.
- Every wake-up commits a processed `internal.wake_up` fact and one stable
  `reflection.run` intent. Reflection consumes it through the normal evidence
  window and watermark/CAS boundary; a wake-up does not write self-model or
  personality values directly.
- `WakeUpWorkflow` uses Temporal `Sleep` plus `ContinueAsNew`, never an
  in-memory ticker or a second delayed-job system. Inactive Fluctlights end the
  workflow; disabled wake-ups remain durable and sleep at the clamped cadence.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/negative cycle or Fluctlight ID | Reject with `wake_up_*_required/invalid`; no fact or action |
| Wake-up assessment omits a stage, returns an unsupported action, or exceeds bounded summary size | Reject; no synthetic thought or fallback action is persisted |
| Provider failure or invalid JSON | Workflow retries; after exhaustion the source intent remains auditable and no fabricated stage is written |
| Autonomy paused or capability is not installed/authorized | Persist the internal cycle as `blocked`/`deferred`; do not create an external Action |
| Capability arguments or manifest are malformed | Fail closed and persist the internal cycle without an external Action |
| Proactive action has no direct conversation | Persist the internal cycle with `proactive_target_invalid`; do not create a conversation |
| Duplicate cycle retry | Return the existing `cognition_wakeups` row and stable action/reflection IDs; do not consume another sequence |
| Reflection has no valid candidate | Advance only its evidence watermark; do not manufacture self-model or personality changes |

### 5. Good/Base/Bad Cases

- Good: a durable wake-up records what the Fluctlight is attending to, freezes
  one policy-approved action, and feeds the same fact into reflection so later
  evidence can change its self-model.
- Base: the model returns a rich internal cycle with `no_op`; the private fact
  and reflection intent are retained even though no visible message is sent.
- Bad: scan every Fluctlight from a process ticker, infer loneliness from idle
  time, store raw chain-of-thought, send a message directly from the wake-up
  activity without a frozen Action, or turn a missing capability into a fake
  successful result.

### 6. Tests Required

- Assert activation creates one `wake_up.current` intent and the dispatcher
  maps it to the lifecycle queue; assert stable cycle IDs across retries.
- Assert interval defaults, lower/upper clamps, disabled behavior, and
  Continue-As-New cycle increment.
- Assert missing stages/invalid actions/provider failures never create a wake
  fact or an external Action.
- Assert a valid wake-up writes one sequenced `internal.wake_up` fact, one
  `cognition_wakeups` row, and one reflection intent; duplicate execution does
  not allocate another sequence.
- Assert autonomy mode/allowlist/direct-target gates and frozen action delivery
  update the wake result without duplicating messages or Moments.

## Scenario: Typed Drive/Preference Slots And Capability Requests

### 1. Scope / Trigger

- Trigger: Reflection proposes a new personality drive/preference, or Agency
  discovers that a desired capability is absent from the installed catalog.

### 2. Signatures

```text
DriveSlot {key, label, description, value_schema, value, confidence,
           evidence_refs, revision, status, decay_policy, update_policy}
PreferenceSlot {key, label, description, value_schema, value, confidence,
                evidence_refs, revision, status, update_policy}
capability.request({capability_key, title, description, rationale,
                    desired_contract, side_effect_class, priority,
                    evidence_refs, idempotency_key}) -> ToolResultV1
```

### 3. Contracts

- Slot keys are instance-scoped stable identifiers, but their semantic names
  are open-ended. Drive `pressure` values and Preference typed values are
  validated against their declared schema; slot revisions are immutable and
  supersede rather than delete prior values.
- Reflection applies slots only inside the evidence-window transaction with
  state-revision CAS. Active slots are included in the next ContextProjection,
  so Attention/Thought/Desire/Agency can change without code redeployment.
- `capability.request` is a native registry slot always visible to the model.
  It records a missing capability request and never executes an external
  provider. Requests are globally aggregated by `capability_key` while each
  Fluctlight's source fact and evidence remain separate.
- Owner review moves a request through `proposed`, `reviewing`, `accepted`,
  `rejected`, `fulfilled`, or `cancelled`. Only a manually registered and
  preflighted CapabilityExecutor may be marked fulfilled.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Slot key/schema/value/evidence invalid | Reject candidate; no slot or revision |
| Duplicate slot revision idempotency | Replay existing revision; no second state change |
| Missing capability request fields or malformed contract | Reject tool call; no request row |
| Same Fluctlight/tool idempotency replay | Return existing request; no duplicate request or outbox event |
| Fulfilled request without a registered matching CapabilityExecutor | Reject review; keep status unchanged |

### 5. Good/Base/Bad Cases

- Good: three Fluctlights request `calendar.read`; one aggregated need shows
  three source facts, and a manually preflighted plugin later fulfills it.
- Base: a new `quiet_evening` Preference slot is persisted with a categorical
  schema and is visible in the next wake-up projection.
- Bad: hard-code a fixed Drive enum, let the model write arbitrary JSON into
  inner state, auto-install a plugin, or hide a missing capability in a prose
  response.

### 6. Tests Required

- Slot tests cover arbitrary keys, typed schemas, bounds, supersede, CAS,
  evidence, idempotency and projection visibility.
- Capability request tests cover manifest exposure, source ownership, contract
  bounds, global aggregate counts, status transitions and fulfilled-version
  checks.

### 7. Wrong vs Correct

#### Wrong

```go
if missingCapability {
	return "我已经查过日历了"
}
```

#### Correct

```go
return capabilityRequestTool.Call(map[string]any{
	"capability_key": "calendar.read",
	"rationale": "需要安排下一步行动",
})
```

### 7. Wrong vs Correct

#### Wrong

```go
if time.Since(lastMessage) > tenMinutes {
	return sendMessage("我想你了")
}
```

#### Correct

```go
assessment := provider.Structured("cognitive_assessment", wakeContext)
fact, reflection := persistWakeUp(assessment, stableCycleID)
if policy.Allows(assessment.ActionType) {
	freezeAutonomyAction(assessment, fact, tx)
}
```
