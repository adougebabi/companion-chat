# Frontend Directory And Structure

```text
src/index.html   Minimal document shell and module entry point
src/main.js      State, API helper, renderers, event binding, streaming, boot
src/style.css    Global layout, responsive rules, dialogs, messages, panels
```

Keep browser behavior in `main.js` unless a new static asset or stylesheet rule is needed. The file is organized around a small state model, HTML render helpers (`avatar`, `messageHtml`, `renderMessages`, `render`), then event handlers and async workflows. New features should extend the nearest existing function rather than create a parallel boot path.

There is no asset pipeline: files are served directly by Express from `src/`. Keep imports browser-compatible and use the vendored `/vendor/marked/marked.esm.js` path for Markdown parsing.

Use kebab-case DOM IDs/classes (`memory-panel`, `clear-memory`) and camelCase JavaScript functions/variables. Keep responsive behavior in the existing media-query sections of `style.css`.
