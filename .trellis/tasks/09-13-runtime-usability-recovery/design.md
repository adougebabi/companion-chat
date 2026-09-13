# Runtime usability recovery — technical design

## 1. Design objective

This change restores three user-visible runtime invariants:

1. WakeUp is a PostgreSQL-authoritative fixed recurring cycle and Reflection is
   a separate interaction quiet-period one-shot.
2. A missing, skipped, waiting, failed or successful lifecycle transition is
   reconstructable from one bounded diagnostic timeline.
3. Initialization preserves the meaning of natural-language character cards,
   including dense single-personality and explicit multi-personality designs,
   while retaining the practical json_object Provider mode.

The design deliberately reuses the current PostgreSQL, Temporal, Redis,
Provider runtime, capability registry and Diagnostics Center. It does not add a
second scheduler, queue, workflow runtime or diagnostic store.

## 2. Confirmed current failures

### 2.1 WakeUp recurrence is not durable

The current WakeUp workflow executes one Activity and completes. The next cycle
is released only when a Redis TTL key expires and the keyevent listener changes
the stable platform_workflow_intents row from completed to retry.

The following windows can permanently strand that row:

- Redis SET failure is ignored.
- Keyspace notifications may be disabled.
- Pub/Sub expiry is lost while the listener is disconnected.
- Worker exits after committing the WakeUp fact but before setting Redis.
- Worker restart repairs only rows already completed at startup; reconciliation
  may move started to completed after that repair has already run.
- paused, disabled and inactive WakeUp paths complete without an explicit
  resume/re-enable rearm.

The claimed PostgreSQL fallback scan does not exist. Dispatcher and
reconciliation do not select a completed WakeUp row whose next due time passed.

### 2.2 WakeUp cadence was changed into a quiet-period debounce

Successful user turns currently reset the same WakeUp Redis TTL. Continuous
conversation can postpone WakeUp forever without producing an error. This
violates the confirmed product split:

- WakeUp: fixed recurring cadence from the prior WakeUp cycle.
- Reflection: inactivity quiet period reset by new user activity.

### 2.3 Schedule incorrectly owns WakeUp liveness

Activation creates a current-day Schedule intent. The stable WakeUp intent is
created only after an accepted current-day Schedule. A missing or terminal
Schedule therefore appears to the user as a missing WakeUp.

Schedule remains valuable context but cannot own whether autonomous cognition
exists.

### 2.4 Diagnostics observe errors, not expected absence

The project has diagnostic_events, diagnostic_model_runs,
diagnostic_workflow_links, workflow management APIs and a browser Diagnostics
view. The pieces are not connected into one lifecycle trace:

- diagnostic_workflow_links has no production writer.
- WakeUp and Reflection do not propagate one cycle correlation through intent,
  Temporal, Provider and outcome.
- no-op and suppression results mostly remain in Temporal result payloads.
- expected-but-absent work has no watchdog.
- multiple diagnostic writes intentionally discard their own errors.
- several dispatcher, Redis listener, Workflow and persistence branches
  continue or return without a stage/reason event.
- model-run creation starts after several Provider preflight steps, so early
  assignment, secret, composition or budget failures can have no model-run row.

### 2.5 Initialization output became semantically sparse

The old baseline sent a strict, full initialization JSON Schema and a long
field-by-field extraction instruction. The current code sends json_object and a
compact skeleton, permits omissions, and tells the model not to fill the
schema. This fixed local constrained-decoding timeouts but also removed the
field vocabulary and extraction pressure that preserved source meaning.

Server defaults restore keys, not semantics. Empty objects, nulls, empty arrays
and neutral numbers cannot count as extracted character information.

An additional P0 path accepts total semantic loss: when Provider JSON is
truncated or cannot be parsed, the Provider layer can return an empty
StructuredFallback. Initialization normalization then creates a complete
neutral default Persona and reports success. The current Web/BFF path also has
a smaller input limit than Core, initialization model-run diagnostics retain
the complete card/response, root semantic extensions become provenance-only,
and preview goals/intentions can diverge from the JSON the user edited.

## 3. Architecture

### 3.1 Durable WakeUp clock

