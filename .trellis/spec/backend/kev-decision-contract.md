# Kev Decision Contract

2026-10-14 availability amendment: apply [Tool Availability](./tool-availability-contract.md).
Older execution-denial wording for Kev No, config revocation and disclosure-window
replacement is superseded for installed current-scope Tools. Keep raw decisions
and disclosure history, expose warning_code, and preserve actual installation and
resource authority. Querying a name does not grant another private surface.

## 1. Scope / Trigger

Kev gates eligible WakeUp, Reflection, Goal assessment/replenishment, permitted
Tool disclosure, declared persistent persona conditions and optional context.
This is a selection adapter over existing execution, not an Agent/loop replacement.
Kev migration is `0056_kev_decisions`, additive from `0055_goal_planner_cadence`. Current head is `0060_media_kev` (additive media audit-point constraint); `0057_logical_agent_leases` serializes mutable logical decisions while retaining physical per-call queue permits.

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

Disabled points do not build optional candidates or call Kev. Except for public Tool disclosure (three-tool baseline), fallback remains
a lazy original branch, never a false/no action. Parent cancellation ends the
business request; only child timeout/invalid response/budget failures fall back.
Check parent cancellation after settings I/O and recheck config version before
adoption. Record actual choice/confidence/distribution separately from application.
A stored model yes stays yes after later disable or audit rejection.

Defaults: choice_argmax, 1500ms request, 3000ms shared selection stage, 1 concurrent
request per service instance, 8 questions/batch, zero retries, 3 deferrals, 60s
backoff and 7-day retention. Missing installed settings leave the original path;
the explicit local/operator configuration command configures ten points (seven original plus three media).
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
request. Match physical budget/diagnostics to that schema. Public schema state is established during assembly even if Kev is disabled. Named catalog requests start a fresh bounded selection stage; every physical request and native execution rechecks that state/configuration.

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
| Global/per-point disabled | zero Kev HTTP; public schemas use baseline, other context/rules use original |
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


## Scenario: Batch-local state and deadline cause (2026-10-10)

### 1. Scope / Trigger
Context/tool selection split questions at eight but repeated all candidate contents. A shared stage deadline expiring during HTTP was labelled request_timeout.

### 2. Signatures
`DecideWithBatchState(ctx,point,scope,candidates,stateForBatch func([]Candidate) string)` constructs the physical state after batching. Existing Decide sends its supplied state unchanged. Core context/tools callers use typed batch builders.

### 3. Contracts
Each context batch contains only that batch's candidate map; each tool batch only that batch's capability descriptions. Current task input, IDs/order, required context, authorization and Admit/Finish semantics remain unchanged. No generic JSON-key inference, proof truncation, parallel chunk execution or raised timeout default. Parent cancellation still terminates business work.

Shared stage and per-Decide deadlines stay bounded. When budget is exhausted, distinguish stage_budget_exhausted from budget_exhausted; an earlier request-only deadline remains request_timeout. Apply classification both during HTTP Do and response-body reads. duration_ms remains local queue+HTTP/body time, not server inference. External latency and actual throughput are not verified by mock HTTP.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Stage deadline during request/body | timeout / stage_budget_exhausted |
| Per-Decide deadline | timeout / budget_exhausted |
| Earlier single request deadline | timeout / request_timeout |
| Parent cancelled | cancelled / request_cancelled and propagated error |
| Generic Decide caller | state unchanged across batches |

### 5. Good / Base / Bad Cases
Good: eleven optional candidates produce disjoint 8/3 candidate states and preserve current input. Base: one batch retains all its candidates. Bad: three questions still carry eleven candidate bodies or report stage expiry as a configured request timer.

### 6. Tests Required
TestKevBatchStateProjectionAndGenericStateCompatibility captures 8/3 requests and caller immutability. TestKevContextAndToolBatchesSendOnlyTheirCandidateState checks real Core call sites. TestKevTimeoutReasonsDistinguishRequestStageBudgetAndParentCancellation and TestKevResponseBodyTimeoutKeepsDeadlineCause verify fallback and cancellation classification.

### 7. Wrong vs Correct
Wrong: only split questions, resend unrelated candidate content, extend the timeout to hide input cost.
Correct: explicit typed per-batch projection and actual deadline-cause diagnostics; measure external inference separately.


## Scenario: Agent-owned mandatory tools and recovery directory (2026-10-10)

### 1. Scope / Trigger
Kev narrows an Agent's authorized Tool installation. Mandatory tools and discovery utilities must remain available without Kev questions.

### 2. Signatures
`capability.catalog({})` lists the authorized installation. `capability.catalog({names:[...]})` validates and asks Kev to admit up to eight names; returns `loaded`, typed `unavailable`, and `replaces_optional_window`. Legacy discover is not in the public executor/disclosure view. `decision.WithFreshStageBudget` creates each later directory stage without changing the shared preparation-stage contract.

