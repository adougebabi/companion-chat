# Studio Tasks And Definition Artifacts

## 1. Scope / Trigger

`0078_studio_jobs`: Owner requests definition assistance or isolated fictional
text from an actual resource and its current working draft. Studio never uses
a private conversation as its assistant transport or writes Life/Memory facts.

## 2. Signatures

`/studio/jobs`, `/{jobId}`, `/{jobId}/{retry|discard|save-material}`;
`/studio/materials`. `studio_jobs` freezes the source; attempts own staged model
output. `studio.generate` uses the existing lifecycle dispatcher/Temporal path.

## 3. Contracts

`0079_studio_media` adds `image_preview` using the existing capture resolver,
optional Kev, MediaPrompt, MediaWorkflow, quality policy and private object
storage. Media intents/assets have mutually exclusive `fluctlight` and `studio`
ownership. Studio uses the real Owner actor and producer job, null Fluctlight
ownership, and no conversation/message/Moment targets. Formal media reads exclude
Studio scope. Pixels are served by the existing authenticated binary proxy;
never expose bucket/key or borrow a fake Fluctlight. Only explicit material
retention makes a preview a resource material. Nonhuman reference admission
requires an owned retained source with a validated generated plan/normalization;
unsupported person/upload/legacy sources are not guessed compatible.

A Studio planning workflow delegates to one stable media intent and a media
workflow for its attempt; planning completion is not image completion. Capture
plans stage before delegation. Retry preserves the plan, Provider job, quality
state and ready asset. It refuses a prior child still active. Completed/discarded
product replays return their terminal state. Discarded late pixels never become
formal publication or a new grant; explicitly retained material stays readable.

Every terminal media failure carries its **originating workflow ID**, including
the Activity callback. Lock the product job then media row; compare the captured
origin with the media row and current attempt before any product write. Do not
reload the newest workflow and treat that as the old callback's identity.
Dispatcher fallback raw updates use the same source fence. A verified ready
asset is the recovery authority, including terminal fallback: finish the product
atomically with media completion instead of demanding regeneration. A stale
origin cannot affect a newer attempt. Existing formal publication stays intact.

Browser transport normalizes envelope/reference fields but preserves the opaque
definition `body` for both Studio resource writes and job snapshots. A unit
converter test must exercise the same conversion helper as the actual route.

Admission is Owner authorized, resource-version checked, idempotent and atomic
with the workflow intent. Model I/O occurs outside transactions. Qwen's formal
assistant returns `proposal` or `needs_input`; independent Cydonia produces
labelled fictional text. No life tools, Memory or persona context are installed.
Validated output is staged before final completion; explicit retry preserves a
completed valid stage. Discard/failure cannot be overwritten by late output;
zero-row state writes never prove completion. Text material is an explicit
resource relation and does not replace canonical definition body.

An incomplete draft is valid assistant input. A proposal must satisfy its full
definition schema. `needs_input` preserves original name/body and identifies the
missing choice instead of substituting an unrelated effect or invented default.
Typed stable references are restricted to explicit draft references verified
against this Owner's catalogues at admission. They are frozen in
`allowed_references`; generated references must belong to that set. Event
references cannot combine independent worlds. This lookup is not a Life read.

Workflow Describe outage/empty observation is not a failed model execution.
Studio and Diary retain queryable state, expose `workflow_status_unavailable`,
and do not consume their execution budget or retry the model because a status
read failed. Confirmed ordinary terminal states and dead-letter branches both
close their owning product rows through existing typed hooks.

The v3 generic, NPC definition and standalone preview pages share one complete
Owner/resource working-draft shape. Reads preserve an existing CAS baseline.
The assistant result is a proposal; only explicit adoption changes that draft.
Commands retain immutable payload/key until their outcome is known. Selected-job
polling never resets directory/material pages or replaces the composer DOM, and
pauses while hidden/submitting. Owner change/unmount invalidates late responses.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown Owner/resource or stale admission version | Explicit error, no job |
| Missing required reference/input | `needs_input`, original draft unchanged |
| Generated invented or cross-world reference | Reject output, no valid proposal |
| Provider failure | Bounded actual failed attempt, explicit retry |
| Temporal status lookup unavailable | Preserve execution, retry observation |
| Late model after failed/discarded task | Preserve real terminal result |
| Completed stage retried | Reuse staged output; no new model call |
| Save preview as material | Persist definition artifact only |

## 5. Good / Base / Bad Cases

Good: a slow assistant returns a separate proposal while newer form edits remain.
Base: it asks for a missing NPC selection without inventing an ID. Bad: local
regex replies, stale status reads terminating running models, or preview text
being saved as actual character diary/Memory.

## 6. Tests Required

Core Studio admission/output/replay/isolation/late-terminal tests, migration and
browser transport tests, and web working-draft/pagination/IME/visibility/late
receipt tests are authored. They remain unexecuted until final batch regression.
Static compilation is not acceptance. Actual model steps are in
`docs/v3-llm-acceptance.md` L15 and are reserved for the user.

