# 提交计划（待一次确认）

初始工作区干净；以下改动均由本任务产生，未识别的其他改动：无。不推送、不amend，不运行配置或真实服务。

1. `fix: isolate media subjects and integrate Kev capture plans`

   - apps/core-go/cmd/kev-config/main.go
   - apps/core-go/internal/ai/decision/decision.go
   - apps/core-go/internal/ai/decision/decision_test.go
   - apps/core-go/internal/ai/prompt/prompts.go
   - apps/core-go/internal/core/builtin_capabilities.go
   - apps/core-go/internal/core/capability_core_test.go
   - apps/core-go/internal/core/current_capture.go
   - apps/core-go/internal/core/media.go
   - apps/core-go/internal/core/media_capture_plan.go
   - apps/core-go/internal/core/media_capture_plan_test.go
   - apps/core-go/internal/core/media_context_test.go
   - apps/core-go/internal/core/media_probe_test.go
   - apps/core-go/internal/core/media_prompt_branches.go
   - apps/core-go/internal/core/media_subject_context.go
   - apps/core-go/internal/core/media_workflow_policy.go
   - apps/core-go/internal/core/model_tasks.go
   - apps/core-go/internal/core/provider_context.go
   - apps/core-go/internal/core/provider_prompts_test.go
   - apps/core-go/internal/core/testdata/media_capture_cases.json
   - apps/core-go/internal/core/tool_contract.go
   - apps/core-go/internal/migrations/media_kev.go
   - apps/core-go/internal/migrations/media_kev_test.go
   - apps/core-go/internal/migrations/runner.go
   - apps/core-go/internal/migrations/runner_test.go
   - apps/core-go/internal/workflow/media_validation_test.go
   - apps/core-go/internal/workflow/workflow.go
   - apps/web/src/components/KevSettings.vue

2. `docs: record media capture contracts and verification`

   - .trellis/spec/backend/index.md
   - .trellis/spec/backend/kev-decision-contract.md
   - .trellis/spec/backend/media-capture-plan-contract.md
   - .trellis/spec/backend/media-prompt-contract.md
   - .trellis/spec/backend/visual-identity-contract.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/check.jsonl
   - .trellis/tasks/10-11-media-kev-prompt-isolation/design.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/implement.jsonl
   - .trellis/tasks/10-11-media-kev-prompt-isolation/implement.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/media-kev-prompt-integration-report.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/prd.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/research/baseline.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/research/entry-map.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/research/legacy-defaults.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/research/temporal-retry-boundary.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/source-requirements.md
   - .trellis/tasks/10-11-media-kev-prompt-isolation/task.json

任务继续保留in_progress供用户真实验收；此计划不包含自动归档或业务数据操作。
