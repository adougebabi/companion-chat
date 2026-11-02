# R23 UI repair — verification report

All eleven requested items are implemented. Changes remain in the working tree; no Git commit, staging, image additions, accept-tree writes or env-file writes.

## Per item

All test names below belong to `apps/web/test/r23-ui.test.mjs` (13 tests). Production geometry is verified by `verification/verify-r23-ui.mjs`.

### P1-3: Chat loading cancellation and recovery

Files: `apps/web/src/state/conversation.ts`

Tests:

- `P1-3 cancelled selection releases loading and obsolete completion cannot clear a newer role read`
- `P1-3 leaving a pending chat then failing preflight keeps retry usable and empty state waits for successful reading`

### P1-4: Nonduplicated diagnostic metadata, readable 12px/28px status badges

Files: `apps/web/src/react-app/shared/DiagnosticsWorkspace.jsx`; `apps/web/src/styles/replica-diagnostics.css`

Tests:

- `P1-4 diagnostic task row speaks its title once and retains time and status`

### P2-1: Label left, standard shadcn Switch right, preserved disabled time draft

Files: `apps/web/src/react-app/views/ManagementPage.jsx`; `apps/web/src/styles/replica-management.css`

Tests:

- `P2-1 quiet hours uses a labelled shadcn Switch row and preserves times while disabled`

### P2-2: Full character trigger, start-aligned Radix menu with trigger width

Files: `apps/web/src/react-app/shared/WorkspaceNavigation.tsx`; `apps/web/src/styles/workspace-replica.css`

Tests:

- `P2-2 character menu uses trigger width and selecting a role invokes the actual callback`

### P2-3: Single whole-diary empty; search guidance retained beside existing selected text

Files: `apps/web/src/react-app/pages/DiaryPage.jsx`

Tests:

- `P2-3 empty diary shows a single leaf empty state and keeps filtered empty guidance`

### P2-4: Transparent demo leaf empty; compact aside cards remove desktop overflow

Files: `apps/web/src/react-app/pages/InboxPage.jsx`; `apps/web/src/styles/replica-management.css`

Tests:

- `P2-4 empty inbox uses the demo leaf and unread action restores the all tab`

### P2-5: Chinese eyebrow and 16px gap below world cards before reread/pagination buttons

Files: `apps/web/src/react-app/views/CharacterWizard.jsx`; `apps/web/src/styles/replica-creation.css`

Tests:

- `P2-5 creation step one has a Chinese header and a separately spaced working world reread action`

### P2-6: Shared leaf for Studio/relationship empties; existing people-directory leaf retained

Files: `apps/web/src/react-app/views/StudioDirectory.jsx`; `apps/web/src/react-app/shared/AboutYouPanel.jsx`

Tests:

- `P2-6 studio empty state uses shared leaf and its create action remains usable`
- `P2-6 relations directory retains the shared leaf when no world people exist`

### P2-7: White 10px/12px captions on dark scene gradient

Files: `apps/web/src/styles/replica-chat.css`

Tests:

- `P2-7 chat moment retains readable caption text in the scene overlay`

### P2-8: One import drop border with working file selection/drag-drop

Files: `apps/web/src/react-app/shared/FilePicker.tsx`; `apps/web/src/react-app/shared/DomainPrimitives.tsx`

Tests:

- `P2-8 unframed file picker keeps selection behavior and adds no nested drop border`

### P2-10: shadcn Skeleton during lazy chunks and initial page reads; retained content during rereads

Files: `apps/web/src/react-app/App.tsx`; `apps/web/src/react-app/shared/PageSkeleton.tsx`; `apps/web/src/react-app/pages/SchedulePage.jsx`; `apps/web/src/react-app/pages/GoalsPage.jsx`; `apps/web/src/react-app/views/WorldPage.jsx`

Tests:

- `P2-10 pending route chunks show shadcn skeletons until the page resolves`

## P1-3 root cause

The conversation controller set `loading=true` during selectFluctlight. ChatPage cleanup invalidated the request by advancing requestEpoch, but invalidateRequest did not release loading. The obsolete selection's finally then refused to clear loading because its epoch no longer matched. If the next openConversation failed in session/directory preflight before selection, nothing could reset the shared flag. ChatPage combined local readLoading with that leaked controller flag, permanently disabling retry.

The fix assigns loading to selectionLoadingEpoch, releases it synchronously on cancellation, and allows only the owning selection to clear it on completion. Old A cannot clear pending B or adopt foreign data. Existing intro/loading predicates are mutually exclusive; regression tests verify that they remain so across delayed reads, cancellation, failed preflight and successful read-only recovery. No timer hides progress, and no backend delay is inferred from screenshots alone.

## Required checks

- `pnpm install --store-dir /tmp/pnpm-store`: PASS (Node 24.12.0, pnpm 11.0.7 from /tmp; the default Corepack home was read-only).
- `pnpm build`: PASS; production fixture guard and React-only graph guard pass (392 modules). Vite reports the existing large-chunk advisory.
- `pnpm --filter @fluctlight/web lint`: PASS.
- `pnpm typecheck`: PASS across both clients and Web.
- `pnpm test`: PASS — 877 tests (834 Web, 40 browser-client, 3 core-client), zero failures.
- `node --test apps/web/test/r23-ui.test.mjs`: PASS — all 13 new regressions after final presentation edits.
- `GOCACHE=/tmp/r23-go-build go -C apps/core-go test ./...`: PASS after updating seven obsolete expectations in six Go test files. No Go production or CI configuration changes.
- `git diff --check` and changed-Go-file gofmt check: PASS.

## Additional full-suite maintenance

The initial Go run exposed seven stale assertions already inconsistent with current production contracts. Updated tests retain strict Actor background validation while checking malformed optional analysis recovery (original data + warning, no adopted authority); request explicit named catalog schema disclosure; include the world event Agent; check the current formatting prerequisite plus separate real evidence gate; inspect React creation's UTF-8 validation rather than deleted Vue; and verify critical queue membership independently of the inserted world-event lane.

Files: `apps/core-go/internal/core/actor_user_background_test.go`, `initialization_contract_test.go`, `no_arg_tool_schema_test.go`, `phase8_contract_matrix_test.go`; `apps/core-go/internal/httpapi/server_test.go`; `apps/core-go/internal/workflow/workflow_test.go`.

## Browser evidence and limits

Production dist is exercised with owned isolated HTTP fixtures in headless Chrome at 1920×1113 and 390×844, plus first-visit chunk/data loading at 1440×1000. Screenshots live only in `/tmp/r23-shots`; the supplied immutable HTML is used for the notification reference. Assertions cover menu geometry, badge font/height, Switch alignment, diary duplication, notification scroll dimensions, creation spacing, leaf empties, caption contrast styles, import border count, original chat read recovery and schedule/goals/world skeletons. See `verification/browser-results.json` for final evidence.

No requested UI item remains unfixed. Relations' main directory already contained the shared leaf in HEAD: it is retained and explicitly regression-tested, with missing relationship-panel leaf variants added. Browser fixtures and environment-gated Go tests do not certify live PostgreSQL/Temporal/model/provider behavior. No prompt, model entry, token limit, backend business behavior or CI configuration changes.
