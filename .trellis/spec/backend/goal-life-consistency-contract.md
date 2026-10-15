# Goal, Actor Facts, Life And Runtime Consistency

This contract refines field ownership in the Persona, Memory, Life World,
Autonomy, Provider and Diagnostics contracts. Earlier blanket persona priority
and active historical-summary prompt rules are superseded for these fields.

## 1. Scope / Trigger

Applies to chat, periodic WakeUp, native cognition, Reflection, scheduled
activities, independent Tools, current capture, summary workers and repair.
Keep the formal Eino/ADK loop, Tool registry, receipts, transactions and outbox.

## 2. Signatures

```text
instant.Format(time.Time) -> YYYY-MM-DDTHH:mm:ss.SSS+00:00
instant.FormatLocal(time.Time, *time.Location) -> fixed millis + numeric offset
App.Clock() -> business instant; App.now() -> one injectable authority
actor.inspect({actor_id?,attribute?,history?,limit?}) -> facts
actor.fact.record({operation:assert|correct|change,attribute,value,reason,
                   source_message_id?,corrected_fact_ids?}) -> fact receipt
item.use({operation:start|stop,item_id,activity?}) -> actual usage receipt
ProcessConversationSummaryIntent(...) -> pending|active|conflict
GET /api/diagnostics/{model-runs|agent-runs}?limit=&correlation_id=&cursor=
  -> {items,next_cursor,snapshot}
history-pollution-repair --owner ID --reason TEXT --manifest JSON
history-pollution-repair ... --apply --expected-plan-digest DIGEST
history-pollution-repair --owner ID --reason TEXT --rollback-batch ID
```

Migrations 0043–0048 add Actor facts/dependencies, ordinary inventory/uses,
cumulative summary/revisions, diagnostic snapshot membership, repair audit,
and proactive deliveries. Existing released migrations are unchanged.

## 3. Contracts

- All public instant fields use fixed milliseconds and numeric offsets. DB
  `timestamptz`, cursor timestamps and audit payloads retain precision. Dates,
  durations and original user text stay their own types. Operational leases,
  diagnostics and physical runtime timestamps retain the real clock.
- Projection records `as_of` and `reference_timezone`. User timezone is null
  until explicitly qualified; sender/browser zone is never location authority.
  Local DST gaps fail; a local ambiguous fold chooses its earlier instant.
- Actor facts bind owner, subject, attribute, source/fingerprint, effective
  interval, transition kind, revision and replacement. Inference stays uncertain.
  Native assertions require the actual inbound user source and matching subject.
  Explicit corrected IDs must be for the same owner, subject and attribute.
- Current location/body/wardrobe come from effective Life; persona controls
  behavioral style; qualified human statements control that human's facts.
  A failed Tool, wish, image or authored narrative cannot create ownership.
- Actor corrections retire linked artifacts; Active, Resident, Self and
  evolution overlay reads filter stale fact revisions. Model-derived outputs
  conservatively link all facts visible to that model call, including overlays.
  This can exclude unrelated output after an unrelated correction; do not claim
  per-candidate evidence precision. Full snapshot validation guards late commits.
  Source edits withdraw current evidence while preserving admission audit.
  Stale compiled working persona fails its source/version check and requires
  recompilation through the existing governed compilation path.
- Sleep is a typed accepted item/Event. Periodic checks settle due state first,
  then return `no_op/sleeping` without model or physiological facts. No human
  speaker is synthesized for system triggers. Autonomous repeated topic+purpose
  with no new inbound sequence is suppressed by the configured window.
- Goal execution is a projection of existing Goal/Intention/Activity records.
  Shared NULL profile scope stays shared. Goal dynamic execution fields do not
  enter opaque reference hashes. Relationship goals cannot complete from shopping.
- Shopping requirements are descriptions before acquisition. Completed purchase
  commits the acquisition Event and all item IDs atomically; an entire bundle
  succeeds or fails. Ownership does not change wear/use. Only sourced available
  owned IDs can be worn/used; ordinary objects have no wearing slot.
