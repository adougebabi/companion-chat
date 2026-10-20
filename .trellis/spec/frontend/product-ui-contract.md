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
