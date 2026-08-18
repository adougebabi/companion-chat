# Life-like AI companion

## Goal

Give each persona a coherent daily life that can change over time, influence conversation and image-generation context, and surface as an interactive activity feed, while keeping the persona's defining identity stable and every automated change understandable to the user.

## Confirmed Facts

- A persona has an editable `basePrompt`, an `initialPrompt`, and an `evolutionHistory`. The server currently invokes an LLM after 10 minutes of conversation idle time and uses recent conversation plus persona-scoped memories to replace `basePrompt`.
- Persona memories are currently private to one persona. The product has no separate user profile memory, event, calendar, schedule, routine, current scene, mood, appearance, activity, notification, or proactive-message model.
- Image and video generation tools accept only a prompt string. They are queued by the browser after chat SSE completes and then submitted to ComfyUI by a server worker.
- There is no user-facing review, diff, rollback, or detailed validation for prompt evolution. Internal persona fields can also be overwritten by the broad persona update endpoint.
- Persisted state is a single JSON document, so any new fields must have defaults and asynchronous updates must re-read and merge current state to prevent lost concurrent writes.
- The existing single-row `app_state.payload` JSON is unsuitable as the authoritative store for long-lived posts, comments, event history, supporting-character relationships, media references, cursor pagination, and concurrent scheduling. Current chat and evolution paths can also save stale whole-state snapshots after external awaits.
- The existing five-second client refresh can support a lightweight polling-based activity update. The existing chat SSE is scoped to an individual chat request and is not a general event stream.

## Product Requirements

