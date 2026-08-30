# Execution plan

1. Freeze and reconcile the Python Core OpenAPI; add cross-language contract
   fixtures and a small Go Core module skeleton with health/config tests.
2. Implement the Go Core transport/application slice and repository seam; add
   Docker/Compose/CI wiring without changing ownership of unmigrated tables.
3. Fix Python compound-effect pre-validation and deterministic settlement;
   add regression tests for invalid siblings, retry, and media idempotency.
4. Fix activation/create lifecycle registration, direct-conversation target
   ordering, and current-day review intent transactionality; add tests.
5. Fix reflection schema validation/typed errors/watermark ordering and add
   valid/malformed/duplicate/retry fixtures/tests.
6. Fix Worker dispatcher persistence, bounded limit, Temporal reuse policy,
   reflection debounce, and independent queue budgets; add restart tests.
7. Run all static/unit/integration/build/Compose checks. Rebuild the real Docker
   stack only as needed; preserve volumes and do not restart Worker manually as
   part of acceptance.
8. Execute real regression cases 1–7 through the public browser/BFF contract,
   record evidence under this task, update specs with durable lessons, then
   commit the stage.

Rollback points: after each numbered step, the preceding service remains
runnable. Never introduce a second writer or alter released migrations.
