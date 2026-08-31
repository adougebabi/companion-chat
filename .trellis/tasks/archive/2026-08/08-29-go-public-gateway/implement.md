# Implementation Plan

1. Add a Go module and standard-library package layout under `apps/gateway-go`.
2. Implement environment configuration parsing and validation.
3. Implement explicit live, readiness, and platform-ping handlers.
4. Wire graceful HTTP server startup/shutdown in `cmd/gateway`.
5. Add fake-Core handler tests for success, downstream failure, credentials,
   and route allow-list behavior.
6. Add a small module README documenting sidecar usage and the no-cutover
   constraint.
7. Run `gofmt`, `go vet ./...`, `go test ./...`, and `go build ./...`.

The implementation must remain independent of Python Core internals and must
not alter the current Compose production topology. A later task may add a
separate opt-in Compose service after the browser contract is complete.
