# T02 Workspace And Platform Foundation Plan

## Entry

- Parent D020/D036 Temporal acceptance recorded.
- Read parent READ_FIRST/decisions/design/implement and exact T02 brief.
- Exact nine-entry manifests loaded.
- No-history handoff dry run passes.
- Child explicitly started as `in_progress`; exclusive writer confirmed.

## Owned Paths

Exactly the paths in parent `research/t02-platform-foundation-brief.md`, including workspace locks, platform/entrypoint/transport packages, app skeletons, generated-client packages, final Compose/infra and new platform/architecture/contract/integration tests.

Frozen old `server/`, `web/`, `test/` are forbidden.

## Checklist

- [ ] Establish uv/pnpm workspace pins and lockfiles.
- [ ] Build FastAPI Core API and separate Temporal Worker entrypoints.
- [ ] Build Fastify BFF/Vue skeletons and generated client pipelines.
- [ ] Add PostgreSQL/pgvector/Alembic/UoW foundation.
- [ ] Add outbox/inbox + Redis durable/progress stream foundation.
- [ ] Add MinIO/S3 private adapter/grant fixture.
- [ ] Promote grouped Temporal topology and remove gate-only code/default paths.
- [ ] Complete live reset/restart/history-point and management authorization/audit integration.
- [ ] Add health/readiness/internal networking and BFF-only host exposure.
- [ ] Add architecture/contract/real-PG tests and clean platform smoke runner.
- [ ] Run all exact commands and complete T02 report.

## Validation

Run exactly the T02-owned uv paths, `test:platform` package scripts, named Temporal management/history tests and minimal Compose readiness/ping commands in the brief. Do not run predecessor/full-product suites. No fixed-duration soak/resource threshold is part of exit.

## Exit

- PASS: report/check/commit/archive and parent prepares T03 child brief.
- FAIL: retain evidence, block T03+, return parent platform planning.