- A persona can have a current, time-aware situation that reflects ordinary daily life, for example class, study, dormitory time, meals, commuting, shopping, or social time. The current situation must be explainable by schedule or an explicit event rather than arbitrary prompt text.
- The normal product shows a concise current situation and a small set of near-term arrangements in chat/persona views. It does not expose a full surveillance-style schedule. A separate development-oriented lifecycle inspector remains available during early debugging to inspect the state source, recent events, next evaluation, and trigger rationale.
- An explicit, time-bounded plan accepted or proposed by a persona during its chat with the user may become a scheduled event for that persona. Casual mentions do not create schedule items. Planned events can influence subsequent scene, activity, image context, and conversation, and must record cancellation or rescheduling as explainable state changes.
- Ordinary and exceptional life events can produce bounded, persistent changes such as mood, temporary availability, location category, or appearance traits. Appearance changes that affect generated images must be recorded in structured state and included in generation context.
- The system autonomously creates ordinary and random life events, activity posts, and temporary state changes without per-event user approval. Every action must remain plausible within the persona's initialized life blueprint, have a duration or resolution condition where applicable, and never silently introduce irreversible identity changes.
- A persona's life follows real-world time. While the local service is running, the engine advances it normally; after downtime it reconciles the elapsed interval from a stored checkpoint into the current situation and a small, coherent set of catch-up events/posts rather than simulating every missed interval.
- All first-release persona schedules, event evaluation, and timeline ordering use the application's default local timezone. Persona-specific timezone overrides and cross-timezone scheduling are deferred.
- Each enabled persona has an independent conversational context, life state, event history, activity stream, and relationship cadence. The persona currently being chatted with, or recently engaged by the user, receives the full evaluation and expression budget; other enabled personas receive only low-frequency schedule progression and required recovery reconciliation.
- Persona initialization derives a default attention budget from the life blueprint. Under ordinary circumstances it permits zero to two activity posts and zero to one unsolicited direct message per day; significant events can grant a bounded exception, normal in-chat replies do not consume the proactive-message budget, and rest hours suppress non-urgent direct messages.
- Activity images are event-driven and optional. Visual events may autonomously request an image using the persona's current structured appearance, scene, and mood; ordinary posts remain text-only. User-requested generation uses the same context composition. Image failures must leave the text post/life event intact, and automatic image generation is bounded by a per-persona daily budget and queue availability. The durable media contract supports either an ordered image set or one video asset, never both on the same post; this first release generates and renders at most one optional image.
- Slow or unavailable LLM/media work is represented by persistent, retryable jobs. Deterministic routine progression remains available during provider downtime; complex narration, proactive content, evolution, image generation, and future video generation are queued rather than discarded or replaced with fabricated content.
- All user knowledge is persona-private. The product must not create a global user-profile memory, implicitly share facts between personas, or use one persona's conversation/activity evidence to enrich another persona's model of the user.
- Users can screen/mute an individual persona. Screening must not stop that persona's underlying life progression and must not create relationship penalties, lost-event penalties, affinity reductions, or any other gamified consequence when the user does not reply or consume content.
- Each persona provides a user-facing memory and evolution record. Users can inspect that persona's learned long-term memories with source/time information, delete individual incorrect or unwanted memories, inspect an accessible evolution summary/diff with its reason, and roll back an automatic evolution without exposing raw model/debug traces.
- Relevant situations and events can yield user-visible companion posts in an activity feed. Every persona has an individual Moments-style feed in its profile, while a separate global timeline presents posts from all personas in chronological order. The user can read posts and converse in their context without treating internal debug traces as social content.
- Relevant situations and events may autonomously create both activity posts and unsolicited direct chat messages when appropriate to the persona, the user relationship, and recent conversation. These are first-class product outputs, not debug notifications.
- Persona posts may include lightweight, system-generated interactions from supporting characters such as classmates, roommates, or friends. In the first release, supporting characters cannot be created by users, cannot receive independent detail pages or chats, and cannot be selected as companion personas.
- Persona initialization creates a small stable core cast of supporting characters. Plausible special events, such as school, shopping, clubs, travel, or work, may introduce new supporting characters and evolve the persona's social context over time.
- Long-term memory and prompt evolution continue to work, but evolution must be constrained to protect the persona's original identity and must be reviewable and reversible.
- Persona initialization separates prompt/context into layered records rather than one model-rewritable string: an immutable foundation layer, a life-blueprint/event layer, a persona-private relationship-learning layer, and a short-lived current-state layer. The effective prompt is composed from those layers.
- The first release must use a bounded, deterministic scheduling and event mechanism instead of unconstrained autonomous planning.
- Creating a persona includes an in-product AI-guided interview. It asks only high-value, currently unknown questions about the desired character, then turns the answers into an initial prompt and structured life blueprint. The user is not expected to manually complete a large configuration form.
- The generated blueprint is presented as a concise, editable preview before the persona becomes active. Previewing is optional: users can skip it and activate the persona immediately. AI-inferred details must be visibly distinguishable from facts supplied by the user, including after an immediate activation.
- First launch starts with no prebuilt/demonstration personas, conversations, memories, or activities. The primary empty state begins the AI-guided persona creation interview; optional inspiration templates only seed that interview and do not create data automatically.
- Important life history is retained long term, but normal UI reads it through cursor/time-based paging or archival views. Routine transitions without narrative or state value do not become permanent user-facing history.
- Published activity content is immutable. Neither the user nor automatic evolution may rewrite an existing post's body, supporting-character interactions, or attached media after publication.
- Users may hide an unwanted published activity post from their own global timeline and persona Moments view, with a non-destructive restore path. Hiding does not delete or alter the activity, its event, its relationship evidence, or its media record.

## Autonomy Decision

- **Decision:** The companion operates fully automatically within a structured life blueprint prepared when the persona is initialized. It does not require user approval for ordinary behavior, incidental events, activity posts, or temporary mood/appearance effects.
- **Rationale:** The user wants the companion to feel like a person with an independent life rather than an assistant awaiting confirmation for each action.
- **Implementation consequence:** The blueprint, event policy, and server-side validators define the permitted behavior. The user retains post-hoc visibility and the ability to adjust the persona configuration, but is not placed in an event-by-event approval loop.

## Persona Initialization Decision

- **Decision:** Use an AI-guided, question-and-answer persona initialization flow. It gathers foundational character facts through adaptive questions, then lets AI fill in compatible everyday-life details and convert the result into the structured life blueprint.
- **Rationale:** It removes the burden of understanding the internal state model while collecting the information necessary to keep autonomous behavior coherent.
- **Boundary:** This is an application feature, not a Codex or developer-facing skill. The resulting blueprint is a product data record that can be displayed and edited in the persona experience.
- **Activation decision:** Users may preview and edit the generated prompt/blueprint once before activation, but may also skip the preview and activate immediately. Preview is an optional quality-control step, not a per-event consent mechanism.
- **First-launch decision:** Start from an empty companion list and route directly to creation. Optional archetype templates are prompts for the interview, not pre-created companion records.

## Time Progression Decision

