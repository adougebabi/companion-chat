# Implementation Plan: Fluctlight Naming

This parent task changes terminology and product-facing names only. It does not start the deferred architecture/performance/tool-call tasks and does not implement the six self-awareness behaviors.

## Ordered Work

1. **Terminology review (completed)**
   - User approved `摇光（Fluctlight）` as the product display name.
   - User approved `摇光实例` for a concrete AI individual, with the instance's own name shown in normal UI copy.
   - User approved keeping `persona` / `companion` only as migration-compatibility identifiers and limiting this task to display/docs changes.

2. **Glossary finalization (completed before activation)**
   - Confirm `CONTEXT.md` contains only the resolved domain terms and behavioral meaning.
   - Remove resolved open decisions from this task's PRD and design.
   - Keep self-awareness language explicitly aspirational/behavioral, not a claim of proven subjective consciousness.

3. **Active product rename (completed)**
   - Update `src/index.html`, `src/companion-main.js`, and any active style/accessible labels that expose the old product name.
   - Update `README.md` and product-facing documentation.
   - Keep legacy technical identifiers untouched unless a separate migration decision is approved.

4. **Compatibility verification (completed)**
   - Search for accidental edits to `/api/companion`, `companion_*`, `companion.sqlite`, `COMPANION_*`, localStorage keys, Docker names, and test hook fields.
   - Run the existing tests and active UI syntax checks.
   - Load the app through Express and verify title, brand labels, empty state, persona name, and accessibility labels.

## Validation Commands

```bash
node --check src/companion-main.js
npm test
rg -n "Fluctlight|Companion|知觉" src/index.html src/companion-main.js README.md
rg -n "/api/companion|companion_|companion\.sqlite|COMPANION_|companion-active-" server.js src test README.md compose.yaml .env.example
```

Use a temporary data directory for browser checks if the service must be started. Do not run or change the MTPLX demo as part of this rename implementation; its result is already recorded as research.

## Deferred Work

- `08-20-backend-modular-boundaries` remains an unlinked planning task until the user chooses an architecture direction.
- `08-20-frontend-runtime-performance` remains an unlinked planning task until the user chooses the frontend state/rendering direction.
- `08-20-native-tool-call-migration` remains an unlinked planning task; the MTPLX proof does not authorize implementation here.

## Rollback

Revert presentation strings and documentation only. Do not reset or restore unrelated uncommitted backend/media changes in the workspace.

## Review Gate Before Activation

- User approved the terminology mapping and display/docs-only rename surface.
- `CONTEXT.md`, `prd.md`, `design.md`, and `implement.md` contain no unresolved contradictory terms.
- Run `python3 ./.trellis/scripts/task.py validate fluctlight-architecture-performance-modernization` before `task.py start`.
