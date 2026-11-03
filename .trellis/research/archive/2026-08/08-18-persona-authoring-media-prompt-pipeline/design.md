# Technical Design: Persona authoring and media prompt composition

## Scope

This child expands the interview/blueprint schema and replaces the server-owned visual-intent compiler with an AI-persona media-concept and image-prompt-master fixed-template pipeline. It preserves SQLite/WAL, persona isolation, immutable foundation revisions, durable jobs, and ComfyUI ownership by the Node service.

## Character Card and Layer Model

Persist an explicit structured character card in the life blueprint (or a versioned companion table if the current JSON boundary becomes too broad):

```text
identityCore      immutable user/revision controlled
personalityCore   immutable user/revision controlled
appearanceCore    immutable user/revision controlled
interactionRules  immutable user/revision controlled
lifePolicy        event/routine controlled
relationshipLayer evolution controlled and audited
systemCapability  application-owned versioned contract
```

Each interview answer and inferred completion stores provenance. “Age” is collected as an age or age band only where appropriate; no sensitive user fact is inferred. Foundation revision remains the only mutation path for immutable fields.

## Context Composition

The server retains one composition owner:

```text
system capability contract
  + immutable identity/appearance/personality
  + structured life/event/current state
  + persona-private relationship patch
  + current conversation or media-concept/template task
```

Automatic evolution receives only the relationship layer. Event workers receive only structured life/current-state fields. The system capability contract defines tool behavior, expected JSON, negative constraints, and non-disclosure rules; it is not exposed in ordinary APIs.

The same application-owned contract defines user-visible reply form: one complete sentence per assistant message. It is appended as a final fixed rule in the common chat context so user/persona data cannot override it. Interactive chat and proactive private-message completions use a shared server-side sentence segmentation/validation helper before persistence. When the completion contains multiple valid sentences, persist and return them in order as multiple assistant messages for one turn; do not apply this helper to internal JSON-only model calls (relationship evolution, AI-persona media-concept generation, and image-prompt-master template filling). The streaming/API and active chat client contract must support a final ordered `messages` collection while retaining a compatibility path for existing single-message consumers during migration.

## AI-Owned Media Concept and Fixed Prompt Template

Visual semantics have two model-owned stages and one server-owned transport boundary. This deliberately replaces the prior deterministic visual-intent compiler.

```text
chat / activity / debug trigger
        |
        v
server-owned MediaConceptEnvelopeV1
  (identity + live-state facts attached; no visual inference)
        |
        v
AI persona produces PersonaMediaConceptV1
  (scene, action, mood, explicit human subjects, non-human objects,
   declared capture relationship)
        |
        v
imagePromptMasterContract produces MediaPromptTemplateV1
  (fills every section of one fixed provider-prompt template)
        |
        v
rule-free template renderer -> provider prompt -> ComfyUI/H3
```

### Server-owned envelope

The server may collect and attach immutable identity, active life/event state, temporary appearance, channel/trigger, requested asset kind/count, and the raw user/event instruction. It validates authentication, supported media kind/provider, JSON structure, key allowlists, bounded size, job ownership, and persistence. It does **not** parse natural-language wording or apply semantic defaults to decide visual content.

The envelope deliberately does not have server-computed equivalents of `visiblePeople`, `requiredCount`, `requestedLandscape`, `requestedSelfie`, `photographing`, `cameraVisibility`, camera geometry, pose, lighting, wardrobe, or negative constraints.

### Persona concept

Every provider-bound media request must have a `PersonaMediaConceptV1` authored by the AI persona. A final chat `<media-intent>` remains media-delivery authorization, but it carries or leads to the persona concept rather than a provider-ready free-text prompt. Direct activity and inspector requests use the same persona-concept call before prompt mastering.

The concept is bounded structured JSON, but its visual content is AI-owned:

- scene, action, mood, and visual narrative;
- explicitly named/labelled `humanSubjects` that may appear in frame;
- separately labelled `nonHumanObjects` for wardrobe, props, animals, reflections, screens, environment objects, and similar entities;
- a declared `capture` relationship: `selfie`, `external_capture`, `operator_pov`, `first_person`, or `other`, with the intended operator/viewer and device-visibility intent;
- optional concise compositional intent.

The schema exists for transport, auditability, and master-template input; it is not a license for the server to derive missing visual facts. The persona receives the attached identity/live-state facts and is responsible for respecting them.

### Prompt master and fixed template

`imagePromptMasterContract` receives the persona concept plus the attached authoritative facts. It must fill every section of `MediaPromptTemplateV1` in the declared order:

1. capture and camera relationship;
2. explicit human subjects;
3. identity and continuity;
4. scene and action;
5. wardrobe and non-human props;
6. lighting and mood;
7. photography style and color;
8. constraints/exclusions.

The master owns how to make those sections visually executable. In particular it must write a physically coherent selfie when the concept declares `selfie`, an off-camera operator/external-photographer treatment when declared, and an explicit POV treatment when declared. It must list explicit human subjects rather than emit a generic “共 X 人” clause, and it must keep the separately provided non-human objects out of the human-subject set.

The server only renders the returned section strings in the fixed order. It must not translate values between capture modes, append negative clauses, count subjects, or otherwise repair model choices. `compileMediaPrompt` / `mediaIntentFor` are removed or replaced by boundary helpers with no semantic visual decision-making.

### Failure and durability

The source chat/activity item and durable media job are created before model/provider work. A missing, malformed, or schema-invalid persona concept or master-template result is a retryable media-job failure. It never falls back to a keyword-derived/default prompt. After the normal retry budget, only the media placeholder/activity media state becomes `failed`; the parent chat/activity remains available and no fabricated media delivery is reported.

Provider calls remain outside transactions. Persist bounded diagnostics for the envelope, persona concept, fixed-template result, rendered final prompt, and failure stage; ordinary consumer APIs continue to expose only media status and asset URLs.

## Observability and Privacy

Store bounded job payload/result metadata: media-concept envelope, persona concept, prompt-master template, rendered final prompt, stage status/error, and redacted workflow summary. These are exposed only through the development debug-context route introduced by the sibling task; ordinary media APIs still return only status and asset URL.

## Migration and Compatibility

Use an ordered new SQLite migration for any new tables/columns/indexes. Existing blueprints receive only schema-safe factual defaults at read time; do not introduce visual-semantic defaults and do not rewrite historical raw foundations, activities, media assets, or legacy application data. Keep image-set/video mutual exclusion.

## Test Persona Deletion

Provide a persona-scoped destructive endpoint backed by a short SQLite transaction. It deletes rows owned by the selected persona in dependency order (messages/conversation, memories/evolutions, activities and their comments/reactions/media links/visibility, jobs, state/blueprint/foundation revisions/supporting characters/schedule/events, then the persona). Delete media assets only when no remaining activity/media link references them. The browser exposes the action only behind an explicit native confirmation, clears a deleted active-persona selection, and reloads the bootstrap/conversation state so it selects another persona or renders its existing empty state. This is a testing cleanup feature, not a reversible screen/archive operation.
