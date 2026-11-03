# Technical Design

## Data Model And Migration

- Add migration `9` in `server.js` with a normalized `companion_groups` table (`id`, unique `name`, `is_default`, timestamps).
- Insert one immutable default group during migration. Existing personas are assigned to it while adding a nullable-compatible `group_id` foreign key column to `companion_personas`; new persona creation always writes the default group ID explicitly.
- Add an index on `companion_personas(group_id, created_at)` and keep the default group identified by `is_default = 1`, not by a hard-coded ID.

## Server Contracts

- `listGroups()` returns `{id, name, isDefault, personaCount}` ordered with the default group first, then creation order.
- `GET /api/companion/bootstrap` adds `groups` alongside the existing `settings`, `personas`, and activity fields. Persona summaries add `groupId` and `groupName`.
- `POST /api/companion/groups` accepts `{name}` after trimming and validates a nonempty bounded name; duplicate names return the normal route error.
- `PUT /api/companion/personas/:personaId/group` accepts `{groupId}`, validates both resources, updates the persona timestamp and returns the updated summary.
- All writes use the existing SQLite transaction and `route()` error conventions. Unknown personas/groups return 404-style errors without mutation.

## Browser State And Flow

- Extend `appState` with `groups` from bootstrap and persist only the selected contact group ID in `localStorage` under `companion-active-group`.
- `loadBootstrap()` keeps the saved group only when it still exists; otherwise it selects the default group returned by the server.
- `renderContacts()` renders a native group `<select>` and adjacent create button in the contacts header, filters `appState.personas` by the selected group, and keeps the existing empty state when a group has no members.
- The create button opens the existing persona dialog with a small native form. On success it reloads bootstrap, selects the new group, and rerenders the contacts page.
- Clicking a contact opens the same dialog with the persona name and a group `<select>`. “进入聊天” preserves current behavior; when the selected group differs, the client calls the assignment endpoint before entering chat. Cancel leaves both group and view unchanged.
- Existing sidebar persona rendering remains unfiltered so the quick chat list and active persona behavior do not regress.

## Compatibility And Failure Handling

- Existing bootstrap consumers ignore the additive `groups` field and additive persona fields.
- Existing personas are backfilled to the default group in one startup migration; no legacy `state.json` writes are introduced.
- Failed create/assign requests keep the dialog open and show the existing alert/error behavior; no optimistic group mutation is applied.
- The default group is never deleted or renamed in this version, so no migration path for orphaned personas is needed.

## Verification Shape

- Add server tests for migration/default group, persona creation assignment, group creation validation, group assignment persistence, bootstrap shape, and unknown-resource failures.
- Run syntax checks, `npm test`, and an isolated temporary-data server smoke test for bootstrap, group creation, assignment, and reload persistence.
- Manually exercise desktop and narrow layouts, including the group selector, empty group, create dialog, contact assignment dialog, cancel path, and chat entry.
