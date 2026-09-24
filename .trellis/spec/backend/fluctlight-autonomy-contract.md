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
- Goal has no side effect. A qualified Intention is required for Intention-driven actions; independent
  Tools use explicit authorization, resources and stable business operations.
- Time triggers use Temporal durable timers; Event triggers use inbox facts; semantic triggers re-enter LLM assessment and are not keyword listeners.
- A ready current-local-day Schedule is one typed lifecycle fact. The Worker may
  enqueue a stable `life_world.daily_review` inbox fact containing the accepted
  Schedule, active Goals/Intentions, existing direct-conversation target when
  present, and the frozen Foundation profile. The model decides `no_op`,
  `proactive_message`, or `moment`; Go Core never derives an action from idle
  duration or Schedule text.
- Execution rechecks current permission, per-action budget, quiet hours, cooldown, concurrency, Context, Schedule, Relationship, state revisions, and expiration.
- Go Core freezes the accepted final decision. Retry reuses it and stable IDs rather than re-assessing implicitly.
- Background Agents pass `AuthorizationPolicy="autonomy"` explicitly. Each Tool
  checks actual action permissions, quiet hours, cooldown and budget in its own
  short transaction. `tool_policy_reservations` reserves budget once per run.
  Internal cognition continues when external actions are denied. Direct Owner
  commands remain governed by Owner authorization rather than Agent names.
- Tool preparation/Provider work occurs outside its transaction. Domain effect,
  receipt and outbox commit together. Later sibling/model/final-output failure
  preserves earlier committed effects; retries cannot rerun the entire Agent.
- Allowed pre-authorized Actions: any installed, preflighted Capability slot plus
  internal Memory/Relationship/Goal/Intention candidates. Product code does not
  restrict the semantic Action type; the CapabilityDefinition and Core hard
  safety/authorization boundary remain authoritative.
- Forbidden autonomous Actions: identity-anchor/safety/Owner permission change, Provider/infrastructure setting change, destructive other-Actor/Fluctlight data action, budget bypass, or external irreversible action without a future explicit authorization model.
- Owner may inspect/pause/resume/cancel pending Goal/Intention/workflow and set `autonomy_mode` plus per-action policy. Governance appends history and audit; it does not erase facts.
- Paused mode blocks new autonomous external Actions but allows time, Context, Schedule, affect/drive decay, inbox facts, and explicit Human interaction.
- Screen/mute blocks proactive visibility/delivery according to policy without punishing Relationship or stopping internal life.
- A proactive message requires an already-existing direct Conversation ID at
  freeze time. A model proposal without that factual target is rejected; code
  never creates a conversation merely to make a proactive message possible.
- Autonomous proactive delivery suppresses an exact duplicate assistant text
  already sent by the same Fluctlight in the same Conversation during the
  recent duplicate window (12 hours). Suppression completes and audits the
  Action with `delivery_status=duplicate_suppressed`; it does not alter normal
  user-triggered reply handling.
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
| Tool commits before later sibling/model/final settlement fails | Preserve committed receipt/effect, record failed run and partial facts; do not replay the whole Agent. |
| Owner pauses/cancels | Append lifecycle/audit transition, cancel cooperative workflow, preserve history. |
| Paused Fluctlight receives direct Human message | Process explicit interaction; do not create unrelated autonomous external Actions. |
| Action targets forbidden infrastructure/destructive capability | Hard reject regardless of LLM confidence. |
| Proactive proposal has no direct Conversation target | Reject before autonomy freeze; do not create a conversation or fallback message. |
| Daily review is retried/replayed | Reuse the stable local-date fact ID; do not create duplicate Moment or proactive Action. |
| Proactive text exactly matches a recent assistant message | Complete the autonomous Action with `duplicate_suppressed`; do not append another visible message. |

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
- Bad: report a committed Tool as rolled back, replay it after model failure,
  or bypass the actual action policy by selecting a different surface.

### 6. Tests Required

- Goal/Intention contract tests for every source/status, evidence, revisions, expiration, and typed trigger.
- Workflow tests for durable time/Event triggers, semantic reassessment, frozen retry, idempotent Action, and restart recovery.
- Policy tests for permission, budget, quiet hours, cooldown, concurrency, screen/mute, and forbidden Actions.
- Governance tests for inspect/pause/resume/cancel, cooperative workflow cancellation, immutable history, audit, and active/paused mode.
- Concurrency tests with direct interaction, lifecycle Event, and due Intention for one Fluctlight inbox.
- Assert recent exact proactive text is suppressed transactionally while
  different text and text outside the duplicate window still deliver.
- Anti-heuristic tests prove time/engagement facts are LLM inputs and code does not infer relationship/action meaning.
- Local atomicity tests fail receipt/outbox persistence and assert that Tool
  rolls back; later-sibling failures preserve previously committed receipts.

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

- Trigger: an active Fluctlight's quiet-period Redis hint expires, or a Worker
  repairs a completed `wake_up.current` intent after restart.
