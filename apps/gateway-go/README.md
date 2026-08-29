# Go BFF

This is the Go implementation of the browser-facing BFF. It preserves the
current Node BFF browser contract while keeping Python Core as the only domain
writer. The Go BFF does not access PostgreSQL, Redis, S3, or Temporal.

## Local run

```bash
CORE_BASE_URL=http://127.0.0.1:8080 \
FLUCTLIGHT_CORE_SERVICE_KEY=replace-me \
FLUCTLIGHT_TRUSTED_ORIGIN=http://localhost:13001 \
go run ./cmd/gateway
```

The process listens on `0.0.0.0:3000` by default. Set `BFF_HOST` and
`BFF_PORT` for the current Compose-compatible names, or
`GATEWAY_LISTEN_ADDR` for an explicit address.

The BFF exposes the same `/auth/*`, `/api/*`, `/health/*` and `OPTIONS /*`
routes as the existing Node implementation. The Node BFF remains in the
repository as a rollback/reference implementation; production switching is a
deployment decision backed by the parity suite and disposable smoke checks.
