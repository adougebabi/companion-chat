# Implementation Plan: Native Tool Calls With Marker Compatibility

This task is an additive migration inside the existing Node/SQLite chat service. Keep the application SSE envelope and marker compatibility while replacing direct per-capability branches with one registry-backed dispatcher.

## Phase 0: Freeze Baseline And Fixtures

- Capture the current `scene_event`, `media_event`, `pending-event` marker, `token`, `done.messages`, and `done.message` behavior in sanitized fixtures.
- Add fixture helpers for MTPLX non-streaming calls, fragmented arguments, `reasoning_content`, `[DONE]`, malformed payloads, missing index, unknown tools, and provider disconnects.
- Record current durable row shapes and normalized replay output without changing existing tables or API paths.

## Phase 1: Registry And Accumulator

- Define the shared `CapabilityCall` envelope and registry entry interface.
- Move `sceneEventTool` and `mediaEventTool` schemas behind registry entries and define the native `pending_event` schema from `normalizePendingEventCall()`.
- Harden streamed accumulation: index/id keying, first-nonempty metadata, argument concatenation, `finishReason`, `doneSeen`, bounded `parseErrors`, and incomplete-call state.
- Preserve `reasoning_content` redaction and emit only visible `delta.content` as `token` events.

## Phase 2: Dispatcher, Ordering, And Idempotency

- Dispatch complete native calls in provider index order with per-capability cardinality limits.
- Add deterministic idempotency keys and persist bounded provenance in existing event/job/pending payload JSON.
- Make scene replay return the existing event result; make media count creation atomic; make pending rows repair a missing job without duplicating the row.
- Ensure invalid native calls block their matching marker fallback and do not become visible text.

## Phase 3: Native Pending Event And Marker Adapters

- Implement the pending-event registry handler and native continuation result.
- Adapt existing media and pending marker parsers to the same `CapabilityCall` envelope without broadening their schema or adding keyword inference.
- Implement the per-capability native-first matrix, including unknown-tool fail-closed behavior and native scene plus marker media coexistence.
- Update the system capability prompt and shared-scene/media contract documentation so the advertised tools and fallback language match runtime behavior.

## Phase 4: Continuation, Disconnect, And Provider Failure

- Build one standard tool-result continuation with `tool_choice: 'none'` and reject a second tool call from the follow-up.
- Preserve committed effects when continuation fails and return capability-specific bounded fallback text through `done`.
- Track missing `[DONE]`, malformed upstream payloads, reader errors, and browser disconnects; abort upstream work where supported and never persist partial assistant text.
- Keep `done.messages` authoritative and `done.message` as the compatibility alias.

## Phase 5: Integration And Regression Coverage

- Add unit tests for accumulator edge cases, registry validation, native/marker precedence, order, dedupe, and capability error matrices.
- Add `/api/companion/chat` SSE tests for media/pending persistence, continuation, malformed/unknown tools, provider failures, missing `[DONE]`, and disconnect cleanup.
- Replay the real MTPLX fixture when the configured endpoint is available; keep deterministic mock fixtures as the required gate.
- Run temporary-`DATA_DIR` health/API/browser smoke checks and verify no raw tool JSON reaches visible messages or token events.

## Phase 6: Cutover Gate

- Remove direct route branching and duplicate dispatchers only after normalized replay matches the baseline for existing scene/media/marker paths.
- Keep marker parsers as explicit compatibility adapters; do not remove them or rename existing persistence/API identifiers.
- Run the full test suite and repository scans after cutover.

## Validation Commands

```bash
node --check server.js
npm test
python3 ./.trellis/scripts/task.py validate .trellis/tasks/08-20-native-tool-call-migration
git diff --check
```

For integration checks, run the service with a temporary `DATA_DIR`, exercise `/api/health` and `/api/companion/chat`, and use sanitized SSE fixtures for provider success, malformed chunks, continuation failure, and disconnect. Do not run against the checked-out `data/` database or expose provider credentials in fixtures.

## Review Gate Before Activation

- `prd.md`, `design.md`, and `implement.md` agree on the single registry/dispatcher, native-first fallback matrix, idempotency strategy, continuation limit, and failure semantics.
- `implement.jsonl` and `check.jsonl` contain real spec/research entries.
- Existing `scene_event`, `media_event`, and marker tests remain green before the first implementation commit.
- No API/SSE event names, database table names, environment variables, or static compatibility identifiers are renamed.
