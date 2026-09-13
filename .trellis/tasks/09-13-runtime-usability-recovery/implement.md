# Runtime usability recovery — implementation plan

Status: in progress; S01-S10 complete and S11 verification is in progress.

Workspace: /Users/vinson/Documents/project/个人/local-ai-companion

## Task shape

This task is not split into parent/child tasks. Although it has three visible
deliverables, lifecycle diagnostics is a prerequisite for proving WakeUp and
Reflection liveness, and initialization source persistence shares the same
diagnostic, migration, API and Owner-authorization boundaries. Parallel child
completion would create intermediate states that cannot be independently
accepted. Work remains isolated through the ordered stages below.

## Strict execution rules

- S01 through S11 execute strictly in order. Do not enter the next stage until
  the current stage’s code, exact tests and evidence are complete.
- A stage may modify only files named in its Stage file allowlist plus files
  inside .trellis/tasks/09-13-runtime-usability-recovery.
- A likely file is not authorization. If implementation needs another file,
  stop, amend this plan and obtain user approval before editing it.
- S01 through S11 must not run go test ./..., race ./..., repository-wide vet or
  build, Docker Compose, Live Provider, full PostgreSQL/Temporal smoke, or any
  cross-stage acceptance command.
- During S01 through S11, run only exact test names covering symbols created or
  modified in the current stage. Existing failures outside the allowlist are
  recorded in implementation-evidence.md and are not fixed opportunistically.
- S12 begins only after S01 through S11 are complete and checked. S12 alone owns
  all Required Validation Commands.
- The unrelated .trellis/tasks/09-08-project-health-evolution directory is
  never in an allowlist. Do not modify, format, delete, stage or commit it.
- Do not commit or push unless the user explicitly authorizes it.

## Ordered checklist

### S01 — Freeze evidence and red-capable contracts

Stage file allowlist:

- .trellis/tasks/09-13-runtime-usability-recovery/implement.md
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md
- apps/core-go/internal/core/wakeup_intents_test.go
- apps/core-go/internal/core/redis_triggers_test.go
- apps/core-go/internal/core/initialization_contract_test.go
- apps/core-go/internal/workflow/workflow_test.go

Checklist:

- [x] Record branch, HEAD, dirty paths and hashes for current WakeUp,
      Reflection, diagnostics and initialization owners.
- [x] Add exact red-capable regressions for: completed WakeUp with lost Redis
      expiry; user turn must not postpone WakeUp; active Fluctlight without
      Schedule still owns WakeUp; expected WakeUp absence becomes overdue;
      single/multi classification; semantic-empty StructuredFallback rejection;
      explicit dense source semantic coverage.
- [x] Tests must fail for the user’s exact symptoms, not only for a helper’s
      return value.
- [x] Run only the exact new test names and record the expected failures.
- [x] Rollback: remove only the new tests/evidence; production tree unchanged.

### S02 — Typed lifecycle diagnostics foundation

Stage file allowlist:

- apps/core-go/internal/core/diagnostics.go
- apps/core-go/internal/core/diagnostics_test.go
- apps/core-go/internal/core/lifecycle_diagnostics.go
- apps/core-go/internal/core/lifecycle_diagnostics_test.go
- apps/core-go/internal/migrations/runner.go
- apps/core-go/internal/migrations/runner_test.go
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Define canonical lifecycle transition/status/reason payload and bounded
      redaction rules.
- [x] Add one writer for diagnostic_events and diagnostic_workflow_links that
      propagates write failures to the caller or a rate-bounded health warning.
- [x] Add deterministic transition dedupe; preserve first/latest cause and
      attempt count without recording every poll.
- [x] Do not return a persisted-looking model-run ID after marshal/INSERT
      failure; retain current queued→terminal row identity until Provider-owned
      attempt propagation is added in S07.
- [x] Add only the additive indexes/columns required for correlation and
      transition queries.
- [x] Exact tests cover redaction, transition identity, dedupe, write failure
      visibility and schema fragments.
- [x] Rollback: revert the additive writer/migration; no runtime producer uses
      it yet.

### S03 — PostgreSQL-authoritative WakeUp clock

Stage file allowlist:

