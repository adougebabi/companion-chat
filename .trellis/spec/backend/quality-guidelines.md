# Backend Quality Guidelines

## Required Patterns

- Validate IDs, required text, cursor shape, and JSON body types at route boundaries.
- Use table-scoped helpers for companion persistence. Do not restore the legacy whole-state JSON model.
- Table-scoped repositories accept an already-open database and injected side-effect functions; they own parameterized SQL for their tables and linked jobs, but never open a second database, run migrations, validate domain policy, or return browser DTOs.
- Use `cleanUrl()` for configured provider URLs so joining `/models` and `/prompt` is stable.
- Clone workflow JSON before replacing `{{prompt}}`; never mutate the configured workflow string in place.
- Keep initial media worker concurrency at one, but use a durable SQLite lease rather than an in-process flag as job ownership.

## Forbidden Patterns

- Adding a second database, external queue, or server process for companion data that belongs in the existing SQLite file.
- Making model or ComfyUI calls from browser code; the server owns credentials and provider URLs.
- Completing a job/event with an expired or different lease owner.
- Changing SSE event names or resource JSON fields without updating the active client entry in the same change.
- Logging bearer tokens or committing `data/`.

## Testing And Review

Run `npm test` and `node --check server.js`. Also start the service against a temporary `DATA_DIR`, call `/api/health`, and exercise changed routes with representative success and failure payloads. Do not test against the checked-out `data/` database.

Review the full request-to-storage-to-UI path for every API change and verify that SSE errors close streams and background workers release their guards.

## Modular Contract Slice

- Contract and flow-registry modules under `server/contracts` and `server/application` must be independently importable and side-effect free.
- A flow step returns `facts`, `projections`, `effects`, and `presentation`; it does not open SQLite, write a job, call a provider, or emit SSE directly.
- Native capability normalization is consumed through the frozen `CapabilityCall` handoff. Do not add a second stream accumulator or capability dispatcher in the backend migration.
- Keep the legacy `server.js` production entrypoint unchanged until the new composition root passes normalized replay and the final API/worker/browser deletion gate.