- Current capture freezes effective body/worn/used items and their versions.
  Models supply closed style fields; Core renders garments and body facts.
  Worker re-renders before `/prompt`; workflow conditioning/reference ancestry
  is validated. Pixels do not write facts. Reference validation is conservative
  and may reject an otherwise visually compatible older configured reference.
- One cumulative summary replaces covered runtime history. Tail=4, batch<=40,
  closed assistant boundary; previous summary plus frozen source range plus
  Actor corrections feed the formal summary Task. Cursor and revision commit
  atomically after source/fact CAS; failure advances neither. Raw history stays.
  Historic episodes remain accessible by explicit recall, not cumulative injection.
- `product.summary`: interval_seconds 300..600 (default300), max_runes 512..4096
  (default2048). `product.autonomy.topic_suppression_seconds`: 300..604800
  (default43200). No-thinking is explicitly requested for cumulative summary;
  provider support/effective mode remain `unverified` until live evidence exists.
- Every physical model call checks complete messages, Tool schemas/results,
  response schema and reserves. Estimates are labeled; required overflow fails
  rather than truncating JSON, facts or protocol. Trace `runtime.*` breakdowns
  overlap their parent; do not sum all trace keys as disjoint sections.
- Diagnostic SQL sorts all sources before LIMIT by fixed time/id DESC,
  NULLS LAST; snapshot membership uses PostgreSQL transaction visibility.
  Updated statuses stay members; late backdated inserts wait for refresh.
- Repair is exact scoped IDs/revisions, audited dry-run by default. First apply
  requires reviewed digest; replay uses stable manifest batch. All items validate
  before mutation. Rollback refuses newer state, appends compensating revisions,
  and never deletes original chat. A nil replay map must stay a nil interface.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Cross-attribute corrected fact ID | actor_correction_target_invalid, full rollback |
| Late Actor snapshot/dependency | conflict, no model-derived commit |
| Sleep action | life_state_sleeping, no side effect |
| Earliest purchase result outside active window | reject admission |
| Purchase failed or partial bundle | no inventory, no completion |
| Unsourced/wrong-owner item or stale snapshot | reject wear/use/capture |
| Model/workflow attempts garment override | reject before renderer submission |
| Required physical input too large | prompt_required_budget_exceeded |
| Edited summary source/CAS conflict | no cursor advancement |
| Malformed/mismatched diagnostic cursor | invalid filter |
| Stale repair digest/revision/newer wardrobe | refuse whole batch |
| Periodic history mixed with real evidence | refuse periodic_only classification |

## 5. Good / Base / Bad Cases

Good: missing boots → descriptive Goal → future schedule → business Clock due
→ real acquisition ID → independent wear → frozen capture → final conditioning.
Base: sleep check has no event and stays silent; user timezone remains null.
Bad: treating an accepted plan as completed ownership, using an image as evidence,
or restoring a corrected fact through cached Self/persona content.

## 6. Tests Required

Use disposable real PostgreSQL plus formal independent Tool and native ADK tests.
Keep the fixed 35-Tool adapter inventory and existing strict live suites.
Assert purchase atomicity/replay, separate wear/use, final worker payload,
20 sleeping checks, legal wake, source withdrawal, restart lineage, overlay
reload, shared Goal scope, summary concurrency/failure/CAS, full-wire budgets,
precise diagnostics pages and actual repair CLI apply/replay/rollback.
Live model semantics and real ComfyUI/S3 pixels require separate evidence;
SKIP, blocked preflight and scripted Providers never count as live PASS.

## 7. Wrong vs Correct

Wrong: `result = nilMap; if result == nil { apply() }` skips first CLI apply.
Correct: assign the replay map to the interface only when the map is non-nil.
Wrong: render clothing from model text or mark shopping complete on scheduling.
Correct: commit acquisition, mutate wear independently, render frozen authority.

## Scenario: WakeUp Silence Reasons Stay In Diagnostics

### 1. Scope / Trigger

An awake periodic check has no reason to contact the human, or a model incorrectly
calls `conversation.reply` with a no-op control value.

### 2. Signatures

```json
{"action_type":"no_op","response_intent":"No new event; continue current work without contacting the user","evidence_refs":[],"influences":[]}
```

