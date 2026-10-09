# Proposed Goal Evaluation feedback-fix commit

`fix(goals): scope evaluation output and expose settlement failures`

One coherent commit covering per-goal output schema, operation/ref guards, complete replacement correction, terminal invalid-output handling, independent settlement diagnostics, regenerated clients, regression tests and matching spec/task/report. No new migration. All listed paths belong to this round; no unrecognized dirty files. No push proposed.

## Files

- `.trellis/spec/backend/fluctlight-provider-contract.md`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/internal/core/diagnostic_model_runs_filter.go`
- `apps/core-go/internal/core/goal_evaluation_diagnostics.go`
- `apps/core-go/internal/core/goal_evaluation_runtime.go`
- `apps/core-go/internal/core/goal_evaluation_scope_test.go`
- `apps/core-go/internal/core/goal_evaluation_task.go`
- `apps/core-go/internal/core/goal_evaluation_wire.go`
- `apps/core-go/internal/core/goal_evaluation_wire_test.go`
- `apps/core-go/internal/httpapi/browser/dto.go`
- `apps/web/src/views/DiagnosticsView.vue`
- `docs/kev-integration-report.md`
- `packages/browser-client/openapi.json`
- `packages/browser-client/scripts/generate-openapi.mjs`
- `packages/browser-client/scripts/generate.mjs`
- `packages/browser-client/src/index.ts`
- `packages/core-client/scripts/generate.mjs`
- `packages/core-client/src/index.ts`

## Validation

Go race: 1374 PASS / 490 SKIP / 0 FAIL (including subtests). Web/client: 98 PASS. Go vet/build and pnpm generate/typecheck/test/build passed. Database and real Provider opt-in checks remain SKIP/NOT_RUN under the user acceptance agreement.
