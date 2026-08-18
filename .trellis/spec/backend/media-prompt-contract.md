# Media Prompt Contract

## Scenario: Direct chat media, capture-contract authority, and durable jobs

### 1. Scope / Trigger

- Trigger: the final application-owned system-capability layer asks a persona to fulfil an image/video request or a definite media-delivery commitment.
- This crosses chat SSE, durable SQLite jobs, the prompt refiner, ComfyUI, and the conversation renderer.

### 2. Signatures

- `extractMediaIntent(modelText) -> {text, media: {kind, prompt, count?, creativeDirection?} | null}`
- `mediaIntentFor(persona, {kind, request, event, creativeDirection?}) -> MediaIntentV3`
- `createChatMediaRequest(personaId, {kind, prompt}) -> {jobId, message}`
- `POST /api/companion/chat` emits ordered `done.messages`, including any queued media placeholder messages.

### 3. Contracts

- Runtime media authorization comes only from a valid `<media-intent>` emitted under the final application-owned system-capability layer. `mediaRequestFromText()` is compatibility-only and must not be called by live chat dispatch.
- The same system contract requires a marker both for explicit user requests and definite first-person commitments (for example, “我待会拍一张，拍完发你”). Free prose alone never creates a job.
- `creativeDirection` accepts only photography style, face/skin, environment texture, accessories, mood, and color/parameter details. It cannot replace a live event's location, action, outfit, people, or safety constraints.
- `refineMediaIntent()` is the second-stage AI image prompt master. It receives locked narrative facts, the persona's `creativeDirection`, and the deterministic eight-section draft; it returns only a bounded enhancement patch before `compileMediaPrompt()` produces the final provider prompt.
- A user visual direction may fill only facts compatible with active state; parse it into the V3 `locked` capture contract instead of emitting the raw request as an unrestricted prompt prefix.
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
| Valid model marker | Queue one to three durable chat jobs after the model completion. |
| Missing/invalid marker | Queue no job, including for free-text promises. |
| Marker with unsupported creative fields | Ignore unsupported fields; preserve locked identity/state facts. |
| Unknown/natural-language detail cannot be structured | Preserve it in `source.userDirection` for inspection; generic defaults fill only missing fields and raw wording is not made a provider-priority clause. |
| Refiner returns invalid JSON or unauthorized fields | Keep deterministic intent and record `deterministic_fallback`. |

### 5. Good / Base / Bad Cases

- Good: “A 和 B 手持手机前置自拍、一起比心、左侧自然光” produces a normalized two-subject self-capture contract, not an external-observer portrait; the same rule applies to any names, identities, and scenes.
- Base: a request with no valid model marker queues no job rather than being guessed by server text matching.
- Bad: creating a job from a regular expression over either user or assistant prose, or allowing creativeDirection to overwrite current state.

### 6. Tests Required

- Assert a valid model marker queues the right durable job kind and a commitment without a marker queues none.
- Assert unknown creativeDirection keys are discarded while allowed photography fields persist.
- Assert a multi-subject self-capture includes both names and locks capture view/operator, device visibility, framing, pose, expression, and light.
- Assert provider section order and absence of an unrestricted raw `userDirection` clause.
- Assert photographer-POV requests exclude the photographer and incompatible extra photographers.
- Assert malformed refiner output preserves the deterministic prompt.

### 7. Wrong vs Correct

#### Wrong

```js
const requested = mediaRequestFromText(userText);
if (requested) createChatMediaRequest(personaId, requested);
// Server wording heuristics have become media authorization.
```

#### Correct

```js
const intent = {
  ...deterministicIntent,
  locked: {capture, subjects, composition, environment, identity},
  enrichable: emptyEnrichableFields
};
```

## Scenario: Provider-selected durable media execution

### 1. Scope / Trigger

- Trigger: an image/video job must be sent to a built-in HTTP provider or a server-owned local command provider.
- This crosses persisted settings, `companion_jobs`, the worker lease, media assets, the proxy route, and the Companion settings UI.

### 2. Signatures

