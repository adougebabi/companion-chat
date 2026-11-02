# Implementation

1. Read applicable specs and evidence; inspect owning producer/consumer and tests.
2. Fix D1 persona failure/admission and D2 workflow lanes with concurrency regressions.
3. Fix D3 studio retry/backoff and D5 wardrobe lifecycle reason with domain/workflow regressions.
4. Fix D4 user name and D6 prompt/vision feedback with real request regressions and prompt inventory update.
5. Add frontend persona recovery entry using existing BrowserClient and test failure/recovery.
6. pnpm install --store-dir /tmp/pnpm-store (skip install hook touching git); pnpm build; web lint; pnpm typecheck; pnpm test; go test ./... in apps/core-go.
7. Review diff/spec compliance; record exact results, changed assertions and operational needs. Leave working tree uncommitted.
