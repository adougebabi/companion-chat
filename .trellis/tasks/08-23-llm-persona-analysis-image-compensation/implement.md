# Implementation Plan

1. Complete `08-23-enforce-llm-persona-analysis`: inspect current interview contracts, add strict analyzer port and MTPLX JSON adapter, wire the default runtime, remove repository fallback from the production analysis path, and add route/runtime/provider tests.
2. Run the analyzer child quality gate before starting the media child; verify old interview rows and activation still work.
3. Complete `08-23-image-generation-compensation`: make activity media projection transaction-aware, add atomic or repairable source-to-poll follow-up, make quality successor IDs deterministic, and add compensation/replay tests.
4. Run focused media/worker tests, then full `npm test`, `npm run typecheck`, `npm run build`, syntax checks, and `git diff --check`.
5. Run the final cross-layer review against this parent PRD and update backend specs if the durable job or interview analyzer contract changed.

## Risky Files and Rollback Points

- `server/infrastructure/llm-provider.js` and new analyzer adapter: provider response parsing and timeout boundaries. Stop after adapter tests before runtime wiring.
- `server/application/interview-service.js`, `server/infrastructure/interview-repository.js`, `server/runtime/runtime.js`: analyzer selection and old-session compatibility. Verify no repository fallback before frontend integration.
- `server/infrastructure/production-proactive-ports.js`, `server/application/media-job-service.js`, `server/application/media-flow.js`: transaction and follow-up ordering. Stop after each failure-window test.
- Existing uncommitted changes are user-owned; never reset or overwrite unrelated files.

## Validation Commands

- `npm test`
- `npm run typecheck`
- `npm run build`
- `node --check server/infrastructure/llm-provider.js`
- `node --check server/application/interview-service.js`
- `node --check server/application/media-job-service.js`
- `git diff --check`
