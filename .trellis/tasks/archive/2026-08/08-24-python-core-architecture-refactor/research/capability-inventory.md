# Clean-Start Capability Inventory

## Status

Approved final cutover inventory as of 2026-08-24. It classifies product capabilities, not old route/file parity. Must Rebuild and Incomplete Old Capabilities are required before T12 cutover; Old Scaffolding is deleted; Future-only remains unimplemented. Evidence comes from the frozen route contract, Vue entrypoints, prior architecture scans, README, tests, and task history.

## Acceptance Classification

- `REQUIRED_ACCEPTANCE`: Must Rebuild and Incomplete Old Capabilities. T12 creates positive functional, contract, failure, security, backup, restore and cross-module acceptance scenarios for these rows.
- `REQUIRED_CLEANUP`: Old Scaffolding or accidental contracts. T12 proves deletion and absence; it does not preserve or positively test the old behavior.
- `EXCLUDED_FUTURE_OR_RESERVED`: Future-only capabilities and intentionally reserved schema/interface seams. They do not create positive acceptance scenarios. T12 only runs a negative scope guard to prove they are not exposed or falsely marked delivered.
- `EXCLUDED_PLACEHOLDER_ONLY`: A feature with no real producer/consumer closure. It is not accepted merely because a stub, fixture, placeholder page, fake adapter or schema exists.
- A `placeholder`, `pending`, `deferred`, or `failed` state remains a Required acceptance target only when it belongs to a real lifecycle with an authoritative producer and consumer. For example, media placeholder/progress/result/failure is Required; a placeholder-only feature is excluded.

## Must Rebuild Before The One-Time Cutover

| Capability | New-system outcome | Evidence |
| --- | --- | --- |
| Owner setup/auth/session | One mandatory Owner Human setup, login/logout, recovery and authorization | New approved auth scope; old system has no real browser auth (`server/http/app.js:301`) |
| Fluctlight creation | Natural-language analysis/preview/activation plus `llm_defined` and `blank_slate` initialization | Old analyze/activate routes `server/http/route-registry.js:12-19`; browser wizard `web/src/app/App.vue:21-24,128-153` |
| Actor/Fluctlight directory | Contacts, selection, unread/current-state summary, group create/assign/filter, future-ready Actor refs | Bootstrap/group routes `server/http/route-registry.js:8,20,22`; types `web/src/types/domain.ts:41-63` |
| Complete Fluctlight detail | Identity, personality, affect/drives, relationships, context/schedule, goals/intentions, memories, behavioral policy and cognitive history | Old detail aggregate route `server/http/route-registry.js:24`; new source requirement `research/fluctlight-domain-model.md` |
| Identity/personality governance | Read/edit/revision/audit/rollback according to confirmed mutability policy | Old foundation edit/restore `server/http/route-registry.js:25-28`; new governance contract in PRD/design |
| Conversation | Actor participants, ordered messages, cursor history, unread/read state, attachments and stable delivery | Old routes `server/http/route-registry.js:36-38`; old browser types `web/src/types/domain.ts:26-39,112-115` |
| Two-stage chat | Assessment/decision, Python freeze, realization NDJSON, cancellation, multiple visible messages and explicit errors | New cognitive/API/BFF contracts; old stream consumer evidence `web/src/types/domain.ts:149-177` |
| Working/long-term memory | Working, episodic, semantic, relationship and autobiographical memory; correction/forgetting and hybrid retrieval | Old detail/delete route `server/http/route-registry.js:34`; new memory source/spec |
| Relationships | Directed Fluctlight→Actor state, evidence, reflection, revisions and rollback | Old rollback route `server/http/route-registry.js:28`; new Actor/Relationship decision |
| Life world/context | Confirmed Event/Schedule authority, Presence overlay, current state and history | Old detail/lifecycle behavior; README life-state contract; new life-world spec |
| Schedule | Create/commitments, immutable daily versions, dynamic replan, cancel/governance, timezone/DST | Old create/reschedule/cancel `server/http/route-registry.js:31-33`; new life-world spec |
| Goals/intentions/autonomy | Autonomous Goal→Intention→trigger→frozen Action with budgets, pause/resume/cancel/history | New source definition and autonomy spec |
| Proactive/delayed behavior | Pending event, deferred reply, proactive message, quiet-hours/budget and idempotent delivery | Old job evidence in `server/application/pending-event-flow.js:320`, `server/application/deferred-chat-policy.js:299`, `server/application/proactive/worker-flows.js:306` |
| Moments | Global/per-Fluctlight feed, unread marker, comments, like, hide/restore, Actor authorship and media | Routes `server/http/route-registry.js:40-44`; types `web/src/types/domain.ts:117-147` |
| Images | Autonomous/chat/Moment image intent, placeholder/progress/result/failure, quality/retry and object storage | Old media route `server/http/route-registry.js:46`; README media behavior; new media spec |
| Video/h3 | Video generation, h3 configuration/preflight/progress/timeout/cancel/recovery and Range playback | Old preflight `server/http/route-registry.js:49`; settings types `web/src/types/domain.ts:73-100`; README h3 behavior |
| Settings | Provider endpoints/roles/models, encrypted keys, workflows, media/autonomy/cognitive policies and safe summaries | Old settings/models `server/http/route-registry.js:9-10`; new config/provider specs |
| Diagnostics/Control Center | Logs, prompts/model runs, turn/state/action chain, workflow/events/media, retry/cancel/pause, filters/export/retention | Old debug routes `server/http/route-registry.js:48-53`; browser inspector aggregation `web/src/api/personas.ts:234-253`; new diagnostics spec |
| Backup/restore/upgrade | PostgreSQL + object data + `.env`, manifest/integrity, explicit migrations and whole-system restore | New NAS/persistence/media requirements |
| One-command deployment | Pinned Node/Python, BFF/Core/Worker/PostgreSQL/Redis/MinIO Compose, internal networking, health/readiness | Approved topology and framework decisions |

