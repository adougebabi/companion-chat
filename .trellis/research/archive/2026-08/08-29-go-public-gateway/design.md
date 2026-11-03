# Technical Design

## Boundary

The Go process is a sidecar-compatible public transport probe. It is not the
production BFF yet and it has no domain ownership. The existing Node BFF stays
on the production path until a later compatibility slice covers cookies,
CSRF, browser DTOs, streaming redaction, and media authorization.

## Runtime

Use the Go standard library only:

- `cmd/gateway` owns process startup and signal-aware HTTP server shutdown.
- `internal/config` parses and validates `GATEWAY_LISTEN_ADDR`, `CORE_BASE_URL`,
  and `FLUCTLIGHT_CORE_SERVICE_KEY`.
- `internal/gateway` owns the public route allow-list and the downstream Core
  client.

The default listen address is `0.0.0.0:3001`. `CORE_BASE_URL` must be an
absolute HTTP(S) URL. The service key is required and must be non-empty. The
HTTP client has a bounded timeout for readiness and ping calls; requests do not
share a mutable body or global state.

## Routes

| Public route | Downstream call | Behavior |
|---|---|---|
| `GET /health/live` | none | Return `200 {"status":"ok","role":"go-public-gateway"}` |
| `GET /health/ready` | `GET /health/ready` | Return ready envelope on 2xx; return 503 on any downstream/transport failure |
| `GET /api/platform/ping` | `GET /internal/platform/ping` | Forward Core status and JSON body; attach `x-fluctlight-service-key` only to downstream |

The gateway must not implement a generic path proxy in this slice. This keeps
the public allow-list explicit and prevents accidental exposure of Core's
internal API.

## Error Contract

Health endpoints return a small stable JSON object. Readiness failures return
`503` with `{"status":"unavailable","role":"go-public-gateway"}`. Ping
errors return `502` with `{"code":"core_unavailable","message":"Core platform is unavailable"}`;
an upstream non-2xx response is returned with its status and bounded JSON body.

## Test Seams

The gateway accepts an injected `http.Client` or `RoundTripper` in tests and
uses an `httptest.Server` fake Core. Tests assert that the service key is only
present on the downstream request, that no arbitrary path is proxied, and that
readiness does not report success after a Core failure.
