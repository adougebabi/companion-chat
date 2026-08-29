# Implementation Plan: Telegram-style Chat Interface

## Order of Work

0. Replace the rail/drawer navigation with a bottom, icon-only Contacts / Activity / Settings / Diagnostics navigation state; make the Contacts list the default view and chat a focused surface with explicit back navigation.
0.1. Recompose the chat header into fixed back, center-name/detail, and settings slots; expose existing settings, persona management, memory, and console features from the Settings list.

1. Reconcile the duplicated CSS tail and establish the bright local token/spacing system without changing server behavior.
2. Repair known interaction defects: header/menu collision, individual emoji action, attachment text overflow, dynamic viewport/safe-area layout, auto-growing composer, and clear-console feedback.
3. Add destination state and navigation for Chats, Activity, persona detail, screened personas, and the developer inspector.
4. Render table-backed persona list signals: current-event summary, direct unread count, screen state, and last-content summary.
5. Implement cursor-paged global activity and persona Moments views, red-dot read watermark, immutable post controls, likes/comments, supporting-character rendering, and hide/restore.
6. Add stable media skeletons and in-place job refresh to chat and feed cards.
7. Implement persona detail memory/evolution governance and concise current/near-term status views.
8. Run the responsive/accessibility/manual verification matrix after each major route; fix full-scope issues before handoff.

## Validation

- Mobile header never overlaps menu/avatar/title at 320px or 375px.
- Keyboard opens without hiding composer; long filenames/text cannot force horizontal scroll.
- One emoji click inserts one emoji.
- Console clear confirms, blocks duplicate submission, and reports failure.
- Direct-message unread count and activity red dot follow their independent read paths.
- Screened personas produce no normal-list, feed, or proactive-message signal and can be restored.
- Activities remain author-scoped, immutable, hideable/restorable, and correctly paged.
- Loading media preserves card geometry and resolves in place after refresh.
- Existing chat streaming, attachments, generation cards, persona switching, settings, and memory screens still work.

## Commands

```sh
node --check server.js
npm start
```

Run against a fresh temporary database when exercising the new life domain. Open `http://localhost:4178`, inspect browser-console errors, and capture desktop/mobile screenshots for the listed viewport matrix.

## Rollback

Keep UI changes behind additive API consumption until the backend domain endpoints are ready. Do not add mock/feed data that becomes an alternate client authority. Reverting the UI changes should leave untouched server records and legacy local data.
