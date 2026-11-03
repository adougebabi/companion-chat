# Frontend and strict runner checks — 2026-09-22

Executed from the repository root during final integration. No live model or
media generation request was made.

| Command | Exit | Observed result |
| --- | --- | --- |
| `node --test infra/acceptance/run-go-live-provider-smoke.test.mjs infra/acceptance/verify-go-e2e-events.test.mjs` | 0 | 14 passed, 0 failed/skipped |
| `pnpm typecheck` | 0 | core-client/browser-client tsc and web vue-tsc passed |
| `pnpm test` | 0 | browser-client 12 + web 47 passed, 0 failed/skipped |
| `pnpm build` | 0 | Vite production build passed, 2522 modules |

The strict verifier checks missing/zero/skip/fail/BLOCKED cases and the serial
runner lock. These results do not substitute for Tool or real Provider Agent
acceptance. Backend integration remains separately tracked in acceptance-matrix.md.

Final runner expansion: the combined command was rerun and passed 17/17 tests.
After narrowing the result label to its selected scope, the selected Tool
regression was rerun and passed; actual isolated memory_event command also
passed inventory, selected adapter and direct business execution.
