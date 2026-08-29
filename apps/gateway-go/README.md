# Go Public Gateway

This is the first Go migration slice. It is a sidecar-compatible transport
probe and is not the production BFF. The Node BFF remains the production
browser boundary until later slices implement the complete cookie, CSRF, DTO,
NDJSON redaction, and media contracts.

## Local run

```bash
CORE_BASE_URL=http://127.0.0.1:8080 \
FLUCTLIGHT_CORE_SERVICE_KEY=replace-me \
go run ./cmd/gateway
```

The process listens on `0.0.0.0:3001` by default. Set
`GATEWAY_LISTEN_ADDR` to override it. The only exposed routes are:

- `GET /health/live`
- `GET /health/ready`
- `GET /api/platform/ping`

The gateway does not connect to PostgreSQL, Redis, S3, or Temporal and does
not write domain state. Do not route production traffic to it until a later
task proves browser-contract parity and adds an explicit rollback path.