- apps/core-go/internal/core/wakeup.go
- apps/core-go/internal/core/wakeup_intents_test.go
- apps/core-go/internal/core/redis_triggers.go
- apps/core-go/internal/core/redis_triggers_test.go
- apps/core-go/cmd/worker/main.go
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Persist next WakeUp due time in the same transaction as each WakeUp
      outcome/fact and optional child intents.
- [x] Implement conditional PostgreSQL due release with expected
      status/due/cycle and RETURNING.
- [x] Make Redis expiry call the same release command; SET/listener failure is
      observable but never correctness-critical.
- [x] Worker periodically sweeps due WakeUp rows and audits overdue/missing
      clocks without logging every scan.
- [x] Duplicate cycle replay repairs the next due state before returning.
- [x] Remove WakeUp rearm/reset from user-turn paths only in the later owning
      stage; S03 tests isolate the clock/release API.
- [x] Exact tests cover lost SET, lost expiry, restart window, duplicate
      release, two-cycle recurrence and overdue event.
- [x] Rollback: disable the sweep and return to existing Redis hint while
      retaining additive diagnostics.

### S04 — Activation, Schedule decoupling and lifecycle rearm

Stage file allowlist:

- apps/core-go/internal/core/app.go
- apps/core-go/internal/core/operations.go
- apps/core-go/internal/core/settings.go
- apps/core-go/internal/core/settings_wakeup_test.go
- apps/core-go/internal/core/mutations.go
- apps/core-go/internal/core/wakeup.go
- apps/core-go/internal/core/wakeup_intents_test.go
- apps/core-go/internal/core/project_health_transaction_integration_test.go
- apps/core-go/internal/httpapi/server.go
- apps/core-go/internal/httpapi/server_test.go
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Activation transaction creates stable Schedule and WakeUp intent/clock
      independently; idempotent replay repairs either deterministic omission.
- [x] Remove accepted-Schedule existence from WakeUp ownership.
- [x] Project schedule_status into WakeUp context; missing Schedule never
      invents activity/place.
- [x] paused/disabled suppresses Provider work while preserving due policy;
      resume/re-enable releases overdue cycles with bounded jitter.
- [x] retired is a terminal suppression.
- [x] User turns no longer reset WakeUp due/TTL; they retain Reflection rearm.
- [x] Exact tests cover activation replay, missing/failed Schedule, lifecycle
      transitions and fixed cadence under frequent user turns.
- [x] PostgreSQL integration tests are authored but not run before S12.
- [x] Rollback: restore Schedule gate/user-turn call sites while leaving the
      durable clock code isolated.

### S05 — Reflection quiet-period and crash recovery

Stage file allowlist:

- apps/core-go/internal/core/cognition.go
- apps/core-go/internal/core/wakeup.go
- apps/core-go/internal/core/workflow_ops.go
- apps/core-go/internal/core/action_outcome.go
- apps/core-go/internal/core/action_outcome_test.go
- apps/core-go/internal/core/operations.go
- apps/core-go/internal/core/autonomy.go
- apps/core-go/internal/core/autonomy_policy_test.go
- apps/core-go/internal/core/reflection_window_v2.go
- apps/core-go/internal/core/reflection_runtime_v2.go
- apps/core-go/internal/core/reflection_v2_test.go
- apps/core-go/internal/core/reflection_closure_v2_test.go
- apps/core-go/internal/workflow/workflow.go
- apps/core-go/internal/workflow/workflow_test.go
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] All user-activity Reflection intents use one durable quiet-period due
      identity and are reset only by newer user activity.
- [x] WakeUp/action evidence creates an explicit due time rather than NULL
      immediate scheduling.
- [x] Reflection window claim has owner/lease/attempt identity and a runnable
      intent remains after crash/lease expiry.
- [x] Terminal retry is bounded, evidence-aware and compatible with workflow-ID
      reuse; no evidence completes as no_evidence without Provider I/O.
- [x] Every reset/cleanup error is observable; successful/no-change outcomes
      retain reason and watermark.
- [x] Check rows.Err on all Reflection evidence/appraisal iterators before
      accepting partial evidence or advancing a watermark.
- [x] Exact tests cover quiet-period reset, crash after claim, expired lease,
      terminal retry, no evidence and no-change.
- [x] Rollback: disable retry release without altering accepted reflection
      history or watermarks.

### S06 — Fair dispatcher, Temporal and Provider capacity

Stage file allowlist:

