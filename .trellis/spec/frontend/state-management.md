# Client State And Synchronization

## State Categories

Pinia app state is the server snapshot (`settings`, `personas`, `groups`, and
activity flags). The conversations store keeps the currently loaded page(s) of
each active 摇光实例 conversation, not necessarily the complete history.
`activePersonaId` is persisted only in `localStorage` under the existing
`companion-active-persona` key. Draft, selection, history anchor, attachments,
`isSending`, and `isComposing` remain transient composable/UI state.

## Server State Rules

Use the generated browser client at the BFF boundary for JSON endpoints; it throws on
non-2xx responses using the server's `error` field. `bootstrap()` loads
contacts first; selecting a 摇光实例 loads the latest bounded message page,
and history pagination requests older pages by cursor. Background refresh must
not eagerly fetch or replace the active conversation page. Polling is disabled
while sending, composing, or while the document is hidden.

Do not treat optimistic messages as persisted until the server stream emits `done` or a later refresh returns them. A streamed chat may end in several separately persisted assistant records: read ordered `payload.messages` first, then fall back to `[payload.message]` for a pre-migration server. Replace the one transient typing entry with that whole collection in order; do not leave the transient entry between or after persisted messages. History pages merge by message ID at the head; new messages merge at the tail. An initial or background page is authoritative for any matching message ID, while local-only optimistic messages are retained and ordered by timestamp. This prevents a queued media placeholder from overwriting the server's later ready projection. Generation jobs are queued through the server chat contract and restored from conversation state after a refresh.

## Derived UI

Compute counts and labels from `state` during `renderMemory()`/`renderPersonaList()` instead of maintaining duplicate counters. When switching personas, update `activePersonaId`, `localStorage`, `messages`, and the input hint together, as `switchPersona()` does.

## Common Mistakes

- Reading `state.personas[0]` when the saved active persona was deleted; `boot()` must fall back first.
- Refreshing during an active stream and overwriting incremental `messages`.
- Replacing the composer DOM during polling, pagination, or streaming and closing a mobile IME.
- Clearing a draft because a background request completed or a provider call is still pending.
- Sending duplicate chat requests; `send()` guards with `isSending` and disables the send button.
- Assuming localStorage contains server truth; it stores only the selected persona ID.

## Scenario: Timeline and deferred-chat projection

### 1. Scope / Trigger

- Trigger: the companion API exposes current scene/location, timeline decisions, or deferred sleep-reply diagnostics.

### 2. Signatures

- Ordinary persona detail `state` includes `scene`, `location`, and `room` in addition to situation/mood/source.
- Debug-only `GET /api/companion/personas/:personaId/lifecycle` additionally includes `timeline`, `decisions`, and `deferredBatches`.
- A deferred chat response emits the normal SSE `done` shape with `messages: []`; it does not emit a synthetic assistant message or a new SSE event name.

### 3. Contracts

- Ordinary UI renders the user-facing scene/location only; it never renders batch delivery time, random draw, intimacy score, or decision rationale.
- The debug inspector may render timeline and batch summaries only when `debugInspector` is true.
- Treat an empty `done.messages` as a completed transport response, not an error or a typing entry that should remain on screen.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Deferred batch chosen | Remove transient typing state; render no fake assistant message. |
| Poll later returns real assistant reply | Render it as an ordinary persisted message. |
| Debug flag disabled | Never request lifecycle-only decision/batch fields. |

### 5. Good/Base/Bad Cases

- Good: a sleeping persona replies later with a normal message and no visible “queued” state.
- Base: detail view shows `图书馆自习区 · 学校` as current scene/location.
- Bad: displaying an internal `deliverAt`, probability, or raw deferred-batch JSON to the user.

### 6. Tests Required

- Verify the client safely handles `done.messages = []`.
- Verify ordinary detail UI consumes scene/location fields and inspector-only fields remain gated.
- Verify background polling does not manufacture a duplicate reply after a deferred batch settles.

