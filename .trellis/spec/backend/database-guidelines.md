# SQLite Storage Guidelines

## Storage Model

Persistence uses one `better-sqlite3` database file in WAL mode. `companion_schema_migrations` is the authoritative version ledger. New companion resources are normalized `companion_*` tables: personas and immutable foundation revisions, life blueprints and state, events and activities, persona-private memories, conversations/messages, media references, and durable jobs.

`companion_settings` is the only low-frequency JSON settings row. Do not put independently queried or high-growth data in it.

## Read And Write Rules

- Query and update the table that owns the resource. Every persona-private read includes `persona_id` directly or through its owning conversation/activity.
- Use short SQLite transactions for a state transition that creates related rows, such as event + current state + activity, or message + conversation timestamp.
- Claim jobs with a conditional lease update inside a transaction. Run MTPLX or ComfyUI outside that transaction, then settle only with the matching lease owner.
- Published activities are immutable. Hide/restore writes `companion_activity_visibility`; it never changes the activity, event, evidence, or media record.
- Keep cursor ordering stable with `(created_at, id)`. Fetch one additional row to determine whether `nextCursor` exists.

## Migrations And Legacy Data

Add a new ordered migration version; never edit a migration that may already have been applied. Each migration and its ledger insert run in one startup transaction. A migration error must stop startup.

The companion domain starts clean. It does not read, import, migrate, or delete a legacy `state.json` or `app_state` row. Legacy files may remain beside the database but are not a companion data source.

## Common Mistakes

- Reading a whole legacy state blob after an external await and overwriting unrelated normalized rows.
- Treating nullable composite keys as an idempotency guarantee. User reactions use a partial unique index and an explicit delete/insert sequence.
- Returning a masked API key and then persisting the mask value. Expose `hasLmStudioApiKey`; treat an empty or sentinel key patch as unchanged.
- Committing `data/`; it contains private local companion data and is intentionally ignored.

## Scenario: Persona life-model timeline and deferred chat batches

### 1. Scope / Trigger

- Trigger: a persona needs a durable life model, daily time-line decisions, and sleep-time delayed replies without adding an external queue or a second database.

### 2. Signatures

- Migration v7 owns `companion_persona_life_blueprint_revisions`, `companion_timeline_slots`, `companion_event_decisions`, `companion_event_links`, and `companion_chat_deferred_batches`.
- `companion_event_decisions` is unique on `(persona_id, decision_key)`.
- `companion_chat_deferred_batches` is unique on `(persona_id, batch_key)` and owns `deliver_at`, `message_ids_json`, and `result_message_id`.

### 3. Contracts

- `companion_life_events` remains the immutable fact record; future slots and choices do not masquerade as facts.
- A queued sleep batch accepts later message IDs into the same JSON array but cannot be replaced by a new immediate-reply decision.
- A deferred-reply job rechecks its lease and batch status before creating its one assistant reply.
- Delete a persona's deferred batches, links, decisions, slots, and blueprint revisions before deleting their referenced conversations, events, jobs, schedules, or persona.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Duplicate `decision_key` / `batch_key` | Reuse the existing durable record; do not create another fact or reply. |
| Life-model candidate lacks a valid default room or safe four-template shape | Use the deterministic v2 fallback and record validation warnings. |
| Negative template is not mild, reversible, and recoverable | Reject the generated candidate and fall back. |
| Deferred batch no longer queued or its job lease is stale | Do not insert a reply. |

### 5. Good/Base/Bad Cases

- Good: a sleeping persona accumulates three messages in one batch and emits one ordinary reply at `deliver_at`.
- Base: no opportunity candidate creates a suppressed `no_event` decision without producing a dynamic.
- Bad: creating a new event on every tick, treating an event decision as an event fact, or allowing a second user message to wake an already deferred sleep batch.

### 6. Tests Required

- Assert v7 migration tables, life-model fallback, default room, and negative-event validation.
- Assert one decision per `decision_key`, no-event persistence, and event priority state projection.
- Assert one deferred batch collects multiple messages and cannot duplicate a reply after retry/restart.
- Assert persona deletion removes all v7 rows atomically.

### 7. Wrong vs Correct

#### Wrong

```js
createEvent(persona, generatedCandidate); // writes a fact before constraints/idempotency
```

#### Correct

```js
const decision = timelineDecision(persona.id, decisionKey, candidate);
if (decision.status === 'accepted') instantiateTimelineEvent(persona);
```

## Scenario: Permanent test-persona deletion

### 1. Scope / Trigger

- Trigger: a tester needs to discard a malformed persona and all of its private generated data without changing any other persona.

### 2. Signatures

- `DELETE /api/companion/personas/:personaId` returns `{id, deleted: true, deletedMediaIds}`.
- `deletePersona(personaId)` owns the SQLite deletion transaction; callers must not issue scattered table deletes from routes or browser code.