- apps/core-go/internal/workflow/workflow.go
- apps/core-go/internal/workflow/workflow_test.go
- apps/core-go/internal/core/provider_queue.go
- apps/core-go/internal/core/provider_queue_test.go
- apps/core-go/internal/core/provider_redis_queue.go
- apps/core-go/internal/core/provider_redis_queue_test.go
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Replace global strict-priority LIMIT starvation with bounded per-class/
      per-queue reservation or aging.
- [x] Reserve Temporal Activity capacity/lane so long Visual Identity work
      cannot occupy every WakeUp/Reflection execution slot while preserving
      active-history compatibility.
- [x] Add Provider queue aging/bounded-wait behavior for priority-70 lifecycle
      work without allowing it to preempt an already-running request.
- [x] Dispatcher settlement uses expected-state CAS and never overwrites cancel
      or another Worker’s transition.
- [x] Exact tests use large mixed backlogs and blocked long activities to prove
      WakeUp/Reflection reach Activity/Provider queue within the design bound.
- [x] Rollback: restore strict priority while retaining diagnostic proof of
      starvation.

### S07 — Instrument active lifecycle boundaries

Stage file allowlist:

- apps/core-go/internal/core/lifecycle_diagnostics.go
- apps/core-go/internal/core/wakeup.go
- apps/core-go/internal/core/workflow_ops.go
- apps/core-go/internal/core/provider.go
- apps/core-go/internal/core/diagnostics.go
- apps/core-go/internal/workflow/workflow.go
- apps/core-go/internal/workflow/workflow_test.go
- apps/core-go/internal/core/diagnostics_test.go
- apps/core-go/internal/core/wakeup_intents_test.go
- apps/core-go/internal/core/visual_identity.go (approved 2026-09-13)
- apps/core-go/internal/core/visual_identity_test.go (approved 2026-09-13)
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Propagate one correlation through WakeUp/Reflection intent, workflow/run,
      activity attempt, Provider model run, domain outcome and next due.
- [x] Add explicit Provider attempt identity so retries remain separate while
      each queued→running→terminal attempt still updates one coherent row.
- [x] Emit the canonical transition vocabulary at state changes only.
- [x] Instrument Redis/Describe/status-update/start/reconcile/Provider preflight
      failures and all intentional no-op/suppression branches.
- [x] Reconcile retry-eligibility lookup errors fail closed and remain eligible;
      Visual Identity/business terminal settlement persists before Activity
      reports success; Workflow status/command updates check RowsAffected.
- [x] Store workflow links and distinguish newly dispatched from already
      running.
- [x] Add a focused static audit test for newly forbidden silent catch-all and
      ignored-error patterns in owning lifecycle files; use an explicit
      allowlist for truly best-effort operations.
- [x] Exact tests reconstruct ordered no-op, actionable, retry and failure
      traces and prove redaction.
- [x] Rollback: retain domain fixes but disable individual event producers.

### S08 — Lifecycle Diagnostics API, BFF and browser

Stage file allowlist:

- apps/core-go/internal/core/operations.go
- apps/core-go/internal/core/diagnostics_test.go (approved 2026-09-13; Core lifecycle query/filter test owner)
- apps/core-go/internal/httpapi/server.go
- apps/core-go/internal/httpapi/domain.go (approved 2026-09-13; actual diagnostics handler owner)
- apps/core-go/internal/httpapi/server_test.go
- apps/gateway-go/internal/bff/routes.go
- apps/gateway-go/internal/bff/parity_behavior_test.go
- packages/browser-client/openapi.json (approved 2026-09-13; generated contract source)
- packages/browser-client/src/index.ts (approved 2026-09-13; actual generated client owner replacing nonexistent client.ts/contracts.ts)
- packages/browser-client/scripts/generate-openapi.mjs (approved 2026-09-13; actual OpenAPI source)
- packages/browser-client/scripts/generate.mjs (approved 2026-09-13; actual typed client source)
- packages/browser-client/test/client.test.ts
- apps/web/src/app/navigation.ts (approved 2026-09-13; DiagnosticsSection owner)
- apps/web/src/stores/control-center.ts
- apps/web/src/views/DiagnosticsView.vue
- apps/web/test/diagnostics.test.mjs
- apps/web/test/layout.test.mjs (approved 2026-09-13; existing diagnostics assertions updated for S08 filters/export)
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Add filtered lifecycle timeline query with Fluctlight/correlation/intent/
      workflow/run/surface/status filters.