- `providerFor(kind, providerId) -> MediaProvider`
- `MediaProvider.submit({kind, prompt, payload, settings}) -> {externalId, pending, files?}`
- `MediaProvider.poll({kind, externalId, settings}) -> {status, files?, error?}`
- Settings: `imageProvider`, `videoProvider`, `h3Defaults`, and the `H3_*` environment defaults.

### 3. Contracts

- A job captures its provider ID at creation. Existing payloads without one resolve to `comfyui` for compatibility; changing the default later never retargets queued jobs.
- The server, not the browser, owns submit, poll, and asset reads. `companion_media_assets.provider` selects the adapter for `/api/companion/media/:mediaId`.
- A local `h3` adapter must call `spawn(executable, args, {shell:false})` with a fixed argument allowlist. It validates the model/output roots, `.mp4` extension, nonempty output, exit code, and timeout before completing the job.
- An h3 job lease is at least the configured process timeout plus a small settle margin. A fixed short lease is invalid for an in-process video command because it can expire before the guarded completion write.
- Public bootstrap exposes provider capabilities and non-sensitive h3 numeric defaults only. It never returns executable, model, output, or allowed-root paths.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown provider or unsupported media kind | Reject settings/request before a job is created. |
| Legacy job lacks provider | Use `comfyui`. |
| h3 timeout, launch error, nonzero exit, absent/empty MP4 | Retry through the existing job policy, then mark the original placeholder failed. |
| Asset provider is unavailable or locator leaves its allowed root | Return a safe provider/asset error; never read an arbitrary local path. |

### 5. Good / Base / Bad Cases

- Good: a queued video records `provider: 'h3'`, runs the configured executable with an argument array, and returns the MP4 through the existing media URL.
- Base: ComfyUI continues to inject `{{prompt}}`, poll history, and proxy `/view` for old and new Comfy jobs.
- Bad: start a long h3 process under the generic 90-second lease, build a shell command from user text, or have the browser fetch a provider directly.

### 6. Tests Required

- Assert capability validation rejects `h3` for image and persisted provider selection reaches a queued job.
- Assert h3 arguments include only configured whitelist values, including numeric `--reuse` and `--ssd-streaming`.
- Assert paths outside the allowed root are rejected, public settings omit local paths, and h3 leases outlast the configured timeout.
- Preserve ComfyUI create/settle/asset compatibility tests.

### 7. Wrong vs Correct

#### Wrong

```js
spawn(`/path/to/h3 -p ${prompt} -o ${output}`, {shell: true});
```

#### Correct

```js
spawn(executable, ['-p', prompt, '-o', output], {shell: false});
```

## Scenario: AI daily plan and live state projection

### 1. Scope / Trigger

- Each persona receives one `daily_plan` durable job per local date; the local model returns 2-6 ordinary, reversible daily items.

### 2. Signatures

- `ensureDailyPlan(personaId, date?) -> {id, personaId, planDate, status}`
- `companion_daily_plans` has unique `(persona_id, plan_date)` and stores the validated JSON plan.

### 3. Contracts

- Planner JSON is only `items[{title, scene, situation, startsAt, endsAt}]`; accepted items become `companion_schedule_items.source = 'ai_daily_plan'`.
- `resolvedStateFor()` is the common read-time projection for UI, chat, and media: active event → active schedule → routine. Do not join a schedule scene with a stale persisted action.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Invalid planner JSON/range | Keep the job retryable; write no schedules. |
| Existing plan for person/date | Reuse it; never enqueue a duplicate job. |
| Persona deletion | Delete daily plans in the same persona transaction. |

### 5. Good / Base / Bad Cases

- Good: an active library plan supplies both library scene and library action to chat/media.
- Base: an unavailable model leaves a retryable planner job, not a static-routine substitute.
- Bad: `scheduledState().scene` plus `stateFor().situation` in one media request.

### 6. Tests Required

- Assert persona creation creates one daily-plan record/job.
- Assert active plan drives both context and media intent scene/action.

### 7. Wrong vs Correct

#### Wrong

```js
const scene = scheduledState(persona).scene;
const action = stateFor(persona.id).situation;
```

#### Correct

```js
const state = resolvedStateFor(persona.id);
const event = {scene: state.resolved_scene, situation: state.situation};
```