### 3. Contracts
The public baseline is conversation.reply, capability.catalog, moment.publish intersected with the actual installation. Establish run-local disclosure even if Kev is disabled/error. Keep full executors, but use the same disclosed set for prompt/parent and physical budgets, diagnostics, Generate/Stream and native execution guard. Named requests replace the optional schema window, never accumulate 38 tools. Optional names first pass Kev when tools.select is enabled. An admitted audited No stays denied. Disabled/unclear/timeout/service/audit failure uses the original explicitly requested authorized window, with admission=catalog_fallback and its real fallback_reason; never full original installation or any unrequested tool. The first request remains baseline. Actual configuration version/enablement changes revoke the optional window; stable disabled/read failure is not labelled a configuration change. Preserve the actual Kev reason when finishing audit records. Validate the entire named request before mutating anything. An unqueried hidden tool is distinct from a queried denial. Private Goal/VisualIdentity tools and the Writer's private image-request contract bypass public directory selection. Tool-free tasks keep zero tools.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| first public request, including disabled/error Kev | installed baseline only, at most three |
| named optional request + current yes or bounded original fallback | next actual schema includes only requested names, max baseline+8 |
| admitted No | denied/tool_not_selected, no schema/execution |
| stable disabled/unclear/service/audit failure | bounded named fallback, truthful admission/fallback_reason |
| configuration really changed after selection | optional window revoked/tool_selection_changed |
| named query includes foreign/private name | whole request rejected; no partial visibility changes |
| empty typed installation | zero tools |

### 5. Good / Base / Bad Cases
Good: catalog names query → actual audited admission → next native request calls installed tool. Base: reply/catalog/moment available without selection. Bad: full table after Kev failure, automatic selection of every installation, or a directory dump granting private tools.

### 6. Tests Required
Actual Generate/Stream baseline and native catalog→hidden execution, fresh catalog budget after expired preparation, all-none/foreign request atomicity, bounded replacement windows, configuration revocation and audit failure. Reply-owned media denial reaches result-aware Writer adjustment without an image execution or invented attachment.

### 7. Wrong vs Correct
Wrong: full-original fallback or a schema-only filter with full executor access. Correct: fixed authorized executor plus a mandatory run-local disclosure/permission state, with demand-driven audited loading.

## Scenario: Media multi-choice and compatible settings (2026-10-11)

### 1. Scope / Trigger
Media subject, capture and view supplementation through the existing System One client.

### 2. Signatures
`Config.MediaEnabled` JSON media_enabled; points media.subject_route/media.capture_plan/media.view_plan.
`ParseChoiceAnswer(raw,config,criteria)`; binary `ParseAnswer` delegates with yes/no/unclear.
`Candidate.Point` may name a per-question media point in a mixed second batch; Admit rechecks that point.
`kev-config --media-only --backup <new-private-file> [--endpoint <reachable URL>] [--disable]`.

### 3. Contracts
Every probability key must match the actual question criteria, be finite [0,1], sum to one and agree with selected argmax. unknown/unclear fall back; unspecified is a real accepted neutral choice. Existing binary behavior and audit-before-adopt stay intact.
Default media_enabled=true, global enabled=false. Old complete seven-point maps remain readable; missing media flags use defaults, unknown keys fail. UI restores new default points before merging saved values. Global/media/per-point disable means no media HTTP or prewarm.
CLI media-only saves a new 0600 fsync backup before updating Owner settings and preserves effective seven-point behavior: when enabling a previously globally disabled Kev, its seven old points stay disabled; media-only disable keeps the existing global setting. Endpoint/settings/strategy/model revisions are retained unless explicitly changed. No migration overwrites settings or credentials. No service environment was activated in this task; user runs the command in their target environment.

### 4. Validation & Error Matrix
Unknown probability/choice → invalid_response/original; off media switch → zero HTTP; config revoked before freeze → original_used with original raw answer; backup absent/existing path/write failure → no settings update.

### 5. Good/Base/Bad Cases
Good: one physical capture/view batch retains each logical point's audit and switch. Base: old seven-point JSON works. Bad: media disable changes Goal behavior or an original yes/no question accepts a new arbitrary option.

### 6. Tests Required
Existing TestKev* plus TestMediaBranchKevTwoStagesAndMixedPointAudit, M34/M40 regressions and criteria-bound multi-choice parsing; independent real probe reports raw answers/errors rather than treating prior binary scores as media accuracy.

### 7. Wrong vs Correct
Wrong: hard-code three answer labels for every new media question or overwrite all settings to enable only media.
Correct: bind parsing to the question's allowed choices, preserve raw audit and merge media settings after private backup.


## Scenario: Explicit empty parameters for recovery tools (2026-10-12)

### 1. Scope / Trigger
LM Studio rejects a request when a no-argument tool omits the root
function.parameters.properties object. The observed tools[36] was capability.catalog.

