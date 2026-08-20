# Frontend Quality Guidelines

## Required Checks

- Run `node --check src/main.js` and load the app through the Express server, not from `file://`.
- Exercise `/api/chat` streaming and confirm token, done, and error events render without uncaught exceptions.
- Verify all new interpolated text uses `esc()` and all changed API fields match the server response.
- Check desktop and narrow/mobile layouts, including the mobile overlay and dialogs.
- Confirm refresh/focus recovery does not lose the active persona or persisted conversation.

## Presentation Naming Boundary

- The active client uses `摇光（Fluctlight）` for the product, `摇光实例` when a concrete AI type label is needed, and the created instance's own escaped name in ordinary UI copy.
- User-facing identity language uses `身份核心`; copy describes continuity, life context, memory, relationships, and bounded behavior as product goals, never as proof of subjective consciousness.
- A presentation rename must not rename compatibility contracts: `/api/companion`, `companion_*` tables or `companion.sqlite`, `COMPANION_*` environment variables, Docker/volume identifiers, localStorage keys, static filenames, test hooks, or payload fields.
- Keep `src/main.js` and `src/style.css` untouched when `src/index.html` and `src/companion-main.js` remain the active entry.

## Forbidden Patterns

- Direct `innerHTML` interpolation of unescaped user/provider content.
- New polling loops that run while the page is hidden or while a send is active.
- Browser-side provider calls that would expose MTPLX keys or ComfyUI configuration.
- Introducing a frontend framework/build dependency for a local change to this vanilla client.

## Verification Notes

There is no frontend test script, bundler, or type checker. Manual browser checks plus syntax validation are the project’s current quality gate. Use a temporary or empty data directory for destructive UI checks when needed.