WakeUp `result.response_intent` persists the final internal decision reason.
`conversation.reply({text,topic_key?,purpose?})` remains the actual message Tool.

### 3. Contracts

- Final `response_intent` contains the internal silence reason and is never
  published as conversation text. Tool `purpose` describes real communication,
  not the reason for no communication. Silence needs no additional Tool.
- The shared reply description and WakeUp final field descriptions must expose
  this distinction to the Provider. Keep the canonical registry definition;
  do not install an unregistered shadow Tool or alternate execution protocol.
- Publication rejects trimmed, case-insensitive exact `no_op`, `noop`, `no-op`
  and `none`. Natural prose mentioning those tokens stays valid; there is no
  keyword interpretation of `purpose` or arbitrary prose classification.
- Rejection returns non-retryable `reply_control_value_invalid` and tells the
  model to return final no_op/response_intent. The native loop may then finish
  silently; do not resend the control value or fabricate a successful message.
- `publicationCapabilityError` preserves explicit typed domain errors before
  generic cause classification, so the model receives the stable correction code.
- WakeUp failed/rejected Tool attempts remain in the audit, but their mere
  presence does not make a cycle `completed/agent_tools_committed` or overwrite
  final no_op with `capability`. A successful Tool result retains the existing
  behavior, including actual natural replies already delivered.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Exact reserved control text | reply_control_value_invalid; zero message/outbox publication |
| Failed reply then final no_op | cycle no_op; failure audited, response_intent persisted |
| Natural sentence mentioning no_op | normal publication |
| Truly delivered reply then final no_op | delivered reply remains a real fact |

### 5. Good / Base / Bad Cases

Good: quiet work interval returns final no_op with a private diagnostic reason.
Base: no new event yields no Tool calls and no chat message.
Bad: `conversation.reply({text:"no_op",purpose:"continue silently"})`.

### 6. Tests Required

`TestConversationReplyControlValuesNeverPublishAndExplainSilentWakeUp` covers
independent ordinary/autonomous execution, stable non-retryable feedback and
zero visible messages. `TestWakeUpControlReplyRecoversToSilentDiagnosticWithoutPrivateMessage`
uses the formal native loop, scripted Provider and real isolated PostgreSQL:
rejected Tool → feedback → final no_op → persisted reason → zero private rows →
cycle replay without another Provider call. Preserve genuine proactive-reply
and internal life-activity WakeUp tests. This does not prove live model compliance.

### 7. Wrong vs Correct

Wrong: send the internal silence decision via `conversation.reply.text`.
Correct: final `action_type=no_op`, private reason in `response_intent`, no message Tool.

## Scenario: Explicit Owner Background Initialization And Governance

### 1. Scope / Trigger

Creation JSON/analysis preview supplies human background; Owner views or edits
that background in a Fluctlight detail/governance surface.

### 2. Signatures

```json
{"actor_user":{"background":{"name":"Vinson","location_scope":"abroad","timezone":null,"meeting_confirmed":false}}}
```

Optional initialization root actor_user is separate from core_persona.
PUT `/internal/fluctlights/{id}/actor-user-background` receives background,
operation=correct|change, reason, expected_current_facts_revision, idempotency_key.
Browser route uses expectedCurrentFactsRevision/idempotencyKey. Detail returns
actor_user={background,facts}. Migration 0049 adds actor_user_background_commands.

### 3. Contracts

- Allowed fields: name, occupation, background, location_scope, location,
  timezone, relationship_distance, meeting_confirmed. Values are bounded
  nonempty strings or null; meeting_confirmed is boolean/null. Zone is explicit
  valid IANA or null; neither server/device nor actor_self supplies a human zone.
- Missing key is no assertion, null is explicit unknown, false stays false.
  Older initialization JSON with no actor_user remains valid; do not insert
  defaults that change old activation digest. New input participates in source
  projection/activation digest and one activation transaction.
- Subject is authenticated created_by_actor_id and scope is the Fluctlight.
  Callers cannot choose an Actor, provenance, status or source. Seed and Owner
  edits call the existing Actor fact transaction service; runtime/Tools and chat
  corrections read this same authority. Foundation is actor_self only.
