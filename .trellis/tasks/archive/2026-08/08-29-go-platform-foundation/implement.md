# Implementation Plan

1. Add `internal/platform/health.go` with role and probe contracts.
2. Add platform unit tests.
3. Refactor Go BFF health handlers to use the platform values and readiness
   probe.
4. Add/retain BFF tests proving live/readiness behavior and no domain access.
5. Run gofmt, `go test ./...`, race, vet, and build.
