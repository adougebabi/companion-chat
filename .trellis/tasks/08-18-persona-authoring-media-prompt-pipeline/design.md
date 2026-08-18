# Technical Design: Persona authoring and media prompt composition

## Scope

This child expands the interview/blueprint schema and adds a server-owned typed media-intent compiler. It preserves SQLite/WAL, persona isolation, immutable foundation revisions, durable jobs, and ComfyUI ownership by the Node service.

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
  + current conversation or media intent
```

Automatic evolution receives only the relationship layer. Event workers receive only structured life/current-state fields. The system capability contract defines tool behavior, expected JSON, negative constraints, and non-disclosure rules; it is not exposed in ordinary APIs.

The same application-owned contract defines user-visible reply form: one complete sentence per assistant message. It is appended as a final fixed rule in the common chat context so user/persona data cannot override it. Interactive chat and proactive private-message completions use a shared server-side sentence segmentation/validation helper before persistence. When the completion contains multiple valid sentences, persist and return them in order as multiple assistant messages for one turn; do not apply this helper to internal JSON-only model calls (relationship evolution and media prompt refinement). The streaming/API and active chat client contract must support a final ordered `messages` collection while retaining a compatibility path for existing single-message consumers during migration.

## Media Intent Compiler

Before a `chat_image`, `chat_video`, or `activity_image` job submits to ComfyUI:

1. Determine channel and immutable visual baseline.
2. Read active temporary appearance, outfit/event details, structured scene, mood, and source request.
3. Create a typed intent with bounded enum/value fields:
   `subject`, `wardrobe`, `scene`, `action`, `pose`, `expression`, `camera`, `framing`, `lighting`, `placement`, `negativeConstraints`, and `mediaKind`.
4. Apply deterministic defaults based on scene/activity. For example, a café defaults to seated-at-table/chair placement unless the event explicitly establishes another pose.
5. Queue optional `media_prompt_refinement` local-model work. It may fill only allowed composition fields in a JSON response.
6. Validate/merge refinement over deterministic intent, compile a provider prompt, and enqueue/continue the existing ComfyUI job.

Provider calls occur outside transactions. The original event/activity/message remains available with its current media skeleton; retries use durable jobs. If prompt refinement is disabled/fails/exhausts retries, the deterministic compiled prompt proceeds (or, for a provider outage, remains retryable according to existing job policy).

## Observability and Privacy

Store bounded job payload/result metadata: input intent, refinement status, final prompt, and redacted workflow summary. These are exposed only through the development debug-context route introduced by the sibling task; ordinary media APIs still return only status and asset URL.

## Migration and Compatibility

Use an ordered new SQLite migration for any new tables/columns/indexes. Existing blueprints receive safe defaults at read/compile time; do not rewrite historical raw foundations, activities, media assets, or legacy application data. Keep image-set/video mutual exclusion.

## Test Persona Deletion

Provide a persona-scoped destructive endpoint backed by a short SQLite transaction. It deletes rows owned by the selected persona in dependency order (messages/conversation, memories/evolutions, activities and their comments/reactions/media links/visibility, jobs, state/blueprint/foundation revisions/supporting characters/schedule/events, then the persona). Delete media assets only when no remaining activity/media link references them. The browser exposes the action only behind an explicit native confirmation, clears a deleted active-persona selection, and reloads the bootstrap/conversation state so it selects another persona or renders its existing empty state. This is a testing cleanup feature, not a reversible screen/archive operation.
