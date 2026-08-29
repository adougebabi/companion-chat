# Ordered Implementation Program

## Status

Review candidate during parent planning. No task below is authorized until final artifact approval and child-specific start. Product cutover occurs only after every task and final integration pass.

## Global Rules

- Read `READ_FIRST.md`; implement numbered decisions without silent substitution.
- New code only under `apps/`, `packages/`, `infra/`, and new-system `tests/` until final legacy deletion.
- Old implementation/tests are evidence, not gates and not maintenance targets.
- T02 keeps platform implementation/readiness checks. T03-T11 may run only minimal implementation checks needed for safe development; those results are evidence only and never child acceptance. Manifest inclusion supplies context; it does not grant T03-T11 acceptance authority.
- Shared files have one integration owner. Generated artifacts change with their authoritative schema.
- Default execution has one `in_progress` child and one writing implementation session. Other sessions may research/review/check read-only. Parallel write work requires a parent-approved non-overlapping worktree split.
- T02-T12 sections are program-level outlines. Before each child is started, the parent must add a child-specific brief with exact decisions/manifests/owned and forbidden paths/implementation-check commands/T12 coverage IDs/report or handoff, then pass a no-history dry run. T01/T01B are the gate templates for this readiness process.

## Child Directory Map

| Step | Trellis child directory | Readiness |
| --- | --- | --- |
| T01 | `archive/2026-08/08-24-dbos-runtime-gate` | Completed gate; formal FAIL evidence |
| T01B | `archive/2026-08/08-24-temporal-runtime-gate` | Completed evaluation; core accepted by D036; old soak/resource FAIL superseded |
| T02 | `08-24-workspace-platform-foundation` | Child-level brief complete; read live child `task.json.status` |
| T03 | `08-24-actors-auth-settings-providers` | Program outline only |
| T04 | `08-24-fluctlight-foundation-inner-state` | Active under D038 exception; implementation evidence only, final acceptance pending T12 |
| T05 | `08-24-cognition-inbox-diagnostics` | Program outline only |
| T06 | `08-24-conversations-chat-experience` | Program outline only |
| T07 | `08-24-memory-relationships-reflection` | Program outline only |
| T08 | `08-24-life-world-schedule-autonomy` | Program outline only |
| T09 | `08-24-moments-media` | Program outline only |
| T10 | `08-24-web-product-control-center` | Program outline only |
| T11 | `08-24-backup-restore-upgrade-operations` | Program outline only |
| T12 | `08-24-integration-cutover-legacy-deletion` | Program outline only |

T01 and T01B are archived evaluations. Parent D020/D036 accept Temporal core and remove the old resource/soak blocker. Read T02/T03/T04 live state from their child `task.json`; T03 remains `in_progress`, and T04 is active under D038 with its deferred runtime gates recorded in the child brief. T05-T12 remain `planning`. Parent/child order is not permission to start without child readiness.

## T01 - DBOS Runtime Gate (Completed FAIL)

Archived evidence: `archive/2026-08/08-24-dbos-runtime-gate` and `research/dbos-runtime-gate-report.md`.

Outcome: resource, queue, durable sleep, cancel, backup and idempotency checks largely passed; missing official pause/restart and failed active-history upgrade replay triggered the pre-approved core FAIL. DBOS is rejected and T02+ remain blocked.

## T01B - Temporal Runtime Gate (Completed, Core Accepted)

Archived evidence: `archive/2026-08/08-24-temporal-runtime-gate` and `research/temporal-runtime-gate-report.md`.

Outcome: grouped topology, three queues, stable IDs, timer/restart, Signals/Queries/Updates, cancellation, Replayer, Worker Deployment Versioning, continue-as-new and low resource use passed. Parent accepts Temporal; incomplete functional checks are reassigned to owning tasks, and resource-duration gates are removed.

Carry-forward: T02 owns live reset/restart/history-point and full management integration; T09 owns long media/provider checkpoint behavior; T11 owns active-workflow restore. No soak/resource-duration gate blocks T02 or release.

## T02 - Workspace And Platform Foundation

