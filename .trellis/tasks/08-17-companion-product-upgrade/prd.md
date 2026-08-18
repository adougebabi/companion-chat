# AI companion product upgrade

## Goal

Evolve the local AI companion into a believable, persistent companion while replacing the existing chat surface with a mobile-first Telegram-inspired experience. The product should make an established persona feel situated in a changing daily life, without losing user control over identity, privacy, or generated content.

## Confirmed Facts

- The application is a local-first Express 5 and vanilla ES module web app. The browser UI is implemented in `src/main.js` and `src/style.css`; `server.js` owns all API routes, external-model calls, and persistence.
- Runtime state is a SQLite-backed single JSON document. It currently includes settings, personas, persona-scoped memories, conversations, generation/debug logs, and generation jobs.
- The product already stores persona memories and runs a background prompt-evolution review after 10 minutes of idle time. It has no model for schedule, routine, scene, life event, mood, appearance, activity feed, or proactive companion behavior.
- The mobile UI has confirmed regressions caused by overlapping duplicate CSS. It is not yet robust to dynamic mobile viewport changes or a narrow chat header, and some interaction controls are incomplete.
- Existing uncommitted changes in `src/main.js` and `src/style.css` predate this task and must not be reverted or silently absorbed.

## Child Deliverables

| Child task | Ownership | Dependency |
| --- | --- | --- |
| `08-17-telegram-chat-interface` | Telegram-inspired responsive chat information architecture, interaction controls, mobile behavior, and visual verification | Can proceed independently; consumes activity/situation display only after its own core shell is complete. |
| `08-17-lifelike-ai-companion` | Daily-life state, event lifecycle, prompt/context rules, activity feed, and evolution governance | Defines stable activity and situation contracts for the interface task. |

## Cross-Task Requirements

- Begin the new companion domain from clean table-backed data. Do not migrate or read legacy personas, conversations, memories, generation jobs, or media references; do not delete legacy local files during upgrade.
- Keep user-visible companion facts distinct from debug and generation logs, which can contain prompts and model output.
- A companion's original identity remains stable. Adaptation may extend behavior and knowledge of the user but cannot silently replace core background or personality.
- The two child tasks must agree on a stable user-visible activity contract before the interface renders life events or dynamic posts.
- Long-lived life-domain data requires a deliberate evolution from the current single JSON state document to versioned, indexed tables in the same local SQLite database; this remains a local single-service product and does not require external infrastructure.

## Cross-Task Acceptance Criteria

- [ ] Both child PRDs contain independently testable acceptance criteria and a documented API/data contract at their boundary.
- [ ] The first launch creates no demo/legacy companion data and guides the user to the persona interview; legacy local files remain untouched and unread.
- [ ] Mobile and desktop experiences support the same chat and activity information without overlapping controls or inaccessible actions.
- [ ] Companion state changes are attributable to an explicit schedule/event/action and are visible to the user in an appropriate product surface.
- [ ] New life-domain persistence supports long-term paged history without whole-state rewrites. The release neither migrates nor reads legacy local data and must not delete it during upgrade.

## Out of Scope

- External calendar synchronization, push notifications, social-network integrations, multi-user accounts, and cloud synchronization are not assumed for the first release.
- Replacing the app's vanilla JavaScript architecture is not part of either child task.

## Planning Status

Product intent and cross-task contracts are converged. The parent and both child tasks have design/implementation artifacts; all remain planning-only until the user reviews and approves implementation.
