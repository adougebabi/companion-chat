# Fluctlight Workspace

This repository contains the clean-start Fluctlight system:

- `apps/core-go`: Go modular Core API, PostgreSQL domain writer, Provider/media
  adapters, Temporal activities and the Go migration runner.
- `apps/gateway-go`: Go browser BFF. It is the only public browser boundary and
  owns cookies, CSRF/origin transport, browser DTOs, NDJSON translation and
  media proxying.
- `apps/web`: Vue 3/Vite/Pinia product UI and Control Center, built to static
  assets and served by Nginx in the production image.
- `packages/core-client` and `packages/browser-client`: generated contract
  clients whose OpenAPI artifacts must be regenerated together. The browser
  artifact generator lives in `packages/browser-client/scripts`.
- `infra/compose`: PostgreSQL/pgvector, Redis Streams, MinIO, Temporal, Go Core,
  Go Worker, Go BFF and static Web deployment topology. Compose runs the Go
  cutover fence before the Worker and exposes a Worker health signal so a
  process that stops polling cannot remain silently healthy.

## Local Checks

```bash
pnpm install --frozen-lockfile
pnpm generate
pnpm typecheck
pnpm test
pnpm build

GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go test -race ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go vet ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go build ./...
```

The full disposable Compose smoke and recovery checks are under
`infra/acceptance/`. They accept a private env file through
`FLUCTLIGHT_ENV_FILE`; do not commit credentials. PostgreSQL, object storage,
Temporal default/visibility databases and `.env` are backed up together using
the manifest tooling under `infra/backup/`.

The repository contains only the clean-start Fluctlight runtime. Deploy with
`infra/compose/fluctlight.compose.yml` and a private, untracked environment
file based on `infra/compose/fluctlight.env.example`.

On a fresh database, issue the one-time Owner setup token from the Go image and
use it through `/auth/setup`:

```bash
docker compose run --rm --no-deps migrate /usr/local/bin/fluctlight-setup-token-go --expires-minutes 30
```

For a NAS or another machine, set both public browser-facing URLs in that
private file: `FLUCTLIGHT_TRUSTED_ORIGIN` is the exact URL used to open the
Web application, while `FLUCTLIGHT_BFF_ORIGIN` is the browser-reachable BFF
URL. The Web container reads `FLUCTLIGHT_BFF_ORIGIN` at startup and writes a
public runtime config file before Nginx serves the static build, so changing
the BFF address requires only recreating `web`, not rebuilding its image. Do
not use a container hostname or
`127.0.0.1` unless the browser itself runs on that same machine.
`CORE_BASE_URL` remains the BFF-to-Core internal address and normally uses the
Compose service name `http://core:8080`. The Go BFF is the only public BFF;
there is no alternate BFF runtime in this repository.

Temporal keeps its existing namespace and deployment. The Go Worker claims the
configured `TEMPORAL_WORKER_BUILD_ID` (default `platform-v1`) on all three
canonical queues (`interaction`, `lifecycle`, `media`). On cutover, the
one-shot `cutover` service cancels and, when necessary, terminates retired
runtime executions and records every result in
`platform_workflow_management_audit`; it does not delete PostgreSQL facts,
media objects, or Temporal history.

## Go migration branch flow

Go migration stages use branches named `codex/go-*`. A stage branch is cut
from the previous stage and is checked by CI on every push and pull request;
these checks run Go tests, vet, Go builds, and non-publishing Docker builds.

Completed stages are merged into `codex/go-build`. A push to that integration
branch publishes the shared validation images with the `go-build` tag (and an
immutable `go-<commit-sha>` tag) for Core, the Go BFF, and Web. Only a push to
`main` or `master` publishes the production `latest` tags.
