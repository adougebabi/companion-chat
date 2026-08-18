# Implementation Plan: Chat interaction, mobile usability, and debug observability

1. Inspect current composer, mobile CSS, `streamPersonaChat`, job records, and lifecycle inspector consumers before changing fields.
2. Add Enter/Shift+Enter behavior and a transient `typing` message shape; test no duplicate send during streaming and refresh recovery.
3. Repair narrow viewport/safe-area composer geometry and validate 320px/375px plus a reduced-height mobile viewport.
4. Remove consumer media buttons; add explicit development-only test-media action to the inspector using the server job contract.
5. Add `commentingActivityId` disclosure/focus/collapse behavior without changing activity persistence.
6. Add redacted persona-scoped debug-context API and inspector renderer behind `COMPANION_DEBUG_INSPECTOR=1`. Reuse existing prompt/job helpers; cap each returned collection at 10 and summaries at 2,000 characters; never return settings secrets.
7. Add focused tests for disabled debug routes, recursive redaction, persona isolation, truncation, test-dispatch behavior, and run manual browser checks for keyboard, dialogs, composer, comments, and mobile nav.

## Validation

```sh
node --check server.js
node --check src/companion-main.js
npm test
git diff --check
```

Manual matrix: desktop, 650px, 375px, 320px, reduced viewport height, Enter/Shift+Enter, send busy state, comment disclosure, debug inspector, and browser console.

## Rollback

The change is additive except removal of consumer media buttons. Keep existing server media job endpoints and original activity/comment persistence, so reverting the browser layer restores the old control placement without data migration.
