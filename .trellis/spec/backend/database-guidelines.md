# SQLite Storage Guidelines

> Historical note: this file is retained as migration evidence for the retired
> SQLite runtime. The active system uses PostgreSQL and the Go migration bundle
> under `apps/core-go/internal/migrations/`.

## Storage Model

Persistence uses one `better-sqlite3` database file in WAL mode. `companion_schema_migrations` is the authoritative version ledger. New companion resources are normalized `companion_*` tables: personas and immutable foundation revisions, life blueprints and state, events and activities, persona-private memories, conversations/messages, media references, and durable jobs.

Persona rows carry an explicit `initialization_mode`: `llm_defined` preserves the strict analyzer/blueprint path, while `blank_slate` permits empty identity anchors and an empty emergence container. The mode is persisted with the persona and must not be inferred from nullable name/role values.

`companion_settings` is the only low-frequency JSON settings row. Do not put independently queried or high-growth data in it.

## Read And Write Rules

- Query and update the table that owns the resource. Every persona-private read includes `persona_id` directly or through its owning conversation/activity.
- Use short SQLite transactions for a state transition that creates related rows, such as event + current state + activity, or message + conversation timestamp.
- A native `appearance_event` change writes its audit life event and normalized current appearance projection in one caller-owned transaction. Appearance-only audit events must not become authoritative scene/situation events; the resolver overlays the persisted outfit onto the selected life-state source.
- When an application flow creates a message placeholder and its durable job together, the caller owns one transaction covering both inserts, including every row in a count batch. The conversation repository writes and returns the raw message row; the media/application layer writes the job envelope and shapes browser DTOs. A later job failure must roll back all placeholders and jobs together.
- Claim jobs with a conditional lease update inside a transaction. Run MTPLX or ComfyUI outside that transaction, then settle only with the matching lease owner.
- A completed media poll is not successful until its generated assets and the associated activity/message target projection both succeed. If `updateTarget()` reports `changed: false`, the media job must remain retryable or fail diagnostically; never settle it as complete with an unattached asset.
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

## Scenario: Persona Contact Groups

### 1. Scope / Trigger

- Trigger: the companion client needs durable user-defined groups for AI personas and a group-aware contact view.

### 2. Signatures

- Migration 9 creates `companion_groups(id, name, is_default, created_at, updated_at)` and adds `companion_personas.group_id`.
- `GET /api/companion/bootstrap` returns `groups: [{id, name, isDefault, personaCount}]`; persona summaries include `groupId` and `groupName`.
- `POST /api/companion/groups` accepts `{name}`; `PUT /api/companion/personas/:personaId/group` accepts `{groupId}`.

### 3. Contracts

- Startup seeds exactly one immutable `默认` group and backfills existing personas to it in the same migration transaction.
- New personas are inserted with the current default group ID; group assignment updates `updated_at` and is visible in the next bootstrap response.
- The browser may filter contacts locally, but SQLite remains the authority for group membership and group counts.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Group name is blank, non-text, or longer than 60 characters | 400 with a bounded validation error; no row is inserted |
| Group name already exists | 400; no duplicate group |
| Persona or target group does not exist | 404; persona membership is unchanged |
| Default group is missing during persona creation | Fail the creation instead of inserting an ungrouped persona |

### 5. Good / Base / Bad Cases

- Good: migration seeds `默认`, a new persona points to it, and assigning `工作` survives a bootstrap reload.
- Base: an empty custom group is returned with `personaCount: 0` and can be selected by the client.
- Bad: storing the selected filter only in the browser or inserting a persona with a null `group_id`.

### 6. Tests Required

- Assert migration 9, the unique default guard, default persona assignment, group counts, bootstrap fields, successful reassignment, duplicate/blank/oversized names, and unknown-resource no-mutation behavior.

### 7. Wrong vs Correct

#### Wrong

```js
database.prepare('UPDATE companion_personas SET group_id = ? WHERE id = ?').run(groupId, personaId);
```

#### Correct

```js
// The route delegates validation and the timestamped update to one owner.
return assignPersonaGroup(personaId, groupId);
```

## Scenario: Ready Daily Plan State Authority

### 1. Scope / Trigger

- Trigger: a persona has a ready daily-plan record and state is requested by the browser, chat prompt, media prompt, or worker.

### 2. Signatures

- State projection includes source, source ID, start/end boundaries, time fact, next boundary, scene, location, and room.
- Ready plans support legacy item arrays and v2 objects containing items plus a continuous timeline.

### 3. Contracts

