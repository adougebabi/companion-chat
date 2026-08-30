# Backend Directory And Module Structure

## Actual Layout

```text
apps/core/
  src/fluctlight_core/           Python Core modular monolith and Temporal Worker
  migrations/                    PostgreSQL/Alembic schema history
apps/gateway-go/
  cmd/gateway/                   Go public BFF composition root
  internal/bff/                  browser routes, DTOs, NDJSON, media proxy
  internal/config/               startup and security configuration
  internal/platform/             transport-neutral health helpers
packages/core-client/            generated/reference Core contract client
packages/browser-client/         Browser OpenAPI artifact and generated client
apps/web/                        Vue/Vite static assets served by Nginx
infra/compose/                   Core, Worker, BFF and middleware deployment
infra/acceptance/                disposable Compose and public-boundary checks
```

`apps/gateway-go` is the only public BFF runtime. The former Node BFF tree,
its Dockerfile, tests, package importer, and generator entrypoint are deleted;
do not restore them as a compatibility entrypoint or rollback service.

## Module Boundaries

- Go BFF packages may depend on the standard library and transport-neutral
  helpers only. They must not import PostgreSQL, Redis, S3, Temporal, Python
  internals, domain repositories, or semantic policy modules.
- Python Core owns domain state, authorization, persistence, provider calls,
  and Worker activities. Its transport adapters map HTTP DTOs to application
  interfaces and never expose raw database rows.
- Browser OpenAPI is generated beside its committed artifact in
  `packages/browser-client/scripts`; root generation updates the artifact and
  generated client together.
- Compose service name `bff` and image repository `fluctlight-bff` are stable
  deployment names, but their process and Dockerfile are Go-only.

## Adding Backend Behavior

- Put browser validation, status selection, and DTO mapping in the explicit Go
  route that owns the operation; do not add a generic Core path proxy.
- Put domain rules and durable writes in the owning Python module; do not
  duplicate them in the BFF.
- Keep external provider/storage/workflow calls behind their existing Core
  adapters and preserve cancellation and bounded error mapping at transport
  boundaries.
- When adding a browser operation, update the OpenAPI artifact, generated
  client, Go route inventory, and public acceptance coverage in one change.

## Naming And Boundaries

Use Go `camelCase` identifiers for local values, browser JSON camelCase, Core
JSON snake_case, opaque session cookies, and explicit `internal/*` endpoints.
Public browser requests must never receive credentials, raw provider payloads,
database rows, workflow internals, or unbounded error details.