### 7. Wrong vs Correct

#### Wrong

```js
messages.push({role: 'assistant', text: '她正在睡觉，稍后回复'});
```

#### Correct

```js
// Keep the user message; wait for the later persisted assistant reply.
activeMessages = activeMessages.filter(message => message !== typingEntry);
```

## Scenario: Trusted Time Facts

### 1. Scope / Trigger

- Trigger: persona detail or chat uses a server-projected state with an end boundary, time fact, or next boundary.

### 2. Signatures

- State payloads expose scene, location, room, start time, end time, time fact, and next boundary.

### 3. Contracts

- UI labels use server-projected scene and location; browser code never infers class, work, or an end time from role text.
- A daily-plan baseline such as sleep or lying in is a real state, not an empty or loading state.
- An unknown time fact means no client-side end-time claim can be rendered or constructed.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Server says daily-plan baseline | Render its sleep/rest situation normally. |
| Server says known time fact | Display a supplied boundary only where the UI needs it. |
| Server says unknown time fact | Do not fabricate an end-time hint. |

### 5. Good/Base/Bad Cases

- Good: a sleep baseline remains visible before a 10:00 plan slot.
- Bad: showing 上课中 because the persona role includes 学生.

### 6. Tests Required

- Verify the detail view uses server scene, location, and room.
- Verify a plan baseline and explicit plan slot do not cause routine labels to leak into chat.

### 7. Wrong vs Correct

#### Wrong

    const status = /学生/.test(persona.role) ? '上课中' : persona.currentSituation;

#### Correct

    const status = persona.currentSituation;

## Scenario: Visual Identity Timeline Projection

### 1. Scope / Trigger

- Trigger: Fluctlight detail includes the server-owned `visual_identity` snapshot and its image-generation timeline.

### 2. Signatures

- Detail field: `visual_identity = {status, current_revision, renderer_constraints, canonical_asset_id?, character_sheet_asset_id?, timeline[]}`.
- Timeline event: `{stage, status, summary, asset_ids, metadata, occurred_at}`; images load only through `/api/media/:assetId`.

### 3. Contracts

- The server is authoritative for stage order, attempt status, `accepted`/`regenerate`, and identity-match decisions. Vue never judges image content or parses prompts.
- The detail dialog may refresh the projection while a visual identity session is non-terminal; once `status=active`, polling stops.
- Candidate images remain visible beside later attempts. Stable image boxes use `overflow-anchor: none` so late media cannot move the reader.
- Transport retry is distinct from Visual Identity regeneration; no client retry reuses a completed turn to create a new attempt.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing `visual_identity` | Render a quiet “尚未创建” state; do not infer defaults. |
| Queued/running timeline | Show stage/status and refresh boundedly while dialog is open. |
| Asset not ready or proxy fails | Keep stage and show the image error state; never call ComfyUI directly. |
| `media` stream event contains `message` or `messages` | Merge by message ID/sequence without replacing newer ready projections with queued placeholders. |

### 5. Good / Base / Bad Cases

- Good: `image_ready → vision_ready → patch_ready → regenerate` remains visible with both candidate images and a later canonical/character-sheet card.
- Base: an old detail payload has no visual identity field; the dialog still renders the rest of the persona detail.
- Bad: deciding “不是自己” from alt text/file name, deleting rejected attempts, or fetching an internal provider URL.

### 6. Tests Required

- Assert the dialog renders status, stage, attempt timeline, canonical and character-sheet images with safe alt text.
- Assert bounded refresh stops at active and does not overwrite user-composed chat state.
- Assert `media` events merge one or multiple messages and late image loading preserves layout anchors.

### 7. Wrong vs Correct

#### Wrong

```ts
if (message.text.includes("不是自己")) startRegeneration();
```

#### Correct

```ts
// Render the server projection; only the backend patch decision advances an attempt.
renderVisualIdentityTimeline(detail.visual_identity);
```

## Scenario: Settings Section Switching And Payload Normalization

