# Frontend Quality Guidelines

## Required Checks

- Run `pnpm --filter @fluctlight/web typecheck`, `pnpm --filter
  @fluctlight/web build`, and load the generated `apps/web/dist/` through the
  production Nginx image (or a local static server), not from `file://`.
- Exercise companion SSE and confirm token, done, and error events render
  without uncaught exceptions.
- Verify generated browser-client fields are normalized at the BFF boundary and user text
  is rendered through Vue bindings.
- Check desktop and narrow/mobile layouts, including overlays and dialogs.
- Confirm refresh recovery does not lose the active persona, draft, IME state,
  or persisted conversation.

## Presentation Naming Boundary

- The active client uses `摇光（Fluctlight）` for the product, `摇光实例` when a concrete AI type label is needed, and the created instance's own escaped name in ordinary UI copy.
- User-facing identity language uses `身份核心`; copy describes continuity, life context, memory, relationships, and bounded behavior as product goals, never as proof of subjective consciousness.
- A presentation rename must not rename compatibility contracts: `/api/companion`, `companion_*` tables or `companion.sqlite`, `COMPANION_*` environment variables, Docker/volume identifiers, localStorage keys, static filenames, test hooks, or payload fields.
- Keep deleted root `src/` assets out of production; active changes belong under
  `apps/web/` and are verified through the Vite build.

## Forbidden Patterns

- Direct HTML interpolation of unescaped user/provider content.
- Implicitly interpolating new Persona layer objects; Core Persona, Developing Self, Current State, evidence, and revision payloads use a bounded `JSON.stringify` helper and remain read-only unless an explicit action is provided.
- New polling loops that run while the page is hidden or while a send is active.
- Browser-side provider calls that would expose MTPLX keys or ComfyUI configuration.
- Reintroducing the deleted vanilla `src/` entry as a production fallback.

## Scenario: Responsive Glass Detail And Governance Surfaces

### 1. Scope / Trigger

- Trigger: a page exposes a Fluctlight's details, editing, governance, or another object-level overlay.
- The active client uses a bright glass visual system. Read-only details and editing actions must not be blended into one uncontrolled, page-height surface.

### 2. Signatures

- Read-only detail state: local `show*Details` state rendered through a Vue `Teleport` dialog with `role="dialog"`, `aria-modal="true"`, a labelled title, a close control, and backdrop dismissal.
- Edit/governance state: an explicit `进入编辑与治理` action; mutation inputs and destructive actions do not appear in the read-only dialog.
- Glass surface tokens: `--surface-glass`, `--surface-border`, `--surface-shadow`, and `--glass-blur` are the required visual owners. Do not introduce a competing local glass palette.

### 3. Contracts

- A read-only dialog has a bounded outer height (`max-height` using `100dvh`), `overflow: hidden`, and exactly one scroll owner in its body (`min-height: 0; overflow-y: auto`). Header and footer remain visible.
- On narrow screens the dialog becomes a bottom sheet within safe-area bounds. Its body remains scrollable, while the page behind it does not grow beyond the viewport.
- Editing and governance use the same glass tokens and responsive constraints as viewing, but present a distinct `EDIT & GOVERN` heading and an explicit close/return action.
- Forms, JSON textareas, lists, identifiers, and action groups must fit a single-column layout at `max-width: 640px`; content may wrap or scroll inside its owned region but must never create horizontal page overflow.

### 4. Validation & Error Matrix

| Condition | Required behavior |
| --- | --- |
| Detail content exceeds available height | Only the dialog body scrolls; header/footer stay reachable. |
| 320-640px viewport | Bottom sheet or full-width work surface fits safe areas and has no horizontal overflow. |
| User wants to modify state | They choose `进入编辑与治理`; read-only details remain mutation-free. |
| Destructive action | It appears only in the governance surface with explicit confirmation, not the detail dialog. |

### 5. Good / Base / Bad Cases

- Good: tapping a chat title opens a bounded glass detail sheet; the reader scrolls its content and closes it without losing the conversation position.
- Base: a desktop editor uses the same translucent border, blur, radius, and type hierarchy as the read-only sheet.
- Bad: a modal whose document body scrolls past the viewport, or a white full-page editor that abandons the established glass visual system.

### 6. Tests Required

- Verify the template keeps dialog labelling, a close control, and backdrop dismiss handling.
- Manually inspect 320px, 390px, tablet, and desktop viewports: no horizontal overflow; dialog/editor body scrolls independently.
- Verify no edit, pause, rollback, or delete control is rendered in the read-only dialog.

### 7. Wrong vs Correct

#### Wrong

```css
.detail-dialog { height: auto; }
```

The document grows with the dialog and the close action can move off-screen.

#### Correct

```css
.detail-dialog { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; max-height: calc(100dvh - 40px); overflow: hidden; }
.detail-dialog__body { min-height: 0; overflow-y: auto; }
```

## Verification Notes

The frontend uses Vite and TypeScript. `pnpm --filter @fluctlight/web typecheck`
and `pnpm --filter @fluctlight/web build` are required before browser checks.
Manual browser checks through the Nginx-served production image, plus API/NDJSON
smoke and narrow/mobile viewport checks, remain the user-facing quality gate.
Use a temporary or empty data directory for destructive UI checks when needed.

## Scenario: Reactive NDJSON assistant streaming

### 1. Scope / Trigger

- Trigger: a chat view appends an optimistic assistant draft and applies
  incremental `token` frames from the Core/BFF NDJSON stream.

### 2. Signatures

- `BrowserTurnEvent` keeps monotonic `sequence` and a terminal `completed` or
  `error` frame.
- The conversations Pinia store owns the transient assistant draft and replaces
  the persisted page only after a valid terminal frame.

### 3. Contracts

- Never mutate the raw object reference kept in a reactive array after pushing
  it; replace the array entry (`splice`/immutable map) so Vue observes every
  token.
- Flush `TextDecoder` at EOF, reject an incomplete stream, and remove a partial
  assistant draft on error while retaining the retry payload.
- Clear the accepted composer draft before awaiting the network request; an
  unsent draft typed while another request is active is not cleared.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Token chunks arrive after the first token | The visible assistant text grows without a page refresh. |
| UTF-8 or NDJSON line is split across reads | Buffer and decode incrementally; do not parse a partial frame. |
| Stream ends without `completed` | Treat as failed/incomplete, remove the transient assistant, keep retry available. |
| User submits non-empty text | Clear only that accepted draft immediately; keep unrelated active editing intact. |

### 5. Good / Base / Bad Cases

- Good: a Chinese sentence streamed in several byte/line fragments renders in
  full while the request is open.
- Base: a completed stream re-reads persisted messages and reconciles the
  optimistic user/assistant entries.
- Bad: assigning `assistantDraft.text = ...` on the raw object after pushing it
  into a reactive array, which freezes the UI at the first chunk.

### 6. Tests Required

- Static regression asserts the store replaces the draft array entry and
  requires a terminal frame.
- Browser/client tests cover split UTF-8, split NDJSON, terminal errors and
  retry cleanup; Web typecheck/build must pass.
- Manual desktop and mobile checks confirm one-row composer layout and no draft
  loss while a request is queued.

### 7. Wrong vs Correct

#### Wrong

```ts
assistantDraft.text = assistantText;
```

#### Correct

```ts
messages.splice(index, 1, { ...messages[index], text: assistantText });
```
