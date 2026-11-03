# Fluctlight Clean-Start Architecture

## Status

Review candidate. This document records confirmed planning decisions, implementation evidence boundaries, and the final T12 acceptance gate. It does not authorize implementation; user approval and a started child task remain mandatory.

## Architecture Drivers

- Preserve and complete the Fluctlight product capabilities while replacing the implementation as a clean start.
- Make Python the only owner of Fluctlight domain behavior, durable state, transactions, workflows, and media metadata.
- Keep a real Node.js / TypeScript BFF without allowing it to become a second domain implementation.
- Support personal local and NAS self-hosting through an understandable Docker Compose deployment.
- Build through ordered internal tasks, then perform one complete product cutover after full-scope integration acceptance.
- Freeze the old implementation during construction and delete it in the final integration task.

## Target Topology

```text
Browser application
        |
        v
Node.js / TypeScript BFF
  - browser API and stream contract
  - request/session boundary
  - page DTOs and redaction
  - static assets and media response policy
        |
        | HTTP/JSON command + query
        | cancellable HTTP stream
        v
Python Core API
  - modular-monolith application layer
  - Fluctlight domain modules
  - transaction and authorization policy
        |
        +--> PostgreSQL: authoritative domain and workflow state
        +--> Redis: rebuildable cache and short-lived coordination
        +--> Redis Streams: committed asynchronous integration events
        +--> Object storage: image/video bytes
        +--> MTPLX / ComfyUI / h3 providers

Python Worker process(es)
  - same Python package and image as Core API
  - separate runtime command and resource limits
  - durable workflow and provider execution
```

The browser, BFF, Core API, Worker, database, Redis, and object storage form one self-hosted product deployment. They are process boundaries, not independently owned microservices.

Target capacity is a 16 GiB NAS. MTPLX, ComfyUI and h3 run remotely, so local sizing covers only BFF/Core/Worker/PostgreSQL/Redis/object storage/workflow runtime. Long-term idle footprint and operational stability are explicit gates.

The Python image exposes two runtime commands:

- `serve-api`: synchronous application API and cancellable streaming only.
- `run-worker`: durable workflow execution and Provider work.

The default NAS deployment runs one Worker container with three logical queues:

- `interaction`: pending events, deferred replies, and proactive messages.
- `lifecycle`: daily plans, timelines, relationship work, memory consolidation, and self-model work.
- `media`: image/video submission, polling, quality retry, and compensation.

Additional Worker containers may subscribe to selected queues later without changing domain code or splitting the modular monolith. The API process never consumes these queues.

## Python API Stack

- Python 3.13 with an exact patch pinned in runtime, image, CI, and local tooling.
- `uv` is the Python project/dependency runner; `.python-version`, `pyproject.toml`, and `uv.lock` are committed, installs use locked sync, and commands run through `uv run`.
- FastAPI as the HTTP/ASGI transport adapter, Pydantic v2 for transport/config/Provider schemas, and Uvicorn as the ASGI server.
- Domain modules use standard dataclasses, enums, value objects, and protocols; they do not import FastAPI, Web Pydantic DTOs, or dependency-injection objects.
- Python OpenAPI is the authoritative command/query contract and generates the internal TypeScript Core client used by Node BFF.
- Incremental Core→BFF output uses ordered `application/x-ndjson`. It exposes client-safe visible chunks/action results/terminal state, not semantic assessments, hidden reasoning, Provider chunks, or database/workflow internals.
- Node disconnect propagates through ASGI cancellation to realization. Already committed assessment/frozen state follows its workflow settlement contract.
- Liveness means process alive. Readiness checks PostgreSQL, required configuration, and runtime role; optional Provider outage does not make Core API unready.

## TypeScript And Browser Stack

