# Technical Design

## Frontend Closure

Keep the current Vue 3 + TypeScript + Vite + Pinia stack and `web/ -> dist/` delivery. Fix the client in place:

- initialize app boot state as loading before Vue replaces the static shell;
- bind the actual top sentinel to `useMessageHistory` and retain scroll fallback;
- store drafts per 摇光实例 and never clear on navigation unless explicitly discarded or successfully submitted for that instance;
- add hidden-activity state/view/restore flow;
- restore complete settings form (model, provider, ComfyUI, h3) with typed DTOs and errors;
- add inspector actions for preflight, simulation and debug media;
- lazy-load settings/persona/inspector components with dynamic imports;
- add compression at the HTTP boundary and retain hashed immutable assets.

## Backend Closure

Do not introduce another architecture. Extend the existing modular runtime:

- register life, timeline, relationship, pending, media and proactive flows in the same flow registry or wrap them with a single generic flow/effect adapter;
- make application flows return plans/effect intents and let the common commit/dispatcher own enqueue, retry and settlement;
- preserve current repository transactions and idempotency while removing direct feature-specific enqueue from flow bodies;
- upgrade context fragments to structured metadata and make the budgeter reserve required sections before optional fragments; keep prompt-selection policy in the existing prompt-optimization owner;
- preserve native/marker capability handoff and current public SSE/API contracts.

## Verification

Use temporary data and mocked providers for deterministic tests. Start a real Express runtime serving `dist/`, call `/api/health`, `/api/companion/bootstrap`, and a cursor conversation page, then exercise the browser at desktop and mobile widths. Use the configured MTPLX only for the existing explicit capability probe, not as a CI dependency.