- [x] Expose PostgreSQL intent snapshot independently from optional Temporal
      status/history and make diagnostics export apply the same Core-side
      filters.
- [x] Keep BFF as transport pass-through and browser contract typed.
- [x] Preserve safe Core error code/details/correlation through BFF and maintain
      per-source browser filter epochs so failed partial refresh cannot mix old
      and new correlation rows.
- [x] Add Lifecycle Diagnostics section with stage/status/reason/attempt/next
      due and links to model run/workflow controls.
- [x] Preserve existing Model Runs, Media Prompts, Events and Workflows.
- [x] Empty, overdue, no-op, retrying and failure states render distinctly.
- [x] Exact Core/BFF/client/Web tests cover mapping, filtering, redaction and
      narrow-screen safe rendering.
- [x] Rollback: hide the additive Lifecycle section; backend records remain
      queryable.

### S09 — Immutable initialization source and schema ownership

Stage file allowlist:

- apps/core-go/internal/migrations/runner.go
- apps/core-go/internal/migrations/runner_test.go
- apps/core-go/internal/core/app.go
- apps/core-go/internal/core/detail.go
- apps/core-go/internal/core/foundation_test.go
- apps/core-go/internal/core/app_capability_context.go
- apps/core-go/internal/core/provider_prompt_composer.go
- apps/core-go/internal/core/provider_schemas.go
- apps/core-go/internal/core/initialization_contract_test.go
- .trellis/tasks/09-13-runtime-usability-recovery/fixtures/
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Add immutable Owner/Fluctlight-scoped initialization source/analysis
      storage linked to accepted Foundation revision.
- [x] Add first-class identity/appearance/conflict/media/daily/profile fields
      identified in design; preserve unknown semantic content in namespaced
      extensions.
- [x] Keep shared semantic extensions in bounded runtime Persona projection;
      correct appearance ownership and shared actor_user Relationship behavior.
- [x] Store source digest, Provider/model/prompt/schema versions,
      classification evidence, field derivation evidence and coverage summary.
- [x] Exclude source text from ordinary ContextProjection and diagnostics.
- [x] Existing Fluctlights remain readable with missing source.
- [x] Exact tests cover field defaults, unknown preservation, source
      immutability, owner scoping and prompt exclusion.
- [x] PostgreSQL migration tests are authored but deferred to S12.
- [x] Rollback: additive source rows/fields remain unused; existing Persona read
      shape stays valid.

### S10 — Rich initialization prompt and semantic coverage

Stage file allowlist:

- apps/core-go/internal/core/app.go
- apps/core-go/internal/core/provider.go
- apps/core-go/internal/core/provider_shape.go
- apps/core-go/internal/core/provider_schemas.go
- apps/core-go/internal/core/diagnostics.go
- apps/core-go/internal/core/initialization_fidelity.go
- apps/core-go/internal/core/initialization_fidelity_test.go
- apps/core-go/internal/core/initialization_contract_test.go
- apps/core-go/internal/core/provider_live_tool_test.go
- apps/core-go/internal/core/testdata/initialization/
- .trellis/tasks/09-13-runtime-usability-recovery/fixtures/
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Keep initialization response_format=json_object.
- [x] Restore complete field vocabulary and source-to-owner instructions,
      finite inference rules, full single/multi classification and a complete
      canonical skeleton.
- [x] Preserve original source semantics and field-evidence basis while
      allowing truly absent values.
- [x] Reject empty StructuredFallback, truncated/unparseable output and
      semantic-empty default Persona with typed correlated retryable errors.
- [x] Add anonymized dense single and explicit multi card fixtures plus semantic
      assertion manifests.
- [x] Add semantic evaluator categories preserved/default_only/missing/
      moved_to_extension/contradicted/invented.
- [x] Add opt-in external real-card path/expectation manifest support with no
      source/response logging or snapshot; canary tests prove no leakage.
- [x] Initialization model-run diagnostics are metadata-only and Web/BFF/Core
      share one explicit byte-based source limit.
- [x] Exact deterministic tests cover prompt content, normalization, evaluator
      and invention guards. Live Provider tests are authored but not run.
