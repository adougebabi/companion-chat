# Technical Design: Fluctlight Naming

## Scope

This task is a terminology and product-renaming change. It does not implement a new consciousness runtime, memory system, goal engine, backend module split, frontend performance pass, or native tool-call migration.

For the deferred backend discussion, the primary problem is architectural layering and dependency direction, not process/database isolation per 摇光实例. The order of investigation is: horizontal layers first, vertical domain modules second, and only then whether an instance-level runtime boundary is needed.

## Two-Dimensional Architecture Model

The target architecture is a matrix rather than a single stack. The vertical axis describes how requests and durable work move through the system; the horizontal axis describes the long-lived capabilities of a 摇光实例.

### Vertical technical axis

```text
Transport / Interfaces
  -> Application / Use Cases
      -> Domain Rules
          -> Ports / Contracts
              -> Persistence + External Adapters + Runtime
```

- **Transport / Interfaces** owns HTTP routes, SSE framing, request validation, DTO mapping, and debug exposure.
- **Application / Use Cases** owns orchestration such as one chat turn, memory update, life advancement, capability execution, media request, or activity publication.
- **Domain Rules** owns identity, memory, life, relationship, presence, capability policy, and state-transition invariants without Express, SQLite, or provider calls.
- **Ports / Contracts** owns stable interfaces for storage, LLM, media, clock, queue, and structured tool/SSE payloads.
- **Persistence + External Adapters + Runtime** owns SQLite repositories, migrations, MTPLX/ComfyUI/h3 clients, durable leases, timers, retries, and filesystem access.

### Horizontal domain axis

Candidate capability modules are:

```text
Identity Core       stable who-I-am facts and revisions
Memory              autobiographical and relationship-scoped memory
Life World          routines, schedules, events, state projection, time progression
Relationship        familiarity, trust, boundaries, evolution, interaction evidence
Presence            current interaction context and shared scene
Capabilities        model-authorized tools, policy, validation, idempotency, continuation
Media               image/video intent, prompt pipeline, provider jobs, assets
Activity            moments/feed, comments, reactions, proactive expression
Conversation        message history, context selection, chat turn application service
```

These are not eight independent services. Each module should expose domain rules and application use cases through the same vertical axis. For example:

```text
Memory transport DTO/SSE
  -> memory use case
      -> memory selection/retention rules
          -> memory repository port
              -> SQLite memory adapter

Capabilities tool chunk/SSE
  -> capability dispatch use case
      -> tool policy and validation rules
          -> LLM/queue/job ports
              -> MTPLX and durable job adapters
```

`Conversation` may orchestrate several horizontal modules, but it must not reimplement their rules. `Capabilities` may invoke media or presence use cases through ports, but it must not write their tables directly. `Media` may persist jobs and assets through its own ports, but it must not own generic chat transport.

The first architecture review should map every current `server.js` function to one vertical layer and one horizontal module (or explicitly mark it as composition/runtime). Only after that map is stable should we decide whether a 摇光实例 deserves a dedicated runtime coordinator.

## Current Mapping Evidence And First Boundaries

The read-only mapping confirms that the current problem is cross-layer penetration, not merely file size:

- `streamPersonaChat()` is a P0 mixed boundary: HTTP/SSE, Conversation persistence, Life/deferred policy, context assembly, LLM streaming, Capabilities, marker fallback, Media/Pending jobs, schedule creation, and final DTO all execute in one flow.
- `createEvent()` is a P0 mixed boundary: Life event rules, state projection, SQLite transaction, Activity publication, Media request, Proactive eligibility, and Queue side effects are coupled.
- `contextFor()` is a P0 implicit global bus: Identity, Life, Memory, Relationship, Presence, image policy, capability prompts, and final prompt text are assembled through direct database reads.
- `submitMediaJob()` / `completeGeneratedMedia()` combine durable lease, Media domain validation, prompt LLM, provider runtime, acceptance, retry, asset persistence, and parent projection.
- Routes and workers directly issue SQL and call domain functions; `companionTestHooks` exposes a large set of entry internals, so tests currently depend on the composition root.

The first extraction order should therefore be:

1. **Ports / Contracts**: `Clock`, `IdGenerator`, `LlmPort`, `JobPort`, `MediaProviderPort`, and the smallest repository interfaces.
2. **Conversation + Capabilities**: split the chat transport adapter from a `ChatTurnUseCase` and a capability dispatcher while preserving the application SSE envelope.
3. **Life State Resolver**: extract a pure resolver from the current schedule/event/daily-plan precedence logic before changing `createEvent()`.
4. **Media Provider Port and settlement**: isolate ComfyUI/h3/provider runtime and durable lease settlement from media domain decisions.
5. **Activity/Proactive projections**, then Identity, Memory, Relationship, and Presence repositories/use cases.

Do not begin by creating one service/process per horizontal module, deleting `companionTestHooks`, or mechanically moving line ranges. During construction, a temporary facade and compatibility hooks may keep tests runnable, but they are migration scaffolding only. The final cutover must remove the old monolithic implementation, old hooks, duplicate dispatchers, and duplicate provider paths; no production code may depend on the scaffolding after completion.