- Node.js 24 LTS with an exact patch pinned in workspace, CI, and images.
- Fastify with strict TypeScript and JSON Schema/TypeBox for the BFF; route schemas produce BFF OpenAPI and a generated browser client.
- Vue 3, Vite, and Pinia remain the browser framework. The browser is rewritten against the new generated contract without changing framework for its own sake.
- Node consumes a generated Core client from Python OpenAPI. Hand-written duplicate Core or browser DTOs are prohibited.
- Browser turns use POST `fetch()` plus NDJSON. BFF validates, redacts, and maps internal Core events into a distinct browser stream envelope and propagates abort in both directions.
- Fastify plugins separate browser routes, stream transport, media proxy, health, and session transport. Plugins do not own Fluctlight rules.
- The workspace uses pnpm with `apps/web`, `apps/bff`, `apps/core`, generated `packages/core-client` and `packages/browser-client`, `infra`, and new-system `tests`.

## Human Actor And Authentication

The first complete delivery has one Owner Human Actor and mandatory authentication. Actor/Participant schema supports future Humans, but account invitation, multiple accounts, roles, and group governance are not implemented now.

Python owns Human Actor/account authority, Argon2id credential hashes, opaque sessions, expiration/revocation, authorization, setup-token use, and recovery commands. Node BFF owns secure cookie and CSRF transport only; it forwards the opaque session and never trusts a browser-supplied Actor ID.

First startup creates no default account. A one-time setup token creates the Owner and is permanently consumed. Production has no anonymous/automatic-login fallback. Recovery CLI resets the owner credential and revokes all sessions with an explicit local administrative action.

BFF is the only host/LAN-exposed application port. Core, PostgreSQL, Redis, and MinIO are internal by default. BFF→Core uses a separate service identity from the Human session. Core authorizes every command/query/media grant from the resolved session Actor and domain visibility.

## Configuration

Configuration has two layers only:

- `.env`: startup/infrastructure values required before PostgreSQL system settings can be read, including database/Redis/S3 connection credentials, internal service credential, runtime mode, bind/port, and an optional single settings-encryption key pending confirmation.
- PostgreSQL system settings: runtime-editable Provider URLs/API keys/models, workflow/config payloads, media policies, cognitive policies, and product/UI options exposed through the settings surface.

The first delivery does not introduce Vault/KMS, a secrets service, per-record data keys, a master-key ring, or a dedicated encrypted backup archive. `.env` and application data are backed up by the NAS owner according to documented procedures.

One `.env` value, `FLUCTLIGHT_SETTINGS_KEY`, encrypts sensitive system-setting fields through a vetted AEAD implementation. The settings UI is write-only for secrets and returns only configured state. Missing/wrong key is a configuration failure for sensitive operations; the system never stores or returns plaintext as a fallback.

## Provider And Model Roles

System settings define reusable `ProviderEndpoint` records and six `ModelRole` assignments: `initialization`, `cognitive_assessment`, `action_realization`, `reflection`, `embedding`, and `media_prompt`.

The five generative roles may point to one local chat model by default; embedding uses a dedicated embedding model. Each role declares required capabilities and independent token/timeout/retry budgets. Settings save runs a role-specific preflight for structured output, streaming/abort, or fixed embedding dimensions.

Every generated artifact records actual endpoint/model, capability/model version, prompt/schema version, and role budget. Provider transport normalizes protocol only and cannot infer Fluctlight semantics. The first delivery has no implicit fallback chain; role failure follows explicit interaction/workflow failure rules.

## Life World And Schedule

Each Fluctlight local date has immutable Schedule versions and exactly one active accepted version. A lifecycle workflow uses the `reflection` role to propose the current-day remainder/next day from identity, personality, behavioral policy, context, affect/drives, goals/intentions, commitments, events, and prior reflection. Python validates timezone, coverage, overlap, references, enums, and policy.

Schedule covers the full local day with explicit activity, free-time, rest, or sleep items. Replan creates a new version with previous-version link, trigger/evidence/reason, model/prompt/policy versions, and diff; history remains unchanged and only current/future segments are replaced.

Context authority is confirmed active Event, then accepted active Schedule item, then explicit `unplanned/schedule_pending`. Conversation presence overlays `current_task/user_presence` without fabricating scene/location/activity. Identity, occupation, weekday, or clock are generation inputs but never code heuristics for current life state.

