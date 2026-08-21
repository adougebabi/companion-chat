# Implementation Plan: Vue Client Rewrite

## Current Status (2026-08-21)

The active client has been cut over to `web/` (Vue 3 + TypeScript + Vite) and
the production server now serves `dist/`. The old root `src/` client is
deleted. Contacts, conversations/history, SSE chat, activity, settings,
persona creation/detail mutations, media states and the debug inspector are
implemented against the modular API. Typecheck, production build, server syntax
and the existing Node test suite pass.

Deferred by explicit session scope: real external-provider checks, browser
visual/performance regression, and redundant full low-level test migration.

This is one complete frontend migration. Work is staged internally, but the final task must ship only the new Vue/Vite client and must include all existing workflows. Visual system redesign is explicitly deferred.

## Phase 0: Inventory And Contract Fixtures

- Inventory every active route/view and interaction in the pre-cutover
  `src/companion-main.js` (historical reference; now deleted).
- Record API DTOs, SSE frames, loading/error/empty states, media states, dialog flows and accessibility labels.
- Add typed contract fixtures for bootstrap, persona detail/create, conversations, cursor pages, chat SSE, activities, settings, media and debug inspector.
- Define browser behavior fixtures for contacts-first boot, IME composition, draft preservation, scroll anchor and stream reconciliation.
- Load the native/backend handoff fixtures and record that the browser consumes DTO/SSE results without parsing tool JSON or recreating capability side effects.

## Phase 1: Vite/Vue Foundation

- Create `web/`, `web/vite.config.ts`, `web/tsconfig.json`, Vue entry, Pinia stores and typed API client.
- Add Vite `/api` proxy for development and a production build manifest.
- Put the static shell, contacts skeleton and loading/error states in `web/index.html`/`App.vue`.
- Keep existing CSS tokens/layout as migration baseline; do not redesign visual language.

## Phase 2: App Shell And Contacts

- Migrate shell/navigation, contacts, groups, active instance selection and empty creation state.
- Make contacts the unconditional first view after bootstrap.
- Preserve localStorage compatibility for the existing active-instance/group keys.
- Add responsive/mobile overlay behavior and accessibility focus management.

## Phase 3: Conversation And History

- Implement `conversations` store and `useMessageHistory` with `limit=20`, cursor, top sentinel, retry, boundary and anchor restoration.
- Merge head pages/tail updates by message ID.
- Implement chat message/media components without replacing the composer.
- Verify empty histories, deleted active instance, long messages and late media dimensions.

## Phase 4: Composer And SSE

- Implement `useComposer` with draft/selection/IME state and explicit-submit clearing.
- Implement `useChatStream` with token frame coalescing and final `done.messages` reconcile.
- Preserve `done.messages=[]`, `done.message`, error behavior and deferred chat.
- Add mobile keyboard, composition, scroll and send-failure browser fixtures.

## Phase 5: Activity, Settings, Persona, Media, Inspector

- Migrate activity cursor pages, comments, reactions, hide/read behavior and media placeholders.
- Migrate settings, provider configuration, persona creation/interview/detail/revision flows.
- Migrate media state, simplified media mode and debug inspector with safe DTOs.
- Use route-level dynamic imports for settings/inspector only after core chat is stable.

## Phase 6: Performance And Delivery

- Add lazy images, video activation, aspect-ratio reservation and batched activity consumption.
- Add boot/stream/history/resource performance measurements.
- Configure cache headers for hashed assets, HTML and API.
- Update Docker and CI to build/serve `dist/`; verify Vite dev proxy.

## Phase 7: Cutover And Deletion (completed)

- [x] Run the available normalized UI/API contract checks while external effects are mocked.
- [x] Switch Express static root and package scripts to the new app.
- [x] Delete old `src/`, old client scripts/styles, old server static references and old checks.
- [x] Run typecheck, build, server syntax checks and backend tests after deletion.
- [ ] Browser/API smoke and mobile visual/performance regression remain deferred for this session.

## Validation Commands

```bash
npm run typecheck
npm run build
npm test
DATA_DIR=$(mktemp -d) DATABASE_PATH="$DATA_DIR/companion.sqlite" node --test test/*.test.mjs
```

Browser verification must use the Vite dev proxy during development and the Express-served `dist/` build for production-like checks. Test contacts-first boot, history paging, IME composition, send failure, stream output, activity, settings, persona creation, media and inspector flows on desktop and narrow mobile viewports.

## Risk And Rollback

- Keep old assets outside the new `web/` source tree during migration; do not import them into production. This migration is now complete and the old assets have been deleted.
- Preserve API/SSE fields before changing presentation components.
- Do not clear or refocus the composer from stores or polling.
- The old `src/` directory was deleted only after the new typecheck/build and available contract checks passed.
- Rollback uses the previous complete build/commit; after deletion, fix behavior in the new client rather than restoring a permanent dual entry.

## Completion Gate

- [x] New client covers every old active workflow.
- [x] `web/` builds to `dist/`, Express/Docker/CI use only the new entry.
- [x] No production reference to old `src/` remains.
- [x] Typecheck, build, backend tests and server syntax checks pass after deletion.
- [ ] Browser/API smoke and mobile visual/performance checks are deferred by explicit session scope.
