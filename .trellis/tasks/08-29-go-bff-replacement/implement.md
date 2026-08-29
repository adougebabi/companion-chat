# Implementation Plan

1. Refactor the first-stage Go health/ping server behind a reusable BFF
   package and add config for trusted origin, cookie security and Core client.
2. Implement common Core JSON transport, error decoding, session lookup,
   cookie/CORS/CSRF middleware and explicit route matching.
3. Implement auth, settings, providers, Fluctlight, actor-group, conversation,
   memory, relationship, life-world, moments, autonomy and workflow routes
   with exact Core body/query mappings and stable browser errors.
4. Implement the conversation NDJSON translator and streaming cancellation.
5. Implement the media Range proxy and allow-listed response headers.
6. Port the existing BFF tests and add route-matrix/golden tests for the
   currently uncovered Browser operations.
7. Add an opt-in Go BFF Docker/Compose entry only after the local parity suite
   passes; keep Node as the default until the deployment smoke is successful.
8. Run gofmt, `go test ./...`, `go vet ./...`, `go build ./...`, existing Web
   checks, and disposable Core/BFF smoke. Record the cutover decision.