The existing stable wake_up.current platform_workflow_intents row becomes the
durable clock. No new queue is introduced.

The row owns:

- stable intent_id and workflow_id;
- payload.cycle;
- next_attempt_at as next_due_at;
- status;
- attempt_count;
- last_error;
- started_at and completed_at.

The accepted WakeUp outcome and next due time are written in the same
PostgreSQL transaction as the cognition_wakeups row, source fact, optional
Reflection intent and optional action/capability intent.

Reconciliation may set the intent to completed after the Temporal Run closes,
but it must preserve next_attempt_at. A Worker due sweep conditionally releases
completed rows whose due time passed:

    completed + next_attempt_at <= now + active Fluctlight
      -> retry + cycle increment + cleared run timestamps

Release uses an expected-status/due/cycle predicate and RETURNING so Redis
expiry, periodic sweep, restart repair and multiple Workers converge on one
cycle.

Redis continues to set a TTL key for low-latency release. The expiry handler
calls the same conditional release command; it is not a separate authority.
Failure to set, subscribe or receive Redis produces a diagnostic and leaves the
PostgreSQL sweep capable of releasing the cycle.

User conversation paths stop resetting WakeUp TTL/due time. They continue to
reset the Reflection quiet-period intent.

### 3.2 Activation, lifecycle and settings repair

Activation creates the stable Schedule and stable WakeUp intents in the same
application-owned transaction. WakeUp does not wait for Schedule acceptance.

Idempotent activation replay and Worker startup ensure both stable intents
exist. The repair is additive and uses the same deterministic IDs.

Lifecycle transitions behave as follows:

- active -> paused: do not call the LLM; preserve due time and record
  suppressed_paused.
- paused -> active: if due is past, release after bounded jitter; otherwise
  preserve the due time.
- enabled -> disabled: suppress Provider work and preserve the clock.
- disabled -> enabled: apply the same overdue/preserve rule.
- retired: stop release and persist a terminal suppression reason.

Schedule status is projected into WakeUp context:

- ready: include the current schedule.
- pending/failed/missing: include explicit schedule_status and no invented
  activity/place.

Schedule keeps an independent retry and diagnostic lifecycle.

### 3.3 WakeUp cognition and visible action

Every released WakeUp cycle performs internal cognition unless suppressed by an
explicit lifecycle setting.

The accepted result has one of these domain outcomes:

- completed_noop with a stable reason;
- completed_actionable with settled capability results;
- blocked with policy/dependency reason;
- suppressed with lifecycle/setting reason;
- failed with typed stage/cause/retryability.

A direct private message is only a capability. Internal assessment text,
narrative prose, hidden reasoning and raw Provider text never become visible
chat. Other valid capabilities include moments/media, Schedule operations,
Goal/Intention changes and existing bounded actions.

### 3.4 Reflection quiet-period and recovery

All user-activity Reflection sources use one durable next_attempt_at computed
from the configured quiet period. New user activity updates the same pending
identity rather than creating an unbounded set.

WakeUp/action outcomes may create Reflection evidence, but their intent receives
an explicit due time; NULL is not used as accidental immediate scheduling.

Reflection terminal retry is permitted only while its evidence window has not
been consumed. Workflow-ID reuse policy, retry status and backoff must agree.
The reflection-window claim records owner/lease/attempt identity. When a Worker
dies, lease expiry plus an existing runnable intent recovers the same evidence.

No evidence is a completed_noop/no_evidence result and does not call the model.

### 3.5 Fair work selection

Fairness must exist at three independent boundaries:

1. PostgreSQL dispatcher selection reserves bounded capacity for due lifecycle
   work instead of allowing one intent class to fill a global LIMIT.
2. Temporal worker options or task-queue placement reserve Activity capacity
   for WakeUp/Reflection so long Visual Identity work cannot occupy every
   lifecycle slot.
3. The generated Provider queue gains bounded-wait aging or a reserved
   background slot so continuous priority-100 traffic cannot starve priority-70
   WakeUp/Reflection forever.

The implementation should prefer a small quota/aging policy over continually
reordering a strict priority CASE expression. Exact SLA is an engineering
acceptance bound: in a healthy local environment, a due WakeUp or Reflection
enters its Activity/Provider queue within two minutes even under supported
background load.

