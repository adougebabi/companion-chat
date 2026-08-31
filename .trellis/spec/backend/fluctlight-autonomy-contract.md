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
- Allowed pre-authorized Actions: proactive/delayed message, Moment, media request, Schedule proposal, governed life Event/Context change, Memory/Relationship candidate, and follow-up Goal/Intention.
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
