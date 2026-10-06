# Effective Life Contract

## 1. Scope / Trigger

Migration `0036_effective_life` separates current body, wardrobe, worn clothing, profile habits and virtual activity results from the immutable Foundation source. Main, WakeUp, independent Tools, persona detail, Schedule input and media read the same current authorities. A change of speaking profile never restores an old shared body or outfit.

## 2. Signatures

```text
App.BackfillEffectiveLife(ctx, ownerID, fluctlightIDs, all, apply) -> []EffectiveLifeBackfillItem
App.VerifyEffectiveLifeReady(ctx) -> error
App.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "wardrobe.inspect"|"wardrobe.wear"|"wardrobe.outfit.save"|"habit.inspect"|"habit.decide"|"intention.inspect"|"intention.decide"|"life.activity.start"|"life.activity.advance"|"appearance.style"|"persona.detail"}) -> ToolExecutionReceipt
App.WardrobeItems(ctx, ownerActorID, fluctlightID, cursor) -> {items,revision,inventory_complete,has_more,next_cursor,...}
GET /api/fluctlights/{fluctlightId}/wardrobe?cursor=... -> BrowserWardrobePage
go run ./cmd/effective-life-backfill --owner <actor-id> (--all|--fluctlights <ids>) [--apply]
go run ./cmd/persona-backfill --owner <actor-id> (--all|--fluctlights <ids>) [--apply]
```

Current tables are `fluctlight_appearance_states`/`_revisions`, `fluctlight_wardrobe_states`/`_items`/`_outfits`/`_outfit_items`, `fluctlight_worn_items`, `fluctlight_profile_habits`/`_revisions`, and `fluctlight_life_activity_runs`. `life_events` and `cognition_action_outcomes` record confirmed results.

## 3. Contracts

- The Foundation and its analysis source remain history. The compiled Working Persona contains stable mechanisms, preferences and effective profile habits; `working-persona.compilation.v2` omits mutable current hair, injuries, clothing and inventory. A nonempty failed compilation does not publish a baseline substitute. One change to body or clothing does not recompile.
- Body and wardrobe belong to the Fluctlight. Outfit references and habit revisions belong to the speaking profile. Body fields distinguish `known`, `cleared` and `unknown`; wardrobe `inventory_complete=false` means a missing record does not prove the object never exists. Initial items enter the wardrobe only when explicitly recorded as held or worn; unknown ownership stays unknown.
- `wardrobe.inspect` is bounded and reports scope/page completeness. `wardrobe.wear` accepts recorded available item IDs and changes specified slots or the full outfit. Purchase success grants one sourced item without dressing it. Loss or unavailability clears normal wear. `habit.decide` records an explicit decision and does not change current wear.
- The Owner-only wardrobe detail read reuses `wardrobe.inspect`'s recorded-item
  query, fixed at 30 items per page with an opaque item-ID cursor. The detail
  dialog loads it only when the wardrobe drawer opens and displays current
  wearing from the effective Life snapshot separately. An empty page with
  `inventory_complete=false` is “no recorded item”, not proof of absence.
- `intention.decide` may create, qualify, update, pause, resume or cancel. Completion requires a verified result. A persisted `intention.due` fact contains the pre-transition reference under `candidate`; native cognition binds its entity ID, due revision and attempt ID to the current scoped projection before presenting the fact to the model. Before native cognition calls the model, Core freezes the validated due Goal/Intention references and snapshots in the durable inbox fact. A due-linked `life.activity.start` commits the activity, Tool receipt, and pending ActionOutcomes in one transaction; failure of the final model response cannot orphan the attempt. Later final settlement retains the already frozen outcomes. A paused, cancelled, edited, or pause/resumed Intention cannot be completed by a stale activity result, while the elapsed activity and its actual item/body effect remain recorded. `life.activity.start` returns `accepted` with `activity_id` and `not_before`; `life.activity.advance` checks elapsed time, obtains a separate structured virtual result, and atomically records the event, item/body effect, Outcome and linked Intention transition. Deferral remains pending. A due decision carries Goal/Intention refs into its pending ActionOutcome; the completed/failed activity result settles the frozen attempt. External real payments are outside this virtual capability.
- A scheduled `hair_dye` starts only from a due accepted item linked to an
  active Intention. Planning and start leave the current `hair_color` intact.
  The completed result must confirm exactly the requested color and may change
  only `hair_color`; failure or deferral leaves it unchanged. For a scheduled
  activity with no confirmed result, `life.activity.advance` locks and checks
  the linked Intention before recording an Event or effect. Pause, cancellation
  or expiry closes that pending run as `cancelled` without a body effect. This
  differs from an independent activity whose already confirmed effect remains
  part of actual history after a later Intention revision.
