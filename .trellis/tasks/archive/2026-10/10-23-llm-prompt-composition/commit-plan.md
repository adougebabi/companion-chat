# 提交计划（已确认并执行）

用户已明确同意提交。工作提交：`refactor: unify LLM prompt composition and semantic inputs`

此提交只包含本任务代码、测试、AGENTS 与规范/组成清单，不 push、不 amend。
确认工作提交后，再按 Trellis 流程归档本任务并记录 journal；它们属于后续记账提交。

工作提交 SHA：`2d82c94e73e823b8001f6cb9d1f3195cbbae9b66`。

## 工作文件

- `.trellis/spec/backend/fact-only-reply-contract.md`
- `.trellis/spec/backend/fluctlight-autonomy-contract.md`
- `.trellis/spec/backend/fluctlight-diagnostics-contract.md`
- `.trellis/spec/backend/fluctlight-memory-contract.md`
- `.trellis/spec/backend/fluctlight-provider-contract.md`
- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/kev-decision-contract.md`
- `.trellis/spec/backend/persona-layer-contract.md`
- `.trellis/spec/backend/prompt-composition-contract.md`
- `.trellis/spec/backend/structured-turn-contract.md`
- `AGENTS.md`
- `apps/core-go/internal/ai/prompt/format.go`
- `apps/core-go/internal/ai/prompt/format_test.go`
- `apps/core-go/internal/core/eino_model_runtime.go`
- `apps/core-go/internal/core/fact_only_actor_wire.go`
- `apps/core-go/internal/core/fact_only_context_views.go`
- `apps/core-go/internal/core/fact_only_history.go`
- `apps/core-go/internal/core/formal_agents.go`
- `apps/core-go/internal/core/goal_evaluation_source_input.go`
- `apps/core-go/internal/core/goal_evaluation_task.go`
- `apps/core-go/internal/core/goal_evaluation_wire.go`
- `apps/core-go/internal/core/goal_evaluation_wire_test.go`
- `apps/core-go/internal/core/goal_planner.go`
- `apps/core-go/internal/core/kev_persona.go`
- `apps/core-go/internal/core/kev_tools.go`
- `apps/core-go/internal/core/media_capture_plan.go`
- `apps/core-go/internal/core/media_context_test.go`
- `apps/core-go/internal/core/media_quality_test.go`
- `apps/core-go/internal/core/model_tasks.go`
- `apps/core-go/internal/core/moment_npc_interactions.go`
- `apps/core-go/internal/core/owner_life_requests.go`
- `apps/core-go/internal/core/persona_compilation.go`
- `apps/core-go/internal/core/prompt_composition_contract_test.go`
- `apps/core-go/internal/core/prompt_context_assembler.go`
- `apps/core-go/internal/core/prompt_context_assembler_test.go`
- `apps/core-go/internal/core/prompt_semantic_views.go`
- `apps/core-go/internal/core/provider.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/provider_context_test.go`
- `apps/core-go/internal/core/provider_prompt_cache_prefix_test.go`
- `apps/core-go/internal/core/provider_prompt_composer.go`
- `apps/core-go/internal/core/provider_prompt_composer_test.go`
- `apps/core-go/internal/core/provider_prompt_format.go`
- `apps/core-go/internal/core/provider_prompt_format_test.go`
- `apps/core-go/internal/core/provider_prompts_test.go`
- `apps/core-go/internal/core/reply_context.go`
- `apps/core-go/internal/core/reply_expression_test.go`
- `apps/core-go/internal/core/studio_runtime.go`
- `apps/core-go/internal/core/visual_identity_tools_test.go`
- `apps/core-go/internal/core/wardrobe_preparation.go`
- `apps/core-go/internal/core/working_memory.go`
- `apps/core-go/internal/core/working_memory_test.go`
- `apps/core-go/internal/core/working_persona_test.go`
- `apps/core-go/internal/core/world_encounter_response.go`
- `apps/core-go/internal/core/world_opportunity_cognition.go`
- `docs/llm-prompt-composition.md`

## 排除

`.trellis/tasks/10-22-pen-production-ui/` 下七类原有未提交记录归其他 session，
不纳入此工作提交，也不修改。当前任务的规划/研究/evidence 在后续任务归档提交中保存。

## 验证边界

变更范围回归和 vet 通过。四项仓库基线失败在干净 HEAD 复现，记录在 evidence；
未运行真实 Provider 或 PostgreSQL 业务验收。提交不会改变此证据范围。