Provider outage retries planning. The existing accepted plan remains valid until its day ends; deterministic commitments still project. A missing next-day plan becomes `schedule_pending`, never a default routine. Timezone changes preserve history and supersede/regenerate future versions and timers.

## Goals, Intentions, And Autonomy

Drives, Events, Human requests, or Reflection may produce LLM-owned Goal candidates. Accepted Goals produce concrete Intentions with typed time/event/semantic triggers. Triggering always rechecks current Context, Schedule/Relationship/state revisions, permission, budget, quiet hours, and concurrency before a final cognitive decision is frozen.

Pre-authorized actions include proactive/delayed messaging, Moments, media requests, Schedule proposals, life Context/Event changes, Memory/Relationship candidates, and follow-up Goals/Intentions. Identity anchors, safety/Owner permissions, Provider/infrastructure settings, destructive cross-Actor data operations, and external irreversible actions are never autonomous.

Owner governance includes per-action enabled/budget/cooldown, quiet hours, `autonomy_mode: active | paused`, and inspect/pause/resume/cancel commands. Governance creates audited lifecycle transitions and cannot erase historical facts. Paused mode blocks new autonomous external behavior but not time, Context, Schedule, decay, or already-observed facts.

## Built-In Diagnostics

The first delivery does not deploy or require OpenTelemetry, Prometheus, Grafana, Loki, Tempo, or an external log collector. The operational priority is an Owner-only built-in Diagnostics surface for rapid local development and NAS debugging.

Diagnostics must correlate BFF/Core/Worker logs, rendered prompt layers, model role/provider/model/version, structured request/response, parse/schema diagnostics, cognitive turn/frozen action, workflow/step/retry, event/outbox/inbox, media progress, and bounded errors through Fluctlight/Actor/Conversation/turn/workflow/event/correlation IDs.

The shared application schema contains `diagnostic_*` tables for events, model runs, turn chains, and workflow/domain links. Model runs and turn details default to 30 days/10,000 rows; structured logs default to 14 days/50,000 rows. Domain audit/revision/evidence is not diagnostics data and is not cleaned by these limits.

Full rendered prompt layers, bounded raw/structured model responses, and parse diagnostics are captured after typed redaction. Secrets, cookies, service credentials, authorization headers, object grants, and hidden reasoning are never captured.

Diagnostic writes are asynchronous and isolated from business transactions. Backpressure/database failure falls back to structured stdout without recursive diagnostic writes. Owner UI supports filters, live tail, correlation-chain navigation, clear, and redacted bundle export.

## Ownership Boundaries

### Node BFF Owns

- The new browser-facing command, query, media, error, and streaming contracts.
- Request/session context, transport validation, response shaping, redaction, compression, and static delivery.
- Page-oriented aggregation when it does not require domain decisions.
- Translation from the normalized Python stream into the browser stream contract.

### Python Core Owns

- Fluctlight instance lifecycle and all domain invariants.
- Identity, personality, affect/drives, relationships, context, schedules, goals, intentions, memory, behavioral policy, cognitive runtime, conversations, activities, media intent, and governance.
- PostgreSQL schema, repositories, transactions, idempotency records, outbox records, and read models.
- Workflow creation, durable timers, retries, cancellation, recovery, compensation, and administrative state.
- Provider policy, Redis usage, Redis Streams publication/consumption, and object metadata/lifecycle.

### Forbidden Crossings

- Node does not read or write PostgreSQL business tables.
- Node does not mutate Redis domain state or publish domain events.
- Node does not create, settle, retry, or cancel workflows directly; it calls a Python application command.
- Python does not expose database rows, provider chunks, or workflow-engine internals as browser DTOs.
- Redis Streams is not used for synchronous RPC.
- No business invariant has independent Node and Python implementations.

## Fluctlight Domain Shape

