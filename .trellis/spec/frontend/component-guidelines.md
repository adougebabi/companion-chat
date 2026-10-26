# React Components And Interaction

## Ownership

`main.tsx` mounts one React root. `react-app/App.tsx` owns navigation and the
workspace lifecycle; pages own editing and effects; views render named models.
`components/ui/*.tsx` are the local shadcn Radix primitives. Domain data and
commands belong to `state/`, existing providers and the generated BrowserClient.
Do not reintroduce native HTML string page renderers or Vue compatibility layers.

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

## Verification

Use React DOM interaction tests for actual controls and page flows; source
regex or a successful build is not enough. Cover precise Select values, input
preservation, late responses, focused DOM after a step change, dialog closing,
media native elements and filtering, and removed-route rejection.