### 3.6 Lifecycle diagnostics

Reuse diagnostic_events for transition events and
diagnostic_workflow_links for cross-runtime identity links.

Canonical lifecycle event shape:

- event_type: lifecycle.<surface>.<transition>;
- severity: info, warn or error;
- fluctlight_id;
- correlation_id: stable cycle/intent correlation;
- causation_id;
- payload.intent_id;
- payload.workflow_id;
- payload.run_id when known;
- payload.activity_type and attempt when known;
- payload.stage;
- payload.status;
- payload.reason_code;
- payload.retryable;
- payload.next_due_at;
- payload.attempt_count;
- payload.dependency_status where relevant.

Canonical transitions:

- scheduled;
- trigger_released;
- intent_created;
- queued;
- dispatched;
- already_running;
- activity_started;
- provider_queued;
- completed_actionable;
- completed_noop;
- retry_scheduled;
- failed;
- next_cycle_scheduled;
- overdue.

Events are transition-only. A deterministic transition identity or current
state comparison deduplicates repeated polls. First cause, latest cause and
attempt count remain visible.

The workflow-link writer connects correlation_id, workflow_id, intent_id and
event_id. Provider calls use WithProviderCorrelation so model-run rows share the
same identity.

Diagnostic write APIs return an error. At business boundaries diagnostics
remain best effort, but a failed write increments a health counter and emits a
rate-bounded operational warning. A diagnostic failure cannot replace the
business result and cannot remain completely silent.

Model-run identity distinguishes attempts instead of overwriting repeated
queued/running/failed timestamps into one ambiguous row. Calculating a
diagnostic ID is not reported as persistence success unless the row was written.

Reconciliation dependency lookups fail closed: a database/read error while
deciding whether WakeUp, Reflection, action or Visual Identity is retryable
leaves the intent eligible for another pass and emits a typed stage failure. It
must not fall through into generic terminal settlement.

All row iterators check rows.Err before accepting partial evidence or advancing
a watermark. Workflow command/status transitions check RowsAffected and preserve
the original business result if the audit sink fails.

### 3.7 Absence and stuck-state audit

The Worker periodically performs a bounded lifecycle health audit:

- active Fluctlight missing stable WakeUp intent;
- completed WakeUp past next_due_at plus grace;
- started intent without observable Temporal progress past threshold;
- retry whose due time passed but was not selected;
- running Reflection window with expired lease and no runnable intent;
- Schedule missing/failed while WakeUp continues without it;
- diagnostic model run stuck queued/running beyond lease.

The audit repairs only safe deterministic omissions. It emits overdue or
operator_required for states that require judgment. It never fabricates domain
outcomes.

### 3.8 Diagnostics API and browser

The Core query returns lifecycle events ordered by created_at/id and supports
filters for:

- fluctlight_id;
- correlation_id;
- intent_id;
- workflow_id;
- run_id;
- surface;
- status/severity.

The BFF remains pass-through. The browser adds a Lifecycle section showing a
compact timeline with stage, status, reason, attempt, next due and linked model
run/workflow controls. It does not display secrets, hidden reasoning or raw
database payloads.

Existing Model Runs, Media Prompts, System Events and Workflow management remain
available.

PostgreSQL intent snapshots remain queryable when Temporal is unavailable;
Temporal status/history is an independently failing enrichment. BFF routes
preserve Core’s safe typed code/details instead of replacing them with a fixed
generic error.

Each browser diagnostics source owns its filter epoch, loading/error state and
rows. Switching correlation filters clears or marks prior rows stale so partial
request failure cannot mix records from two correlations. Export uses the same
Core-side filters and includes workflow links/intent snapshots rather than
pretending a browser-only filter applied.

### 3.9 Initialization source and analysis

Keep initialization response_format as json_object for the configured local
Provider. Restore richness through a complete explicit extraction contract
rather than strict constrained decoding.

Provider messages are split into cohesive instructions:

