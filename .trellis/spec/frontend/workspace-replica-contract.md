# Supplied workspace Demo contract (2026-10-27)

## Image storage and commit policy (Owner request, 2026-10-29)

Images, screenshots and reference-image copies are local verification artifacts.
Do not add or modify image files in Git, including tracked images and SVG files.
Image deletions are allowed. Preserve text reports, request/check results, geometry
and historical paths; after cleanup a historical image path is not proof that its
file still exists. `.githooks/pre-commit` runs the staged-image guard. New checkouts
install it with `pnpm hooks:install`; never force-add ignored images or bypass the
guard. This policy supersedes earlier instructions to include screenshots in Git.

## r19 route, preference and configuration presentation contract

Management directory actions navigate to the actual `/diag?view=tasks`,
`/settings?section=…` and `/data` routes. `/settings` always selects configuration,
including entry from a diagnostic view without `section`. Diagnostic URLs retain
only `view`, `taskId`, `correlationId` and a valid `returnTo`; read the live route
when forwarding initialization context. Ordinary entry does not fabricate a return
banner. Explicit views/correlated links precede the Owner display preference;
a fresh diagnostic entry defaults to tasks.

Studio's main directory uses `fluctlight-studio:${owner}:directory-ui`; fixed-kind
views append `:${fixedKind}`. These are display preferences, never business drafts.
A new resource may be dirty for save/navigation protection while the UI calls it
“新资源，尚未保存”. Do not weaken `dirty()` to remove an early warning.

Settings use `value_sources[key] === "runtime_default"` to distinguish genuine
runtime defaults from saved configuration. Keep the actual endpoint/model/numbers
and unknown extensions intact. Only acknowledged values set their existing source
to `stored`. No configuration mutation occurs merely by viewing a category.
`scrollTo(section, focus, reveal)` leaves nested technical disclosures closed on
ordinary navigation; explicit validation recovery can reveal the invalid control.
Clean forms have no permanent save bar; dirty/recovery/save feedback stays in normal
flow. Domain navigation's `.domain-rich-label` must inherit its anchor line-height,
otherwise retained general rich-label styles enlarge every row.

`CommandSyncAction` puts a real ordinary read-latest action in the secondary menu,
exposes a direct action for an actual conflict, and is absent during unknown or
accepted/readback recovery. `canReviewLatest(draft)` may suppress a noop reader.
Successful review scrolls/focuses its connected heading. Keep body/key/CAS and
GET-only accepted recovery unchanged.

Chinese preview is a typed display projection. Wardrobe aliases are contextual;
known relation `metrics` numbers use Chinese percent labels via `relationMetrics`.
String numbers, booleans, false/zero and unknown extensions preserve their source
values; complete JSON stays accessible. Do not normalize the submitted document.

Required checks: `r19-ux-regression.test.mjs` plus current owning command/settings/
Studio tests; production browser `verify-r19-ui.mjs` at 1920, 1348 and 390px. Assert
settings→diagnostics→global settings, live initialization return, blank unconfigured
roles, runtime-default→stored after receipt, isolated directory preferences, exact
original activation count and typed JSON preservation. Fixture results certify
only the recorded UI/HTTP behavior. Images stay outside Git.

## r18 UI state and context contract

### Scope / trigger

The r18 regression continues `10-28-demo-fullsite-audit`. Keep the immutable
Demo as the visual source and current formal DTOs as business authority.
All report items require individual findings; a passing build or normal-page
capture does not accept the entire fullsite matrix.

### Signatures

- `refreshActiveTurn(explicit, targetTurnId?)`: an explicit target reads that
  turn's observed state and never drains `queuedTurn`. The existing untargeted
  observer retains its authorized queue-drain behavior after confirmed completion.
- `creationController.importFile(file)` uses both a per-invocation file epoch
  and the existing request/mode/busy/Owner/lifecycle fences.
- Diagnostic `page(kind, {correlationId, cursor, limit})` includes the current
  correlation for main and auxiliary record kinds. Selection details and cursors
  are invalidated when the route correlation changes.

### Contracts

Task lifecycle events are not a declared phase model. Wardrobe phase derivation
requires the same task and fid plus actual committed preparation facts; missing
phase timestamps remain missing. Domain recovery metadata does not grant generic
restart/reset actions. Notifications can show tasks in their existing dialog;
reading, marking read, and controlling a task remain separate operations.

Settings directory reads never create a dirty connection draft. Compare editable
connection/binding values, not fetched model lists or read timestamps. Preserve
CAS, precise numeric values, credentials-pending markers and unknown autonomy
action strings. Standard JSON creation remains usable without an LLM when Core
supports direct validation; configuration failures must not blame provider paths
on editable user fields.

Place collection controls retain plain strings and string arrays as distinct
types. Object/mixed arrays have a readable view and explicit full-structure
editing. Unchanged nested collections are omitted from writes, preserving unknown
extensions. After an accepted world binding, read its catalog and shared role
detail; never infer that the role physically moved.

Media metadata has one active dialog at a time and uses the original command
controller, CAS, frozen request and readback rules. Avatar RadioGroup exposes only
`canonical` and `asset`. Saved model-service deletion, media favorites, explicit
initial-avatar mode and Demo world-art `kind` are absent from the current APIs;
do not create pretend persisted controls for them.

### Validation and error matrix

| Trigger | Required behavior |
| --- | --- |
| File A, then B selected; A resolves or rejects first | Ignore A; only current B can adopt input or report its failure |
| A file resolves after analysis, mode change or activation | Keep the newer request/input and accepted identity |
| Targeted chat state inspection with a queued follow-up | GET only; queued body/key and persisted record remain unchanged |
| Correlation changes during auxiliary pagination/detail | Clear the old panel/cursor and reject its late response |
| World binding accepted but shared detail read fails | Preserve receipt; recovery reads only, without another binding command |
| Diary has no available body/attempt | Compact ordinary explanation and disabled reply; no fabricated prose |
| Confirmed body returns | Remove obsolete no-body guidance and enable the actual reply entry |

### Good / base / bad cases

Good: metadata cancel returns to its original detail with one dialog; confirmed
save reads actual new revisions. Base: empty profile fields are grouped once per
section, while real values, false, zero and edit entries remain available. Bad:
copying Demo's local favorite toggle into a production page with no save API.

### Required tests

Use `r18-ux-regression`, `demo-replica-creation`, `conversation-delivery`,
`r16-regressions`, and owning diary/media/world/profile tests. Assert the newest
file selection, GET-only targeted status checks, exact correlation on auxiliary
pagination, stale-response rejection, one active metadata dialog, and original
binding/command recovery without a second mutation.

Browser evidence uses production dist at 1348×925 and 390×844, with 1920×1113
for settings/diagnostic density. Each mutating scenario needs its own fixture
state; desktop edits must not seed a mobile case which assumes original values.
Reference screenshots remain immutable and their source hashes must be checked.

### Wrong versus correct

Wrong: a historical-response status button calls the general queue observer and
silently sends another message. Correct: supply its target turn, read actual
state, and keep queue advancement in the original untargeted observer.

The Owner's `初始化流程演示.zip` supersedes older visual/layout references for
pages migrated under `10-27-demo-workspace-replica`. Its original REPLICA.md,
SOURCE_MAP.md and screenshots are preserved in that task's `reference/`.
The current React/Radix, business protocol and prompt contracts remain active.
Read the exact Demo source and target code before migrating each page.

## Ownership and signatures

- App uses the existing Sidebar/Inset lifecycle. The root SidebarProvider sets
  `mobileBreakpoint={701}`; the chat preview retains its independent provider.
