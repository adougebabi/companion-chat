---
{"id":"legacy-d3b38a8f7acafd2c78695715","title":"design.md","kind":"research","status":"historical","paths":[],"source":".trellis/research/archive/2026-08/08-29-go-platform-foundation/design.md#sha256=0730b4f3d23dbd72ea73b653e855af761b38d75c06a1b51e28ce2d1995244947","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# Technical Design

## Package Boundary

`apps/gateway-go/internal/platform` owns transport-neutral process health
values. It does not know HTTP routes, Core URLs, cookies, or domain modules.
The BFF adapter maps platform values to JSON and owns the downstream Core
probe.

```text
platform.Health
  ├─ BFF /health/live
  └─ BFF /health/ready ← platform.Probe(Core readiness)
```

## API

```go
type Role string
const (RoleBFF Role = "bff"; RoleCore Role = "core"; RoleWorker Role = "worker")

type Health struct { Status string; Role Role }
func Live(role Role) Health
func Ready(role Role) Health
type Probe func(context.Context) error
func IsReady(ctx context.Context, probe Probe) bool
```

The module returns values only; it does not write an HTTP response. This keeps
the core module testable without a socket and allows a future Core/Worker
process to reuse the same health contract.

## Compatibility

The public BFF continues to return `{status, role}` with the role `bff`. A Core
readiness failure remains HTTP 503. Liveness never invokes the Core client.

