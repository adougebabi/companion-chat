# NPC Presence, Opportunities And Encounters

## 1. Scope / Trigger

An enabled runtime NPC's current routine interval is executed; actual shared
presence may produce an encounter opportunity. Schema: `0076_world_encounter_runtime`.
NPC definitions, NPC Actors, Fluctlights, Owner identity and acquaintances differ.

## 2. Signatures

- `RepairWorldNPCPresence(ctx,limit)` and `RepairNPCOpportunities(ctx,limit)`.
- `world_npc_presence_runs`, operational `world_npc_runtime_clocks`, durable
  `world_npc_opportunities`; existing `world_encounters` retains final events.
- `world.npc.encounter({opportunity_id,expected_revision,decision:contact|pass,reason})`.
  `npc_response` is not a model argument. Independent FormalAgent `npc_response`
  uses the configured cognitive model and its own structured result contract.
- GET `/api/fluctlights/{fid}/npcs/{npcId}/{encounters|presences|opportunities}`;
  GET a specific opportunity and POST its `/retry`. Reads are paged and Owner scoped.

## 3. Contracts

`planned_position` is prediction. Actual presence requires a committed run matching
NPC revision, place, status and time. Its start is the executor's current time;
missed offline intervals are never backfilled as executed. Each NPC has a short
transaction and operational polling clock. A failure rolls back its tick, records
error/backoff separately, continues other targets and returns aggregated errors.

Character presence requires a current confirmed event or genuinely started,
still-effective activity with its authority and linked Intention active. A
schedule or inferred scene alone is insufficient. Capture exact event/activity,
world, profile and NPC presence versions. Profile-scoped opportunity identities
remain subject to a shared 30-minute pair cooldown and normal autonomy budgets.

NativeCognition chooses the character's contact/pass/defer. Contact Prepare runs
the independent NPC response outside the effect transaction, using only that
NPC's private definition and real relevant interaction history. Freeze its first
prepared result for the source/definition key. Every Preparer path, including
pass, must emit nonempty frozen payload; pass uses `state:not_required` without
a model call. Private NPC definition/reason never enters character Tool output
or episodic memory content. Current environment context carries only relevant,
bounded public presence/opportunity facts, not the whole NPC catalogue.

Final Tool execution shares the Life lock with NPC/definition writers and rechecks
all sources, profile, world, versions, current policy and frozen response. Accepted
contact may create acquaintance. Decline records a sourced attempt and memory
without new relationship; pass records no encounter. Event, optional relationship,
Memory, opportunity terminal result, outbox and Tool receipt commit together.
Automatic `virtual_world_encounter` attribution is the Fluctlight and actual
opportunity, never Owner confirmation or automatic Human co-presence. Existing
Owner-confirmed history uses the shared writer with its original explicit evidence.
Permanent harm remains unavailable.

Blank-slate activation does not create or select a runtime world. Catalogue
creation and active selection are separate explicit writes. NPC reads must not
initialize a world: no binding remains `422 world_not_configured`. Cross-world
isolation acceptance first creates/selects and reads back two distinct owned
worlds, then applies the same exact published definition to each and checks
independent NPC/Actor identities, interaction history and bidirectional rejection.
An unconfigured second character cannot certify this boundary. L04 and L13 share
`scripts/e2e/v3-regression/npc-isolation.mjs` for this prerequisite and proof;
`TestWorldNPCPersistenceIsolationEncounterAndSource` pins both the unconfigured
negative path and the configured scope path.

## 4. Validation & Error Matrix

| Condition | Behavior |
| --- | --- |
| Pure GET or catalogue application | No movement, opportunity, acquaintance or memory |
| No effective routine/place | No executed presence; explicit prediction/unknown read state |
| One bad NPC tick | Persist error/backoff; other targets advance; error remains visible |
| Old opportunity revision | Handled no-effect state conflict |
| Cancelled/expired event/activity; world/profile/NPC changed | No interaction side effects |
| Character supplies NPC response | Input schema rejects it |
| NPC model failure | Real failed task, no default acceptance |
| Pass | Frozen `not_required` preparation; no NPC model or encounter |
| Receipt already committed | Recover original result; do not restart model/effects |
| Source expired when retry requested | Reject; retry cannot revive old presence |

## 5. Good / Base / Bad Cases

Good: actual activity + executed NPC presence → durable opportunity → character
approach → independent NPC acceptance → Tool event/acquaintance/memory. Base:
decline, pass, policy deferral or legal absence. Bad: treating routine prediction
as confirmed presence, making acquaintance on application, or importing biography
into character memory before any sourced interaction.

## 6. Tests Required

`world_encounter_runtime_test.go` covers GET purity, execution timing/replay,
overnight boundaries, active-activity cancellation, isolated polling failure,
contact/decline/pass, private-input separation, source cancellation during the
model stage, failure and committed recovery. Controlled Provider tests do not
prove real-model behavior. Test execution is deferred to the final batch; user
real-model procedure is `docs/v3-llm-acceptance.md` L13. Preserve the explicit
50-Tool inventory including Reply; Conversation remains 38.

## 7. Wrong vs Correct

Wrong: a pass Preparer returns its original empty invocation, or a failed oldest
target ends every repair batch. Correct: emit a frozen not-required preparation;
isolate each failed tick with persisted error/backoff while returning real errors
and allowing unrelated NPCs to progress.