The required state model is defined in `research/fluctlight-domain-model.md`. A Fluctlight instance is not a chat profile. It owns stable identity and personality, dynamic affect and drives, directed relationships, current context and schedule, goals and intentions, typed memory, behavioral policy, and a cognitive runtime loop.

The cognitive runtime is a pipeline:

```text
perception
  -> appraisal
  -> state update
  -> decision
  -> action
  -> reflection
```

Each stage consumes typed facts and produces validated candidates or committed outcomes. Model output proposes semantic judgments; Python policy owns scope, authorization, invariants, idempotency, transactions, lifecycle, and audit. No stage derives hidden state by reparsing user-visible assistant text.

The executable ownership and failure contract is `.trellis/spec/backend/fluctlight-cognitive-runtime.md`. It is a mandatory context and review input for every task that touches perception, appraisal, state update, decision, action, reflection, prompting, Provider output, or fallback behavior.

Semantic inference in production code cannot use regex, keywords, substring checks, phrase/sentiment dictionaries, emoji/punctuation heuristics, fixed semantic scores, or default semantic fallbacks. Deterministic code may validate facts and protocol shape, calculate policy-owned numeric transitions, enforce hard safety, and execute frozen decisions. If the LLM result is absent or invalid, the only outcomes are explicit failure, retry, `deferred`, `no_op`, or terminal failure.

### Two-Stage Interactive Turn

```text
structured assessment call
  -> perception + appraisal + candidate decision + evidence
  -> Python validation and numeric state transition
  -> policy gate and frozen final decision
  -> separate action-realization call when content is required
  -> visible stream and committed action result
```

The realization call receives the frozen action and post-transition read model. It controls wording or media content only; it cannot introduce new state candidates. `ignore`, `delay`, and other no-content actions skip realization. Reflection never runs inline with this path and cannot extend interactive latency.

### Per-Fluctlight Concurrency

- Every observation is first persisted as an idempotent inbox fact with a monotonic per-Fluctlight sequence.
- One cognitive writer processes state-changing facts and ordered action delivery for one Fluctlight instance.
- Different Fluctlight instances process in parallel.
- Reflection reads an evidence watermark and state revisions, computes outside the interactive path, and commits with CAS. A stale proposal is discarded or explicitly rescheduled.
- Provider-heavy media execution runs independently; its completion/failure becomes another inbox fact and cannot mutate cognitive state directly.
- Future shared-conversation messages are delivered to each participating Fluctlight inbox; each instance assesses and updates its own private state independently.
- The implementation uses durable workflow ordering, idempotency, sequence, and revision checks. It does not hold a PostgreSQL row lock during model or media calls.

### Identity And Personality Governance

- `identity.id` is immutable.
- Identity anchors are changed only by explicit human governance commands.
- Lived identity fields may receive revision candidates from confirmed life facts or reflection.
- Personality traits may receive only small, policy-bounded reflection revisions backed by accumulated cross-event evidence.
- Immediate state update cannot mutate identity or personality.
- Every accepted identity/personality change is a new revision containing previous/next values, source, evidence references, reason, timestamp, policy version, and rollback status.
- A human correction creates another revision; it does not erase the model-generated history.

### Numeric Contract

- PAD and emotional momentum use `-1..1`.
- Personality, mood intensity, drives, relationships, goal state, intention confidence, appraisal factors, action costs, and other normalized continuous values use `0..1`.
- PostgreSQL columns enforce finite in-range values with constraints.
- Model output supplies semantic direction, bounded strength, confidence, and evidence, not arbitrary numeric deltas.
- Python policy calculates requested and applied deltas, clamps results, and records previous/resulting state.
- Growth and decay are functions of elapsed wall time, not Worker tick count. Half-life or explicit per-time-unit rates are preferred over ambiguous per-tick rates.

## Future Multi-Participant Conversations

Group chat is not implemented in this architecture program. However, the new model must not encode “one user per Fluctlight instance” into message, relationship, memory, or event identity.

The confirmed model is:

