# Fluctlight Workspace

This repository contains the clean-start Fluctlight system:

- `apps/core`: Python 3.13 FastAPI/Core and Temporal Worker. It owns Actors,
  Fluctlights, cognition, conversations, Memory, Relationships, Life World,
  Moments, Media, persistence and authorization.
- `apps/bff`: Node 24/Fastify browser boundary. It owns cookies, CSRF/origin
  transport, generated browser DTOs, NDJSON translation and media proxying.
- `apps/web`: Vue 3/Vite/Pinia product UI and Control Center.
- `packages/core-client` and `packages/browser-client`: generated contract
  clients whose OpenAPI artifacts must be regenerated together.
- `infra/compose`: PostgreSQL/pgvector, Redis Streams, MinIO, Temporal, Core,
  Worker, BFF and Web deployment topology.

## Local Checks

```bash
pnpm install --frozen-lockfile
pnpm generate
pnpm typecheck
pnpm test
pnpm build

uv sync --locked
.venv/bin/ruff format --check apps/core/src apps/core/tests
.venv/bin/ruff check apps/core/src apps/core/tests
.venv/bin/mypy --follow-imports=skip apps/core/src apps/core/tests
.venv/bin/pytest -q apps/core/tests
```

The full disposable Compose smoke and recovery checks are under
`infra/acceptance/`. They accept a private env file through
`FLUCTLIGHT_ENV_FILE`; do not commit credentials. PostgreSQL, object storage,
Temporal default/visibility databases and `.env` are backed up together using
the manifest tooling under `infra/backup/`.

The repository contains only the clean-start Fluctlight runtime. Deploy with
`infra/compose/fluctlight.compose.yml` and a private, untracked environment
file based on `infra/compose/fluctlight.env.example`.

For a NAS or another machine, set both public browser-facing URLs in that
private file before building the Web image: `FLUCTLIGHT_TRUSTED_ORIGIN` is the
exact URL used to open the Web application, while `VITE_BFF_ORIGIN` is the
browser-reachable BFF URL. `VITE_BFF_ORIGIN` is embedded into Vite's static
build, so changing it requires rebuilding the `web` image. Do not use a
container hostname or `127.0.0.1` unless the browser itself runs on that same
machine. `CORE_BASE_URL` remains the BFF-to-Core internal address and normally
uses the Compose service name `http://core:8080`.