- **Decision:** Use real-world time plus recovery reconciliation. The companion progresses continuously while the local service runs and catches up on elapsed time when it resumes.
- **Rationale:** It preserves a believable life trajectory in a local-first application without pretending that a stopped machine was executing an agent continuously.
- **Catch-up constraint:** A recovery may update durable current state and publish only a bounded, meaningful summary of missed activity; it must not flood the feed with every skipped routine transition.
- **Timezone decision:** Use the default application/device local timezone for every persona in the first release; do not infer or manage separate persona timezones yet.

## Expression Channels Decision

- **Decision:** The companion may autonomously publish an activity and may also proactively send a direct message. Either channel can be triggered by a plausible life event.
- **Rationale:** Both are necessary for the persona to feel like a person sharing a life rather than a passive feed publisher.
- **Follow-up boundary:** Delivery must still be paced by a product-defined attention budget and contextual relevance so autonomous behavior remains welcome rather than disruptive.
- **Initialization decision:** The initial attention budget is generated alongside the life blueprint from the persona's social style, routine, and expected relationship cadence. It is adaptive rather than a user-maintained daily quota.

## Event-Driven Image Decision

- **Decision:** Generate an activity image only when an event has meaningful visual value, not for every post.
- **Context contract:** A generation prompt derives from immutable visual baseline plus active appearance changes, event scene, and current mood. It must be the same contract for automatic activity images and images requested during chat.
- **Resilience constraint:** Textual activity/event records succeed independently of ComfyUI image generation and remain visible when generation is delayed or fails.
- **Media rollout decision:** The model reserves mutually exclusive `image_set` and `video` post modes. First-release activity output is limited to one optional image; multi-image galleries and video publishing are deferred rather than excluded from the data model.
- **Queue/placeholder decision:** Image, video, and other slow AI work use durable retryable jobs. Chat and activity UI reserve stable media dimensions and show an explicit skeleton/loading state while a result is pending, so textual events remain available without layout shifts.
- **Completion decision:** A completed media job updates the original activity/chat record in place. On the next refresh, the placeholder resolves to the finished asset without creating a follow-up post/message or a new unread item.

## Event Decision Engine Decision

- **Decision:** A deterministic server-side event engine selects eligible event categories and permitted state transitions before calling an LLM. The LLM narrates approved events in persona voice and proposes bounded display content; it does not freely select or persist event types.
- **Eligibility inputs:** Real-world time, life blueprint/routine, current situation, current state, recent event history, cooldowns, user-focus tier, and persona constraints.
- **Rationale:** This combines believable variety with explainable temporal behavior, resource control, and protection against implausible identity or schedule changes.

## Long-Term Retention Decision

- **Decision:** Retain meaningful posts, relationship events, comments, media associations, and evolution history long term. Layer the presentation through chronological/cursor pagination and archive views rather than loading an unbounded history into the main UI or prompt context.
- **Exclusion:** Fine-grained routine transitions are not automatically permanent feed records unless they have narrative value or a durable state effect.
- **Content integrity decision:** Published activity posts are immutable records. Corrections happen through subsequent events/posts or explicit visibility controls, never by silently replacing the original content.
- **Visibility decision:** Users can non-destructively hide and later restore an immutable post; they cannot delete it from the authoritative life/event history.

## Persistence Architecture Decision

- **Decision:** Evolve persistence before implementing the life domain. Keep the current local deployment shape: one Node service, `better-sqlite3`, WAL, and the existing `companion.sqlite` file. Replace the single JSON document as the authority for high-growth, independently queried resources with versioned, normalized tables in that same database.
- **Required boundaries:** `app_state` becomes low-frequency global settings/compatibility state; personas and revisions, persona-private memories, conversations/messages, activity posts/comments, supporting characters, media assets/references, durable event history, and scheduled/leased jobs receive table-level storage with indexed ownership and time/cursor query fields.
- **Correctness requirement:** Event/job claims and updates are table-scoped transactions. Provider calls remain outside transactions. No external await may be followed by a stale whole-state overwrite.
- **Availability requirement:** Jobs carry retry/state metadata so provider downtime can defer model narration/media work without losing it. User-facing event text and deterministic routine changes are not blocked by a slow image/video provider.
- **Compatibility requirement:** Existing API response shapes can be preserved where useful, but the first-release life domain starts from the new table model and does not automatically migrate or read existing local personas, conversations, memories, generation jobs, or media references. Existing state/database files must not be deleted as part of the upgrade.

