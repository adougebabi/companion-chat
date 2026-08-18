# Backend Quality Guidelines

## Required Patterns

- Validate IDs, required text, cursor shape, and JSON body types at route boundaries.
- Use table-scoped helpers for companion persistence. Do not restore the legacy whole-state JSON model.
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