- Purpose: give the Fluctlight a bounded internal cycle even when no human or
  life-world event arrived, while keeping external autonomy governed.

### 2. Signatures

```text
WakeUpWorkflow(ctx, {fluctlight_id, cycle}) -> WakeUpResult
ProcessWakeUp(ctx, fluctlight_id, cycle) -> WakeUpResult
```

`WakeUpResult` contains `wake_up_id`, `cycle`, `status`, `action_type`,
`action_id`, `reflection_intent_id`, and `interval_seconds`. Wake-up is an
action assessment only; it does not run a second cognition-stage pass or mutate
Current State. The persisted `cognition_wakeups` row keeps the current
`internal_dynamics` snapshot for audit compatibility, the requested/actual
action type, and the bounded action result. Legacy stage columns remain empty
for new rows. The corresponding `internal.wake_up` cognition fact is assigned
the next per-Fluctlight sequence and is marked processed only after its row and
the `reflection.run` intent are committed.

### 3. Contracts

- Activation writes one stable `wake_up.current` intent with workflow ID
  `wake_up:<fluctlight_id>` and cycle `0`; each cycle ID is derived from the
  Fluctlight ID and cycle number, so retries and Worker restarts are idempotent.
- The default interval is 1800 seconds. Core clamps configured
  `product.wakeup.interval_seconds` to 300–86400 seconds and accepts
  `product.wakeup.enabled=false` as an explicit pause of the internal timer.
- The formal WakeUp Agent owns semantic decisions and native Tool selection.
  Each Tool executes through `ExecuteTool` and feeds its actual receipt into the
  next model decision. A legal final no-op completes without visible output;
  invalid final output, cancellation and runtime errors remain failures.
- `conversation.reply` and `moment.publish` commit through the existing
  publication service. Media Tools return actual accepted intents. The outer
  WakeUp handler consumes committed results and never schedules a second
  execution of those native calls.
- Proactive reply requires an explicit authorized direct conversation target.
  Exact recent duplicate suppression applies only under the autonomy policy.
  No synthetic user message is needed to execute the Agent or a Tool.
- A state Tool owns its explicit evidence/business constraints. There is no
  universal freeze stage or sidecar requirement before Tool execution.
- Every wake-up commits a processed `internal.wake_up` fact and one stable
  `reflection.run` intent. Reflection consumes it through the normal evidence
  window and watermark/CAS boundary; a wake-up does not write self-model or
  personality values directly.
- `WakeUpWorkflow` executes one cycle and completes. Core resets a Redis
  `fluctlight:wakeup:due:<fluctlight_id>` quiet-period hint after a completed
  user turn and after a successful wake-up; expiry advances the durable intent
  cycle and dispatches the stable workflow ID again. PostgreSQL/Temporal remain
  authoritative and Redis is only a debounce/recovery hint. Inactive or
  disabled Fluctlights do not schedule another quiet-period key.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/negative cycle or Fluctlight ID | Reject with `wake_up_*_required/invalid`; no fact or action |
| Wake-up assessment has neither an action decision nor any CapabilityInvocation, returns an unsupported action, or exceeds bounded field size | Reject; no synthetic cognition state or fallback action is persisted |
| Native Tools execute and a legal final DTO completes | Record actual committed outputs and finish lifecycle once |
| Provider failure or invalid final JSON | Record failed run and retain committed effects; same-run recovery cannot restart the model decision loop |
| Autonomy paused or capability is not installed/authorized | Persist the internal cycle as `blocked`/`deferred`; do not create an external Action |
| Capability arguments or Definition are malformed | Fail closed and persist the internal cycle without an external Action |
| Proactive action has no direct conversation projection | Core ensures the Owner/Fluctlight direct conversation before freezing the action; only a creation failure remains a bounded workflow error |
| Duplicate cycle retry | Return the existing `cognition_wakeups` row and stable action/reflection IDs; do not consume another sequence |
| Reflection has no valid candidate | Advance only its evidence watermark; do not manufacture self-model or personality changes |

### 5. Good/Base/Bad Cases

- Good: a durable wake-up records what the Fluctlight is attending to, freezes
  one policy-approved action, and feeds the same fact into reflection so later
  evidence can change its self-model.
- Base: the model returns a rich internal cycle with `no_op`; the private fact
  and reflection intent are retained even though no visible message is sent.
- Bad: scan every Fluctlight from a process ticker, infer loneliness from idle
  time, store raw chain-of-thought, publish outside the authorized Tool service, or turn a missing capability into a fake
  successful result.

### 6. Tests Required

- Assert activation creates one `wake_up.current` intent and the dispatcher
  maps it to the lifecycle queue; assert stable cycle IDs across retries.
- Assert interval defaults, lower/upper clamps, disabled behavior, and
  Continue-As-New cycle increment.
- Assert missing action decisions/invalid actions/provider failures never create
  a wake fact or an external Action; assert `moment.publish` is registered and
  binds text to the durable Moment target.
- Assert a valid wake-up writes one sequenced `internal.wake_up` fact, one
  `cognition_wakeups` row, and one reflection intent; duplicate execution does
  not allocate another sequence.