## Generic Flow Runtime

The application layer uses a typed pipeline registry rather than one special-case orchestrator per feature:

```text
FlowDefinition
  -> Step<Context, Command, Result>
      -> domain capability
          -> facts / projections / effect intents
              -> generic transaction + outbox/job runtime
```

Shared step categories include `ContextLoader`, `Validator`, `PolicyEvaluator`, `CapabilityDispatcher`, `FactRecorder`, `ProjectionUpdater`, `EffectPublisher`, `JobDispatcher`, and `ResultSettler`. A flow is a composition of steps with typed input/output and explicit dependencies; it does not know Express, SSE, SQLite, or a concrete provider.

The runner owns ordering, transaction boundaries, correlation/causation IDs, effect-intent persistence, leases, retries, bounded logs, and result aggregation. Domain modules own the meaning of facts, projections, policies, and capability decisions. `FlowDefinition` and `Step` must be serializable/describable enough to evolve into a persisted DAG later, but the first implementation is an in-process typed pipeline.

### Step output contract

Every step returns a typed result envelope rather than performing infrastructure work directly:

```ts
type StepResult = {
  facts: DomainFact[];
  projections: ProjectionChange[];
  effects: EffectIntent[];
  presentation: PresentationEvent[];
};
```

`facts` and `projections` are committed by the transaction runner. `effects` are persisted and dispatched after commit by registered handlers. `presentation` is request-scoped output for SSE/UI and is not a durable fact. A domain step cannot import Express, SQLite, MTPLX, ComfyUI, h3, or the filesystem.

## Context And Prompt Pipeline

Horizontal modules do not concatenate the final model prompt. Each module emits a structured `ContextFragment` with section, priority, required flag, token budget, provenance, and structured content. A centralized `ContextBudgeter` applies the prompt-selection policy and deterministic degradation order; a `PromptSerializer` converts the selected fragments into model messages; an `LlmPort` performs the provider request and stream parsing.

The existing prompt-optimization task owns the concrete budget, relevance, recency, confidence, and history-window policy. This architecture task owns only the boundaries and data flow, so prompt policy cannot diverge between the old and new context paths.

## Runtime Language Decision

The runtime language is a separate architecture decision from module boundaries. Current evidence does not show Node as the immediate bottleneck: the service is primarily network/I/O bound by MTPLX, ComfyUI, h3, SQLite, and browser SSE. The observed risks are untyped cross-layer coupling, a large JavaScript composition root, mixed responsibilities, and a central worker dispatcher.

Candidate directions:

| Option | Strength | Cost / risk | Current fit |
| --- | --- | --- | --- |
| Node.js, preferably TypeScript | Keeps current HTTP/SSE/browser deployment and provider integrations; fast iteration; good I/O concurrency | Needs explicit types, module boundaries, and worker discipline; CPU-heavy work remains unsuitable in-process | Best near-term default |
| Go service | Strong long-running service model, simple concurrency, static binary, good operational behavior | Large rewrite; less direct alignment with current JS/client and AI experimentation; provider/domain code must be rebuilt | Consider if service/reliability scale becomes primary |
| Python service | Strong AI/data ecosystem and model tooling | Async/SSE and durable concurrency need discipline; deployment/runtime isolation becomes more involved | Consider for dedicated AI/agent workers, not necessarily the control plane |
| Rust service | Strong safety and performance for critical runtimes | Highest rewrite and iteration cost; provider/product behavior is still evolving | Not justified by current evidence |

Recommended policy: keep the control plane as a modular Node service while the domain and infrastructure ports are being established, and make the language replaceable at ports rather than rewriting preemptively. The frontend direction is now Vue 3 + TypeScript + Vite: Vue owns componentized reactive UI, TypeScript owns compile-time contracts, and Vite owns production builds, dynamic-import code splitting, hashed assets, compression, and cacheable output. The active client is rewritten as one complete replacement, with the old client retained only as a reference/rollback artifact; the rewrite must preserve API, SSE, persistence, and user workflow contracts before the new entry replaces it. If future requirements demand CPU-heavy work, large-scale scheduling, or model-specific workers, add a specialized worker behind an existing port instead of moving the whole system first.

The current runtime baseline is fixed to Node.js `22.23.2` across `.nvmrc`, `package.json` engines, Docker images, and CI. This is a compatibility baseline, not a permanent promise that Node must remain the only runtime. Any future runtime replacement or specialized worker must implement the established ports/contracts and include a workload comparison.

### Frontend cutover directory

The new Vue client is built in a separate `web/` project during migration. The old `src/` client remains only as a temporary reference. Vite emits `dist/`, and the final Express/Docker entry serves only `dist/`. Before this task is considered complete, the old `src/` directory, old entry scripts, and any static-server references to them are deleted. This is a migration workspace technique, not a dual-frontend delivery strategy.

The decision should be revisited after a workload profile exists for concurrent 摇光实例, message throughput, job latency, memory footprint, and CPU-bound tasks. A language migration without that profile would trade a visible code-organization problem for a much larger compatibility and deployment problem.