- `--sidebar-width: var(--workspace-sidebar-width)` must reach the provider's
  inline style, otherwise its default 16rem wins over the layout stylesheet.
  Widths: 218px desktop, 240px >=1600, 198px <=1150, 185px <=900, 67px collapsed.
- `workspaceReadingKey(owner, route)` includes canonical path, fid and sorted
  query entries. Global library position survives choosing a different role;
  distinct feed filters/settings sections never share a position.
- `fluctlight:library-view:${owner}` in sessionStorage holds only search/tag/view.
  Observe edits while mounted; unload is not a React disposal event. Storage
  failures cannot block the directory, and Owner changes release the old key.

## Presentation and data

`WorkspaceNavigation` owns three flat navigation groups, side role switching,
account actions and canonical page values. Icons remain labelled when collapsed.
`WorkspaceQuickJump` composes a Radix Dialog with its Trigger, Cmd/Ctrl+K,
page/role filtering and Escape dismissal. Never copy Demo reset/simulation tools.

`workspace-replica.css` owns migrated shell/library layouts. Library reads the
existing directory/detail projection and creation recovery. No fake characters,
world names, recent-chat timestamps or published results may fill missing data.
WorldArt/StoryMark are decorative artwork, never real portraits or world facts.

Radix asChild trigger composition may replace `data-slot="button"` with
`dropdown-menu-trigger` or `dialog-trigger`. Layout classes on those semantic
compositions must not require the overwritten slot. Ordinary primitives retain
ownership of keyboard, portal, dismissal and focus behavior.

## Validation and errors

| Condition | Required behavior |
| --- | --- |
| Directory empty | First-character entry; scoped navigation disabled; global tools usable |
| Search has no matches | Clear filters, preserve real directory |
| Details loading/fail | Keep confirmed cards; distinct loading/error with real reread action |
| Navigate to profile/back | Preserve Owner-scoped search, tag, view and route reading position |
| Refresh while filtering | Latest UI preference was already persisted |
| 390px / 700→701 boundary | No document overflow; correct Sheet/desktop navigation |

Tests: `demo-replica-library.test.mjs` covers these state transitions, real
callbacks, mounted persistence, Owner isolation, query isolation and QuickJump.
R12/R14 now assert direct life navigation instead of the superseded collapse menu.
Before advancing pages, compare production dist against running Demo at
1348×925 plus narrow width and record normal/empty/edit/loading/failure evidence.
Fixture UI evidence does not certify live PostgreSQL/model/workflow acceptance.


## Migrated chat and now pages

The latest Demo replaces the former <=800px chat Sheet with an inline moment
preview below the conversation at <=1050px. Keep the fold preference scoped to
Owner/character; `.without-preview` must override every responsive grid rule.
Chat renders real stream deltas and lets the existing controller replace them
with committed records. `clearActiveConversation()` releases `loading` when it
invalidates the failed selection epoch, so initial-read failure remains retryable.

`ConversationHistory` uses the real directory/message/search endpoints. Browsing
never selects a session; explicit resume does. Initial directory admission blocks
search until its cursor/result type is known. Failed reads retain their original
query, cursor and append mode. Resume cannot be dismissed in flight, and a confirmed
selection reconciles the same active Owner/role even after the reader unmounts.
Dialogs return focus to their connected source and have a bounded scrolling body.

NowPage consumes the existing LifePage model. `loading.life` includes schedule
reads. `ScheduleTimeline` derives execution styles only from `action_status`,
never elapsed clock windows. `WearingSummary` consumes only confirmed
`_wardrobe.wearing.worn_items`; catalogue rows can supply names for those IDs,
never prove wearing. Missing and confirmed-empty arrays remain distinct.
Read buttons catch recorded domain rejections; partial schedule failures retain
confirmed rows and appear once. Missing weather, temperature and state stay missing.

Regressions: `demo-replica-chat`, `demo-replica-now`, `r10-active-chat-protocol`,
`conversation-scope-recovery`; browser evidence under the active replica task.

## Schedule commands and response recovery

### 1. Scope

The production schedule page is `react-app/pages/SchedulePage.jsx`; the formal
life provider remains the command owner. The Demo's accept/reject simulation
controls never become Owner commands.

### 2. Signatures

- `lifeClient.schedules(fid,{from,to})`, `requests(fid,cursor)`, `request(fid,id)`.
- `scheduleItem / scheduleItemPending / retryScheduleItem / rebaseScheduleItem`.
- `submitSchedule / schedulePending / retrySchedule` retain their existing body.
- `rebaseSchedule(fid,date,calendar,expectedKey)` and `releaseSchedule(fid,date)`
  handle only an explicitly proven first pre-admission rejection.
- `jsonValueSnapshot(dto)` detaches serializable JSON values from nested Valtio
  proxies. Use it for owned read/edit baselines; not DOM nodes or controllers.

### 3. Contracts

Calendar reads carry Owner/fid/date epochs and retain the same date's confirmed
snapshot on failure. A missing date is not another day. Unread data is unknown,
not a confirmed zero. Date edits reread their own calendar/context revision
before admitting a generation command. Any existing schedule record, including
`items=[]`, prevents ordinary generation.

The provider stores frozen command input and identity before sending. Validate
receipt ID and fid before releasing that record. A first explicit
`409 revision_conflict` permits reviewed new versions and a new identity while
preserving date/reason or the original item operation/content. An unknown attempt
followed by conflict is still unknown; it never enables release/rebase.

Recovering an existing pending date restores its original date/reason before
freezing fields. RequestStatus observes actual requests, preserves records through
read errors, pauses while hidden and offers manual continuation after its existing
read guard. `deferred`/`awaiting_goal` remain observable, not terminal success.
Accepted intention is not completed action. A disconnected dialog source falls
back to the current page heading.

### 4. Error matrix

| Condition | Required behavior |
| --- | --- |
| Initial calendar failure | Unknown count and retry; no fake empty date |
| Failed refresh | Retain that date's snapshot |
| Older date/Owner/fid reply | Ignore before assignment |
| Missing/foreign receipt | Retain original body/key |
| First explicit CAS rejection | Show original input and an explicit review |
| Unknown then conflict | Retry original identity, no release |
| Existing empty record | Protect it; generation stays disabled |
| No character scope | No scoped request before shell redirect |
| Nested Valtio JSON | Detach values before structured clone/edit; extensions survive |

### 5. Cases

Good: lost response replays equal body/key and reads the original request result.
Base: cancelled editing keeps the original schedule. Bad: UI-only acceptance,
manual completed flags, inferring execution from the clock, or generating over an
empty but existing record.

### 6. Verification

`demo-replica-schedule.test.mjs` and `schedule-receipt-recovery.test.mjs` cover
editing/cancel, precision, epochs, date preflight, existing record protection,
unknown remount recovery, first-rejection review, changed Owner admission fence,
receipt validation, ongoing statuses, retired state and focus fallback. Production
fixture evidence is under `evidence/schedule/`, including request attempt digests.
It is not live database/model acceptance.

### 7. Wrong / correct

Wrong: release pending after any HTTP 200 or call retry forever on a proven CAS
rejection. Correct: validate the receipt before acknowledgement; only an explicit
first rejection can use reviewed versions, while unknown outcomes replay exactly.

## Goals projection and commands

`GoalsPage.jsx` reads current and terminal-history partitions independently, each
with its own cursor and retry. `goalAssessment` matches goal scope, current criteria
version and criterion IDs; backend progress is authoritative. Stage/commitment
evaluations never satisfy the parent goal. Evidence pagination does not erase a
saved judgment. Reassessment follows the original evaluation request ID through
goal detail and never directly marks completed.

