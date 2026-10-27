# React shadcn/ui migration contract (2026-10-26)

## 1. Scope / status

The Owner explicitly selected **React shadcn/ui + Tailwind CSS + Radix UI +
Lucide React**, accepting a migration from Vue. `shadcn-vue`, Reka UI and
Lucide Vue do not satisfy this requirement. This decision overrides older
instructions to preserve Vue/native implementations; the approved visual and
domain behavior contracts remain in force.

Status: React is the production implementation. Vue/Pinia/Reka dependencies,
SFCs and the old page/mount route branch are removed. The active task records
the implementation and actual validation evidence.

## 2. Component and state interfaces

- React shadcn components live in `apps/web/src/components/ui/*.tsx`; official
  Radix registry implementations own primitive behavior, Tailwind owns styling,
  `lucide-react` owns ordinary UI icons. Business compositions own domain intent.
- Explicitly choose the Radix base. Current shadcn defaults to Base UI, so a
  default CLI initialization does not satisfy the selected stack.
- Configure the React schema, TSX, local aliases and the actual loaded CSS
  entry in `components.json`. Lock generated dependencies; do not replace the
  existing workspace with an unrelated starter application.
- External controllers expose stable `subscribe(listener) => unsubscribe`
  and cached immutable `getSnapshot()` for React subscriptions. Deep operation
  changes must notify; unchanged state must return the same snapshot.

## 3. Contracts

An installed dependency or an unused generated component is not adoption.
Trace the default production entry through each migrated route to actual
component usage. Do not add one React root per Vue control or reproduce Vue
compiled render functions inside React.

Button/Input/Textarea may render native HTML as their official implementation.
Inspect ownership and behavior, not the presence of a button/input DOM tag.
Use Radix for compound interaction primitives; retain semantic native media,
file and date/time controls where appropriate. Dialog/Sheet focus management,
keyboard behavior and portals belong to Radix, not new handwritten handlers.

Preserve route identity, Owner/character scope, epochs, frozen requests, CAS,
retry/receipt recovery, drafts and IME. Generic UI components cannot own API
calls or request identities. UI migration must not change backend or prompt policy. Independently reviewed
remote domain changes are preserved when merging, including R10 world-copy APIs.

Retain the approved Sage visual baseline while mapping tokens. Vue `data-v-*`
selectors need explicit conversion; do not copy fake scope hashes into React.
Tailwind resets and domain CSS variables must preserve current page layouts.

The Owner explicitly requires legacy=1 pages and routes to be unavailable.
Reject them before route dispatch; no redirect or compatibility mount. Current
detail/auth/editor paths have explicit React owners. R10 unique domain tools
are temporarily mounted by React DomainPage using named route selectors, with
Owner/epoch fences and unmount cleanup. Their imperative forms and overlays are
remaining custom controls, not completed shadcn adoption; no old app or shell
may be mounted to host them. No old QA HTML or demo
injector may remain as an alternate entry.

## 4. Validation and error matrix

| Condition | Required outcome |
| --- | --- |
| Component generated but production does not import it | Migration incomplete |
| shadcn defaults to Base UI / generated Reka components | Wrong base; select Radix React |
| Stream/background refresh during typing | Keep draft, focus, selection and composition |
| Unknown command result / route change | Recover original identity; do not resubmit new body |
| Dialog closes / component unmounts | Restore focus; release portal/listeners/subscriptions |
| Changed nested operation state | Publish a new snapshot and update UI |
| Unchanged snapshot read | Return cached identity; do not trigger render loop |
| legacy=1 or unknown old page | Unavailable; no old renderer is loaded |

## 5. Good / base / bad cases

Good: a real settings page consumes shadcn Input/Switch/Button and preserves
new input while acknowledging the submitted snapshot. Base: library search
and role selection use React controls against real provider data. Bad: keep
Vue pages, install React dependencies, and report the stack migrated.

## 6. Required checks

Typecheck must include TSX; source scanners must not silently scan only `.vue`.
Preserve domain contract tests when replacing Vue-specific harnesses. Verify
the production dist and fixture exclusion, then browser keyboard/focus/form
behavior, deep links, repeated navigation, streaming input continuity, desktop
and narrow layouts. Fixture evidence does not certify live model/database work.
The final report lists migrated families, intentional native semantics, remaining custom controls and reusable domain compositions with paths.