- Owner writes hold lifecycle/Actor locks, validate current-facts CAS, apply the
  entire batch and record its immutable command result atomically. Identical
  retry replays before CAS; changed payload under the same key conflicts.
  The command ledger is audit/recovery, never another background authority.
- Current detail is authoritative after chat correction. Read-only detail has
  no mutation inputs; governance distinguishes mistaken old info (correct) from
  formerly true info that changed (change). Dirty form drafts freeze their
  expected revision and survive failed saves; instance changes discard old scope.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unsupported key/type/invalid explicit zone | reject; no partial facts |
| Caller supplies subject/source/status | boundary rejects; Owner binding stays server-owned |
| Current facts changed during editing | 409, preserve draft and ask reload |
| Identical committed command retry | recorded result replay, no extra facts |
| Same key with changed payload | conflict |
| Another Owner edits/reads instance | deny without exposing background |

### 5. Good / Base / Bad Cases

Good: actor_user timezone null and actor_self Asia/Shanghai coexist.
Base: old JSON omits actor_user and seeds nothing; Owner can configure it later.
Bad: hide human profile under core_persona or require an LLM chat to initialize it.

### 6. Tests Required

Initialization retention/validation, complete activation transaction/digest
replay, Owner update/CAS/replay/foreign scope and chat correction sharing facts.
Core/BFF/client generation and typed preview retain null/false. Interface tests
cover read-only detail, separate governance, dirty draft and scope guards.
Real PostgreSQL integration must be marked unverified if the service is unavailable.

### 7. Wrong vs Correct

Wrong: place user location inside Fluctlight identity and hope chat remembers it.
Correct: actor_user.background → owner-approved actor_facts → same runtime projection.

## Goal closure evidence and governance (0050–0053)

- A claimed due Intention has a durable Attempt before Provider I/O. Sync ACTION, QUERY/no_op, async wait and unknown-operation reconciliation have distinct settlement boundaries. Unknown external effects retain the original identity and cannot be blindly reissued.
- Stage/Commitment are typed Goal-owned records; stable criteria IDs/version apply at every level. Pause/cancel/completion cascades preserve links/audit without requiring the now-closed parent stage to remain active. Admission still requires active parents and valid windows. Shared parents allow a scoped current-persona Intention; private parents require matching scope.
- Only `ApplyGoalEvaluation` with re-read actual sources completes a Goal. Main message candidates link real persisted messages, including without a prebound Intention/Attempt. Candidate/confirmed/rejected/withdrawn links and per-criterion judgments remain distinguishable. Plan/governance acknowledgements do not prove business completion. Real item ownership additionally uses existing inventory-source verification.
- Evaluation claim revision fences Provider completion/failure/empty settlements. Life lock precedes request row lock. Six-goal batches carry deferred IDs and preserve source waterlines until the remainder completes. Full CAS snapshots remain in PostgreSQL; Provider input is compact decision authority, never truncated proof/JSON.
- Mutual relationship resolution requires actual same-context authors/participants and relationship evidence kind. The chosen relationship ID/revision is frozen before model work and CAS checked by the shared relationship writer. Expression, openness, silence and acceptance remain distinct. Optional invalid follow-up candidates are audited rejections and cannot roll back a valid original completion.
- Review uses Actor IANA local dates and UTC date-derived windows (23/25 hours across DST). DailyReview and Reflection queue the same evidence evaluation authority. Source arrival requeues existing cycles in the source transaction and increments Review revision, including pending processing races; old decisions cannot overwrite new evidence. Ended-before-claim reviews become superseded.
- No-opportunity, not-yet-due, waiting external, sleeping, refusal, blocked, missed opportunity, ineffective attempt and incomplete evaluation are separate reasons. Missed/ineffective counters are consecutive cycles, with configurable 1..30 thresholds and audited resets. Period evidence, stage and feasible alternatives guard opportunity claims; thresholds demand concrete next steps or strategy changes, never emotion/frequency increases. Soft expiry is a visible review flag, not completion/cancellation.
- Owner commands carry authenticated actor/source, expected revision, reason and idempotency. Browser 409 preserves a draft's frozen Goal ID/version. Backend rejects actor/source/progress/actionPlan injection and forced completion. Current and ended history have bounded stable paging; full Goal details include evidence/evaluation/review/resolution and recent retry/attempt projections.
- The stock CLI defaults to read-only bounded previews. Apply requires the reviewed digest, records/replays one batch, flags unverified legacy completions without reopening them, and queues only planning/evidence work. Unknown operations remain observable reconciliation needs. No SQL model calls, fabricated stages/proof or production auto-migration.
- Native Tool catalogs remain canonical. Compact instructions preserve authority/safety, avoid duplicated root response-plan judgment fields, and omit only empty read-model placeholders/default zero counters. Keep physical budget limits and current-run ToolCall/ToolResult protocol. Scripted tests do not prove live Provider semantics or pixel quality.