Goal commands use WorkspaceCommandDialog/createDomainCommand. `afterAcknowledged`
may merge the returned entity identity into the current draft while preserving
newer fields. Apply it when importing an accepted checkpoint into an active model,
including a late acknowledgement after close/reopen, and persist the merged draft
before clearing the receipt. Latest-object review receives the current draft so
recovered creates can read their real ID. Cancel discards only an unsubmitted
draft; pending originals and accepted readbacks remain recoverable. A new detail
selection resets evidence/history and reads the selected goal's own records.

Regressions: demo-replica-goals and r11-domain-command; production fixture
five-state/narrow evidence under evidence/goals. This is not live Provider proof.

## Wardrobe replica and direct-write readback

Wardrobe keeps its inventory controller and three stable tab keys, starting in
the current garment catalogue. WearingSummary and card badges use confirmed
worn_items only. Decorative GarmentArt never represents a real image or color.
Archived catalogue and item/outfit history keep independent cursors and scope
epochs. Archived records are read-only because no restore command exists.
Ownership and availability map to the formal inventory body.

Composition must validate both selected IDs and their saved slot/type before
normalization. Missing or moved items require explicit removal/reselection.
Archive confirmations recheck readonly/busy at execution and retain failures.
Dialog return focus captures event.currentTarget; macOS clicks need not focus
buttons, so document.activeElement alone is insufficient.

workspace.command records with a received direct-write result set
readbackPending before onSaved. Until readback succeeds, the same operation
returns its original record; recovery runs onSaved only and cannot send another
mutation. This protects the mounted operation record and does not claim durable
direct-receipt recovery across a browser refresh. Actual OwnerRequest checkpoints
retain their existing durable observer path. Clear originalInput once its
original request is acknowledged so success no longer claims it is waiting.

Regressions: demo-replica-wardrobe, workspace-command-readback, r6-wear-readback,
r7-outfit-results and r13-regressions. Browser evidence: evidence/wardrobe/.

## Diary paper, annotation and generation

DiaryPage reads all/favorite/shared directories independently, with frozen failed
cursors and Owner/fid/selection/read-order fences. The paper renders real date
and body; missing title/mood/nighttime authorship are not filled from Demo seeds.
After annotation changes, refresh both all and favorite caches. Older background
reads cannot replace a newer same-revision projection or newer annotation.

WorkspaceCommandDialog inline presentation retains the same command lifecycle
for paper annotations. Annotation CAS uses annotation.revision and preserves
note whitespace with a 5000-code-point limit. Reply CAS uses the diary revision
and returns an OwnerRequest, never proof of a chat reply. Shared experience rows
are remote messages and do not establish physical copresence.

Generation uses entry.date and pending/running/ready/no_content/failed/inactive.
Only closed local dates are admitted; failed/inactive require an explicitly
reviewed diary revision, while ready/no_content are preserved by Core. Follow the
original diary ID, pause reads while hidden, and keep source/attempt/task records
reachable. Scope generation pointers and drafts to Owner/fid and clear on logout.

Regressions: demo-replica-diary; fixture evidence under evidence/diary/.

## Memory, reflection and self governance

MemoryPage composes three formal read domains. useWorkspaceRead fences Owner,
fid, identity and request epoch, retains original failed loader/cursor, and hides
the previous identity before a new-scope effect. adopt accepts an already checked
server readback and invalidates older reads; secondary refresh failures must not
replace the confirmed new detail with the previous revision. Separate dialog and
command focus sources so closing an editor does not lose the original card.

Managed memory mutations are CAS-only browser contracts: no client idempotency
key, content <=4096 UTF16 units, evidenceRefs and version/target fields only.
Unknown admission requires explicit current-object review; acknowledged failures
recover by reads only. Revise replaces support refs, while history retains old
refs; source pagination always pins revision. A restored snapshot adds revision
and may compensate lineage. Scores use the actual 0–1 scale. Counts describe
loaded records; aboutOwner is a Core query, never synthetic tags or text matching.

Merged support refs can contain earlier Owner notes. memoryRevisionReason uses
the current semantic_reason before any single-ref fallback, preserving the exact
Owner reason while leaving opaque refs in full source disclosures.

Reflection feedback is an actual OwnerRequest; agree does not mean adoption.
Self claims have review/forget/restore, no direct text-update endpoint. Review
is Owner acknowledgement for that claim revision and does not increase its
revision/confidence. Restoration requires accepted, complete active/uncertain
snapshot and actual source validation; old expiry/supersession boundaries remain
Core-owned. Validate returned operation/IDs and review Owner/claim/fid before
clearing write recovery state.

Regressions: demo-replica-memory; browser and API audit evidence: evidence/memory/.

## World catalogue, runtime panels and retained input

WorldPage owns scoped BrowserClient catalogue/snapshot reads through
useWorkspaceRead. App skips its former duplicate world preload and heading.
Catalogue records belong to one Fluctlight. Inspect inactive worlds via
catalogWorld; never select a world merely to browse it. Item/template/event
catalogues read the active binding only, so inactive detail tabs explain that
boundary and do not issue an active-world read under the inspected world's name.
Current-world badges derive from active_binding.world_id. World selection CAS
uses active_binding.revision; world/place/item/template/run revisions are separate.
Selection receipt life_write=false never proves a new place or activity.

WorldWrite does not send unsupported world kind/scene/status. Supplied WorldArt
is decorative and selected from the existing name/flavour for presentation only.
Place names and established venue categories stay immutable; status is explicitly
preserved on place update. Optional unchanged furniture/connections/roomItems and
world itemPool are omitted so nested unknown data survives unrelated edits.
When adopting a reviewed baseline, replace untouched draft fields with latest
values and retain actual user changes; replacing only source would turn stale
collections into destructive updates. A recovered place create adopts the
acknowledged immutable name/category and keeps later conflicting identity input
as explicit unapplied draft data, alongside later editable fields.

Use WorkspaceCommandDialog for world/place/category/binding/item/default/event
commands. Its afterAcknowledged receives the actual executed body and merges the
accepted identity into the draft. World creation has a visible cross-refresh
recovery entry. onSaved must compare any recovered target with its second
argument (the original executed body), not the current opening's opposite toggle.
Validate receipt and minimum object revision before clearing readback recovery.
Readback failure restores GETs only; unknown admission retains original body/key.

Place defaults require place and default revisions. Only capture of actual
current_saved/current_definition or restore of recorded is supported. A missing
baseline stays missing. History covers field edits and baseline operations;
parent/category/connection IDs are read from current full place data. Opening
history from an editor retains that editor draft rather than saving it.

Item policy changes use item revision and never add inventory. Acquisition opens
the existing Owner/role chat draft, appends to unsent text, and never auto sends.
Template triggering returns a real queued execution; applied/filtered/failed are
independent recorded outcomes. Retry preserves the run identity and requires
template plus run revisions, enabled original template and a valid expiry.
Queued run observation pauses while hidden and releases its interval on unmount.
Actual life-event detail shows only recorded results and direct references.

Regressions: demo-replica-world, react-flows and react-review-regressions. The
world catalogue toolbar/card geometry and desktop/narrow five-state evidence
are recorded under the replica task's evidence/world/. These fixtures certify
the UI/protocol boundary, not live Core/DB/worker/model execution.

## People, authored background and actual encounters

