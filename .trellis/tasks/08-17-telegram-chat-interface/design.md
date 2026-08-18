# Technical Design: Telegram-style Chat Interface

## Scope

Build a bright, mobile-first, Telegram-inspired interface around the new companion APIs. The design borrows compact conversation hierarchy and focused chat ergonomics from the supplied references without copying Telegram branding, assets, or protocol behavior. It does not implement dark mode.

## Information Architecture

| Destination | Desktop | Mobile | Purpose |
| --- | --- | --- | --- |
| Chats | Left persona/chat list plus main chat pane | Full-screen active chat; menu opens list | Primary direct conversation |
| Activity | Main-pane global chronological timeline | Full-screen timeline from primary navigation | All-persona Moments discovery |
| Persona detail | Contextual detail entry from avatar/list | Detail page/sheet | Current situation, limited availability, individual Moments, memory governance |
| Lifecycle inspector | Explicit developer-only entry | Explicit developer-only entry | State/event/job trace and simulation controls |

The desktop workspace stays dense rather than card-heavy: fixed left rail, optional narrow secondary context column only where needed, and a single dominant main content region. Mobile has one active surface at a time; its menu and primary navigation never cover the chat header or composer.

## UI State and Data Refresh

Extend the existing module-level client state with an explicit active destination, feed cursors, activity red-dot watermark, chat unread counts, pending media/job state, screen state, and debug mode. Keep server state canonical. Reuse the existing visible-page polling model for feed/job refresh and preserve chat SSE semantics.

All server/user text uses the existing escaping/sanitization path. New API fields are traced in server and client before rendering. No raw prompt/debug data appears outside the explicit inspector.

## Chat and List

Each list row presents avatar, name, concise last-message/activity summary, exact direct-message unread badge, and optional current-event summary. Current status is a compact rendering of the authoritative event, not an invented online/typing indicator or full schedule. Screened personas move out of the normal list/timeline and can only be restored from an explicit screened area.

The composer uses individual accessible icon controls, auto-growing text input within a stable maximum height, attachment truncation, clear sending/disabled behavior, and keyboard-safe layout. Destructive console actions require confirmation, pending state, and error feedback.

## Activity UI

- Global feed is reverse chronological and cursor-paged. It has a small red dot only, no unread count.
- Persona detail renders that persona's Moments feed using the same post component and scoped APIs.
- A post always identifies its author persona. Supporting-character mentions/reactions are display-only and never link to profiles or chats.
- User likes and threaded comments are scoped to the author persona. Comments do not duplicate into direct chat.
- Persona post content/media/reactions are immutable. Users may hide/restore posts but cannot edit or delete them.
- First-release media is one optional image. Its card reserves dimensions and renders a skeleton/loading state until the original item updates in place. The component contract supports later image sets or one video mode.

## Responsive and Accessibility Rules

- Use dynamic viewport units with safe-area insets; do not lock mobile composition to `100vh`.
- At small widths, reserve header space for fixed navigation controls and ensure all labels/touch targets remain reachable.
- Apply `min-width: 0`, text ellipsis/wrapping, and constrained media geometry to prevent long text/attachments from horizontal overflow.
- Use buttons for icon actions with accessible labels/tooltips. Emoji choices are individually focusable controls.
- Verify keyboard navigation, focus restoration from drawers/dialogs, readable status contrast, and media alternative text/loading labels.

## CSS Strategy

First consolidate the existing duplicated compressed CSS tail into one authoritative stylesheet. Define local tokens for bright surfaces, borders, text, accent, destructive state, unread badge, skeleton, and spacing. Avoid broad visual rewrites that obscure functional regressions. Use predictable grid/flex constraints rather than viewport-scaled typography.

## Verification

Visual/manual coverage at 320px, 375px, 650px, tablet, and desktop includes navigation, header/menu spacing, chat send/SSE, dynamic viewport keyboard behavior, list unread signals, global/persona feed, comments/likes, hide/restore, media placeholders, settings/memory dialogs, and the developer inspector. Check browser console errors and refresh persistence.
