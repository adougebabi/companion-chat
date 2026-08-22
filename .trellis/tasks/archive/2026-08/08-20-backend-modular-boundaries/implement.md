# Implementation Plan: Backend Two-Dimensional Migration

The work is staged for verification but belongs to one final migration. Temporary compatibility code is allowed only until the final deletion gate.

## Phase 0: Freeze Contracts And Baselines

- Inventory every route, SSE event, DTO field, migration, job type, provider capability and test hook.
- Add contract fixtures for bootstrap, persona lifecycle, conversations, activities, settings, media, debug routes and errors.
- Add normalized replay helpers that ignore generated IDs, timestamps and model wording.
- Record current request/response, job and provider timing baselines.

## Phase 1: Composition And Contracts

- Create `server/index.js` as the future composition root without changing public behavior.
- Add contracts for ports, context fragments, flow steps, facts, projections, effects, presentation events and SSE.
- Add the generic flow registry/runner with a mock effect executor.
- Add correlation/causation IDs and bounded redacted logging.
- Keep the old root runnable only as a temporary comparison path.
- Freeze the native task's `CapabilityCall`/registry contract in a handoff fixture; do not reimplement its accumulator.

## Phase 2: Infrastructure Ports

- Extract SQLite open/pragma/migration handling into `infrastructure/sqlite`.
- Add repositories for conversation, identity, memory, life, presence, activity, effects/jobs and assets.
- Extract `LlmPort` and upstream stream parser from `lmCompletion()`.
- Extract `MediaProviderPort` for ComfyUI/h3 and move provider-specific process/filesystem code behind it.
- Extract effect lease/retry/settlement and a registry-based dispatcher.

## Phase 3: Conversation And Capability Flow

- Implement `ChatTurnFlow` with typed context fragments and `ContextBudgeter`/`PromptSerializer` ports.
- Consume the native task's `CapabilityCall` output through `CapabilityTransportAdapter`; do not create a second parser/dispatcher.
- Implement the backend `CapabilityDispatcher` flow step as a consumer that returns `CapabilityResult`/`EffectIntent`; generic runner owns SQL/jobs and settlement.
- Preserve `token/done/error`, `done.messages`, `done.message`, deferred-chat empty messages and tool continuation semantics.
- Add native MTPLX fixtures, malformed-call fixtures, disconnect tests and dry-run effect tests.

## Phase 4: Life, Presence And Relationship Flows

- Extract pure `LifeStateResolver` and tests for schedule/event/daily-plan precedence.
- Implement `RecordLifeEventFlow`, `SceneEventFlow`, schedule flows and recovery flows.
- Move relationship evidence and evolution into typed flows; comments/events submit evidence rather than writing memory/relationship tables directly.
- Remove `createEvent()`'s cross-domain option union.

## Phase 5: Memory, Activity And Media Flows

- Implement `ContextFragment` producers and memory selection port.
- Split activity publication, comments/reactions/visibility and proactive delivery into separate flows.
- Implement `MediaFlow`, provider effects, acceptance, retry and settlement through generic effect handlers.
- Remove `runMediaJob()`'s feature-specific branch and old provider implementations.

## Phase 6: Routes, Workers And Test Migration

- Move routes to `server/http`; routes only validate input, invoke a flow, and map DTO/status.
- Move workers/timers to `server/runtime`; runtime only claims effects and invokes registered handlers.
- Replace direct internal test-hook imports with port/use-case tests while retaining API contract tests.
- Migrate debug routes to safe DTOs and preserve the explicit debug flag.

## Phase 7: Replay, Cutover And Deletion

- Run old/new normalized replay with all external effects in dry-run mode.
- Verify native and backend handoff fixtures have one dispatcher and one idempotency/provenance contract.
- Run temporary-database API, SSE, worker, restart, lease, provider-failure and browser smoke tests.
- Switch package scripts, Docker, CI and health checks to `server/index.js`.
- Delete old `server.js`, `companionTestHooks`, duplicate dispatchers, duplicate providers and old route implementations.
- Run repository import scan and the full test suite after deletion.

