# Technical Design

## Scope And Approach

Keep the existing single-process Express + SQLite architecture. Extend the current companion chat path with one model-native `scene_event` tool and keep ordinary parenthesized actions as message text. Do not add a second hidden semantic extraction call, a client-side intent parser, a keyword/regex trigger, or a separate multi-persona world model.

The design has two distinct layers:

1. **Natural expression:** system prompt teaches the model that ordinary prose expresses content and parenthesized prose expresses an optional action. The browser stores and renders the text as written for both user and persona.
2. **Durable scene transition:** after the model has naturally proposed a material location/activity change and interpreted the user's response as acceptance, it calls `scene_event`. The server validates the call and persists the shared scene. The server never decides acceptance from user wording.

## Persistence

Add one ordered companion migration after the current latest migration:

- `companion_personas.image_generation_policy TEXT NOT NULL DEFAULT 'autonomous'`
- `companion_persona_states.shared_scene_json TEXT NOT NULL DEFAULT '{}'`

Use internal policy values `ask`, `always`, `important`, `user_only`, and `autonomous`; the default is `autonomous`. The browser maps these stable values to the five Chinese labels in the persona detail `<select>`. Bootstrap persona summaries do not expose the setting. The persona detail response exposes it and a dedicated `PUT /api/companion/personas/:personaId/image-generation-policy` route updates it.

`shared_scene_json` is the current durable projection, not a second event history. It stores a bounded object with:

```json
{
  "location": "湖边公园",
  "room": "",
  "activity": "沿湖散步",
  "situation": "和用户一起沿湖边散步",
  "mood": "放松",
  "objects": ["雨伞"],
  "participants": ["user", "persona"],
  "startedAt": "2026-08-19T14:00:00.000Z",
  "eventId": "event_..."
}
```

Every accepted `scene_event` also creates a `companion_life_events` row (`type = 'shared_scene'` for `start`/`switch`, `type = 'shared_scene_end'` for `end`) with the complete validated payload, the source user message ID in `causation_id`, and the previous scene summary for a `switch`. This preserves an explainable history while the state column gives read paths a stable current projection. A scene end clears `shared_scene_json` and lets the existing schedule/routine projection resume.

Use a dedicated transactional `applySceneEvent(persona, call, causationId)` helper rather than trying to compose `createEvent()` with a second state update after its transaction. It updates the life-event row, `companion_persona_states` (`situation`, `mood`, `checkpoint_at`, `source_event_id`, and `shared_scene_json`), and persona timestamp together. It does not publish a Moments activity or proactive message.

Primary integration points are the existing `resolvedStateFor()` / `stateShape()` projection (`server.js:578-645`), context composition (`server.js:1877-1995`), `streamPersonaChat()` (`server.js:2766-2880`), persona detail route (`server.js:4340-4347`), and detail renderer (`src/companion-main.js:601-655`). The implementation should update these owners rather than add parallel scene or chat paths.

`resolvedStateFor()` checks the shared-scene projection before the existing scheduled state projection. When present, it returns `resolved_source = 'shared_scene'`, scene/location/room/situation/mood from the projection, and an unknown end boundary unless the model explicitly supplied one. When absent, existing event/schedule/routine resolution remains unchanged. `contextFor()` includes the shared scene and the selected image generation policy in the descriptive life-state/system-capability context. Media envelopes use the same resolved state, so a queued image freezes the scene that was authoritative when the tool/marker was emitted.

## `scene_event` Tool Contract

Add two OpenAI-compatible tool definitions to the chat completion request:

```json
{
  "type": "function",
  "function": {
    "name": "scene_event",
    "description": "Persist a material shared-scene start, switch, or end after the conversation supports it.",
    "parameters": {
      "type": "object",
      "additionalProperties": false,
      "required": ["operation"],
      "properties": {
        "operation": {"type": "string", "enum": ["start", "switch", "end"]},
        "location": {"type": "string", "maxLength": 160},
        "room": {"type": "string", "maxLength": 120},
        "activity": {"type": "string", "maxLength": 160},
        "situation": {"type": "string", "maxLength": 240},
        "mood": {"type": "string", "maxLength": 80},
        "objects": {"type": "array", "items": {"type": "string", "maxLength": 80}, "maxItems": 12},
        "participants": {"type": "array", "items": {"type": "string", "enum": ["user", "persona"]}, "maxItems": 2}
      }
    }
  }
}
```

The model-facing system prompt explains:

