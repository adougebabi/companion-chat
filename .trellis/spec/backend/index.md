# Backend Guidelines

The backend is a modular Node control plane rooted at [`server/index.js`](../../../server/index.js). Express routes, typed application flows, domain rules, SQLite repositories, provider adapters, and background runtime live under `server/` with one composition root. Keep additions inside the vertical layers and horizontal capability boundaries described below.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding routes, helpers, or runtime assets |
| [Database Guidelines](./database-guidelines.md) | Reading or changing persisted state |
| [Error Handling](./error-handling.md) | HTTP, provider, SSE, and worker failures |
| [Development Debug Observability](./debug-observability.md) | Explicitly gated, persona-scoped prompt/job diagnostics |
| [Media Prompt Contract](./media-prompt-contract.md) | Typed image/video intent, direct chat requests, and prompt authority |
| [Shared Scene Contract](./shared-scene-contract.md) | Native scene-event tool, durable single-persona scene projection, and policy settings |
| [Quality Guidelines](./quality-guidelines.md) | Safe changes and verification |
| [Structured Turn Contract](./structured-turn-contract.md) | Provider JSON/tool control, affect/drives state, explicit memory, and chat commit boundaries |
| [Logging Guidelines](./logging-guidelines.md) | Operational and debug output |
| [Persona Analysis And Media Jobs](./persona-analysis-and-media-jobs.md) | MTPLX persona extraction, ready interview sessions, and deterministic media follow-up compensation |

## Pre-Development Checklist

- Identify whether the change affects the state shape, an API contract, a streaming event, or a background worker.
- Read the corresponding route and its frontend consumer before changing a payload.
- Preserve the data directory and environment-variable defaults described in [`README.md`](../../../README.md) and [`.env.example`](../../../.env.example).
- Check both the normal response and the failure path; the HTTP boundary owns bounded error mapping and SSE terminal errors.

## Quality Check

Run `npm start` (or `npm run dev` during development), then check `GET /api/health`. For endpoint changes, exercise the route with a real JSON request and inspect the resulting SQLite-backed state. Run `npm test`, `npm run typecheck`, `npm run build`, and the relevant temporary-data/browser smoke checks.