## Scenario: Goal assessment authority and evidence continuation

### 1. Scope / Trigger
Goal due settlement, structured evidence assessment, subobject completion and Owner evidence/history browsing.

### 2. Signatures
- `ProcessGoalEvaluationIntent(ctx, requestID)` requires one durable native root submission per frozen Goal; independent object/plan submissions supplement it.
- `GoalEvidence(ctx, actorID, fluctlightID, goalID, limit, cursor) -> GoalPage`.
- `GET /internal/fluctlights/{fluctlightID}/goals/{goalID}/evidence` and `GET /api/fluctlights/{fluctlightId}/goals/{goalId}/evidence`; query `limit=1..50`, `cursor` opaque.
- `BrowserClient.goalEvidence(fluctlightId, goalId, cursor)` returns `{items,next_cursor?}`; Goal detail includes first 20 `evidence` and `evidence_next_cursor`.
- Core OpenAPI owns the same list/detail/history/evidence/command routes; CoreClient Goal types derive from those components and mutations preserve independent service-key/human-session headers.

### 3. Contracts
A plan cannot consume source events without an evaluation. All-unknown judgments cannot masquerade as `no_change`; return `needs_evidence` with a next/wait state. Invalid structured output retains retry and unprocessed sources. Every referenced source, including counter-evidence, respects instance/persona/Goal binding; a withdrawn source may explain regression but cannot support success. Assistant speech proves communication, never self-reported acquisition or completed business action.
Relationship confirmation requires a relationship-scoped Goal and satisfied relationship judgments; confirmation refs must belong to those judgments, and actual mutual participants plus frozen relationship CAS still apply. Stage/Commitment IDs and versions are restricted to the claimed snapshot. Expired commitments with actual in-window proof can be included within the eight-object bound; cancelled/history-only objects cannot be selected by guessed ID. Persist subobject evidence links before consuming sources, without increasing parent criterion progress.
Duplicate suppression does not complete an Attempt. Pending side effects settle before a terminal aggregate; successful siblings do not hide failures. Carry child failure codes into primary outcomes and eventual async aggregation, prioritizing retry-stopping authority failures over transient sibling errors independent of Tool order, so policy/permission failures stop retries. Due admission retains typed reasons (`goal_not_active`, `intention_due_stale`) across replay. `conversation.reply` is served to the NativeCognition Agent as well as Conversation/WakeUp/Autonomy; actual effects still use the same authorization, Goal admission, policy budget and duplicate-delivery transaction. Native final cognition contains state summaries, not a second visible message. Trigger and native execution resolve the same existing Owner private-contact mapping before building the projection; absence does not create a hidden conversation. Rebind queued context tokens by persisted Intention/Goal/Attempt identity plus live revision/state and scoped projected parent linkage, rather than requiring an obsolete conversation-scoped token to remain byte-identical.
Evidence cursors bind Owner instance, Goal and evidence collection; order is `(source.recorded_at, link.id) DESC`. Do not interchange them with revision cursors. API calls authenticate the Owner and join source/link instance scope. UI uses scope and selection epochs for both history and evidence continuation, resets pending flags on navigation, deduplicates stable IDs and retains editing drafts on CAS failure.

