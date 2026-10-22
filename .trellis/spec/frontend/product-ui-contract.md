# Product UI And Interaction Contracts

## 1. Scope / Trigger

Changes to shared controls, forms, resource feedback, guided settings or diagnostic presentation in `apps/web`. Read with component/state and Goal/Actor governance guidelines.

## 2. Signatures

- Product components: `FormField`, `ChoiceSelect`, `FeedbackNotice`, `AsyncState`, `ConfirmDialog`, `FilterPanel`, `JsonViewer`, `WorkspaceState` under `components/product/`.
- `FormField` slot `controlAttrs` supplies `id`, `aria-invalid`, `aria-required`, `aria-describedby`; bind it to the actual Input/Textarea/SelectTrigger.
- `useFormErrors().validate(FieldRule[])` validates and focuses the first error after DOM updates. Rules support code-point `maxLength` and UTF-8 `maxBytes` for the corresponding server contract.
- `ChoiceSelect<T>` accepts exact domain option values and emits only a configured value. Internal `choice:` prefixes preserve an empty domain option without sending a UI sentinel to the server.
- `mediaSettingsPatch` and guided autonomy helpers merge open objects; queue/wakeup serialization rebuilds closed schemas.

## 3. Contracts

- `globals.css` maps UI primary/foreground/ring to product tokens. UI hover accent is separate from brand `--accent`; product rules load after foundation/application CSS. Default Button foreground must remain readable despite unlayered CSS inheritance.
- Default controls are 44px high; compact actions/tabs are at least 40px. Use Lucide for functional icons. Forms and long Select values fit 320px workspaces without internal horizontal overflow.
- Use `novalidate` with explicit inline validation; retain required validation. Required badges are visually present and `aria-hidden`; `aria-required` belongs to the control.
- Loading precedes ready-empty. Initial failures have a visible retry action. Do not expose temporary defaults as saved settings before the first successful read.
- Creation, directory, settings, diagnostics, moments and governance errors have separate owners. Dismissed creation requests are consumed; late responses cannot reopen them or pollute another surface.
- Resource identity changes invalidate local feedback. Model-role switches clear field errors and increment the feedback epoch, even when the section does not change. Disable role switching during save.
- Confirmation failures render inside the still-open modal. Cancellation emits no mutation, clears its action error, and returns focus to the real DialogTrigger.
- Actor editing has one primary entry and all ten fields. Trusted Owner identity comes from `session.actorId`, is cleared on unauthentication, and selects current-facts CAS. Other Actors retain context-version CAS. Owner reasons are at most 500 code points; other Actor reasons at most 1000 UTF-8 bytes. Draft/version changes require a new idempotency fingerprint; 409 keeps the draft. Empty targets fence old requests.
- Open media/autonomy objects preserve unknown fields, workflow/stage maps, custom actions and advanced values. Freeze import target before `await file.text()`; successful clear/save synchronizes the local workflow draft.
- Wake-up is an enabled switch plus fixed 600-second interval. Effective diagnostic retention is read-only 30 days / 10000 rows; stored retention settings currently have no runtime consumer. Do not advertise unsupported editable behavior.
- Lifecycle/Kev inputs remain local drafts until Apply. Polling, load-more and export use the applied snapshot. Pagination keeps independent source epochs and pauses polling after older pages.
- Workflow rows use Workflow+Run identity, show known status/time and explicitly unknown legacy fields. Existing query/control targets current execution, not a historical Run.
- JSON defaults collapsed, uses safe text bindings and owned scrolling, and keeps unknown data reachable. Read-only details expose no mutations; only their bounded body scrolls.
- Nonessential motion has a loaded `prefers-reduced-motion` override. Browser evidence distinguishes actual preference/zoom activation from CSS inspection and equivalent viewport reflow.

## 4. Validation & Error Matrix

| Condition | Behavior |
| --- | --- |
| Empty/whitespace or invalid field | Inline Chinese error, focus actual field, no request |
| Initial Actor read fails | Retry is visible even when context is null |
| Clear diagnostics fails | Keep dialog and display failure inside it |
| Model-role identity changes | Clear old errors/notices; ignore old-role completion |
| Owner reason has 501 code points | Reject before request |
| Other Actor reason exceeds 1000 bytes | Reject before request |
| Filter draft changes during polling/paging | Continue using applied snapshot |
| Media import target changes during read | Commit to captured target |
| Save removes visual workflow | Clear old draft; subsequent save cannot resurrect it |
| 320px long select value | Constrain trigger/value; no internal overflow |

