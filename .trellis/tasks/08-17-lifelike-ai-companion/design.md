# Technical Design: Life-like AI Companion

## Scope

Implement a clean-start, local-first life domain for personas. It supplies structured daily life, bounded events, persona-private learning, autonomous activity/direct messages, durable media jobs, and governance records. It does not migrate or read legacy application data.

## Architecture

The service remains one Express process using `better-sqlite3` and the existing SQLite file in WAL mode. `server.js` remains the application entry point. New persistence helpers use table-level queries and transactions; provider calls occur outside SQLite transactions.

`app_state` is reduced to global settings and a storage marker. The new schema is created by an idempotent migration ledger on fresh startup. This deliberately replaces the former one-document model for all newly implemented companion resources.

## Domain Model

| Resource | Responsibility | Key ownership/query fields |
| --- | --- | --- |
| `personas` | Primary companion identity, enabled/screened state, color, timestamps | `id`, `enabled`, `created_at` |
| `persona_foundation_revisions` | Immutable foundation versions and explicit user revision audit | `persona_id`, `version`, `reason`, `created_at` |
| `persona_life_blueprints` | Structured routine, interests, event policy, supporting-cast seed, attention budget, visual baseline | `persona_id`, `blueprint_json` |
| `persona_states` | Current situation, mood, temporary appearance, next evaluation/checkpoint | `persona_id`, `updated_at` |
| `supporting_characters` | Durable, non-selectable people in one persona's world | `persona_id`, `relationship_kind`, `introduced_event_id` |
| `schedule_items` | Routine, explicit chat plan, cancellation/reschedule history | `persona_id`, `starts_at`, `status`, `kind` |
| `life_events` | Explainable state transitions and event resolution | `persona_id`, `occurred_at`, `type`, `causation_id`, `payload_json` |
| `activities` | Immutable persona posts and visibility metadata | `persona_id`, `created_at`, `event_id`, `content`, `media_mode` |
| `activity_comments` / `activity_reactions` | User and bounded supporting-character interaction | `activity_id`, `author_kind`, `created_at` |
| `activity_visibility` | User-specific hidden/restored status | `activity_id`, `hidden_at` |
| `memories` | Persona-private learned user facts, source evidence, deletion/supersession | `persona_id`, `source_type`, `source_id`, `status` |
| `persona_evolutions` | Allowed relationship-layer changes, diff, evidence and rollback record | `persona_id`, `created_at`, `status` |
| `conversations` / `messages` | Paginated direct chat, including proactive messages | `persona_id`, `created_at`, `read_at` |
| `media_assets` / `activity_media` | Stable ComfyUI reference and future image-set/video relationship | `provider`, `media_kind`, `activity_id`, `position` |
| `jobs` | Durable LLM, image, video, evolution, and event work | `status`, `run_after`, `lease_expires_at`, `persona_id`, `payload_json` |

Only a primary persona is selectable for chat. Supporting characters are tied to their owner persona and never receive a profile, direct chat route, or selectable row.

## Persona Context Composition

Each call composes context in fixed authority order:

1. Foundation: immutable initialized identity, core values, background, visual baseline, and exclusions.
2. Life blueprint: routines, interests, permitted event families, social-world seed, and attention policy.
3. Durable current life: current situation, active temporary appearance/mood effects, current schedule/event facts.
4. Persona-private relationship layer: approved memories, communication preferences, shared experiences, and allowed evolution patches from that same persona only.
5. Current user input and recent persona conversation/activity thread.

Automatic evolution may only add/update layer 4. State and event workers only write layers 2-3 through structured fields. Foundation updates require an explicit user revision and create a new immutable version. All automated changes record reason, source IDs, before/after summary, and rollback data.

## Event and Scheduling Flow

```text
timer or refresh recovery
  -> select eligible persona by focus tier
  -> deterministic eligibility evaluation
  -> create life event/state transition transactionally
  -> enqueue narration/activity/media jobs when applicable
  -> job worker leases one job
  -> provider call outside transaction
  -> complete original event/activity/job in a guarded update
```