- An Actor is either a Human or a Fluctlight instance.
- A conversation has one or more participant memberships.
- A message author is one participant/actor, not a fixed `user` / `assistant` enum.
- A Relationship is directed from its owning Fluctlight instance to another Actor. The reverse direction is a separate Relationship.
- Memories and events may reference actor IDs with explicit roles while remaining owned by one Fluctlight instance.
- The private internal state of each Fluctlight instance remains independently owned even when a future conversation or event is shared.

Actor references are typed; a fixed `user_id` or an untyped universal ID cannot replace ownership and authorization checks.

## Communication Model

### Synchronous Commands And Queries

Node calls a versioned internal HTTP/JSON application API. The contract is designed for the new system and has no old endpoint or DTO aliases. Domain validation remains in Python; Node performs only transport and client-boundary validation.

### Streaming

Chat and other incremental model output use a cancellable HTTP stream from Python to Node. Node owns browser framing, terminal behavior, disconnect propagation, and client-safe diagnostics. The new stream contract may reuse sound concepts from the old implementation, but it is specified independently.

### Asynchronous Events

Python writes the domain state change and an outbox record in one PostgreSQL transaction. A publisher sends the outbox event to Redis Streams. Consumers use stable event IDs and a durable inbox/idempotency record before applying side effects. Stream delivery state is not the domain source of truth.

Two streams are used:

- `fluctlight:events:v1`: durable domain/integration transport populated only from committed PostgreSQL outbox rows.
- `fluctlight:progress:v1`: short-lived Worker/media progress that may be dropped and is never a business fact.

Durable consumers read through consumer groups, commit consumer-owned effects plus an inbox `event_id` in PostgreSQL, then `XACK`. `XAUTOCLAIM` recovers abandoned PEL entries. Poison events create bounded PostgreSQL failure records rather than remaining pending forever.

Redis uses AOF `everysec` plus normal backup, but PostgreSQL remains the rebuild authority. Stream trim is allowed only behind critical group progress/pending windows. Missing or lost transport entries are republished from outbox. Temporal workflow state and delayed execution never use Streams.

## Python Modules

This confirmed map defines in-process modules inside one Python deployable, not microservices. All application tables share one PostgreSQL schema; each module owns its tables and presents one small external interface. Internal repositories and adapters are not exported to callers.

### `actors`

Owns Actor identity, type (`human | fluctlight`), lifecycle status, and typed Actor references. It does not own Fluctlight internal identity/personality or Conversation membership.

Interface intent: register/resolve/deactivate Actor identities and validate typed references.

### `fluctlights`

Owns Fluctlight lifecycle, identity, personality, behavioral policy, initialization mode, revisions, governance, and deletion/retirement policy.

Interface intent: create a Fluctlight instance, read its stable model, submit/accept/reject/rollback governed revisions, and retire the instance.

### `inner_state`

Owns affect/PAD/mood/momentum/regulation, drives/conflicts, goals, intentions, numeric event history, and current revisions.

Interface intent: read the post-decay snapshot, apply a validated semantic assessment, apply goal/intention transitions, and accept bounded reflection recalibration.

### `life_world`

Owns context, current scene/activity/location/environment, schedule, life events, supporting world facts, interruption state, and time-based projections.

Interface intent: read an observation snapshot, plan/replan a local day, apply a confirmed life event, and resolve current context at a wall-clock time.

### `relationships`

Owns directed Fluctlight-to-Actor relationship state, evidence, trends, emotional association, revisions, reflection, and rollback.

Interface intent: read one directed relationship, apply validated interaction evidence, and accept/reject/rollback a reflection revision.

### `memory`

Owns episodic, semantic, relationship, and autobiographical memories, provenance, evidence, confidence, revision, retrieval, consolidation, correction, and forgetting. Working memory is a bounded read model composed from current Conversation/Cognition facts, not an unbounded duplicate message store.

Interface intent: retrieve a bounded evidence-scoped memory context and apply governed consolidation/correction/forgetting commands.

### `conversations`

