---
{"id":"legacy-07e9eea9eac442a10f9d34f1","title":"index.md","kind":"spec","status":"current","paths":["apps/core-go","packages/core-client","packages/browser-client","infra"],"source":".trellis/spec/backend/index.md#sha256=e949ac8e9b929bc1ee8f140a9e0cf826aa1b3c6e547e4874a34661f68c6fddb6","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# Backend Guidelines

The active backend is the Go Core plus its in-process browser boundary under
`apps/core-go/`. The retired Python/Node runtimes and the deleted standalone
gateway are historical context; `apps/core-go/internal/httpapi/browser/` is the
only public browser boundary.
Keep additions inside the clean-start vertical layers and horizontal capability
boundaries described below.

## Active Specifications

The [shared world contract](legacy-798b28c9a1eb2ddd90e0fa2d.md) governs independent
world identity/time, shared NPCs with private actor relationships, Schedule
place binding and hourly LLM → mandatory Kev → executable world events.

The [R8 recovery contract](legacy-62b09df75f5f6c7cc221ca74.md) covers persistent venue
categories, shopping feedback, schedule/image ordering, segment attribution and
read-only self-Moment recall.

The [r3 recovery contract](legacy-78bd0f07be62d7ed619b4cd0.md) covers bounded generation
failure, private media no-progress handling, segment admission and browser recovery.

| Guide | Use it for |
| --- | --- |
| [Wardrobe Failure Feedback](legacy-0d14aeae4e47cc42a75edb51.md) | Exact model assignment and pre-model context failure causes, truthful UI recovery guidance |
| [LLM Prompt Composition](legacy-9b89c41b4be449b963dcba95.md) | Required for every model input: one JSON format, task-specific fields, exact deduplication, history authority and prohibition on restoring prompt/response token limits |
| [Owner Schedule / Story Contracts](legacy-f5c764d7d40d27e61da7e095.md) | Durable non-overwriting schedule requests, accepted-promise associations and authoritative read-only story totals |
| [Directory Structure](legacy-5a6a6e37d4358f587a50b0ef.md) | Adding routes, helpers, or runtime assets |
| [Kev Decisions](legacy-45e4a958da0e6f675ccf8a53.md) | Seven selection gates, reliable raw audit, dynamic tool disclosure, pre-effect persona admission and fallback |
| [Independent Goal Planner](legacy-c937f77348000bc693bd71da.md) | Set capacity, native Planner, Actor edits, policy/fact fences, scope-safe events, migration and acceptance |
| [Owner Promise Lifecycle](legacy-48ab1c123aa1d08699e1c771.md) | Original request, Planner binding, independent commitments, deadline/evidence, cancellation and submission recovery |
| [NPC Runtime](legacy-193952f1f161064b47b7a03f.md) | Executed presence, opportunities, independent NPC responses, real encounter effects and recovery |
| [World Events](legacy-41e5709f50ec980f8b77d957.md) | Structured templates, ambient facts, candidate execution, temporary effects/recovery and interrupted routine segments |
| [Studio](legacy-738e45dfddc64d5b25cf4871.md) | Isolated definition jobs, verified references, staged model results, material relations and truthful recovery |
| [Goal/Life Consistency](legacy-942f35b99a9df37a002a5b20.md) | Actor facts, business time, acquisition/use, cumulative summary, repair and diagnostic paging |
| [Quality Guidelines](legacy-fc3636bd1efca411dc0130dd.md) | Safe changes and verification |
| [Tool Availability](legacy-07362c9912e0162f959cf673.md) | All installed Tools, warning/recovery, truthful handled receipts, savepoint rollback and retained authority/effect conditions |
| [Fluctlight API Contract](legacy-c5accd8e2637bd6567b4aaac.md) | Go HTTP/OpenAPI boundary, generated/reference clients, NDJSON streaming, cancellation, errors, health, and framework isolation |
| [Fluctlight Browser Boundary Contract](legacy-fb329dd1f2b4a2bdcebda2c9.md) | Go API HTTP browser boundary, checked browser contract, NDJSON translation, media proxy, errors, and storage isolation |
| [Fluctlight Auth Contract](legacy-c8a6bb354225ca2576f759a1.md) | Single Owner Human setup, Argon2id, opaque sessions, cookie/CSRF transport, service identity, authorization, and recovery |
| [Fluctlight Configuration Contract](legacy-a19b324f3d0e36b3681b9e87.md) | Startup env, PostgreSQL system settings, single-key AEAD, write-only secrets, validation, and redaction |
| [Fluctlight Persistence Contract](legacy-c513863a7ee899bc5e933bad.md) | Clean-start PostgreSQL module ownership, Unit of Work, short transactions, outbox, external intents, and recovery |
| [Fluctlight Cognitive Runtime Contract](legacy-afcb2db4ef2f7d6eaa8662ca.md) | Clean-start semantic ownership, LLM-first perception/appraisal/decision/reflection, forbidden heuristic fallbacks, and anti-drift tests |
| [Structured Turn Contract](legacy-411c55a6831516ad36ec63e9.md) | Provider JSON/tool control, affect/drives state, explicit memory, and chat commit boundaries |
| [Fact-only Reply Contract](legacy-4bc80c68710c2ca549e16e7e.md) | Independent reply model, run schema/config snapshots, draft cache, publication and failure boundaries |
| [Effective Life Contract](legacy-38a4f720010f179755bfc21b.md) | Shared current body, wardrobe, profile habits, virtual activity outcomes, backfill and prompt consumers |
| [Persona Layer Contract](legacy-f5bfb4c746561fb9a78b07c7.md) | Core Persona, evidence-backed Developing Self, Current State ownership, initialization, reflection and anti-drift boundaries |
| [Fluctlight Memory Contract](legacy-880a140250e48fec52748a49.md) | Typed Memory authority, provenance, pgvector/FTS hybrid retrieval, async embeddings, visibility, and prompt budgeting |
| [Fluctlight Life World Contract](legacy-c8dbdb59473e6ec1f5cec243.md) | Local-day versioned Schedule, replan, Event/Context authority, timezone, pending state, and no heuristic routine |
| [Fluctlight Autonomy Contract](legacy-68be5396ba3fecfcf55a2f9f.md) | Goal/Intention lifecycle, typed triggers, pre-authorized actions, budgets, independent Tool authorization and receipts, pause/cancel, and governance |
| [Fluctlight Media Contract](legacy-ca9d998ac0924c2be8e56e6e.md) | S3-compatible private media, MinIO default deployment, API media authorization, checksums, lifecycle, recovery, and backup |
| [Media Capture Plan Contract](legacy-1674c11f45ee41bcfc4d5239.md) | Branch-first v1, lazy scoped subjects, media Kev, separate references and final graph validation |
| [Media Prompt Contract](legacy-287d88efcd22fd08eb3fad44.md) | Typed image/video intent, direct chat requests, and prompt authority |
| [Visual Identity Contract](legacy-753711cbce16f60f93967f16.md) | Durable visual self-creation, formal visual Agent and independent generation/review/finalize Tools, canonical assets, and renderer constraints |
| [Fluctlight Event Contract](legacy-b1389371420bc3c166879175.md) | PostgreSQL outbox/inbox authority, Redis Streams delivery, reclaim, poison handling, retention, replay, and progress |
| [Fluctlight Workflow Contract](legacy-c2aca3c655fcadadf030b601.md) | Runtime-neutral durable execution, domain intent/state separation, stable IDs, long activities, management, history versioning, and single-runtime rule |
| [Fluctlight Provider Contract](legacy-cd194e76215c171ff6edc5c6.md) | Endpoint/model roles, capability preflight, structured/stream/embedding behavior, budgets, provenance, and failure |
| [Fluctlight Provider Queue Contract](legacy-59423a08873346eecfd129f8.md) | Generic/embedding bindings, trigger scenarios, priority/FIFO queues, concurrency settings, lifecycle diagnostics, and recovery |
| [Fluctlight Diagnostics Contract](legacy-5d5463aca143e1265d6b6a59.md) | Built-in logs/prompts/model runs, correlation, typed redaction, retention, live query/export, and isolated failure |

## Pre-Development Checklist

- Identify whether the change affects the state shape, an API contract, a streaming event, or a background worker.
- Read the corresponding route and its frontend consumer before changing a payload.
- Preserve the environment-variable defaults described in [`README.md`](../../../README.md) and [`infra/compose/fluctlight.env.example`](../../../infra/compose/fluctlight.env.example).
- Check both the normal response and the failure path; the HTTP boundary owns bounded error mapping and NDJSON terminal errors.

## Clean-Start Validation Ownership

For the Fluctlight clean-start program, a manifest entry supplies implementation context and proposed T12 coverage; it does not grant T03-T11 acceptance authority. T03-T11 may record minimal implementation checks, but their results are evidence only. T12 alone re-runs and accepts the complete required spec union, capability matrix, cross-module e2e/failure/security/backup/restore/upgrade and legacy-deletion proof.

| Contract/test slice | Owning child |
| --- | --- |
| Platform skeleton, empty→head migrations, generated clients, minimal Compose readiness/ping, Temporal management/reset | T02 |
| Auth/config/Provider and secret/redaction implementation context | T03; final acceptance T12 |
| Identity/personality/inner-state numeric/revision implementation context | T04; final acceptance T12 |
| Cognition/inbox/diagnostics backend implementation context | T05; final acceptance T12 |
| Conversation/chat and streaming cancellation implementation context | T06; final acceptance T12 |
| Memory/Relationship/reflection/pgvector implementation context | T07; final acceptance T12 |
| Life-world/Schedule/Autonomy workflow implementation context | T08; final acceptance T12 |
| Moments/media adapters/MinIO/heartbeat/idempotency implementation context | T09; final acceptance T12 |
| Browser flows and Diagnostics UI implementation context | T10; final acceptance T12 |
| Previous-release migration, backup/restore, active Temporal workflow implementation context | T11; final acceptance T12 |
| Full combined lint/type/test/build, Compose, Required capability/e2e/failure/security/backup/restore/upgrade and deletion proof | T12 only |

Specific split rules:

- Persistence `empty→head`, previous-release migration, and restore behavior are implementation context from T02/T11; T12 owns the final migration/restore acceptance.
- Media adapter/lifecycle is implemented in T09 and object restore is prepared in T11; T12 owns the final media/backup/recovery acceptance.
- Configuration behavior is implemented in T03 and `.env + data` restore is prepared in T11; T12 owns the final security/restore acceptance.
- Diagnostics storage/query and browser/UI are implemented in T05/T10; T12 owns the final Diagnostics correlation/redaction/UI acceptance.
- API/browser-boundary schema generation and stream/cancellation are implemented in T02/T06; T12 owns the final browser/Core aggregate acceptance.
- Workflow history compatibility and active-workflow recovery are prepared by the owning implementation tasks; T12 owns the final workflow regression.

### Excluded From Positive Acceptance

- Future-only and intentionally reserved schema/interface capabilities do not produce positive acceptance cases.
- Placeholder-only features, fake-only adapters, and stubs without a real producer/consumer closure do not produce positive acceptance cases.
- Required lifecycle states named `placeholder`, `pending`, `deferred`, or `failed` remain testable when they belong to a real authoritative producer/consumer flow.

