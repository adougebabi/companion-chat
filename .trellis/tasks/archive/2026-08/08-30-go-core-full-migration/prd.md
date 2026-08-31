# Complete Python Core migration to Go

## Goal

Replace the Python Core runtime completely with a Go Core runtime on the
`codex/go-core-full-migration` branch. Go must become the sole owner of the
current Core API, authentication, PostgreSQL domain state, Provider adapters,
media/object-storage integration, cognition/reflection/autonomy, workflow
intent dispatch, and Temporal Worker execution. After cutover, `apps/core`
must no longer be deployed, imported, or required by Compose/CI. The public Go
BFF contract and all existing product behavior must remain intact.

## Confirmed facts

- The previous stage added `apps/core-go` as a PostgreSQL-backed read/transport
  slice; it is not yet a complete Core and is not wired as the BFF's writer.
- Python Core currently contains 117 source files, 63 API paths/68 operations,
  six product domain groups, provider/media integrations, and eight Temporal
  workflows/activities across three queues (`interaction`, `lifecycle`,
  `media`).
- The previous real Docker regression demonstrated required behavior for
  normal chat, image generation, blank and described creation, complete detail
  and schedule, Moment publication, and proactive contact. The final run also
  proved activation-time daily-review registration, strict effect/reflection
  validation, and restart-safe bounded dispatch.
- Existing database migration head is `0020_media_provider_job`; released
  Alembic IDs and existing Docker volumes must not be changed or deleted.

## Requirements

- Implement every active Python Core API operation in Go with the same Core
  snake_case contract, service-key/session authorization, status/error behavior,
  NDJSON streaming, cancellation and idempotency semantics.
- Port all domain modules: actors/auth, settings/configuration, Fluctlights and
  foundation creation/governance, inner state, conversations, cognition,
  memory, relationships, reflection, life world/schedule, autonomy, moments,
  media, diagnostics, operations and platform/outbox/inbox primitives.
- Port Provider adapters and structured contracts without semantic heuristics;
  preserve model roles, evidence, prompt versions, bounded diagnostics,
  media-intent authority, and explicit DecisionEffect/ReflectionProposal
  validation.
- Port Temporal workflows and activities with deterministic replay/versioning,
  stable workflow IDs, queue ownership, heartbeats, cancellation, continue-as-
  new behavior, management operations and recovery. There must be one active
  Worker implementation per task queue and one workflow runtime.
- Establish one Go domain writer per table before switching any BFF route. The
  Python Core may remain only as a short-lived migration tool in tests/builds;
  it must not run in the final Compose topology or receive production traffic.
- Preserve existing PostgreSQL domain data and media assets. Compatibility
  with in-flight Python Temporal workflow histories is explicitly out of scope;
  the cutover may fence/cancel old executions and rebuild their durable intents
  under Go workflow IDs, but must record the outcome and never silently lose
  domain facts.
- Update Dockerfiles, Compose, GitHub Actions, OpenAPI/generated clients,
  acceptance scripts, documentation and resource limits so deployment has only
  Go Core + Go Worker + Go BFF + static Web (middleware unchanged).
- Execute real Docker regression cases 1–7 after cutover, with request timeout
  no greater than ten minutes, no mock Core/Provider, no hidden retry and no
  manual Worker restart as an acceptance step.

## Acceptance Criteria

- [ ] Go Core implements and contract-tests all 63 API paths/68 operations;
      generated Core OpenAPI and Browser client remain compatible.
- [ ] Go Core and Go Worker are the only Core/Worker services in Compose and
      GitHub Actions; `apps/core` is absent from the final runtime graph.
- [ ] Existing PostgreSQL data, auth sessions, media assets, workflow intents,
      and active Temporal histories survive the cutover with no duplicate or
      lost domain effects.
- [ ] Go tests cover transport, persistence, provider, effect validation,
      reflection watermark/CAS, workflow replay, media recovery and all
      authorization/error paths; race/vet/build pass.
- [ ] Real regression cases 1–7 pass through the public Go BFF after cutover:
      chat, first-attempt image generation, blank creation, described creation,
      full detail/schedule, visible Moment, and proactive Owner contact.
- [ ] Python Core is no longer imported, launched or referenced by active
      deployment/CI scripts; only migration notes/tests may mention it.

## Confirmed migration decision

- Old Python Temporal workflow history compatibility is not required. The Go
  cutover will stop new Python starts, fence/cancel old executions through the
  authorized Temporal API, preserve domain facts/assets, and rebuild eligible
  pending intents with Go workflow IDs and an explicit migration correlation.
  This shortens the migration but intentionally does not promise continuation
  of old timer/history state.
