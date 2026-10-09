# Proposed commit

`feat(kev): integrate active decisions with guarded fallback and diagnostics`

Includes all implementation, tests, generated clients, deployment configuration, code-spec updates and task/report files edited in this session. No unrecognized dirty paths were found; the initial product checkout was clean. No push is proposed.

## Files

- `.trellis/spec/backend/goal-planner-contract.md`
- `.trellis/spec/backend/index.md`
- `.trellis/spec/frontend/state-management.md`
- `apps/core-go/Dockerfile`
- `apps/core-go/cmd/api/main.go`
- `apps/core-go/cmd/worker/main.go`
- `apps/core-go/internal/core/adk_conversation_runtime.go`
- `apps/core-go/internal/core/agent_result_adapter.go`
- `apps/core-go/internal/core/app.go`
- `apps/core-go/internal/core/builtin_capabilities.go`
- `apps/core-go/internal/core/capability_runtime.go`
- `apps/core-go/internal/core/diagnostics_test.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/formal_tool_adapter_e2e_test.go`
- `apps/core-go/internal/core/goal_evaluation_runtime.go`
- `apps/core-go/internal/core/goal_planner.go`
- `apps/core-go/internal/core/independent_tools_e2e_test.go`
- `apps/core-go/internal/core/intelligence.go`
- `apps/core-go/internal/core/persona_action_service.go`
- `apps/core-go/internal/core/phase8_contract_matrix_test.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/runtime_context_refresh.go`
- `apps/core-go/internal/core/settings.go`
- `apps/core-go/internal/core/wakeup.go`
- `apps/core-go/internal/core/workflow_ops.go`
- `apps/core-go/internal/httpapi/browser/route_matrix_test.go`
- `apps/core-go/internal/httpapi/browser/routes.go`
- `apps/core-go/internal/httpapi/browser_backend.go`
- `apps/core-go/internal/httpapi/server.go`
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/core-go/internal/workflow/workflow.go`
- `apps/web/src/app/navigation.ts`
- `apps/web/src/stores/control-center.ts`
- `apps/web/src/views/DiagnosticsView.vue`
- `apps/web/src/views/SettingsView.vue`
- `apps/web/test/layout.test.mjs`
- `infra/acceptance/run-go-live-provider-smoke.sh`
- `infra/compose/fluctlight.compose.yml`
- `packages/browser-client/openapi.json`
- `packages/browser-client/scripts/generate-openapi.mjs`
- `packages/browser-client/scripts/generate.mjs`
- `packages/browser-client/src/index.ts`
- `packages/core-client/openapi.json`
- `packages/core-client/src/index.ts`
- `.trellis/spec/backend/kev-decision-contract.md`
- `.trellis/tasks/10-09-kev-decision-integration/check.jsonl`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.jsonl`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/research/autonomy-contract-part-1.md`
- `.trellis/tasks/10-09-kev-decision-integration/research/autonomy-contract-part-2.md`
- `.trellis/tasks/10-09-kev-decision-integration/research/baseline.md`
- `.trellis/tasks/10-09-kev-decision-integration/research/entry-mapping.md`
- `.trellis/tasks/10-09-kev-decision-integration/source-decision-analysis.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/cmd/kev-config/main.go`
- `apps/core-go/internal/ai/decision/decision.go`
- `apps/core-go/internal/ai/decision/decision_test.go`
- `apps/core-go/internal/core/kev_decisions.go`
- `apps/core-go/internal/core/kev_diagnostics.go`
- `apps/core-go/internal/core/kev_persona.go`
- `apps/core-go/internal/core/kev_test.go`
- `apps/core-go/internal/core/kev_tools.go`
- `apps/core-go/internal/httpapi/kev.go`
- `apps/core-go/internal/migrations/kev_decisions.go`
- `apps/core-go/internal/workflow/kev_test.go`
- `apps/web/src/components/KevDiagnostics.vue`
- `apps/web/src/components/KevSettings.vue`
- `docs/kev-integration-report.md`
- `packages/browser-client/test/kev.test.ts`
- `.trellis/tasks/10-09-kev-decision-integration/commit-plan.md`

Awaiting the one-shot confirmation required by workflow Phase 3.4. After confirmation: work commit, current task archive and developer journal in that order.
