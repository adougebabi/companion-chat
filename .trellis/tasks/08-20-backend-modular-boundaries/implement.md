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

## Phase 2: Infrastructure Ports

- Extract SQLite open/pragma/migration handling into `infrastructure/sqlite`.
- Add repositories for conversation, identity, memory, life, presence, activity, effects/jobs and assets.
- Extract `LlmPort` and upstream stream parser from `lmCompletion()`.
- Extract `MediaProviderPort` for ComfyUI/h3 and move provider-specific process/filesystem code behind it.
- Extract effect lease/retry/settlement and a registry-based dispatcher.

## Phase 3: Conversation And Capability Flow

- Implement `ChatTurnFlow` with typed context fragments and `ContextBudgeter`/`PromptSerializer` ports.
- Move native tool-call parsing and marker fallback into `CapabilityTransportAdapter`.
- Implement `CapabilityDispatcher` returning domain commands/effect intents, not direct SQL/jobs.
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
