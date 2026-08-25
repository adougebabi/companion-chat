# T02 Workspace And Platform Foundation Brief

## Purpose

Create the clean-start monorepo/platform foundation after Temporal core acceptance. T02 builds runtime seams and one-command infrastructure, not Fluctlight product/domain behavior.

## Required Decisions

D001-D007, D016-D024, D028-D029, D031-D037.

Decision meanings required by this child:

- Local/NAS Compose, clean start, one final product cutover, old code frozen.
- Node BFF owns browser transport only; Python owns application/runtime state.
- Python API/Worker share package/image but are separate processes.
- Unit of Work, stable intent/outbox/inbox, one PostgreSQL application schema.
- Memory/media/event infrastructure uses PostgreSQL/pgvector, S3/MinIO and Redis Streams with their approved ownership.
- Temporal is the sole runtime; grouped non-HA Server, PostgreSQL visibility, no ES/default UI/metrics, three task queues.
- Python 3.13/uv/FastAPI and Node 24/pnpm/Fastify/Vue stacks are fixed.
- Built-in diagnostics only; SQLAlchemy/Psycopg/Alembic; no resource-duration/soak hard gates.
- Strict single writer and child-specific readiness remain mandatory.

Before editing, manually read parent `READ_FIRST.md`, `decisions.md`, `design.md`, parent `implement.md` T01B/T02/T03, and `research/temporal-runtime-acceptance.md`.

## Exact Child Manifest

1. `.trellis/spec/backend/fluctlight-persistence-contract.md`
2. `.trellis/spec/backend/fluctlight-event-contract.md`
3. `.trellis/spec/backend/fluctlight-workflow-contract.md`
4. `.trellis/spec/backend/fluctlight-api-contract.md`
5. `.trellis/spec/backend/fluctlight-bff-contract.md`
6. `.trellis/spec/backend/fluctlight-media-contract.md`
7. `.trellis/spec/backend/fluctlight-configuration-contract.md`
8. `.trellis/tasks/08-24-python-core-architecture-refactor/research/t02-platform-foundation-brief.md`
9. `.trellis/tasks/08-24-python-core-architecture-refactor/research/temporal-runtime-acceptance.md`

## Owned Paths

- `.python-version`, root `pyproject.toml`, `uv.lock`.
- `pnpm-workspace.yaml`, `pnpm-lock.yaml`, `.node-version`; do not rewrite frozen old root npm scripts/dependencies except a parent-approved minimal workspace hook.
- `apps/core/pyproject.toml`.
- `apps/core/src/fluctlight_core/platform/`, `apps/core/src/fluctlight_core/entrypoints/`, `apps/core/src/fluctlight_core/transport/`.
- `apps/core/tests/platform/`, `apps/core/tests/contract/`, `apps/core/tests/architecture/`.
- Existing `apps/core/src/fluctlight_core/temporal_gate/` and `apps/core/tests/temporal_gate/` only to extract/promote reusable Temporal adapter code and remove gate-only production entrypoints.
- `apps/bff/`, `apps/web/` platform skeletons only.
- `packages/core-client/`, `packages/browser-client/` generated-client pipelines.
- `infra/compose/fluctlight.compose.yml`, `infra/compose/fluctlight.env.example`, `infra/compose/run-platform-smoke.sh`.
- `infra/postgres/`, `infra/redis/`, `infra/minio/`.
- root new-system `tests/architecture/`, `tests/contract/`, `tests/integration/`.
- Parent `research/t02-platform-foundation-report.md`.

Forbidden: old `server/`, old `web/`, old `test/`; product/domain modules; auth/account behavior; final UI; alternate workflow/message/vector/object runtime; compatibility adapters.

## Deliverables

- Reproducible uv/pnpm workspaces and pinned runtimes/locks.
- FastAPI Core API and separate Temporal Worker entrypoints with health/readiness.
- Fastify BFF and Vue skeleton with generated Core/browser OpenAPI clients.
- PostgreSQL 16 + pgvector, explicit Alembic migration command/revision check, AsyncSession Unit of Work foundation.
- Minimal stable intent/outbox/inbox fixture and Redis durable/progress stream adapters.
- Private MinIO/S3 adapter contract and media grant transport fixture, not product Media behavior.
- Final grouped Temporal topology promoted from the gate, with DBOS absent.
- Live reset/restart/history-point and complete management operation integration with fake authorized/audited context.
- One-command clean Compose smoke and architecture/contract/real-PostgreSQL tests.

No fixed-duration soak, strict RSS/CPU threshold, 30-day disk projection or repeated resource research is required.

## Validation Commands

```bash
uv sync --locked
uv run ruff format --check apps/core/src/fluctlight_core/platform apps/core/src/fluctlight_core/entrypoints apps/core/src/fluctlight_core/transport apps/core/tests/platform apps/core/tests/contract apps/core/tests/architecture
uv run ruff check apps/core/src/fluctlight_core/platform apps/core/src/fluctlight_core/entrypoints apps/core/src/fluctlight_core/transport apps/core/tests/platform apps/core/tests/contract apps/core/tests/architecture
uv run mypy apps/core/src/fluctlight_core/platform apps/core/src/fluctlight_core/entrypoints apps/core/src/fluctlight_core/transport
uv run pytest apps/core/tests/platform apps/core/tests/contract apps/core/tests/architecture -q
uv run pytest tests/architecture tests/contract tests/integration -q
uv run pytest apps/core/tests/temporal_gate/test_management.py apps/core/tests/temporal_gate/test_history_versioning.py -q
pnpm install --frozen-lockfile
pnpm --filter @fluctlight/bff lint
pnpm --filter @fluctlight/bff typecheck
pnpm --filter @fluctlight/bff test:platform
pnpm --filter @fluctlight/bff build
pnpm --filter @fluctlight/web lint
pnpm --filter @fluctlight/web typecheck
pnpm --filter @fluctlight/web test:platform
pnpm --filter @fluctlight/web build
docker compose -f infra/compose/fluctlight.compose.yml config
./infra/compose/run-platform-smoke.sh --clean
```

The smoke runner uses trap cleanup and `down -v --remove-orphans`. It verifies BFF-only host exposure, Core/Worker/Temporal/PG/Redis/MinIO health, BFF→Core platform ping, running containers and no DBOS process/dependency. Intent/workflow/event/object adapter behavior is covered by the targeted platform/contract/integration tests, not duplicated in smoke.

## Exit And Handoff

- Complete T02 report with exact versions/commands/results/paths/risks.
- Run child check and normal commit/archive workflow.
- PASS allows parent to prepare T03 brief; no product cutover occurs.
- Functional Temporal management/reset failure blocks T03 and returns parent platform planning, but resource-duration studies cannot block exit.
