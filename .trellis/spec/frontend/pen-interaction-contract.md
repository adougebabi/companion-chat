# Confirmed Pen production interaction contracts

## 1. Scope / Trigger

Apply to the production renderer `main.ts → App.vue → v3/proto/app.js` when implementing the confirmed `ux-round2-closure-2026-10-22` design. The directory name `proto` does not authorize a second demo implementation. The user's latest 2026-10-22 instruction requires agent-led Pen and production-preview regression as well as implementation. Syntax, type and build results do not imply visual or business acceptance; record the actual inspected states and data-dependent gaps.

## 2. Signatures

- `mountCalendarReading(el,{scope,zone,read,dateOf,render,onView,...})` retains a UI reading snapshot keyed by Owner and business scope.
- `StudioComparison({name,body},proposal)` / `studioChanges(draft,proposal)` compare complete current drafts.
- `mountStudioJobs(...).updateDraft()` must be called by real editor input persistence, including NPC definitions.
- Configuration save slots are `binding:<role>`, `conversation.reply`, `vision.understanding`.
- `sourceHref(href,ctx,label)` captures the actual source route. `sourceReturn` accepts local `#/` routes only.
- `readCreationProgress(client,fid,{localDate,worldId})` reads independent setting-sheet, display-avatar, accepted-schedule and world authorities.
- `requestDecisionText(request)` renders status/effect-qualified system reasons; `RequestDecisionSource(request)` retains a safely escaped, default-collapsed original request.

## 3. Contracts

- Existing visual identity/candidates and separate display avatar load automatically. Polls only observe the captured character/session. A changed session requires a fresh explicit read; observers never generate/review/accept. Later read failures retain successful images and explain stale state.
- Assistant proposals compare against current full `name/body`, including edits during generation. Changed fields come first; unchanged/full content is optional. One modal makes the background inert. Escape/cancel preserves draft, suggestion, disclosures, focus and scroll. Adoption changes the draft; saving/publishing/applying remain separate.
- One model-purpose draft per role. Switching endpoints must persist cleared model/window/derived fields immediately, even when no input was typed. Saved summaries derive only from confirmed DTOs. A selected endpoint is not an already saved binding. Save snapshots and late callbacks do not clear newer edits or other slots.
- Fact-only input/output limits are the minimum of the saved role binding and reply limits. Unknown values remain unknown. Reply timeout includes queueing and is bounded by an earlier upstream deadline; legacy/fallback uses its actual path.
- Global and per-character autonomy labels share `autonomyModes`: 允许自治行动 / 暂停新自治行动 / 屏蔽自治外发. Notification muting remains separate.
- Diary date is the authoritative local day. Memory calendar date is local `created_at`; never substitute source or `occurred_at`. Calendar dots/counts represent loaded records. Explicit date seeking reads at most three cursor pages per action and offers continuation. A partial range cannot prove emptiness.
- UI reading caches retain cursor/loaded data/source routes. Refresh failures preserve prior content, and failed pagination keeps the visible page's cursor. Detail reads reconcile cached record revisions. Logout clears Owner-scoped drafts and reading snapshots.
- The 2026-10-23 creation entry starts with a description and a resumable real default-world command after activation; published-world selection and blank/JSON modes remain optional. Four independent stages still determine full completion. Template creation remains a separate linked resource branch with review. A durable activated checkpoint prohibits a second activation. Four completed stages plus the selected world readback are required for completion. Progress order is presentation, not a claim of serial backend execution.
- Ordinary-object borrowing/returning and cross-object draft-to-chat chains remain cancelled. Do not add `?say=` links or infer a physical encounter from browsing/media/remote communication.
- The 2026-10-23 insight review supersedes the module sidebar: the five primary destinations are 聊天 / 她的生活 / 我们 / 她 / 角色. Existing detail/edit routes remain reachable from their reading home or More. Life combines saved diary/moments/images/goals with explicitly planned schedules; 我们 owns shared experiences and owner-referenced memory. Technical/service failures are not human tasks. The user's subsequent instruction removes the separate story/shared-experiences button beside diary filters; keep the sidebar route and real shared-experiences capability.
- The latest chat-summary reference requires a compact avatar/name/activity identity row, a single context strip, a schedule block and a slot-grouped wear block. Nearby people remain an optional secondary disclosure; 关于你 is a light footer entry. Use actual DTO values only, do not infer completion from a plan window, and omit unknown source-update timestamps. Ordinary schedule start instants retain complete Beijing labels; full ranges and original machine datetime remain available in details/title/aria. Fold/day/object-stack and scroll state stay scoped by Owner and character.
- Fixed English owner-life settlement reasons are presentation inputs, not ordinary copy. Map them using the actual status/error/disposition; preserve meaningful Chinese decisions and the original object. Unknown English reasons get an honest summary and source disclosure. Failed/unknown judgments cannot claim zero side effects or committed success; `no_change` success requires completed status. Rendering must not rewrite DTOs, historical data, request identity or retry semantics.
- Ordinary full timestamps use `formatBeijingInstant` / `userInstant` / `InstantLabel`: `YYYY-MM-DD HH:mm (北京时间)`. Convert the actual instant to Asia/Shanghai; do not relabel the original clock. Business local-day keys, IANA timezone settings, machine `datetime` attributes and original diagnostic records retain their semantics. Ordinary instant editors use Beijing `datetime-local` controls and a readable preview. `zonedInputInstant(value,'Asia/Shanghai',original)` preserves original precision/offset when the displayed minute is unchanged. Frozen pending commands are never reformatted or regenerated for presentation.
- Disclosure summaries are full-row controls with title, secondary state, status and chevron. Expanded details use neutral/sage surfaces and structured labels. Reading pages show saved content first; annotation, generation, provenance and maintenance have explicit entries or optional disclosures.
- World presentation follows the user's latest catalogue / searchable left tree / right form references, using real DTO data and the existing sage palette. Background and description remain independent fields; the 世界观 editor changes `background` and omits an untouched `description`. Unchanged, invalid, stale, busy and retired saves are disabled in both UI and submit handler.
- `sidebarSectionPath` maps editor and support routes to their visible sidebar owner without changing the actual route or backend workspace. World editors remain under 世界, goal editor/planning under 目标, diagnostics/data/setup under 管理中心.

