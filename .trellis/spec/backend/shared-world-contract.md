# Shared World And Independent Actors

## Scope and authority

The Owner confirmed shared-world semantics on 2026-10-30. A world has its own
Owner and IANA timezone. Human, Fluctlight and NPC identities are independent.
Joining an existing world retains its world/place/NPC IDs; an explicit copy is
a separate parallel world. Equal names or definition provenance never merge old
worlds. `life_worlds.fluctlight_id` is retained creation provenance, not access
authority. World authorization uses `owner_actor_id`, including database checks
for bindings and world-item/inventory sources. Relationships, opportunities and
private memories remain scoped to the interacting Fluctlight.

Migration `0102_shared_worlds` is additive over `0101_serial_llm_queue`. It
retains existing IDs, bindings and histories, backfills world Owner/timezone,
relaxes creator-based composite FKs and preserves later Owner configuration
edits on rerun. API and Worker must use the new schema together.

## Schedule and presence

Schedule generation/replanning receives the declared residence independently
from occupation, real world settings/timezone and active place choices. It can
select existing `place_ref`/`place_id`, or propose `new_places` local keys. Core
validates and registers referenced proposals inside Schedule acceptance after
replay/CAS. No text-to-ID heuristic or same-name first-row selection is allowed.
Concrete venues require stable references. `location_kind=transit` has no venue
ID; unknown remains unknown. A generated plan never proves travel or arrival.

Replanning preserves completed intervals, including their historical world IDs.
Only an exact carried-forward prior interval may retain an old-world place;
current/future intervals use the selected world's active places. World changes
and legacy future unbound places enqueue durable native reassessment. A
world-aware Schedule marker and inbox identity prevent repair loops. No repair
rewrites completed history in place or infers semantic locations from keywords.

`scene_event` carries the stable place through preparation/digest, active-world
and closure validation, persisted result and refreshed Life. Scene inference
alone is not actual NPC presence. Confirmed events/started effective activities
remain the actual-presence authority.

## Shared NPC and environment execution

NPC prediction and execution use world time, not an observing character's zone.
One NPC presence is executed once, independently of its creating character;
eligible members receive separate opportunities. Joining/browsing never creates
acquaintance. Encounters write the interacting actor's directed relationship
and private memory. Actor-facing history and opportunity reads cannot disclose
another member's private record IDs. The independent NPC model retains its
shared interaction experience with participant name/current-character labels.

World/place effects are shared by world; character effects remain target scoped.
World-only events need no surrogate Fluctlight or fabricated Life Event. Their
applied run is authority; actual affected/present members receive native facts.
NPC visitor presence also accepts this world-only execution authority.

Shared writes use a conservative Owner mutation coordinator before sorted
actor lifecycle/Life locks and object writes. Private Tools stay actor local.
World/binding/child changes invalidate affected member generations.
Model and Kev requests remain outside mutation transactions.

## Hourly generated events and mandatory Kev

One `world_event_generation:<worldID>` durable clock runs on `world-events`.
It continues at UTC hour boundaries, skips missed slots and is independent of
character pause/chat preemption. The repair loop only ensures clocks; it never
performs LLM/Kev I/O. A world/hour staging row prevents repeated generation after
retry/restart. `no_event` is a valid terminal result.
Unexpected failed clocks can restart with a failed-only reuse policy; explicit
cancellation is preserved. Crossing an hour boundary uses the Activity's next
slot without sleeping through an additional hour. Expired unapproved slots are
labelled expired, and the workflow ends when its world no longer exists.

`world_event_generation` is tool free. Its input contains world setting, real
places/NPCs, actually present characters and recent occurred events, without
private chat or a roleplay persona. Candidate definitions stay non-executable
in staging/diagnostics until `world.event.admit` explicitly returns Yes and
Admit succeeds. No, unclear, disabled, unavailable, invalid/audit failures and
configuration changes cannot queue an event. The admission transaction checks
the stored Kev config revision under lock, actual world/context/references,
target presence and slot expiry, and atomically links approval/template/run.
The final Kev application audit reports actual commit; audit errors are not
silently discarded or converted into a second effect.

Generated templates use a generation source instead of a fabricated Studio
resource, remain manual/one-shot, and enter the existing structured executor.
They do not accumulate in the editable resource-template settings directory.
Their applied world-only executions are in paged world history; all staging,
approval and execution states are available through the read-only
`world-event-generations` diagnostic dataset, excluded from diagnostic clear.
Recoverable minor injury retains confirmed target presence and actual recovery;
membership, a plan or model prose cannot establish injury or arrival. Existing
Owner-authored templates retain their separate explicit configuration contract.

