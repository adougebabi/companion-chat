# Proposed coverage feedback-fix commit

`fix(goals): require complete evaluation batches and disable thinking`

All current dirty files belong to this feedback fix: output cardinality, explicit thinking policy, HTTP/schema and production-entry regression assertions, accompanying contract/task/report. No unrecognized dirty files. No migration, deployment or push.

## Files

- `.trellis/spec/backend/fluctlight-provider-contract.md`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/goal_assessment_memo_integration_test.go`
- `apps/core-go/internal/core/goal_evaluation_scope_test.go`
- `apps/core-go/internal/core/goal_evaluation_task.go`
- `apps/core-go/internal/core/provider.go`
- `docs/kev-integration-report.md`

## Validation

Go race 1378 PASS/490 SKIP/0 FAIL (including subtests); vet/build/typecheck passed. Web/client 98 PASS; production build passed. Live environment read-only confirms coverage failure; new fix is local and uncommitted.
