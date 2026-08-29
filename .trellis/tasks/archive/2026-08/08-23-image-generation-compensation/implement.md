# Implementation Plan

1. Read media flow, production activity projection, job repository, dispatcher, and worker specs; add focused fixtures for activity and source/poll failure windows.
2. Thread the caller transaction through activity media publication and prove rollback/idempotent replay.
3. Add deterministic source-to-poll compensation/atomic follow-up and register it with the media worker.
4. Make quality retry successor identity deterministic and add crash/replay/stale lease tests.
5. Run focused media/worker tests, then full `npm test`, typecheck, build, syntax checks, and diff check.

## Validation

- `node --test test/media-flow.test.mjs test/media-job-service.test.mjs test/media-job-composition.test.mjs`
- `node --test test/job-repository.test.mjs test/job-dispatcher.test.mjs test/companion-api-modular-life-proactive.test.mjs`
- `npm test`
- `npm run typecheck`
- `npm run build`
- `git diff --check`