## 5. Good / Base / Bad Cases

Good: Owner editor sends all ten fields using current-facts revision, and the actual browser route accepts them. Base: unknown relationship state remains null. Bad: fixture accepts a payload rejected by the route validator, or role B displays role A's late error.

## 6. Tests Required

- `ui-contracts.test.mjs`: executable parsers/patches and real SFC/store SSR for loading, accessibility and scope.
- `ui-interactions.test.mjs`: async import target, workflow clear, applied pagination, date focus, exact credential text, empty choices, lifecycle drafts, role epochs and modal errors.
- `goal-planning-panel.test.mjs`: Owner/Actor CAS, payload identity, empty-target fence and reason limits.
- Actual Go route/validator tests cover ten Owner fields, null/false, unknown-field rejection and 500/501-character reasons. Fixture success alone is insufficient.
- Typecheck/test/build; serve production dist using loopback fictional fixtures. Use CUA for browser interaction; never mutate real user data during QA. Record widths, success/failure/cancel, draft/scroll/focus and console results with limitations.

## 7. Wrong vs Correct

Wrong: bind a filter input directly to store-applied filters and poll automatically.

Correct: keep a local draft, copy to applied state only on Apply, and use applied state for continuation.

Wrong: place an operation error behind its open confirmation modal.

Correct: render the action error within the modal slot, with retry and cancel reachable.

## Current v3 setup and connection workspace (2026-10-20)

- Keep the accepted v3 components and sage palette. Settings fields/actions wrap
  inside their owning container; 390px verification records actual viewport and
  document scroll widths, not a resized screenshot alone.
- A selected connection owns address, secret purpose, model directory and role
  draft. Fixed endpoint context must survive reset/rebase/adoption. Changing
  endpoint clears dependent model/window; changing model clears its former
  loaded window. Do not change a frozen pending request to fit a newer draft.
- Exchange/management are navigation workspaces. Sidebar, more menu, mobile
  drawer and direct routes use `workspace-navigation.mjs`; management exposes
  settings, diagnostics, data and setup. Server authorization remains unchanged.
- Setup links service configuration → published world definition → character.
  Exact source/version enters analysis and later per-character runtime apply,
  select and readback. World changes invalidate uncommitted analysis; accepted
  character and original pending commands retain their identities.
- World/group continuation failures keep the accepted character and retained
  checkpoint. They offer continuation without another activation. A character
  with no selected world can explicitly add that source later in the guide.
- Initialization feedback consumes error code, validation path and correlation
  and links filtered model diagnostics. Malformed warning entries must never
  throw during preview; preserve original values in full JSON.

## Semantic character presentation and preparation progress (2026-10-20)

- A shared renderer is not sufficient if its primary output is raw JSON. Profile
  and intention components show Chinese domain labels, narrative settings,
  real lifecycle and linked goals. Keep unmodified source/IDs/hashes/versions
  under collapsed technical details. Never invent traits for null fields.
- `SemanticFields` reuses the shared display dictionary. `IntentionRow/List`
  owns the goal, current-life and promise presentations. Candidate intentions
  are not accepted executions; unknown states stay unknown. Numeric zero is
  preserved; null confidence does not become 0%.
- Persona names resolve from declared profiles; current persona comes only from
  personality runtime. Effective appearance null never falls back to initial
  appearance. Switch success/conflict/unknown-result feedback must remain visible
  even when controls are progressively disclosed.
- `readInitializationProgress` reads visual identity, avatar, current-session
  timeline and its actual task independently. `mountInitializationProgress`
  refreshes only its card, so persona draft and main profile are preserved.
  Stop on disposal; pause while hidden; bound automatic polling. No read starts
  a generation. Candidate assets are not accepted references, skipped review is
  not successful validation, queued is not evidence of Worker execution.
- Complete reference/sheet/avatar milestones require their real artifacts or
  readable avatar projection. Optional read failure is explicitly unknown,
  never a claim of no asset. Keep task/review/recovery navigation available.
- Settings subsections declare `backHref/backLabel` in the route and expose one
  shell-owned return control. Normal pages retain history behavior. NPC management
  uses `/people/npcs`, with `/people/manage` as a same-renderer compatibility URL.
- Page/card spacing belongs to `stack/grid` parents, not global card margins.

## NAS report regression contracts (2026-10-20)

- Generic recursion cannot replace entity presentation. Initial `wardrobe_items`
  describes initial availability and initial wear; effective `worn_items` requires
  known wearing state and verified available rows. Known-empty wear and unknown
  wear have different copy. Current body uses known/unknown/cleared wrappers and
  must not fall back to Foundation. Use contextual labels for booleans, not
  orphaned 是/否 or category/slot tokens. Preserve authored narratives under open
  or custom keys with a neutral Chinese label; technical fields remain folded.