### 4. Validation & Error Matrix
| Condition | Behavior |
| --- | --- |
| no root native submission (summary/plan/object only) | `goal_evaluation_native_submission_missing`; no shared source consumption; independently accepted submissions survive |
| unknown judgments with no_change | `goal_evaluation_unknown_requires_evidence` |
| assistant self-report as business proof | `goal_judgment_self_report_not_business_fact` |
| counter-evidence from another instance/profile/binding | source/profile/binding validation error |
| non-relationship Goal attaches confirmation | `goal_relationship_resolution_scope_invalid` |
| unserved Stage/Commitment ID or stale version | object scope validation error |
| policy failure beside successful ACTION | failed Attempt, retained policy code, paused Intention |
| evidence cursor reused for revision history | invalid arguments |
| late UI page after navigation/refresh | discard without clearing the new request's busy flag |

### 5. Good / Base / Bad Cases
Good: a timely sent message settles an expired commitment while the parent waits for its own remaining criteria. Base: an Owner reads the latest 20 evidence associations then pages older ones. Bad: plan-only output consumes proof, a suppressed reply succeeds, or subobject proof disappears after journal consumption.

### 6. Tests Required
`goal_boundary_regression_test.go` covers rejected root isolation, self-report rejection, counter-evidence scope, durable mixed/suppressed/policy settlement, Goal-terminal callbacks, hard deadlines, withdrawal through actual evaluation and timely expired commitments. `goal_stage_replay_test.go` covers distinct redelivery identity, stage dependency, one next stage and unchanged parent progress. Mutual relationship tests assert distinct post-terminal delivery does not duplicate relationship/resolution/outbox. Owner evidence test pages 23 actual records including equal timestamps and rejects foreign/collection scope. Browser route/client and executable GoalPanel async tests cover session forwarding, encoded cursors, stale selection responses and retained CAS drafts.

### 7. Wrong vs Correct
Wrong: `covered[plan.GoalID]=true`, or only check source scope for `satisfied`.
Correct: require a frozen Goal evaluation independently of its plan; validate every source association before committing any judgment, then strengthen success-authority checks for `satisfied`.


## Worker stop ownership during Goal restart acceptance

Production `StartWorkers` retains Run(nil), queues, deployment version and registrations. Returned workers wrap the SDK stop boundary with `sync.Once`; explicit restart and context cancellation may race, but each physical SDK Worker is stopped once. Verify `TestWorkerExplicitAndContextShutdownStopSDKOnce` under race and the actual PG/Redis/Temporal `TestGoalJointPostgresRedisTemporalWorkerRestart` with race, including cleanup. A successful Goal assertion followed by a cleanup panic is a failed test, never positive acceptance evidence.


## Goal Evaluation private native submissions (policy v4)

- Only the typed `RunGoalEvaluationTask` installs an unexported session. Its native catalog exposes `goal.evaluation.submit`, `goal.object.submit`, `goal.plan.submit`. Ordinary Conversation/WakeUp/NativeCognition/Autonomy and Planner catalogs expose none of these; `goal.decide` is Owner direct governance only. `goal.inspect/evaluate/review` retain their read/queue meanings. Tool surface visibility alone is never authorization.
- `toolExecutionAuthorizer` is capability-owned and checked before preparation and both receipt replay paths. Evaluation checks formal Agent ID, actual native/provider identity, private run including claim revision, surface, Owner/Fluctlight/profile/request, and active DB claim. Public metadata cannot create private authority.
- Canonical closed Tool schemas carry short refs only. Immutable wire hydration binds real IDs/versions and enforces ownership. Root payload excludes subordinate evaluations; a root and its frozen due review/relationship confirmation share one transaction. Review adjust requires an already accepted independent plan. Core tracks only its own resulting Goal revision chain; external changes still fail CAS.
- Each submission commits independently through the existing Tool boundary. A domain rejection rolls its savepoint back before producing a durable failed Tool receipt and normal native feedback. Infrastructure/cancellation errors retain their cause. Root completion cascades normally and does not require positive subordinate judgments. A later malformed Stage, plan, summary or model failure cannot undo completed roots. Paused completion retains paused/ready_for_settlement.
- Frozen snapshot protocol and request result store accepted per-target digest/result, revision chain, outcome/error diagnostics and successful memos. An accepted target replays its original result without revision/outbox changes, including an ignored new payload; original digest/history remain unchanged. Claim, projection, live revision and source fingerprints still guard replay. Same-request retries preserve ref numbers and accepted state; fresh claim RunID avoids replaying failed agent_runs. Truly changed authority supersedes the frozen request and queues only unresolved roots with fresh authority, preserving pending review links.
- Final assistant JSON is summary-only. Finalization reads the durable journal, never fabricates native calls or applies final prose. Plan/object success cannot cover root evaluation. Unresolved/deferred goals keep shared source events pending; only matching offered source versions are consumed after complete root coverage. Derived goal_revision events are triggers, not proof. Partial successful memos remain reusable and order by actual assessment timestamp; subobject proof links persist independently.
- Evaluation submissions are excluded from actual business outcome evidence. No new persistence authority, database or LLM orchestration is introduced; snapshot/result and existing tool_executions provide the transaction journal.
- Local pure/native HTTP tests demonstrate protocol/semantic boundaries. Isolated PostgreSQL tests demonstrate durable completion/rejection/replay/partial-retry/review/source fences only when GO_CORE_TEST_DATABASE_URL is supplied. A skipped database case is not persistence acceptance or live-model evidence.