Depends: parent D020/D036 Temporal acceptance. Owns root pnpm/Python workspace, `apps/*` skeletons, generated-client pipeline, Compose PostgreSQL/Redis/MinIO/Temporal, config, health/readiness, Alembic, Unit of Work, outbox/inbox/event transport, shared test harness, and live Temporal reset/restart/management integration.

Exit: one-command local stack; OpenAPI drift gates; real PostgreSQL migrations/tests; Redis loss/rebuild; object adapter contract; architecture dependency rules.

## T03 - Actors, Owner Auth, Settings, Providers

Depends: T02. Owns `actors`, Owner setup/auth/session/authorization, settings encryption/UI contract, Provider endpoints/Model Roles/preflight/provenance.

Exit: implementation evidence and handoff record the auth/config/provider implementation, owned paths, unresolved risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T04 - Fluctlight Foundation And Inner State

Depends: T03. Owns `fluctlights` and `inner_state`: identity/personality/behavior policy, revisions/governance, affect/mood/momentum/regulation, drives, Goals/Intentions core state and numeric policies.

Exit: implementation evidence and handoff record domain/property/revision work, canonical range/wall-time assumptions, unresolved T03/runtime risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T05 - Cognition, Inbox, Diagnostics Core

Depends: T04. Owns `cognition`, per-Fluctlight inbox/coordinator, semantic Provider interface, two-stage turn state/freeze, reflection framework, diagnostics persistence/query/retention/redaction.

Exit: implementation evidence and handoff record cognition/diagnostics boundaries, known failure/no-fallback assumptions, ordering/CAS interfaces and T12 coverage IDs; no child acceptance or production-readiness claim.

## T06 - Conversations And Chat Experience

Depends: T05. Owns `conversations`, Participant/Message/read/delivery, Core NDJSON, BFF mapping, generated browser contract, Vue chat/history/composer/media placeholders/error/abort.

Exit: implementation evidence and handoff record Conversation/NDJSON/cancellation interfaces, generated artifacts, unresolved integration risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T07 - Memory, Relationships, Reflection Closure

Depends: T06. Owns `memory`, `relationships`, pgvector/FTS retrieval, embedding workflows, reflection consolidation/revisions/rollback/governance and prompt context.

Exit: implementation evidence and handoff record Memory/Relationship/reflection interfaces, retrieval assumptions, unresolved closure risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T08 - Life World, Schedule, Autonomy

Depends: T07. Owns `life_world`, Context/Event/Schedule/timezone, Goals/Intentions workflows, pending/deferred/proactive action, budgets/quiet-hours/pause/cancel and reconciliation.

Exit: implementation evidence and handoff record Schedule/Context/Autonomy interfaces, pending/deferred state semantics, unresolved workflow risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T09 - Moments And Media

Depends: T08. Owns `moments`, comments/reactions/visibility/unread, `media`, S3/MinIO, image/video/ComfyUI/h3, quality/progress/compensation/tombstone/orphan/Range.

Exit: implementation evidence and handoff record Moments/media/object lifecycle interfaces, provider recovery assumptions, unresolved backup risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T10 - Complete Web Product And Control Center

Depends: T09. Owns full Vue contacts/groups/create/detail/settings/governance/Moments/Diagnostics/Control Center UX, responsive/accessibility and generated browser client integration.

Exit: implementation evidence and handoff record Web/Control Center flows, generated-client artifacts, accessibility scope, unresolved browser risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T11 - Backup, Restore, Upgrade And Operations

Depends: T10. Owns backup/restore manifests, `.env` procedure, PostgreSQL/object verification, migration/Temporal active-history upgrade checks, lifecycle cleanup, operator docs and recovery drills.

Exit: implementation evidence and handoff record backup/restore/upgrade procedures, operator commands, unresolved recovery risks and T12 coverage IDs; no child acceptance or production-readiness claim.

## T12 - Full Integration, Cutover And Legacy Deletion

Depends: T01B and T02-T11. Sole owner of cutover, shared final cleanup and old-code deletion.