- Ordinary `userInstant` output follows the latest confirmed Beijing format in
  `pen-interaction-contract.md`. Business local-day calculations still choose
  schedule.timezone before context.timezone; ordinary instant inputs display
  Beijing wall time without changing that business zone. Compact ranges retain
  both dates across midnight. Precise diagnostic/source timestamps retain raw
  precision and offsets in their own technical context and unchanged commands.
- Diagnostic lifecycle/task labels use the domain presentation helper. Completed
  model/worker runs are not a claim of completed business effects. Primary cards
  show readable time/status/name; raw keys, IDs and error codes stay available in
  source details. Home and cursor streams each need their own stack gap.
- Autonomy fields have distinct contact/action/frequency/quiet sections. Scoped
  switch layout must retain names, data selectors, inheritance disablement,
  budget omission and CAS/pending-draft semantics.
- Runtime capability definitions currently arrive with Go PascalCase fields;
  configuration provider normalizes display aliases, preserves sourceContract,
  and remains compatible with lowercase DTOs. Do not change shared Go JSON tags
  merely to fix a browser title. Catalog, goal history and creation review default
  to readable summaries with closed technical JSON.
- NAS route visual coverage and functional regression are separate evidence.
  Missing typed resources block detail routes; local report-shaped replay does
  not upgrade them or prove deployment acceptance. See nas-ui-report.test.mjs.

## V3 experience continuity and responsive review (2026-10-21)

The current production owner is `v3/proto/app.js` plus `styles/v3-proto.css`.
Retain the accepted sage tokens; shared changes belong in native v3 components,
not the historical Vue views. This pass does not rename API fields or states.

- `Field` supplies independent control IDs and combines caller descriptions with
  help IDs. Required indicators exclude disabled/readonly controls. TextComposer
  uses a textarea: Enter sends, Shift+Enter inserts a newline, and composition/
  keyCode229 never submits. Stream updates must keep the composer mounted.
- Overlay focus candidates include native `summary` as well as controls and
  links. A drawer containing only Close and collapsed records must still allow
  Tab/Shift+Tab to reach every summary. Only the top layer owns focus; lower
  layers/background are inert, body scrolling stays locked until the last close.
  Cancel retains outer drafts and returns focus. Async owners disabling a trigger
  before opening confirmation must restore that actual trigger after re-enabling.
- Chat has one bounded message scroll owner and a reachable composer. A located
  historical message belongs inside `.stream`; a long original cannot occupy a
  sibling flex row that pushes the composer outside the viewport. Pixel reading
  offsets are reusable only with the same conversation and message ID sequence;
  re-reading a bounded latest page must not reuse a previous expanded-history
  offset. History pagination is not a full cached transcript.
- `zonedInput(instant, zone)` displays the schedule's explicit zone.
  `zonedInputInstant(local, zone, original)` preserves the original instant string
  when the displayed minute is unchanged, including its seconds, milliseconds
  and numeric offset. Exact/local modes have one enabled field pair at a time;
  switching back records the exact input as the new original. DST gaps/folds
  require explicit correction or an offset. End <= start focuses the end field,
  preserves the draft and sends no request. Provider CAS/idempotency stay owners.
- Reading cards may use `InstantLabel`/`readingInstant` for short dates, retaining
  the full `userInstant` in title/aria and the original source. Diagnostic time
  formatting keeps milliseconds, offset and selected IANA zone.
- At <=760px, sibling tabs and search get their own toolbar rows. Long entity
  names keep a readable identity row; status and multiple actions wrap below.
  No horizontal document overflow at390px; additionally check320px controls and
  an intermediate desktop/sidebar breakpoint. Check actual innerWidth and
  document scrollWidth, not screenshot scaling. Nonessential motion respects
  the loaded reduced-motion override; do not claim an OS preference was toggled
  from CSS inspection alone.
- Diagnostic filters remain draft inputs until Query. Phone entry with no URL
  filters may start collapsed so records are visible; supplied URL filters open
  the panel. Polling/export/pagination still use the applied snapshot. Technical
  raw fields and low-frequency maintenance stay available through disclosure.
- Global media uses `ctx.params.charId` only for an explicit character route;
  the shell's selected character is not global asset authority. A selected
  character must not silently convert `/studio/media` into that character's list.
