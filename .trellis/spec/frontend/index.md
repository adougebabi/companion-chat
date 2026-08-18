# Frontend Guidelines

The browser client is a dependency-light vanilla ES module: [`src/index.html`](../../../src/index.html) loads [`src/main.js`](../../../src/main.js), which renders the entire UI and uses [`src/style.css`](../../../src/style.css) for presentation. There is no React, component framework, router, hook system, TypeScript, or build step.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding UI features or assets |
| [Component Guidelines](./component-guidelines.md) | DOM rendering, events, and accessibility |
| [State Management](./state-management.md) | Local/server state and refresh behavior |
| [Quality Guidelines](./quality-guidelines.md) | Browser verification and safe DOM updates |

## Pre-Development Checklist

- Find the existing render function and the event binding for the UI you are changing.
- Trace the API payload in both `server.js` and `main.js` before renaming a field.
- Escape server/user text with `esc()` before interpolating it into HTML.
- Decide whether the update belongs in canonical `state`, current `messages`, or transient UI variables.

## Quality Check

Run the server and open `http://localhost:4178`. Exercise persona switching, sending text, settings, memory, console, attachments, and mobile menu as relevant. Check browser console errors and verify that a refresh restores the same server-backed state.