## Verification ownership

`shared_worlds_test.go`, world event/NPC/scene/Schedule tests and
`workflow/world_events_test.go` cover the intended paths. The Owner elected to
perform business and live-model regression. Compilation/static evidence is
separate from final database/Temporal/model acceptance. No private fixtures or
images are committed.

Model diagnostics retain the actual recorded world-clock correlation supplied by
the Activity, separate from generation correlation. Native event perception marks
whether the observed source world is still current; a later world switch cannot
apply old weather to a new world. Shared place history is Owner/world scoped and
records the operating actor, rather than attributing every edit to the creator.

## Scenario: Explicit character world choice and nonempty new worlds (D8/D10)

### 1. Scope / Trigger
Character creation, explicit world PUT/catalog POST/Studio apply/configuration copy.

### 2. Signatures
`finishCreationWorld`, `requireWorldCreationChoiceTx`, `bootstrapWorldTx`;
`BrowserWorldWriteBody.creationPreset?: 'daily'` maps to `creation_preset`.

### 3. Contracts
Activation/preparation/GET never creates a world. New drafts have no default
world authorization. Existing worlds are listed for explicit selection; only a
complete zero-world read offers 使用日常生活世界 and 创建世界. A legacy
`useDefaultWorld=true` without `worldChoiceConfirmed` authorizes no write.
The daily command rechecks the catalog; Core checks zero worlds under the Owner
world mutation lock, returning `409 world_choice_required` on a concurrent create.
All explicit new-world paths materialize the supplied framework or the base
framework (8 places in 3 venue categories), plus 2 independent base NPCs with
immutable definition provenance and unknown position. Configuration proves no
arrival, presence, encounter, relationship or memory. Existing worlds are never
merged, deduplicated, renamed or backfilled by this path.

### 4. Validation & Error Matrix
| Condition | Required result |
| --- | --- |
| No explicit daily choice | No create command |
| Partial/failed catalog | No empty-catalog assumption or create option |
| Concurrent daily choices | One world; loser requires explicit selection |
| Replayed creation command | Same world/place/NPC identities |
| Published/custom framework | Preserve its place keys and category names |
| Existing world lacks active places or NPCs | Hourly cycle `skipped/world_base_objects_missing`; no model/Kev |

### 5. Good / Base / Bad Cases
Good: Owner chooses an existing shared world and no clone is created. Base:
explicit daily selection creates configuration atomically. Bad: interpreting a
boolean draft default as permission to create an empty world on every activation.

### 6. Tests Required
`TestCharacterAdmissionNeverCreatesWorldAndExplicitDailyCreatesBaseObjects`,
`TestConcurrentDailyWorldChoicesCreateOnlyOneWorldPerOwner`,
`TestExplicitWorldCatalogCreationIncludesBaseObjectsWithoutBinding`,
`TestIncompleteWorldsSkipEventsWithoutModelOrKev`; creation-world/r20 DOM and
BrowserClient wire tests cover no-choice, late catalog, scope and replay.

### 7. Wrong vs Correct
Wrong: `if draft.useDefaultWorld { createWorld() }`.
Correct: require an explicit confirmation, complete zero-world catalog and
transactional zero-world check; create usable configuration only in that command.

## Hour-slot reliability and diagnostics (D7/D9)
The durable clock remains `started` while waiting; one generation is bounded to
10 minutes inside the existing 15-minute Temporal Activity timeout. Its staging
row reaches failed/no_event/skipped/admitted/rejected or explicit awaiting_kev.
Worker crash repair closes abandoned generating rows and the matching clock
cycle metadata after 10 minutes. The world advisory lock is held through cleanup
and diagnostic settlement; UNIQUE(world_id,slot_at) plus conflict-safe insertion
fences the world/hour before model I/O. No-event/failed/skipped slots are consumed.
Different worlds at the same hour remain independent. Task list/detail expose
`waiting`, last cycle status and next cycle time, keeping the underlying clock
status in diagnostics. Tests: `TestWorldHourLockDeduplicatesConcurrentCallsRestartAndMultipleMembers`,
`TestWorldGenerationDeadlinePersistsTerminalFailureAndReleasesHourLock`,
`TestAbandonedWorldGenerationTimesOutAndClockTaskShowsTerminalCycle`,
`TestWorldEventClockFailedHourStillSchedulesNextHour`.

## Scenario: Reclaim an abandoned world generation Activity

### 1. Scope / Trigger
`Dispatcher.ReconcileOnce` observes a running `world.events.generate` clock
whose `GenerateWorldEventActivity` has exceeded 15 minutes since scheduling.
The clock's persistent `started`/attempt=1 alone is normal and proves no stall.