Deliver: full capability matrix e2e/failure/security/backup pass; delete old `server/`, old `web/`, old `test/`, SQLite/data/job code, old dependencies/routes/docs/CI; promote new commands/images/docs as sole production implementation.

Exit: repository contains one implementation; no prohibited old terminology/compatibility/scaffolding; one complete Compose deployment passes final acceptance.

## Implementation Evidence And Final Acceptance Ownership

Exact implementation-check commands and T12 coverage IDs are pinned in each child brief. T03-T11 results are evidence only; T12 is the sole final acceptance owner.

| Child | Required local/targeted gate |
| --- | --- |
| T02 | Workspace lint/type/build for touched packages, platform/architecture/contract tests, empty→head PostgreSQL, minimal Compose readiness/ping, Temporal management/reset integration |
| T03 | Implementation evidence only: auth/config/provider code, generated artifacts and local checks; final acceptance moves to T12 |
| T04 | Implementation evidence only: Fluctlight/inner-state code and local checks; final acceptance moves to T12 |
| T05 | Implementation evidence only: cognition/inbox/diagnostics interfaces and local checks; final acceptance moves to T12 |
| T06 | Implementation evidence only: Conversation/NDJSON/browser contract artifacts and local checks; final acceptance moves to T12 |
| T07 | Implementation evidence only: Memory/Relationship/reflection artifacts and local checks; final acceptance moves to T12 |
| T08 | Implementation evidence only: Life-world/Schedule/Autonomy artifacts and local checks; final acceptance moves to T12 |
| T09 | Implementation evidence only: Moments/media adapter/lifecycle artifacts and local checks; final acceptance moves to T12 |
| T10 | Implementation evidence only: Web/Control Center/browser artifacts and local checks; final acceptance moves to T12 |
| T11 | Implementation evidence only: backup/restore/upgrade procedures and local checks; final acceptance moves to T12 |
| T12 | Sole final gate: full lint/type/test/build, complete Compose, Required capability matrix, cross-module e2e/failure/security/backup/restore/upgrade, excluded-scope guard and legacy-deletion proof |

T03-T11 implementation evidence never grants child PASS or product readiness. T12 must rerun the final scenarios it accepts rather than trusting child-local results. Future-only/reserved/placeholder-only capabilities are excluded from positive acceptance.

## T12 Final Acceptance Scope

- `REQUIRED_ACCEPTANCE`: Must Rebuild and Incomplete Old capabilities from `research/capability-inventory.md`, plus their assigned contract/error matrices and cross-module scenarios.
- `REQUIRED_CLEANUP`: Old Scaffolding and obsolete routes/dependencies/docs/CI require deletion proof, not positive behavior tests.
- `EXCLUDED_FUTURE_OR_RESERVED`: Future-only and intentionally reserved capabilities do not produce positive acceptance cases. T12 only checks that they are not exposed, falsely marked delivered, or wired into production traffic.
- `EXCLUDED_PLACEHOLDER_ONLY`: A feature with no real producer/consumer closure is not accepted. A `placeholder`, `pending`, `deferred`, or `failed` state remains testable only when it is part of a Required lifecycle with a real authoritative producer and consumer.
- T12 final evidence must include the complete spec union, Compose readiness, full lint/type/test/build, capability scenarios, cross-module e2e, failure/security/redaction, backup/restore/upgrade, excluded-scope guard, and legacy-deletion manifest.

## Rollback Points

- T01 DBOS failure: completed and archived; T01B Temporal is now the only path to unblock T02.
- T01B original resource/soak FAIL: superseded by D036; Temporal core is accepted and carry-forward checks are assigned to T02/T09/T11.
- T02-T11: child changes remain internal and never cut over product traffic; revert/repair the child without old-system compatibility work. T03-T11 evidence failure returns to implementation/planning and is not a product acceptance result.
- T12 before deletion: abort cutover and keep frozen old system as development reference.
- T12 after final deletion/cutover: clean start has no old-data rollback; restore the new-system backup or redeploy a prior new-system release according to operations docs.