- `appearance.style` records a bounded temporary arrangement. It cannot store
  arbitrary color-change prose or substitute for a confirmed dye result.
- Main and WakeUp keep current body, actual worn items and relevant activities in Runtime, with history/time labels for summary and `persona.detail`. Current `persona.detail` filters mutable legacy extension keys recursively; `operation=history` reads the accepted Foundation revision with its former appearance intact and labels the response `historical_foundation`. The media request freezes appearance and wardrobe revisions. Completion locks both current authority revision rows through its stale comparison and commit, so a concurrent body or wardrobe update cannot interleave before the marker is stored. The canonical identity image is a reference, not current hair or clothing authority.
- Every physical Provider request, including Tool continuation, enforces the total input budget across messages, native Tool schemas and response format. Required content exceeding budget fails; optional sections are removed as units. Trace separates bytes, characters and estimated Tokens. Model and media credentials are required to claim live behavior, while scripted transports prove wiring only.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing effective body/wardrobe/profile-habit baseline on startup | Fail readiness; run `effective-life-backfill` for the authorized Owner before `persona-backfill` recompiles the v2 portrait. |
| Preview or repeated apply | Preview writes nothing; apply creates missing baselines once; rerun reports `skipped` and preserves newer changes. |
| Free-text appearance or outfit preference lacks unambiguous temporal/ownership evidence | Report `appearance_description_requires_semantic_classification` or `outfit_preference_does_not_prove_ownership`; do not invent current ownership/hair. |
| Missing/unavailable/wrong-scope item, stale revision or changed operation payload | Reject without a partial effect; the same completed operation replays its receipt. |
| Wardrobe read for another Owner's Fluctlight or cursor over 128 bytes | Reject before listing items; never return another Fluctlight's inventory. |
| Historical persona detail requested for an accepted Foundation revision | Return the sanitized original appearance with `time_semantics=historical_foundation`; only current detail excludes mutable extension keys. |
| Activity before `not_before` or virtual result `deferred` | Return accepted/pending without item, body effect or successful Intention attempt. |
| Completed/failed activity with an `intention.due` pending Outcome | Settle the corresponding external-ref Outcome and frozen attempt in the result transaction; failed purchase leaves no item and requalifies the Intention. Agent final failure after committed start does not erase the pending Outcome. |
| Independent activity result after an Intention update or pause/resume | Record the actual event/item/body effect and resolve its Outcome, but skip the superseded attempt and leave the revised Intention open. |
| Scheduled dye is paused/cancelled before result settlement | Close the pending run as `cancelled`; write no dye Event or current-color change. |
| Failed portrait compilation or required prompt content over budget | Preserve the last consistent authority; report an explicit error, no silent source fallback/truncation. |

## 5. Good / Base / Bad Cases

- Good: wanting boots creates an unfinished Intention; shopping starts and later succeeds; one pair enters the wardrobe, and a separate later wear action uses its ID.
- Base: a profile switch retains one short haircut and the same shirt while each profile has its own habit revision.
- Bad: treat preference for boots as inventory, call an activity `completed` at start, restore long hair from Foundation at midnight, or mark a due attempt successful after failed shopping.

## 6. Tests Required