## Scenario: Redis expiration hints for delayed reflection and wake-up

### 1. Scope / Trigger

- Trigger: a user turn needs a quiet reflection delay, or a long-lived wake-up
  cycle needs a low-latency dispatcher nudge after a Redis TTL expires.
- PostgreSQL/Temporal remain the durable authority. Redis keyevent Pub/Sub is
  an optimization and is allowed to lose notifications.

### 2. Signatures

- Reflection hint key: `fluctlight:reflection:due:<intent_id>` with TTL from
  the existing `product.wakeup.interval_seconds` setting until a dedicated
  reflection setting is introduced.
- Wake-up hint key: `fluctlight:wakeup:due:<fluctlight_id>` with the same
  clamped cadence.
- `RedisTriggerListener.Run(ctx)` subscribes to
  `__keyevent@*__:expired` and `HandleRedisExpiredTrigger(ctx, key)` advances
  only the matching PostgreSQL due state.

### 3. Contracts

- A completed user turn creates one delayed `reflection.run` intent; its
  action-result fact is included in that same evidence window rather than
  creating a second reflection call for the turn.
- Expiration handling advances `next_attempt_at` and the wake-up cycle for a
  matching completed wake-up intent. It never directly performs a provider call
  or starts a second workflow.
- Wake-up executes one Temporal cycle at a time; the Redis wake key is reset
  after each completed user turn and successful wake-up, and is repaired for
  completed intents at Worker startup. PostgreSQL/Temporal remain the durable
  authority.
- `notify-keyspace-events Ex` is required in the Redis config. Listener
  reconnect and periodic PostgreSQL due scans cover dropped Pub/Sub messages.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Duplicate expiration event | PG status predicate/stable workflow ID makes it a no-op. |
| Listener disconnect or Redis restart | Startup repair and periodic durable dispatcher scan still run due work. |
| Fluctlight paused/inactive/disabled | No new wake-up cycle is created or key renewed. |
| Multiple reflection source facts | Preserve evidence window/watermark/CAS; do not merge by Fluctlight ID alone. |
| Redis unavailable | User turn/workflow remains on PG/Temporal path; no business failure. |

### 5. Good / Base / Bad Cases

- Good: a key expires, the listener nudges a pending intent, and the existing
  dispatcher starts one stable workflow.
- Base: the listener misses the event; the intent's PG `next_attempt_at` and
  Worker scan start it later.
- Bad: perform reflection directly inside the Pub/Sub callback, use key payload
  as the only business data, or replace the Temporal wake-up timer without a
  durable recovery path.

### 6. Tests Required

- Assert key names/TTL calculation, one delayed reflection intent per user
  turn, duplicate-event idempotency, listener reconnect, startup repair, and
  pause/cancel/disabled behavior.
- Run Redis/Compose expiration tests where local listener ports are available;
  retain PG/Temporal fallback tests independently.

### 7. Wrong vs Correct

#### Wrong

```go
case key := <-expired:
    app.ProcessReflection(context.Background(), key)
```

#### Correct

```go
case key := <-expired:
    app.HandleRedisExpiredTrigger(ctx, key) // durable PG nudge only
```
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
                    evidence_refs, idempotency_key}) -> CapabilityResult
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
  preflighted direct `Capability` with a valid `CapabilityDefinition` may be
  marked fulfilled.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Slot key/schema/value/evidence invalid | Reject candidate; no slot or revision |
| Duplicate slot revision idempotency | Replay existing revision; no second state change |
| Missing capability request fields or malformed contract | Reject tool call; no request row |
| Same Fluctlight/tool idempotency replay | Return existing request; no duplicate request or outbox event |
| Fulfilled request without a registered matching direct Capability | Reject review; keep status unchanged |

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
- Capability request tests cover Definition catalog exposure, source ownership, contract
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

## Scenario: Goal/Intention Trigger And Outcome Closure

### 1. Scope / Trigger

- Trigger: a V2 Intention becomes `qualified`, a typed trigger becomes due, or
  a primary ActionOutcome settles an Intention attempt.

### 2. Signatures

```text
persistIntentionAuthorityTx(... qualified ...) -> intention.trigger intent
IntentionTriggerWorkflow(Input{intention_id,due_at})
ProcessIntentionTrigger(ctx,intention_id) -> pending|due|expired
persistActionOutcomesTx(... primary outcome ...) -> IntentionAttempt settlement
```

### 3. Contracts

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

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Free-form trigger/code/keyword expression | Reject; only time/event/semantic typed shapes are valid. |
| Same trigger revision is replayed | Return the same due fact, inbox and attempt IDs. |
| Intention expires before trigger apply | Append `expired`; create no due fact/action. |
| Due decision omits Goal/Intention influence refs | Fail before freeze; do not execute a Capability. |
| Failed/cancelled/suppressed Outcome | Requalify or cancel according to mechanical policy; never advance Goal progress. |
| Same primary Outcome is replayed | Reuse the stored attempt settlement; no second revision. |

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
