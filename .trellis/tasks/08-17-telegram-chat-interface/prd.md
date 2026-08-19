# Telegram-style chat interface

## Goal

Replace the current visual treatment with a dense, calm, mobile-first Telegram-inspired chat experience that makes messaging, persona identity, media, settings, memories, and future companion activities easy to scan and use.

## Confirmed Facts

- The client is a single vanilla JavaScript page built by `build()`, `render()`, and `bind()` in `src/main.js`; presentation is centralized in `src/style.css`.
- On screens at or below 650px, the fixed menu button overlaps the chat header because a late duplicate CSS block overrides the original left padding.
- The style sheet has a late compressed duplicate block that redefines desktop, mobile, drawer, composer, and dialog styles. It must be consolidated into a single authoritative rule set.
- The page uses `100vh` and a root `overflow: hidden`, which does not account for dynamic browser chrome, mobile keyboards, or safe areas.
- The current emoji target appends the complete emoji-list text instead of the one emoji a user selected.
- Existing state refreshes on focus and every five seconds while visible; the UI must preserve this behavior unless the relevant data flow is deliberately updated together with the server.

## Requirements

### Revised Navigation And Chat Scope (2026-08-19)

- The primary application shell uses a Telegram-inspired conversation-first layout with exactly three fixed bottom navigation controls: Contacts, Activity, and Settings. These controls use recognizable icons only; their accessible labels and tooltips may contain text.
- Contacts is the default destination and behaves like a Telegram chat list: all enabled companion conversations are vertically listed with persona identity and the best available last-conversation summary. Selecting a row opens a dedicated chat surface rather than leaving list and conversation permanently side-by-side.
- The chat surface has a left back control, a geometrically centered persona name that opens companion detail/memory, and a right settings action that retains the existing per-persona configuration capability. The message stream and composer remain in the middle/bottom of the surface.
- Activity remains a WeChat-Moments-inspired destination. This iteration preserves the existing product’s limited activity capability as a calm, non-debug placeholder/overview rather than treating the runtime console as a social feed.
- Settings becomes a navigable option list. Its entries open the existing system configuration, companion-management, memory, and developer-console capabilities instead of exposing configuration only from a sidebar.

- Establish one maintained visual system for the application and remove style-source-order conflicts before adding new presentation behavior.
- Make the chat shell resilient from narrow mobile screens through desktop layouts, including safe header spacing, dynamic viewport sizing, touch targets, text overflow behavior, and keyboard-safe composer placement.
- Preserve the practical Telegram cues shown in the supplied references: compact navigation, recognizable conversation hierarchy, clear persona identity, readable message grouping, and command-focused composer controls. The product must not reproduce Telegram branding or assets.
- Treat Chats and the all-persona activity timeline as peer primary destinations. Persona detail owns the individual Moments feed. On desktop, the conversation/persona list and the active destination share the main workspace; on mobile, the active destination is full-screen with compact navigation to the list and timeline.
- A persona's visible current status is a concise human-readable summary of its authoritative current event/situation only. It is not a separate online-presence, typing, or schedule-monitoring system, and clears or changes as the underlying event resolves.
- The first-release display system uses one bright Telegram-inspired theme. Theme switching and dark mode are out of scope, although implementation should avoid hard-to-replace color literals where a local design token is appropriate.
- Unread direct messages and activity updates use distinct signals: direct messages show an exact per-persona unread count; global activity has only one small red-dot indicator, never an activity count. Screening suppresses both signals for that persona.
- Make composer actions fully functional and accessible, including individual emoji selection, attachment display/removal, multi-line message entry, send/busy states, and destructive-action confirmation.
- Provide a clear, non-debug-facing activity/feed surface: an all-persona chronological timeline with filters and unread indicators, plus an individual Moments-style feed in each persona detail view. Posts must preserve author identity and offer a scoped route into comments or direct chat.
- Provide a visible persona screening state that hides the screened persona's new global-timeline posts and proactive direct messages until manually removed, while retaining non-destructive access to their history and persona detail view. It must not imply a relationship penalty.
- Render supporting-character mentions and reactions as contextual feed content when supplied by the companion domain, without exposing selectable accounts, profile links, or third-party chat affordances.
- Render only the bounded set of comments supplied for a post, with event participants visually treated as the relevant social context and no client-side invention of additional supporting-character activity.
- Support a user-visible like action and threaded text comments on activity posts. Likes become lightweight persona-private relationship evidence but do not mechanically create a direct message.
- Treat post media as mutually exclusive ordered-image-set or single-video content. First-release cards render at most one optional image, while later galleries and video cards remain compatible extensions of the same content shape.
- Do not provide an edit affordance for published persona posts, supporting-character reactions, or attached media. The feed renders those items as immutable authored records.
- Provide a non-destructive hide/restore control for an activity post. Hiding removes it only from the current user's visible global and persona feed views; it is not presented as post editing or deletion.
- Reserve stable media geometry for queued image/video work in both chat and activity cards, rendering an explicit skeleton/loading state until a durable job reaches a terminal state. The surrounding text and controls remain usable while media is pending.
- Resolve a completed media job by replacing the original item's placeholder in place on state refresh. Do not emit a second chat message/activity post or a new unread signal merely because its asset became available.

