# Execution plan

1. Freeze the current route/OpenAPI and baseline product regression evidence;
   record the capability matrix for outbox, Redis consumer, Temporal registry,
   dispatcher and reconciliation.
2. Add the Go Redis transport dependency behind a small platform interface;
   implement bounded publisher claim/publish/settle behavior and tests for
   duplicate/retry/crash windows.
3. Implement durable consumer groups with inbox/effect/head transactions,
   `XAUTOCLAIM`, poison failure records, bounded batches and stream retention.
4. Add cognition-processing and platform-control workflow registrations,
   deterministic replay tests, stable ID normalization and queue ownership
   assertions.
5. Move committed cognition/event processing triggers behind Worker intents;
   preserve the public conversation response and do not alter baseline routes
   unless the shared seam requires it.
6. Harden dispatcher/reconciliation and add integration tests for Temporal
   accepted-start/DB-failure, restart, duplicate delivery and terminal mapping.
7. Run focused platform tests plus Core/Gateway race/vet/build/OpenAPI guards.
   Rerun baseline product cases only if shared product code or deployment graph
   changed; the user owns final baseline acceptance when it is unaffected.
8. Run a final capability-matrix review, update specs and regression evidence,
   commit, and archive only when every required row is green.

Validation commands:

```bash
GOCACHE=/tmp/go-core-worker-closure go -C apps/core-go test -race ./...
GOCACHE=/tmp/go-core-worker-closure go -C apps/core-go vet ./...
GOCACHE=/tmp/go-core-worker-closure go -C apps/core-go build ./...
GOCACHE=/tmp/go-core-worker-closure go -C apps/gateway-go test -race ./...
bash infra/acceptance/check-core-openapi.sh
bash infra/acceptance/go-core-reference-guard.sh
```

Required rollback points: after each platform slice, keep the previous Go
Worker image runnable; never reset PostgreSQL/Redis/MinIO/Temporal volumes.
