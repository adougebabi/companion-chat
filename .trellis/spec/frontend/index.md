# Frontend Guidelines

The active browser client is a Vue 3 + TypeScript application under `web/`.
[`web/index.html`](../../../web/index.html) provides the static shell and loads
[`web/src/main.ts`](../../../web/src/main.ts). Vite builds it to `dist/`, which
Express serves in production. The deleted root `src/` client is historical
context only and must not be restored as a compatibility entry point.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding UI features or assets |
| [Component Guidelines](./component-guidelines.md) | DOM rendering, events, and accessibility |
| [State Management](./state-management.md) | Local/server state and refresh behavior |
| [Quality Guidelines](./quality-guidelines.md) | Browser verification and safe DOM updates |

## Pre-Development Checklist

- Find the owning Vue view/component and Pinia store/composable before editing.
- Trace API payloads through `web/src/api/contracts.ts` and the server route
  registry before renaming a field.
- Render server/user text through Vue text bindings; do not add raw HTML
  interpolation for untrusted values.
- Decide whether the update belongs in Pinia server state or transient local
  component/composable state.

## Quality Check

Run the production build through Express and open `http://localhost:4178`.
Exercise persona switching, sending text, settings, memory, attachments,
activity actions, inspector and mobile menu as relevant. Check browser console
errors and verify that a refresh restores the same server-backed state.