## Validation Commands

```bash
npm test
node --check server/index.js
node --test test/contracts/*.test.mjs test/application/*.test.mjs test/domain/*.test.mjs
DATA_DIR=$(mktemp -d) DATABASE_PATH="$DATA_DIR/companion.sqlite" node --test test/*.test.mjs
```

Provider tests use mock/dry-run adapters. Real MTPLX capability probing remains the explicit research demo and is not part of normal CI.

## Risk And Rollback

- Keep public DTO/SSE/schema fixtures green after every phase.
- Never perform external calls inside a fact/projection transaction.
- Never delete the old root until the new root runs independently and the replay suite is green.
- If cutover fails, deploy the previous complete build/commit; do not restore a partial facade.
- After deletion, any missing behavior is a new fix in the new architecture, not a reason to resurrect the old monolith.

## Completion Gate

- No production import references the old root, hooks, dispatcher or provider paths.
- No horizontal module directly imports Express, SQLite or a provider.
- All flows use the generic runner and effect registry.
- Full API/SSE/SQLite/worker/browser/replay suite passes after old-layer deletion.

## Deep Parity Closeout (2026-08-21)

- `createLifeEventFlow` now owns generic event validation, idempotency, automatic safety policy, scene-reference checks, mild-setback recovery requirements, participant ownership/introduction, shared-scene projection protection, activity/supporting-character fan-out, frozen media authorization, proactive/activity effects, and one transaction boundary.
- The default runtime now composes timeline decision/slot/link persistence, supporting-character persistence, centralized context fragments and prompt contracts, model-message serialization (including tool continuation), life-state reconciliation through the event flow, and proactive/deferred/activity/debug context through the same context reader.
- Job enqueueing reuses an active SQLite transaction so fact, projection, activity and effect rows cannot fail on nested `better-sqlite3` transactions. Media activity jobs carry the frozen `personaMediaConcept` at the top-level worker envelope and preserve activity ownership.
- Validation in this implementation phase was limited to `node --check`, module-import smoke, and `git diff --check`; test migration, test repair, and regression execution remain intentionally deferred until the unified replay/regression phase.

## Scope Adjustment (2026-08-21)

Per the latest user direction, this continuation explicitly excludes old/new normalized replay, real MTPLX probing, real ComfyUI/H3 process verification, provider performance/failure-recovery checks, lease/retry/restart recovery checks, log-format/redaction verification, and frontend visual/performance regression. API/SSE/SQLite/worker regression is now being run after the legacy root cutover. The old `server.js` root and direct legacy test imports have been removed; remaining failures are recorded as regression work rather than hidden by restoring compatibility code.

The modular hardening pass also completed route/debug fail-closed wiring, shared provider-message serialization with tool-call correlation, timeline stale-slot safety, life-state recovery idempotency, strict activity-media association checks, and duplicate job-registration detection. The boundary audit now records these as `partial` legacy cutover boundaries rather than `blocked` missing modular implementations; `readyForLegacyDeletion` remains false until the old root, hooks, and duplicate paths are removed.

Legacy cutover is now applied: `server.js`, direct `server.js`/`companionTestHooks` imports, the compatibility harness, legacy fixtures, and the legacy boundary inventory/test were removed. `server/index.js` is the only package start/dev entrypoint. The deleted compatibility suite is no longer part of the regression surface; remaining tests exercise the modular contracts, applications, runtime, providers, workers, and routes directly.

## Follow-up Closure (2026-08-22)

The migration follow-up added a shared flow-effect adapter for life/timeline/relationship/pending/media effects, structured context-fragment metadata with required-section budgeting, and regression coverage. Automated verification now reports 310 passing tests, typecheck/build/syntax success, and temporary Express/API/browser smoke. Real external-provider execution remains an environment-dependent follow-up rather than an unverified claim.