### 1. Scope / Trigger

- Trigger: a settings section is changed without a route-level component
  remount, or a migrated API payload uses an older field name.

### 2. Signatures

- `SettingsView` receives `section?: SettingsSection | null` and renders one
  matching Accordion item.
- `normalizeActorGroups(values: unknown): ActorGroupSnapshot[]` is the single
  browser owner for group payload compatibility.

### 3. Contracts

- Section-specific Accordion roots use a key derived from `currentSection` (or
  a controlled model) so the selected item opens immediately when the prop
  changes; the whole app must not reload.
- Server snapshots are normalized before entering Pinia. Every stored actor
  group has `actor_ids: string[]`, even when the server temporarily returns
  legacy `members`.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Settings section changes | The new section content is visible without refresh. |
| Accordion trigger is clicked | The user can still collapse/reopen the section. |
| Group payload has `members` | It is copied to `actor_ids` before derived filters run. |
| Group payload is malformed | It is dropped or treated as empty; no `undefined.includes` call. |

### 5. Good/Base/Bad Cases

- Good: navigating from model-role to media changes the visible form in the
  same view instance.
- Base: a cached group payload with `members` remains selectable on mobile.
- Bad: passing a changing `default-value` to a reused uncontrolled Accordion or
  storing raw group JSON in Pinia.

### 6. Tests Required

- Static/component test for section key or controlled Accordion state.
- Normalizer tests for current and legacy group payloads plus empty fields.
- Narrow viewport regression pass for group tabs and section content.

### 7. Wrong vs Correct

#### Wrong

```vue
<Accordion :default-value="currentSection">
```

#### Correct

```vue
<Accordion :key="currentSection" :default-value="currentSection">
```

## Scenario: Mobile composer and image-safe background refresh

### 1. Scope / Trigger

- Trigger: background bootstrap/conversation polling runs while a mobile user is composing or reading messages that contain late-loading images.

### 2. Signatures

- `chatViewSnapshot()` captures the textarea draft, selection, stream scroll position, and whether the reader is near the bottom.
- `renderChat({followLatest?})` restores that snapshot after a controlled chat redraw.
- `refreshQuietly()` skips chat refresh while composing and only rebuilds the chat when the conversation payload changed.

### 3. Contracts

- Only a deliberate accepted send clears `chatDraft`; polling, bootstrap, detail refresh, persona refresh, and keyboard focus changes must not clear or refocus it.
- A reader away from the bottom keeps their prior scroll position after a chat redraw. Auto-follow is reserved for sends, stream tokens, and readers already at the bottom.
- Do not refresh chat from `window.focus`: mobile software keyboards create focus transitions unrelated to a user request.
- Media boxes use `overflow-anchor: none` so late image dimension resolution cannot move a reader to a different message.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Textarea focused or nonempty | Poll skips chat rebuild. |
| Conversation unchanged | Refresh updates only sidebar state. |
| Reader is scrolled up | Restore previous `scrollTop`, not bottom. |
| Reader is at bottom and a new response arrives | Follow the latest message. |

### 5. Good / Base / Bad Cases

- Good: type half a message on a phone during polling; the keyboard and draft remain intact.
- Base: an idle, empty composer receives a new assistant message and follows it naturally.
- Bad: using `window.focus` to refresh, recreating the textarea, and forcing `scrollTop = scrollHeight` for every render.

### 6. Tests Required

- Manual mobile pass: type with the virtual keyboard open through several polling intervals and verify the draft/selection stay intact.
- Manual scroll pass: read older text above a late-loading image and verify the viewport stays anchored.
- Client regression check: unchanged poll does not call a full chat render.

### 7. Wrong vs Correct

#### Wrong

```js
window.addEventListener('focus', refreshQuietly);
renderChat(); // always forces the stream to the bottom
```

#### Correct

```js
if (editing || chatDraft) return;
if (conversationChanged) renderChat({followLatest: false});
else renderSidebar();
```
