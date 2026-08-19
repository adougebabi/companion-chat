# Frontend Guidelines

The active browser client is a dependency-light vanilla ES module:
[`src/index.html`](../../../src/index.html) loads
[`src/companion-main.js`](../../../src/companion-main.js) and
[`src/companion-style.css`](../../../src/companion-style.css). There is no
React, component framework, router, hook system, TypeScript, or build step.
`src/main.js` and `src/style.css` are legacy, unreferenced UI files; do not
change them for active-companion work unless the entry point changes.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding UI features or assets |
| [Component Guidelines](./component-guidelines.md) | DOM rendering, events, and accessibility |
| [State Management](./state-management.md) | Local/server state and refresh behavior |
| [Quality Guidelines](./quality-guidelines.md) | Browser verification and safe DOM updates |

## Pre-Development Checklist

- Find the existing render function and the event binding in
  `companion-main.js` for the UI you are changing.
- Trace the API payload in both `server.js` and `companion-main.js` before
  renaming a field.
- Escape server/user text with `esc()` before interpolating it into HTML.
- Decide whether the update belongs in canonical `state`, current `messages`, or transient UI variables.

## Quality Check

Run the server and open `http://localhost:4178`. Exercise persona switching, sending text, settings, memory, console, attachments, and mobile menu as relevant. Check browser console errors and verify that a refresh restores the same server-backed state.