### 2. Signatures
`objectSchema(properties map[string]any, required []string, additionalProperties bool)`
normalizes nil properties to a nonnil empty map before typed Eino conversion.

### 3. Contracts
No-argument closed objects must emit type=object, properties={},
additionalProperties=false on the actual Provider wire. Nil serializes as null,
then the pinned typed Schema.Properties pointer/omitempty drops the field.
Retain required fields, closed validation, original definitions, tool installation,
mandatory/discovery rules and Kev fallback. Never remove catalog to avoid a
Provider schema rejection or weaken argument authorization.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| objectSchema receives nil properties | explicit nonnil empty object |
| catalog arguments {} | valid |
| catalog arguments contain extra field | reject |
| Kev disabled/version/settings fallback | baseline schemas retain properties objects |
| tool discovered for next request | subsequent actual schema retains properties object |

### 5. Good/Base/Bad Cases
Good: catalog remains available with explicit object parameters; optional names does not make {} invalid.
Base: existing nonempty object schemas are unchanged. Bad: delete the tool,
relax additionalProperties or only check a map before SDK serialization.

### 6. Tests Required
TestNoArgToolPropertiesOnNativeProviderWire uses actual pinned SDK/Eino queued
Generate/Stream serialization for all registered roots, Kev fallback and native
discovery continuation. TestNoArgCatalogSchemaValidationAndCatalogParity asserts
closed arguments, output schema parity and no original-definition mutation.
Live LM Studio acceptance is separate from controlled HTTP regression.

### 7. Wrong vs Correct
Wrong: properties:nil becomes absent after typed schema conversion.
Correct: properties:map[string]any{} stays a JSON object on the physical request.

## Scenario: Live provider native-tool prerequisite (2026-10-13)

### 1. Scope / Trigger
When a real Agent returns content-only under required tools, separate Core wire
regressions from model/template/serving behavior using fixed synthetic public
provider requests. Kev disclosure cannot repair an upstream native-tool failure.

### 2. Signatures
Run the task's `research/lmstudio-compatibility/probe.py --base-url <public /v1>
--model <loaded model> --output <report.json> --timeout 30`. `--self-test` performs
no inference. Exit0 requires all required cases PASS; NOT_RUN is not acceptance.

### 3. Contracts
Use native message.tool_calls or actual SSE delta.tool_calls, nonempty original
IDs, offered names and parsed closed arguments. Never reinterpret content as
ToolCalls. Continuation pairs an assistant call with a tool message by its ID;
final JSON is a separate tool-free request. Fixed synthetic ToolResults have no
implementation and prove no business effect. Record model, template, serving
mode and load parameters alongside results; API trained_for_tool_use metadata
alone is not a hard capability verdict.

### 4. Validation & Error Matrix
| Observation | Meaning / action |
| --- | --- |
| HTTP400 missing parameters.properties or object tool_choice | wire defect; use actual SDK serialization regressions |
| HTTP200, content resembling a call, no native fields | fail native prerequisite; do not publish or execute it |
| Template rejects tool role | serving/template defect; repair transport before judging model ability |
| SSE events/DONE present but no native call | SSE transport works; native prerequisite fails |
| Same weights pass after serving-mode change | reject weight-only unsupported conclusion; record usable combination |

### 5. Good / Base / Bad Cases
Good: same Cydonia IQ3_M passed14/14 after the user disabled Engine Protocol,
while the preceding V7-template Engine Protocol run passed5/14. Base: controlled
HTTP fixture proves Core serialization, not real provider behavior. Bad: blame
weights from metadata or add content parsing to hide an upstream failure.

### 6. Tests Required
Require empty/dot/underscore tools, required/auto/none, native SSE, paired
ToolResult consumption and strict JSON. Inspect actual two-call output separately
from the case name. Add a real-returned-ID synthetic roundtrip and dynamic schema
growth check when a provider initially fails these. Re-run after model/template/
runtime/mode changes. Long prompts, semantic Goal success, publication, media and
end-to-end Core acceptance remain separate checks.

### 7. Wrong vs Correct
Wrong: metadata says false, therefore model cannot call tools; or content-only
output is parsed into a fabricated reply. Correct: verify the exact loaded
combination through the public API, keep original native IDs/receipts, and retain
the existing execution/authorization contract. Never extract private backend
credentials as a compatibility workaround.

2026-10-14 regression: globally disabled or media-only tools.select=false previously produced two baseline loaded and six tool_selection_changed despite a stable revision. Test both modes and actual Generate/Stream catalog→execution; test that neither fallback nor later directory calls widen beyond the current named window. Raw unaudited model answers are not adopted.

A failed initial read has an unknown revision, not revision-zero authority. Anchor a named fallback on the next successful snapshot. Distinguish Admit settings_unavailable from config_changed. Once an optional window observes a real revision/enablement change, revoke it persistently until a fresh named query; later read errors cannot resurrect it.