PeoplePage owns the world people, About You and relationship tabs, with native
person detail/edit/create routes. A person is known only when an actual stored
relationship targets that Actor and belongs to the active personality or global
null scope. Directory membership, predicted presence and a declined encounter
never imply acquaintance. Current presence, prediction, default home and a
historical confirmed encounter remain separate location facts.

AboutYouPanel uses the existing readBackground/saveBackground domains. Submit
only changed fields; omission preserves hidden/unknown values. Age and gender
remain free strings, optional nulls/booleans retain their real values. Owner
facts_gen_N CAS is separate from another human's opaque context version. Owner
readback needs coherent background/current-facts proof or a documented later
generation; a newer context fact ID alone cannot prove an older Owner detail.
Opaque context changes need accepted fact IDs in the actual current/history data.
owner_defined facts stay separate from independently learned user statements
and effective memories. History counts loaded field records, not save attempts.

NPC edits retain the complete supported routine/status/enabled/positionMode
body. Reviewed rebases replace untouched values and retain actual edits.
Actual encounters require explicit confirmation, real place/time/evidence and
life-context revision. Receipts must include encounter/event/relationship/memory
IDs, and readback verifies the event, actual relationship target and memory event
reference before closing. Opportunity retry reassesses its original revision;
it never forces acquaintance. Reference-blocked deletion retains the original
command and offers archive through the actual runtime update contract.

New person creation is definition save -> publish exact version -> read preview
-> explicit apply. Persist only the resource identity/version pointer; reload
reads the actual resource. Never create runtime NPCs directly. Core always
preserves local_overrides, so previews cannot promise to overwrite them. New
creation presents adopted definition values without a keep-null choice. Apply
receipt must prove submitted resource/version/runtime revision; a later current
source is presented as superseding the original, never relabelled as the exact
original version. There is no invented atomic binding-version CAS guarantee.

WorkspaceCommandDialog retains original unknown requests and acknowledged
readbacks. Successful unchanged restored readback clears its draft, avoiding a
phantom recovery banner on the next remount. Inline edits and person command
return paths resolve newly mounted button refs after paint, restoring focus to
the correct connected source. Dirty About cancellation requires a real discard
confirmation. Browser/UI/API evidence under evidence/people/ covers desktop,
390px, five states and equal original body/key across refresh recovery; these
fixtures do not certify live Core/DB/model execution.

## Feed, comments and independently judged sharing suggestions

FeedPage replaces the Browse feed and owns the native /feed/:postId dialog.
Use the formal moments provider's paged list/record/comments/interaction APIs,
never the older unversioned endpoints. Queries bind fid/status/q/cursor; changing
scope resets the cursor. A Fluctlight filter selects owner_fluctlight_id scope,
not author identity. The optional "我发布的" projection filters loaded records by
author_type=user and keeps older-page loading available and labelled.

useWorkspaceRead characterScoped=false permits global Owner reads with no
selected character, retaining Owner/identity/epoch fences. RequestStatus
ownerScoped=true can follow the explicit request/fid while another character is
selected; validate actual request ID and fid, and keep GET observation independent
from mutation admission. Neither option weakens the default character-scoped path.

Post authors, count fields, viewer_reaction, media readability and timestamps are
formal data. like_count is not reaction_count. Missing/unreadable media stays
unavailable; use supplied URL or existing media provider URL only after actual
readability. Preserve momentReading's narrow historical reading projection and
its original-text disclosure; never rewrite stored text or legitimate languages.

Publishing always uses the Owner's real identity. The existing createMoment
worldId parameter is a Fluctlight scope ID in Core, never the runtime world ID.
Attachments retain their original producer scope and do not prove real events.
Sharing suggestions return OwnerRequest, not a published post. Show actual
decision and only expose a publication link from real result effects. A failed
suggestion can seed a separate draft, followed by explicit new submission while
retaining its failure history.

Comment/reaction/hide/restore and NPC retry use WorkspaceCommandDialog with
original body/key and actual object revision. Comment drafts belong to post ID,
not list index. First explicit CAS rejection requires reviewed latest versions;
unknown admission replays the original. Acknowledged readback failures recover
GETs only. Comment receipts are updated moments, not comment objects: verify the
new revision/count and actual author/text, following comments cursors if the
accepted comment has moved to an older page. Never get permanently stuck by
checking only the latest comment page. Preserve newer input during old receipt
recovery. Expose known publication/suggestion draft and command recovery entries
after refresh, and clear resolved records after verified success.

Every confirmed detail refresh adopts the same record into the feed, applying
its filters and refusing older revisions. A hidden detail cannot leave a stale
visible card after close. Hide preserves comments/provenance and blocks new
interaction until actual restore. NPC retry only reassesses the original actual
interaction and revisions; it cannot create acquaintance or physical copresence.

Regressions: demo-replica-feed covers 17 interaction/scope/recovery cases, including
older-page comment proof, late query/ack replies, no-character global publishing
and actual Owner-switch cleanup. Browser/API evidence under evidence/feed/ covers
desktop, 390px, five states and identical original publish retries. These
fixtures do not certify live Core/DB/worker/model behavior.

## Media, visual identity and independent display avatars

MediaWorkspace owns the four supplied-Demo tabs and native asset, visual editor,
identity, generation and avatar feature routes. Global media never inherits a
generic route.fid bookmark value: only an explicit /c/:fid/media path or fid query
sets the library filter. The visual/generation target stays explicit and follows
the shell role when no route target was supplied. Assets, pages and observers use
Owner/role/query/object fences; closing a native deep-link asset preserves its
explicit local returnTo, while an in-place asset dialog keeps its list context.

Use real media-library, metadata, reference-grant, lifecycle and visual APIs.
Asset revision and metadata revision are distinct. Preserve unchanged bytes,
source snapshots and unknown server fields; submit only supported mutable fields.
readable=false prevents image/video/audio URL playback. Known read states have
ordinary labels; initial read failure stays unknown rather than becoming an empty
library, missing definition or missing canonical image. Decorative StoryMark and
WorldArt appear only in empty presentation, never as candidate or generated assets.
The Demo's synthetic favorites and initial-avatar save mode have no formal writes;
do not invent either API or a second authoritative media store.

Visual brief has four text fields, its own revision/history and scoped source
metadata. Saving it does not change a previous candidate, canonical image, body or
wearing. Visual generation freezes expectedVisualRevision/expectedBriefRevision.
The actual response may be pending or queued, despite the older generated TS
literal type. Do not claim supersedeSessionId ended a waiting session: the inspected
Core does not consume that promise. Keep the original received request/session/task
IDs only; never manufacture a task link for a pending queue receipt.

Human review pins session, attempt, candidate and visual revision. Sliders display
percent but submit 0–1 values; require observations/summary/reason and reject missing
sections on acceptance. Validate operation/session/attempt/decision receipt fields.
Accepting canonical is separate from character_sheet_pending and the actual sheet
asset readback. An already later canonical must be described as later, not relabelled
as the original accepted version. If a regenerated session has been superseded, the
actual timeline can prove the original session/attempt's regenerate stage.

Persist the old review's UI context separately from its current candidate. Recovery
entries must remain available after the original attempt has a decision or a newer
attempt exists. Recreate exactly the original review storage scope, preserving
unknown body/key or acknowledged GET-only recovery. The current canReview button
cannot be the only route back to a pending original. Revoked grant rows likewise
expose recovery of their original revoke command; unreadable assets do not hide
their existing original grant/deletion recovery entries.

Generation requests freeze optional item, place and grant versions. Source review
checks the same selection fingerprint at adoption; changed grant revision needs
explicit selection, never automatic upgrade. Suggestion/intent/task completion and
readable asset completion remain separate. Follow only actual request IDs through
GET, pause hidden views, and describe the existing observation limit as a pause.
No arbitrary client cancellation or timer-generated success is available.