## Incomplete Old Capabilities That Must Become Real Closed Loops

| Capability | Old gap | Required clean-start closure |
| --- | --- | --- |
| Relationship evolution | Handler registered but no production evidence submitter/evaluator in default composition | Interaction evidence → reflection proposal → policy/revision → prompt/behavior consumption → governance |
| Memory consolidation | Candidate ledger does not promote to authoritative Memory | Evidence window → candidate → policy/revision → typed Memory → embedding/retrieval → correction/forgetting |
| Agency | Candidate ledger has little/no production consumer | Goals/Intentions lifecycle → trigger → qualification → frozen Action → workflow/outcome/reflection |
| Personality/self evolution | Existing self-model claims are partial projections | Confirmed identity/personality model, slow reflection revisions, behavior/prompt consumption and rollback |
| Daily Schedule | Recent old plan may remain baseline-only and planner incomplete | Full reflection-generated local-day Schedule, version/replan, timers and Context authority |
| Timeline reconciliation | Old handler exists without a production enqueue path | Fold required reconciliation into life-world/lifecycle workflows with explicit triggers; do not copy dormant handler name |
| Long media tasks | Old 60s lease conflicts with up-to-15-minute h3 and no general heartbeat | Temporal Activity heartbeat/timeout/cancel/recovery and stable Provider request identity |
| Media lifecycle | Old asset references/deletion are incomplete and Provider/local locators are authority-adjacent | Normalized references, S3 object/checksum/version, tombstone/orphan collection, backup/restore |
| Workflow operations | Old inspector mostly read-only and Worker lacks generic cancel/retry/resume | Owner-authorized query/pause/resume/cancel/restart/repair with audit |
| Semantic anti-drift | Existing protections are localized and implementation repeatedly drifted to code heuristics | Project-level LLM semantic ownership contract, static gate and failure tests |

## Old Scaffolding Or Accidental Contracts To Delete, Not Rebuild

| Old surface | Reason |
| --- | --- |
| `/api/companion/*`, `persona`, `companion`, Chinese deprecated product names | Clean-start naming and browser API have no compatibility requirement |
| Legacy SSE `token/done/error` payload and `done.message` alias | Browser/Core streams are independently specified NDJSON; reuse concepts only where intentionally selected |
| `timeline.activity_decision` alias | Historical spelling alias with no product value |
| Generic `createTaskRuntime()` unused in production | Temporal owns durable schedules/workflows after T01B passes |
| Browser-unused append-message helper/old envelope | Old client/server contract drift and no current UI consumer |
| Old activity-comment envelope mismatch | Rebuild one generated contract rather than preserve either accidental shape |
| Old marker/tool compatibility fallbacks | New Provider role/capability contract has no old marker compatibility requirement |
| Old SQLite migrations, repositories, leases, jobs and state aliases | Clean start; no data/job compatibility or dual runtime |
| ComfyUI locator/h3 absolute-path asset authority | S3-compatible media contract replaces it; Providers are generation inputs only |
| Debug env-gated fragmented inspector routes | Replaced by authenticated built-in Diagnostics/Control Center |
| Rules/regex/keyword/default-personality semantic fallbacks | Explicitly prohibited by cognitive runtime contract |
| Per-route/browser DTO normalizers for snake/camel/old shapes | Generated new OpenAPI clients are the only contract |

## Not Implemented Now, But Schema/Interfaces Must Leave The Path Open

- Multiple Human accounts, invites, roles and account-management UI.
- Group chat UI/orchestration/notifications and autonomous Fluctlight-to-Fluctlight sessions.
- Direct browser presigned media transfer and cloud/multi-tenant deployment.
- External irreversible actions on behalf of Human accounts.
- Horizontal high-availability infrastructure and external telemetry stacks.
