# T06 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`; `acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/conversations/` typed contracts, PostgreSQL
  schema and application service for Actor-aware conversations, participants,
  ordered messages, read/delivery positions and turn responder ports.
- `apps/core/src/fluctlight_core/transport/conversations.py` Core request
  models, page projection and strict `application/x-ndjson` turn producer.
- Core route composition, migration `0005_t06_conversations`, generated Core
  client/OpenAPI and T06 Python contract/architecture tests.
- `apps/bff/src/ndjson.ts` incremental Core-to-browser translator with hidden
  payload rejection, sequence/terminal validation and abort handling.
- BFF conversation routes, generated browser OpenAPI/client, NDJSON and route
  tests.
- `apps/web/src/stores/conversations.ts`, `apps/web/src/App.vue` and updated
  boundary test for the usable chat/history/composer/cancel surface.

## Implementation Evidence

```text
UV_CACHE_DIR=/tmp/fluctlight-uv-cache uv run --offline pytest -q apps/core/tests
86 passed
pnpm --filter @fluctlight/core-client generate && typecheck
passed
pnpm --filter @fluctlight/bff typecheck
passed
pnpm --filter @fluctlight/bff test
9 passed
pnpm --filter @fluctlight/browser-client generate && typecheck
passed
pnpm --filter @fluctlight/web test && typecheck && build
passed
```

## Produced Contracts / Schema

- Alembic head `0005_t06_conversations` with conversation, participant,
  message, sequence head and read-position tables.
- Core generated client methods for create/history/read/turn and a strict
  Core NDJSON stream.
- Browser generated client methods and `token | message | media | completed |
  error | heartbeat` translation boundary.

## Remaining Risks / Excluded Scope

- The default Core `ConversationService` has no Provider responder configured;
  it persists the user message and returns an explicit bounded stream error
  until a later Provider/cognition integration supplies the public responder.
- Real PostgreSQL foreign-key behavior, Core+BFF disconnect, CSRF policy,
  browser accessibility and cross-module cognition delivery remain T12-only.
- Attachment values are references only. T09 owns media authorization/storage
  and lifecycle. Group-chat UI, multi-Human accounts and legacy compatibility
  remain excluded.

## T12 Coverage

Re-run `T06-CON-01`, `T06-CON-02`, `T06-CON-03`, `T06-API-01`, `T06-API-02`,
`T06-BFF-01`, `T06-BFF-02`, and `T06-WEB-01` from the child brief.

Rollback point: remove only T06-owned paths and migration `0005` before T07 if
the aggregate stream/client gate cannot be satisfied; preserve T05 and prior
unrelated changes.
