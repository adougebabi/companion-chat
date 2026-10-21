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
- The shared `userInstant` formatter validates zones, preserves milliseconds
  and numeric offset for diagnostics, and shows the actual chosen zone. Missing
  zone uses the browser's valid display zone; malformed zones cannot crash pages.
  Schedule header, range and suggestion context choose schedule.timezone before
  context.timezone. Compact ranges retain both dates across midnight; raw offset
  timestamps remain in folded source and unchanged in commands.
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
