# Persona Analysis And Media Job Contracts

## Scenario: LLM Persona Analysis

### 1. Scope / Trigger

- Trigger: `POST /api/companion/interviews/analyze` receives a long natural-language description and must create a ready interview without heuristic extraction.
- The application analyzer owns the persona schema; the MTPLX JSON port owns transport and bounded provider failures; the interview repository owns the ready session transaction.

### 2. Signatures

```js
createInterviewAnalyzer({jsonCompletion}).analyze({description, signal})
createMtplxJsonCompletionPort({provider, settings, timeoutMs?}).complete({model, messages, signal, trace})
createInterviewRepository({database}).createReadyInterview({answers, source, inferredFields})
```

### 3. Contracts

- Request: `{description: string}` with 1-6000 trimmed characters.
- Model result: exactly `{answers, inferredFields, blueprint}`. `answers.name`, `answers.role`, and `answers.foundation` are required strings. List fields are bounded string arrays.
- List fields (`interests`, `routine`, and `supportingCast`) are persisted and exposed as bounded short-string arrays. The analyzer may accept an explicit structured item from the model and project its supported text field (`name`/`label`/field-specific equivalent) to one string; unknown keys, missing text, wrong types, item-count overflow, and text-length overflow still fail closed. This is schema normalization only: it must not infer or rewrite meaning from the description.
- The analyzer also canonicalizes bounded alternate shapes for legacy and extended fields: nested `identity` labels, `relationship` objects, `languageStyle`/`toneAndVocabulary` objects, `personalityCoordinates` aliases, structured personality lists, background arrays, and `interactionBoundaries`. Canonical output remains stable and field-specific; aliases never enable arbitrary keys or keyword inference.
- Success response: `{status:'ready', source:'llm', interviewId, answers, preview, inferredFields}`. The raw description is request-scoped and must not be in `companion_interview_sessions.answers_json`, API DTOs, or prompt-run traces.
- Activation consumes only the persisted structured answers plus explicit user overrides.
- `timeoutMs` is optional. Production leaves it unset, so slow local models are not killed by an automatic deadline. Explicit positive values are available for tests or deployments that need a bounded request.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/blank/overlong description | HTTP 400 before provider work |
| Provider unavailable, timeout, empty body | HTTP 502; no interview/persona mutation |
| Non-JSON, fenced JSON with unknown keys, wrong types, missing required fields | HTTP 502; no interview/persona mutation |
| Analyzer port absent in a custom composition | bounded 501; repository analyzer is never selected |
| Valid model result | one ready interview row; persona exists only after `/activate` |

### 5. Good/Base/Bad Cases

- Good: a description with no literal `叫/名为/名字是` phrase still produces the model-supplied name and role.
- Base: an optional interest or routine field is omitted by the model and remains an empty structured list.
- Bad: a repository regex extracts a name, a raw description is stored as `foundation`, or a provider failure creates a draft session.

### 6. Tests Required

- Assert provider receives `stream:false` and the configured model, and returns bounded JSON.
- Assert malformed/unknown/oversized output returns 502 and session/persona counts remain unchanged.
- Assert the repository has no natural-language rule analyzer and the service fails closed when no analyzer is injected.
- Assert activation overrides persist user values but never persist the raw description or debug trace.

### 7. Wrong vs Correct

#### Wrong

```js
const name = description.match(/(?:叫|名为|名字是)\s*([^，。,.]+)/)?.[1] ?? '新朋友';
```

#### Correct

```js
const extraction = await interviewAnalyzer.analyze({description, signal});
return interviewRepository.createReadyInterview(extraction);
```

## Scenario: Media Follow-Up Compensation

### 1. Scope / Trigger

- Trigger: a media submit returns an external locator and the subsequent poll enqueue can fail or be interrupted by process termination.
- Activity projection and media effect enqueue are one caller-owned SQLite transaction. Source/poll repair stays inside the media application/worker boundary.

### 2. Signatures

```js
mediaJobService.submit(sourceJob, context)
mediaJobService.compensatePoll(compensationJob, context)
mediaJobRepository.enqueuePoll({job, payload, now})
mediaJobRepository.enqueuePollCompensation({job, payload, now})
```

Deterministic payload keys:

- poll: `media:poll:<sourceJobId>:<externalId>`
- compensation: `media:poll-compensation:<sourceJobId>:<externalId>`
- quality successor: `media:quality-retry:<sourceJobId>:<retryCount>`

### 3. Contracts

