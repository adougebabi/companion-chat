# T10 Web Product / Control Center Implementation Brief

## Status

Parent-authorized implementation brief for the sixth executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T09 Moments/Media public contracts and handoff are available. T10 consumes
generated browser/BFF contracts and does not query Python domain tables.

## Owned Paths

- `apps/web/src/**`, `apps/web/test/**`
- `apps/bff/src/app.ts`, `apps/bff/scripts/generate-browser-openapi.mjs`,
  `apps/bff/test/control-center.test.ts`
- `packages/browser-client/openapi.json`, `packages/browser-client/scripts/generate.mjs`,
  `packages/browser-client/src/index.ts`
- `packages/core-client/openapi.json`, `packages/core-client/scripts/generate.mjs`,
  `packages/core-client/src/index.ts` for diagnostics/settings reads only

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**` and root legacy runtime files.
- T11/T12 operations/cutover/deletion; no backup scripts or old-system removal.
- Browser-side Provider/storage/workflow calls, hidden reasoning/secrets,
  direct Core internal routes, or fake domain rows for future-only group chat.

## Decisions And Contracts

Implement without changing D002, D005-D006, D022-D024, D028-D029, D031-D033,
and D039. The assigned contracts are `fluctlight-bff-contract.md`,
`fluctlight-auth-contract.md`, `fluctlight-diagnostics-contract.md`,
`fluctlight-configuration-contract.md`, and the frontend structure/state/
component guides. Browser data uses generated client methods; Diagnostics is
Owner-only and displays only redacted summaries.

## Implementation Checklist

1. Extend generated Core/BFF/browser contracts for settings and Diagnostics.
2. Add Control Center routes with stable errors, session/auth boundaries and
   no direct domain/storage imports.
3. Build responsive Vue views for Chat, Actors/contacts, Moments state,
   Diagnostics correlation and write-only Settings; keep async effects in
   Pinia/composables and all external text in Vue bindings.
4. Add keyboard/focus/empty/error/loading/cancel states and narrow viewport
   layout checks; no group-chat or placeholder-only positive acceptance.
5. Run client typecheck/test/build and BFF checks; T12 owns browser/a11y and
   full capability matrix.

## Implementation Checks

```bash
pnpm --filter @fluctlight/core-client generate
pnpm --filter @fluctlight/core-client typecheck
pnpm --filter @fluctlight/bff generate:browser-client
pnpm --filter @fluctlight/bff typecheck
pnpm --filter @fluctlight/bff test
pnpm --filter @fluctlight/browser-client generate
pnpm --filter @fluctlight/browser-client typecheck
pnpm --filter @fluctlight/web typecheck
pnpm --filter @fluctlight/web test
pnpm --filter @fluctlight/web build
```

## T12 Coverage IDs

`T10-WEB-01` responsive navigation and Chat/Actors/Moments views;
`T10-WEB-02` composer/history/cancel/attachment states; `T10-CTL-01`
Owner-only Diagnostics correlation/redaction/export/clear;
`T10-CTL-02` write-only Settings and stable error states; `T10-CTL-03`
keyboard/focus/empty/loading/error accessibility; `T10-API-01` generated
browser no-drift and BFF session/error contract.

## Rollback Point

Before T11 starts, revert only T10-owned frontend/client/BFF paths if the
Control Center contract gate cannot be satisfied. Preserve T05-T09 and prior
unrelated edits.

## Implementation Evidence Handoff

Record changed paths, generated contract artifacts, implementation-check
commands/results, remaining browser/diagnostics risks, excluded scope, T12
coverage IDs and rollback point. State `acceptance_owner=T12` and
`acceptance=pending`; no child PASS, production readiness or cutover is
established here.
