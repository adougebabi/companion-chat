---
{"id":"legacy-5d25ca7e3d2e577dbe8e453e","title":"component-guidelines.md","kind":"spec","status":"current","paths":["apps/web","packages/browser-client"],"source":".trellis/spec/frontend/component-guidelines.md#sha256=03fa21c313fb1f6a0e9c5d5b76369782c199ff4cba644d38bb6e1b8a259c7c8d","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# React Components And Interaction

## Ownership

`main.tsx` mounts one React root. `react-app/App.tsx` owns navigation and the
workspace lifecycle; pages own editing and effects; views render named models.
`components/ui/*.tsx` are the local shadcn Radix primitives. Domain data and
commands belong to `state/`, existing providers and the generated BrowserClient.
Do not introduce new HTML-string pages or Vue compatibility layers. The R10
merge retains unique `v3-domain` editing/history/recovery tools as a bounded
migration exception: named routes mount them through React `DomainPage`, which
owns lifecycle cleanup and validation. They cannot mount an app/router/shell.
Their custom forms, overlays and icons are tracked in the remaining-controls
report and must be migrated to React primitives incrementally.

## Composition and forms

Use Button/Input/Textarea/Label/Select/Checkbox/Switch and appropriate
Card/Badge/Avatar/Tabs/Dialog/AlertDialog/Sheet/DropdownMenu/Collapsible/Accordion.
Business wrappers may add semantic variants, precise domain value mapping or
request feedback. They must compose the actual library component, not reproduce
its appearance and implement keyboard/focus behavior again.

A Button link uses `asChild` with an anchor. Form submission explicitly uses
`type="submit"`; secondary form actions use `type="button"`. Never nest buttons.
Select values preserve empty strings, null, booleans and numbers; UI choice keys
must not enter request bodies. Checkbox array selections and Switch booleans are
separate contracts. Bind labels and errors to the actual control.

Render user/server text as JSX children. Do not use dangerouslySetInnerHTML for
untrusted text. Native audio/video, date/time/file controls and decorative domain
SVG are valid semantic exceptions; ordinary UI icons use lucide-react.

## State and effects

Prefer direct React hooks for UI state. Durable domain state exposes subscribe
and cached immutable snapshots through useSyncExternalStore. The controller hook
is a small lifecycle boundary for existing long-lived asynchronous models; it
must not interpret Vue options, templates, VNodes or directive/cache metadata.
Its DOM-dependent selection effects run after React commits. New simple controls
do not need a controller. Use named bindings such as filteredMedia/allMedia and
expandedMemoryId; never compiler abbreviations or ambiguous object-or-ID state.

Keep callback refs stable. Changing a callback ref on every render must not
write null/node into observable state and create a render loop. AbortController
and DOM nodes are untracked references. Release listeners, subscriptions and
observers on unmount; late responses remain fenced by Owner/role/epoch.

## Keyboard and overlays

Enter sends, Shift+Enter adds a newline. Check nativeEvent.isComposing as well as
isComposing/keyCode229 before acting on Enter, including comments. Updating a
stream or server snapshot cannot remount or clear the composer.

Every controlled Dialog/Sheet has a close callback, a title and a cancellation
path. Escape and the library close control must work. Radix owns portal, focus
trap and dismissal. Restore focus only to a connected trigger, otherwise use a
current main/list heading. Nested content must not duplicate the dialog role.

## Styling

Tailwind and shadcn semantic tokens load through `styles/react.css`. Approved
layout rules are scoped in `react-layout.css` inside the components layer so
primitive utilities retain ownership. No fake Vue data-v attributes. Keep
className merges in the shared cn helper and preserve responsive scroll owners.

## Button focus, icon sizing and mobile hit areas

- Outline Button links retain the shared `focus-visible:ring-[3px]` utilities.
  Unlayered theme decorations must not set `box-shadow:none` on outline Buttons:
  that wins over Tailwind's ring, including anchors rendered through `asChild`.
- Icon-only actions use `Button size="icon"` (36×36px, zero padding), including
  reload/query, menus and close controls. Remove page width/height overrides;
  scope text-action dimensions away from `[data-size="icon"]`.
- At <=700px, compact icon actions, home tag filters, RadioGroupItem and Checkbox
  use transparent `::after` hit areas of at least 44×44px. Radio/checkbox visuals
  retain the original `size-4` roots and indicator sizes. Verify actual hits
  outside the visual box with `elementFromPoint`
  and pointer clicks, not only the pseudo-element's computed size.
- Icon hit-plane positioning belongs in the components layer by `data-size`, so
  utility-level `absolute` still wins. Adding `.relative` to every icon also
  matches old select-wrapper layout rules. Calendar `[data-day]` cells keep
  their own grid dimensions and existing recorded-date pseudo markers.
- Regressions: `test/r9-control-accessibility.test.mjs` checks DOM/CSS contracts;
  `scripts/verify-r9-controls.mjs` checks the production build in Chrome, real
  Tab focus, icon geometry and transparent mobile hit areas. Screenshots stay
  under `/tmp/r9-shots`, outside Git.
- A deployment audit must identify the **loaded** hashed CSS/JS assets. An open
  SPA tab can retain a previous release even when the server runs the new commit;
  compare its CDP styles with a fresh browser and the local production build.
  R10 found `index-CmVhmy0N.css` in old tabs while fresh d6843350 served
  `index-bRyegPHY.css`. Do not attribute old-tab geometry to the new source.
- A 44px minimum does not promise a hit 10px outside a 36px icon or a 28.5px
  filter. Home filters and inbox reload reserve additional transparent space;
  preserve their visual boxes. Check actual 10px coordinates, pointer dispatch,
  clipping ancestors and the winning pseudo-element declarations in CDP.
  `scripts/verify-r10-controls.mjs` applies the same selectors/measurements to
  existing tabs, fresh deployment and local dist with isolated GET replay.
  It records evidence under `/tmp/r10-shots`; it never proxies replay writes.

## Verification

Use React DOM interaction tests for actual controls and page flows; source
regex or a successful build is not enough. Cover precise Select values, input
preservation, late responses, focused DOM after a step change, dialog closing,
media native elements and filtering, and removed-route rejection.

