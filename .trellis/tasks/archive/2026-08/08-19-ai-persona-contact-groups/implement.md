# Implementation Plan

1. Update `server.js` migration and persistence helpers.
   - Add the groups table/default seed and persona `group_id` backfill.
   - Add group listing/creation/assignment helpers.
   - Include group data in persona summaries and bootstrap.
   - Default every new persona to the seeded default group.
   - Add the two JSON routes with validation and route-consistent errors.
2. Extend `src/companion-main.js`.
   - Track the selected group and recover it from bootstrap/local storage.
   - Add contacts-page group selection and create-group dialog.
   - Add contact-click assignment dialog while preserving chat navigation.
   - Refresh bootstrap and render after successful writes.
3. Extend `src/companion-style.css` for the compact header controls, group form, and mobile dialog/select layout using existing tokens/classes.
4. Add focused API tests in `test/companion-api.test.mjs` covering default assignment, creation, reassignment, bootstrap payload, and validation failures.
5. Verify with `node --check server.js`, `node --check src/companion-main.js`, `npm test`, and an isolated server/API smoke test; manually inspect desktop/mobile UI behavior.

## Risk And Rollback Points

- Migration 9 is additive and backfills once; if startup fails before the migration ledger insert, SQLite rolls back the whole migration.
- Bootstrap is additive, but contact filtering depends on a valid default group; keep `loadBootstrap()` fallback defensive.
- Contact click now opens a modal before chat; the modal must always provide a direct “进入聊天” action and a cancel path.
