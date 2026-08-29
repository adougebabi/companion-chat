# Technical Design

## Compatibility Boundary

The Node BFF is the executable behavior oracle. The Go implementation keeps
the same browser paths and response contract while calling the same Python
Core `/internal/*` paths. Until parity tests pass, Node remains the production
entrypoint and Go runs on an isolated port/profile.

## Package Layout

```text
apps/gateway-go/
  cmd/gateway/             process startup
  internal/config/         environment and cookie/security settings
  internal/bff/             HTTP boundary and route registry
    core_client.go          Core HTTP transport and error decoding
    auth.go                 cookie, CORS, Origin/CSRF helpers
    routes.go               explicit public routes and body/query mappings
    dto.go                  browser-specific response mappings
    ndjson.go               incremental stream translator
    media.go                bounded streaming media proxy
```

The existing health/ping implementation can be moved behind the BFF server
package. The package remains standard-library-only so the first runtime
comparison measures Go itself rather than a framework baseline.

## Request Lifecycle

1. The outer handler applies CORS and issues a CSRF cookie when absent.
2. A route-specific validator checks method, body/query/path bounds, then
   checks Origin/CSRF and resolves the opaque session cookie where required.
3. A route-specific mapper builds a Core request with the service key and, when
   applicable, the human session header.
4. The Core response is mapped to the browser DTO or streamed through the
   NDJSON/media boundary. Core storage and workflow details never cross this
   package.

## Route Strategy

The implementation uses an explicit route table and handler functions rather
than a generic public-to-internal path proxy. This is necessary because the
current BFF has per-route validation, error mapping, body conversion and DTO
rules. Generic helpers may share transport mechanics, but each route declares
its Core path and mapping explicitly.

The route inventory is the current `packages/browser-client/src/index.ts` plus
the dynamic workflow and moment action routes in `apps/bff/src/app.ts`. The Go
tests use a fake Core that records method, URL, headers and JSON body, making
the mapping observable without a database or provider.

## Security and Cookies

- `fluctlight_session`: HttpOnly, SameSite=Lax, Path=/, Secure according to
  trusted HTTPS origin, no server-side session cache.
- `fluctlight_csrf`: readable by browser JavaScript, SameSite=Lax, Path=/,
  Max-Age=86400, random 32-byte base64url value.
- Mutations require exact trusted Origin, both CSRF values, equal byte length,
  and constant-time equality. Login/setup/logout retain their current
  session-optional rules.
- CORS headers are added only for exact trusted Origin; preflight allows the
  current methods and `content-type,range,x-csrf-token` headers.

## Stream and Media Ownership

`ndjson.go` must decode arbitrary byte chunks incrementally, reject malformed
Core events, redact by rejecting recursive hidden keys, map action results and
stop after the first terminal event. Request context cancellation cancels the
Core request and suppresses later writes.

`media.go` must copy the Core body directly to the browser response and only
forward the current allow-listed headers. It must never read the whole body or
expose object-storage identifiers.

## Rollback

The production Compose service remains Node until the full parity suite and
disposable smoke pass. Switching is a deployment-only change after review; a
failed smoke restores the Node image/command without any data migration or
dual-write cleanup.