Display avatar uses independent avatar and asset revisions. A saved selection is
not proof of a valid avatar: when display-avatar returns avatar=null, respect its
source-identity fallback instead of re-reading current bytes by the selected ID.
Only an explicitly new selection previews current media, checking its chosen asset
revision/version/SHA. Saving the avatar does not alter canonical identity or grant
model reference use. Successful saves refresh the shared product projection.

Grants require a real Owner-owned target, purpose, asset revision, reason and
explicit confirmation. Revoke prevents future consumption and preserves past use.
Deletion checks total protected reference_count, not merely visible page rows;
accepted deletion tombstones the asset and follows the original deletion record.
Only failed deletion with an inactive execution can retry at its own revision.
There is no deleted-bytes restoration endpoint. Upload reuses MediaUpload and its
existing 64MiB/type/fingerprint/original-file recovery contract.

WorkspaceCommandDialog accepts scoped presentation className and a pure resetDraft
for inline generation/avatar forms. Reset edits only the current draft and keeps
the frozen original command/checkpoint. Ordinary modal cancellation is unchanged.
Media forms override the shared two-column lp-form owner so complete composed grids
span their expected width. Dialog return focus ignores disabled triggers and falls
back to the current page heading; inline avatar remount resolves its connected save
button after paint. Actual media dialogs keep the supplied 590/860px bounds and
internal scrolling, including 356px at a 390px viewport.

Regressions: demo-replica-media covers 27 interaction, scope and recovery cases;
updated r10/r13/r16 media checks preserve global scope, original cursor retries and
the new in-place detail behavior. Browser/API evidence under evidence/media/ covers
all four tabs, desktop/narrow layouts, five states and original review retries.
These fixtures do not certify live Core/DB/storage/model execution.

## Studio directory, definitions and sandbox

StudioDirectory and StudioEditor keep the existing studio provider and
createStudioEditor controller. Nine resource kinds use StudioFields; complete
JSON and unknown nested extensions remain available. A malformed venue or place
entry disables structured framework writes and exposes a visible JSON warning.
Event scope/kind changes remove place_id only when the resulting scope is not
place, matching the existing event definition contract.

Draft CAS uses the opaque draft_version. Publish, copy and archive use the numeric
resource revision. Publishing freezes the saved version, and preview reads that
exact immutable publication in parallel with its actual target differences.
Published status alone never proves the current working draft is published.
Historical copies use explicit source=published and the selected sourceVersion;
their new resource is independent and unpublished.

Editor, jobs and lifecycle acceptance pointers are persisted before
persistentCommand clears the pending record. Pointer persistence requires the
original Owner and exact original command identity still in storage, but does
not require the component to remain mounted. Owner cleanup must prevent late
responses from resurrecting cleared records. Accepted readback retries GETs only;
unknown admission retries the original body/key. New saves checkpoint the actual
resource ID in both new and actual-ID draft slots before navigation; subsequent
input remains separate from the accepted definition.

Legacy approved-studio:<owner>:directory-command records remain in their original
localStorage lane with their original DTO/key. Directory recovery uses an explicit
storage provider in createDomainCommand; it must not rewrite those frozen values
into the new command shape. Block new directory archive/restore operations until
legacy pending or accepted recovery is resolved.

Application uses publication.kind. Existing place names and NPC local_overrides
remain locked; NPC world/binding is rechecked before admission without claiming
atomic Core CAS. Generic accepted application receipts can omit target_kind.
Foundation proposed/applied=false is a saved proposal requiring separate review.
Item registration, outfit saving and visual brief updates do not prove inventory,
wearing or generated output. Outfit candidates remain supplemented by an explicit
other-inventory-ID input. Text-template preparation points to the actual assistant
job for review/adoption/save/publication before previewing again.

Sandbox jobs freeze actual current working input. Completed assistant proposals
change the draft only after explicit review. Raw bodyText must be parsed or
withdrawn before adoption, and changes during confirmation invalidate it. Material
retention uses the shared command lifecycle and preserves original jobs/bytes.
Directory filters are Owner-scoped UI preferences; they are not business snapshots.
Dialog source refs handle macOS pointer focus and reconnect after reading.

Regressions: demo-replica-studio, r11-studio and r11-controls. Production fixtures
and 1348px/390px evidence live under evidence/studio in the active task. These
verify UI/API contracts and do not certify live DB, worker, storage or model runs.

## Initialization: complete preview and actual preparation

### 1. Scope and trigger

CharacterWizardPage keeps createCreationController and its durable creation
recovery. CharacterWizard view follows the supplied CreationPage/creation.css:
four icon steps, three world sources, four person sources, grouped preview and
seven preparation stages. Demo source cards supersede the earlier Questionnaire
wizard layout; keep native keyboard-operable buttons and Radix field/dialog
primitives without restoring the old form. No Demo runner, seed or local-success
simulation enters the product graph.

### 2. Signatures

- `previewDefinition()` returns the complete currently edited business preview,
  excluding `analysis_id` and `correlation_id` from the editable projection.
- `updatePreviewDefinition(value, {consumeJson=false}={})` validates the complete
  analysis structure while retaining its real source ID/correlation/schema.
- `activate(true)` replays the original frozen activation body/requestId. An
  accepted `checkpointCreation` persists `activationSnapshot` before releasing
  pending identity; ordinary unmount cannot lose the original accepted input.
- `recoveryPending`, `reviewRejectedRecovery(entry)` and
  `confirmRejectedRecovery()` keep original phase commands reachable and allow
  explicit release only of a proven first pre-admission rejection.
- All mutations remain the existing BrowserClient/directory methods. No API,
  backend schema or LLM prompt signature changes are required.

### 3. Contracts

Existing runtime worlds, exact published world definitions and default daily
worlds are independent explicit choices. Directory cards read exact immutable
publications, not a newer mutable draft body. Failed append retries retain their
original cursor and merge by resource ID rather than duplicate cards. Group
choices independently read the actual Owner directory and preserve an unread or
removed original selection for review.

Unapplied full JSON is a separate durable draft and blocks activation/structural
adoption. Explicit null/false/0, unknown extensions and nested records survive.
Removing/clearing an identity name or timezone does not fall back to older input.
The `previewDefinitionEdited` marker prevents reload from manufacturing a default
for an explicitly missing timezone. Structured editors preserve scalar, object,
array and null types. Initial goal/intention opt-out changes the activation copy
only; initial relationships remain.

A definitive 400/422 or reviewed first stale-analysis rejection keeps
`rejectedPreviewText` as editable/exportable business input without old analysis
credentials. Adopting it as raw JSON requires fresh analysis and preview. Unknown
activation stays in preparation and can only recover the original request.
After activation, previous steps stay locked; view the original accepted snapshot
and recover the same role rather than activating again.

Preparation order is analysis/world/wardrobe/schedule/reference/sheet/avatar.
Only actual ready stages count. World confirmation requires a nonempty string
`worldRuntimeId` equal to the actual current `/world` ID, plus the existing group
and persona checks. Missing IDs cannot match each other into success. A legacy
missing local `worldSelectionConfirmed` does not override a proven world ready
readback; the chat CTA and duplicate source-review prompt use the same actual
confirmation. Ready images still require readable assets.

### 4. Validation and error matrix

