# Kev Decision Contract

## 1. Scope / Trigger

Kev gates eligible WakeUp, Reflection, Goal assessment/replenishment, permitted
Tool disclosure, declared persistent persona conditions and optional context.
This is a selection adapter over existing execution, not an Agent/loop replacement.
Kev migration is `0056_kev_decisions`, additive from `0055_goal_planner_cadence`. Current head `0057_logical_agent_leases` serializes mutable logical decisions while retaining physical per-call queue permits.

## 2. Signatures

- `decision.Service.Decide(ctx, point, scope, state, candidates) ([]Result,error)`.
- `Result.Admit(ctx) (Outcome,error)` and `Result.Finish(...) error`.
- `POST /v1/systemone`: state string, questions map, answers map.
- Owner settings key `kev`; encrypted credential purpose `kev:systemone`.
- `GET /api/diagnostics/kev-decisions` / equivalent internal route returns
  `{items,next_cursor,snapshot}`; filters are explicit snake_case fields.
- `POST /api/settings/kev/test-connection` with `{}` is a manual no-business test.
- `kev-config --endpoint <actual reachable URL> [--disable]` uses Owner settings.
- Tables: `kev_decisions`, `kev_deferrals`; `runtime_settings.revision` increments
  in the original atomic settings upsert.
- `FLUCTLIGHT_KEV_SPOOL_DIR` defaults to `/var/lib/fluctlight/kev`; Core/Worker
  have separate named volumes. Each process imports its own journal.

## 3. Contracts

Disabled points do not build optional candidates or call Kev. Fallback remains
a lazy original branch, never a false/no action. Parent cancellation ends the
business request; only child timeout/invalid response/budget failures fall back.
Check parent cancellation after settings I/O and recheck config version before
adoption. Record actual choice/confidence/distribution separately from application.
A stored model yes stays yes after later disable or audit rejection.

Defaults: choice_argmax, 1500ms request, 3000ms shared selection stage, 1 concurrent
request per service instance, 8 questions/batch, zero retries, 3 deferrals, 60s
backoff and 7-day retention. Missing installed settings leave the original path;
the explicit local/operator configuration command enables all seven points.
Do not write production endpoint or silently overwrite Owner settings in migration.

Complete raw response and each question's raw answer must be durably stored before
adoption. Use a bounded fsync append journal if PostgreSQL diagnostic writes fail;
if both fail, use original and warn. Import by stable identity/recorded_at so an
older spooled observation cannot overwrite a newer application result. Process
locks, incomplete-tail quarantine and capacity limits are mandatory; import never
replays business effects. Retention deletes only bounded diagnostic batches.

Automatic no preserves original cycle/evidence watermarks and execution attempts.
WakeUp/Reflection defer via native durable workflow timers; Goal flows retain
retry/available_at and deferred subsets. Maximum deferral re-enters original after
hard eligibility. Manual WakeUp overrides a matching cycle and signals
kev.force_original to interrupt its timer. Old workflow histories lacking the
new deferred branch re-run the unexecuted candidate through original, rather than
terminally losing it.

Tool discovery is an independent pure query (`read_only`) and returns only the
current Agent's original installation when run-scoped. Validate the complete load
request before changing visibility. The full authorized executor table is fixed;
v0.7.37 `model.WithTools` call options supply the actual visible schema per physical
request. Match physical budget/diagnostics to that schema. Selection happens at
prompt assembly; invocation cannot select against a stale already-built prompt.

Persona admission requires the original installed persona.switch and a refresh
plan before any side effect. A semantic yes uses the original independent Tool,
with expected persona revision and real direct identity, then discards the entire
A batch and regenerates B within the existing model adapter/native Runner. No
synthetic native IDs or Tool successes. No blocks later same-rule native switches;
only one persistent switch completes per run. Required context/goals/history and
committed prior effects remain authoritative.

Nested model components inherit Eino callbacks. During the gated proposal call,
use request-scoped `callbacks.InitCallbacks` to prevent the rejected A proposal
from entering Agent events/ExecutedToolCalls. Keep physical raw diagnostics in the
existing adapter; leave global callback registration unchanged. Streaming proposals
are bounded and consumed before release. Test both Generate and Stream.

## 4. Validation & Error Matrix

| Condition | Behavior |
| --- | --- |
| Global/per-point disabled | zero Kev HTTP, original schemas/context/rules |
| Parent cancellation during settings/model call | cancelled error, no fallback work |
| Unknown choice, invalid distribution, missing answer | original for affected question |
| Invalid envelope/unknown question ID/over-limit response | batch original; retained bounded evidence labelled |
| Config changed after response | original_used/config_changed or discarded_after_disable |
| Database and bounded spool unavailable | original, audit_unavailable, safe warning |
| Tail torn by crash | quarantine unconfirmed tail; preserve complete rows |
| Discovery requests installed + foreign names | reject whole load; no partial expansion |
| Persona gate lacks install/refresh | do not install or commit |
| Stale persona | refresh and regenerate against current authority |
| Old A nested callback | excluded from loop; retained only in private diagnostic |

## 5. Good / Base / Bad Cases

Good: missed Tool is discovered, next real HTTP schema grows, a real result returns
through its native ID. Good: Kev yes commits one authorized switch before B reply.
Base: timeout leaves the original pipeline available. Bad: record yes but always
run the original judge; mark a no candidate processed; call hidden tools without
loading; copy an A native call ID onto a direct switch; advertise A as executed.

## 6. Tests Required

`TestKev*` covers seven point policies, disabled/cancel/timeout/limits/partial
questions, config revoke, audit failure, required-source preservation, journal
capacity/torn tail, scoped discovery and actual Generate/Stream loop feedback.
`TestKevNativePersonaLoopExecutesOnlyRegeneratedB` proves A never executes or leaks
into the loop's ToolCalls. Temporal tests prove manual interruption and durable
waiting. Fixed Tool inventories and API route matrices include discovery and Kev
routes. Database/real-model acceptance remains separate; SKIP is not PASS.

## 7. Wrong vs Correct

Wrong: innerModel.Generate(ctx) publishes A via inherited callbacks, then outer
wrapper discards A; the loop still reports its calls as executed.
Correct: isolate callbacks only for the gated inner proposal, save original output
through Core diagnostics, return the admitted response through the unchanged outer
Runner callback contract.
