# T03 Implementation Evidence Handoff

Status: implementation evidence complete; `acceptance_owner=T12`;
`acceptance=pending`.

## Changed Paths

- `apps/core/src/fluctlight_core/actors/`, `settings/` and `providers/`
  application contracts, persistence and authorization services.
- `apps/core/migrations/versions/0002_actors_auth_settings_providers.py`,
  shared migration registration and Core readiness integration.
- Core/BFF transport, generated Core/browser OpenAPI artifacts and clients,
  Compose configuration and the focused T03 tests.

## Implementation Evidence

- Core full regression before final cutover work: `.venv/bin/pytest -q
  apps/core/tests` passed `110` tests; Ruff format/check and mypy passed.
- BFF external IPC test run passed `11` tests; browser/Core client generation
  and typechecks passed.
- Disposable auth/domain smoke issued the one-time setup flow, resolved an
  opaque Owner session, created a Fluctlight with a matching `fluctlight`
  Actor row, created a Conversation participant and preserved an explicit
  unconfigured-Provider error.
- Disposable Compose startup verified migration head
  `0009_t12_vector_column`, private settings-key configuration and BFF-only
  browser exposure.

## Produced Contracts / Schema

- Owner Human Actor, one-time setup, Argon2id credential and opaque hashed
  session records with revoke-current/revoke-all and password recovery.
- Runtime settings with write-only safe views and single-key AEAD secret
  storage.
- Six explicit Provider roles, endpoint/role preflight and configured
  OpenAI-compatible structured, realization and embedding adapter ports.
- Generated Core/BFF/browser contract artifacts with snake/camel boundary
  mapping owned by the Core client.

## T12 Coverage

`T03-AUTH-01` through `T03-AUTH-06`, `T03-CFG-01` through `T03-CFG-05`,
`T03-PROV-01` through `T03-PROV-05`, plus the cross-module Owner/session,
CSRF, secret-redaction, real-PostgreSQL and external-Provider scenarios.

## Remaining Risks / Excluded Scope

- T03 evidence does not establish final acceptance. T12 must still prove
  transaction rollback/recovery, browser security, successful configured
  Provider execution and persisted provenance under the final matrix.
- Multi-Human accounts, group chat, anonymous mode, implicit Provider fallback
  and legacy compatibility remain excluded.

Rollback point: remove only T03-owned modules and revision `0002` before
cutover if the final T12 matrix cannot be satisfied; preserve later migrations
and unrelated evidence until the dependency graph is reviewed.
