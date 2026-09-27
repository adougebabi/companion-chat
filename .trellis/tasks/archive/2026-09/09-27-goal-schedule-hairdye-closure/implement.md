# Implementation

1. Add isolated PostgreSQL regression tests for due-fact Worker routing, planned dye acceptance, before/after due execution, completion/failure, and chat context truth.
2. Add migration `0038` and typed link fields to schedule item storage/read/DTO, preserving old rows and immutable versions.
3. Implement the planning Tool with Core-owned Goal/Intention qualification and schedule acceptance in one mutation; validate one linked item and planner output.
4. Implement scheduled Intention due execution and durable activity resolution in the IntentionTrigger Workflow, with restart/replay/stale-plan guards.
5. Add `hair_dye` start/result/settlement and strengthen private conversation state guidance; verify direct style cannot substitute for dye.
6. Update OpenAPI/generated clients, specs and tests; run Go, TypeScript, migration, isolated PostgreSQL and Temporal workflow gates.

Risk points: versioned schedule replacement, Intention revision/trigger workflow replacement, Temporal history replay, Tool idempotency, and truthfulness of current Life Context. Do not call a Provider inside a database transaction. Fail closed when the planner cannot produce a valid linked slot.
