# Backend Quality Guidelines

## Required Patterns

- Validate IDs, required text, cursor/query shape, and JSON body types at the
  Go BFF route boundary before calling Core.
- Keep browser camelCase↔Core snake_case mappings explicit per route. Do not
  recursively guess field names or proxy arbitrary `/internal/*` paths.
- Forward the Core service identity and opaque human session independently;
  never authorize from a browser-supplied Actor ID.
- Stream NDJSON incrementally across arbitrary byte boundaries, enforce one
  monotonic sequence and one terminal event, and suppress writes after
  cancellation.
- Proxy media bodies directly with bounded allow-listed headers; never expose
  bucket/object keys, credentials, or provider details.
- Sanitize Core error details recursively with bounded depth/collections/text
  before returning browser errors.
- Keep Python Core as the only domain writer; domain repositories and workflow
  clients do not cross into the Go BFF.

## Forbidden Patterns

- Restoring `apps/bff`, a Node BFF runtime, or a second public gateway.
- Adding a second database, external queue, or storage client to the Go BFF.
- Making model, ComfyUI, PostgreSQL, Redis, MinIO, or Temporal calls from
  browser code or BFF route handlers.
- Completing a job/event with an expired or different lease owner.
- Changing browser event names or resource JSON fields without updating the
  Browser OpenAPI artifact, generated client, and Go route tests together.
- Logging bearer tokens, passwords, setup tokens, provider secrets, or
  committing private deployment data.

## Testing And Review

Run the owning Go checks and full browser workspace checks before committing:

```bash
gofmt -l apps/gateway-go
GOCACHE=/tmp/fluctlight-go-cache go -C apps/gateway-go test -race ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/gateway-go vet ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/gateway-go build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

Run Python Core pytest and the disposable Compose/public-boundary smoke when
the environment is available. Do not test against a checked-in or shared
production database unless the task explicitly authorizes a real regression
run and the scope of created records is recorded.

Review the full browser→BFF→Core path for every API change. Verify normal and
failure responses, stable error codes, cookie/CSRF behavior, stream terminal
errors, media Range behavior, and cancellation. Keep the Browser OpenAPI
artifact and generated clients drift-free.
