# Initialization recovery and durable preparation

## 1. Scope / Trigger

Character activation, accepted Foundation changes, creation validation failures,
and the 2026-10-23 request to serialize initialization and set LLM concurrency one.
No prompt/model semantics are owned by this contract.

## 2. Signatures

- `AdmitFluctlight` / `AdmitFoundationDecisionExpected` persist source and a
  `persona.compile` intent atomically; synchronous maintenance wrappers remain.
- `ProcessPersonaPreparationIntent(ctx,id)` runs the existing compiler outside
  a transaction, then publishes under source/lifecycle/habits/overlay/budget CAS.
- `POST /api/fluctlights/:fid/persona-preparation/recover` accepts
  `{expectedRevision: nonnegative integer,idempotencyKey: string}`.
- Detail `persona_preparation` exposes pending/running/failed/ready, revision,
  durable intent identity and can_recover.
- Schema head `0101_serial_llm_queue`, previous `0100_kev_timeout`.

## 3. Contracts

Current-source compilation intents permit API/Worker readiness while pending or
failed. A completed job does not excuse a stale or missing compiled portrait.
Failed compilation retains the already saved source. Explicit recovery creates
one new intent; unknown command outcomes replay their immutable body/key.

The dispatcher filters prerequisites before ranking/LIMIT, scoped to creations
with an initialization persona intent: persisted world binding → persona compile
→ wardrobe completed → visual reference → character sheet/session completed →
canonical display-avatar projection → initial Schedule → WakeUp. Reference and
sheet dependencies remain in the visual owner. Routine later Schedule jobs do
not reacquire bootstrap dependencies. Status reads may run concurrently.

Queue defaults remain generated=1, embedding=1. The additive migration lowers
previously saved queue limits to one once, preserves other keys and increments
configuration revision. Head reruns preserve later explicit Owner edits.

## 4. Validation & Error Matrix

| Case | Required result |
| --- | --- |
| Malformed or incomplete analysis | Keep raw input editable, show exact field path; no activation |
| First activation 400/422 | Return to input, invalidate rejected analysis, permit fresh review |
| Activation response lost then later rejection | Retain original command identity; never silently start a second activation |
| Source revision changes during compile | Supersede result; do not publish obsolete portrait |
| Retirement during model call | Do not publish portrait or admit recovery |
| Any bootstrap prerequisite fails | Keep successors queued; expose original stage recovery |
| Recovery with stale expectedRevision | Conflict before creating another intent |

## 5. Good / Base / Bad Cases

Good: source saves immediately, compiler fails, Owner recovers the same character.
Base: blank admission uses a deterministic initial portrait with no model call.
Bad: unlock another activation after a transport loss, or dispatch initial
Schedule while its visual predecessor is still running.

## 6. Tests Required

`persona_preparation_test.go`: real disposable PostgreSQL proves admission/replay,
deferred save, failure preservation, revision conflict/recovery, supersession and
retirement during compile. `creation_order_test.go` executes prerequisite SQL
against migrated PostgreSQL and Temporal activity retry. `serial_llm_queue_test.go`
covers one-time config reset and later Owner edits. Approved creation tests cover
malformed → missing-field → corrected-input success, definitive versus unknown
activation rejection, raw JSON repair, archive isolation and legacy migration.

## 7. Wrong vs Correct

Wrong: leave a failed creation on a progress screen with only a retry button;
repeat the rejected cached JSON. Correct: known rejection returns to editable
source with a field path, while an unknown activation retains its frozen identity.