- Real PostgreSQL `0035→0036`, empty→head and rerun; preview→apply→rerun with count and later-state preservation assertions.
- Independent Tool matrix: success/rejection, scope, pagination, stale/replay and failure. Activity tests assert earliest completion, success/failure/deferral, item ID reuse, no automatic wear, and due ActionOutcome/attempt settlement.
- Owner wardrobe projection tests cover 30-item pagination, incomplete
  inventory semantics and cross-Owner denial; browser client and detail tests
  cover cursor encoding and a read-only “load more” display.
- Formal Main/WakeUp native ToolCall→receipt→ToolResult→next model request; Provider wire checks required current state, one current input and total budget in the first and continuation rounds. Native `intention.due` must reject unknown/stale influence refs and must settle the matching attempt only after an elapsed confirmed result; pause/cancel/update/pause→resume before result must preserve the activity fact without reviving the old attempt. Start followed by final Provider failure must leave one recoverable pending Outcome that settles once.
- Media test asserts frozen appearance/revision and stale-at-completion; historical persona detail retains former hair while current extensions omit it; live Provider and ComfyUI visual results are separately evaluated and marked BLOCKED when not configured.

## 7. Wrong vs Correct

Wrong: copy `life_profile.appearance` and `daily_outfit_preferences` into every System prompt, then infer ownership or overwrite current hair at schedule generation.

Correct: keep Foundation as sourced history, initialize only evidenced current state, update that state through confirmed Tool/event results, and project the current snapshot into Runtime and media inputs.

## Current-state consistency fence (0037)

`fluctlight_context_generations` is the Fluctlight-scoped read and settlement fence. Domain authorities remain in their existing tables. Relevant writes, including semantic correction or deletion of a cognition source, advance the generation in the owning transaction. Prompt projection compares generations before and after reading; a mutating Tool receipt records its before and after generation. Final settlement follows only committed receipts and locks the generation row before checking the expected value. Do not replace this with a timestamp or a scan of historical rows. Initial Foundation appearance and clothing are consumed once at initialization; an absent current value never means reapply the initial value. Visual Identity initialization reads the current body and worn items in one transaction.

The fence covers `cognition_claims` and newly inserted user `conversation_messages` because both feed a formal projection. An assistant message may be published before its own turn settlement, so its initial insert does not advance the fence; the sourced cognition result at settlement does. A semantic message update or deletion advances the fence and invalidates summaries that cite the old message. Do not refresh Resident on every new user message when no existing Memory source can depend on it.

The Provider-visible current appearance, including worn items, carries one `appearance:ctx_...` reference built from its current snapshot. Its token excludes `captured_at`, because a reread is not a body or wardrobe change; the visible timestamp may remain in the data. It is a data reference for evidence and decision influences, never a second appearance authority. Keep it in the compact current-state surface; a model should cite this exact token rather than invent `current_state:ctx_...` or use raw wardrobe IDs as evidence.


## Scenario: Tool Before Outfit Claims and Scene/Schedule Reconciliation (2026-10-06)

- Conversation, WakeUp and DailyReview share a life-consistency operation rule.
  An intended outfit change must complete through `wardrobe.wear` using recorded,
  available item IDs before a current photo is requested or the change is claimed.
  Inspect when IDs or current wearing are unclear; no wear is needed when the
  authoritative current outfit already matches. Purchase, planning and failed
  wear do not prove wearing. Accepted media generation does not prove a finished
  photo; describe historical photos using their frozen capture appearance.
- Resolve scene/schedule conflicts from effective Life, current time and active
  activities. Valid Event authority stays above the accepted Schedule. When the
  activity ends or an authorized return is chosen, settle applicable activities
  and end/switch the scene through authorized Tools. When continuing the activity,
  inspect and edit/replan permitted remaining/future schedule segments, preserving
  completed history and interruption restrictions. Consume real Tool results and
  refreshed state before claiming a transition; failed/rejected calls leave the
  actual state authoritative. Historical chat cannot restore an obsolete scene.
- WakeUp silence requires no publication Tool; authorized state maintenance can
  still proceed. Do not turn consistency checks into private diagnostic messages.
- This is prompt guidance on top of existing domain validation. At the user's
  explicit request, no tests or live-model checks were run for this amendment;
  no new model-behavior acceptance is claimed.