Owns Conversation identity, Participant membership, ordered Messages, author Actor references, read/delivery state, and conversation-scoped context. It does not infer semantics or mutate Fluctlight internal state.

Interface intent: open/manage a Conversation, append an authored Message, read stable pages, and record ordered action delivery.

### `cognition`

Owns per-Fluctlight inbox facts and sequence, semantic assessment records, policy decisions, frozen actions, action results, reflection evidence windows, and the two-stage cognitive orchestration.

Interface intent: enqueue an observation, process the next turn, realize a frozen action, and run/commit reflection through owned module interfaces.

`cognition` is an application orchestration module. It may depend on the public interfaces of `fluctlights`, `inner_state`, `life_world`, `relationships`, `memory`, `conversations`, `moments`, and `media`; those modules never depend back on `cognition`.

### `moments`

Owns feed entries, comments, reactions, visibility, Actor authorship, publication status, and feed read models. It does not own the life event or cognitive decision that caused publication.

Interface intent: publish a frozen action result, interact with an entry, manage visibility, and query feeds.

### `media`

Owns media intent, asset identity, object metadata/checksum, references, generation lifecycle, Provider request identity, quality decisions, deletion/tombstone state, and media read authorization.

Interface intent: request generation from a frozen action, record Provider progress/result, attach an accepted asset, read/authorize media, and delete through lifecycle policy.

### Platform Modules

`workflows`, `events`, `persistence`, `providers`, `object_storage`, `cache`, `observability`, and `configuration` are platform modules/adapters. They do not own Fluctlight semantic state. Owning domain modules define workflows and events; the platform supplies durable execution, transport, storage, and telemetry.

### Dependency Rules

```text
HTTP adapters / Worker commands
           |
           v
application commands + cognition orchestration
           |
           v
domain module interfaces
           |
           v
PostgreSQL / Temporal / Redis / Provider / Object adapters
```

- Domain modules do not import HTTP, Temporal, Redis, object storage SDKs, or Provider clients.
- One module cannot query another module's tables or import its internal repository.
- Cross-module work uses public module interfaces under an application-owned unit of work, or a committed domain event when atomicity is not required.
- Modules publish semantic domain events; the outbox/Redis adapter owns delivery mechanics.
- Tests use the same external module interface as callers. Old shallow-module tests are not copied.

## Transaction Model

Application commands own a Unit of Work. Domain module interfaces participate in the same short PostgreSQL transaction without exposing internal repositories or committing independently. Cross-module atomicity is allowed only for one application invariant; it is not permission to query another module's tables.

Atomic operations include Fluctlight creation/retirement, inbox assessment plus accepted state transitions and frozen decision, visible action delivery plus ordered position, and reflection revisions guarded by evidence watermark/CAS.

External calls never run inside the database transaction. The command writes a stable intent and outbox row with the business state; after commit, the Worker performs LLM, Redis Streams, object storage, ComfyUI, h3, notification, or cleanup work with stable workflow/request IDs.

A cognitive turn uses short phases:

```text
TX A: claim inbox fact and capture revisions
external assessment
TX B: verify revisions, apply state, freeze decision, write outbox
external realization/provider work
TX C: persist action result, ordered delivery, and outbox
```

Revision conflict cannot overwrite current state. External success followed by process failure before TX C is recovered by stable idempotency keys and workflow replay, not a distributed transaction.

## PostgreSQL Data Access

- SQLAlchemy 2 with Psycopg 3 async driver/pool and Alembic migrations.
- One `public` application schema, one shared MetaData/naming convention, and one linear Alembic revision graph.
- Modules declare and own their table definitions/mappings/repositories in their own packages; other modules cannot import them.
- SQLAlchemy Core is the default for pgvector, FTS, outbox/inbox, locking, JSONB, arrays, partial indexes, and complex queries. ORM is optional inside one module and cannot cross its interface.
- One application Unit of Work owns one AsyncSession. AsyncSession is not shared across concurrent tasks.
- ORM rows/entities never enter domain interfaces, FastAPI routes, generated DTOs, or other modules.
- Production runs an explicit migration command; API/Worker verify expected revision and never call `create_all()` or auto-upgrade.
- Tests use temporary real PostgreSQL. Migrations validate empty→head and previous-release→head; concurrency/workflow tests use real commits/restarts.

