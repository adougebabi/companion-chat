# Frontend Quality Guidelines

## Required Checks

- Run `npm run typecheck`, `npm run build`, `node --check server/index.js`, and
  load the generated `dist/` through Express, not from `file://`.
- Exercise companion SSE and confirm token, done, and error events render
  without uncaught exceptions.
- Verify API fields are normalized in `web/src/api/contracts.ts` and user text
  is rendered through Vue bindings.
- Check desktop and narrow/mobile layouts, including overlays and dialogs.
- Confirm refresh recovery does not lose the active persona, draft, IME state,
  or persisted conversation.

## Presentation Naming Boundary

- The active client uses `摇光（Fluctlight）` for the product, `摇光实例` when a concrete AI type label is needed, and the created instance's own escaped name in ordinary UI copy.
- User-facing identity language uses `身份核心`; copy describes continuity, life context, memory, relationships, and bounded behavior as product goals, never as proof of subjective consciousness.
- A presentation rename must not rename compatibility contracts: `/api/companion`, `companion_*` tables or `companion.sqlite`, `COMPANION_*` environment variables, Docker/volume identifiers, localStorage keys, static filenames, test hooks, or payload fields.
- Keep deleted root `src/` assets out of production; active changes belong under
  `web/` and are verified through the Vite build.

## Forbidden Patterns

- Direct HTML interpolation of unescaped user/provider content.
- New polling loops that run while the page is hidden or while a send is active.
- Browser-side provider calls that would expose MTPLX keys or ComfyUI configuration.
- Reintroducing the deleted vanilla `src/` entry as a production fallback.

## Verification Notes

There is no frontend test script, bundler, or type checker. Manual browser checks plus syntax validation are the project’s current quality gate. Use a temporary or empty data directory for destructive UI checks when needed.
