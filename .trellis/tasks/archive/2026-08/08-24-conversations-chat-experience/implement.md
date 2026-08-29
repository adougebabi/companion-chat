# T06 Conversation / Chat Implementation Brief

## Status

Parent-authorized implementation brief for the second executable child in the
T05-T12 chain. This child produces implementation evidence only; T12 remains
the acceptance owner.

## Dependency

T05 cognition and diagnostics public contracts/handoff are available in the
current workspace. T06 consumes those interfaces and does not query cognition
tables or repositories.

## Owned Paths

- `apps/core/src/fluctlight_core/conversations/**`
- `apps/core/src/fluctlight_core/transport/conversations.py`
- `apps/core/migrations/versions/0005_t06_conversations.py`
- `apps/core/tests/conversations/**`
- `apps/core/tests/contract/test_t06_*.py`
- `apps/core/tests/architecture/test_t06_*.py`
- `apps/core/src/fluctlight_core/transport/api.py` (T06 route registration only)
- `apps/core/migrations/env.py` (T06 schema import only)
- `packages/core-client/openapi.json`, `packages/core-client/scripts/generate.mjs`,
  `packages/core-client/src/index.ts` (generated Core conversation/turn methods)
- `apps/bff/src/ndjson.ts`, `apps/bff/src/app.ts`,
  `apps/bff/scripts/generate-browser-openapi.mjs`
- `packages/browser-client/openapi.json`, `packages/browser-client/scripts/generate.mjs`,
  `packages/browser-client/src/index.ts`
- `apps/bff/test/conversations.test.ts`, `apps/bff/test/ndjson.test.ts`
- `apps/web/src/**` and `apps/web/test/**` for the new chat surface

## Forbidden Paths

- Frozen legacy `server/**`, `web/**`, `test/**`, `compose.yaml`, and legacy
  root Node entrypoints/dependencies.
- T07-T12 domain modules and their migrations; T06 may expose an attachment
  reference but cannot implement media storage/lifecycle.
- Direct BFF imports of PostgreSQL, Redis, Temporal, Python modules, ORM rows,
  or semantic policy.

## Decisions And Contracts

Implement without changing D002, D005-D008, D013-D016, D021-D023, D031-D033,
and D039. The assigned contracts are `fluctlight-api-contract.md`,
`fluctlight-bff-contract.md`, `fluctlight-auth-contract.md`,
`fluctlight-cognitive-runtime.md`, and `fluctlight-persistence-contract.md`.
Core stream events are `token | action_result | completed | error | heartbeat`;
browser events are translated to `token | message | media | completed | error |
heartbeat`, with one monotonic sequence and one terminal event.

## Implementation Checklist

1. Add typed Actor-aware Conversation/Participant/Message contracts and
   PostgreSQL sequence/read/delivery tables with idempotent append.
2. Add Core application routes for create, history, mark-read and turn; map
   commands through the conversation service and produce strict NDJSON.
3. Extend the generated Core client and OpenAPI artifact together.
4. Add BFF TypeBox routes and an incremental, schema/sequence-validating Core
   NDJSON translator with abort propagation and stable browser error mapping.
5. Regenerate the browser client from the BFF artifact and build a usable Vue
   chat/history/composer surface with loading, attachment-reference,
   cancellation and error states.
6. Add focused contract/architecture/unit checks. T12 reruns real PostgreSQL,
   Core+BFF+browser aggregate, disconnect and security scenarios.

## Implementation Checks

```bash
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline ruff check apps/core/src/fluctlight_core/conversations apps/core/tests/conversations apps/core/tests/contract/test_t06_*.py
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests/conversations apps/core/tests/contract/test_t06_*.py
pnpm --filter @fluctlight/core-client generate
pnpm --filter @fluctlight/core-client typecheck
pnpm --filter @fluctlight/bff generate:browser-client
pnpm --filter @fluctlight/bff typecheck
pnpm --filter @fluctlight/bff test
pnpm --filter @fluctlight/browser-client generate
pnpm --filter @fluctlight/browser-client typecheck
pnpm --filter @fluctlight/web typecheck
pnpm --filter @fluctlight/web test
```

## T12 Coverage IDs

`T06-CON-01` Actor-aware conversation/participant ownership;
`T06-CON-02` ordered cursor history and idempotent append;
`T06-CON-03` read/delivery monotonicity; `T06-API-01` generated Core
OpenAPI/client parity; `T06-API-02` split UTF-8 NDJSON, one terminal and
bounded error; `T06-BFF-01` TypeBox/auth/error mapping;
`T06-BFF-02` browser stream translation and abort; `T06-WEB-01` composer,
history, attachment reference and cancellation behavior.

## Rollback Point

Before T07 starts, revert only T06-owned paths and migration `0005` if the
aggregate stream/client gate cannot be satisfied. Preserve T05 and all prior
unrelated worktree edits.

## Implementation Evidence Handoff

Record changed paths, generated contract artifacts, implementation-check
commands/results, remaining risks, excluded scope, T12 coverage IDs and the
rollback point. State `acceptance_owner=T12` and `acceptance=pending`; no child
PASS, production readiness or cutover is established here.
