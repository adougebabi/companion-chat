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
