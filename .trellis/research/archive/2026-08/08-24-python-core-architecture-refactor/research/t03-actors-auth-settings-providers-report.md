# T03 Actors, Owner Auth, Settings And Providers Report

## Status

`IMPLEMENTATION EVIDENCE ONLY — T12 ACCEPTANCE PENDING — 2026-08-24`

The Owner initially directed the implementation session to skip the remaining
T03-local verification work, then directed implementation to continue. The
record contains historical implementation evidence only; it is not `PASS`,
does not establish T03 acceptance or production readiness, and must be
re-run by T12 under the final matrix.

## Implemented Scope

- Actor, OwnerAccount, opaque Session, setup-token and audit persistence
  schemas; Argon2id credential handling; hashed opaque sessions; Owner setup,
  login, session resolution, revoke-all and password reset application flows.
- Single-key AES-GCM secret codec, Owner-only runtime settings patch/read/clear
  paths, write-only safe views and settings audit.
- Explicit model-role declarations, no-fallback preflight policy and durable
  Provider endpoint/role/preflight metadata schema, provenance persistence and
  an OpenAI-compatible `/models` plus structured/stream/embedding probe.
- Core internal auth/settings routes, generated Core-client auth/settings
  methods, BFF HttpOnly/Secure/SameSite=Lax cookie transport, trusted-Origin
  mutation guard and browser-safe settings route/operation metadata.

## Implementation Checks Recorded

- Focused Python auth/settings/provider unit tests were recorded as local
  implementation evidence before final acceptance was moved to T12.
- Ruff and mypy passed for the new Core modules and routes.
- Core/browser client generation and TypeScript typechecks passed.
- BFF Fastify inject tests passed for the existing auth/session/logout paths.
- Provider adapter fake-transport tests passed for structured, streaming,
  embedding and unknown-model/no-fallback preflight behavior; BFF Provider
  endpoint/role tests passed for session forwarding and safe camelCase output.
- Compose configuration parsed with only BFF host-exposed and with a valid
  base64 settings-key default.
- A real PostgreSQL migration container completed after the T03 revision was
  shortened to `0002_t03_auth`; a read-only database query confirmed current
  Alembic head `0003_t04_fluctlight` and all T03 tables exist.
- A real PostgreSQL AuthService probe issued a one-time setup token, created
  one Owner, resolved its opaque session, and a read-only query verified one
  64-character session hash with no password text stored in `auth_sessions`.
- A real PostgreSQL Owner settings probe encrypted a Provider secret; read-only
  checks confirmed nonempty ciphertext, a 12-byte nonce, no plaintext secret
  in runtime-settings JSON and a matching Owner audit record.

## T12 Acceptance Pending

- Full Compose startup/readiness, BFF-to-Core auth/settings integration and
  browser settings/provider end-to-end coverage.
- Real PostgreSQL transaction rollback and recovery CLI evidence beyond the
  setup/session/settings probes.
- A configured external Provider endpoint integration run, including timeout,
  budget, degraded-health and persisted provenance evidence.

## Known Failure And Follow-Up

The first clean Compose run reached Alembic version-table update and failed
because revision ID `0002_actors_auth_settings_providers` exceeded the default
32-character `alembic_version.version_num` limit. The revision was renamed to
`0002_t03_auth` and Core readiness was updated accordingly. The corrected
migration was then rerun non-destructively against the existing PostgreSQL
volume and completed through the current `0003_t04_fluctlight` head.

## Decision

The Owner-directed scope leaves T03 as implementation evidence only. T03
remains `in_progress`; do not archive, label `PASS`, or use this report to
unblock a production cutover. T12 must explicitly re-run and decide the
listed final acceptance work.
