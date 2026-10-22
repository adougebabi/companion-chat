# Confirmed Pen production interaction contracts

## 1. Scope / Trigger

Apply to the production renderer `main.ts → App.vue → v3/proto/app.js` when implementing the confirmed `ux-round2-closure-2026-10-22` design. The directory name `proto` does not authorize a second demo implementation. The user owns unified visual/business acceptance for the 2026-10-22 implementation; syntax, type and build results do not imply that acceptance.

## 2. Signatures

- `mountCalendarReading(el,{scope,zone,read,dateOf,render,onView,...})` retains a UI reading snapshot keyed by Owner and business scope.
- `StudioComparison({name,body},proposal)` / `studioChanges(draft,proposal)` compare complete current drafts.
- `mountStudioJobs(...).updateDraft()` must be called by real editor input persistence, including NPC definitions.
- Configuration save slots are `binding:<role>`, `conversation.reply`, `vision.understanding`.
- `sourceHref(href,ctx,label)` captures the actual source route. `sourceReturn` accepts local `#/` routes only.
- `readCreationProgress(client,fid,{localDate,worldId})` reads independent setting-sheet, display-avatar, accepted-schedule and world authorities.

## 3. Contracts

- Existing visual identity/candidates and separate display avatar load automatically. Polls only observe the captured character/session. A changed session requires a fresh explicit read; observers never generate/review/accept. Later read failures retain successful images and explain stale state.
- Assistant proposals compare against current full `name/body`, including edits during generation. Changed fields come first; unchanged/full content is optional. One modal makes the background inert. Escape/cancel preserves draft, suggestion, disclosures, focus and scroll. Adoption changes the draft; saving/publishing/applying remain separate.
- One model-purpose draft per role. Switching endpoints must persist cleared model/window/derived fields immediately, even when no input was typed. Saved summaries derive only from confirmed DTOs. A selected endpoint is not an already saved binding. Save snapshots and late callbacks do not clear newer edits or other slots.
- Fact-only input/output limits are the minimum of the saved role binding and reply limits. Unknown values remain unknown. Reply timeout includes queueing and is bounded by an earlier upstream deadline; legacy/fallback uses its actual path.
- Global and per-character autonomy labels share `autonomyModes`: 允许自治行动 / 暂停新自治行动 / 屏蔽自治外发. Notification muting remains separate.
- Diary date is the authoritative local day. Memory calendar date is local `created_at`; never substitute source or `occurred_at`. Calendar dots/counts represent loaded records. Explicit date seeking reads at most three cursor pages per action and offers continuation. A partial range cannot prove emptiness.
- UI reading caches retain cursor/loaded data/source routes. Refresh failures preserve prior content, and failed pagination keeps the visible page's cursor. Detail reads reconcile cached record revisions. Logout clears Owner-scoped drafts and reading snapshots.
- The creation main flow is world → blank/description/JSON → four independent stages → completion. Template creation remains a separate linked resource branch with review. A durable activated checkpoint prohibits a second activation. Four completed stages plus the selected world readback are required for completion. Progress order is presentation, not a claim of serial backend execution.
- Ordinary-object borrowing/returning and cross-object draft-to-chat chains remain cancelled. Do not add `?say=` links or infer a physical encounter from browsing/media/remote communication.

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

## 5. Good / Base / Bad Cases

- Good: adopt a reviewed proposal, see unsaved draft status, then save separately.
- Base: an old NAS without newly required read contracts displays unavailable state while other real functionality remains usable.
- Bad: clear images on a failed refresh; count a cursor page as a total; count task acceptance as completed initialization.

## 6. Tests Required

Assertion points for unified acceptance: live editor input updates comparison; cancellation preserves caret/scroll/disclosures; no duplicate adoption handlers after polling; endpoint X→Y→X does not restore a second per-endpoint draft; three save slots preserve independent in-flight changes; old data survives read failures; memory created/source dates remain distinct; returned cursor/filters/date/record revisions match their source; progress cannot complete without actual readable artifacts and schedule/world readback. Existing task evidence is partial and must not be generalized to all groups.

## 7. Wrong vs Correct

Wrong: `total = page.items.length; reloadPageAfterReview(); adopt(snapshotAtGenerationStart)`.

Correct: display loaded count, preserve successful local read projections, observe the original task and compare `readDraft()` immediately before confirmation; after confirmation recheck the reviewed snapshot before adopting to the working draft.
