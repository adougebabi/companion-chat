# T03 Implementation Plan

## Status And Entry

This plan is document-ready but not implementation-authorizing. Do not call `task.py start` until the entry conditions in `research/t03-actors-auth-settings-providers-brief.md` are satisfied and the parent confirms the exclusive writer.

## Owned Paths After Entry

- New Core modules under `apps/core/src/fluctlight_core/actors/`, `settings/`, and `providers/`, plus their public transport-neutral application interfaces.
- New T03-focused tests under `apps/core/tests/actors/`, `settings/`, `providers/`, `contract/`, and `architecture/`.
- One Alembic revision after `apps/core/migrations/versions/0001_platform.py`.
- Approved shared integration changes only: `apps/core/migrations/env.py`, `apps/core/src/fluctlight_core/transport/api.py`, Core/BFF OpenAPI and generated-client inputs/outputs, `apps/bff/src/` and its tests/package manifest, the relevant lock files, and `infra/compose/fluctlight.*` only when needed to preserve the Core-internal/BFF-only boundary.
- T03 report and dry-run artifacts named in the parent `research/` directory.

Forbidden: old `server/`, `web/`, and `test/`; existing `0001_platform` migration; T02 platform semantics; T04+ domain modules; product cutover; compatibility layers; a second migration/metadata/client pipeline; secret material in DTOs or diagnostics.

## Ordered Checklist

1. Verify T02 exit evidence, task status and handoff; re-run the T03 no-history dry run against the real T02 report and paths.
2. Add typed Actor, OwnerAccount and opaque Session persistence/application interfaces using the shared metadata/UoW and next Alembic revision.
3. Implement one-time setup, Argon2id credential policy, login/logout/session resolution/revoke-all and local recovery with audit/atomic rollback.
4. Add Core authorization boundary and BFF cookie/CSRF/origin transport; update generated Core and browser contracts together.
5. Implement typed runtime settings, write-only safe views, single-key AEAD secret codec, Owner-only atomic patch/clear and audited secret resolution.
6. Implement ProviderEndpoint, six independent ModelRole assignments, role-specific preflight/health/provenance and normalized fake/OpenAI-compatible adapters with no fallback.
7. Update readiness revision, generated-client drift gates, Compose/network expectations and tests through the designated shared-file owner.
8. Record implementation evidence, T12 coverage IDs, unresolved risks and the T04 handoff; do not run or claim a T03 acceptance matrix.

## Implementation Checks (Non-Authoritative)

If needed for safe development, T03 may run the following local checks. They are implementation evidence only, are not a T03 acceptance gate, and must be re-run by T12 under the final matrix. Do not use their result to claim PASS or production readiness:

```bash
uv sync --locked
uv run ruff format --check apps/core/src/fluctlight_core/actors apps/core/src/fluctlight_core/settings apps/core/src/fluctlight_core/providers
uv run ruff check apps/core/src/fluctlight_core/actors apps/core/src/fluctlight_core/settings apps/core/src/fluctlight_core/providers apps/core/tests/actors apps/core/tests/settings apps/core/tests/providers
uv run mypy apps/core/src/fluctlight_core/actors apps/core/src/fluctlight_core/settings apps/core/src/fluctlight_core/providers apps/core/tests/actors apps/core/tests/settings apps/core/tests/providers
uv run pytest apps/core/tests/actors apps/core/tests/settings apps/core/tests/providers apps/core/tests/contract/test_t03_*.py apps/core/tests/architecture/test_t03_*.py -q
pnpm install --frozen-lockfile
pnpm --filter @fluctlight/core-client generate
pnpm --filter @fluctlight/core-client typecheck
pnpm --filter @fluctlight/bff generate:browser-client
pnpm --dir apps/bff exec tsx --test test/auth*.test.ts test/settings*.test.ts test/providers*.test.ts
pnpm --filter @fluctlight/bff typecheck
pnpm --filter @fluctlight/browser-client generate
pnpm --filter @fluctlight/browser-client typecheck
docker compose -f infra/compose/fluctlight.compose.yml config
```

Any evidence must identify fixtures, generated artifacts and boundary assumptions without exposing secrets. The T03 handoff must list the corresponding T12 coverage IDs and unresolved final validation items.

## Exit And Handoff

The handoff records exact versions, implementation-check commands/results, owned/shared paths, migration revision, OpenAPI artifacts, remaining risks and the public resolved-HumanActor interface for T04. T03 does not issue PASS; assigned contract/error matrices and architecture/real-PostgreSQL/failure/redaction acceptance remain pending T12. Any T02 incompatibility, security/redaction failure or required shared-owner conflict returns to parent planning; it does not create a side implementation.

## Historical Owner Skip (T12 Pending)

On 2026-08-24 the Owner explicitly skipped the remaining T03-local verification
work after
the requested implementation work. The record is
`research/t03-actors-auth-settings-providers-report.md`. This does not change
T12's final acceptance scope, archive T03, or establish production readiness;
it records historical implementation evidence and leaves the listed final
verification work pending T12.
