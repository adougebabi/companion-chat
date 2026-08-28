# Frontend Guidelines

The active browser client is a Vue 3 + TypeScript application under
`apps/web/`. [`apps/web/index.html`](../../../apps/web/index.html) provides the
static shell and loads [`apps/web/src/main.ts`](../../../apps/web/src/main.ts).
Vite builds it to `apps/web/dist/`, which the production image serves through
Nginx. `apps/web/scripts/serve-preview.mjs` is a local preview helper only. The
deleted root `src/` client is historical context only and must not be restored
as a compatibility entry point.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding UI features or assets |
| [Component Guidelines](./component-guidelines.md) | DOM rendering, events, and accessibility |
| [State Management](./state-management.md) | Local/server state and refresh behavior |
| [Quality Guidelines](./quality-guidelines.md) | Browser verification and safe DOM updates |

## Pre-Development Checklist

- Find the owning Vue view/component and Pinia store/composable before editing.
- Trace API payloads through the generated browser client and the BFF route
  registry before renaming a field.
- Render server/user text through Vue text bindings; do not add raw HTML
  interpolation for untrusted values.
- Decide whether the update belongs in Pinia server state or transient local
  component/composable state.

## Quality Check

Run the production build through the Nginx production image (or a local static
server) and open the configured Web URL.
Exercise persona switching, sending text, settings, memory, attachments,
activity actions, inspector and mobile menu as relevant. Check browser console
errors and verify that a refresh restores the same server-backed state.
