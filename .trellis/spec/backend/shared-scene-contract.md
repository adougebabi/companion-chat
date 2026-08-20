# Shared Scene Contract

## Scenario: Single-persona shared scene and native scene tool

### 1. Scope / Trigger

- Trigger: chat needs to preserve a material shared location/activity across turns without parsing parenthesized natural-language actions.
- Scope: one user and one selectable persona; no cross-persona world or participant references.

### 2. Signatures

- `scene_event` model tool: `{operation: 'start'|'switch'|'end', location?, room?, activity?, situation?, mood?, objects?, participants?}`.
- `applySceneEvent(persona, call, causationUserMessageId) -> {eventId, operation, scene, previousScene}`.
- `sharedSceneFor(personaId) -> SharedScene | null`.
- `resolvedStateFor(personaId)`: shared-scene projection takes precedence while `shared_scene_json` is non-empty.
- Migration columns: `companion_personas.image_generation_policy` (default `autonomous`) and `companion_persona_states.shared_scene_json` (default `{}`).

### 3. Contracts

- Parenthesized text is optional natural-language action prose. It is stored/rendered as ordinary message text; no parser, keyword, or regex may create a scene transition from it.
- `start` and `switch` require a non-empty `location` or `activity` plus `situation`; `end` requires only `operation`.
- A valid call creates `companion_life_events.type = 'shared_scene'` for `start`/`switch` or `shared_scene_end` for `end`, with `causation_id` bound to a user message in the same persona conversation.
- `start`/`switch` update `shared_scene_json`; `end` clears it and resumes the existing schedule/routine projection. No activity or proactive message is published.
- The chat request advertises exactly one `scene_event` tool and preserves existing `token`, `done`, and `error` SSE names. A tool-only completion may use one standard tool-result continuation; this is not a hidden semantic extraction pass.
- Image-generation policy values are `ask`, `always`, `important`, `user_only`, `autonomous`; the server exposes the value only in persona detail and the detail update route. The model receives the behavioral meaning in the system capability layer; the server does not text-match it.
- For `always`, the model must append one validated image `<media-intent>` whenever its user-visible reply contains a parenthesized action; an ordinary reply without an action does not force an image. This is a model instruction, not a server parser.
- When the chat provider advertises tools, `media_event` is the preferred native delivery path; `<media-intent>` remains a compatibility fallback and the same turn must not create duplicate media jobs.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown operation, unsupported field, malformed array, or invalid participant | Reject the tool call; do not write an event or state projection. |
| `start`/`switch` missing location/activity or situation | Reject the tool call; keep the previous scene. |
| Missing or foreign causation user message | Reject the tool call; keep the previous scene. |
| More than one `scene_event` in a completion | Execute none and return a bounded tool error; keep visible text. |
| Invalid policy update | HTTP 400; stored policy is unchanged. |
| Continuation completion fails after a committed scene event | Keep the scene event and return a concise fallback assistant message. |

### 5. Good / Base / Bad Cases

- Good: the model writes `(把伞递给你)` in ordinary text, then later calls `scene_event` after a confirmed move from a cafe to a park; the scene remains in the next context.
- Base: a short `(看了你一眼)` action is retained in the message but creates no event or database state.
- Bad: `/去公园/` or `/拍照/` in user text directly calls `applySceneEvent()` or queues media.

### 6. Tests Required

- Assert migration defaults and normalization for both new columns.
- Assert detail policy read/write for all five values and HTTP 400 for an unknown value.
- Assert `start`, `switch`, and `end` write persona-scoped events, preserve previous scene metadata, and make `resolvedStateFor()`/`contextFor()` consistent.
- Assert a regular life event does not overwrite an active shared scene and `end` clears the shared projection.
- Assert streamed tool-call argument fragments concatenate into valid JSON and ordinary parenthesized message text remains unchanged.
- Assert the `always` policy line explicitly requires an image marker for replies containing parenthesized actions; a weaker “may initiate” instruction is a regression.
- Assert no scene or media transition is created from free text without a model tool/marker contract.

### 7. Wrong vs Correct

#### Wrong

```js
if (/去公园|拍照/.test(userText)) applySceneEvent(persona, {operation: 'switch', location: '公园'}, message.id);
```

#### Correct

```js
const toolCall = modelCompletionToolCall('scene_event');
if (toolCall) applySceneEvent(persona, normalizeSceneEventCall(toolCall.arguments), userMessage.id);
```