- A failed lazy module import offers an explicit full-page refresh. Retrying the
  same cached rejected import after deployment cannot recover removed chunks.

Verification: `v3-ux-scope.test.mjs` covers global/character media authority,
Field identities and fallback character navigation. `zoned-input.test.mjs` covers
precision/offset preservation, DST gap/fold and invalid dates. Production-image
CUA checks cover nested cancellation, summary focus, exact/local round trips,
chat draft/history and reading position, configuration cancellation, responsive
surfaces and module-refresh recovery. Missing real resources are blocked in the
route matrix, never counted as real detail-page acceptance.

## 2026-10-21 wardrobe and world workspace

Wardrobe uses shared composer/library/preview/preparation components, with tabs
为她搭配/方案库/全部衣服. Actual wearing is its own authoritative read, separate
from saved plans. Compose one ordinary garment per slot and multiple accessories;
selected archived accessories stay removable. New standard types and seasons
use Chinese choices; old custom garments stay visible. Plan/tag drafts and
commands use stable Owner/fid/profile/operation identity. Store only presentation
input and frozen pending commands in session storage, never a second business
snapshot. Closing/remounting/refresh preserves unknown commands for exact replay;
a first explicit CAS rejection needs actual reread and explicit rebase. Old
acknowledgement clears only the exact submitted draft, preserving newer edits.
Name-only PATCH does not resubmit archived members. Logout/Owner change clears
wardrobe, world-framework and runtime-framework draft/command prefixes.

World editor reuses the fixed category directory and concrete-point preview.
Categories have no location link or edit control; existing canonical point name
is readonly, alias and descriptive fields are editable. Raw objects are folded
and must never show [object Object]. Generating a framework first produces a
reviewable proposal, then saves/applies through the owning existing commands.
Global hover links have no underline per the latest user decision; keyboard
focus outlines remain. Browser fixture replay and real NAS/model acceptance must
be recorded separately; unavailable browser transport is an unverified check.

## Scenario: UX report errors, validation and recovery (2026-10-22)

### 1. Scope / Trigger
Production v3 forms, asynchronous reads, configuration tests, long-running
requests and world-definition composition. Source report9989026 predates the PC
corrections; reconcile current behavior before reimplementing an item.

### 2. Signatures
- `errorMessage(error,fallback?)` from `v3/domain-presentation.mjs` supplies the ordinary error summary.
- `installFieldValidation(root=document)` returns event-listener cleanup; native constraints and page `reportValidity()` remain active.
- `asyncPage(el,read,paint,{backHref,backLabel,errorTitle?,errorFallback?}={})` keeps the original three arguments compatible.
- `mountCursorRecords(box,{...,errorFallback?,onError?})` owns its independent epoch, cursor and last successful page.
- `durableCommandDrawer({...options,originalOnly?,onPendingChange?})` can expose only recovery of an existing pending identity.
- `configurationTestSummary(result)` classifies actual test evidence; `diagnosticTimeFilter(text)` converts ordinary Beijing input to an explicit instant.
- `worldDefinitionError(body)` validates current ordinary world fields and optional `world-framework.v1` using Core's real types and limits.

### 3. Contracts
- Keep Chinese business errors; map known HTTP/network/error codes for ordinary reading. Raw source remains optional technical data, never interpolated unescaped.
- Capture native invalid feedback as Chinese inline errors and focus the first actual control. Synchronize whitespace-required constraints before footer click/Enter/reportValidity; clear only the shared layer's own constraint when repaired, retaining page-specific errors.
- Initial read failure removes loading and provides retry/explicit business return. Cursor refresh failure retains its last successful records and position. Disposed/old-scope reads do not repaint.
- A healthy saved configuration is not evidence of a successful test. Failed call/error/last_error takes priority; completed worker execution is not an image, reflection, publication or applied effect.
- Long waiting presents actual elapsed time and available original-request query/cancel. Never invent a 30-second terminal failure or generate a replacement identity. Recovery-only UI with no pending command cannot admit a new command.
- Bell badge counts unread notifications only; pending counts remain separately labelled. Global NPC definition creation is Owner-authorized resource creation and does not require a runtime character. Runtime apply still requires its real target, versions and changed fields; existing runtime+empty diff disables and guards submit.
- World resources cannot save/publish an empty ordinary definition in the editor. Optional framework keys/names must be strings before length/uniqueness checks. Keep unknown fields and historical server publications; do not synthesize a default world or change backend legacy compatibility.

