# Backend Guidelines

The active backend is the Go Core plus its in-process browser boundary under
`apps/core-go/`. The retired Python/Node runtimes and the deleted standalone
gateway are historical context; `apps/core-go/internal/httpapi/browser/` is the
only public browser boundary.
Keep additions inside the clean-start vertical layers and horizontal capability
boundaries described below.

## Active Specifications

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Adding routes, helpers, or runtime assets |
| [Quality Guidelines](./quality-guidelines.md) | Safe changes and verification |
| [Fluctlight API Contract](./fluctlight-api-contract.md) | Go HTTP/OpenAPI boundary, generated/reference clients, NDJSON streaming, cancellation, errors, health, and framework isolation |
| [Fluctlight Browser Boundary Contract](./fluctlight-bff-contract.md) | Go API HTTP browser boundary, checked browser contract, NDJSON translation, media proxy, errors, and storage isolation |
| [Fluctlight Auth Contract](./fluctlight-auth-contract.md) | Single Owner Human setup, Argon2id, opaque sessions, cookie/CSRF transport, service identity, authorization, and recovery |
| [Fluctlight Configuration Contract](./fluctlight-configuration-contract.md) | Startup env, PostgreSQL system settings, single-key AEAD, write-only secrets, validation, and redaction |
| [Fluctlight Persistence Contract](./fluctlight-persistence-contract.md) | Clean-start PostgreSQL module ownership, Unit of Work, short transactions, outbox, external intents, and recovery |
| [Fluctlight Cognitive Runtime Contract](./fluctlight-cognitive-runtime.md) | Clean-start semantic ownership, LLM-first perception/appraisal/decision/reflection, forbidden heuristic fallbacks, and anti-drift tests |
| [Structured Turn Contract](./structured-turn-contract.md) | Provider JSON/tool control, affect/drives state, explicit memory, and chat commit boundaries |
| [Persona Layer Contract](./persona-layer-contract.md) | Core Persona, evidence-backed Developing Self, Current State ownership, initialization, reflection and anti-drift boundaries |
| [Fluctlight Memory Contract](./fluctlight-memory-contract.md) | Typed Memory authority, provenance, pgvector/FTS hybrid retrieval, async embeddings, visibility, and prompt budgeting |
| [Fluctlight Life World Contract](./fluctlight-life-world-contract.md) | Local-day versioned Schedule, replan, Event/Context authority, timezone, pending state, and no heuristic routine |
| [Fluctlight Autonomy Contract](./fluctlight-autonomy-contract.md) | Goal/Intention lifecycle, typed triggers, pre-authorized actions, budgets, independent Tool authorization and receipts, pause/cancel, and governance |
| [Fluctlight Media Contract](./fluctlight-media-contract.md) | S3-compatible private media, MinIO default deployment, API media authorization, checksums, lifecycle, recovery, and backup |
| [Media Prompt Contract](./media-prompt-contract.md) | Typed image/video intent, direct chat requests, and prompt authority |
| [Visual Identity Contract](./visual-identity-contract.md) | Durable visual self-creation, formal visual Agent and independent generation/review/finalize Tools, canonical assets, and renderer constraints |
| [Fluctlight Event Contract](./fluctlight-event-contract.md) | PostgreSQL outbox/inbox authority, Redis Streams delivery, reclaim, poison handling, retention, replay, and progress |
| [Fluctlight Workflow Contract](./fluctlight-workflow-contract.md) | Runtime-neutral durable execution, domain intent/state separation, stable IDs, long activities, management, history versioning, and single-runtime rule |
| [Fluctlight Provider Contract](./fluctlight-provider-contract.md) | Endpoint/model roles, capability preflight, structured/stream/embedding behavior, budgets, provenance, and failure |
| [Fluctlight Provider Queue Contract](./fluctlight-provider-queue-contract.md) | Generic/embedding bindings, trigger scenarios, priority/FIFO queues, concurrency settings, lifecycle diagnostics, and recovery |
| [Fluctlight Diagnostics Contract](./fluctlight-diagnostics-contract.md) | Built-in logs/prompts/model runs, correlation, typed redaction, retention, live query/export, and isolated failure |

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