The domain model is recorded so that future architecture work has a stable vocabulary:

```text
摇光系统
  -> 摇光实例
       -> identity core
       -> life world
       -> self-model / current state
       -> relationships
       -> presence / shared scene
```

`摇光（Fluctlight）` is the concept and product direction for a persistent, self-modeling AI personality. It is not the proper name of each created AI. UI copy should use the created AI's own name and the instance term “摇光实例” where a type label is needed; documentation can say that the project is building a 摇光系统. The old `persona` and `companion` terms are deprecated domain vocabulary; existing identifiers are migration details only.

## Self-Awareness Boundary

The project goal is to build AI individuals that behave as if they have a continuing self-model. For this naming task, that is a semantic definition only. The six observable properties are continuity of identity, continuity across time, current-situation awareness, private autobiographical/relationship memory, bounded initiative, and self-reflection about state and limits. No new behavior is added by this task, and public copy must not claim that subjective consciousness has been scientifically established.

## Rename Surface

### First-phase changes

- active browser title, brand button, sidebar wordmark, empty states, accessibility labels, and old product-facing copy;
- README title and product description;
- domain glossary in `CONTEXT.md` and task/spec references that describe the product concept.

### Compatibility-preserved names

- `/api/companion/...` routes;
- `companion_*` SQLite tables and `companion.sqlite`;
- `COMPANION_*` environment variables;
- Docker image/volume identifiers;
- localStorage keys such as `companion-active-persona`;
- static filenames and legacy entry names;
- test hook names and existing payload fields.

Renaming these technical identifiers would require a separate migration with dual-read/dual-write or explicit deployment migration. It is not part of this task.

## Terminology Mapping

| Concept | Canonical term | Current implementation hint | User-facing rule |
| --- | --- | --- | --- |
| Product/runtime | 摇光系统 | Companion Chat app | Use 摇光/Fluctlight when referring to the product |
| Persistent AI individual | 摇光实例 | legacy aggregate | Use the individual’s own name; use 摇光实例 only where a type label is needed |
| Stable identity | identity core | foundation/base prompt | Do not call it the whole Fluctlight |
| Ongoing life context | life world | blueprint/state/events | Use life/continuity language, not generic “profile” |
| User connection | relationship | memories/evolution | Do not use `companion` as the canonical domain term |
| Current joint interaction | presence/shared scene | shared scene/state | Show natural context; do not expose internal state terms by default |
| Legacy implementation terms | deprecated `persona` / `companion` | current code/API identifiers | Keep only as explicit migration-compatibility references |

The table is the approved terminology mapping, not a license to rename API/database symbols. Any terminology conflict discovered during implementation must be resolved in `CONTEXT.md` before changing copy.

## Rollback And Verification

- Reverting the presentation strings and documentation restores the prior display without touching data or API contracts.
- Run the existing test suite and a repository search for accidental changes to `companion` technical identifiers.
- The naming task currently verifies the existing active UI through `src/index.html` and `src/companion-main.js`; the later frontend modernization verifies the new `web/` -> `dist/` entry and deletes the old client.
- Confirm that product copy describes a design goal/behavioral model, not proven machine consciousness.

## Migration Verification And Final Deletion

During construction, the old implementation may be loaded only as a comparison/reference path. Contract fixtures and replay harnesses compare normalized API payloads, SSE envelopes, facts, projections, effect intents, and error categories; unstable IDs, timestamps, provider randomness, and model wording are normalized or excluded. External effects use mock/dry-run handlers so comparison cannot double-submit media, proactive messages, or provider work.

The final deletion gate requires the new backend/frontend entrypoints to run independently, all API/SQLite/worker/browser tests to pass, a repository scan to show no production reference to old facades/hooks/dispatchers/entries, deletion of the old implementation, and one more full test run after deletion. Rollback is a previous build/commit, not a runtime fallback layer.

## Delivery Entry Points

Development uses a Vite dev server with `/api` proxied to the Node/Express control plane. Production builds `web/` into hashed `dist/` assets; Express and Docker serve only `dist/` plus API routes. CI runs TypeScript typecheck, Vite production build, backend tests, temporary-database checks, and browser/API smoke tests against the same new entrypoint. No environment is allowed to silently fall back to the old `src/` client.

## Observability And Recovery

Every flow, step, and effect carries bounded identifiers: `requestId`, `flowId`, `correlationId`, `causationId`, `subjectId`, `stepId`, and `effectId`. Structured logs and debug summaries record flow/step/effect status, facts/projections/effects counts, LLM timing/tool counts, context-budget selection, SSE lifecycle, pagination cursors, and frontend boot/stream/resource timings. Logs are redacted, bounded, and never contain full prompts, API keys, authorization headers, or provider credentials.

Durable jobs/effects remain recoverable from SQLite: after restart, expired leases are reclaimed, retryable failures are rescheduled, terminal failures are surfaced, and the browser reconstructs state from bootstrap/conversation/activity APIs. Rollback uses the previous complete build/commit; the new runtime does not retain the old implementation as an operational fallback.