1. authority and no-invention policy;
2. full canonical field vocabulary and source-to-owner mapping;
3. single/multiple classification rules;
4. complete canonical JSON skeleton;
5. appearance/renderer-specific rules;
6. user character-card source.

The prompt requires exhaustive extraction of source-supported semantics while
allowing truly absent facts to remain omitted. It explicitly distinguishes:

- explicit fact;
- source-supported derived value;
- server default/empty.

Semantic-empty output is not a valid omission case. For non-empty source input,
empty StructuredFallback, unparseable/truncated JSON, or a result with zero
source-supported semantic units returns a typed correlated retryable error.
Defaults may complete a partially extracted Persona, but they cannot replace
the entire source.

Initialization diagnostics store metadata, timing, Provider identity, status,
error code and coverage counts by default. The full card and response are held
only by the Owner-authorized initialization source boundary.

### 3.10 Persona field ownership

Extend existing canonical owners rather than adding a parallel character-card
blob:

- Identity: name, nicknames, gender, age, occupation, height_cm plus optional
  height description, blood_type, birthplace, residence, timezone, birthday,
  background, biography/background_story, core values, worldview and notes.
- Life Profile appearance: long description, structured physical features,
  everyday outfit descriptions, style preferences and renderer fields.
- Relationship: initial directed actor_user relationship only.
- Personality profiles: existing identity, personality, behavioral_policy,
  emotional_state, voice, body_language, behavior_state_machine,
  behavior_loops, scenario_behavior, secrets, intimacy_progression,
  output_preferences, fears, desires and extensions.
- Personality system: explicit switching/forced activation rules, influence,
  conflict_resolution with long-form core-conflict description, integration/
  fusion, shared state machine and extensions.
- Special settings: profile/shared habits, rituals, sensitivities, boundaries
  and fixed behaviors under existing behavior loops, scenario behavior,
  secrets, preferences or character constraints.
- Media behavior: channels, moment/selfie/art preferences, frequency and
  triggers under output_preferences. Initialization records policy only and
  does not execute media.
- Daily settings: preferences/likes/dislikes, life habits, recurring
  commitments and character constraints.

Unknown but meaningful source material is stored under a bounded namespaced
extensions path. It is never silently dropped because it does not match a known
field.

Shared character-card semantics that have no first-class field remain in a
typed Core Persona semantic extension and are projected through the normal
Persona allowlist. They must not be moved only to infrastructure provenance.
Profile-local unknowns remain in profile extensions. Metadata-like IDs,
diagnostic fields, credentials and source bookkeeping never enter cognition.

Initial social relationships accept only actor_user for this release.
actor_self and profile-to-profile relations are represented inside
personality_system, not as Actor relationships. A shared initial relationship
remains visible after switching from default to a named profile.

### 3.11 Immutable initialization source

Add an append-only, Fluctlight-owned initialization source record linked to the
accepted Foundation revision. It stores:

- source text and digest;
- initialization correlation;
- Provider endpoint/model identity;
- prompt/schema/policy versions;
- single/multiple classification and evidence;
- field-evidence/derivation map;
- semantic coverage report;
- accepted structured projection;
- creation timestamp.

The source is Owner-only. It is excluded from default diagnostics and ordinary
prompt projection. A dedicated detail query exposes it in a collapsed
Initialization Source panel.

Re-analysis creates a new source/analysis record and a governed Foundation
proposal; it never overwrites the immutable original or bypasses Foundation
revision governance.

Web, BFF and Core share one byte-based source limit matching the Core contract.
Every layer rejects oversize input explicitly and none truncates it.

The preview store owns one typed Foundation object. Raw JSON is an advanced
editor for that same object rather than a second goals/intentions state.
Starting a new analysis marks the prior result stale; only the latest successful
request epoch may activate.

### 3.12 Semantic coverage

Coverage is based on asserted semantic units, not key presence.

Each anonymized fixture contains semantic assertions such as:

- source concept ID;
- required canonical/extension target;
- expected explicit or derived basis;
- accepted value predicate;
- forbidden invention predicate;
- required single/multiple classification.

Results classify every assertion as:

- preserved;
- default_only;
- missing;
- moved_to_extension;
- contradicted;
- invented.