## Memory Retrieval

PostgreSQL owns typed Memory content, provenance, Actor/Conversation references, confidence, importance, emotional significance, visibility, lifecycle, and revisions. A separate pgvector embedding row is a rebuildable index keyed by memory ID, embedding model ID, and dimensions.

Working memory is composed from current Conversation and unresolved Cognition facts and is not embedded as a duplicate durable memory. Episodic, semantic, relationship, and autobiographical memories may be embedded asynchronously after the authoritative memory transaction commits.

Retrieval applies hard ownership/visibility/type/Actor/Conversation/time/status filters, then combines PostgreSQL full-text and vector candidates, recency, importance, and emotional significance. A bounded LLM reranker may select from the authorized candidate set before prompt budgeting. FTS is lexical retrieval only and cannot create semantic state.

Exact vector search is the default for local-scale data. HNSW is enabled only after benchmarked volume/latency/recall thresholds justify it. Embeddings from different models or dimensions are never compared; model changes use separate indexes and background rebuilding.

## Media Storage And Access

The application depends on an S3-compatible object-storage interface. Docker Compose uses a pinned MinIO single-node persistent-volume deployment by default. MinIO-specific administration APIs do not enter application modules.

The Python `media` module owns asset identity, authorization, references, Provider provenance, MIME, size, SHA-256, ETag, object version, bucket/key, generation state, tombstones, and physical-deletion state. Stable object keys use generated asset/version identities rather than user filenames.

Buckets are private. Browser media requests go to Node BFF; Python authorizes the Actor and returns a short-lived internal media grant, then Node proxies the object stream with Range, ETag, Content-Type, and cache semantics. Direct browser presigned transfer is a future deployment optimization, not the local/NAS default.

Upload/generation and deletion use committed intents:

```text
TX: create intent/asset + outbox
Worker: generate/fetch and upload stable object key + checksum
TX: validate revision, mark ready, attach reference

TX: remove references, tombstone + outbox
Worker: delete object/version
TX: mark physically deleted
```

Bucket versioning is enabled and lifecycle rules expire obsolete versions. PostgreSQL records SHA-256 and byte size independently of ETag. Backup covers PostgreSQL and object data under one backup manifest; a single-node object server is not itself a backup.

## Persistence Model

PostgreSQL is the only authoritative store for user-owned and operationally durable state. Redis contains only rebuildable cache entries, short-lived coordination, and transport bookkeeping. Object storage is authoritative for media bytes; PostgreSQL owns media identity, metadata, references, checksums, lifecycle, and deletion state.

The system starts with a new schema and empty stores. It does not import, read, dual-write, or translate old SQLite data, old media locators, old jobs, old debug records, or old client payloads.

## Workflow Direction

T01 rejected DBOS 2.30.0. Queue isolation, durable sleep, cancellation, backup/restore, idempotency and measured resources passed, but the official client lacked required pause/restart operations and active workflow history did not replay under a new application version. The accepted FAIL report is `research/dbos-runtime-gate-report.md`.

Temporal is the accepted sole workflow runtime. The NAS topology is one grouped non-HA Temporal Server process/container, PostgreSQL default and visibility databases, no Elasticsearch/OpenSearch, Temporal UI/Prometheus/OTLP collector off by default, and Python Workers polling `interaction`, `lifecycle` and `media` task queues.

T01B proved the core topology, stable Workflow IDs, Signals/Queries/Updates, bounded Activity heartbeat/timeout/cancellation, durable timers/restarts, Event History replay, current Worker Deployment Versioning and `continue_as_new`. Remaining live reset/management integration belongs to T02, media-specific long Activity/idempotency belongs to T09, and active-workflow restore belongs to T11. Domain facts remain in the Fluctlight application schema; Temporal owns execution history only.