## Conversation-to-Plan Decision

- **Decision:** Allow a chat to create a persona schedule item only after an explicit, concrete, time-bounded plan is accepted or proposed by that persona.
- **Guardrail:** Casual discussion, wishful language, or ambiguous date references do not persist a plan. Cancellation and rescheduling are explicit event transitions rather than silent prompt changes.
- **Rationale:** This enables shared-life continuity without turning ordinary conversation into an unreliable calendar parser.

## Situation Visibility Decision

- **Decision:** Surface concise, explainable current situation and limited near-term availability in ordinary persona/chat UI. Keep detailed lifecycle internals in a distinct debug inspector.
- **Debug requirement:** The inspector must make it possible to trace the schedule/event source of the current state, recent transitions, planned/recovery evaluation, and decision rationale during early development.
- **Simulation decision:** The inspector includes clearly development-only controls to advance simulated time, run an event evaluation, and simulate only an event category that passes the same production eligibility rules. Simulated records are explicitly marked and can be excluded from ordinary user-facing activity.

## Adverse Event Safety Decision

- **Decision:** Permit only mild, everyday, recoverable adverse events. Each must have bounded state effects, a plausible resolution path, and cooldown protection against repetitive misfortune.
- **Allowed examples:** Academic pressure, a teacher's mild criticism, a small social misunderstanding, a minor cold, a cancelled plan, or an ordinary disappointment.
- **Prohibited examples:** Severe illness, accidents, self-harm, serious financial loss, family trauma, severe bullying, illegal-risk situations, or other high-stakes/irreversible harm.
- **Rationale:** A believable life includes setbacks, but a random companion engine must not manufacture trauma or situations requiring real-world intervention.

## Visual Identity Rollout Decision

- **First-release decision:** Maintain visual continuity through the persona's structured text appearance baseline and current appearance-state prompt composition only. Reference-image upload and image-to-image identity conditioning are not first-release dependencies.
- **Forward-compatible reservation:** Persona initialization and persistence reserve a visual-profile/reference-artifact slot for a later character-card/gacha-style portrait flow. That future artifact may become the basis for image-to-image face consistency, but this task does not implement or promise it.

## Multi-Persona Resource Decision

- **Decision:** All enabled personas remain alive independently, but resources are tiered by user focus. The active or recently engaged persona receives priority for detailed event processing and direct expression; inactive personas retain only a lightweight continuous life simulation.
- **Rationale:** It preserves distinct, believable lives without wasting model or generation capacity on personas the user is not currently engaging.
- **Context isolation:** Conversation history, life state, events, posts, and relationship behavior are isolated by persona. Cross-persona context is not implicitly shared.
- **World boundary:** First-release personas are separate worlds. They may be viewed together in the user-facing global timeline, but do not meet, comment on each other, share supporting characters, exchange events, or access each other's context.

## Persona Screening Decision

- **Decision:** Provide a persona-level screen/mute control for the first release.
- **Non-punitive rule:** Screening is a user experience preference only. The screened persona continues a lightweight life simulation, and missing a message, post, event, or response has no adverse relationship effect.
- **Explicit exclusion:** The first release has no affinity/relationship score and no cultivation-game mechanics that penalize absence or missed events.
- **Visibility decision:** Screening hides the persona's new content from the global timeline and blocks unsolicited direct messages. The user must explicitly remove the screen to receive new content again; historical records and the persona detail view remain non-destructively available.

## User Knowledge Isolation Decision

- **Decision:** No user facts are shared across personas, including basic profile data. Each persona learns who the user is only through its own direct conversations and activity interactions.
- **Rationale:** Separate relationship histories are a core part of making each persona feel like an independent person rather than a differently worded interface over one shared profile.
- **Consequence:** Memory records need persona ownership and source evidence. APIs and prompt assembly must not introduce a global-memory fallback.

## Layered Persona Evolution Decision

- **Decision:** Initialize personas with layered, composable context. The foundation layer defines the base identity and cannot be modified by automatic evolution; autonomous learning writes only to allowed relationship/behavior layers, while life and event changes write to their own structured layers.
- **Rationale:** The companion can become more familiar and expressive over time without drifting into a different person.
- **Audit requirement:** Every automated layer change records a reason, source conversation/event evidence, before/after diff, and rollback point. Prompt composition must make each layer's authority explicit.
- **Manual correction decision:** The user may change the foundation layer only through an explicit, versioned identity revision. The product retains the prior foundation version, offers rollback, and previews likely effects on dependent life/relationship layers; it is not an untracked in-place edit.

