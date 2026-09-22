# Remaining independent Tool E2E report

Date: 2026-09-22

## Scope and fixed expectation

The new suite is `apps/core-go/internal/core/independent_tools_e2e_test.go` and every test name starts with `TestIndependentToolE2E`. It calls the production `App.ExecuteTool` boundary against a per-test disposable PostgreSQL database created by `isolatedCoreTestRepository`. It does not start Main, does not install `StaticContextResolver`, and does not construct a fake cognition/persona state.

The expectation is a literal product baseline, not a value generated from the current registry:

1. `active_memory_event`
2. `affect_event`
3. `capability.request`
4. `conversation.reply`
5. `media.image.generate`
6. `memory.recall`
7. `memory_event`
8. `moment.publish`
9. `persona.switch`
10. `persona.takeover`
11. `presence_event`
12. `relationship.lookup`
13. `scene_event`
14. `schedule.replan`
15. `visual_identity.initialize`

`TestIndependentToolE2EFixedProductInventory` compares that fixed list with the formal registry and separately asserts the seven Tool names owned by this slice. This prevents a removed registration from silently reducing the expected test population.

## Matrix implemented

| Tool | Real success and product proof | Business failure | Repeat semantics | Dependency failure | Result |
| --- | --- | --- | --- | --- | --- |
| `scene_event` | Direct request with no native ToolCall ID; `life_events`, `life.scene.updated` outbox and native fact independently queried | A second `start` while an Event is active is rejected | Same operation with a new native correlation replays one event; changed payload conflicts | Capability service removed from the injected formal registry | PASS |
| `presence_event` | Direct request; overlay actor/task/presence/revision, outbox and native fact independently queried | Expired requested presence is rejected | Same operation replays one overlay; changed payload conflicts | Capability service removed | PASS |
| `schedule.replan` | Formal `providerSchedulePlanner` uses the configured real Provider, then PostgreSQL proves accepted revision 2, nonempty items and one accepted schedule | Empty intent is rejected before Provider execution | Successful run proved replay with a different native correlation and payload conflict without a second business write | Planner removed | Real success observed; final rerun externally blocked, details below |
| `affect_event` | Direct request; state event, inner-state revision and `affect.updated` outbox queried | Unknown affect type is rejected | Same operation replays one event; changed payload conflicts | Affect service removed | PASS |
| `relationship.lookup` | Real row seeded and result compared with PostgreSQL authority | Unauthorized target is rejected | Pure query repeats without a `tool_executions` ledger row | Lookup service removed | PASS |
| `capability.request` | Direct request; proposal row, source evidence and outbox queried | Sensitive desired contract is rejected | Same operation replays one request; changed payload conflicts | Request service removed | PASS |
| `visual_identity.initialize` | Direct request returns `accepted`; queued session, attempt, timeline and lifecycle workflow intent independently queried | Wrong Owner scope is rejected | Same operation replays one session; changed target binding conflicts | Visual Identity service removed | PASS at accepted boundary |

Every mutating Tool uses the same six named subcases: direct success without native ID, durable product, business failure, operation replay with a new native correlation, same-operation conflict, and isolated dependency failure. `relationship.lookup` has the query-equivalent six cases: direct query, authority comparison, forbidden target, repeat-without-ledger, optional native correlation, and dependency failure.

The native IDs supplied in replay/query cases are only correlation checks. They are not described as Eino adapter evidence. Formal Eino adapter construction and real Eino ToolCall identity remain covered by their owning adapter/Agent suites.

The dependency cases remove the corresponding production dependency from the formal capability instance. They are failure injection only; every positive case uses the real App resolver, domain implementation and PostgreSQL services.

## Commands and evidence

All private environment values were loaded by `/tmp/lac-run-tool-test.py` from the task-owned private environment and were not printed. Each run records Core JSON output plus commit/diff metadata under `research/runs/`.