| Condition | Required behavior |
| --- | --- |
| No worlds/templates | Honest empty source; explicit alternative source; no implicit create |
| Catalogue/immutable publication read fails | Retain confirmed rows, freeze failed cursor, retry actual read |
| Analysis waiting/cancelled | Preserve input, abort observation, fence late result; no activation |
| Raw JSON malformed or stale against other edits | Preserve raw text and require explicit repair/adoption |
| Activation definitely rejected | Preserve credential-free preview copy; require new analysis |
| Activation unknown | Frozen original body/requestId; no replacement create |
| World ID null/undefined/blank | Never ready; source review/recovery stays available |
| Old local checkpoint missing but actual world ready | Completed state and chat entry agree |
| A phase becomes ready after a lost response | Original recovery entry remains independently reachable |
| First known recovery CAS rejection | Read actual authority, explicitly release old record; no auto-retry |

### 5. Good, base and bad cases

Good: a world revision conflict remains 6/7, then an explicitly reviewed newer
source is copied/selected and actual readback reaches 7/7 with one activation.
Base: cancel a structured edit and retain its original string or null. Bad:
restoring a deleted name, converting null to an empty array without editing,
counting `undefined === undefined`, or losing raw preview edits after rejection.

### 6. Verification assertions

`demo-replica-creation` and `demo-replica-creation-ui` cover source authority,
unknown extension/type preservation, raw JSON fencing, rejected preview copies,
immutable replay, missing world IDs, legacy checkpoints and original recovery
reachability. Retain the existing approved-v62, creation-world, r10-world-reuse,
r8-recovery and R11/R16 contracts. The task's creation evidence contains formal
production dist plus isolated API normal/empty/edit/loading/failure/cancellation,
unknown replay and world source conflict cases, 1348x925/390px screenshots and
request audits. These fixtures do not certify live DB/model/worker/storage runs.

### 7. Wrong and correct

Wrong: `actual.world?.id === draft.worldRuntimeId` alone can count two missing
values as complete; forcing a local flag can then hide a valid legacy chat CTA.
Correct: require a nonempty expected ID, compare with actual current-world
readback and use that same proven ready state for the completed UI and entry.

## Management, notifications, tasks, data and authentication

### 1. Scope and trigger

The supplied Demo's final page group uses native InboxPage, ManagementPage,
DiagnosticsWorkspace, TaskDetailPage and AuthPage. Data and unique object tools
retain the existing bounded DomainPage/DomainPrimitives port. Do not describe
that exception as an all-native React rewrite or restore a second application.

### 2. Signatures

- `operationsClient.notifications(...)`, `pending(fid, kind, cursor)`,
  `readNotification(id)` and `readAllNotifications(before)` retain their formal
  DTOs; `before` is the server's returned `as_of`.
- Management keeps configurationPlan/configuration acknowledgement, original
  command identities, versions, actual field schemas, secrets and quiet-policy CAS.
- WorkflowRecovery consumes actual `allowed_actions`, run ID, revision, reason
  and resettable checkpoints. TaskDetail consumes recorded steps/history.
- Data keeps existing pack validate/export/import/adopt and business backup
  create/restore/retry/cancel providers. No new backend endpoint is implied.
- Authentication uses `setupStatus()`, `auth.setup(token,password)`,
  `auth.login(password)` and the existing logout path.

### 3. Contracts

Notification reads and pending-kind cursors have independent retry state. Read
is not resolve; marking all read must not include events newer than `as_of`.
Same-page details preserve source filters and actual task/proposal links.

Management uses the Demo's 180px secondary directory with a 28px main-column
separation, service/four-purpose tabs, section fields and sticky save/discard.
Actual endpoints, model IDs, supported fields/enums and configured secrets are
formal data. Never implement a saved endpoint deletion or purpose transfer
without the existing corresponding command. Initial read failure prevents save.

Task progress is recorded stage state, never a percentage inferred from status.
An accepted restart/reset is not completed work. Actual original requests and
execution records remain accessible through explicit feature routes.

Packs create independent definition drafts; import binds the exact original
JSON and validation digest and does not automatically publish, apply or replace
runtime data. Deployment backups require their actual operator and explicit
isolated restore target. No browser snapshot overwrite/reset, fake download,
backup deletion or timeline rollback is available.

Auth follows the source's two panels and <=900px single-column rule. Enforce
12-character password, >=16-character setup token and exact confirmation. Keep
passwords in the current form only. Unauthenticated App returns AuthPage before
private directory reads. Do not copy the Demo's fake email/register/guest paths.

### 4. Validation and error matrix

| Condition | Required behavior |
| --- | --- |
| Notifications or one pending kind fails | Independent error and original-cursor retry; unknown is not empty |
| Mark-all concurrent with newer events | Use the loaded server as_of; do not manufacture a local cutoff |
| Configuration read/save/ack fails | Retain inputs, actual versions and original-command recovery |
| Workflow action unavailable | Follow actual allowed_actions; no status-derived permission |
| Task is queued/accepted | State receipt accurately; no fabricated progress/completion |
| Pack input changes during validation | Require validation of the new exact input |
| Import accepted but readback fails | Restore original accepted result with GETs only |
| Setup/login fails | Preserve current input and explicit retry; no credential persistence |

### 5. Good, base and bad cases

Good: a failed task opens its original workflow and explicit reason/checkpoint
confirmation, then observes the actual accepted command. Base: an empty inbox
has no invented edit command. Bad: marking a notification read resolves the
proposal, or importing a pack overwrites a running role.

### 6. Verification assertions

`demo-replica-inbox`, `demo-replica-management`, `react-flows`, `r11-flows` and
retained data/auth regressions cover actual commands, drafts and recovery.
The replica task records normal/empty/edit-or-filter/loading/failure, keyboard,
1348x925/390x844 and request audits. Group16's corrected same-size Demo images
are `check-demo-*`; earlier differently sized reference images are historical.
Loopback fixtures cannot certify live Core/DB/Temporal/model/NAS/operator runs.

### 7. Wrong and correct

Wrong: status=failed implies restart permission and 33% progress.
Correct: show real steps and obtain permission/run/revision from WorkflowRecovery.

## Durable domain receipts, Owner cleanup and field focus

### 1. Scope and trigger

Shared bounded domain tools compose the single React overlay and primitive host.
A command can have an accepted mutation and a failed later read; these are
separate states across close/reopen and refresh.

### 2. Signatures

- `durableCommandDrawer({slot,title,body,read,execute,onSaved,...})` keeps
  `execute(value,idempotencyKey)` as its only mutation and `onSaved(result)` as
  readback/navigation/UI merge only.
- Pending key: `fluctlight-studio:${owner}:${slot}:command`.
- Accepted key: pending key + `:accepted`, record
  `{version:1,idempotencyKey,value,result,inputSnapshot}`.
- Domain field bridge dispatches native `FocusEvent('focusout',
  {bubbles:true,relatedTarget})` on its original hidden carrier.

### 3. Contracts

Checkpoint the accepted result inside execute, while the exact pending identity
still exists, before persistentCommand removes it. Disable new submission until
readback succeeds. Reopening or refreshing retries onSaved of the original
result, never execute. Preserve edits made during both submit and readback.

A continuously observed Owner departure invalidates that drawer's session even
if the same Owner later returns. Check exact stored identity before storing or
clearing a receipt; late responses cannot resurrect cleared recovery or delete a
newer slot. Logout/Owner changes clean exact original-Owner prefixes including
approved-config-command and approved-policy JSON scopes and management-view.
Ordinary navigation does not clear them; other Owners' keys remain intact.

The visible field owns actual focus. On blur, sync selection on the hidden
carrier using its native prototype, then forward the real relatedTarget.
Explicit source.focus()/setSelectionRange recovery still forwards to the visible
control. This preserves Radix focus traps and Tab between multiple fields.

