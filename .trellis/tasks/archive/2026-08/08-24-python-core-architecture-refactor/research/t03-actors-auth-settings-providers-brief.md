# T03 Actors, Owner Auth, Settings And Providers Brief

## Purpose

Build the clean-start Actor/authentication/configuration/Provider foundation after the verified T02 platform handoff. This task creates no legacy compatibility surface and no product cutover.

## Required Decisions

T03 implements D001-D006, D008, D016, D021-D025, D028-D029 and D031-D033, plus the D037 implementation exception.

- Local/NAS Compose, clean start, one final cutover and old-code freeze remain in force.
- Node BFF owns browser transport; Python owns domain state, authorization, transactions, settings and Provider credential authority.
- Actor is a typed Human or Fluctlight reference. The first delivery has exactly one mandatory Owner Human.
- The shared UoW/Alembic/PostgreSQL boundary is inherited from T02. External Provider calls occur outside the transaction through explicit contracts.
- FastAPI/Pydantic/uv and Fastify/TypeBox/pnpm are fixed. BFF/Core use generated contracts; hand-written duplicate DTO pipelines are prohibited.
- `.env` plus PostgreSQL system settings are the only configuration authorities. One `FLUCTLIGHT_SETTINGS_KEY` encrypts sensitive runtime values. Six explicit Model Roles have role-specific preflight/provenance and no implicit fallback.
- Diagnostics/audit are redacted and built in. There is one writer and every child needs an exact brief, manifests, paths, commands, report and no-history dry run.

Before editing, read parent `READ_FIRST.md`, `decisions.md`, parent `design.md` sections for topology/auth/configuration/Provider/module boundaries, parent `implement.md` T02/T03 sections, this brief, and every manifest input.

## Exact Child Manifest

1. `.trellis/spec/backend/fluctlight-auth-contract.md`
2. `.trellis/spec/backend/fluctlight-configuration-contract.md`
3. `.trellis/spec/backend/fluctlight-provider-contract.md`
4. `.trellis/tasks/08-24-python-core-architecture-refactor/research/fluctlight-domain-model.md`
5. `.trellis/tasks/08-24-python-core-architecture-refactor/research/capability-inventory.md`
6. `.trellis/tasks/08-24-python-core-architecture-refactor/research/t03-actors-auth-settings-providers-brief.md`
7. `.trellis/tasks/08-24-python-core-architecture-refactor/research/t03-actors-auth-settings-providers-report-template.md`

## Entry Gate

The Owner approved D037 on 2026-08-24. It permits T03 to implement against the committed T02 foundation before T02's remaining implementation evidence, without creating a second writer. T03 does not own final acceptance; T12 does.

- Confirm T03 is the one exclusive writer. T02 may not modify shared migration, Core transport, generated client, BFF, Compose or lock files while this exception is active.
- Record T02's `in_progress` state and absent final acceptance report in the T03 handoff as a carry-forward platform risk.
- Re-run the T03 no-history dry run with D037. It must report no document ambiguity; T02 pending implementation evidence is a tracked risk rather than an operational start blocker.
- Any failed T02 implementation check, changed shared surface or unresolved T02 report that conflicts with implementation returns to parent planning.

## Owned Paths

After entry, T03 owns:

- `apps/core/src/fluctlight_core/actors/`, `apps/core/src/fluctlight_core/settings/`, and `apps/core/src/fluctlight_core/providers/`.
- New tests under `apps/core/tests/actors/`, `settings/`, `providers/`, `contract/`, and `architecture/`.
- The next linear Alembic revision after `apps/core/migrations/versions/0001_platform.py`.
- The parent T03 report and dry-run artifacts.

T03's exclusive writer is the integration owner for only the shared changes required by its accepted design: `apps/core/migrations/env.py`, `apps/core/src/fluctlight_core/transport/api.py`, generated Core/BFF/browser OpenAPI artifacts and clients, `apps/bff/src/` plus its tests/package manifest, lock files, and `infra/compose/fluctlight.*` when needed to keep Core internal and BFF the sole host exposure.

Forbidden: old `server/`, old `web/`, old `test/`, `0001_platform`, T02 platform behavior, T04+ domain modules, legacy settings/secret migration, compatibility/dual-write routes, alternate auth/key/workflow/Provider runtimes, and cutover/deletion work.

## Deliverables

- Typed Actor identity/reference API, one OwnerAccount and opaque Session persistence/application interfaces.
- One-time setup token, Argon2id credentials, login/logout/revocation, session resolver/authorization and audited local recovery.
- BFF cookie/CSRF/origin transport that forwards both independent service identity and opaque Human session to Core.
- Typed runtime settings, safe write-only views, AEAD codec, atomic Owner patch/clear/audit and Python-only Provider secret resolver.
- ProviderEndpoint and six ModelRole records, role-specific preflight/health/provenance, fake and configured OpenAI-compatible normalized adapters, no role/model fallback.
- Linear migration/readiness revision change, generated Core/browser contracts and tests that prove safe BFF-only exposure and no secret leakage.

## Implementation Checks (Non-Authoritative)

T03 may run these local checks only to support safe implementation. They are evidence inputs, not child acceptance; T12 must re-run the required auth/config/provider matrix and all aggregate scenarios:

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

The report must name focused real-PostgreSQL auth/settings/provider fixtures, BFF cookie/CSRF integration coverage and generated-client drift evidence. Raw fixture secrets must never appear in normal output, snapshots or report evidence.

## Evidence Handoff

Record exact versions, implementation-check commands/results, migration revision, safe generated schemas, owned/shared paths, T12 coverage IDs and risks in the T03 report. Do not issue PASS; the complete assigned auth/configuration/provider contract and error matrices, architecture/real-PostgreSQL/failure/redaction tests and browser aggregate scenarios remain T12-owned. The handoff to T04 exposes only the public resolved HumanActor authorization/reference interface; it must not expose repositories, raw sessions or secrets.
