# Proposed Goal/Kev feedback-fix commit

`fix(goals): validate completion contracts and trim Kev batch state`

One feedback-fix commit covers frozen requirement quotes, optional improvements, semantic preflight/reciprocal object completion, bounded correction, policy-v3 memo invalidation, batch-local context/tool state, exact deadline diagnostics, corresponding tests/spec/task/report. All current dirty files are owned by this round. No unrecognized dirty files. No migration, production mutation, deployment, or push.

## Files

- `.trellis/spec/backend/fluctlight-provider-contract.md`
- `.trellis/spec/backend/kev-decision-contract.md`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/internal/ai/decision/decision.go`
- `apps/core-go/internal/ai/decision/decision_test.go`
- `apps/core-go/internal/core/goal_assessment_memo_test.go`
- `apps/core-go/internal/core/goal_boundary_regression_test.go`
- `apps/core-go/internal/core/goal_closure.go`
- `apps/core-go/internal/core/goal_evaluation_persistence.go`
- `apps/core-go/internal/core/goal_evaluation_scope_test.go`
- `apps/core-go/internal/core/goal_evaluation_task.go`
- `apps/core-go/internal/core/goal_evaluation_validation.go`
- `apps/core-go/internal/core/goal_evaluation_validation_test.go`
- `apps/core-go/internal/core/goal_evaluation_wire.go`
- `apps/core-go/internal/core/goal_evaluation_wire_dto.go`
- `apps/core-go/internal/core/goal_evaluation_wire_test.go`
- `apps/core-go/internal/core/kev_decisions.go`
- `apps/core-go/internal/core/kev_test.go`
- `apps/core-go/internal/core/kev_tools.go`
- `docs/kev-integration-report.md`

## Validation

Go race 1396 PASS / 490 SKIP / 0 FAIL (including subtests); vet/build and generate/typecheck/Web-client tests(98)/build passed. Prior-policy memo focused test also passed after adding v2. Real Provider/PG/production performance acceptance remains with user.