### 3. Contracts

- Resolve the persona first with `requirePersona()`; an unknown/already deleted persona returns the normal persona-not-found error and makes no mutation.
- Delete in dependency order: jobs; activity child rows/media links; activities; conversation messages/conversation; persona-private memory/evolution/supporting characters/schedules/state/blueprint/foundation revisions; life events; persona row.
- Media assets are deleted only when they are no longer referenced by another activity-media link **or** a serialized message attachment. The cleanup is permanent and is not a screen/archive feature.
- The client must obtain an explicit native confirmation, clear an active deleted selection, then reload bootstrap before selecting a remaining persona or rendering its empty state.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Persona ID exists | All selected-persona rows are deleted atomically and `{deleted: true}` is returned. |
| Persona ID is missing/already removed | Error response; no other rows are touched. |
| A candidate media asset is still referenced by another row | The persona's link is removed but the shared asset remains. |
| A statement in the cleanup transaction fails | SQLite rolls back the whole deletion; the persona remains reachable. |

### 5. Good/Base/Bad Cases

- Good: deleting a temporary persona removes its conversation, queued media job, activity, and foundation revisions while a second persona is unchanged.
- Base: a deleted active persona leaves the browser with the next available persona, or the normal no-persona view.
- Bad: setting `deleted_at` alone, deleting the parent before its foreign-key children, or deleting every media asset without checking cross-persona references.

### 6. Tests Required

- Create two personas; give the removable one a message, activity, and media job; delete it; assert all scoped rows are gone.
- Assert `requirePersona()` fails for the removed ID and an unrelated persona's messages/data remain intact.
- Assert an asset with another remaining activity or message attachment is not removed.

### 7. Wrong vs Correct

#### Wrong

```js
database.prepare('DELETE FROM companion_personas WHERE id = ?').run(personaId);
```

#### Correct

```js
database.transaction(() => {
  deletePersonaPrivateChildren(personaId);
  database.prepare('DELETE FROM companion_personas WHERE id = ?').run(personaId);
})();
```

## Scenario: Audited schedule changes and proactive delivery

### 1. Scope / Trigger

- Trigger: a plan is rescheduled or a life event may become an unsolicited direct message. Both paths cross SQLite, an HTTP/UI boundary, and the durable job worker.

### 2. Signatures

- `PATCH /api/companion/personas/:personaId/schedule/:scheduleId` accepts `{title?, startsAt, endsAt?, scene?}` and returns the updated schedule shape.
- `rescheduleScheduleItem(personaId, scheduleId, input)` updates one active future schedule and appends a `schedule_rescheduled` life event.
- `companion_jobs.job_type = 'proactive_message'` carries `{eventId, fallbackText}` and is settled by `completeProactiveMessageJob(job, text)`.

### 3. Contracts

- A reschedule retains the original schedule ID and writes `{source:'user', previous, next}` to the causative event payload. It never silently overwrites the historical state transition.
- Proactive output and relationship evolution are created only from a leased, unexpired job. Their completion transaction rechecks `status = 'leased'`, `lease_owner`, `lease_expires_at`, persona screen state, focus/recent engagement, rest hours, event relevance, daily blueprint budget, and existing `proactive_event_id` delivery.
- The ordinary persona-detail response returns only `foundationSummary`, concise evolution changes, and evidence summaries. Raw foundation text is available only from the explicit `GET /foundation/draft` editing surface.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Schedule is missing, owned by another persona, inactive, or already started | `404` or a validation error; no state/event write |
| Start is not future or end is invalid | `400 {error: ...}`; schedule remains unchanged |
| Proactive persona is screened, idle, resting, over budget, or event is irrelevant | Job completes with `result_json.skipped`; no message is inserted |
| Lease owner is stale, expired, or job already completed | Completion changes nothing and never duplicates a message/evolution |

### 5. Good/Base/Bad Cases

- Good: a future plan keeps its ID after a user moves it; a single `schedule_rescheduled` event preserves both snapshots.
- Base: routine/recovery remains deterministic for idle personas, while focused personas may create bounded event variations.
- Bad: inserting an unsolicited message while initially creating a life event, trusting a browser-provided schedule, or exposing raw foundation text in the normal detail response.

### 6. Tests Required

- Assert reschedule ownership/time validation, stable ID, and causative audit payload.
- Assert proactive focus/screen eligibility, stale lease rejection, one-message settlement, and no duplicate settlement.
- Assert persona-detail UI contracts never require raw prompt text, while the explicit revision flow can request a draft.

### 7. Wrong vs Correct

#### Wrong

```js
createEvent(persona, event, {proactive: true}); // inserts a message inside the event transaction
```

#### Correct

```js
createEvent(persona, event, {proactive: true}); // queues a durable proactive_message job
// The worker rechecks policy and writes the message only under its active lease.
```
