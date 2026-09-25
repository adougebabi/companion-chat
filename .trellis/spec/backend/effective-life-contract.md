# Effective Life Contract

## 1. Scope / Trigger

Migration `0036_effective_life` separates current body, wardrobe, worn clothing, profile habits and virtual activity results from the immutable Foundation source. Main, WakeUp, independent Tools, persona detail, Schedule input and media read the same current authorities. A change of speaking profile never restores an old shared body or outfit.

## 2. Signatures

```text
App.BackfillEffectiveLife(ctx, ownerID, fluctlightIDs, all, apply) -> []EffectiveLifeBackfillItem
App.VerifyEffectiveLifeReady(ctx) -> error
App.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "wardrobe.inspect"|"wardrobe.wear"|"wardrobe.outfit.save"|"habit.inspect"|"habit.decide"|"intention.inspect"|"intention.decide"|"life.activity.start"|"life.activity.advance"|"appearance.style"|"persona.detail"}) -> ToolExecutionReceipt
go run ./cmd/effective-life-backfill --owner <actor-id> (--all|--fluctlights <ids>) [--apply]
go run ./cmd/persona-backfill --owner <actor-id> (--all|--fluctlights <ids>) [--apply]
```

Current tables are `fluctlight_appearance_states`/`_revisions`, `fluctlight_wardrobe_states`/`_items`/`_outfits`/`_outfit_items`, `fluctlight_worn_items`, `fluctlight_profile_habits`/`_revisions`, and `fluctlight_life_activity_runs`. `life_events` and `cognition_action_outcomes` record confirmed results.

## 3. Contracts

- The Foundation and its analysis source remain history. The compiled Working Persona contains stable mechanisms, preferences and effective profile habits; `working-persona.compilation.v2` omits mutable current hair, injuries, clothing and inventory. A nonempty failed compilation does not publish a baseline substitute. One change to body or clothing does not recompile.
- Body and wardrobe belong to the Fluctlight. Outfit references and habit revisions belong to the speaking profile. Body fields distinguish `known`, `cleared` and `unknown`; wardrobe `inventory_complete=false` means a missing record does not prove the object never exists. Initial items enter the wardrobe only when explicitly recorded as held or worn; unknown ownership stays unknown.
- `wardrobe.inspect` is bounded and reports scope/page completeness. `wardrobe.wear` accepts recorded available item IDs and changes specified slots or the full outfit. Purchase success grants one sourced item without dressing it. Loss or unavailability clears normal wear. `habit.decide` records an explicit decision and does not change current wear.
- `intention.decide` may create, qualify, update, pause, resume or cancel. Completion requires a verified result. A persisted `intention.due` fact contains the pre-transition reference under `candidate`; native cognition binds its entity ID, due revision and attempt ID to the current scoped projection before presenting the fact to the model. Before native cognition calls the model, Core freezes the validated due Goal/Intention references and snapshots in the durable inbox fact. A due-linked `life.activity.start` commits the activity, Tool receipt, and pending ActionOutcomes in one transaction; failure of the final model response cannot orphan the attempt. Later final settlement retains the already frozen outcomes. A paused, cancelled, edited, or pause/resumed Intention cannot be completed by a stale activity result, while the elapsed activity and its actual item/body effect remain recorded. `life.activity.start` returns `accepted` with `activity_id` and `not_before`; `life.activity.advance` checks elapsed time, obtains a separate structured virtual result, and atomically records the event, item/body effect, Outcome and linked Intention transition. Deferral remains pending. A due decision carries Goal/Intention refs into its pending ActionOutcome; the completed/failed activity result settles the frozen attempt. External real payments are outside this virtual capability.
- Main and WakeUp keep current body, actual worn items and relevant activities in Runtime, with history/time labels for summary and `persona.detail`. Current `persona.detail` filters mutable legacy extension keys recursively; `operation=history` reads the accepted Foundation revision with its former appearance intact and labels the response `historical_foundation`. The media request freezes appearance and wardrobe revisions. Completion locks both current authority revision rows through its stale comparison and commit, so a concurrent body or wardrobe update cannot interleave before the marker is stored. The canonical identity image is a reference, not current hair or clothing authority.
- Every physical Provider request, including Tool continuation, enforces the total input budget across messages, native Tool schemas and response format. Required content exceeding budget fails; optional sections are removed as units. Trace separates bytes, characters and estimated Tokens. Model and media credentials are required to claim live behavior, while scripted transports prove wiring only.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing effective body/wardrobe/profile-habit baseline on startup | Fail readiness; run `effective-life-backfill` for the authorized Owner before `persona-backfill` recompiles the v2 portrait. |
| Preview or repeated apply | Preview writes nothing; apply creates missing baselines once; rerun reports `skipped` and preserves newer changes. |
| Free-text appearance or outfit preference lacks unambiguous temporal/ownership evidence | Report `appearance_description_requires_semantic_classification` or `outfit_preference_does_not_prove_ownership`; do not invent current ownership/hair. |
| Missing/unavailable/wrong-scope item, stale revision or changed operation payload | Reject without a partial effect; the same completed operation replays its receipt. |
| Historical persona detail requested for an accepted Foundation revision | Return the sanitized original appearance with `time_semantics=historical_foundation`; only current detail excludes mutable extension keys. |
| Activity before `not_before` or virtual result `deferred` | Return accepted/pending without item, body effect or successful Intention attempt. |
| Completed/failed activity with an `intention.due` pending Outcome | Settle the corresponding external-ref Outcome and frozen attempt in the result transaction; failed purchase leaves no item and requalifies the Intention. Agent final failure after committed start does not erase the pending Outcome. |
| Activity result after an Intention update or pause/resume | Record the actual event/item/body effect and resolve its Outcome, but skip the superseded attempt and leave the revised Intention open. |
| Failed portrait compilation or required prompt content over budget | Preserve the last consistent authority; report an explicit error, no silent source fallback/truncation. |