- Pending source result is written before settlement and contains `provider`, `externalId`, `promptId`, `pending:true`, and the frozen media payload.
- If normal poll enqueue fails, source settles complete and one `media_poll_compensation` job is enqueued with the source id, source job type, provider, external id, kind, target association, and deterministic key.
- Compensation checks source completion and target state, reuses the frozen payload, and creates one poll job only when the target is still processing and the poll is absent.
- Compensation retains the original activity/message association so a terminal missing-locator, missing-target, or invalid-target failure can mark the user-visible target failed under the same persona scope.
- Generic dispatcher remains the owner of lease, retry/backoff, and terminal settlement.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Activity insert or media enqueue fails in the same transaction | both roll back |
| Poll already exists | compensation completes as replay; no provider call |
| Target is ready/failed/none | compensation completes as skipped |
| Source missing/incomplete or frozen locator missing | bounded terminal failure |
| Provider temporary failure | generic retry; target remains processing |
| Attempts exhausted | target becomes failed with bounded error |

### 5. Good/Base/Bad Cases

- Good: a crash after source completion creates one compensation job and a later tick creates one poll.
- Base: repeated quality acceptance retries find the same deterministic successor.
- Bad: re-submit the provider because poll enqueue failed, create a random successor id, or leave a processing target with no durable follow-up.

### 6. Tests Required

- Assert activity insert and media effect enqueue roll back together.
- Assert poll enqueue failure creates compensation, compensation repairs one poll, and provider submit is called once.
- Assert repeated compensation, quality retry, stale leases, and worker restart do not duplicate jobs/assets.

### 7. Wrong vs Correct

#### Wrong

```js
settle(sourceJob, {status: 'complete'});
enqueue({jobType: 'chat_media_poll'}); // crash here leaves processing target orphaned
```

#### Correct

```js
const poll = enqueuePoll(sourceJob);
if (!poll) enqueuePollCompensation(sourceJob);
settle(sourceJob, {status: 'complete'});
```

## Scenario: Cross-Day Daily-Plan Continuity

### 1. Scope / Trigger

- Trigger: a persona crosses a local calendar day and its previous `daily_plan` job has completed, been delayed, or been replayed after worker recovery.
- The worker must restore the durable plan/job chain for each missing local date; it must not rely on an in-process timer or the browser polling loop.

### 2. Signatures

```js
dailyPlanRepository.read({personaId, planDate?, at?})
dailyPlanRepository.ensure({personaId, planDate?, at?, runAfter?, blueprint?})
daily_plan({personaId, planDate, dailyPlanId}, {now})
```

### 3. Contracts

- `ensure()` is persona-scoped and idempotent on `(personaId, planDate)`. A missing date creates one `ready` plan with a continuous `daily_plan_baseline` slot; an existing plan is reused without overwriting items, event decisions, or user schedule bindings.
- When `at` is provided without `planDate`, reads resolve the persona's local date from the blueprint timezone. A latest-plan compatibility read is allowed only when no date/time is supplied.
- Each plan date has at most one logical `daily_plan` job. The payload contains `{personaId, dailyPlanId, planDate, idempotencyKey}`; replay checks the stable date key before enqueueing.
- The maintenance handler uses dispatcher `context.now`, ensures every missing date from the job's source date through the current local date, syncs each plan through `timelineFlow.syncDailyPlanSlots()`, then ensures the next date's job.
- Newly created future jobs use the target local midnight as `runAfter`; this prevents a catch-up worker from immediately generating an unbounded chain of future plans in one tick.
- The catch-up span is bounded (currently 366 local days). An over-limit or failed write returns a retryable/terminal bounded job error; it must not report `ready` when no ready row exists.
- Existing non-baseline slots and baseline-linked event decisions remain protected by the timeline repository and current candidate idempotency rules.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Same persona/date is ensured repeatedly | one plan, baseline slot, and logical daily-plan job are reused |
| Worker is offline across multiple local dates | next claimed job catches up every missing date up to the bounded span |
| `dailyPlan` row is missing, not ready, or belongs to another persona | handler fails/retries; it never returns synthetic `ready` |
| Plan/job payload has an invalid calendar date | bounded validation failure; no new rows are written |
| Existing schedule/event-decision-bound slot is replayed | slot is preserved and no duplicate candidate is created |
| Catch-up creates a future job | `runAfter` is the future date's local midnight, not the current tick time |

### 5. Good/Base/Bad Cases

- Good: a Shanghai persona created on D is first processed on D+2; D+1 and D+2 each receive one ready baseline plan, and D+3 is queued for its local midnight.
- Base: replaying the D+1 job after lease recovery returns the same plan/job identities and does not overwrite a user-confirmed slot.
- Bad: reading the latest D plan on D+2, marking an absent row ready by default, or enqueueing D+3 with `runAfter=now` so the worker advances forever in one day.