### 4. Validation & Error Matrix
| Input / state | Required result |
|---|---|
| Pure whitespace required field, footer action | Chinese inline error, focus, no command |
| Page-specific custom validity exists | Shared repair does not erase it |
| First read rejects | No permanent loading, visible retry and business return |
| Later page refresh rejects | Last successful page/cursor remains |
| Test nested last_error with healthy configuration | Visible failed summary near test |
| Existing runtime NPC has no diff | Disabled button and handler guard |
| Original-only recovery has no identity | No new submit or empty-body command |
| World framework key/name is numeric | Reject, do not coerce with String |

### 5. Good / Base / Bad Cases
Good: original turn is still processing, show elapsed time and query that exact
turn. Base: unavailable image Provider displays actual failure without clearing
an existing accepted reference. Bad: worker completed is presented as image
success, or empty recovery creates a new mutation.

### 6. Tests Required
`ux-report-shell-chat.test.mjs` covers footer whitespace, repair and custom-error
ownership, valid route metadata and scoped navigation. `ux-settings-support.test.mjs`
covers failure precedence, Beijing parsing, initial/later read failure, bounded
download and original-only recovery. `ux-report-creation.test.mjs` validates real
framework field types and constraints. World/provider tests cover global creation
without a runtime fid and unchanged wire request identity. Browser checks record
actual 850px reflow and narrow-drawer to desktop inert cleanup; viewport reflow is
not evidence of browser 250%/300% zoom or real Provider success.

### 7. Wrong vs Correct
Wrong: `if (worker.status==='completed') showSuccess(); retry(newId);`.
Correct: inspect the original business result/artifact, display its readable
failure or unknown state, and query/retry only the captured original identity.

## Scenario: New-user continuation and visible request progress (2026-10-22)

### 1. Scope / Trigger

Home/creation continuation, chat observer recovery, suggestions, wardrobe and diary.

### 2. Signatures

`requestReceipt(fid,envelope,onTerminal,{container,alive,onRetry})`;
`refreshActiveTurn(explicit=false)`; `scheduleItemPending/retryScheduleItem`;
Owner-scoped `creationDraft` and persistent schedule-item command channels.

### 3. Contracts

A suggestion may render progress inline without opening a drawer. Observe the
same request ID; preserve successful reads on errors, pause while hidden, stop
on disposal/Owner change, and explain bounded observation pauses. Translate task
names/statuses; keep raw data in collapsed source details. Retry is an explicit
caller action and requires a newly read failed request with mutation_committed
false. A declined or deferred request is not a technical failure.

Chat automatic polling remains disabled during a send. Explicit progress reads
preserve incremental text while the server says pending/running. Only the same
scope/epoch and a confirmed terminal active turn may abort local observation,
remove its transient messages, merge saved records and restore the original
retry identity. A timer alone cannot mark a turn failed or create a new turn.

The library exposes saved creation drafts and never hides a server character
because its group has not loaded. Auto-select a sole published world only on the
complete first result page. Outfit choice reads the current inventory/outfit
versions before opening confirmation; optional user text has a meaningful audit
default. Explicitly adopted governance baselines survive drawer remount.

Schedule-item unknown outcomes retain the frozen payload, revisions and
idempotency key across provider recreation. Proposed cards remain distinct from
accepted schedule items. Beijing editors retain the existing precision contract.
Diary generation explains ended-day restrictions before opening the form;
pending entries observe without discarding saved notes or successful content.

### 4. Validation & Error Matrix

Hidden/disposed/changed Owner: no late repaint. Failed read: retain the previous
status. Changed stream epoch: ignore late data. Unknown submission: replay the
original body before accepting another proposal. Missing inventory registration
must not fabricate clothing or completed wear.

### 5. Good / Base / Bad Cases

Good: return to an initialization card and resume its saved creation identity.
Base: wait, leave and later read a suggestion's actual result. Bad: automatically
resubmit a pending request, overwrite streaming text with an ordinary poll, or
count a submitted suggestion as an accepted schedule.

### 6. Tests Required

`ux-recovery-flows.test.mjs` exercises actual inline progress, read failure,
Owner isolation, draft continuation and fresh outfit versions.
`conversation-scope-recovery.test.mjs` checks pending vs terminal observer reads.
`v3-life-provider.test.mjs` checks transport loss → provider recreation → identical
request body/key and original update/cancel target through conflict recovery.
A slow previous-receipt read cannot replace a newer suggestion observer. Keep
previous explicit rebase tests passing.

### 7. Wrong vs Correct

Wrong: show only an internal accepted code and require users to locate a request
ID. Correct: show progress/results in context and expose concrete recovery actions.
