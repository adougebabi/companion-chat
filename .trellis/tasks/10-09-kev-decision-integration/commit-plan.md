# Proposed Wake/context-authority feedback fix

`fix(wakeup): isolate periodic context and fence only semantic facts`

One coherent feedback commit covering holistic fixed protocol, Wake historical input, shared section separators, Goal compact copy, truthful action/duplicate suppression,0058 cache-trigger correction and idempotent persona writes, regression tests and matching spec/task/report/review artifact. All dirty paths belong to this round; no unrecognized files. No deployment, production mutation or push.

## Files

- `.trellis/spec/backend/fluctlight-persistence-contract.md`
- `.trellis/spec/backend/fluctlight-provider-contract.md`
- `.trellis/tasks/10-09-kev-decision-integration/design.md`
- `.trellis/tasks/10-09-kev-decision-integration/implement.md`
- `.trellis/tasks/10-09-kev-decision-integration/prd.md`
- `.trellis/tasks/10-09-kev-decision-integration/task.json`
- `apps/core-go/internal/core/agent_result_adapter.go`
- `apps/core-go/internal/core/capability_prompt_policy.go`
- `apps/core-go/internal/core/current_authority_cache_test.go`
- `apps/core-go/internal/core/prompt_context_assembler.go`
- `apps/core-go/internal/core/prompt_context_assembler_test.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_context_test.go`
- `apps/core-go/internal/core/provider_prompt_composer.go`
- `apps/core-go/internal/core/provider_prompt_composer_test.go`
- `apps/core-go/internal/core/provider_schemas.go`
- `apps/core-go/internal/core/provider_schemas_test.go`
- `apps/core-go/internal/core/wakeup.go`
- `apps/core-go/internal/core/wakeup_test.go`
- `apps/core-go/internal/core/working_persona_persistence.go`
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `apps/core-go/internal/migrations/semantic_fact_generation.go`
- `apps/core-go/internal/migrations/semantic_fact_generation_test.go`
- `docs/kev-integration-report.md`
- `docs/wake-up-fixed-prompt.md`

## Validation

Go race1422PASS/494SKIP/0FAIL (including subtests); Web/client98PASS. generate/typecheck/vet/build/diff passed. Isolated PostgreSQL migration and actual service performance remain SKIP/NOT_RUN under user agreement.