## Acceptance Criteria

- [ ] At 320px, 375px, 650px, tablet, and desktop widths, the chat header, menu, persona identity, message area, composer, drawers, and dialogs have no overlapping or unreachable controls.
- [ ] The application has exactly three icon-only fixed bottom tabs: Contacts, Activity, and Settings. Contacts opens by default; neither desktop nor mobile uses the legacy hamburger/sidebar navigation.
- [ ] Tapping a contact opens a dedicated chat page. Its header has a back action on the left, a centered clickable name in the middle, and an existing-functionality settings action on the right.
- [ ] The focused chat surface hides the three primary bottom tabs; returning to Contacts restores them.
- [ ] Routine and schedule state projections never directly publish an activity. A qualifying life event is first presented to the persona model, which decides whether to publish and, only when publishing, supplies the post text and an optional none/image/video media choice.
- [ ] A persona decision not to publish preserves the event and current state but creates no activity, media job, or activity-unread signal.
- [ ] The Contacts page lists every enabled persona as a conversation row with identity and an available preview; Settings presents existing configuration actions as a list of options.
- [ ] The mobile composer remains reachable with dynamic browser chrome and an on-screen keyboard; long attachments and text cannot force horizontal overflow.
- [ ] Selecting one emoji inserts only that emoji, and all composer controls have accessible labels and keyboard behavior appropriate to their function.
- [ ] Clear-console behavior asks for confirmation, prevents duplicate requests while pending, and displays a failure state without pretending that data was cleared.
- [ ] Persona switching, text send/SSE receive, settings, memory management, attachments, and generation cards continue to work after visual refactoring.
- [ ] The UI offers a global chronological activity timeline and a persona-specific Moments feed, with clear author identity, filters/consumption state, and a scoped path into comments or direct chat; neither surface exposes raw prompts or debug logs.
- [ ] Chats and global activity are directly reachable primary destinations on both desktop and mobile; persona details expose only their own Moments feed without breaking the scoped interaction contract.
- [ ] Supporting-character content can enrich a post without presenting it as a selectable companion, detail page, or chat participant.
- [ ] Each post visibly respects its server-provided comment limit and preserves clear attribution between the author, user comments, and supporting-character reactions.
- [ ] Users can like and comment on a post; the resulting states remain scoped to its author persona and do not automatically duplicate into direct chat.
- [ ] Pending media renders a stable, non-jumping skeleton/loading state in chat and activity; provider delays or failures do not block the associated text/event or leave inaccessible controls.

## Out of Scope

- Copying Telegram source code, branding, stickers, proprietary assets, protocol behavior, or account features.
- External notifications, external contact lists, and a complete social network.
- Dark mode and user-selectable themes.

## Planning Status

Product intent, `design.md`, and `implement.md` are complete. The task remains in planning until the user reviews and approves implementation.
