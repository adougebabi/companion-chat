# Backend Guidelines

The backend is a single ES module, [`server.js`](../../../server.js), run by Node 22. Express routes, state transitions, provider calls, and background workers live in that file. Keep additions consistent with this deliberately small deployment model unless a change clearly requires a new module.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding routes, helpers, or runtime assets |
| [Database Guidelines](./database-guidelines.md) | Reading or changing persisted state |
| [Error Handling](./error-handling.md) | HTTP, provider, SSE, and worker failures |
| [Development Debug Observability](./debug-observability.md) | Explicitly gated, persona-scoped prompt/job diagnostics |
| [Media Prompt Contract](./media-prompt-contract.md) | Typed image/video intent, direct chat requests, and prompt authority |
| [Quality Guidelines](./quality-guidelines.md) | Safe changes and verification |
| [Logging Guidelines](./logging-guidelines.md) | Operational and debug output |

## Pre-Development Checklist

- Identify whether the change affects the state shape, an API contract, a streaming event, or a background worker.
- Read the corresponding route and its frontend consumer before changing a payload.
- Preserve the data directory and environment-variable defaults described in [`README.md`](../../../README.md) and [`.env.example`](../../../.env.example).
- Check both the normal response and the failure path; this service has no central Express error middleware.

## Quality Check

Run `npm start` (or `npm run dev` during development), then check `GET /api/health`. For endpoint changes, exercise the route with a real JSON request and inspect the resulting SQLite-backed state. There is currently no automated test suite or lint script, so syntax/runtime checks are required.
