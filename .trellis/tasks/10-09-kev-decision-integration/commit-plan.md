# Proposed conversation feedback-fix commit

`fix(conversation): repair missing output and preserve settlement diagnostics`

One commit: conditional visible final output requirement, existing tool-free repair, caller-owned post-run diagnostic boundaries, fact-revision mismatch diagnostics, corresponding tests/spec/task/report. All dirty files are this round. No unrecognized files. No migration, deployment, production writes or push. Current facts CAS remains unchanged; actual production conflict writer remains unproven.

## Files

- `.trellis/spec/backend/fluctlight-diagnostics-contract.md`
- `.trellis/spec/backend/fluctlight-provider-contract.md`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/internal/core/agent_result_adapter.go`
- `apps/core-go/internal/core/agent_run_record.go`
- `apps/core-go/internal/core/conversation_delivery_regression_test.go`
- `apps/core-go/internal/core/conversation_output_contract_test.go`
- `apps/core-go/internal/core/current_authority.go`
- `apps/core-go/internal/core/durable_turn_test.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `docs/kev-integration-report.md`

## Validation

Go race 1403 PASS / 491 SKIP / 0 FAIL; vet/build and pnpm generate/typecheck/test/build passed; Web/client98 PASS. Database opt-in validation remains SKIP.
