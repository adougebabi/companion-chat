# Vue Components And Interaction

## Rendering Pattern

`web/index.html` creates the static shell once. Vue views and components update
reactive props from Pinia stores; composables own asynchronous side effects.
Avoid whole-app rerenders or direct DOM replacement during chat, activity, or
composer updates.

## Safe HTML

All external or user-controlled text must be rendered through Vue text nodes.
Only explicitly trusted, sanitized HTML may use `v-html` (none is needed for
the current chat/activity flows).

## Events And Accessibility

Declare component emits and props explicitly. Prefer existing buttons, labels,
dialogs, and `aria-label`/`title` patterns. Keyboard send behavior is Enter,
with Shift+Enter reserved for a newline. Modal actions must preserve a cancel
path and return focus to the trigger.

## Styling

Use existing semantic classes and CSS custom properties where present. Keep
layout and responsive rules in `web/src/styles/*.css`; do not add inline styles
unless a value is genuinely data-driven.

## Common Mistakes

- Replacing the entire app tree on every refresh and losing input state.
- Rendering server refresh over an active SSE stream without reconciliation.
- Treating a server refresh as authoritative while an active SSE stream is still rendering.
- Adding framework-style components or hooks that the project does not have.
