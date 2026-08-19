# Implementation Plan

1. Add migration 10 for interview source/provenance metadata and update the interview persistence/view helpers without changing the legacy interview payload contract.
2. Add bounded natural-language extraction constants and helpers in `server.js`: description validation, strict LLM JSON prompt, allowlist normalization, deterministic defaults, provenance application, and ready-session creation.
3. Register `POST /api/companion/interviews/analyze` before the existing interview routes. Keep provider/model errors as `{error}` responses and ensure failed analysis creates no database row.
4. Make both interview activation paths consume stored inferred-field metadata and preview overrides so the final persisted blueprint retains correct provenance.
5. Export the new extraction/session helpers through `companionTestHooks`.
6. Replace the active wizard in `src/companion-main.js` with a single description form, inline retry errors, and a preview that permits editing generated name/role/core fields. Remove the user-visible question-by-question path while leaving legacy server routes intact.
7. Add minimal `.wizard-error` styling and verify narrow/mobile dialog layout.
8. Extend `test/companion-api.test.mjs` with mocked-model success, defaults/provenance, malformed/unknown/oversized input, provider failure, and route/session no-mutation assertions. Preserve existing adaptive interview coverage.
9. Run quality gates:
   - `npm test`
   - `node --check server.js`
   - `node --check src/companion-main.js`
   - start with temporary `DATA_DIR`, call `/api/health`, and exercise analyze/activate success and failure payloads
   - manually verify the dialog at desktop and narrow/mobile widths, including retry and preview back navigation

## Risky Files and Rollback Points

- `server.js`: migration, provider call, interview state, and activation contract. Stop after migration/helper changes and run tests before route/frontend edits.
- `src/companion-main.js`: active wizard replacement. Keep the existing `api()` and `esc()` owners; do not touch legacy `src/main.js`.
- `test/companion-api.test.mjs`: test fixture fetch restoration must run in `finally` so later model tests remain isolated.