## Scenario: Borrowed Shop Clothing Uses Existing Wardrobe (2026-10-06)

### 1. Scope / Trigger

Conversation, WakeUp or NativeCognition chooses to receive clothing on loan for
shop try-on, return it, or replace one worn slot. Browsing/desire is not receipt.

### 2. Signatures

```text
wardrobe.borrow({lender, reason, items:[{category, slot, description}]})
  -> completed {items:[{item_id, ownership:"borrowed", availability:"available",
                       category, slot, description, wardrobe_revision}]}
wardrobe.return({reason, item_ids:[recorded_id]})
  -> completed {items:[{item_id, ownership:"borrowed", availability:"unavailable",
                       wardrobe_revision}]}
wardrobe.wear({mode:"partial", item_ids:[replacement_id]})
```

### 3. Contracts

- Borrow/return are registered transactional Tools, available on Conversation,
  WakeUp and NativeCognition, with CurrentLife context, awake checks, Owner/
  autonomy authorization, prepared wardrobe revision CAS, and the existing Tool
  operation ledger. No model-owned ownership/revision/idempotency arguments.
- Borrow accepts 1–8 concrete wearable descriptions; category/slot are bounded
  at 64 characters, description/reason at 512 and lender at 256. It fixes
  ownership to borrowed and registers available items, without auto-wear or
  purchase/Goal completion. Inspect/reuse already recorded items before borrowing.
- Each item receives a confirmed wardrobe_gain Event with lender, reason, source
  fact and Life context provenance. The existing confirmed-Event effect writer
  creates inventory and returns the server-generated ID. These are already-ended
  inventory Events (`expires_at=end_at`), never active scene authority. A small
  positive Event interval respects the released `end_at>start_at` constraint.
- Return accepts 1–8 distinct IDs scoped to the instance; each must be wearable,
  borrowed and available. The existing wardrobe_unavailable Event effect clears
  worn links and sets unavailable, preserving ownership, source and history.
  Restore one's own outfit through a separate wear Tool; no implicit dressing.
- Batch effects, outbox and Tool receipt commit atomically. Same-operation replay
  returns original IDs/results without duplicate effects; changed payload conflicts.
- Partial wear automatically replaces selected slots through the existing UPSERT.
  remove_slots only removes slots without a selected replacement. Full is an
  explicit replacement of the entire outfit, not a conflict recovery default.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Selected slot also appears in remove_slots | wardrobe_slot_conflict with model-visible correction: omit overlapping removal and retry partial. |
| Missing/invalid loan items or missing lender/source | borrowing_items_invalid / borrowed_item_invalid / borrowing_lender_required / borrowing_source_required; no inventory effect. |
| Duplicate loan descriptions or return IDs | borrowed_item_duplicate (or schema rejection); rollback batch. |
| Foreign/missing item | Owner authorization denial / wardrobe_item_not_found; no other item returned. |
| Return owned, unavailable or non-wearable item | borrowed_item_not_returnable; retain state. |
| Prepared wardrobe revision stale | wardrobe_revision_conflict; no effect. |
| Borrow/return while sleeping | life_state_sleeping; no effect. |
| Attempt to wear returned item | wardrobe_item_unavailable. |

### 5. Good / Base / Bad Cases

- Good: receive blue shirt from shop -> borrow returns ID -> partial wear -> photo
  freezes real worn borrowed shirt -> return -> restore personal outfit.
- Base: an already recorded available borrowed shirt is worn directly, without
  another registration. Replay of the same borrow/return operation has no effect.
- Bad: describe a nonexistent outfit, fabricate an ID, claim a purchase, or switch
  full merely because a replacement and removal target the same slot.

### 6. Tests Required

`wardrobe_borrow_capability_test.go` supplies real-domain/disposable-PG assertions
for registration source verification, no auto-wear, borrow and return replay,
partial slot replacement preserving accessories, failed-batch rollback, owned and
foreign return rejection, returned clothing unusability, and model-visible conflict
correction followed by successful partial wear. These tests were authored but not
executed in this change; compilation/static checks are not behavioral acceptance.
Live-model choice of borrowed items and photo pixels remain unverified.

