# Persona authoring and media prompt composition

## Goal

Expand persona initialization into a guided character-authoring flow and make image/video generation visually coherent by compiling a structured, safe media intent into a complete prompt before ComfyUI receives it.

## Requirements

- Preserve the adaptive, one-question-at-a-time interview and optional preview/skip activation behavior. Ask only missing high-value questions, then let AI infer compatible everyday detail and mark each inferred field.
- The guided interview must collect a complete character card rather than only a free-form foundation: role core (name, age/age band, occupation/study, social identity, household/family context, initial relationships); personality/core settings (traits, attitude to others, language style, special setting); appearance (user-provided cultural presentation, face/build, skin/aura, hair, ordinary wardrobe, distinguishing non-sensitive features); and interaction rules, including this persona's understanding of the user's identity, desired communication distance, and explicit boundaries. AI may suggest missing everyday details, but they remain optional and must be marked as inferred.
- The guided character card must cover, at minimum:
  - role core: name, age/age band, occupation or student status, social identity, home/family context, initial relationship network;
  - personality core: personality traits, attitudes toward others, language style, boundaries, special setting/constraints;
  - appearance: ethnicity or cultural presentation when user-provided, face/build, skin/aura, hair, everyday clothing, distinguishing non-sensitive traits;
  - interaction rules: the user’s relationship/identity as understood by this persona, desired communication distance, and explicit boundaries.
- The interview may ask AI-suggested follow-up questions only when needed to resolve daily-life plausibility, visual continuity, safety, or interaction boundaries. It must not infer sensitive user facts or irreversible identity claims.
- Persist four explicit prompt authorities:
  1. immutable identity layer — only a user versioned revision can change it;
  2. life/event layer — structured routine, current state, temporary appearance, and event effects;
  3. evolvable relationship layer — persona-private learned preferences and communication adaptation, with audit/rollback;
  4. system capability layer — non-persona behavior such as tool contracts, media safety/composition rules, provider-output schema, and prohibited disclosure. This layer is application-owned and cannot be edited/evolved by a persona or model.
- The application-owned system capability layer must impose a universal user-visible reply form: every individual assistant message contains exactly one complete sentence. A single turn may deliver multiple separately persisted/sent assistant messages when more than one sentence is needed. This rule applies to interactive chat and proactive private messages, but not to internal structured-model calls such as relationship evolution or media-intent refinement.
- Before **every** image/video job reaches ComfyUI—whether originated by a life activity, a private-chat image request, a private-chat video request, or the development inspector—compile a typed media intent from persona appearance, temporary outfit, event/current scene, requested subject, and channel. Add deterministic composition defaults for subject(s), location, action, pose, expression, camera/framing, light, placement, and negative constraints; then optionally ask the local LLM to refine only this bounded schema. Event/context facts override generic defaults, and generic defaults only fill absent details.
- The typed intent must distinguish the acting persona from the people visible in-frame, and must carry camera perspective plus explicit exclusions. For example, if a female persona is photographing her female friend, the intent requires first-person POV from the persona's camera, the friend as the only visible subject, the photographer excluded from frame, and no male photographer. A server-owned JSON-only “media prompt refinement” call may enrich pose/composition/photographic detail, but cannot change these locked narrative facts.
- When the user explicitly asks for an image/video (or the model is explicitly instructed to fulfill such a request), the conversation path must create a durable `chat_image`/`chat_video` job rather than merely replying that it will look for media. The development inspector must show the trigger, input media intent, final provider prompt, workflow-configuration summary, and job state for the selected persona.
- The deterministic compiler is authoritative for identity and safety fields. LLM refinement may fill absent photographic/compositional details but cannot replace immutable appearance, invent a high-risk event, alter active state, create unsupported media mode, or write directly to persistence without server validation.
- Store the immutable input intent and final enriched prompt/redacted summary with the media job so the development inspector can explain a result. Consumer activity/chat APIs continue to expose only media state and asset URLs.
- Testers must be able to permanently delete a persona that was created with incorrect settings. Deletion removes that persona and all persona-private conversation, memories, life data, activities, media-job records, supporting characters, interview-derived data, and dependent rows; it must not affect any other persona. The UI must require explicit confirmation and select a safe remaining persona (or show the empty state) after deletion.
- The chat input must never be cleared, replaced, or lose a user's unfinished draft because of polling, bootstrap refresh, persona-detail refresh, or any other background UI render. Only a successful deliberate send may clear the draft.
- The displayed current state and the context supplied to chat/media must use one authoritative state-resolution rule. A live event-derived state (for example, “in the library”) must override the routine/schedule label (for example, “in class”) until that event expires or is resolved.

## Acceptance Criteria

- [ ] Persona creation asks the structured character-card questions adaptively, preserves user/inferred provenance, supports preview/skip activation, and never requires a large raw-prompt form.
- [ ] The character-card preview and persisted blueprint include explicit interaction rules about the user's identity/relationship and communication boundaries, plus the role/personality/appearance fields listed above, with every inferred field marked.
- [ ] The composed chat context has explicit immutable identity, life/event, relationship, and system-capability layers; automatic evolution cannot modify the first or fourth layer.
- [ ] Interactive and proactive user-visible assistant output inherits the non-editable one-sentence-per-message rule; when one turn has multiple sentences, each is delivered as a separately persisted assistant message without losing ordering, streaming completion, or the existing chat presentation.
- [ ] A shopping/café/social life event produces a media intent with identity baseline, active appearance/outfit, scene, action/pose, expression, camera/framing, and composition/negative constraints before any provider call.
- [ ] Private-chat image requests, private-chat video requests, activity media, and inspector test media all persist the same complete media-intent schema and final compiled prompt before a ComfyUI provider call.
- [ ] A “persona photographs her female friend” event compiles to a photographer POV with the friend as the in-frame subject and explicit exclusions for the photographer and any male photographer; malformed or unavailable prompt refinement falls back to this deterministic result.
- [ ] An unavailable local model leaves deterministic media-intent output/job retry behavior intact; it does not drop the parent event/activity or fabricate a free-form prompt.
- [ ] LLM prompt refinement is JSON/schema constrained and server-validated. Invalid/missing output falls back to the deterministic compiler.
- [ ] The selected persona’s development inspector can trace media intent → enriched prompt → ComfyUI request summary without showing secrets or leaking another persona’s layers.
- [ ] A tester can explicitly confirm deletion of a persona; its detail/chat/activity data are no longer reachable or returned, its dependent jobs/media records are removed, and another persona remains unchanged.
- [ ] A user can type through repeated background refreshes without losing the input draft; sending intentionally still clears only after the request has been accepted for streaming.
- [ ] When an active life event says the persona is in a place that differs from the routine/schedule, the detail view and chat context report the same event-derived location until its expiry.

## Out of Scope

- Image-to-image face reference upload, actual character gacha/portrait generation, face-recognition guarantees, sensitive demographic inference, and end-user free editing of system capability prompts.

## Open Questions

None blocking. The first implementation uses a typed deterministic compiler plus optional local-LLM refinement, rather than an unconstrained “prompt rewrite” call.
