# Frontend Directory And Structure

```text
src/index.html              Minimal document shell and active module entry point
src/companion-main.js       Active state, renderers, event binding, and boot
src/companion-style.css     Active layout, responsive rules, dialogs, messages, panels
src/main.js                 Legacy, currently unreferenced UI module
src/style.css               Legacy, currently unreferenced UI stylesheet
```

Keep active browser behavior in `companion-main.js` unless a new static asset or
stylesheet rule is needed. New features should extend the nearest existing
function rather than create a parallel boot path.

There is no asset pipeline: files are served directly by Express from `src/`. Keep imports browser-compatible and use the vendored `/vendor/marked/marked.esm.js` path for Markdown parsing.

Use kebab-case DOM IDs/classes (`memory-panel`, `clear-memory`) and camelCase
JavaScript functions/variables. Keep responsive behavior in the existing
media-query sections of `companion-style.css`.
