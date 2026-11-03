# T02 Design Baseline

## Authority

Parent task owns architecture. `research/t02-platform-foundation-brief.md` is the executable child boundary; this child may refine only implementation details inside it.

## Scope

pnpm/uv workspace、apps skeleton、FastAPI/Fastify/Vue transport foundation、PostgreSQL/Redis/MinIO Compose、Alembic/Unit of Work/outbox/inbox、generated clients、health 与 shared test harness.

## Dependencies

T01/T01B are archived; parent D020/D036 accepts Temporal core. No resource-duration gate remains.

## Boundary Rules

- Use only public module/application interfaces and assigned shared-file ownership.
- No old-code maintenance, compatibility, alternative infrastructure/runtime, or silent decision changes.
- External I/O, persistence, semantic and security behavior follow assigned parent code-specs.
- Product remains uncut-over until T12.

## Readiness Gate

Before `task.py start`, the existing exact brief/manifests/paths/commands must pass a no-history handoff dry run. T02 builds platform seams only and cannot implement T03+ product modules.
