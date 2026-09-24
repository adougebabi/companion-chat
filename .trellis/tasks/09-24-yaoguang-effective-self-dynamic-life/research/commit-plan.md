# 提交分组（待确认）

本任务可识别改动共 78 个文件；未识别脏文件：无。忽略的本地脱敏抓包 `research/wire-excerpts.json` 不提交。

## 1. `feat(core-go): close effective self and dynamic life loop`（64 个文件）

- `apps/core-go/cmd/effective-life-backfill/main.go`
- `apps/core-go/internal/core/action_outcome.go`
- `apps/core-go/internal/core/agent_result_adapter.go`
- `apps/core-go/internal/core/app.go`
- `apps/core-go/internal/core/app_capability_context.go`
- `apps/core-go/internal/core/appearance_style_capability.go`
- `apps/core-go/internal/core/appearance_style_capability_test.go`
- `apps/core-go/internal/core/body_event.go`
- `apps/core-go/internal/core/body_event_test.go`
- `apps/core-go/internal/core/builtin_capabilities.go`
- `apps/core-go/internal/core/capability_core.go`
- `apps/core-go/internal/core/capability_prompt_policy.go`
- `apps/core-go/internal/core/conversation_mixed_media_reply_test.go`
- `apps/core-go/internal/core/conversation_summary.go`
- `apps/core-go/internal/core/diagnostics.go`
- `apps/core-go/internal/core/effective_life.go`
- `apps/core-go/internal/core/effective_life_agent_loop_test.go`
- `apps/core-go/internal/core/effective_life_backfill.go`
- `apps/core-go/internal/core/effective_life_backfill_test.go`
- `apps/core-go/internal/core/effective_life_prompt_test.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/formal_agent_e2e_test.go`
- `apps/core-go/internal/core/formal_agents.go`
- `apps/core-go/internal/core/formal_tool_adapter_e2e_test.go`
- `apps/core-go/internal/core/goal_intention.go`
- `apps/core-go/internal/core/habit_capabilities.go`
- `apps/core-go/internal/core/habit_capabilities_test.go`
- `apps/core-go/internal/core/independent_tools_e2e_test.go`
- `apps/core-go/internal/core/intelligence.go`
- `apps/core-go/internal/core/intention_capabilities.go`
- `apps/core-go/internal/core/intention_capabilities_test.go`
- `apps/core-go/internal/core/life_activity_capabilities.go`
- `apps/core-go/internal/core/life_activity_capabilities_test.go`
- `apps/core-go/internal/core/life_activity_resolution.go`
- `apps/core-go/internal/core/media.go`
- `apps/core-go/internal/core/media_context_test.go`
- `apps/core-go/internal/core/media_quality.go`
- `apps/core-go/internal/core/model_tasks.go`
- `apps/core-go/internal/core/operations.go`
- `apps/core-go/internal/core/persona_compilation.go`
- `apps/core-go/internal/core/persona_compilation_test.go`
- `apps/core-go/internal/core/persona_detail_capability.go`
- `apps/core-go/internal/core/persona_detail_capability_test.go`
- `apps/core-go/internal/core/phase8_contract_matrix_test.go`
- `apps/core-go/internal/core/prompt_context_assembler.go`
- `apps/core-go/internal/core/prompt_context_assembler_test.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_context_test.go`
- `apps/core-go/internal/core/provider_schemas.go`
- `apps/core-go/internal/core/schedule_generation.go`
- `apps/core-go/internal/core/testdata/turn_path_cost_report.json`
- `apps/core-go/internal/core/wardrobe_capabilities.go`
- `apps/core-go/internal/core/wardrobe_capabilities_test.go`
- `apps/core-go/internal/core/wardrobe_event.go`
- `apps/core-go/internal/core/wardrobe_event_test.go`
- `apps/core-go/internal/core/working_persona_backfill.go`
- `apps/core-go/internal/core/working_persona_chain_test.go`
- `apps/core-go/internal/core/working_persona_fixture_test.go`
- `apps/core-go/internal/core/working_persona_persistence.go`
- `apps/core-go/internal/migrations/effective_life.go`
- `apps/core-go/internal/migrations/runner.go`
- `apps/core-go/internal/migrations/runner_test.go`
- `infra/acceptance/run-go-live-provider-smoke.sh`
- `infra/acceptance/run-go-live-provider-smoke.test.mjs`

## 2. `docs(trellis): record effective life contracts and acceptance evidence`（14 个文件）

- `.trellis/spec/backend/effective-life-contract.md`
- `.trellis/spec/backend/fluctlight-autonomy-contract.md`
- `.trellis/spec/backend/fluctlight-persistence-contract.md`
- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/persona-layer-contract.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/check.jsonl`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/design.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/implement.jsonl`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/implement.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/prd.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/research/acceptance.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/research/baseline.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/research/commit-plan.md`
- `.trellis/tasks/09-24-yaoguang-effective-self-dynamic-life/task.json`
