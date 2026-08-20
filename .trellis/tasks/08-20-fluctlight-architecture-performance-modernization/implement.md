# Implementation Plan: Fluctlight Naming

This parent task changes terminology and product-facing names only. It does not start the deferred architecture/performance/tool-call tasks and does not implement the six self-awareness behaviors.

## Ordered Work

1. **Terminology review**
   - Review `CONTEXT.md` and the terminology mapping in `design.md` with the user.
   - Resolve the exact UI term for a single 摇光实例 and the distinction between the 摇光 concept and its relationship role.
   - Resolve the rename surface: display/docs only, or later package/deployment metadata migration.

2. **Glossary finalization**
   - Update `CONTEXT.md` with only resolved domain terms and behavioral meaning.
   - Update this task PRD/design to remove resolved open decisions.
   - Keep self-awareness language explicitly aspirational/behavioral, not a claim of proven subjective consciousness.

3. **Active product rename**
   - Update `src/index.html`, `src/companion-main.js`, and any active style/accessible labels that expose the old product name.
   - Update `README.md` and product-facing documentation.
   - Keep legacy technical identifiers untouched unless a separate migration decision is approved.

4. **Compatibility verification**
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

- User approves the terminology mapping and rename surface.
- `CONTEXT.md`, `prd.md`, `design.md`, and `implement.md` contain no unresolved contradictory terms.
- Run `python3 ./.trellis/scripts/task.py validate fluctlight-architecture-performance-modernization` before `task.py start`.