## 7. Wrong vs correct

Wrong: change `package.json`, leave the active renderer and CSS entry unchanged,
and rely on an old test asserting `Button.vue`.
Correct: update the actual entry/page/state/primitive imports, load Tailwind,
verify browser behavior and update tests/specs to the React Radix contract.

## Migration regression rules

- Dynamic media tags use capitalized React element variables. Assert real
  img/video/audio elements in previews. Grid/count/empty-state consume the
  filtered list; detail lookup may use the complete list.
- Selected memory detail state is an ID or null, never a row object. Direct links
  open a reading Sheet; edits have a separate draft and cancellation keeps the
  original record.
- Tabs use stable machine values; display labels use the actual character name.
- Every controlled modal receives onClose. Test Escape and the built-in close
  button as well as business Cancel.
- Domain regressions survive UI harness replacement. Tests for deleted old
  renderers are retired with a recorded mapping to current React behavior.
- The build gate checks the real import graph, dependency manifest, root HTML
  scripts and public HTML. Computed/glob module entry points require explicit
  review; a package installation or unused generated file cannot satisfy it.

## R11 shared domain primitive boundary

Existing unique imperative controllers may use `DomainPrimitives` through
explicit `data-domain-ui` markers. The port accepts only named primitives,
keeps one nonvisual original form/event carrier, and renders shadcn portals in
the application root. It is not a page HTML interpreter. Do not add another
root per field, new imperative pages, or a second command implementation.

Visible controls must not duplicate carrier names in FormData. Keep exact
source listeners, checked/value, constraints, disabled/hidden and selection.
Controllers that capture focused fields use the port's `domainSource` to find
the original name/dataset and current selection; do not duplicate business
attributes onto visible controls. Replacing content releases old ports and
restores focus to the new visible control. Test actual submission, Radio groups,
checkboxes, custom Select values and redraws, not only source markers.

Dialogs/Sheets must not inherit the shell's full-screen minimum height. Keep
old layout selectors domain-scoped: `.grid` is a Tailwind utility, not a place
to install auto-fit columns. Diagnostic desktop Table and mobile Cards share
one record/action model; avoid fighting table primitive utility styles with
an incompatible grid display on table rows.

Commands retain newer edits made during BOTH submission and post-receipt reads.
Reread current draft after awaited onSaved before closing. Persist original
body/key for endpoints with idempotency support and pass that SAME key through
the provider. CAS-only endpoints never invent replay keys; unknown admission
requires explicit current-object review. Acknowledged readback recovery never
sends another mutation. Owner-scoped session records clear on logout/switch.

## R2 presentation host preservation

Pure Card/Alert/Item/Empty ports keep the ORIGINAL host visible and apply the
actual component's asChild props in place. Never use the hidden interactive
carrier path or move/clone children for a form, content shell or record list.
Retain named form controls, descendant queries, handlers, hidden state and
source node identity across body redraws. Attribute writes compare old/new
values to prevent MutationObserver loops. Empty error placeholders have no
visible frame until their owner supplies content.

UI wrappers may retain semantic composition (Tooltip, form values), but variants
and Lucide icons are explicit at call sites; no legacy class-to-variant or text
icon interpreter. Cursor pagination expresses only real previous/next cursors;
append readers remain append readers. Confirmations use one AlertDialog role,
with labelled content and cancellation that never invokes the mutation.

## R12 composed layout and primitive contracts

- Library/world cards share EntityCardGrid. Use auto-fill and bounded cards so one
  item does not stretch across the content area; never invent placeholder data.
- Sidebar adoption includes Group/Menu/MenuButton/Sub, not only the shell. Active
  styles match data-active=true explicitly. Keep scoped disables and global routes.
- Ordinary CTA buttons keep intrinsic width inside flex Cards. Disclosure, select
  and navigable list rows may intentionally fill their structural click target.
- Inline more menus use end with collision padding; the header switcher uses start.
- Surface is a Card, not a Disclosure. Rich title/open belong to actual Disclosure
  children; never serialize JSX as the HTML title. Test production consumers.
