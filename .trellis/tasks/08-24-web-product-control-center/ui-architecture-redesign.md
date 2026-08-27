# Web UI Architecture Reset

## Decision

The active `apps/web` client uses one workspace shell with three primary
surfaces: Instances, Moments, and Settings. Chat is a focused surface opened
from the instance/contact list; Diagnostics is a secondary Settings surface. A
selected Fluctlight provides context for a read-only detail dialog and a
separate full governance work surface.

```text
AppShell
├── AuthPanel
└── Workspace
    ├── InstancesView
    │   ├── InstanceDirectory
    │   ├── InstanceCreateSurface
    │   └── GovernanceView
    ├── ChatView
    ├── MomentsView
    ├── SettingsView
    └── DiagnosticsView
        └── Advanced workflow controls
```

## Boundaries

- `App.vue` coordinates authentication, active view, navigation, and the
  read-only detail dialog only.
- Feature views own page-local form state and emit navigation intent; they do
  not create browser clients or perform raw HTTP requests.
- Pinia remains the owner of server snapshots and deep asynchronous behavior.
  Conversations SSE, optimistic messages, sequence checks, abort handling and
  persistence reconciliation remain in `conversations.ts`.
- Read-only details never render mutation controls. Identity, schedule,
  Event/Presence, memory, relationship, autonomy, revision and retirement
  actions render only in `GovernanceView`.
- The bottom navigation is mounted by `AppShell` and reserves safe-area space;
  each page has one primary scroll owner. Dialogs use a bounded grid with only
  the body scrolling.

## Route Semantics

The UI defines route semantics without requiring a router dependency yet:

```text
/chat/:fluctlightId
/moments?scope=global|fluctlight&id=:id
/instances
/instances/new
/instances/:id/govern
/settings
/settings/diagnostics?correlation_id=:id
```

The current implementation keeps typed in-memory view state to avoid changing
browser history and refresh semantics while components are being extracted.
A later router adapter may map this route contract without changing feature
interfaces or Pinia state ownership.

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