- Use parenthesized natural language for optional, momentary actions; do not call a tool for every gesture.
- Call `scene_event` only when the shared location/activity actually starts, changes, or ends.
- For a material change, discuss it naturally first and wait for the user's contextual response; the model, not the server, decides whether that response supports a transition.
- A vague response should be handled naturally without a tool call until the model has enough context.
- The selected image-generation policy is a behavioral preference, not a server-side text trigger.

`media_event` is the native media counterpart to `scene_event`. It accepts `kind`, `request`, `count`, and a complete `personaMediaConcept`; the server supplies authoritative current event/appearance fields, validates the same `MediaCapabilityCallV2`, and calls `createChatMediaRequest()` for each requested asset. The existing `<media-intent>` marker remains a compatibility fallback for providers that do not support tools. If both a native call and a marker appear in one turn, the native call wins and the marker is not duplicated.

The server validates only transport shape, enum/length bounds, operation-specific required fields, participant values, and persona ownership. It does not validate acceptance semantics by searching the user message. `end` needs no scene fields. `start` and `switch` require a non-empty location or activity plus a situation; `switch` records the prior projection. No extra frequency, cooldown, budget, or semantic safety policy is introduced by this task.

## Streaming And Tool Continuation

Extend `lmCompletion()` to accept `tools` and `tool_choice: 'auto'`. The existing upstream SSE decoder continues forwarding only text deltas as the application's `token` events. It additionally accumulates streamed `tool_calls` by index, concatenating function argument fragments for `scene_event` and `media_event`.

At completion:

1. Persist the visible assistant text exactly as today, after removing only the existing application capability markers.
2. Parse at most one `scene_event` call. If malformed or invalid, keep the visible reply and report a bounded non-fatal tool error in the completion result; do not infer a transition.
3. If the model returned visible text and a valid tool call, apply the event before emitting `done`, so the next bootstrap/context sees the new scene.
4. If the provider returns a tool call without a final visible reply, append the assistant tool-call message and a deterministic tool result to a continuation message list, make one normal follow-up completion, and stream its text through the same SSE `token`/`done` path. This is a standard function-call continuation, not a second hidden semantic extraction pass. If continuation fails, retain the persisted scene event and return a concise fallback assistant message.

Do not expose tool-call JSON or tool errors as user-visible message text. Keep existing SSE event names (`token`, `done`, `error`) and ordered `done.messages`. Existing `<media-intent>` and `<pending-event>` marker parsing remains unchanged and remains the only path for those capabilities.

## Image Generation Policy

Pass the policy label and meaning into the final system-capability layer. The model remains responsible for deciding whether a visual moment warrants a media marker:

- `ask`: ask naturally before emitting a media marker;
- `always`: a visual capture decision may emit the marker directly;
- `important`: emit for moments the model considers important;
- `user_only`: do not emit a proactive marker; respond to an explicit user request through the existing marker contract;
- `autonomous`: decide naturally.

The server does not match “拍照” or other words to enforce these modes. It only validates a model-emitted media capability call and queues the existing durable image job. The first release uses image jobs only for this feature; existing video/debug functionality remains unchanged.

## Frontend

- Keep the existing message renderer and escape/render user and assistant text uniformly; no bracket parser is added. Parenthesized text remains visible as part of the message.
- Add a normal `<select>` labelled “人格生图频率” to the existing persona detail sheet. It loads the detail value, saves through the dedicated route, and does not appear in contact rows/bootstrap summaries. New personas use the server default `autonomous`.
- Do not add a current-scene panel, confirmation buttons, or quick-reply controls. The chat header continues to show the existing persona situation summary only.
- After a successful scene tool call, refresh bootstrap/detail state when the current view is open; the next chat context is authoritative even if no visible scene UI changes.

## Failure And Compatibility

- Migration failure stops startup as with other companion migrations; no legacy state is read or rewritten.
- Invalid tool JSON never changes state and never turns a valid visible chat completion into an SSE error.
- A failed tool continuation does not roll back an already committed scene event; the event is a durable fact and the user still receives a fallback response.
- Existing chat messages, media jobs, activity events, private memories, and SSE consumers remain compatible. `generation_policy` and `shared_scene_json` have defaults for existing rows.
- Persona deletion removes shared-scene events through the existing persona-scoped life-event cleanup and removes the new state column with the persona state row.

## Verification Strategy

Backend tests should cover migration defaults, policy detail/update, valid/invalid `scene_event` calls for all operations, persistence and precedence of the shared scene, malformed tool calls, tool continuation, and preservation of existing SSE/media behavior. Frontend checks should cover the detail select and the fact that parenthesized text is rendered unchanged with no scene panel or quick-reply controls. Run syntax/tests in a temporary database and manually exercise one streamed tool-call chat against the configured local model.
