# Execution plan

## 1. Establish failure fixtures

- [x] Capture or reproduce each reported error with sanitized Agent/Tool/Provider diagnostics. For `schedule_replan_intention_link_unknown`, compare planner output with accepted schedule; for malformed JSON, inspect final Content and finish reason.
- [x] Add focused failing tests for planner invented/missing action links, missing capability request, both governance entry paths, invalid refs and absent evidence list, malformed final JSON, and two `conversation.reply` calls in one turn.

## 2. Schedule capabilities

- [x] Implement bounded `schedule.inspect` with stable item identity and current-day state; register in appropriate surfaces and catalog tests.
- [x] Implement targeted move/revise/cancel via immutable schedule replacement and linked Intention safeguards. Verify current/future boundaries, CAS, active-run rejection, trigger reconciliation, and cancellation.
- [x] Repair planner link handling for `intention.schedule` and `schedule.replan` without trusting model-authored new IDs. Preserve accepted schedule on rejection and return a more specific diagnostic.

## 3. Capability request closure

- [x] Make absent capability intent route to `capability.request`; handle an actual native unknown ToolCall as non-executed and idempotently record a bounded need where the authenticated source is available.
- [x] Make both governance entrances load the Owner pool; add explicit loading, empty, and failure rendering plus the fields needed to implement the requested capability.
- [x] Verify Core DB/outbox, Browser DTO, client/store and UI contract together.

## 4. Agent final-output reliability

- [x] Preserve final physical finish reason and useful bounded structured-parse diagnostics; classify errors by the actual boundary.
- [x] Align appraisal evidence-list semantics and add a bounded correction path for malformed/schema-invalid final Content, without Tool replay.
- [x] Validate dynamic refs only against the frozen current-run index; test alias decode, projection refresh and influence correction/failure.
- [x] Add Provider role preflight coverage for required structured output/tool calling where the configured endpoint allows meaningful verification; handle unsupported providers explicitly (model availability preflight in place; active schema probe deferred per PRD).

## 5. Reply publication idempotency

- [x] Add turn-level publication check/serialization inside the existing transaction path. Return replay or typed duplicate result as appropriate; retain unique index.
- [x] Verify two native reply calls, retries with equal/different OperationIDs, natural fallback, and concurrent publication produce at most one assistant message.

## 6. Integration and review

- [x] Run focused Go core tests, browser route tests and web tests, then repository-required lint/typecheck/build. Include race/integration tests for publication and schedule mutation when the local test environment supports them.
- [x] Review full model → Tool → Core → DB → Browser paths and ensure all added capabilities appear in the production catalog.
- [x] Reconcile `.trellis/spec/` contracts where current documents disagree with implementation, especially Provider structured-output parsing and missing capability semantics.
- [x] Inspect the final diff against the initial dirty-state snapshot; keep unrelated user edits out of this task's commit plan.

## Risk checkpoints

- After schedule work: confirm immutable versions and linked Intention invariants before touching UI integration.
- After final-output correction: confirm an already committed Tool is never invoked twice and unknown refs remain rejected.
- After reply work: confirm the database unique index stays in place and serial/concurrent cases are stable.