## Memory Governance Decision

- **Decision:** Expose persona-private memory and evolution governance to the user. Long-term memories are individually inspectable and deletable; evolution records are reviewable and reversible through concise summaries, evidence links, and diffs.
- **Boundary:** Product UI does not expose raw prompt text, full debug logs, or cross-persona data as part of ordinary memory review.

## Activity Feed Structure Decision

- **Decision:** Provide both a persona-owned Moments feed and a global chronological timeline that aggregates posts from all personas.
- **Rationale:** The individual feed preserves the feeling of visiting one person's life, while the timeline lets users naturally discover what all their companions have been doing.
- **Interaction boundary:** Each post retains its author persona identity. Reading, commenting, or entering chat from a post must attach its context only to the author persona, even when the post was opened from the global timeline.
- **Comment continuity decision:** User comments and persona replies remain visibly in their activity thread, but become persona-private relationship evidence. The persona may later carry a topic into direct chat only when contextually appropriate and within its proactive-message budget; comments do not mechanically mirror into the chat transcript.

## Supporting Characters Decision

- **Decision:** Include AI-generated supporting-character mentions, reactions, and comments in persona activity feeds in the first release.
- **First-release boundary:** Supporting characters are background social context only. Users cannot add them, open their profiles, start chats with them, or treat them as selectable personas.
- **Rationale:** This makes a persona's school, dormitory, family, and friendship events feel socially grounded without expanding the scope into a multi-character companion platform.
- **Lifecycle decision:** Initialize a small, stable core cast with the persona. Special events may introduce new people, so the cast grows naturally and relationships remain durable rather than being retired to meet a global cast-size cap.
- **Post participation rule:** When rendering a post, prioritize supporting characters who participated in or are directly relevant to the post's event. Only then may unrelated established characters be selected as occasional social reactions.
- **Comment constraint:** System-generated supporting-character comments have a per-post hard limit. This bounds content, prompt context, and visual density without deleting relationship history.

## Initial Guardrails

- Do not infer sensitive user facts or relationship claims as long-term memory without an explicit product decision and evidence policy.
- Keep user-facing activity data separate from raw model prompts, conversation traces, and debug output.
- Do not let model output directly write schedules, reminders, or permanent appearance/personality changes without server validation against the initialized life blueprint.
- Initialize the new life domain from clean table-backed data. Do not migrate or read legacy state, conversations, memories, jobs, or media; do not delete the legacy files as part of the release.

## Acceptance Criteria

- [ ] A persona's current situation is derived from recorded schedule/event data and is injected consistently into chat and image/video generation context.
- [ ] A scheduled or random event can affect only its allowed state fields, has clear timing/resolution behavior, and creates a readable user-visible activity when appropriate.
- [ ] An appearance change remains available to image-generation context for its intended duration and can be inspected in the persona's current state.
- [ ] The activity feed persists posts, does not expose raw debug/prompt data, supports unread/read state or an equivalent clear consumption model, and provides a defined path to chat about a post.
- [ ] Prompt evolution cannot overwrite immutable persona identity fields; users can inspect the reason and history of each accepted evolution and restore a previous version.
- [ ] A fresh database initializes all new companion tables deterministically, and concurrent asynchronous writes do not discard unrelated rows.

## Out of Scope

- External calendar synchronization, real-world location tracking, multi-user social feeds, external push notifications, and open-ended autonomous scheduling.
- Medical, financial, legal, or high-stakes relationship advice workflows.

## Implementation Status

Implementation now uses clean, normalized companion storage, an interview-led persona creation flow, deterministic routine/event state, persona-private activity interactions, durable job records, and the Telegram-inspired client. The current slice also includes blueprint-bound focused life variation, unexpired-lease proactive-message/evolution jobs, auditable plan rescheduling, generation result polling, and privacy-safe detail projections. The automated API suite covers clean startup, isolation, screening, focus/budget/lease policy, rescheduling, temporary appearance, media settlement, and foundation revisions. Real configured-provider and viewport/manual-browser verification remain required before task archival.