Empty objects, nulls, empty arrays and unrelated neutral defaults do not count
as preserved.

The repository contains dense anonymized single- and multi-personality fixtures.
The Owner’s real card and optional expectation manifest are supplied by external
paths to an opt-in Live Provider test. Reports never contain the source or full
response.

The private Live Provider harness bypasses ordinary prompt/response recording
or uses a metadata-only diagnostic sink. It rejects repository-contained source
paths and includes canary tests proving source, expectations, response and
reasoning do not appear in report output or test errors.

## 4. Data and migration impact

Prefer additive migration changes:

- immutable initialization source/analysis table;
- indexes supporting owner lookup and source digest;
- lifecycle diagnostic link indexes if current indexes are insufficient;
- no destructive rewrite of existing Foundation/Persona rows.

Existing Fluctlights receive no fabricated source record. Their source view is
missing until a future explicit re-analysis.

WakeUp clock repair reuses existing platform_workflow_intents columns. If a
schema field is required for safe CAS or explicit suppression and cannot be
represented in payload without weakening constraints, add it in one additive
migration with empty-to-head and previous-head coverage.

## 5. Compatibility

- Existing API error codes remain valid; new details are additive and bounded.
- Existing Diagnostics sections remain unchanged; Lifecycle is additive.
- Existing Persona fields remain readable. New fields are additive and use
  typed empty/null defaults.
- Existing multi-profile IDs remain stable.
- Existing WakeUp intent/workflow IDs remain stable; cycle identity changes are
  payload/additive.
- Active Temporal histories must replay. Workflow function changes require
  saved-history or testsuite replay evidence before rollout.

## 6. Security and redaction

- No credentials, authorization headers, cookies, decrypted secrets or object
  storage locators enter lifecycle events.
- No hidden reasoning or unrestricted Provider response enters diagnostics.
- Character-card source is Owner-only and excluded from ordinary diagnostics,
  logs and prompt projection.
- Diagnostic reason text is bounded and redacted; stable reason_code is the
  primary operator contract.
- Real-card fixture paths and contents never enter Git or snapshots.

## 7. Rollout and recovery

1. Apply additive migrations.
2. Deploy Core/BFF/Web/Worker with old IDs and APIs preserved.
3. Worker startup repairs stable WakeUp intents and due clocks.
4. PostgreSQL due sweep restores recurrence even if Redis is unhealthy.
5. Existing failed/stuck workflows remain inspectable and recover through the
   compatible retry policy.
6. Rollback keeps additive rows/columns readable by the prior release; no
   destructive downgrade is required.

Operational success signals:

- no completed WakeUp remains overdue without an overdue/operator event;
- two consecutive WakeUp cycles complete under forced Redis loss;
- lifecycle timeline links Provider and Temporal stages;
- initialization semantic coverage meets the fixture manifest without strict
  Schema timeout.

## 8. Trade-offs

- Persisted transition events add writes, but transition-only dedupe avoids poll
  storms and makes silent absence observable.
- PostgreSQL due sweep adds a bounded periodic query, but removes Redis Pub/Sub
  from correctness authority.
- Fixed WakeUp cadence can run during frequent conversation; bounded jitter and
  internal no-op behavior prevent unnecessary visible messages.
- Rich json_object prompting uses more input tokens than the compact prompt, but
  avoids the known constrained-decoding timeout while restoring field coverage.
- Preserving source text uses storage and requires owner authorization, but is
  necessary for audit, semantic comparison and future re-analysis.

## 9. Rejected alternatives

- Restore the old full strict json_schema immediately: rejected because the
  configured local Provider timed out under complex constrained decoding.
- Keep Redis expiry as the sole WakeUp scheduler and add logs: rejected because
  logs do not close lost-event/crash windows.
- Require Schedule before any WakeUp: rejected because Schedule failure would
  still stop the autonomous lifecycle.
- Force one chat message per WakeUp: rejected because private messaging is an
  optional capability, not proof that cognition ran.
- Log every poll or raw error/payload: rejected because it creates noise,
  retention pressure and leakage risk without a coherent trace.
- Store the full character card in every prompt projection: rejected because it
  wastes context and competes with current state, memory and growth.
