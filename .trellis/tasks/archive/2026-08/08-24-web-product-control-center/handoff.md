# T10 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- Core/BFF/browser generated contracts now include settings/Diagnostics reads
  and clear operations in addition to Conversation/turn routes.
- BFF adds Owner-session diagnostics mapping, Origin-protected clear, stable
  error responses and generated browser OpenAPI paths.
- `apps/web/src/stores/control-center.ts` owns async settings/Diagnostics
  effects; `apps/web/src/App.vue` provides responsive Chat, Actors, Moments,
  Diagnostics and Settings views with keyboard composer/cancel/error/empty
  states, safe text binding and write-only secret input.
- Web boundary tests now cover generated-client ownership and required product
  views; generated client artifacts are committed with their source scripts.

## Implementation Evidence

```text
pnpm --filter @fluctlight/core-client generate && typecheck
passed
pnpm --filter @fluctlight/bff typecheck && test
passed; 10 tests passed
pnpm --filter @fluctlight/browser-client generate && typecheck
passed
pnpm --filter @fluctlight/web test && typecheck && build
passed; 2 tests passed
```

## Produced Contracts / Schema

- Browser diagnostics events preserve event type, severity, correlation and
  already-redacted payload only.
- Settings UI writes provider URL and a write-only secret; no secret response
  is rendered or stored in browser state.
- Responsive tab navigation preserves the active Chat/composer state while
  control-center views load independently through Pinia actions.

## Remaining Risks / Excluded Scope

- Browser acceptance still requires a real authenticated Compose session,
  accessibility/focus/manual mobile viewport run and full capability matrix.
- Actor/contacts view is based on authoritative Conversation participants;
  group-chat UI, backup, media authoring and legacy deletion remain excluded.
- Core/BFF secure-cookie behavior on local HTTP and CSRF token hardening remain
  T12 integration/security checks.

## T12 Coverage

Re-run `T10-WEB-01`, `T10-WEB-02`, `T10-CTL-01`, `T10-CTL-02`, `T10-CTL-03`,
and `T10-API-01` from the child brief.

Rollback point: remove only T10-owned frontend/client/BFF paths before T11 if
the Control Center contract gate cannot be satisfied.
