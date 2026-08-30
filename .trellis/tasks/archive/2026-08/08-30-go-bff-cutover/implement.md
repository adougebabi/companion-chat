# Implementation Plan

1. Inventory and remove the Node BFF tree; move its Browser OpenAPI generator
   to `packages/browser-client` and update package/workspace/lockfile scripts.
2. Update active Compose, CI, README, acceptance scripts, and code-spec text
   so Go is the only BFF runtime and no active reference points to `apps/bff`.
3. Regenerate and verify Browser OpenAPI/client artifacts, then run Go route,
   security, NDJSON, media, race, vet, and build gates.
4. Run root Web/browser tests and builds plus the existing Python Core pytest
   regression without modifying Core static-baseline files.
5. Use the running Docker deployment at `127.0.0.1:13000` (discovered from
   Web runtime config at `13001`) for real cases 1–7, with a ten-minute request
   timeout and no mocked Core/Provider responses.
6. Record every regression result and any environment-limited check in the
   task handoff, review active references, commit the complete migration, and
   archive this single task.