### 7. Wrong vs Correct

Wrong: partial replacement with selected top ID plus remove_slots=[top], or chat
claims that unregistered shop clothing is already worn.
Correct: borrow concrete shop clothing -> consume real item_id -> partial wear
without overlapping removal -> consume refreshed wearing -> request photo.


## Scenario: Scene-First Try-On and Contradictory Chat Recovery (2026-10-06)

### 1. Scope / Trigger

The chat or cumulative summary claims a shop visit/new outfit while effective
Life still reports home/old wearing. Persona shopping habits are not execution.

### 2. Signatures

```text
renderProviderRuntimeProtocol(persona) -> protocol + ProviderContextAuthorityRule
renderProviderSystem(operationRules, ...) -> multiline action sections
compactSummaryForSurface(summary, surface)
  -> {summary, time_semantics:"historical_conversation", ending_state?, ...}
```

### 3. Contracts

- Single- and multi-personality outbound prompts embed the shared domain-fact
  authority exactly once; filtering the same operation-rule fragment does not
  remove its actual content from the wire. Persona identity/behavior constraints
  never override effective body/wearing/inventory/Life facts.
- Rendering preserves operation-rule headings, ordered steps and newlines; do
  not flatten them into one YAML list paragraph. Summary ending_state remains
  historical and cannot restore an old scene/outfit.
- Decision order is current place -> authorized own scene transition -> actual
  acquisition/borrowing -> wear -> photo. At home, do not assume a shop from a
  wish or history, register imaginary shop loans, or change scene merely to
  manufacture a precondition. Respect schedule timing and movement authorization;
  consume scene_event results and refreshed Life before shop actions.
- If old prose claims changed clothes but worn_items disagrees, query wearing as
  needed and acknowledge the unconfirmed action. Do not invent wrong-photo/cache/
  changed-back excuses or repeatedly photograph the old outfit. Borrow only the
  actual authorized target, not a different personal garment offered as if it
  were the promised shop outfit.
- Correct parameters from specific Tool feedback; with no new information,
  do not repeat the same failed call. Missing items/movement/permission require
  an accurate incomplete/blocker reply and an end to the current turn. This is
  model decision guidance, not a new hard iteration limit or text parser.

### 4. Validation & Error Matrix

| Condition | Required model behavior |
| --- | --- |
| Current home scene, historical shop chat | Keep home factual; complete authorized transition before shop actions. |
| Old dialogue says new outfit, current wearing is old | Inspect/reconcile actual wearing; no successful-wear claim. |
| No legal scene transition or actual borrowing conditions | Express plan/incompletion, do not fabricate inventory or loan. |
| Failed Tool, identical context/arguments | Do not repeat indefinitely or invent success. |
| Historical summary ending_state conflicts with Runtime | Runtime owns current facts; summary stays labeled historical. |

### 5. Good/Base/Bad Cases

- Good: current home -> authorized scene Tool -> refreshed shop -> borrowed IDs
  -> successful wear -> refreshed wearing -> frozen photo.
- Base: stay home because the visit is only planned; explain that no try-on has
  happened yet.
- Bad: persona loves shopping, therefore claim a shop visit/new outfit despite
  current home/old worn_items, then explain the repeated old photos as mistakes.

### 6. Tests Required

Provider prompt composer regression cases assert actual outgoing authority once,
no blanket persona hierarchy, preservation of scene-first multiline rules and
historical labeling of summary ending_state. These cases were authored but not
run for this prompt-focused change. Build/vet and source/diff checks are separate
from behavioral/live-model acceptance. The existing Tool continuation refresh
path was inspected, not replaced or newly claimed as live-verified.

### 7. Wrong vs Correct

Wrong: remove shared authority as redundant while short runtime omits it; flatten
scene-before-borrow instructions and let historical ending_state act as current.
Correct: embed domain authority in the emitted runtime protocol, preserve action
sections, label history and decide from refreshed current place and wearing.
