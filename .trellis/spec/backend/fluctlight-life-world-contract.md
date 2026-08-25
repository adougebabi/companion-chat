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
```

Schedule version includes local date/timezone, generated-at/from, immutable items, reschedule policy, previous version, trigger/evidence/reason, model/prompt/policy versions, diff, status, and revision.

### 3. Contracts

- One local date has immutable versions and exactly one active accepted version.
- Accepted items cover the full local day. Gaps are explicit free-time/rest/sleep items, not implicit code defaults.
- New Fluctlight creation plans the remaining current local day and next local day.
- Lifecycle workflow plans the next day using `reflection` role and validated facts/state.
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

### 5. Good / Base / Bad Cases

- Good: a confirmed interruption triggers an LLM replan; completed items remain, future items form one validated replacement version.
- Good: a chat overlays user presence while Context still reports the accepted physical scene/activity.
- Base: Provider outage at midnight yields `schedule_pending` plus fixed commitments and later recovers without inventing past activity.
- Bad: `occupation == student && hour < 16` means studying, clone yesterday as fallback, leave gaps for a resolver to guess, or update schedule rows in place.

### 6. Tests Required

- Schedule proposal/acceptance tests for full-day coverage, overlap/gap, enum/reference validation, immutable versions, diff, and rollback.
- Replan tests for completed-history preservation, current/future replacement, CAS conflict, trigger/evidence/model provenance, and idempotency.
- Context tests for Event > Schedule > pending authority and Conversation Presence overlay.
- Time tests for multiple zones, DST gap/fold, local-date boundaries, timezone change, future timer replacement, and historical offsets.
- Provider failure tests assert retry/pending and no identity/occupation/clock/default-routine heuristic path.
- End-to-end lifecycle workflow test creates current/next plan, survives restart, handles Event/replan, and resolves Context consistently for prompt/API/media.

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