Eligibility combines default local time, routine, current state, active plans, recent event cooldowns, event policy, screen state, and active/recent user focus. The engine selects an approved event category and permitted state fields. An LLM may narrate an approved event but cannot select arbitrary event categories or persist raw proposals directly.

Routine state can continue while a provider is unavailable. Complex narration, comments, proactive messages, evolution, images, and future video are queued for retry. Mild recoverable negative events are allowed; high-risk, traumatic, illegal, self-harm, or irreversible events are prohibited.

On restart, the engine reconciles elapsed wall time from the last checkpoint. It emits a bounded meaningful summary rather than every skipped routine transition.

## Activity and Interaction Contract

- Activities are immutable posts, stored in a persona-specific Moments feed and read through a global chronological timeline.
- The global timeline remains a viewing surface only. Persona worlds have no cross-persona interactions or shared facts.
- A user can like and comment. Comment content stays in the activity thread but becomes evidence for that author persona's relationship layer. A later direct message is an independent, budgeted decision.
- Supporting-character interactions prioritize event participants, then occasionally relevant established people. They are capped per post and remain non-clickable background context.
- Users may hide and restore a post. Hiding affects visible feeds only, never the authoritative post/event/memory record.
- A screen/mute hides a persona's new feed content and proactive direct messages, while its lightweight life continues. There are no affinity scores or missed-event penalties.
- Feed reads use a stable descending cursor `(created_at, id)`. Activity unread state is a single red-dot watermark, not a count; direct messages use per-persona counts.

## Media and Jobs

An activity supports exactly one of `image_set` or `video`. First release accepts/renders zero or one image only. The normalized media relation reserves ordered image sets and one-video posts for later work.

Creating a visual activity creates its text activity first, then attaches a queued media job. The UI shows a stable skeleton in the original item. A successful job updates that item in place on later refresh; it does not emit a follow-up post/message or unread item. Failure leaves the text activity visible and exposes a retryable terminal job state in the debug inspector.

Job lease algorithm:

1. In a short `BEGIN IMMEDIATE` transaction, select an eligible queued or expired-lease job ordered by `run_after`, priority, and creation time.
2. Conditionally write lease owner, expiry, attempt count, and `leased` status, then commit.
3. Invoke MTPLX or ComfyUI outside the transaction.
4. Conditionally write complete/failed result using job ID plus lease owner. Retried work gets a backoff `run_after`.

## API Boundary

Additive endpoint families, with request validation at the route boundary:

- persona interview sessions, generated blueprint preview, activation, and user foundation revisions
- persona detail, current situation, recent schedule, screen/restore control, memory/evolution governance
- cursor-paginated global and persona activities; comments, likes, hide/restore operations
- direct messages retain existing streaming semantics but gain explicit persona validation, unread management, and proactive-message provenance
- debug lifecycle inspector: read event/state/job rationale; development-only time advance/evaluation controls using the production rules

Do not change existing chat SSE event names (`token`, `done`, `error`). A new polling-compatible feed endpoint is sufficient for first release.

## Failure, Privacy, and Safety

- Never put model prompts or debug traces into activities or ordinary activity APIs.
- Do not leak configured provider API keys through bootstrap/state endpoints.
- Provider failures result in retryable jobs, clear UI terminal states, and debug evidence; they do not block deterministic routine state.
- Job/event visibility and all memory queries are constrained by `persona_id`.
- Fresh-schema validation runs against temporary `DATA_DIR` and `DATABASE_PATH`; no legacy data is read or altered.

## Rollout and Rollback

This is a clean-start schema rollout. Create all new tables idempotently with a migration ledger. Do not delete legacy rows/files. During development, failure to initialize new storage fails clearly rather than falling back to legacy reads. Rollback is deploying the old binary against unchanged legacy state; new companion data is intentionally not backported to it.
