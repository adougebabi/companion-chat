# Media Prompt Contract

## Scenario: Direct chat media, capture-contract authority, and durable jobs

### 1. Scope / Trigger

- Trigger: a user asks for an image/video, or an assistant explicitly commits to supplying one, while also giving scene or camera instructions.
- This crosses chat SSE, durable SQLite jobs, the prompt refiner, ComfyUI, and the conversation renderer.

### 2. Signatures

- `mediaRequestFromText(text, {assistant?}) -> {kind: 'image' | 'video', prompt, count?} | null`
- `extractMediaIntent(modelText) -> {text, media}`
- `mediaIntentFor(persona, {kind, request, event}) -> MediaIntentV3`
- `createChatMediaRequest(personaId, {kind, prompt}) -> {jobId, message}`
- `POST /api/companion/chat` emits ordered `done.messages`, including any queued media placeholder messages.

### 3. Contracts

- A direct request or a strong first-person commitment (for example, “我待会拍一张，拍完发你”) queues a `chat_image` or `chat_video` job without waiting for a second user message or a model marker.
- Negative wording such as “不要生成图片” produces no job.
- User visual direction has higher authority than active-state/default composition. Parse it into the V3 `locked` capture contract instead of emitting the raw request as an unrestricted prompt prefix.
- Authority for scene facts is ordered: active event/context (current location, outfit, action, posture, state) → compatible user request → persona/AI completion. A user clothing request is accepted only when the active context permits a change (for example, a clothing-store or try-on event); otherwise the current event outfit remains authoritative.
- `locked.capture` owns `view`, `operator`, `device`, `cameraVisibility`, `orientation`, `framing`, and `subjectGaze`; `locked.subjects` owns visible names/count and exclusions; `locked.composition` owns action, required pose/expression, and prohibited compositions. These values never change during refinement.
- The compiler emits sections in this exact order: photography/gear → subject base → face/skin → environment/light → wardrobe/accessories → mood/temperament → color/parameters → negative constraints.
- A capture device is out of frame unless the request explicitly says it must be visible (for example, a mirror shot or “手机入镜”). This is a generic capture rule, not a persona- or role-specific exception.
- The most recent completed media intent is only a continuity fallback. It cannot override current event state or an allowed explicit change, and it is scoped by `persona_id`.
- `count` is bounded to 1–3 and queues separate durable placeholders/jobs; never rely on one ComfyUI job to imply several images.
- The local refinement call may only complete `photographyStyle`, `faceSkinDetail`, `environmentTexture`, `wardrobeAccessories`, `moodAtmosphere`, and `colorToneAndParameters`; it cannot change any locked capture, subject, composition, environment, or identity field.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Explicit image/video request | Queue durable chat media job before model completion. |
| Marker and direct request both exist | Queue only the direct-request jobs; do not duplicate them from the marker. |
| “不要/不用/别生成图片” | Return `null` intent and create no job. |
| Unknown/natural-language detail cannot be structured | Preserve it in `source.userDirection` for inspection; generic defaults fill only missing fields and raw wording is not made a provider-priority clause. |
| Refiner returns invalid JSON or unauthorized fields | Keep deterministic intent and record `deterministic_fallback`. |

### 5. Good / Base / Bad Cases

- Good: “A 和 B 手持手机前置自拍、一起比心、左侧自然光” produces a normalized two-subject self-capture contract, not an external-observer portrait; the same rule applies to any names, identities, and scenes.
- Base: “来张照片” queues one image with the active event scene and persona appearance defaults.
- Bad: replying “待会拍完发你” without a job, or allowing a generic scene/default camera to overwrite an explicit user camera instruction.

### 6. Tests Required

- Assert direct image and video requests queue the right durable job kind, and negative requests queue none.
- Assert an assistant commitment without a marker yields a media intent.
- Assert a multi-subject self-capture includes both names and locks capture view/operator, device visibility, framing, pose, expression, and light.
- Assert provider section order and absence of an unrestricted raw `userDirection` clause.
- Assert photographer-POV requests exclude the photographer and incompatible extra photographers.
- Assert malformed refiner output preserves the deterministic prompt.

### 7. Wrong vs Correct

#### Wrong

```js
const intent = {...defaultIntent, action: request};
// leaves camera relation, device visibility, and subject count semantically ambiguous
```

#### Correct

```js
const intent = {
  ...deterministicIntent,
  locked: {capture, subjects, composition, environment, identity},
  enrichable: emptyEnrichableFields
};
```
