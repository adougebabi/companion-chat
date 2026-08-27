# Fluctlight Life World Contract

## Scenario: LLM-Planned Local Day With Explicit Context Authority

### 1. Scope / Trigger

- Trigger: a Fluctlight instance is created, a local day approaches, Schedule is generated/replanned, an Event changes life state, Context is read, or timezone changes.
- Schedule is a domain life plan, not cron. Temporal provides durable timing only.
- Semantic planning/replanning is LLM-owned through the `reflection` role; Python owns validation, versions, authority, time math, transactions, and workflow.

### 2. Signatures

```python
propose_schedule(command: SchedulePlanningInput) -> ScheduleProposal
accept_schedule(command: AcceptSchedule, tx: UnitOfWork) -> ScheduleVersion
propose_replan(command: ReplanInput) -> ScheduleProposal
resolve_context(query: ResolveContextAt) -> ContextSnapshot
change_timezone(command: ChangeTimezone, tx: UnitOfWork) -> TimezoneChange
register_schedule_lifecycle(fluctlight_id: str, tx: UnitOfWork) -> CommittedWorkflowIntent
ensure_current_day_schedule(payload: {fluctlight_id, intent_id}) -> {timezone, status}
```

Schedule version includes local date/timezone, generated-at/from, immutable items, reschedule policy, previous version, trigger/evidence/reason, model/prompt/policy versions, diff, status, and revision.

### 3. Contracts

- One local date has immutable versions and exactly one active accepted version.
- Accepted items cover the full local day. Gaps are explicit free-time/rest/sleep items, not implicit code defaults.
- Description creation may persist only model-owned Foundation, Goal and Intention
  semantics. The same creation transaction also commits exactly one stable
  `schedule.current_day` intent for the Fluctlight; it never calls an LLM or
  starts Temporal directly.
- The sole Worker intent dispatcher starts one long-lived workflow with
  `intent_id=schedule_intent:{fluctlight_id}`, `workflow_id=schedule:{fluctlight_id}`
  and queue `lifecycle`. Replays commit or start the same identity, not another
  per-day workflow.
- Its activity reloads the current Fluctlight, canonicalizes its IANA timezone,
  and generates only a missing Schedule for the current local date. It must not
  derive, persist, or request a missed historical date.
- If that activity leaves the current date missing because the Provider is
  unavailable or invalid, it reports `pending`, sleeps a bounded durable retry
  interval, and continues as new. It does not misclassify failure as an
  accepted Schedule or wait until the following day.
- The workflow uses deterministic `workflow.now()`, an aware local-midnight
  duration, `workflow.sleep()`, and `continue_as_new(payload)`. Each cycle
  reloads the timezone through the activity before scheduling the next boundary.
- Replan preserves completed history and replaces only current/future segments through a new version.
- User commitments are explicit high-authority facts with provenance; affect/drives/goals/interaction may cause the LLM to propose replan but code thresholds cannot invent semantic schedule changes.
- Context authority: confirmed active Event > accepted active Schedule item > explicit `unplanned/schedule_pending`.
- Conversation Presence may overlay user presence/current task but cannot fabricate scene, activity, location, or Event.
- Identity/occupation/weekday/clock are prompt inputs, not code rules for “working,” “studying,” “sleeping,” or other semantic state.
- Provider outage retries. Existing accepted Schedule remains through its day; missing plan yields `schedule_pending` and no fabricated past activity.
- Timezone change preserves historical versions, supersedes future versions, and regenerates future plans/timers in the new timezone.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Proposal has gaps/overlaps/out-of-day times/invalid timezone | Reject proposal; retain prior accepted version or pending state. |
| Proposal references unauthorized Actor/Event/Goal | Reject before acceptance. |
| Replan attempts to rewrite completed history | Reject; future-only replacement required. |
| Accepted version/revision changed during planning | CAS fails; re-read and replan explicitly. |
| Reflection Provider unavailable/invalid | Retry workflow; no code-generated default routine. |
| New day has no accepted Schedule | Return `schedule_pending/unplanned`; deterministic commitments only. |
| Event and Schedule overlap | Confirmed active Event is Context authority; Schedule remains historical plan unless replan is accepted. |
| Timezone changes | Preserve past, supersede future, cancel/recreate future timers, generate new proposal. |
| DST creates ambiguous/nonexistent local time | Resolve through explicit timezone policy and store offsets; never compare naive local strings. |
| Activation replay, Worker restart, or duplicate dispatcher delivery | Reuse the stable schedule intent/workflow ID; Activity checks accepted current local day and is a no-op when present. |
| Worker or NAS was offline across a local-day boundary | Activity observes the date at execution and creates only that current local day; no historical Schedule is fabricated. |

### 5. Good / Base / Bad Cases

- Good: a confirmed interruption triggers an LLM replan; completed items remain, future items form one validated replacement version.
- Good: a chat overlays user presence while Context still reports the accepted physical scene/activity.
- Base: Provider outage at midnight yields `schedule_pending` plus fixed commitments and later recovers without inventing past activity.
- Good: a Fluctlight created at 23:59 commits its lifecycle intent with the
  Foundation. The activity runs after midnight and plans that current date,
  rather than writing a stale date captured at activation.
- Bad: `occupation == student && hour < 16` means studying, clone yesterday as fallback, leave gaps for a resolver to guess, or update schedule rows in place.

### 6. Tests Required

- Schedule proposal/acceptance tests for full-day coverage, overlap/gap, enum/reference validation, immutable versions, diff, and rollback.
- Replan tests for completed-history preservation, current/future replacement, CAS conflict, trigger/evidence/model provenance, and idempotency.
- Context tests for Event > Schedule > pending authority and Conversation Presence overlay.
- Time tests for multiple zones, DST gap/fold, local-date boundaries, timezone change, future timer replacement, and historical offsets.
- Provider failure tests assert retry/pending and no identity/occupation/clock/default-routine heuristic path.
- End-to-end lifecycle workflow test creates only the current local-day plan,
  survives restart and a local-day boundary, handles Event/replan, and resolves
  Context consistently for prompt/API/media.
- Lifecycle registration test asserts Fluctlight creation, inner-state baseline,
  and the stable `platform_workflow_intents` row share one transaction.
- Activity tests assert active/current-day-only behavior, `UTC+8` canonicalizes
  to `Asia/Shanghai`, inactive Fluctlights are not planned, and an existing
  current-day Schedule yields no semantic Provider call.
- Workflow sandbox/timer tests assert local-midnight timing (including DST),
  stable payload preservation through `continue_as_new`, and no wall-clock call
  inside the workflow body.

### 7. Wrong vs Correct

#### Wrong

```python
if identity.occupation == "student" and 8 <= now.hour < 16:
    return Context(activity="studying", scene="school")
return yesterday_schedule.clone_for(today)
```

#### Correct

```python
context = life_world.resolve_context(
    ResolveContextAt(fluctlight_id=fluctlight_id, instant=now)
)
# Result is a confirmed Event, accepted Schedule item, or explicit pending state.
```

#### Wrong

```python
await schedule_initializer.ensure_for(created)  # API request waits for the model
await temporal_client.start_workflow(...)       # bypasses committed-intent dispatch
```

#### Correct

```python
await schedule_lifecycle.register(created.id, tx=tx)
# Worker dispatcher owns Temporal start. The Activity later ensures only the
# local day in which it actually runs.
```