- Confirmations use explicit labelled AlertDialog Header/Footer/Action/Cancel.
  Async owners prevent default close until success; cancellation never mutates.
- Calendar/Popover and text time controls replace visible browser date/time UI.
  Numeric Input retains number semantics, constraints and explicit step buttons.

## R13 layout, domain context and append reads

- Wide business-table CSS belongs to .table-wrap, including table minimum width,
  th/td spacing and tbody hover. Never target every table under the workspace:
  embedded Calendar also owns a semantic table, while a portaled Calendar may
  happen to escape the bad selector. Check every date column against its frame.
- Domain renderers use the current imported store/service facade. DomainToolHost
  does not provide the retired ctx.store. Tests must use its actual context shape.
  The host owns navigation and accessible section naming; business content owns
  its visible heading. Explicit drawer titles use one SheetTitle.
- At <=800px chat preview is a controlled Sheet with close/Escape, focus return
  and the same App preference callback; wide layout uses an independently controlled right Sidebar.
  Composer bottom padding includes the safe-area environment value.
- Root rem is 14px. Mobile 44px hit areas use physical pixel minimums on explicit
  semantic actions. Opt-in classes must reach portaled controls. Do not apply a
  blanket min-width to all buttons and thereby expand seven Calendar columns.
- CursorControls distinguishes page replacement and append reads. Append has no
  fake previous page/index; failed reads retain original options/cursor/mode and
  existing rows. Reading errors cannot replace Studio command recovery errors.
- Badges contain labels, not taller editing buttons. Tag management uses natural
  height flex rows. Archive copy names archive while tag deletion names deletion.

## R14 composed component ownership

- The full-height left Sidebar uses `collapsible="icon"`, `variant="inset"` and
  SidebarRail; its sibling SidebarInset contains the header and `.workspace-page`
  owns page scrolling. No topbar offset or second `.shell/.layout` layout.
  Icon mode keeps life subpages in a labelled DropdownMenu and all icons have
  tooltips. Hidden group labels must leave hit testing as well as the paint
  tree; opacity-only labels can block the preceding group. Do not share its
  open state or Ctrl/Cmd+B listener with the chat preview; its nested provider
  sets `keyboardShortcut={false}`.
- Chat composes MessageScrollerProvider/Viewport/Content/Item. Every content row,
  including markers and waiting/error states, is an Item. The primitive owns
  follow/prepend behavior; do not attach a competing scroll-to-bottom effect.
  Message/Bubble own the message layout; Attachment is for actual files only.
- DetailDrawer disables the default Sheet X and exposes exactly one labelled
  close button. Header stays visible, only its body scrolls. Escape and closing
  restore the connected trigger; nested cancellation preserves reading state.
  Detail/edit remount chains pass `returnFocus: () => HTMLElement | null` so
  final dismissal restores the original memory row.
- Switch visuals and thumb displacement belong to the local Radix primitive.
  Never add `.switch::before`, `:checked` or size/translation overrides to the
  root button. Tests use the real switch and its boolean value.
- `Combobox` accepts strings or `{value:string,label:string}` options. Selection
  sends the exact value, including empty string, and search includes IDs and
  labels. Custom model IDs require explicit adoption; Escape commits nothing.
  Known short enums retain ChoiceSelect and its typed-value mapping.
- ModelDirectory is a searchable browsing popover; model-purpose drafts use a
  separate Combobox with real `onChange`. Selecting a directory row never saves
  a binding. Account actions share the management tabs' header.
- Wrapping Tabs use `group-data-horizontal/tabs:h-auto` as well as `flex-wrap`;
  the primitive's default group height otherwise wins. Long tab labels fit
  their own row. Wardrobe renders slot choices only after a tag is selected.
- Questionnaire is a form. Use it as the world's source form, never nest it in
  another form. Its choices retain the existing source callbacks; step Tabs
  display controlled progress and cannot bypass validation. Preparation Trace
  shows independently read states and only confirmed-ready stages count.

Regression assertions: `r14-regressions`, `r11-flows`, `react-flows`,
`react-review-regressions` and `r10-media-default`. Geometry is verified on the
production build separately from these DOM behavior tests.