- A ready plan owns the entire local day. Legacy routine applies only when no ready plan exists for that persona/date.
- Persona creation must atomically create the current local day's ready plan and at least one durable `daily_plan_baseline` timeline slot; the initial maintenance job may rebuild it but must not be the only source of the first schedule.
- Explicit user schedules override only their overlap; generated plan slots resume before and after that interval.
- Baseline slots use the blueprint default room. A first activity that means sleep or lying in produces a pre-first sleep baseline.
- Plan item overlap is invalid. A projected state has exactly one authoritative source and a trusted time fact.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Plan items overlap | Reject generated plan; do not persist ambiguous daily slots. |
| Ready plan begins at 10:00 with sleep semantics | 08:47 resolves to a sleep baseline, never a student lesson routine. |
| User schedule overlaps 11:00–12:00 of an AI 10:00–13:00 slot | User schedule is primary only during its interval; AI slot resumes at 10:15 and 12:15. |
| No ready daily plan | Legacy routine remains the conservative fallback. |

### 5. Good/Base/Bad Cases

- Good: sleep until ten, then game has a midnight-to-ten bedroom sleep baseline and a ten-to-thirteen game slot.
- Base: a state with an unknown time fact makes no end-time claim.
- Bad: treating a student role as evidence of a current class when the ready plan says otherwise.

### 6. Tests Required

- Pin an Asia/Shanghai 08:47 fixture with a 10:00–13:00 sleep/game plan; assert no 上课中 state.
- Assert state API, chat context, media intent, and sleep availability share source, scene, room, and end boundary.
- Assert partial explicit schedule overlays and overlapping LLM-plan rejection.

### 7. Wrong vs Correct

#### Wrong

    return activeSchedule || routineForStudent(persona);

#### Correct

    return explicitSchedule || dailyPlanSlotAt(persona, at) || legacyRoutine;

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
- Sleep deferral is eligible only when the resolved state explicitly indicates sleep (or an equivalent explicit sleeping flag); clock hour, ordinary rest, an empty daily plan, and a shared scene never imply sleep by themselves. The first sleep reply decision is an LLM-gated bounded choice using relationship/affect/random facts, with a deterministic fallback when the optional decision call is unavailable.
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

## Scenario: Chat-declared pending events and one-shot proactive evaluation

### 1. Scope / Trigger

- Trigger: the persona may register a bounded “follow up later” fact during a normal chat, then deliver at a durable time without scanning every persona or reinterpreting the chat history.
- This crosses the chat SSE marker, SQLite migration, durable worker, current-chat context, and assistant-message provenance.

### 2. Signatures

- Marker: `<pending-event>{"schemaVersion":1,"summary":string,"notBefore":ISO-with-offset,"expiresAt":ISO-with-offset,"dedupeKey":string}</pending-event>`.
- `normalizePendingEventCall(value, reference?) -> {schemaVersion, summary, notBefore, expiresAt, dedupeKey}`; rejects missing fields, timezone-less timestamps, invalid ordering, past `notBefore`, or a horizon beyond 30 days.
- `createPendingEvent(persona, value, sourceMessageId) -> {pendingEvent, jobId, created}`; binds the source to a user message owned by the same persona and creates one `pending_event` job at `run_after = notBefore`.
- `companion_messages.proactive_pending_event_id` is nullable provenance for assistant messages created from a pending event.
- Structured proactive decision: `{"schemaVersion":1,"send":boolean,"reason":string,"message":string}`; `send=true` message is limited to 90 characters.

### 3. Contracts

- `companion_pending_events` is persona-scoped with `status` in `pending|triggered|consumed|cancelled|expired`, immutable summary/time fields, `UNIQUE(persona_id, dedupe_key, not_before)`, and a due index.
- `source_message_id` and `proactive_pending_event_id` use `ON DELETE SET NULL`; pending jobs/events are removed before conversations/messages during persona deletion.
- Marker parsing removes complete, oversized, malformed, or unclosed marker regions from the visible assistant text; invalid capability data never blocks the ordinary chat response.
- The due worker may evaluate a pending event even when the latest user message is under ten minutes old, because the marker is an explicit follow-up authorization. It sends the current persona context, the frozen pending fact, and at most the existing recent-message window (18 messages) to one structured model call.
- The model decision is frozen in `companion_jobs.result_json.decision` under the active lease before delivery. Retries reuse it and do not call the model again. Terminal evaluation failure changes a `triggered` pending event to `cancelled`.
- A life-event proactive job remains distinct: if the latest user message is under ten minutes old, it is skipped before any model call; the life event fact itself remains persisted.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/invalid marker JSON, no timezone, empty summary, `expiresAt <= notBefore`, or horizon > 30 days | Strip marker, keep normal chat text, create no pending row/job |
| Source message missing, assistant-owned, or another persona's message | Reject pending registration; no row/job |
| Same persona, `dedupeKey`, and `notBefore` already registered | Return existing pending row/job; do not duplicate |
| Pending event is past `expiresAt` before execution | Mark `expired`, complete job, make no model call |
| Pending event model call fails but retries remain | Keep row `triggered`, requeue job with bounded retry |
| Pending event model call reaches terminal failure | Mark row `cancelled`, job `failed`, persist bounded diagnostic, no visible message |
| Life event arrives during active chat | Persist life event only; no proactive evaluation/job |
| Stale lease or duplicate settlement | Conditional update changes zero rows; never duplicate a message |

