# Visual Identity Contract

## Scenario: Durable self-creation and renderer continuity

### 1. Scope / Trigger

- Trigger: a Fluctlight is initialized, a WakeUp finds no accepted canonical visual identity, or the current Visual Identity attempt advances through image generation, vision review, regeneration, or character-sheet creation.
- Ownership: Go Core owns visual identity state and CAS; Go Worker/Temporal owns orchestration on `lifecycle`; MediaWorkflow remains the only ComfyUI executor on `media`; BFF/Vue expose only safe projections and proxied assets.

### 2. Signatures

- `ensureVisualIdentityInitializationTx(ctx, tx, fluctlightID, triggerType, sourceFactID, corePersona) -> sessionID` creates/reuses the aggregate, session, attempt 1, timeline event and `visual_identity.initialize` intent in the caller transaction.
- `ProcessVisualIdentity(ctx, sessionID) -> {session_id, attempt, status, stage, media_intent_id?, asset_id?}` advances one idempotent checkpoint.
- `VisualIdentityWorkflow(ctx, Input{fluctlight_id, session_id, intent_id})` runs on `lifecycle` and continue-as-news while Core state is pending.
- `ContextProjection.visual_identity` and Fluctlight detail `visual_identity` expose the current safe snapshot and bounded timeline.
- `chestCupToLoRAWeight(cup) -> (weight, adapterVersion, error)` normalizes A/B/C/D and returns a finite `[-10,10]` weight.

### 3. Contracts

- Tables: `fluctlight_visual_identities`, `_revisions`, `_sessions`, `_attempts`, and `_timeline` are Fluctlight-scoped. Sessions have one active (`queued|running`) row per Fluctlight; attempts are unique by `(session_id, attempt_number)`.
- Session triggers are `initialization|wakeup`; initial and WakeUp triggers share the same Core helper. On WakeUp, the model must issue exactly one `visual_identity.initialize` tool call when no active canonical exists; the native executor is restricted to `wake_up_*` source facts and reuses an active session.
- Attempt stages are `seed_requested`, `seed_ready`, `image_requested`, `image_ready`, `vision_requested`, `vision_ready`, `patch_requested`, `patch_ready`, `regenerate`, `accepted`, `character_sheet_requested`, `character_sheet_ready`, and `completed`.
- `visual_identity_patch(stage=seed)` returns a seed prompt. `visual_identity_vision` receives a multimodal image content block and bounded identity snapshot. `visual_identity_patch(stage=review)` returns `decision: accepted|regenerate` and structured patch data.
- Automatic regeneration is bounded to three attempts. `accepted` promotes canonical and queues a separate character-sheet media intent; rejected attempts and assets remain immutable history.
- Renderer constraints preserve `chest_cup`, resolved `chest_lora_weight`, and `adapter_version`. Mapping is explicit code (`A=-5`, `B=-3`, `C=-1`, `D=1` in adapter v1) and must be bumped when tuning changes.
- `media.comfyui.visual_identity_workflow` is an optional structured workflow map. Its `seed`/`character_sheet` variants are selected only from explicit concept fields; the legacy `workflow` remains the Scene Image fallback. `{{prompt}}` injects text, while `{{chest_lora_weight}}` (or `{{renderer_constraints.chest_lora_weight}}`) injects a validated numeric weight when it occupies a whole JSON value; missing weight is an error. Provider/job persistence uses the existing MediaWorkflow.
- Browser detail may include stage summaries, attempt/session IDs, safe asset IDs and proxied image URLs. It must not include provider payloads, credentials, raw prompts, binary data, or private storage locators.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/unsupported cup or adapter output outside `[-10,10]` | Set `renderer_config_pending`; do not create a provider media job. |
| No visual identity row on an old Fluctlight | Read as `missing`; initialization/WakeUp lazily creates the durable rows. |
| Provider role missing, malformed seed/vision/patch JSON, or missing Comfy workflow | Leave the attempt retryable/pending and record a bounded timeline/error; never synthesize a semantic fallback prompt. |
| Duplicate initialization/WakeUp | Reuse the active session and stable intent/workflow/media IDs; no duplicate external submission. |
| Patch decision `regenerate` with attempts remaining | Mark prior attempt `rejected_not_self`, append timeline, create the next attempt, preserve prior asset/vision/patch. |
| Patch decision `regenerate` at attempt 3 | Set session `awaiting_review`; stop automatic generation. |
| Accepted attempt | CAS increment canonical revision, preserve candidate asset, queue character-sheet media intent, then mark profile/session active/completed when ready. |
| Worker restart/provider retry | Re-read Core state, reuse persisted provider job IDs, and continue from the latest stable stage. |
| Scene Image without active canonical | Return explicit `identity_pending`; never infer appearance from role/scene text. |

### 5. Good / Base / Bad Cases

- Good: initialization creates a session, attempt 1 produces an image, vision returns observations, patch returns `regenerate`, attempt 2 is shown beside attempt 1, and an accepted attempt becomes canonical with a character sheet.
- Good: a WakeUp with missing identity emits a concise model-realized notice and queues/reuses the same session in the wake-up transaction.
- Base: no ComfyUI visual workflow is configured; the timeline remains at pending/configuration while the user can add JSON in Media settings.
- Bad: parsing “自拍/两人/胸部” in a prompt, changing canonical state from a rejected attempt, generating a new Provider ID after retry, or returning MinIO/ComfyUI URLs to the browser.

### 6. Tests Required

- Migration tests assert all five tables, indexes, compatibility column and idempotent startup.
- Unit tests cover cup normalization/adapter boundaries, seed/review schemas, explicit workflow selection, visual identity context binding and timeline stage labels.
- Integration tests cover initialization/WakeUp transaction idempotency, missing-identity notice, media job reuse, multimodal vision input, accepted/regenerate loop, three-attempt stop, canonical/character-sheet CAS and restart recovery.
- API/BFF tests assert detail projection authorization and absence of provider secrets/locators; browser tests assert timeline refresh, media event merge, stable image boxes and safe missing/pending states.

### 7. Wrong vs Correct

#### Wrong

```go
if strings.Contains(prompt, "胸") { weight = -3 }
createNewProviderJobOnRetry()
```

#### Correct

```go
weight, version, err := chestCupToLoRAWeight(appearance["chest_cup"])
// Freeze cup + weight + adapter version on the attempt; retry the persisted
// media intent and let the structured patch decide whether to regenerate.
```
