# Companion interaction and prompt composition upgrades

## Goal

Turn the next QA feedback batch into a coherent companion-product upgrade: chat should feel natural and work reliably on mobile; debugging should be available without polluting normal product UI; persona creation and media generation should have enough structured information to keep a person visually and behaviorally coherent.

## Confirmed Facts

- The active client is `src/companion-main.js` and `src/companion-style.css`; it is a vanilla browser module with a five-second/15-second refresh pattern and server-owned provider calls.
- The companion domain already has normalized SQLite tables, persona-isolated context, life blueprints, durable jobs, media placeholders, and a development lifecycle inspector.
- Current chat exposes adjacent image/video request controls, renders a generic “正在组织回复...” placeholder, and needs a real mobile interaction pass.
- Persona interview is adaptive but currently collects only a compact foundation/routine/interests/visual-baseline/supporting-cast set. The stored blueprint already reserves evolution, event, visual, and relationship layers.
- Ordinary persona detail APIs intentionally avoid raw foundation prompts; debugging must remain separately gated and must never reveal provider credentials.

## Child Deliverables

| Child task | Ownership | Dependency |
| --- | --- | --- |
| `08-18-chat-mobile-debug-upgrades` | Composer behavior, mobile usability, natural response states, activity comments, and development-only observability | Consumes existing chat/jobs/media contract; may expose new safe debug DTOs. |
| `08-18-persona-authoring-media-prompt-pipeline` | Guided persona character card, prompt-layer model, and media-intent enrichment | Defines the structured character/media contract consumed by chats, life events, and ComfyUI jobs. |

## Cross-Task Requirements

- Normal conversation is the primary way a user requests images or videos. Composer-adjacent manual generation actions are removed from the consumer UI; any retained controls live only in the development inspector.
- Product-facing text avoids robotic status wording. Pending assistant output should be represented as a natural typing state and must remain distinguishable from a persisted message.
- Debug observability is explicit, local, bounded, and development-only. It may show prompt layers, rendered provider request payloads, job state, and media workflows, but never API keys, authorization headers, or data from another persona.
- Persona creation remains a one-question-at-a-time guided flow. The expanded information model must preserve user-provided versus AI-inferred provenance and never auto-mutate the immutable identity layer.
- Media prompt enrichment must be deterministic and schema-bounded before any optional LLM narration/refinement. A provider outage may defer media output but cannot invent unbounded appearance, activity, or identity changes.
- The system supports a stable visual composition contract: identity baseline + active appearance/outfit + scene + approved action/pose + appropriate expression + camera/framing + composition constraints. It must avoid implausible defaults such as a person sitting on a café floor without an event stating so.

## Cross-Task Acceptance Criteria

- [ ] A mobile user can focus, type multi-line text, press Enter to send (with Shift+Enter for newline), and tap send reliably at supported narrow viewports.
- [ ] Normal chat no longer advertises raw image/video generation controls; user-requested or autonomous media still reaches the existing durable media workflow through an explicit intent contract.
- [ ] Debug users can inspect the assembled chat/media prompt layers and job payload summaries without exposing credentials or ordinary UI debug traces.
- [ ] The persona wizard creates an auditable structured character card with immutable, event-driven, evolvable, and system-capability layers.
- [ ] Every automatic media request has a bounded, inspectable media-intent record and a complete/fallback prompt that includes composition safeguards.

## Out of Scope

- Replacing the local Node/SQLite deployment, external image identity conditioning, external calendar sync, user-uploaded reference images, and a general social-network media editor.
- Giving end users a raw provider prompt editor in normal companion views.
- Guaranteeing a particular image-model output; this task governs context quality, constraints, retryability, and inspectability.

## Planning Status

Created on August 18, 2026 after the first implementation QA pass. The parent is planning-only; its children must receive design and implementation artifacts and be explicitly started in a new session before code changes begin.
