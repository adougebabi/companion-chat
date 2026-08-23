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