### 4. Validation and error matrix

| Condition | Required behavior |
| --- | --- |
| Unknown mutation response | Retain frozen original body/key |
| Accepted result, failed GET | Persist receipt; offer readback only, including after reopen |
| Malformed accepted checkpoint | Visible error; do not admit a replacement mutation |
| Logout followed by same-Owner entry | Old completion cannot recreate or clear command slots |
| Tab first field to second inside Sheet | Second visible field keeps focus and accepts input |
| Blur selection sync | No focus jump to the first visible control |

### 5. Good, base and bad cases

Good: pack import acceptance survives a failed GET and refresh without a second
import. Base: ordinary close retains the original unknown command. Bad: clear
pending, lose receipt, then re-enable submit after the read fails.

### 6. Verification assertions

`demo-replica-management` mounts real OverlayHost/Sheet and checks durable
accepted readback, newer input, logout/same-Owner reentry, exact cleanup and
multi-field focus. Existing shared primitive/IME/original-command regressions
must continue passing; browser evidence includes actual Tab then Escape.

### 7. Wrong and correct

Wrong: dispatch `new Event('focusout')` or run execute again after onSaved fails.
Correct: forward FocusEvent.relatedTarget and recover only the checkpointed result.

## Retained source routes and exact chat messages

### 1. Scope and trigger

Native pages must consume the query state returned by retained object tools.
A route regex resolving successfully is not proof that its source object opened.

### 2. Signatures

- `/c/:fid/chat/history?q=...` and `/c/:fid/chat?conversation=:id&message=:id`
  share ChatPage and ConversationHistory in the same role-keyed lifecycle.
- ConversationHistory accepts `initialQuery`, `initialConversationId`,
  `initialMessageId`; uses existing conversationDirectory, conversationMessage,
  messages and explicit selectDirectConversation methods.
- WardrobePage receives route.query and restores `q`, `state=archived|current`
  and stable `clothes/garments`, `plans/outfits`, `compose` aliases.

### 3. Contracts

Confirm the requested source conversation in the current role's directory before
reading its exact message. If it is on a later directory page, retain that
explicit request and offer further directory reads; never substitute another
current session. Validate returned message ID, conversation ID and integer
sequence. Read its message window with beforeSequence=sequence+1, keep the
actual anchor if the page omits it, and label the source message. Retry freezes
the original ID/window even when a newer search draft exists.

Opening, searching, retrying or dismissing the reader never selects/resumes a
session. Explicit continue remains separate. Close returns to safe local
returnTo, otherwise that role's chat, without remounting/losing the composer.
Owner/role/route keys and read epochs reject old results. Wardrobe return queries
update the native controls even when the component remains mounted; an in-place
native garment dialog retains its existing local filters and list position.

### 4. Validation and error matrix

| Condition | Required behavior |
| --- | --- |
| Source session absent from first directory page | No different session read; permit directory append |
| Source message ID/conversation mismatch | Error without displaying foreign text |
| Anchor GET fails | Retry same original anchor; preserve unsent search/composer |
| Source reader closes | Restore original source query; no automatic resume |
| Wardrobe return q changes | Actual search control and displayed garments both change |
| Owner/role or source route changes | Dispose/remount or reject late read by scope/epoch |

### 5. Good, base and bad cases

Good: shared experience opens its recorded conversation/message and Escape
returns to shared?view=records. Base: ordinary history opens the real current
session for browsing. Bad: ignore source IDs, display the latest chat and claim
the original exchange was opened; retain q in URL but show an unfiltered wardrobe.

### 6. Verification assertions

`demo-replica-chat` covers exact anchor/window, same failed retry, later directory
membership, foreign message rejection and no auto-selection. Existing history
cursor/draft and scope tests remain required. `demo-replica-wardrobe` verifies
query search and plan-tab aliases on the actual controller/view. Final browser
samples record shared source, Escape return, 356px dialog at390px and wardrobe
search restoration. Static55-route inventory supplements these interactions.

### 7. Wrong and correct

Wrong: treat /chat?message=source as ordinary current chat or read current session
when an explicitly named source is absent from the first directory page.
Correct: use the shared read-only history reader and confirm the exact source.

## Fullsite audit: shared presentation and notification ownership (2026-10-28)

### 1. Scope and trigger

The second pass uses the immutable supplied Demo at identical content/viewport.
Its migrated replica styles share the same unlayered presentation cascade; the
older react-layout remains in the components layer. Wrapping only the management
replica in components lets unlayered shared control rules suppress its intended
sizes/colors. Inspect computed rules before changing layer ownership. Keep
`.settings-group[hidden]` protection and existing primitive disabled semantics.

### 2. Signatures

- `Sidebar({returnFocus?: () => HTMLElement | null})` restores the connected
  mobile trigger from `SheetContent.onCloseAutoFocus`.
- `ChatAttachment({src, alt, onOpen})` keeps one AttachmentTrigger node across
  loading/error/retry. The media URL remains the original authenticated URL.
- `workspace.refreshNotificationCount()`, `markNotificationRead(id)`, and
  `markAllNotificationsRead(before)` own global notification synchronization.
- Notification count GET returns `{count, alert_count}`; list role filters use
  the existing `fluctlightId`. Read-all freezes the server-observed `as_of`.

### 3. Contracts

Directory `identity.occupation`, color and `last_conversation_at` remain in the
projection even before a role detail loads. Unknown timestamps stay unknown.
Global notification count uses Owner/session generation and request epochs,
independent of character selection. A confirmed write invalidates older reads
and starts count refresh without delaying or reversing the successful write.

Creation's ordinary profileFields excludes shared personality. Its sliders
therefore update `previewDefinition().core_persona.personality` through
`updatePreviewDefinition`; they must not look for a nonexistent profileField.
That path retains original analysis authority, the complete nested structure,
and unknown extension facts. Pending full-JSON edits disable grouped edits.
Creation preview themes remain one horizontally scrolling row at narrow widths;
identity follows the Demo's known field order, with remaining fields retained.

### 4. Validation and error matrix

| Condition | Required behavior |
| --- | --- |
| Image fails, keyboard retries | Same focused button remains; original URL reread |
| Mobile navigation Escape | Restore a connected, enabled source trigger |
| Notification count fails after read commit | Keep committed read successful and last confirmed count |
| Owner A → B → A or pre-write count returns late | Reject obsolete generation/epoch |
| Shared personality number exists | Slider edits the full preview and persists it |
| Full JSON is unapplied, activation accepted or Owner changed | No new grouped preview mutation |
| No actual diary title/mood or personality dimensions | Show supported missing state; do not seed Demo facts |

### 5. Good, base and bad cases

Good: a 0.72 personality slider changes to0.73, full JSON and refresh retain0.73
while relationships/extensions remain equal. Base: absent personality offers an
explicit setting action. Bad: disable every slider because profileFields has no
personality, or replace the failed-image retry button and drop keyboard focus.

### 6. Verification assertions

`demo-fullsite-shared`, `notification-counter`, `approved-workspace`, and
`demo-replica-inbox` cover draft adoption, same image node/URL, confirmed counters,
Owner races and server filters. Browser checks use production dist, including
700/701 boundaries, keyboard image retry and full-preview persistence. Screenshot
creation alone never certifies visual parity; raw pixel differences have no
completion threshold. Controlled HTTP/JSDOM evidence never certifies live Core.

### 7. Wrong and correct