- [x] Rollback: restore compact prompt; additive schema/source remains valid.

### S11 — Initialization source UI, contract audit and S12 preparation

Stage file allowlist:

- apps/core-go/internal/core/operations.go
- apps/core-go/internal/core/app.go (approved 2026-09-13; stale analysis activation authority)
- apps/core-go/internal/httpapi/server.go
- apps/core-go/internal/httpapi/server_test.go
- apps/gateway-go/internal/bff/routes.go
- packages/browser-client/scripts/generate-openapi.mjs (approved 2026-09-13; actual OpenAPI source)
- packages/browser-client/scripts/generate.mjs (approved 2026-09-13; actual typed client source)
- packages/browser-client/openapi.json (approved 2026-09-13; generated artifact)
- packages/browser-client/src/index.ts (approved 2026-09-13; generated client replacing nonexistent client.ts/contracts.ts)
- packages/browser-client/test/client.test.ts
- apps/web/src/components/instances/InstanceDetailsDialog.vue
- apps/web/src/views/InstancesView.vue
- apps/web/src/stores/control-center.ts
- apps/web/test/detail-display.test.mjs
- apps/web/test/skeleton.test.mjs
- .trellis/spec/backend/fluctlight-workflow-contract.md
- .trellis/spec/backend/fluctlight-diagnostics-contract.md
- .trellis/spec/backend/persona-layer-contract.md
- .trellis/spec/backend/visual-identity-contract.md
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md

Checklist:

- [x] Add collapsed Owner-only Initialization Source detail view and coverage
      summary without including source in normal diagnostics.
- [x] Make preview one typed Foundation state, honor JSON edits to
      goals/intentions, reject stale analysis activation and align the source
      input limit across Web/BFF/Core.
- [x] Audit all R1–R4 producer/consumer paths and update executable contracts.
- [x] Verify each S01–S10 checkbox/evidence and enumerate exact S12 environment
      prerequisites.
- [x] Run only exact S11 route/client/Web tests and targeted diff/format checks.
- [x] Do not run Live Provider, PostgreSQL integration, Temporal live or Docker.
- [x] Rollback: hide additive source UI; stored immutable source remains safe.

### S12 — Full verification and acceptance gate

Stage file allowlist:

- Files already modified in S01–S11
- apps/gateway-go/internal/bff/parity_routes_test.go (approved 2026-09-13; browser route inventory omitted the S08 Lifecycle Diagnostics route)
- apps/core-go/internal/migrations/prompt_context_memory_postgres_test.go (approved 2026-09-13; previous-head migration test still expected 0032 after current runner advances to 0033)
- apps/core-go/internal/core/prompt_context_assembler.go (approved 2026-09-13; full 11-capability conversation catalog plus response schema needs 17086 tokens, exceeding the obsolete 16384 sub-cap while remaining below total input budget)
- apps/core-go/internal/core/life_context_surface_test.go (approved 2026-09-13; stale WakeUp now freezes capability work for a separate action Activity and must not Prepare it before the authoritative WakeUp transaction succeeds)
- packages/core-client/openapi.json (approved 2026-09-13; Core Lifecycle Diagnostics route and initialization request contracts drift from active Go handlers)
- packages/core-client/scripts/generate.mjs (approved 2026-09-13; generated Core client must expose the lifecycle query and current initialization contract)
- packages/core-client/src/index.ts (approved 2026-09-13; regenerated Core client artifact)
- .trellis/tasks/09-13-runtime-usability-recovery/implementation-evidence.md
- .trellis/tasks/09-13-runtime-usability-recovery/final-report.md

Entry gate:

- [x] S01 through S11 are complete and checked.
- [x] A disposable PostgreSQL database, isolated Compose project and configured
      local Provider are available.
- [x] User-authorized complex external character-card/expectation paths are
      outside the repository and explicitly selected for the private live gate.

Checklist:

- [x] Run Required Validation Commands in order.
- [x] Record all output, environment exceptions and failure ownership.
- [x] Do not repair unrelated failures outside the accumulated allowlist; record
      them and request scope if needed.
- [x] Produce final acceptance report mapping every PRD criterion to evidence.

## Risk matrix

