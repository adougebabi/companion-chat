# Go Core/Worker platform regression

## Focused checks (2026-08-31)

The closure branch was deployed to the existing Compose project
`fluctlight-t12-browser-53392`; PostgreSQL, Redis, MinIO and Temporal volumes
were preserved.

| Check | Evidence | Result |
| --- | --- | --- |
| Go Core/Worker race/vet/build | `go test -race ./...`, `go vet ./...`, `go build ./...` | PASS |
| Redis publisher/consumer unit | miniredis `TestEventEnvelopeRoundTrip` | PASS |
| PostgreSQL + Redis publisher/consumer | Compose-network `TestRedisConsumerPublishesAndDeduplicates` | PASS |
| Poison handling | Compose-network `TestRedisConsumerRecordsPoisonEvent` | PASS |
| Pending reclaim | Compose-network `TestRedisConsumerReclaimsPendingDelivery` | PASS |
| Workflow registry | `TestWorkflowFunctionRegistryIncludesPlatformBoundaries` | PASS |
| Platform control replay | `TestPlatformControlWorkflowStopsOnSignal` | PASS |
| Worker queue health | three Go workers, BuildID `platform-v1` | PASS |
| Redis durable groups | `bff-notifications`, `cache-projections`, `integration-observers`; pending 0, lag 0 | PASS |
| Outbox backlog | `platform_outbox_events` eligible unpublished/failed count 0 | PASS |
| Cognition durable intent | fresh public turn created `cognition.processing`; intent and inbox reached completed/processed | PASS |
| Event fan-out | fresh cognition outbox event published and observed once by all three groups | PASS |

The existing product baseline is intentionally not repeated here because this
closure branch does not alter those product handlers. The previous task's
1–7 evidence, including the post-v30 fresh image run, remains the baseline
acceptance record.
