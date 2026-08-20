# Technical Design: Backend Two-Dimensional Migration

## Objective

Replace the coupled `server.js` backend with one modular Node control plane. The migration is staged internally for safety, but the deliverable is a single final cutover: the old monolith, compatibility facade, old test hooks, duplicate dispatchers, and duplicate provider paths are deleted before the task is complete.

The migration preserves the public `/api/companion/...` contract, application SSE envelope, existing SQLite data and migrations, environment variables, Docker data volume, provider behavior, lease/retry semantics, and persona-private ownership rules. Internal implementation names may change only behind those contracts.

## Two-Dimensional Shape

### Vertical technical layers

```text
server/index.js                 composition root and startup
server/http/                    routes, request validation, DTOs, SSE framing
server/application/             typed flows/use cases and effect orchestration
server/domain/                  pure rules and typed facts/projections
server/contracts/               schemas, ports, transport/result contracts
server/infrastructure/          SQLite, LLM, media, jobs, files, clock
server/runtime/                  worker loop, lease reclaim, timers, shutdown
```

Dependency direction is downward only:

```text
http -> application -> domain/contracts -> infrastructure
runtime -> application/contracts/infrastructure
composition -> everything for registration only
```

Domain modules cannot import Express, SQLite, provider clients, process spawning, or filesystem helpers. HTTP modules cannot issue SQL. Infrastructure adapters cannot decide domain policy.

### Horizontal domain capabilities

```text
identity-core
memory
life-world
relationship
presence
capabilities
conversation
media
activity
```

Each capability owns rules and application steps across the vertical layers. It does not become an independent process or database. Cross-capability orchestration happens through typed flow steps and ports.

## Generic Flow Runtime

The application layer uses a registry of typed flow definitions:

```js
/** @typedef {{facts: DomainFact[], projections: ProjectionChange[], effects: EffectIntent[], presentation: PresentationEvent[]}} StepResult */
/** @typedef {{id: string, version: number, run(context, command): Promise<StepResult>}} FlowStep */
```

Shared step kinds are `ContextLoader`, `Validator`, `PolicyEvaluator`, `CapabilityDispatcher`, `FactRecorder`, `ProjectionUpdater`, `EffectPublisher`, `JobDispatcher`, and `ResultSettler`. A flow composes steps with explicit input/output contracts and dependencies. The first runtime is an in-process pipeline; definitions are serializable enough to evolve into a persisted DAG later.

The runner owns:

- flow/step ordering and correlation IDs;
- transaction boundaries for facts and projections;
- persistence of effect intents;
- effect lease, retry, idempotency and settlement;
- bounded structured logs;
- aggregation of presentation events and final results.

Steps return data and intents. They never directly call a provider, write a job row, write SSE, or open a transaction.

## Facts, Projections, Effects, Presentation

Every flow result is separated into four channels:

```text
facts          immutable domain occurrences
projections    current-state changes derived from facts/decisions
effects        post-commit external work requests
presentation   request-scoped SSE/UI output
```

The transaction runner commits facts, projections and the durable effect intent atomically. External providers run only after commit. A repeated effect is rejected or settled idempotently by `effectId`/`idempotencyKey`; a continuation failure does not roll back committed facts.

The effect store is a generic port. The first adapter may reuse the existing durable job storage while preserving its lease/retry columns; if a separate generic effect table is required by mapping, it is additive and shared by all capabilities, never one table per feature.

## Ports And Contracts

Required ports:

```text
Clock
IdGenerator
LlmPort
ConversationRepository
IdentityRepository
MemoryRepository
LifeEventRepository
ScheduleRepository
PresenceRepository
ActivityRepository
EffectRepository
MediaProviderPort
AssetRepository
```

Required transport contracts:

- `ChatRequest`, `ChatResult`, `SseEvent` (`token`, `done`, `error`);
- `MessagePage` (`items`, `nextCursor`), with 20-item default for the new client;
- `ContextFragment`, `ContextBudget`, `PromptRequest`;
- `CapabilityCall`, `CapabilityResult`, `EffectIntent`;
- persona/bootstrap/activity DTOs with existing field names.

Schemas are validated at boundaries. Internal domain objects are not sent directly to the browser.

## Context And Prompt Pipeline

Horizontal modules emit structured fragments:

```text
identity/life/memory/relationship/presence/capabilities
  -> ContextFragment[]
  -> ContextBudgeter
  -> PromptSerializer
  -> LlmPort
```

The existing prompt-optimization task owns the concrete budget, relevance, recency, confidence and history policy. The new backend only provides the fragment and serializer contracts; `contextFor()` is replaced by a facade over the context pipeline and no longer performs SQL or final prompt assembly.

## Conversation And Capabilities

`streamPersonaChat()` is replaced by:

```text
ChatTransportAdapter
  -> ChatTurnFlow
      -> ChatContextReader
      -> LlmStreamingPort
      -> CapabilityDispatcher
      -> ConversationRepository
      -> EffectPublisher
      -> PresentationEventMapper
```

Native tool calls and marker fallback are both transport adapters into the same `CapabilityCall` contract. Tool JSON never becomes visible token data. The browser still receives `token`, `done`, and `error`, with `done.messages` as the authoritative ordered collection and `done.message` as a compatibility alias.

## Life, Presence, Activity And Media

- `LifeStateResolver` is pure: it receives blueprint, schedule, events, daily plan, presence snapshot and time, then returns `ResolvedLifeState`.
- `RecordLifeEventFlow` records a fact and state projection; activity, proactive, memory and media are post-commit effect/projection flows.
- `SceneEventFlow` owns presence validation and projection; it does not write life tables directly except through the repository port.
- `ActivityFlow` owns feed publication and interaction projections; comments emit memory evidence rather than writing memory tables directly.
- `MediaFlow` owns frozen media envelopes, prompt-master calls, provider effects, acceptance, retry and settlement through ports.

The old `createEvent()` option object and `runMediaJob()` type switch are removed. Their behavior is represented by registered flow definitions and effect handlers.

## Compatibility And Cutover

During migration, old `server.js` and `companionTestHooks` may be loaded by replay tests only. They are never the final application path. New code must pass contract fixtures against the same temporary SQLite database and use provider dry-run handlers for comparison.

Final cutover:

1. switch `package.json`, Docker and CI to `server/index.js`;
2. run API/SSE/worker/browser tests against the new root;
3. delete old `server.js` implementation, old hooks, old dispatcher and duplicate adapters;
4. scan production imports for old paths;
5. rerun the full suite after deletion.

Rollback is a previous complete build/commit. No runtime fallback layer remains.

## Observability And Recovery

Every flow, step and effect carries `requestId`, `flowId`, `correlationId`, `causationId`, `subjectId`, `stepId` and `effectId`. Logs record bounded status, counts, timing, retry/lease state, context selection and SSE lifecycle without full prompts or credentials. SQLite lease reclaim and retry restore incomplete effects after process restart; terminal failures are surfaced through existing debug/user-safe DTOs.