Goal Evaluation's native request stages follow structured-turn-contract.md: uncovered roots require ToolCalls while the session is making progress; ordinary native requests omit final grammar. Root coverage permits optional calls; finite no-progress handling closes Tools and uses the summary grammar. Frozen-version exits and handled awaiting_evidence cycles never count as accepted roots or Goal completion. Tool-free final repair suppresses the request policy. This rule is task-private and does not alter Chat/WakeUp or Goal completion criteria.


### Goal submission journal must remain writable after JSON decode

`goalSubmissionRecords(result)` returns a non-nil map for nil/empty request results and missing/null submissions, retaining stored records. `encoding/json.Unmarshal` of null resets even an initialized map to nil; reinitialize after decoding before returning to `executeGoalEvaluationSubmissionTx`, which writes records after its savepoint. Do not treat initialization before decode as a sufficient invariant or catch the panic as success. The outer Tool transaction must still own domain state/journal/receipt atomicity.

`TestGoalSubmissionRecordsAlwaysReturnsWritableJournal` must actually assign a new record in nil-result/missing/null/existing-record cases and assert existing digest/output retention. Its original-bug RED is assignment to entry in nil map. Real-PG first-native-success/replay tests remain separately required when the isolated database is available; a skipped DB test is not persistence evidence.


0059_fact_generation_provenance refines the facts fence: Intention cursor-only updates and same-value summary consolidation are operational metadata; real content/status/source changes remain fenced. Runtime summary and daily-memory processing now share chat's per-Fluctlight durable logical lease, with snapshot and commit inside it. New mismatch diagnostic generation-source groups are available only after migration and label incomplete retained/history intervals. See structured-turn-contract.md for exact signatures, matrix and required tests.

## Scenario: Periodic evaluation and deterministic native recovery (2026-10-14)

### 1. Scope / Trigger
Goal Evaluation was event-driven after two seconds, with one-minute retries and
unbounded forced Tool correction. User requires one evaluation cycle per 30
minutes and program handling of recoverable model representation errors.

### 2. Signatures
- `admitGoalEvaluationCycleTx(ctx, tx, owner, request, available) -> notBefore,error`.
- `RepairGoalEvaluationCadence(ctx, limit)` produces original durable intents.
- `normalizeGoalSubmission`, `goalSubmissionReplay`,
  `goalSubmissionAwaitingEvidence`, `goalEvaluationAcceptedWireResults`.
- `goalEvaluationProgress.decide(accepted, attempts, missing)` is session-private.
- Shared `physicalModelRequestDecision.ToolCallFallback *schema.Message` is valid
  only with forbidden Tool choice and an explicitly empty AllowedToolNames.

