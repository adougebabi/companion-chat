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
calls or request identities. Migration must not change backend or prompt policy.

Retain the approved Sage visual baseline while mapping tokens. Vue `data-v-*`
selectors need explicit conversion; do not copy fake scope hashes into React.
Global Tailwind resets and old CSS variables must not corrupt legacy pages.

The Owner explicitly requires legacy=1 pages and routes to be unavailable.
Reject them before route dispatch; no redirect or compatibility mount. Current
detail/auth/editor paths have explicit React owners. No old QA HTML or demo
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
The final report lists migrated families, intentional native semantics, intentional native semantics and reusable domain compositions with paths.

## 7. Wrong vs correct

Wrong: change `package.json`, leave the active renderer and CSS entry unchanged,
and rely on an old test asserting `Button.vue`.
Correct: update the actual entry/page/state/primitive imports, load Tailwind,
verify browser behavior and update tests/specs to the React Radix contract.

## Migration regression rules

- Dynamic media tags use capitalized React element variables. Assert real
  img/video/audio elements in previews. Grid/count/empty-state consume the
  filtered list; detail lookup may use the complete list.
- Expanded memory state is an ID or null, never a row object. Test direct links.
- Tabs use stable machine values; display labels use the actual character name.
- Every controlled modal receives onClose. Test Escape and the built-in close
  button as well as business Cancel.
- Domain regressions survive UI harness replacement. Tests for deleted old
  renderers are retired with a recorded mapping to current React behavior.
- The build gate checks the real import graph, dependency manifest, root HTML
  scripts and public HTML. Computed/glob module entry points require explicit
  review; a package installation or unused generated file cannot satisfy it.