## 7. Wrong vs Correct

Wrong: the NPC editor has a different draft store from its sandbox, or polling
rebuilds the input node and erases loaded older pages. Correct: use the shared
working-draft helper, stable composer and independent selected-job observation.


## Published creation templates and catalogue reads

Scope: character_template is a reusable creation definition, never a runtime archive or an activation credential. `character-template.v1` contains exactly one description or initialization object; structured sources exclude Actor backgrounds and source/runtime identity and pass the existing initializer at publish. Assistant proposals use this same validator.

Signatures: GET /studio/resources?kind=&q=&published=&cursor=&limit=; GET /studio/resources/:id/versions/:version; POST creation analysis with an exclusive description, json, or resourceId/resourceVersion selector. Resolve a selector in Core from the Owner's exact studio_resource_versions row; working body or latest publication is never a substitute. Carry the verified id/version/digest into initialization source classification_evidence. Reuse normal source freshness, single use, working-persona compilation and activation replay.

Filter kind, literal name search and verified publication existence in SQL before keyset pagination. Nonmatching newer records cannot consume a filtered page. Unknown Owner/version is unavailable; wrong resource kind is an input rejection, and browser creationError preserves actual 400/404/409/422 instead of returning service unavailable. Sources/definitions stay private/no-store.

Creation recovery persists accepted character id and original group before releasing the immutable pending activation. A disposed page may still checkpoint for the same Owner/request; logout/newer draft cannot be recreated or overwritten. An older mounted page cannot erase an accepted checkpoint. Group failure/reselection operates on the existing activated id, never a new activation request. Only a first known pre-admission stale-source rejection permits explicit reanalysis; a conflict after an unknown response cannot prove the original had no effect.

Required final tests: filters before pages, literal wildcard search, exact publication despite newer working draft, Owner isolation, source provenance, controlled compiler/activation replay, input error transport, raw JSON byte/shape contract, refresh/unknown retry, disposed-page completion, stale mounted draft, deleted-group reselection, logout/late result. Authored tests and static compilation are not runtime or real-model acceptance.


## Catalogue lifecycle invariants

Copy declares draft/published and exact source version under resource revision CAS. It produces one independent unpublished resource with copied_from evidence; never copies runtime references, retained materials or publication authority. Its command key replays the same new id. Version history queries studio_resource_versions with Owner/keyset paging, not working-draft metadata.

Delete is an explicit archive operation retaining publication/job/material history. Referenced world/place/NPC/event definitions, linked creation templates and typed resource_id references from other active definitions reject archive. Restore only returns catalogue availability. Archive/apply/publish/job admission share the resource row fence; new initialization links read trusted template_source evidence and lock that resource before accepting activation. A previously analyzed preview cannot bypass later archive. Precompile checking avoids an unnecessary model call, and the final short transaction repeats the check.

A lifecycle component may be disposed by parent redraw while its command continues. Persist only the returned target identifier before clearing pending identity, with original command key and Owner ownership checks. A new mounted component must query actual server state before updating parent or offering the created-copy link. Result-read failure retains its pointer; logout cannot recreate it. Shared component event messages contain scope identifiers, never a second business-state cache.

Required final coverage: precise copy and replay, publication/history retained after archive, actual reference rejection, archive after analysis before activation, concurrency with apply, both generic/NPC surfaces, parent redraw during accepted commands, failed result lookup, Owner switch and failed-copy draft retention. Current authored tests are unexecuted; static checks do not grant acceptance.


## Visual style publication and reference application

0080 adds paired exact-published source/version FK and local field overrides to the existing Visual Brief. visual-style.v1 has only schema and four explicit bounded strings. Apply checks Foundation and Brief revisions, requires keep/override for every changed existing field, and calls the same Brief writer/history/audit in one transaction. Manual refinement preserves source and recalculates overrides. New sessions freeze the full brief; no effective body, wearing, canonical or global workflow write. Consumer dispatch uses immutable publication.kind even if a later draft changes type; actual references are projected independently of draft kind. Source display updates from the save receipt without rebuilding the active form. PG upgrade/replay/retag/freeze and two controlled UI regressions are authored; only pure UI/validator/static checks have executed in this slice. L09 actual model/media stays reserved for the user after full delivery.


## Foundation definition application (0081, source not yet deployed)

persona-fragment.v1 contains explicit changes to identity/personality/behavioral_policy/life_profile/personality_system. Published character templates use their explicit core fields, never normalization defaults or initialization Goal/relationship/Self seeds. A template initial active selector cannot change the existing runtime. JSON Pointer field paths identify leaf differences; keep omits the patch, override uses exact publication values. The existing Foundation proposer stores paired source/version FK plus immutable choices in the same short CAS/idempotency transaction, locking the source against archive. A proposed receipt is applied=false and links to governed accept/compile, never a success toast claiming application. Real source references independently project proposed/accepted/rejected and whether the accepted revision is current. Pending/accepted sources prevent archive.

