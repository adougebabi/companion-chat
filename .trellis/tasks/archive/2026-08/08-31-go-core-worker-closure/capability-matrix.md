# Go Core/Worker platform capability matrix

This matrix is the closure gate for this task. A row is green only when the
implementation, focused test and runtime evidence all exist. Baseline product
flows are referenced rather than rerun unless their shared seam changes.

| Capability | Implementation | Focused test | Runtime evidence | Status |
| --- | --- | --- | --- | --- |
| PostgreSQL outbox claim/lease | `internal/platform/redis_pipeline.go` | publisher integration | Compose DB/Redis run | PASS |
| Redis publisher/retry/terminal failure | `internal/platform/redis_pipeline.go` | miniredis + PostgreSQL | Compose DB/Redis run | PASS |
| Durable consumer inbox/effect/head | `internal/platform/redis_pipeline.go` | duplicate integration | three groups, pending 0 | PASS |
| XAUTOCLAIM/reclaim | `internal/platform/redis_pipeline.go` | reclaim integration | Compose DB/Redis run | PASS |
| Poison/failure acknowledgement | `internal/platform/redis_pipeline.go` | poison integration | Compose DB/Redis run | PASS |
| Stream trim bound | `internal/platform/redis_pipeline.go` | PEL-safe trim code path | worker runtime | PASS |
| Cognition processing workflow | `internal/workflow/workflow.go` | registry/replay test | fresh intent completed | PASS |
| Platform control workflow | `internal/workflow/workflow.go` | registry/replay test | signal stop test | PASS |
| Queue-specific workflow ownership | `StartWorkers` registration | static registration + logs | three queue owners | PASS |
| Cognition durable intent | `core/cognition.go` + dispatcher | intent replay test | fresh intent completed | PASS |
| Dispatcher accepted-start recovery | `internal/workflow/workflow.go` | reconciliation path | pending/started terminal scan | PASS |
| Baseline product scenarios | previous task evidence | rerun only if impacted | already recorded | baseline |
