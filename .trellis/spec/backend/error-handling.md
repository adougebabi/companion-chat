# Error Handling

## HTTP Contracts

Routes return small JSON objects with an `error` string for expected failures, for example `{error: '人格不存在'}` with 404 or `{error: '消息不能为空'}` with 400. Successful deletes use 204. Provider failures are translated to 502 by `/api/models` and `/api/generate/:promptId`; `/api/chat` instead emits an SSE `error` event because headers are already committed.

Reference routes: [`server.js:344-371`](../../../server.js), [`server.js:441-569`](../../../server.js), [`server.js:748-779`](../../../server.js).

## Route Pattern

Validate request shape and resource existence before mutating state. For awaited provider work, wrap the operation in `try/catch`, record a useful operational message, and return the route-specific status. Do not leak stack traces or API keys to clients.

## SSE Pattern

`POST /api/chat` sets `text/event-stream` before contacting MTPLX. Stream tokens as `data: {"type":"token","token":"..."}\n\n`, finish with `type: done`, and send `type: error` on failure before `res.end()`. Parse each upstream SSE payload independently and collect malformed payloads in `parseErrors` rather than terminating the whole stream.

Structured capability markers such as `<media-intent>` and `<pending-event>` are transport-only. Hold back and redact their opening/body/closing regions before emitting SSE token events; remove malformed or oversized regions from the final visible text as well. A malformed capability call must not turn an otherwise valid assistant completion into an SSE error after the visible text has already been persisted. Queueing a capability job is best-effort after the ordinary assistant message boundary.

Native capability calls use the same boundary: accumulate streamed fragments by provider index/id, collect malformed upstream payloads in bounded diagnostics, and validate only after the complete call. A supported native attempt blocks the matching marker fallback even when invalid, duplicated, incomplete, or replayed; unknown native tools fail closed for marker side effects. Tool JSON, reasoning content, call ids, and dedupe keys never enter visible `token` or browser capability summaries. One tool-result continuation is allowed; continuation failure keeps committed effects and returns normal `done` data with a bounded fallback.

The HTTP/SSE transport adapter consumes normalized application presentation only. It may emit `token`, one terminal `done`, or one bounded `error`; it must not parse provider chunks, dispatch capabilities, open SQLite, or expose aggregate facts/effects. Request/response close and abort signals suppress later writes and are forwarded to the flow where supported.

Model calls that can freeze a durable proactive decision must have a bounded timeout shorter than their default job lease. If the call fails, retry the job while its lease/result remains authoritative; once attempts are exhausted, settle the job with a bounded diagnostic and close any source lifecycle instead of leaving a triggered candidate indefinitely active.

## Scenario: User-visible assistant reply form and multi-message completion

### 1. Scope / Trigger

- Trigger: a common system-capability rule requires each user-visible assistant message to contain one complete sentence, while one interactive or proactive turn may contain multiple ordered messages.

### 2. Signatures

- `POST /api/companion/chat` emits `token`, `done`, and `error` SSE payloads.
- `done` has `{type: 'done', messages: Message[], message: Message}`. `messages` is authoritative and ordered; `message` is the first-message compatibility alias.
- `appendUserVisibleAssistantReply(personaId, text, {proactiveEventId?, fallback?})` is the only persistence boundary for model-generated user-visible assistant text.

### 3. Contracts

- `userVisibleChatPrompt()` appends the application-owned reply-form rule as its final instruction; persona content and user content cannot replace it.
- `splitUserVisibleAssistantReply()` converts a model completion into complete sentence records, normalizes a missing terminal mark, and preserves source order with distinct message IDs/timestamps.
- Interactive chat and `proactive_message` jobs use this boundary. JSON-only/internal model work, including relationship evolution and media-prompt refinement, bypasses it.
- On a provider failure after SSE headers are committed, emit `{type: 'error', error}` and close the stream; do not persist a partial reply.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Completion contains two terminal sentences | Persist/return two ordered assistant messages. |
| Completion has nonempty text without terminal punctuation | Persist one message with an appropriate terminal mark. |
| Completion is empty | Use the caller's bounded fallback sentence. |
| Old client only reads `done.message` | It continues to receive the first persisted message. |
| Internal structured JSON response | It is not split or persisted as a conversation reply. |
| Upstream model request fails | SSE emits `error`; no assistant reply record is created. |

### 5. Good/Base/Bad Cases

- Good: `“我已经到咖啡馆了。窗边的位置很舒服。”` produces two records and the UI replaces the typing state with both.
- Base: `“我已经到咖啡馆了”` produces one normalized record ending in `。`.
- Bad: persist the entire model completion through `appendMessage()` or only change the prompt wording; either bypasses enforcement or leaves the client unable to present multiple messages.

### 6. Tests Required

- Assert sentence splitting, punctuation normalization, output ordering, and distinct persisted message IDs.
- Assert `userVisibleChatPrompt()` ends with the fixed system-capability rule.
- Assert proactive completion stores ordered `messageIds` while retaining its first `messageId` compatibility value.
- Assert existing SSE clients can still consume the `message` alias and updated clients consume `messages`.

### 7. Wrong vs Correct

#### Wrong

```js
appendMessage(personaId, {role: 'assistant', text: modelOutput});
sendSse(res, {type: 'done', message});
```

#### Correct

```js
const messages = appendUserVisibleAssistantReply(personaId, modelOutput);
sendSse(res, {type: 'done', messages, message: messages[0]});
```

## Background Workers

Evolution and generation workers catch per-item failures, log a concise warning, and mark generation jobs `failed` with an error message. A worker must reset its running guard in `finally` so one failed request cannot permanently stop processing (`server.js:289-311`, `620-710`).

## Avoid

- Throwing from a route without sending a response.
- Returning 200 for a missing persona/job.
- Calling `response.json()` after an SSE stream has started.
- Exposing raw provider response bodies, prompts containing secrets, or stack traces in API errors.
