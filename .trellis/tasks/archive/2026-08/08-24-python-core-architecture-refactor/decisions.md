# Architecture Decision Ledger

Child tasks must implement these decisions as written. Replacing one requires returning the parent task to planning, recording evidence/trade-offs, updating affected specs, and obtaining user approval.

| ID | Decision |
| --- | --- |
| D001 | Personal local/NAS self-hosting is the 12-18 month deployment priority; Docker Compose is the default. |
| D002 | Clean start: no SQLite/media/job import, compatibility logic, dual read/write, old API/DTO alias, or mixed runtime. |
| D003 | Ordered child tasks are allowed, but product delivery is one complete cutover after full integration; old implementation is frozen then deleted. |
| D004 | Canonical naming is Fluctlight / Fluctlight instance / Fluctlight system; prohibited old terms do not enter new APIs/schema/code. |
| D005 | Node/TypeScript BFF owns browser transport/session/DTO/media proxy; Python owns all domain state, authorization, transactions, workflows and media metadata. |
| D006 | BFF→Core uses generated HTTP/JSON commands/queries and cancellable NDJSON; Redis Streams is not RPC. |
| D007 | Python API and Worker share package/image but run separate processes; default Worker consumes interaction/lifecycle/media logical queues. |
| D008 | Human and Fluctlight are Actors; Participant is Conversation membership; Relationship is Fluctlight-owned and directed to an Actor. |
| D009 | Implement the complete Fluctlight state/cognitive model in `research/fluctlight-domain-model.md`; group chat remains future-only. |
| D010 | Identity has immutable/human-governed/lived fields; personality changes only through slow evidence-backed reflection; all revisions are audited/reversible. |
| D011 | PAD/momentum use -1..1; other normalized state uses 0..1; Python policy owns numeric changes and wall-time evolution. |
| D012 | LLM owns semantic perception/appraisal/candidate decision/reflection; code heuristics/default semantic fallbacks are prohibited. |
| D013 | Interactive cognition is two-stage: invisible assessment/decision, Python state/freeze, separate realization; reflection is asynchronous. |
| D014 | One durable sequenced cognitive writer per Fluctlight; different Fluctlights parallel; reflection uses watermark/CAS; media returns inbox facts. |
| D015 | Python domain modules: actors, fluctlights, inner_state, life_world, relationships, memory, conversations, cognition, moments and media. |
| D016 | Application Unit of Work composes module interfaces in short PostgreSQL transactions; external I/O uses stable intent/outbox and idempotent workflow. |
| D017 | Memory uses PostgreSQL authority + pgvector/FTS hybrid retrieval; embeddings are async/versioned/rebuildable; exact first, benchmark-gated HNSW. |
| D018 | Media uses private S3-compatible storage with MinIO default; Python authorizes/owns lifecycle; Node BFF proxies bytes. |
| D019 | Redis uses one outbox-driven durable event stream and one ephemeral progress stream; PostgreSQL outbox/inbox is recovery authority. |
| D020 | DBOS is rejected and Temporal is the final sole durable workflow runtime. T01B proved the required core topology, Signals/Queries/Updates, history replay, Worker Deployment Versioning, continue-as-new, cancellation and recovery. Celery/custom queues remain excluded. |
| D021 | Python stack: 3.13 + FastAPI/Pydantic v2/Uvicorn; domain is framework-free; OpenAPI generates Core client. |
| D022 | TypeScript/browser stack: Node 24 LTS + Fastify/TypeBox + Vue 3/Vite/Pinia + pnpm; OpenAPI generates browser client; POST NDJSON streaming. |
| D023 | First delivery has one mandatory-auth Owner Human; Python owns account/session/authorization; BFF owns secure cookie/CSRF transport. |
| D024 | Startup config lives in `.env`; runtime settings live in PostgreSQL; one env `FLUCTLIGHT_SETTINGS_KEY` encrypts sensitive settings; no complex key system. |
| D025 | Six explicit Model Roles with capability preflight/version/budget/provenance; generative roles may share one model; no implicit fallback. |
| D026 | Schedule is reflection-generated, full local-day, immutable/versioned and future-only replanned; Context authority is Event > Schedule > explicit pending. |
| D027 | Fluctlight may autonomously execute pre-authorized budgeted actions; Owner governs/pause/cancel without erasing history. |
| D028 | Built-in Owner Diagnostics stores redacted prompts/model runs/logs/turn links with bounded retention; no external telemetry stack in first delivery. |
| D029 | PostgreSQL access: SQLAlchemy 2 + Psycopg 3 + Alembic; one public schema/MetaData/linear graph; Core-first/ORM-selective; real PostgreSQL tests. |
| D030 | Final scope is `research/capability-inventory.md`: rebuild + close-loop all listed items, delete scaffolding, keep future-only unimplemented. |
| D031 | Implementation is strictly serialized by default: one active child and one writing session; other sessions are read-only research/check unless parent-approved non-overlapping worktrees are defined. |
| D032 | Python dependency/runtime tooling uses pinned uv with `.python-version`, `pyproject.toml`, committed `uv.lock`, locked sync and `uv run` commands. |
| D033 | Every child needs a parent-approved child brief, exact manifests/decisions/paths/commands and a no-history handoff dry run before `task.py start`; program outlines alone never authorize implementation. |
| D034 | Target host is a 16 GiB personal NAS; MTPLX, ComfyUI and h3 run on another machine. The Fluctlight stack must optimize low idle footprint and long-term stability rather than provider compute capacity. |
| D035 | Temporal uses one grouped non-HA Server, PostgreSQL default+visibility stores, no Elasticsearch/OpenSearch, Temporal UI/Prometheus off by default, and three application task queues. |
| D036 | T01B measured about 425 MiB complete gate-stack RSS and proved Temporal is viable. Resource-duration gates, 12-hour NAS soak, strict RSS/CPU thresholds and 30-day disk projection are removed as implementation/release blockers; the system only needs normal bounded health/cleanup checks. |
| D037 | On 2026-08-24 the Owner explicitly authorized T03 implementation before T02's pending validation is completed. This is a one-child, one-writer exception: T03 exclusively owns its declared new modules and required shared migration/Core transport/BFF/generated-client/Compose/lock-file changes; T02 must not concurrently modify those paths. T03 still uses the committed T02 platform foundation, records every divergence in its report, and returns to parent planning on a shared-platform or security conflict. |
| D038 | On 2026-08-24 the Owner explicitly authorized T04 implementation to continue while T03 remains unmerged and directed that Docker, Compose, long-running process, and full-stack runtime checks be deferred as T12-owned unresolved evidence. This is a temporary serialized one-writer exception: T04 may consume the current public T03 actor reference, owns only its declared domain modules and implementation evidence, and must carry T03's skipped work and all deferred runtime gates to T12. This exception does not establish T03/T04 PASS, production readiness, or product cutover. |
| D039 | T03-T11 produce implementation evidence and handoff only; they do not own child acceptance, PASS, production readiness, or cutover authorization. T12 exclusively owns the final required validation union, including full-system Compose, capability, cross-module e2e/failure/security/backup/restore and legacy-deletion proof. |
| D040 | T12 positive acceptance covers only Required capabilities: Must Rebuild and closed Incomplete Old capabilities. Future-only, reserved, and placeholder-only capabilities are excluded from positive acceptance; T12 may run only a negative scope guard and must reject any exposed or falsely delivered excluded capability. |
