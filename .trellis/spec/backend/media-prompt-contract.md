# Media Prompt Contract

## Scenario: AI-owned media concept, prompt-master template, and durable jobs

### 1. Scope / Trigger

- Trigger: the final application-owned system-capability layer asks a persona to fulfil an image/video request or a definite media-delivery commitment.
- This crosses chat SSE, activity/debug producers, durable SQLite jobs, the AI-persona concept call, the image prompt master, ComfyUI/H3, and the conversation renderer.

### 2. Signatures

- `extractMediaIntent(modelText) -> {text, media: {kind, request?, count?} | null}`
- `mediaConceptEnvelopeFor(persona, {kind, request?, event?, trigger}) -> MediaConceptEnvelopeV1`
- `generatePersonaMediaConcept(envelope) -> PersonaMediaConceptV1`
- `fillMediaPromptTemplate({envelope, concept}) -> MediaPromptTemplateV1`
- `renderMediaPromptTemplate(template) -> string`
- `createChatMediaRequest(personaId, {kind, request?}) -> {jobId, message}`
- `POST /api/companion/chat` emits ordered `done.messages`, including any queued media placeholder messages.

### 3. Contracts

- Runtime media authorization comes only from a valid `<media-intent>` emitted under the final application-owned system-capability layer. `mediaRequestFromText()` is compatibility-only and must not be called by live chat dispatch.
- The same system contract requires a marker both for explicit user requests and definite first-person commitments (for example, “我待会拍一张，拍完发你”). Free prose alone never creates a job. The marker authorizes media delivery; it is not a server instruction to extract visual semantics from its prose.
- For every producer (chat image/video, direct activity, model-driven activity, and debug inspector), the server creates `MediaConceptEnvelopeV1` with requested kind/count, channel/trigger, immutable identity facts, active life/event facts, temporary appearance, and the original request/event instruction. The server may authenticate, attach facts, check keys/types/size/provider/kind, redact, persist, and retry; it must not infer visual content.
- Every provider-bound request then receives a `PersonaMediaConceptV1` from the AI persona. The concept contains bounded JSON fields for scene, action, mood, explicitly visible `humanSubjects`, separate `nonHumanObjects`, and declared capture intent (`selfie`, `external_capture`, `operator_pov`, `first_person`, or `other`) with operator/device/framing intent. The concept is AI-owned even when its source is a direct activity or debug request.
- `imagePromptMasterContract` receives the envelope facts and persona concept, then returns a complete `MediaPromptTemplateV1`, not a partial “refinement” patch. The fixed template order is: capture and camera relationship → explicit human subjects → identity and continuity → scene and action → wardrobe and non-human props → lighting and mood → photography style and color → constraints/exclusions.
- `renderMediaPromptTemplate()` may only concatenate/label the returned fixed template sections in their prescribed order. It cannot translate capture modes, add defaults, count/rename subjects, derive camera geometry, append negatives, or otherwise change semantics.
- Prompt compactness is owned by `imagePromptMasterContract`: it must remove repetition and prioritise essential visual facts. The server must not truncate, summarise, reject solely for length, or otherwise rewrite a structurally valid master template, because that would silently remove model-owned visual facts.
- The master must render the declared capture relationship coherently: a `selfie` concept has a plausible self-capture treatment; an `external_capture` concept describes an external camera/photographer relationship; an `operator_pov` or `first_person` concept describes the declared off-camera point of view. These are image-prompt-master responsibilities, not server branches.
- The master must list the explicit human subjects rather than generate a broad “共 X 人” clause. Clothes, props, animals, reflections, screens, environmental objects, and other `nonHumanObjects` are never semantically counted or reclassified by the server as people.
- Provider-facing prompts are continuous natural-language photography/video descriptions, never `field=value`, internal enum, raw user-direction prefix, or an unstructured direct model answer.
- `count` is bounded to 1–3 and queues separate durable placeholders/jobs; it is asset quantity, never a server-derived in-frame human count.
- Semantic visual-rule code is forbidden in `mediaIntentFor`, `compileMediaPrompt`, replacement helpers, parsers, and job producers. Do not add keyword/regex/default branches that infer or override people, personhood, scene occupancy, selfie/POV/external capture, device visibility, camera geometry, pose, expression, wardrobe, lighting, composition, exclusions, or negative prompt text.
- The most recent completed result may be attached as persona-scoped continuity context for the AI persona/master. It cannot be turned into a server default or override current facts.
- A missing, malformed, or schema-invalid persona concept/master template is retryable. After normal retry exhaustion, fail only the media placeholder/activity media state with a safe diagnostic. Never submit a deterministic/heuristic fallback prompt or claim a media delivery succeeded.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Valid model marker | Queue one to three durable chat jobs after the model completion; their provider submission waits for a valid persona concept and master template. |
| Missing/invalid marker | Queue no job, including for free-text promises. |
| Unsupported envelope kind/provider/count or malformed transport JSON | Reject/mark invalid before provider submission; never generate a semantic replacement. |
| Persona concept is unavailable, malformed, or violates the structural schema | Record the concept-stage failure and retry the durable source media job. |
| Prompt-master result is unavailable, malformed, or violates the fixed-template structural schema | Record the master-stage failure and retry the durable source media job. |
| Concept/master retries are exhausted | Mark only the media message/activity target failed; retain parent text/activity and do not submit a fallback prompt. |

### 5. Good / Base / Bad Cases

- Good: the AI persona declares two human subjects, an in-frame phone as a non-human object, a `selfie` capture relationship, an interaction, and light intent; the prompt master fills the selfie sections coherently without any server-side keyword detection.
- Good: the AI persona declares the friend as the only human subject and the persona as off-camera operator; the master makes the photographer POV explicit without server code deciding who “给谁拍照”.
- Base: a request with no valid model marker queues no job rather than being guessed by server text matching.
- Bad: a helper uses `/自拍|POV|宠物|两人/`, participant defaults, or a “共 X 人” formatter to decide the visual output; a source job submits a handcrafted fallback prompt when either model stage fails.

### 6. Tests Required

- Assert a valid model marker queues the right durable job kind and a commitment without a marker queues none.
- Assert all producer paths persist the same envelope, persona concept, master template, and rendered final prompt before provider submission.
- Assert the fixed template renders its eight sections in order and does not add a generic “共 X 人” clause.
- With model fixtures, assert a selfie concept yields the fixture's coherent selfie template, an external-capture concept yields the fixture's external template, and a photographer-POV concept yields the fixture's declared in-frame/out-of-frame treatment; these tests must not rely on server keyword parsing.
- Assert a clothing/prop/animal/reflection/screen fixture remains in `nonHumanObjects` and is not converted into a human subject by server code.
- Assert source code/regression tests reject semantic regex/default branches in media-boundary helpers and prevent raw request text from becoming a provider-priority prefix.
- Assert malformed/unavailable persona-concept and master-template responses retry, then fail only the media target after exhaustion; assert no provider submission or heuristic fallback prompt occurs.

### 7. Wrong vs Correct

#### Wrong

```js
const requested = mediaRequestFromText(userText);
if (requested) createChatMediaRequest(personaId, requested);
// Server wording heuristics have become media authorization.
```

#### Correct

```js
const envelope = mediaConceptEnvelopeFor(persona, source);
const concept = await generatePersonaMediaConcept(envelope);
const template = await fillMediaPromptTemplate({envelope, concept});
const prompt = renderMediaPromptTemplate(template); // joins fixed slots only
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