## 4. Validation & Error Matrix

| Condition | UI behavior |
|---|---|
| Visual GET fails after images loaded | Keep images, show update error, retain original session |
| Draft changes while proposal runs | Recompute comparison from current full draft |
| Confirmation cancelled | Preserve all local work and comparison context |
| Endpoint changes | Clear model/window/derived fields in the same role slot |
| Input changes during config save | Acknowledge submitted snapshot; retain new draft |
| Date not covered by loaded cursors | State range is incomplete; allow bounded continuation |
| Activation response unknown | Query/replay original identity; never create a new character |
| Display avatar/sheet/schedule unreadable | Show pending/unknown/failed independently, not complete |
| Failed request carries no_change disposition | Keep failure; never title it already satisfied |
| Unknown English decision_reason | Readable summary; original remains in technical disclosure |
| planned status has no actual action_status | Show plan only; never infer completion from elapsed time |

## 5. Good / Base / Bad Cases

- Good: adopt a reviewed proposal, see unsaved draft status, then save separately.
- Base: an old NAS without newly required read contracts displays unavailable state while other real functionality remains usable.
- Bad: clear images on a failed refresh; count a cursor page as a total; count task acceptance as completed initialization.

## 6. Tests Required

Assertion points for unified acceptance: live editor input updates comparison; cancellation preserves caret/scroll/disclosures; no duplicate adoption handlers after polling; endpoint X→Y→X does not restore a second per-endpoint draft; three save slots preserve independent in-flight changes; old data survives read failures; memory created/source dates remain distinct; returned cursor/filters/date/record revisions match their source; progress cannot complete without actual readable artifacts and schedule/world readback. Existing task evidence is partial and must not be generalized to all groups.

`chat-life-summary.test.mjs` executes the real mount renderer and actions for compact hierarchy, plan/actual separation, avatar fallback, all-day/object stack/reading restoration and Owner/fid isolation. `request-reason-diary.test.mjs` executes actual dynamic, reflection, promise, receipt and detail renderers: the exact UTC sample `2026-10-20T15:01:58.000+00:00` must read `2026-10-20 23:01 (北京时间)` in ordinary output, with raw reason/status/time only in default-collapsed source. Diary toolbar contains only all/favorite filters; shared sidebar/route remains.

## 7. Wrong vs Correct

Wrong: `total = page.items.length; reloadPageAfterReview(); adopt(snapshotAtGenerationStart)`.

Correct: display loaded count, preserve successful local read projections, observe the original task and compare `readDraft()` immediately before confirmation; after confirmation recheck the reviewed snapshot before adopting to the working draft.