The T01B report remains `FAIL` under the superseded resource/soak gate, but parent D036 accepts the core result and removes resource-duration gates as blockers. Celery/custom queues and concurrent workflow runtimes are prohibited.

## Delivery And Cutover

- The architecture program may be decomposed into ordered Trellis child tasks.
- Each child produces internally integrated code and implementation evidence, but does not replace a portion of the running old product or claim acceptance.
- The old implementation and tests are frozen evidence. They are not updated to accept new contracts or kept green as a requirement for new-system tasks.
- T12 final integration re-runs the complete required product capability inventory on the new deployment; T03-T11 handoffs are evidence inputs, not substitutes for final verification.
- One cutover replaces the old frontend, backend, storage, workers, and media path together.
- The final task deletes old code, tests, dependencies, entrypoints, CI paths, and obsolete documentation so the repository has one production implementation.

## Multi-Session Documentation Contract

- `READ_FIRST.md` is the mandatory entrypoint for every future session.
- `decisions.md` is the numbered decision ledger. A child task cannot silently contradict a decision; it must return the parent task to planning and record the replacement decision.
- `prd.md` owns product requirements/scope/acceptance; `design.md` owns architecture/data flow/trade-offs; `implement.md` owns ordered work, ownership, gates, commands, and rollback points.
- `research/capability-inventory.md` owns rebuild/close/delete/future-only classification.
- `.trellis/spec/backend/fluctlight-*.md` owns executable cross-layer contracts with failure matrices and tests; T12 owns their final acceptance union.
- Child prompts/manifests include only the required specs/research plus the parent decision IDs they implement.
- Old code/tests are frozen evidence and are never a child-task or T12 positive quality gate; T12 may only use them for deletion/scope proof.
- Default execution is strictly serialized: one `in_progress` child and one writing implementation session. Other sessions may research/review/check read-only. Parallel writers require a parent-approved split into non-overlapping worktree tasks; shared migration/OpenAPI/generated-client/Compose/spec files retain one integration owner.

## Terminology

New requirements, product copy, architecture, APIs, schemas, and identifiers use `Fluctlight`, `Fluctlight system`, and `Fluctlight instance`. The terms prohibited by `CONTEXT.md` appear only when citing historical code or records.

## Remaining Validation Gates

- T01 DBOS and T01B Temporal are archived evaluation evidence; parent decisions accept Temporal core and unblock preparation of T02.
- Child-owned detailed route/schema/table/cache-key definitions must satisfy the frozen ownership and code-spec contracts; they are implementation design details, not permission to reopen parent decisions silently.
- T11 prepares backup/restore and active-workflow upgrade implementation evidence; T12 re-runs and accepts those scenarios before cutover.
- T12 must pass the full required capability matrix, exclude Future-only/reserved/placeholder-only positive cases, and delete every frozen old implementation path before the product is considered delivered.

## Validation Ownership And Scope Exclusions

- T03-T11 own implementation evidence only: formatting/type/import checks, local development checks, generated-artifact checks, and handoff records. Their results never establish child PASS, production readiness, or cutover authorization.
- T12 consumes all T03-T11 handoffs and re-runs the complete required spec/error matrix, capability scenarios, cross-module e2e, failure/security/redaction, backup/restore/upgrade, Compose, and legacy-deletion proof.
- `REQUIRED_ACCEPTANCE` includes Must Rebuild and Incomplete Old capabilities from `research/capability-inventory.md`; `REQUIRED_CLEANUP` includes Old Scaffolding deletion proof.
- `EXCLUDED_FUTURE_OR_RESERVED` and `EXCLUDED_PLACEHOLDER_ONLY` capabilities do not generate positive T12 acceptance cases. T12 may run only a negative scope guard to prove they are not exposed or falsely marked delivered.
- A real lifecycle state named `placeholder`, `pending`, `deferred`, or `failed` remains in scope when it is part of a Required capability; only a placeholder-only feature with no real producer/consumer closure is excluded.
