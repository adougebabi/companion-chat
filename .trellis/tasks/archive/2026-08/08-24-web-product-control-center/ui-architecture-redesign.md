# Web UI Architecture Reset

## Decision

The active `apps/web` client uses one workspace shell with four primary
surfaces: Instances, Moments, Settings, and Diagnostics. Chat is a focused
surface opened from the instance/contact list; Diagnostics keeps its redacted
correlation query and workflow controls but is promoted to a peer tab beside
Settings. A selected Fluctlight provides context for a read-only detail dialog
and a separate full governance work surface.

```text
AppShell
├── AuthPanel
└── Workspace
    ├── InstancesView
    │   ├── InstanceDirectory
    │   ├── InstanceCreateDialog
    │   └── GovernanceView
    ├── ChatView
    ├── MomentsView
    ├── SettingsView
    │   └── Settings section pages
    │       ├── Model role binding
    │       ├── Provider endpoints
    │       ├── Active role bindings
    │       ├── ComfyUI
    │       ├── Autonomy / retention
    │       └── Owner session
    └── DiagnosticsView
        └── Advanced workflow controls
```

## Boundaries

- `App.vue` coordinates authentication, active view, navigation, and the
  read-only detail dialog only.
- Feature views own page-local form state and emit navigation intent; they do
  not create browser clients or perform raw HTTP requests.
- Shared controls are local shadcn-vue components under
  `apps/web/src/components/ui`, generated from `components.json`; feature
  views compose these primitives and keep product-specific chat/list surfaces
  in the existing layout stylesheet.
- Pinia remains the owner of server snapshots and deep asynchronous behavior.
  Conversations SSE, optimistic messages, sequence checks, abort handling and
  persistence reconciliation remain in `conversations.ts`.
- Read-only details never render mutation controls. Identity, schedule,
  Event/Presence, memory, relationship, autonomy, revision and retirement
  actions render only in `GovernanceView`.
- Settings and Diagnostics expose their real domains in the contextual left
  panel. Selecting one updates the URL section and renders only that section
  in the right-hand page; on mobile the same list becomes the page index with a
  back link. The selected settings form still uses progressive disclosure
  inside its section where appropriate.
- The bottom navigation is mounted in the workspace shell; desktop keeps it as
  a textual vertical rail at the lower-left of the contextual panel while
  mobile uses a full-width safe-area-aware bar. Each page has one primary
  scroll owner. Dialogs use a bounded grid with only the body scrolling.

## Route Semantics

The UI defines route semantics without requiring a router dependency yet:

```text
/chat/:fluctlightId
/moments?scope=global|fluctlight&id=:id
/instances
/instances/new
/instances/:id/govern
/settings
/settings?section=model-role|endpoint|binding|media|operations|owner
/settings/diagnostics?section=model-runs|events|workflows&correlation_id=:id
```

The current implementation keeps typed in-memory view state alongside the
section query to avoid a router dependency while preserving browser history
and refresh semantics. A later router adapter may map this route contract
without changing feature interfaces or Pinia state ownership.

## Responsive Contract

- Desktop uses a centered glass workspace; mobile switches to a full-width
  `100dvh` surface.
- Mobile navigation uses a 48px touch target and safe-area-aware bottom
  placement.
- Forms and governance facts collapse to one column below 760px.
- Long identifiers, JSON, media and field-source values wrap inside their
  owned surface; page-level horizontal overflow is hidden.
- Chat keeps a single message timeline scroll owner and a persistent composer.

## Verification

The architecture reset must preserve generated Browser Client ownership,
secure write-only settings, safe Vue text bindings, and the existing
Conversations Store stream contract. Required local checks are:

```text
pnpm --filter @fluctlight/web test
pnpm --filter @fluctlight/web typecheck
pnpm --filter @fluctlight/web build
```

## Telegram / PC Skin Decision

The product keeps four primary tabs—Instances, Moments, Settings, and
Diagnostics. Chat is a focused surface opened from an instance and hides the
primary tab bar. On desktop, the authenticated chat shell uses a contextual
conversation panel and message pane inspired by the supplied messaging
reference; its list shows Recent plus flat group tabs, without an Archive tab.
On mobile, the contextual panel collapses into one inset rounded work card and
the four tabs become a full-width safe-area-aware bottom bar; the chat list
keeps the same Recent/group rhythm.

Chat height is owned by the viewport: the shell and chat page use a definite
`100dvh`-based height, the message timeline is the only scrolling region, and
the composer remains the final visible row. Entering a conversation schedules a
post-layout scroll to the latest message after the initial store load.

Diagnostics is reachable directly from the fourth primary tab and preserves a
`correlation_id` query parameter while section navigation uses `section`. The
left list routes to Model Runs, Events, or Workflow Controls, and the right
surface shows only the selected domain. Refreshes are request-ordered, retain
the last successful snapshot when one source fails, and distinguish runtime
unavailability from owner authorization failures.
