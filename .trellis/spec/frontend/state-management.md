# Client State And Synchronization

## State Categories

`state` is the server snapshot (`settings`, `personas`, `memories`). `messages` is the active persona conversation. `activePersonaId` is persisted only in `localStorage` under `companion-active-persona`. `attachments` and `isSending` are transient browser state.

## Server State Rules

Use the `api()` helper for JSON endpoints; it throws on non-2xx responses using the server's `error` field. `boot()` loads state and the active conversation, `switchPersona()` loads a new conversation, and `refreshState()` reloads both. The five-second refresh interval is disabled while sending and while the document is hidden.

Do not treat optimistic messages as persisted until the server stream emits `done` or a later refresh returns them. A streamed chat may end in several separately persisted assistant records: read ordered `payload.messages` first, then fall back to `[payload.message]` for a pre-migration server. Replace the one transient typing entry with that whole collection in order; do not leave the transient entry between or after persisted messages. Generation jobs are queued through `/api/generate` and restored from conversation state after a refresh.

## Derived UI

Compute counts and labels from `state` during `renderMemory()`/`renderPersonaList()` instead of maintaining duplicate counters. When switching personas, update `activePersonaId`, `localStorage`, `messages`, and the input hint together, as `switchPersona()` does.

## Common Mistakes

- Reading `state.personas[0]` when the saved active persona was deleted; `boot()` must fall back first.
- Refreshing during an active stream and overwriting incremental `messages`.
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