## 5. Good / Base / Bad Cases

- Good: wanting boots creates an unfinished Intention; shopping starts and later succeeds; one pair enters the wardrobe, and a separate later wear action uses its ID.
- Base: a profile switch retains one short haircut and the same shirt while each profile has its own habit revision.
- Bad: treat preference for boots as inventory, call an activity `completed` at start, restore long hair from Foundation at midnight, or mark a due attempt successful after failed shopping.

## 6. Tests Required

- Real PostgreSQL `0035→0036`, empty→head and rerun; preview→apply→rerun with count and later-state preservation assertions.
- Independent Tool matrix: success/rejection, scope, pagination, stale/replay and failure. Activity tests assert earliest completion, success/failure/deferral, item ID reuse, no automatic wear, and due ActionOutcome/attempt settlement.
- Formal Main/WakeUp native ToolCall→receipt→ToolResult→next model request; Provider wire checks required current state, one current input and total budget in the first and continuation rounds. Native `intention.due` must reject unknown/stale influence refs and must settle the matching attempt only after an elapsed confirmed result; pause/cancel/update/pause→resume before result must preserve the activity fact without reviving the old attempt. Start followed by final Provider failure must leave one recoverable pending Outcome that settles once.
- Media test asserts frozen appearance/revision and stale-at-completion; historical persona detail retains former hair while current extensions omit it; live Provider and ComfyUI visual results are separately evaluated and marked BLOCKED when not configured.

## 7. Wrong vs Correct

Wrong: copy `life_profile.appearance` and `daily_outfit_preferences` into every System prompt, then infer ownership or overwrite current hair at schedule generation.

Correct: keep Foundation as sourced history, initialize only evidenced current state, update that state through confirmed Tool/event results, and project the current snapshot into Runtime and media inputs.

## Current-state consistency fence (0037)

`fluctlight_context_generations` is the Fluctlight-scoped read and settlement fence. Domain authorities remain in their existing tables. Relevant writes, including semantic correction or deletion of a cognition source, advance the generation in the owning transaction. Prompt projection compares generations before and after reading; a mutating Tool receipt records its before and after generation. Final settlement follows only committed receipts and locks the generation row before checking the expected value. Do not replace this with a timestamp or a scan of historical rows. Initial Foundation appearance and clothing are consumed once at initialization; an absent current value never means reapply the initial value. Visual Identity initialization reads the current body and worn items in one transaction.

The fence covers `cognition_claims` and newly inserted user `conversation_messages` because both feed a formal projection. An assistant message may be published before its own turn settlement, so its initial insert does not advance the fence; the sourced cognition result at settlement does. A semantic message update or deletion advances the fence and invalidates summaries that cite the old message. Do not refresh Resident on every new user message when no existing Memory source can depend on it.

The Provider-visible current appearance, including worn items, carries one `appearance:ctx_...` reference built from its current snapshot. Its token excludes `captured_at`, because a reread is not a body or wardrobe change; the visible timestamp may remain in the data. It is a data reference for evidence and decision influences, never a second appearance authority. Keep it in the compact current-state surface; a model should cite this exact token rather than invent `current_state:ctx_...` or use raw wardrobe IDs as evidence.
