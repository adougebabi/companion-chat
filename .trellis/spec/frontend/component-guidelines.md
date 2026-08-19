# DOM Rendering And Interaction

## Rendering Pattern

`build()` creates the static shell once. `render()` updates the active persona, header, message stream, and memory list; specialized renderers return HTML strings. Follow this pattern for repeated UI rather than appending arbitrary nodes from several event handlers.

Reference: [`src/companion-main.js`](../../../src/companion-main.js).

## Safe HTML

All external or user-controlled text must pass through `esc()`. Markdown is the one deliberate exception: `renderMarkdown()` calls `marked.parse()` and then sanitizes dangerous `script`, `iframe`, `object`, `embed`, and event-handler attributes before insertion. Keep the same boundary for new message-like content.

## Events And Accessibility

Bind events in `bind()` after `build()`. Prefer existing buttons, labels, dialogs, and `aria-label`/`title` patterns. Keyboard send behavior is Enter, with Shift+Enter reserved for a newline. When adding a modal action, close it through the native `<dialog>` API and preserve a cancel path.

## Styling

Use existing semantic classes and CSS custom properties where present. Keep
layout and responsive rules in `companion-style.css`; do not add inline styles
to generated HTML unless a value is genuinely data-driven (the persona color is
the established example).

## Common Mistakes

- Rebinding the entire document on every refresh and losing input state.
- Inserting `entry.prompt`, persona names, or attachment names without `esc()`.
- Treating a server refresh as authoritative while an active SSE stream is still rendering.
- Adding framework-style components or hooks that the project does not have.
