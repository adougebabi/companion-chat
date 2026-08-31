# Go Core runtime stability migration

## Goal

Begin the Core migration with a real Go-owned transport/application slice while
keeping unmigrated cognition/media behavior behind an explicit Python
compatibility boundary. In the same stage, remove the four regressions found by
the previous real Docker acceptance run: compound-effect partial settlement,
missing daily-review registration, malformed reflection proposals, and Worker
restart/backlog replay. The public Go BFF remains the only browser boundary.

## Requirements

- Freeze the current live Python Core contract before implementing Go Core; the
  committed Core OpenAPI artifact must include retire, current password rules,
  and the complete activation payload.
- Go Core must own a real, persisted vertical slice (foundation/auth,
  Fluctlight lifecycle, and conversation target/message transport) and expose a
  health/readiness endpoint plus the existing service-key boundary.
- During migration there is exactly one domain writer for each table and one
  Worker owner for each Temporal task queue. Python remains the explicit
  compatibility owner for cognition, media, reflection, autonomy and workflow
  tables until a later handoff.
- Validate a complete DecisionEffect/MediaIntent/ReflectionProposal contract
  before any side effect. Invalid siblings cannot leave frozen messages,
  queued media, or duplicate retries.
- New or activated Fluctlights register the current local-day review intent in
  the same transaction as their baseline and schedule initialization. Worker
  startup repair is allowed only for historical rows.
- Reflection proposals use strict typed validation and commit the proposal,
  applier result, and watermark atomically. Malformed candidates return a typed
  failure and never advance the watermark.
- Dispatcher processing is bounded, durable, idempotent, and restart-safe;
  reflection work is debounced and isolated from daily-review/media capacity.
- Preserve all existing browser/API contracts: service-key and human-session
  headers/cookies, Core snake_case, browser camelCase, NDJSON streaming,
  `turn_id`/`idempotency_key`, monotonic sequences, and one terminal event.
- Run unit/integration checks, Go/Python build checks, Compose readiness, and
  real Docker regression cases 1–7 (including proactive contact). No mocks or
  hidden retry/manual Worker restart may be used to claim acceptance.

## Acceptance Criteria

- [x] Current live Python OpenAPI is reconciled with `packages/core-client` and
      semantic-diff checks pass.
- [x] Go Core vertical slice builds, has contract tests, persists through the
      existing database boundary, and is wired as an optional Compose runtime
      without a second writer.
- [x] Python Core compound effects, lifecycle review registration, and strict
      reflection validation pass focused regression tests.
- [x] Python Worker dispatcher passes bounded-dispatch, replay/idempotency,
      reflection debounce, and queue-isolation tests.
- [x] Go formatting/vet/test/build, Python tests for touched modules, browser
      type/build checks, and Compose config/readiness checks pass.
- [x] Real Docker cases 1–7 pass on first attempt within the ten-minute request
      limit; media retrieval, full creation details/schedule, visible moments,
      and proactive owner messages are verified through public APIs.
- [x] No `apps/bff` or Node BFF compatibility runtime is restored; no `master`
      changes are merged.

## Notes

- Work only on `codex/go-core-runtime-stability`, based on
  `codex/go-bff-cutover`.
- Do not alter released Alembic migration IDs or delete existing Docker data.
- Do not log credentials, service keys, cookies, provider secrets, or
  unbounded prompts/responses.
