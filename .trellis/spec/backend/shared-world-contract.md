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