### 2. Signatures
`worldEventActivityStale(execution, now)`;
`App.ReclaimWorldEventGenerationAttempt(ctx, intent, attempts, payload)`.
New workflow histories use `world-event-schedule-timeout-v1` to add a
15-minute ScheduleToClose timeout while retaining the old replay branch.

### 3. Contracts
Only an overdue, unpaused generation Activity is reclaimed. First terminate
its exact observed Temporal Run ID; an observation/termination error cannot
release the attempt. Then atomically CAS the intent's started status, attempt
count and payload snapshot to retry with a one-minute delay. Preserve the
attempt count until dispatch actually starts its successor. Terminalize this
cycle's generating/review_pending slot with world_event_generation_timeout;
keep completed hours, other worlds and explicit awaiting_kev gates intact.
Late model/Kev results cannot reopen a reclaimed row or create executable
templates/runs. Admission locks and validates the staging status before effects.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| Old clock waiting on hourly timer | Keep running; no termination |
| Fresh or paused Activity | Keep existing attempt |
| Overdue Activity, including never-started Activity | Exact Run termination, retry after one minute |
| Describe/terminate unavailable | Preserve started attempt; retry observation |
| Cancellation/new attempt/new payload won CAS | Preserve newer ledger state |
| Late model/Kev response | Retain failed timeout; no executable event |

### 5. Good / Base / Bad Cases
Good: reclaim an abandoned scheduled Activity and dispatch attempt 2. Base:
terminalize the consumed hour and generate again next hour. Bad: infer staleness
from started_at, or requeue before terminating the observed Run.

### 6. Tests Required
`TestPostgresWorldEventStaleActivityReclaimsAndDispatchesNextAttempt`,
`TestWorldEventActivityStaleRequiresOverdueGeneration`,
`TestWorldEventActivityTotalTimeoutPreservesExistingHistories`, and
`TestWorldEventAttemptReclaim*` cover backoff, exact Run identity, CAS,
late model/Kev results, hour deduplication and next-hour progress.
Database regressions create disposable databases, never clear a shared database.

### 7. Wrong vs Correct
Wrong: trust every RUNNING workflow indefinitely or terminate by workflow ID
with an empty Run ID. Correct: inspect the pending generation's scheduled time,
terminate its precise Run, then fence the durable retry against the old snapshot.

## Scenario: Automatic event and clock visibility (D12)

### 1. Scope / Trigger
World page 世界事件 and management diagnostics must expose automatic work even
without manual templates, personal Life Events, or recent clock creation.

### 2. Signatures
`GET /api/diagnostic-records/{kind}?worldId=<world>&intentType=<type>`;
`worldId` matches top-level or payload world_id; `intentType` matches intent_type.
Both filters are exact, at most 256 runes, and participate in cursor identity.
Workflow diagnostic world clocks also expose actual world_id/world_name.

### 3. Contracts
Keep generated templates out of the editable manual-template directory.
WorldAutomaticEvents reads world-event-generations and world.events.generate
clocks by worldId, without fluctlightId. It may read an unbound browsed world;
reads never bind or execute it. Generation admission and execution outcome are
displayed separately. Applied ambient runs retain their existing Life Event
projection. Management independently reads permanent world clocks and merges
by record ref with recent tasks, with separate append cursors and retries.
Selecting a task type filters on the server before pagination. Keep Owner/read
epochs, raw references, redaction, and waiting/last-cycle/next-cycle semantics.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| More than 20 newer character tasks | Older world clocks remain visible |
| No manual templates / no_event / rejected | Automatic records remain readable |
| Different world filter | No foreign-world records; previous cursor rejected |
| Read fails or world/Owner changes | Original GET retry; obsolete response ignored |
| Clock appears in both pages | One visible task, deduplicated by ref |

### 5. Good / Base / Bad Cases
Good: show admitted plus applied separately. Base: no_event is a valid visible
result. Bad: infer event generation is absent from an empty manual template list.

### 6. Tests Required
`TestWorldEventDiagnosticsFiltersReachOldClocksAndWorldOnlyGenerations`,
`TestWorldDiagnosticFiltersValidateLength`,
`TestWorldEventDiagnosticFiltersPassThroughBrowserBoundary`,
`world-event-diagnostics.test.ts`, `d12-world-tasks.test.mjs` and the D12 world
page regressions cover exact filters, opaque cursors, scopes, states and links.

### 7. Wrong vs Correct
Wrong: filter task kinds from only the 20 newest rows or filter shared work by
its creator character. Correct: exact server filters and independent world-clock
reads; preserve shared world identity through UI rendering.