### 5. Good / Base / Bad Cases

- Good: a chat response registers “下午面试结束后问问感受” with absolute `notBefore`/`expiresAt`; one durable job later evaluates the current conversation and writes one pending-provenance assistant message.
- Base: the current conversation makes an intervention awkward; the model returns `send=false`, the pending row becomes `consumed`, and no assistant message is inserted.
- Bad: scanning all personas every ten minutes, deriving a timestamp from free text on the server, exposing marker JSON in the streamed UI, or regenerating a different message after a lease retry.

### 6. Tests Required

- Assert migration v8 tables/columns/indexes and persona deletion with source-message and pending-message provenance.
- Assert marker normalization/removal for valid, duplicate, malformed, oversized, and unclosed markers; assert marker redaction before SSE token emission.
- Assert source ownership, timezone/horizon/order validation, exact dedupe, expiry without model call, active-chat pending intervention, and life-event active-chat suppression.
- Mock one structured model response and assert one call, frozen decision reuse, `send=false` consumption, terminal failure cancellation, stale lease rejection, and no duplicate assistant message.

### 7. Wrong vs Correct

#### Wrong

```js
// Re-read the whole chat every ten minutes and infer a follow-up timestamp.
scanAllPersonasAndGuessPendingEvents();
```

#### Correct

```js
// The model explicitly registers a bounded fact; the worker reuses its frozen
// time/source and evaluates exactly once at the durable due boundary.
const pending = createPendingEvent(persona, marker, currentUserMessage.id);
enqueueJob({jobType: 'pending_event', runAfter: pending.notBefore, payload: {pendingEventId: pending.id}});
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

## Scenario: Natural-language persona initialization

### 1. Scope / Trigger

- Trigger: the active browser wizard accepts one bounded natural-language description and asks the server to extract only fields needed for a persona preview.

### 2. Signatures

- Migration 10 adds `source TEXT NOT NULL DEFAULT 'interview'` and `inferred_fields_json TEXT NOT NULL DEFAULT '[]'` to `companion_interview_sessions`.
- `POST /api/companion/interviews/analyze` accepts `{description: string}` and returns a ready interview view with `source`, normalized `answers`, `inferredFields`, and `preview`.
- `POST /api/companion/interviews/:interviewId/activate` remains the only persona persistence boundary for this flow.

### 3. Contracts

- Descriptions are trimmed, bounded to 6000 characters, sent to the server-side `lmCompletion()` provider, and never stored in SQLite or returned in the preview.
- Model output is a strict object with only `answers` and `inferredFields`; answer keys are limited to `interviewQuestions` and values are bounded by their field limits.
- Missing `name`, `role`, or `foundation` receives a deterministic default and the field is listed in `inferredFields` until the user edits it.
- The browser submits only preview fields whose values differ from the analyzed values; untouched inferred defaults remain inferred after activation.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Description missing, non-text, or blank | `400 {error: '人格描述不能为空'}`; no provider call or session row |
| Description exceeds 6000 characters | `400 {error: ...}`; no provider call or session row |
| Provider timeout/non-2xx, empty response, invalid JSON, unknown model key, or invalid field type | `502 {error: '人格分析失败：...'}`; no session or persona row |
| User changes a generated preview field | Activation stores the override as `user` provenance |
| User leaves a generated preview field unchanged | Activation retains `inferred` provenance |

### 5. Good/Base/Bad Cases

- Good: a paragraph yields `name`, `role`, personality, and interests, while only the explicitly inferred fields are marked `inferred` in the preview and final blueprint.
- Base: a paragraph omits identity; conservative defaults appear in the preview and can be edited before creation.
- Bad: persist the raw paragraph, create a persona before analysis succeeds, or submit every displayed preview value as an override and erase provenance.

### 6. Tests Required

- Assert migration 10 columns and natural-language session source metadata.
- Mock fenced JSON success and assert allowlisted answers, defaults, provenance, activation, and raw-description exclusion.
- Assert blank/oversized input, provider failure, malformed JSON, unknown keys, and invalid types return stable errors with no new session.
- Assert activation with no preview edits retains inferred provenance and explicit edits become user provenance.

### 7. Wrong vs Correct

#### Wrong

```js
const persona = createPersona(await extractFromDescription(req.body.description));
```

#### Correct

```js
const interview = await createNaturalLanguageInterview(req.body.description);
// Wait for explicit preview confirmation, then activateInterviewWithLifeModel().
```
