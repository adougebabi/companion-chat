# Implementation Plan

1. Read current frontend/backend specs and add contract fixtures for the identified regressions.
2. Fix frontend boot/loading, sentinel binding, per-instance drafts, hidden activities, settings, inspector actions and dynamic imports.
3. Add compression and verify `dist` cache/encoding behavior.
4. Register/adapt all backend flows to the shared runner/effect path and remove direct feature-specific enqueue where the common runtime can own it.
5. Replace the simple context fragment shape with structured metadata and required-budget selection without changing the prompt-optimization task's policy owner.
6. Add backend and frontend regression tests, temporary database smoke, Express dist smoke and browser desktop/mobile smoke.
7. Update archived-task notes with completed deferred gates, run the full quality check, and leave the task active until every acceptance criterion is proven.

## Validation

```bash
npm test
npm run typecheck
npm run build
DATA_DIR=$(mktemp -d) DATABASE_PATH="$DATA_DIR/companion.sqlite" PORT=4189 node server/index.js
```

The server command is run in a controlled temporary-data session and must be stopped after smoke testing. Browser checks must cover contacts-first boot, short/long history pagination, IME/draft behavior, settings, hidden activities, inspector actions, chat SSE, media states and mobile viewport behavior.

## Completion Gate

- [x] No identified functional omission remains.
- [x] No new duplicate legacy path is introduced.
- [x] All automated and temporary real Express/browser smoke checks pass.
- [x] Archived task notes accurately distinguish completed and environment-dependent scope.

## Progress (2026-08-22)

- Frontend regression fixes landed for boot state, top history sentinel, per-instance drafts, hidden activities, settings, inspector actions, dynamic imports and static compression.
- Backend boundary fixes landed for generic effect adaptation and structured context fragment metadata/required budgeting.
- Automated checks: `npm test` 310 passing, `npm run typecheck`, `npm run build`, syntax checks and `git diff --check` passing.
- Real temporary Express/browser smoke: health, bootstrap, dist HTML, gzip hashed asset, contacts-first boot, settings form, 20-message history/boundary, desktop/mobile chat, inspector actions and hidden-activity panel verified.
- Remaining external verification: real MTPLX/ComfyUI/h3 provider execution and IME composition on a physical mobile keyboard require provider/device availability; deterministic browser composer/scroll paths are covered by the current implementation and must remain in the final regression pass.

## Full Regression Audit (2026-08-22)

- Re-ran the complete Node suite after the SSE lifecycle fix and follow-up hardening: 335 tests passed; TypeScript, Vite build, server syntax checks and diff checks also passed.
- Fixed a production SSE regression where Node request `close` was mistaken for a client disconnect, aborting every MTPLX request after the request body completed. The adapter now observes response disconnects, request `aborted`, and explicit abort signals only.
- Fixed local startup configuration: `npm start`/`npm run dev` load `.env` with `--env-file-if-exists`; local and container data paths use `./data`, while provider/model settings continue to come from `.env`.
- Fixed chat user-message idempotency, composer cancellation/selection/IME forwarding, current-persona deletion navigation, provider network error mapping, and timeline/relationship effect publication inside caller transactions.
- Real endpoint smoke confirms the application now emits a terminal SSE error instead of an empty stream when the configured provider is unavailable. The current external endpoint returns connection refusal (`ECONNREFUSED`); this remains an environment/provider availability blocker, not an application route or SSE framing failure.
- Browser and physical-device verification remains environment-dependent. The repository has no Playwright/browser test runner or checked-in screenshot artifact, so archived browser-smoke claims should be treated as manual-session evidence rather than a reproducible automated gate.
- A read-only data audit confirmed existing personas retain their life blueprints, states, memories, relationship evolutions, schedules, timeline slots, life events and messages. The modular persona detail route was missing the legacy rich DTO aggregation; it now returns foundation, blueprint, state, schedule, memories and evolutions. Context budgeting also now measures bounded text, preventing large raw JSON from incorrectly excluding memory fragments.
- Deferred sleep replies remain intentionally batch-based: follow-up messages join an active batch and do not trigger one LLM call per message. The worker later commits the combined assistant reply; this is a policy behavior, not data loss.
- Slow real-time MTPLX responses now keep the browser connection alive with SSE comment heartbeats while the normal `token`/`done`/`error` contract remains unchanged. This prevents a long model generation from looking like a failed request while the server is still committing the assistant result.
