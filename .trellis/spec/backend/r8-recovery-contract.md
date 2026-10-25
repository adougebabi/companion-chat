# R8 World, Shopping And Conversation Recovery

## 1. Scope / Trigger

World category edits, catalogue purchases, initial lifecycle ordering, segment
participant attribution, and recalling the Fluctlight's published Moments.
The R8 report is feedback evidence, not authorization to alter a business DB.

## 2. Signatures

- `PUT /api/fluctlights/{fid}/world/catalog/{wid}` accepts optional
  `venueCategories: Array<{id:string,name:string}>` along with name, CAS,
  idempotency key and reason. Browser transport maps to `venue_categories`.
- `NormalizeModelArguments(raw, schema)` decodes serialized array/object values
  only where the declared schema expects the same container.
- `life.activity.start` reports `shopping_catalogue_place_required` with
  `required_place_id`, or `shopping_catalogue_definition_mismatch` with the exact
  `field` and `expected_value`, as handled/no-effect feedback.
- `RunConversationSegmentTask(ctx, ConversationSegmentTaskInput)` receives source
  messages, authoritative self/Owner IDs and the actual self display name.
- `moment.inspect({local_date?,cursor?,limit?})` reads committed self posts.

## 3. Contracts

Category IDs and names are fixed after creation; at most 32 categories, unique
nonempty IDs/names, all existing entries retained. A category is persisted even
without concrete places. It does not create an enterable store or move anyone.
The UI reloads the catalog after save. Editing a world retains its source ID/CAS.

Shopping uses either single item fields or a bundle array, never both. Catalogue
items retain exact definitions, selected-world scope, availability and stable
place binding. A text location does not establish place identity. Precondition
feedback commits no activity or inventory effect. Start is timed/accepted;
only a due successful result grants inventory, followed by independent wearing.
Do not bypass the timer or derive acquisition from dialogue.

Initial bootstrap is world/persona -> wardrobe -> schedule -> reference/sheet/
avatar. Dispatch the original stable schedule intent after wardrobe completion;
keep current-revision persona compilation and accepted-schedule visual gates.
Missing images do not block non-image life preparation. Model concurrency stays 1.

Segment source messages retain original text/time/role, with author aliases and
display labels from actual participants. The summarizer is not a participant.
The exact native response schema remains authoritative; explicit task rules
name required fields and schema_version. Historical self-denials remain historical
claims, not capability facts. Never delete/rewrite chat to conceal these errors.

Moment queries authorize Owner plus Fluctlight and restrict both stored owner
and author to self; exclude hidden posts and drafts. Limit defaults to 10, max
30. Cursor binds Owner/self/date and uses (created_at,id), preserving posts at
the same instant. The query does not mutate feed positions or publish anything.
Actual scoped query results reach Writer as `published_moments`; a completed
moment.publish receipt may supply `published_text`, accepted/failed drafts cannot.

Two identical consecutive successful named catalog loads with no unavailable
entries suspend discovery for the next private physical request. Loaded business
tools and reply remain available; query/action progress restores discovery.
Three consecutive identical successful loads, or three identical discovery
results (including unavailable-name/empty-directory results), stop before the
next physical call with `tool_discovery_no_progress`. An ignored tool-list
restriction must not silently run to the full iteration limit. This does not
choose a business action or force consent.

## 4. Validation / Error Matrix

| Input/state | Behavior |
| --- | --- |
| JSON array string at array field | Decode, recursively normalize, strictly validate |
| Wrong container / malformed JSON | Reject; do not synthesize an item |
| JSON-looking user/string value | Preserve exactly |
| Model authority envelope | Reject before normalization |
| Catalogue item requires another place | Handled needs_input with actual required place, no start |
| Category remove/rename or stale world revision | Reject, keep prior DB/UI authority |
| No image provider or failed portrait | Original schedule is eligible after wardrobe |
| Foreign Owner moment query / cursor | Reject; no leak, no mutation |

## 5. Good / Base / Bad Cases

Good: save 商店, refresh, add a concrete 衣橱店, bind a product there, start shopping
with its stable ID, settle after elapsed time, then wear actual acquired ID.
Base: a world/category may have no places yet. Bad: invent a placeholder place,
claim purchase from accepted start, or answer a recall by publishing another post.

## 6. Tests Required

`r8_recovery_test.go` checks real PG category/point readback, failed-admission
absence of activity, stringified bundle recovery, acquisition provenance, wearing
preservation, same-instant scoped feed paging, actual Writer facts and schema
repair. `creation_order_test.go` checks exact dispatcher eligibility with missing
images and retained persona/wardrobe barriers. Browser validation and OpenAPI/
client generation must agree. `r8-recovery.test.mjs` executes actual World page
save/edit/reload and body applicability/creation progress rendering.

Opt-in live tests use disposable PG and real local inference, serially, with no
token caps. Inspect actual ToolResults and DB effects; pure queries need not
create mutation-ledger rows. A script exit, a loaded schema or model prose alone
is not execution evidence. Keep failed live attempts and limitations explicit.

## 7. Wrong / Correct

Wrong: local `sites.push()` followed by a save toast; generic catalogue failure
after rejecting the undisclosed stable place; summarize anonymous assistant text.

Correct: await the Core CAS command and reload; return precise scoped feedback;
provide participant attribution without changing original statements.