### 6. Tests Required

- Assert D -> D+2 catch-up creates exactly one plan, baseline slot, and logical job per date and one future job at local midnight.
- Assert repeated ensure/replay/lease recovery preserves row and candidate counts.
- Assert Asia/Shanghai and a DST transition calculate local dates and baseline boundaries correctly.
- Assert missing/foreign/not-ready rows and invalid dates fail without synthetic success.
- Assert existing non-baseline/event-bound slots survive replay.

### 7. Wrong vs Correct

#### Wrong

```js
const plan = dailyPlan.read({personaId});
return {status: 'complete', result: {status: plan?.status ?? 'ready'}};
```

#### Correct

```js
const plan = dailyPlan.ensure({personaId, planDate, at: context.now});
if (plan.status !== 'ready') throw new Error('Daily plan is not ready');
enqueue({jobType: 'daily_plan', runAfter: localMidnight(nextPlanDate)});
```

## Scenario: LLM-Gated Proactive Trigger

### 1. Scope / Trigger

- Trigger: a ready daily-plan slot reaches `startsAt` and should become an opportunity for the persona to decide whether to contact the user.
- Scheduling a candidate is not a semantic decision. The server may schedule a durable candidate from persisted timeline facts, but only the proactive LLM decision may choose `send` or `skip` and compose the user-visible message.

### 2. Signatures

```js
timelineFlow.syncDailyPlanSlots({personaId, planDate, plan, at})
timelineFlow.handleJob({jobType: 'timeline_candidate', payload}, {now})
createProactiveJobService().run('proactive_message', job, context)
```

### 3. Contracts

- `syncDailyPlanSlots()` persists slots and creates one idempotent `timeline_candidate` job per slot with `runAfter = slot.startsAt`.
- A due candidate creates a life-event fact with `proactive: true`; the life-event flow owns anti-spam, screening, safety and source idempotency checks, then publishes one `proactive_message` job when eligible.
- `proactive_message` freezes the LLM decision before delivery. `send=false` creates no visible message; `send=true` uses the existing reply projection and may later hand off a validated media capability.
- When the frozen proactive decision includes `media`, the flow must pass that complete capability (including `personaMediaConcept`) to the existing `mediaFlow.plan/apply` boundary. The server may attach authoritative source event and temporary appearance facts, but it must not invent the media concept or decide whether media is useful.
- Historical `timeline.activity_decision` job names resolve to the canonical `activity_decision` handler and do not create a second flow.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Replaying the same ready plan | one slot and one candidate job per idempotency key |
| Candidate is not due | worker does not invoke the proactive LLM |
| Candidate reaches the life-event flow | one LLM-gated proactive job, subject to existing safety/quiet/active-chat guards |
| Proactive decision returns `send=false` | job completes with a bounded skip result and no user-visible message |
| Proactive decision includes an invalid/missing media concept | proactive media is rejected terminally; no provider or fallback prompt is called |
| Legacy timeline job spelling | canonical handler runs under the same lease/settlement owner |

### 5. Good/Base/Bad Cases

- Good: a daily-plan slot becomes an opportunity; the model decides it is not a natural time to interrupt and returns `send=false`.
- Base: the model returns `send=true`; the existing proactive message flow persists one reply and freezes the decision for retries.
- Good media: the model returns `send=true` with a complete media capability; the existing media flow creates the placeholder/job and the media worker owns provider execution.
- Bad: a server keyword rule directly sends a message or image, or a timeline effect is written under an unregistered job type.

### 6. Tests Required

- Assert daily-plan replay produces one candidate per slot and due execution passes `proactive: true` to life-event creation.
- Assert the resulting life-event path creates a `proactive_message` job and not an unknown timeline job.
- Assert legacy `timeline.activity_decision` jobs resolve to the canonical handler.
- Assert the existing proactive flow tests still cover frozen LLM decisions, `send=false`, retries and lease recovery.
- Assert a proactive media decision calls `mediaFlow.plan/apply` with the model-supplied concept and does not use server-side keyword logic.

### 7. Wrong vs Correct

#### Wrong

```js
if (slot.startsAt <= now) sendMessage('该主动联系用户了');
```

#### Correct

```js
enqueue({jobType: 'timeline_candidate', runAfter: slot.startsAt, payload: {candidate}});
// The due candidate creates a proactive_message job; the LLM decides send/skip.
```