| Risk | Mitigation | Rollback |
| --- | --- | --- |
| Due sweep double-releases a cycle | expected status/due/cycle CAS, stable IDs and two-Worker tests | disable sweep; Redis hint remains |
| Fixed WakeUp overlaps active chat | short jitter and no-op/action policy; no forced message | temporarily defer due release |
| Schedule-missing cognition invents context | explicit missing status and prompt guard | suppress only schedule-dependent actions |
| Diagnostics create write storms | transition-only identity, dedupe and retention | disable selected info producers |
| Diagnostic writer failure affects business | best effort plus rate-bounded health warning | fall back to operational warning |
| Fairness reduces foreground throughput | bounded reservation/aging, never preempt running request | tune quota/age thresholds |
| Reflection retry duplicates evolution | evidence watermark, lease owner and stable intent/run IDs | stop retry; preserve dead letter |
| Rich prompt times out local Provider | keep json_object, token budget, Live Provider latency gate | revert prompt only |
| LLM invents facts | source evidence map and forbidden-invention assertions | reject preview, retain source |
| New source text leaks to prompts/logs | dedicated owner-only storage and static redaction tests | disable source API |
| Active Temporal history cannot replay | saved-history/testsuite verification and compatible registration | route old build/drain or rollback |

## Required Validation Commands

S12 only. Running any command in this section during S01–S11 violates the task
boundary.

Core/Gateway complete checks:

    GOCACHE=/tmp/lac-runtime-recovery-core-test go -C apps/core-go test ./... -count=1
    GOCACHE=/tmp/lac-runtime-recovery-core-race go -C apps/core-go test -race ./... -count=1
    go -C apps/core-go vet ./...
    go -C apps/core-go build ./...
    GOCACHE=/tmp/lac-runtime-recovery-gateway-test go -C apps/gateway-go test ./... -count=1
    GOCACHE=/tmp/lac-runtime-recovery-gateway-race go -C apps/gateway-go test -race ./... -count=1
    go -C apps/gateway-go vet ./...
    go -C apps/gateway-go build ./...
    test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
    git diff --check

Workspace/browser checks:

    pnpm -r test
    pnpm -r typecheck
    pnpm -r build
    pnpm generate
    git diff --check

Disposable PostgreSQL and Temporal-focused checks:

    CORE_GO_DATABASE_URL=<dedicated-disposable-postgres> go -C apps/core-go run ./cmd/migrate
    GO_CORE_TEST_DATABASE_URL=<dedicated-disposable-postgres> GOCACHE=/tmp/lac-runtime-recovery-pg go -C apps/core-go test -p 1 ./internal/migrations ./internal/core ./internal/httpapi -count=1
    GO_CORE_TEST_DATABASE_URL=<dedicated-disposable-postgres> GOCACHE=/tmp/lac-runtime-recovery-wakeup go -C apps/core-go test ./internal/core -run 'Test.*(WakeUp|Reflection|LifecycleDiagnostic|InitializationSource)' -count=1 -v

Local Live Provider semantic-fidelity gate:

    FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
    FLUCTLIGHT_LIVE_PROVIDER_URL=http://127.0.0.1:11234/v1 \
    FLUCTLIGHT_LIVE_PROVIDER_MODEL=huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX \
    FLUCTLIGHT_LIVE_INITIALIZATION_CARD_PATH=<external-owner-card> \
    FLUCTLIGHT_LIVE_INITIALIZATION_EXPECTATIONS_PATH=<external-owner-expectations> \
    GOCACHE=/tmp/lac-runtime-recovery-live \
    go -C apps/core-go test ./internal/core -run 'TestLiveProviderDense(Single|Multi|External)Initialization' -count=1 -v

Compose and acceptance, using an isolated task-specific env/project:

    docker compose --env-file <isolated-env> -p lac-runtime-recovery -f infra/compose/fluctlight.compose.yml up -d --build
    FLUCTLIGHT_ENV_FILE=<isolated-env> COMPOSE_PROJECT_NAME=lac-runtime-recovery ./infra/compose/run-platform-smoke.sh
    ./infra/acceptance/check-compose-bind-sources.sh
    ./infra/acceptance/check-core-openapi.sh
    ./infra/acceptance/check-go-projections.sh
    ./infra/acceptance/run-go-active-workflow.sh

Do not copy over an existing infra/compose/fluctlight.env or point destructive
migration fixtures at a user database.
