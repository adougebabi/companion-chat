# Media Capture Plan Contract

## Scenario: Branch-first media capture v1 (2026-10-11)

### 1. Scope / Trigger
`media.image.generate` from Conversation, WakeUp or a direct Tool. Task ownership is separate from photographer and visible subject. VisualIdentity seed/character sheet remains its own exact workflow.

### 2. Signatures
- Tool input `{intent,capture?,media_plan?,visible_subjects?:[{actor_id,visible_scope}],human_count?,reference_asset?:{asset_id,purpose,subject_id?}}`.
- `media_plan` has primary_subject, human_presence, self_in_frame, visible_scope, capture_method, viewpoint, framing, camera_angle. Enumerations are owned by mediaCapturePlanSchema; capture.method includes self_timer.
- `ResolveMediaCapturePlan` uses existing Kev Service and an optional bounded original-model continuation; no separate Agent or Runner.
- Frozen concept `media_plan_version="media.capture.v1"`, media_capture_plan, explicit_constraints, field_sources, input_digest, context_binding, optional subject_contexts and appearance_snapshot_digest.
- Normalizer result `{prompt,declared_human_presence,self_in_frame,capture_method,used_subject_ids,used_reference_ids,declared_human_count}`; statements are internal, not image text.
- Comfy config adds `media_workflows.{none,unknown,partial,other,self,face,back}` and `style_loras:[exact_filename]` / `face_identity_loras:[exact_filename]` / `quality_negative_prompts:[exact_text]`; references use distinct `{{media_reference_image}}` and `{{visual_identity_reference_image}}` channels.

### 3. Contracts
- Image RequiredContext is empty. Prepare determines the branch before reading slots. none/nonself/unknown do not read task-actor appearance or VisualIdentity. self visible scopes project current body/clothes; hands/feet/back exclude full-face identity. face excludes unrelated body and footwear. No CurrentState/CorePersona/history/Goal slot is imported.
- A typed visible_subjects list only uses existing IDs. Additional known Fluctlights must belong to the authenticated Owner; authorization is a boolean query, not a persona read. Each subject has a separate scoped current snapshot. Anonymous people can remain described by the original intent; Human/unknown Actor IDs have no Fluctlight visual authority and are rejected if requested for lookup.
- Source text remains intact. Canonical structural capture fields are locked; raw natural-language capture hints also reach both parsing and normalization. Contradictory explicit fields fail rather than being silently neutralized. `primary_subject=person` plus `human_presence=none` is invalid; animal/mixed plus present remains legal.
- Kev normally uses at most two physical batches (subject, then applicable capture/view with per-question point). A complete reliable plan may skip every question. Unknown or invalid fields use neutral values or at most one bounded original-model parse; do not infer image semantics with word matching. Validate/recheck accepted decisions before plan freeze; revoked additions are discarded without changing raw answers.
- Normalize with the tool-free existing MediaPrompt Agent, isolated allowed data and Chinese output. Extra sources and unknown IDs are rejected. Output declarations are insufficient semantic proof; high-precision known English/Chinese selfie skeletons catch explicit pollution. One normalization repair maximum, including malformed JSON; failure returns typed non-retryable media_prompt_validation_failed before submit. ProcessMediaActivity maps it to the SDK non-retryable error and records terminal failure immediately, so Activity retries cannot reset the repair budget. Ordinary transport retries and parent cancellation remain unchanged.
- Workflow text validation follows actual positive conditioning, rejects prompt additions and fixed input images. Nonself identity adapters are rejected; unclassified LoRAs cannot become hidden self conditions. Full-self body adapters require the typed current-cup placeholder; face identity adapters must be explicitly classified, and face scope cannot receive a body-weight placeholder. Explicitly classified style LoRAs remain. Scene/animal/object references require a ready same-owner source asset whose versioned plan establishes a compatible nonhuman subject; unverified legacy assets fail rather than assuming their purpose. A subject reference needs an allowed visible subject and a verified individual/canonical source; partial/unknown scope cannot use full-person refs. Generic and self-identity upload filenames never share a channel.
- Legacy identity body/cup values never enter normalization as a replacement for current appearance. Unknown negative encoders fail until their quality text is explicitly classified; valid quality negatives remain. Current full-body cup facts alone produce current renderer weights; face/partial/none do not inherit canonical body weights. Per-intent cached prompt is checked against its versioned normalization record. Messages/workflows/snapshots are deep-copied; no cross-Actor result reuse.
- Prepared replay compares all Tool argument data through input_digest. Known subject body snapshots are locked in sorted ID order and checked before the intent mutation. Completed/submitted legacy tasks retain their execution; unsubmitted legacy tasks are rebranched, clear old provider text and persist the new version before submission. Migration readers may inspect old prepared payloads, but a new executor requires v1.
- Use media:<stable intent ID> for preparation/Kev/normalization/submit correlation, retaining parent correlation in diagnostics. No publication or fact mutation occurs in independent probes.

### 4. Validation & Error Matrix
| Condition | Result |
| --- | --- |
| explicit none + self yes / person / selfie | media_explicit_constraint_conflict |
| no resolvable subject | media_subject_unresolved |
| another subject outside Owner scope | media_subject_not_authorized |
| changed prepared plan/reference/subjects | media_prepared_arguments_conflict |
| wrong source/scope/identity | media_context_* or media_subject_*; no submit |
| known prompt pollution after one repair | media_prompt_validation_failed |
| fixed ref, unclassified LoRA, hidden self condition | media_workflow_*; choose compatible configured branch |
| person asset labelled scene / unverified source | media_reference_*; no upload or submit |
| final placeholder or cached text mismatch | media_workflow_placeholder_unresolved / media_prompt_cache_conflict |

### 5. Good/Base/Bad Cases
Good: a cat or coffee image reads zero self visual slots; a hand/cup request reads only projected current local facts. Good: known A/B subjects have separate data and permitted IDs. Base: disabled Kev still routes through the repaired parser/normalizer. Bad: appearance existence means self in frame, body LoRA remains in a neutral workflow, or a product reference fills the identity channel.

### 6. Tests Required
TestMediaBranch* covers M01–M40 contract slices, two Chinese phrasings, sentinel isolation, read counts, current traits, mixed subjects, reference purpose, final graph, replay, rejection/raw audit, two batches, cancellation/revocation and bounded repair. These fixtures prove deterministic wiring, not model semantic accuracy or pixels.
Independent `TestMediaPlanProbe` calls only Kev with FLUCTLIGHT_MEDIA_KEV_URL; `TestMediaNormalizationProbe` requires FLUCTLIGHT_MEDIA_PROBE=1, defaults to stub and supports FLUCTLIGHT_MEDIA_MODEL_MODE=live. Actual Comfy execution additionally requires FLUCTLIGHT_MEDIA_COMFY_EXECUTE=1 and a private compatible config file; no social publication. Real fixture DB is disposable GO_CORE_TEST_DATABASE_URL. User performs real service/pixel acceptance for this task; do not silently run those probes.

### 7. Wrong vs Correct
Wrong: request all person slots, then tell the model to ignore them for a cat.
Correct: determine a neutral cat scope, load no self slots, normalize the original cat description, and inspect the final graph before POST /prompt.

Schema migration `0060_media_kev` widens the closed decision-point CHECK after immutable `0056`; it runs before the single ledger advance under the existing migration transaction/advisory lock. Existing audit/settings remain unchanged. The real 0059→0060/rerun preservation test is opt-in and was not executed in this task.