Wrong: `fields.find(f => f.path[0] === 'personality')` gates creation sliders, or
badge=`notifications.filter(unread).length` caps total to the first list page.
Correct: update the complete editable definition through its existing controller;
consume the separate Owner-scoped authoritative notification counter.

### Complete groups inside command forms

`WorkspaceCommandDialog` renders `.lp-form` as a two-column grid. A renderer
returning a complete group (WorldForm, NPC edit/create/contact/apply, feed
composer) uses the existing `lp-wide` class on that group's root. Keep its theme
scope on a portaled root: feed uses `replica-feed companion-form-stack lp-wide`;
people uses `replica-profile replica-people companion-form-stack lp-wide`.
Rules for a scope and layout on the same node need `.scope.layout`, not only
`.scope .layout`. Otherwise the portal loses flex spacing even after spanning.
Inspect existing effective rules first: media command forms already have a
specific single-column `.replica-media .lp-form` rule, so adding span classes
there is redundant. Never assume a missing class alone proves a layout defect.
Browser assertions compare the rendered group and form widths and check20px
stack gap at desktop/narrow widths; do not replace them with class-name regexes.

Malformed goal evidence/history responses must not store an undefined `items`
and crash the selected detail. Reject missing/non-object records before adoption,
keep the confirmed goal, and retain the original read retry. The actual React
regression in `demo-replica-goals` verifies this recovery without a new mutation.

### Confirmed read views and publication history

When an unknown CAS-only memory correction is explicitly reconciled through
`readLatest` and `confirmLatest`, adopt the validated current object into the
reader and refresh its list/history/source revision as well as the command's
baseline. Keep the current editable text/reason. This is a read reconciliation,
not proof that the lost command succeeded and not permission to repeat it.
Closing the command must not return to stale prose/version after that explicit
confirmation. `demo-replica-memory` checks one mutation and current read views.

Studio's immutable version reader includes Owner, resource ID and the actual
`published_version` in its cursor scope. Publishing a new saved version must
refresh history without remounting the editor; old exact versions stay readable
and can create independent unpublished copies. `demo-replica-studio` checks both
the previous and new publication in the same mounted page.

### Primitive line boxes and narrow headings

The supplied Demo's ordinary Surface inherits14px/1.5. The local Card primitive
adds `text-sm`, so the replica Card rule explicitly owns1.5 rather than inheriting
its20px utility line box. Typography inside cards still uses its own documented
sizes/line heights. Chat metadata uses1.5, attachment captions1.95 and timeline
secondary text1.5. Profile narrow page gap is18px; its tag variants remain
distinct and the current-persona status uses natural height. The original
progress indicator's effective color comes from Demo index.css (`#5d8766`),
not only the primitive's `bg-primary` class.

At<=900px page headings align actions to the top, and<=700px retains wrapping.
Keep formal refresh actions compact in their corresponding controls when they
otherwise force an extra heading row. Search clear and refresh remain separate
labelled buttons with reserved input space. Explanatory `.replica-info` directly
inside `.lp-form` spans both columns. Diary index excerpts have a bounded width
and min-width0 so ellipsis works under the grid's start alignment; the full diary
body is unchanged. Production-build desktop/narrow geometry remains the gate.

### Retained tool portal themes and decision summaries

OverlayHost and the disclosure Sheet port put `replica-overlay` on the actual
portaled content root. This is a typography/control theme, independent of Dialog
or Sheet positioning. Sheet uses the existing `product-sheet` width and a
separate wide width; only `[data-slot="dialog-content"]` receives centered
Dialog dimensions. Keep the retained original node, one primitive host, Owner
cleanup, focus bridge and command recovery intact. Theme rules retain a visible
focus outline and normal13px/400 input text.

WorkflowRecovery composes WorkspaceDialog with a real labelled form and stable
form ID for the footer submit button. Busy guards still prevent dismissal;
returnFocus records the actual opening button for both action and rejected
command review. A control receipt means accepted submission, not task completion.
Schedule's conflict review emphasizes the frozen proposal's activity, interval,
place and reason; its full original protocol remains in an explicit disclosure.
Neither presentation changes the original key, CAS baseline or explicit rebase.

Confirmed Goal edits, state commands and a matched evaluation terminal refresh
the selected detail's evidence/history through its existing kind epochs. New
selection already loads records once; closed or other goals are not refreshed.
Secondary record failures retain confirmed goal data and their own read retry,
without repeating a mutation. State command readback must reach the receipt's
revision, and same-version GoalStatus receipts must match the actual goal state.
Evaluation receipt `pending` is a request status and is excluded from that goal
status comparison. A stale primary read remains accepted-receipt GET-only recovery.

`durableCommandDrawer({destructive=true,...})` preserves its existing visual
default; independent resource-pack import explicitly uses false. This option
affects only the displayed button variant, never execute/read/accepted receipts.
Admin browser evidence checks actual portaled widths, typography, variants,
focus, cancellation and controlled original-command submission.

Creation's known `session.status=awaiting_review` explains the human decision and
labels unread reference/sheet links `前往审阅候选`. It does not count them ready,
does not hide independent source read failures and does not demote already
confirmed readable images. Source reread is independent; a failed initial world
binding can still require the explicit original world preparation entry, without
a second activation. A discarded Studio job is explicitly `已丢弃`, not unknown.

## Creation route entry identity

The wizard controller reads world/template/resume query inputs when mounted.
App keys CharacterWizardPage by path+query, so a same-page historical resume
releases the prior controller and reads the correct archived creation. The existing
Owner-scoped archive and original activation remain authoritative; changing the
URL must never replay activation or erase the separate current draft.
Browser regression: creation-supplement snapshot-completion desktop/mobile.

## Retained nested controls and responsive tools

DomainPrimitives checks root.contains(source) for each static scan candidate.
An ancestor Disclosure temporarily moves original children out of root; admit
its nested candidates only after the portal content is mounted back. This keeps
the original form and listeners; JSDOM contract checks alone do not reproduce
the browser scheduling defect, so verify the real diagnostic maintenance region.

A failed cancellation can leave the turn running. Keep its explicit unknown
result visible during waiting, and keep recovery a read of the original turn.
Do not infer that a failed cancel or a reread has cancelled the server task.

Button theme matching accepts actual button/a elements with the Button size and
variant markers as well as its original data-slot. Radix composed triggers may
overwrite that slot; keep theme specificity low so page geometry remains owned
by its named classes. Preserve mobile card breakpoints after semantic primitive
styles, secondary navigation as one horizontal scroll row and compact toolbars.
Moved refresh buttons inside forms must explicitly type=button, retaining newer
unsubmitted search input. The narrow diary filter operates the same favorite
state and reader as the desktop checkbox; it does not create another cache.

Demo test-art extraction keeps the original decorative paths and MediaImage/Art
wrapper. Raw wardrobe SVG and the media outfit branch are different resources.
Such fixture corrections do not alter reference images or production media.

## Source-based status presentation

Studio directory presentation uses the actual published_version independently
from row.status; archived rows stay neutral even when an older publication exists.
Do not change the domain status to produce a green badge. Inbox icons consume
explicit committed source_kind values and the existing five pending group kinds;
unknown values preserve a neutral icon and readable content. Map lookups must
check own properties so source names such as constructor/__proto__ cannot become
React component values. Never infer a new notification event from a Demo title.

Role menus retain actual name/job values. Long selected text keeps its full title
and menu copy while the sidebar width stays bounded. Memory search is320px wide
at most on desktop; local filter wrappers keep the supplied130px geometry. Inbox
uses its supplied SegTabs presentation and the original narrow typography/icon
sizes while Radix continues to own keyboard and selection behavior.
