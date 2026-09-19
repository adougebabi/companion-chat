# Backend Quality Guidelines

## Required Patterns

- Validate IDs, required text, cursor/query shape, and JSON body types at the
  Go browser boundary route boundary before calling Core.
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
- Keep Go Core as the only domain writer; domain repositories and workflow
  clients do not cross into the Go browser boundary.

## Forbidden Patterns

- Restoring `apps/browser`, a Node browser boundary runtime, or a second public gateway.
- Adding a second database, external queue, or storage client to the Go browser boundary.
- Making model, ComfyUI, PostgreSQL, Redis, MinIO, or Temporal calls from
  browser code or browser boundary route handlers.
- Completing a job/event with an expired or different lease owner.
- Changing browser event names or resource JSON fields without updating the
  Browser OpenAPI artifact, generated client, and Go route tests together.
- Logging bearer tokens, passwords, setup tokens, provider secrets, or
  committing private deployment data.

## Testing And Review

Run the owning Go checks and full browser workspace checks before committing:

```bash
gofmt -l apps/core-go/internal/httpapi/browser
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go/internal/httpapi/browser test -race ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go/internal/httpapi/browser vet ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go/internal/httpapi/browser build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

Run Go Core tests and the disposable Compose/public-boundary smoke when the
environment is available. Do not test against a checked-in or shared
production database unless the task explicitly authorizes a real regression
run and the scope of created records is recorded.

Review the full browser→browser boundary→Core path for every API change. Verify normal and
failure responses, stable error codes, cookie/CSRF behavior, stream terminal
errors, media Range behavior, and cancellation. Keep the Browser OpenAPI
artifact and generated clients drift-free.
