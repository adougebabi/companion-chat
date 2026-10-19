# World Event Templates And Runtime

## 1. Scope / Trigger

Schema `0077_world_events`: a published event definition is explicitly applied
to one runtime world, enabled, and evaluated by the bounded Worker. Studio
drafts/publications, runtime policy, queued execution and occurred facts differ.

## 2. Signatures

- `validateWorldEventDefinition`: closed `world-event.v1` schema; condition,
  trigger interval/chance/cooldown, duration, scopes and typed effects.
- `RepairWorldEvents(ctx,limit)`: automatic evaluation, queued effects and
  pending physical recovery, each in a short transaction without external I/O.
- `/world/templates/{id}`; `/runs`, `/runs/{runId}`, `/runs/{runId}/retry`.
- `/world/events`, `/world/events/{eventId}`: actual Life Event history and
  recorded direct references. Unattributed history never inherits current world.

## 3. Contracts

Saving/publishing does not apply or trigger. First application starts disabled.
An upgrade preserves explicit enabled policy but increments the template revision;
queued work for old revisions is filtered. CAS, immutable publication and durable
Owner command replay remain in the existing world domain boundary.

Configured virtual-world effects are explicit structured simulation, not a
heuristic interpretation of user conversation. Automatic checks store each
candidate outcome, advance from the current execution time without historical
backfill, and apply chance/cooldown/conditions. Manual trigger bypasses chance
only, retaining all other guards. Failed effects roll back together, retain a
failed execution and require explicit retry within the original window. Recovery
failures keep the original physical obligation with persisted backoff.

Ambient events use trusted internal context, never a browser field. They retain
real Life Event evidence, Native facts, body effects and outbox but do not become
scene/location authority. Current scoped world effects participate in the Life
context revision and model/schedule consumers. An identified current-scene
activity inherits its stable place when omitted; changing only location/scene
text cannot discard that identity to evade temporary closure admission.

NPC visits create actual bounded presence and normal opportunities, not a
relationship. Every current reader and final encounter validation checks
`world_npc_effective_presence_sources`, which excludes a cancelled visitor event
immediately. The interrupted routine remains cancelled history. Resumption starts
a **new actual segment now**, with a durable source identity distinct from the
old interval segment; never return a cancelled routine record as current.

Permanent harm is unavailable. Only recoverable minor injury is admitted with
confirmed character presence. Its committed recovery runs after duration even
if the template is disabled, world switched or character retired. It uses the
existing body recovery writer and does not erase the original event/history.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Publication/application/GET | No execution, movement or acquaintance |
| Unpublished/foreign/stale reference | Explicit rejection, no partial apply |
| Unknown/contradictory effect, permanent harm | Reject definition/execution |
| Old/disabled template or inactive world | Durable filtered result |
| Condition/cooldown/chance not met | Durable reason, no fake success |
| Unknown prior command response | Replay immutable original payload/key |
| First explicit CAS rejection | User may deliberately adopt latest baseline |
| Logout or newer command at same browser key | Late result cannot restore/delete its record |
| Cancelled/expired visit | No current visitor or valid encounter; routine resumes now |

## 5. Good / Base / Bad Cases

Good: virtual rain produces an ambient sourced fact; the character may decide
to replan through existing Tools. Base: a recorded candidate is filtered with a
real reason. Bad: treating template application as an event, replacing the
character's place for weather, or claiming a diary/relationship without a result.

## 6. Tests Required

`world_event_runtime_test.go`, `world_events_test.go` and browser transport tests
cover actual execution, replay, source cancellation, fresh routine segments,
temporary closure, minor injury recovery, probability and schema upgrade. These
tests are authored and compile-only checked; business execution is deferred to
the final batch. Real model consumer steps belong to `docs/v3-llm-acceptance.md`
L14 and remain unexecuted until the user runs them.

## 7. Wrong vs Correct

Wrong: reuse a cancelled routine interval after a visit, release a command key
after an ambiguous earlier attempt, or let a late rejection recreate logout
data. Correct: create a fresh executed segment, preserve the original unknown
identity, and condition browser settlement writes on the matching persisted
identity and attempt.
