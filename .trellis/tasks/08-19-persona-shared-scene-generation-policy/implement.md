# Implementation Plan

## Ordered Work

1. **Migration and domain helpers**
   - Add the next companion migration for `image_generation_policy` and `shared_scene_json` with defaults and a one-time normalization of unexpected legacy policy values to `autonomous`.
   - Add policy normalization/labels at the server boundary.
   - Add shared-scene projection/read helpers and `applySceneEvent()` with a short transaction, event history, causation ID, and `start`/`switch`/`end` behavior.
   - Update `resolvedStateFor()`, `stateShape()`, `contextFor()`, media envelope construction, and persona deletion/read paths.

2. **Chat tool contract**
   - Add the `scene_event` tool schema and system prompt instructions to the application-owned capability layer.
   - Add the five policy meanings to the final context without exposing internal implementation details to the user.
   - Extend the OpenAI-compatible completion request and streaming decoder to accumulate tool-call fragments while preserving text-token behavior.
   - Execute one validated scene call per turn; support a standard tool-result continuation only when the provider did not return final visible text.
   - Preserve existing media/pending-event marker redaction, message ordering, and `token`/`done`/`error` SSE names.

3. **Persona detail setting**
   - Return the policy only from the persona detail endpoint.
   - Add a dedicated update route with server-side enum validation.
   - Add a plain detail-page `<select>` with the five Chinese labels, defaulting to the server's `autonomous` value for new personas.
   - Do not add policy text to bootstrap summaries, contact rows, current-scene panels, or quick-reply controls.

4. **Rendering and refresh behavior**
   - Keep parenthesized user/persona text unchanged and escaped through the existing renderer.
   - Ensure a tool-created scene is visible to the next chat context and detail refresh without adding a synthetic chat message.
   - Confirm images remain attached to the triggering message and keep their existing queued/success/failure presentation.

5. **Tests and manual verification**
   - Add backend regression tests for migration/defaults, policy updates, all scene-event operations, invalid tool calls, current-state precedence, and tool continuation/fallback.
   - Add frontend/source assertions for the detail select, absent scene-panel/quick-reply UI, and unchanged parenthesized text rendering.
   - Run `node --check server.js`, the existing test command, and a temporary-DB integration pass through the chat route with a mocked streamed tool call.
   - Start the local service, verify `GET /api/health`, then manually send one scene-changing turn and one ordinary parenthesized-action turn.

## Validation Commands

```bash
node --check server.js
npm test
DATA_DIR=$(mktemp -d) DATABASE_PATH="$DATA_DIR/companion.sqlite" node --test test/companion-api.test.mjs
```

For the manual pass, use a temporary database and configured local model; do not commit the generated `data/` directory.

## Risk And Rollback Points

- **Migration risk:** stop before changing chat code if a fresh temporary database does not initialize with the new columns and defaults.
- **Streaming risk:** keep the existing text-only path green before enabling tool definitions; a provider that ignores tools must still produce ordinary chat.
- **State precedence risk:** verify shared-scene projection wins only while `shared_scene_json` is present and that `end` returns to the existing schedule/routine projection.
- **Media regression risk:** validate that adding tools does not alter existing `<media-intent>` parsing or durable-job creation.
- **Rollback:** revert the application code and deploy the previous binary; the additive columns and rows remain harmless for the old code. Do not delete database data during rollback.

## Review Gate Before Activation

- User reviews `prd.md`, `design.md`, and `implement.md`.
- `python3 ./.trellis/scripts/task.py validate persona-shared-scene-generation-policy` passes.
- Task is started only after the planning artifacts are approved; implementation begins in a separate phase.
