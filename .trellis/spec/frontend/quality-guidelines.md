# Frontend Quality Guidelines

## Required Checks

- Run `node --check src/main.js` and load the app through the Express server, not from `file://`.
- Exercise `/api/chat` streaming and confirm token, done, and error events render without uncaught exceptions.
- Verify all new interpolated text uses `esc()` and all changed API fields match the server response.
- Check desktop and narrow/mobile layouts, including the mobile overlay and dialogs.
- Confirm refresh/focus recovery does not lose the active persona or persisted conversation.

## Forbidden Patterns

- Direct `innerHTML` interpolation of unescaped user/provider content.
- New polling loops that run while the page is hidden or while a send is active.
- Browser-side provider calls that would expose MTPLX keys or ComfyUI configuration.
- Introducing a frontend framework/build dependency for a local change to this vanilla client.

## Verification Notes

There is no frontend test script, bundler, or type checker. Manual browser checks plus syntax validation are the project’s current quality gate. Use a temporary or empty data directory for destructive UI checks when needed.