Replay identifies the original application payload and source id before recomputing a diff against new state, so an already accepted application returns the same proposal. The accepted definition compiles through the existing Working Persona consumer, then atomically commits source/portraits. Existing profile IDs remain to protect historical references. New profiles compile from explicit initial habits before rows exist and initialize only missing profile habit rows on commit; effective body/wardrobe/existing habits/current runtime are not reseeded. Existing missing habit data is an error, not a fallback.

Text templates expose preparation_required in preview. Explicit Studio assistant purpose=foundation_prepare requires an exact published text version and matching source body/name; its durable attempt reuses the existing initializer Agent and validation directly, without calling AnalyzeDescription or persisting an Owner initialization credential. It returns a reviewable structured character-template proposal. Only explicit adopt/save/publish enables subsequent Foundation application; parsing never writes a character. Studio retry/discard/staged completion rules remain unchanged. Browser purpose/version are paired and cannot be used with image references. L08 adds governed real application and L15 adds isolated text parsing; actual model execution stays with the user after full delivery. Authored PG/migration fixtures do not prove execution.

## World item registration and sourced acquisition (0082, not deployed)

item-definition.v1 is an explicit bounded item_kind/category/slot/description object, with no ownership/wearing/use or runtime credentials. Its immutable publication applies only to a world_items catalogue entry, with Foundation, runtime entry and active-world binding fences plus keep/override. Current world/entry identity and binding revision are captured by preview; switching worlds cannot silently move the application. Existing source definitions or text item_pool do not grant inventory. Registry availability/place limits affect new activities. Started activities retain their admitted immutable source snapshot.

wardrobe.inspect catalogue/catalogue_detail query this current-world catalogue lazily; IDs are world_item_id, distinct from inventory IDs. life.activity.start validates exact item fields, current world, availability and permitted place under the Life/registry lock and freezes catalogue_source in its request. The model cannot submit source authority. Original elapsed result processing still validates each exact requested member and confirmed shopping Event. Definition provenance is copied only from the frozen server request, never from acquired model output. Inventory insert records both purchase_result event source and exact resource/version/world entry/revision atomically; legacy free-description shopping keeps null definition provenance. Different actual activities may buy multiple copies. One activity's receipt and stable member identity cannot duplicate its acquisition. Inventory detail reads the immutable registry revision snapshot, so later definition/config changes do not rewrite acquisition provenance. Reference guards protect source resources, worlds and places.

Current v3 adds shared item-definition form, world item cards/directory/detail/configuration and a link that prepares an unsent chat request through the existing say route. Browsing/configuring/suggesting does not move or acquire. L05 purchase uses a separate character and actual model Tool path, waits at least fifteen elapsed minutes, then checks confirmed Event/owned item and both source dimensions. It blocks when the configured wait budget is insufficient, or when no actual activity starts. Real model runs stay with the user after the full delivery. Controlled clock/Provider PG fixtures only prove domain persistence and replay, and are currently authored/unexecuted.

## Reusable outfit definition to actual inventory combination (0083, not deployed)

outfit-definition.v1 is an independent resource kind with bounded description and 1–16 exact item publication references. Publish resolves this Owner's precise wearable definitions and rejects repeated slots. This is a reusable definition, not concrete inventory. Its application targets the existing profile-scoped wardrobe outfit writer rather than a second outfit-state system. Preview captures Foundation, profile, wardrobe collection and saved-outfit revision, and returns bounded concrete inventory suggestions by the definition slot (with explicit overflow). Owner chooses one actual ID per definition reference; a same-slot substitute is allowed only as an explicit selection and recorded local override. Apply checks scope/CAS, validates every mapping, then calls saveOutfitTx. A saved combination neither acquires nor dresses anything; the existing inspect/outfit request/wear consumers remain responsible for use.

Source resource/version FK and derived overrides are stored on the saved outfit in the same transaction before its immutable revision snapshot. Ordinary Owner/Tool refinement preserves origin and recomputes name/member differences from actual inventory provenance, without changing that inventory's original acquisition source. Original application replay returns its old receipt and cannot overwrite later refinement. Source/outfit/item definition archive guards include actual saved combinations and active published outfit member references.

Studio assistants may use only exact garment references verified at admission, frozen with kind/id/version/slot/description. Outfit image planning and normalization receive these verified garment semantics, not just reference IDs, and still no character Life/private context. Model/image consumers are tested by user-run L15 outfit-assistant and L16 outfit-image; actual pixels remain a separate manual fidelity review. L08 includes deterministic source-to-concrete-outfit application. L05 has a separate actual outfit request and reports legal refusal/deferral without fabricating wear. None of these live steps have been run by the agent.

New outfit publication and new Studio job admission check member catalogue availability under the same short Owner reference-graph advisory as archive, before acquiring parent rows. Member rows are locked active; ordinary historical source recalculation remains allowed after archive. Frozen slot uniqueness is enforced on assistant proposals, and image admission rejects nonwearable references. A source resource referencing its own older immutable typed version does not by itself block its own archive; other active published parents and actual saved instances still protect it.