### 3. Contracts
Migration 0063 adds `goal_agent_cadence(fluctlight_id, evaluation_next_at,
evaluation_request_id, updated_at)`. Under the existing lifecycle/Life/request
locks, DB time reserves now()+30min per instance, shared across profiles/Workers.
Pending available_at, retries, frozen snapshots and old direct SQL producers all
pass this gate. Return deferred/reason=evaluation_cadence/not_before without
increasing attempt or consuming sources; never sleep while holding a logical
lease. Repair gives active/paused visible Goals periodic opportunities even with
no new events. Same-authority memo skips remain legal; clocks do not create facts.

Only the trusted native adapter maps all normalized unknown/no_change to
needs_evidence, preserving actual unknown judgments. It can supply a technical
evidence wait if no next/wait state exists. Core's strict validator remains
unchanged; no unsupported satisfied judgment is promoted. Exact frozen criterion
refs may trim whitespace; identical duplicate judgments merge evidence. Unknown,
cross-Goal/object or contradictory duplicates return a handled awaiting_evidence
receipt with assessment_accepted=false, mutation_committed=false, reason and
allowed_criterion_refs, with no Goal/revision/memo/accepted-root mutation.

Accepted target replay returns original output plus replayed,
ignored_payload_change, new_mutation_committed=false and unresolved_goal_refs.
It never overwrites original digest/history. The recovery input sorts records
and labels capability_name and submission_kind=root/plan/object with short refs.

Only newly durable accepted targets reset progress. Three consecutive native
attempts without a new target close this session's Tools. The final request has
the original summary grammar and no tools/tool_choice/parallel_tool_calls. If a
successful Provider still returns ToolCalls, task-owned ToolCallFallback supplies
an operational waiting summary without dispatching those calls. Generate/Stream
preserve real Provider errors and cancellation; original response/usage diagnostics
are recorded before adaptation. Other Agents' loop defaults are unchanged.

Before that final request, persist cycle_disposition=awaiting_evidence under claim
CAS. The handled request may end status=succeeded with disposition=awaiting_evidence,
assessment_complete=false and unresolved_goals. This is operational cycle closure,
not native assessment coverage. Do not consume unresolved shared sources, create
their memo, fabricate a Tool or complete their Goal. Accepted siblings survive;
replay returns the handled result without restarting the model loop. Actual stale
authority retains the fresh-snapshot path instead of being disguised as success.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Same instance, next cycle before 30min | deferred with persisted not_before; no new Agent work |
| Accepted target submitted again with changed payload | original receipt replayed, no second write |
| All unknown or omitted judgments + no_change | native needs_evidence normalization; Goal not completed |
| Unresolvable/cross-scope/conflicting criterion | handled waiting receipt; existing authority untouched |
| Three native attempts add no accepted target | task-owned tool-free closure, explicit unresolved result |
| Provider ignores empty catalog and returns calls | procedural waiting summary; calls never dispatched |
| Provider503, stream error, DB failure or actual cancellation | preserve actual error; no fabricated coverage |

### 5. Good / Base / Bad Cases
Good: accepted plan replay returns its result, unknown root becomes needs_evidence,
and a stuck sibling ends awaiting evidence. Base: different valid targets keep
making progress without premature closure. Bad: loop on non-retryable feedback,
call a plan receipt root coverage, or guess a foreign criterion to make completion.

### 6. Tests Required
`goal_submission_recovery_test.go` executes production normalization/replay/kind
helpers and real HTTP/native Generate/Stream, including uncooperative calls after
empty Tools, Provider503/cancel, and distinct-target progress. Sensitivity tests
must fail when the old conflict/forced loop behavior is restored. Isolated PG
tests separately cover no-domain-write waiting, handled replay, original digest,
partial siblings and pending source retention; SKIP is not persistence proof.
`goal_agent_cadence_test.go` covers 29m59s/30m boundaries, pending/frozen/profile
gates, unchanged attempts and no-event repair on a real disposable DB.

### 7. Wrong vs Correct
Wrong: ask the model indefinitely to fix unknown/no_change or a repeated accepted
plan. Correct: deterministic native adaptation and original receipt replay,
finite operational closure, explicit pending evidence and unchanged truth guards.