| Run | Selector | Exit | Evidence / interpretation |
| --- | --- | ---: | --- |
| `remaining-tools-e2e-initial` | `^TestIndependentToolE2E` | 1 | Deliberate first run exposed the production gates listed below. It also proved all six `affect_event` and `capability.request` cases already passed. |
| `remaining-tools-e2e-db-rerun` | all non-schedule rows | 1 | Production fixes worked; only the scene business-failure fixture was wrong because the preceding success had created an active Event. The fixture was corrected to attempt a second `start`. |
| `remaining-tools-e2e-scene-final` | `^TestIndependentToolE2ESceneEvent$` | 0 | All six scene cases passed. |
| `remaining-tools-e2e-schedule-retry` | `^TestIndependentToolE2EScheduleReplan$` | 1 | Real Provider direct success passed in 23.88 s; PostgreSQL showed accepted revision 2 with two items and one accepted schedule. Business failure, replay, conflict and dependency cases all passed. The suite failed only because the product assertion expected exactly one diagnostic row while the formal Agent recorded two physical `schedule_replan_planner` model runs. The assertion now accepts one or more physical planner runs and still requires zero unrelated/Main scenarios. |
| `remaining-tools-e2e-final` | `^TestIndependentToolE2E` | 1 | All database-only rows passed. The real Provider rejected schedule planning with HTTP 400 because the prompt needed about 1248 MB GPU memory and only about 1198 MB was available. No schedule write was accepted. |
| `remaining-tools-e2e-schedule-final-2` | schedule only | 1 | A second preserved attempt was also rejected by the real Provider because only about 324 MB GPU memory was available. The test correctly remained failed and created no false success. |
| `remaining-tools-e2e-db-final` | fixed inventory plus all six non-schedule tools | 0 | Final current-code PostgreSQL suite passed: fixed list plus 36 Tool subcases. |

Primary evidence files:

- `research/runs/remaining-tools-e2e-db-final.jsonl` and `.meta.json`
- `research/runs/remaining-tools-e2e-scene-final.jsonl` and `.meta.json`
- `research/runs/remaining-tools-e2e-schedule-retry.jsonl` and `.meta.json`
- `research/runs/remaining-tools-e2e-final.jsonl` and `.meta.json`
- `research/runs/remaining-tools-e2e-schedule-final-2.jsonl` and `.meta.json`

The schedule row has real success evidence, including the formal Provider call and independent PostgreSQL product checks, from `remaining-tools-e2e-schedule-retry`. The current final complete selector is not reported as passing because the later external Provider attempts failed. A later acceptance run must rerun `^TestIndependentToolE2EScheduleReplan$` after Provider GPU capacity is available and must finish with exit 0.

## Production gates discovered and fixed by the owning main thread

The initial failing run is retained rather than overwritten:

- `scene_event` failed with `scene_frozen_action_required`, and `presence_event` failed with `presence_frozen_action_required`. The direct application boundary has a business operation ID but no model Action ID. Current `life_capability_plans.go:61-108` and `:111-145` prepare from validated arguments, actual resolved life authority, the operation ID and the validated authorization actor instead of requiring fabricated Main state.
- `visual_identity.initialize` was classified as `external_async_intent`, but `ExecuteTool` handled only query/transactional implementations and returned `direct_execution_not_implemented`. Current `tool_execution.go:180-186` runs both transactional mutations and external async intents through their real transactional implementation.
- Visual initialization returned `completed` while its session and workflow were queued. Current `builtin_capabilities.go:483` and `:499` return `accepted` with the real queued session ID. The E2E independently verifies the queued session/attempt/timeline/workflow intent and does not claim a final visual asset; final image/canonical completion belongs to the separate visual workflow acceptance slice.
- `relationship.lookup` scanned the PostgreSQL text summary into `[]byte` and exposed a byte array in Tool output. Current `relationship_capability.go:111-121` scans it into a string; the E2E compares the exact string with the database authority.
- `tool_execution.go:129-138` now carries the already validated authorization actor in invocation metadata. This lets Presence bind a real application actor without inventing a context-reference snapshot.

## Remaining limitation

The six database-only Tool suites and fixed-list guard are green on current code. `schedule.replan` has one complete real success run and subsequent correct replay/conflict/failure behavior, but the last two current-code attempts were blocked by real Provider GPU capacity. Therefore this report does not mark the complete seven-Tool selector as finally green. No assertion was weakened to treat Provider failure, queued work, or a missing product as success.
