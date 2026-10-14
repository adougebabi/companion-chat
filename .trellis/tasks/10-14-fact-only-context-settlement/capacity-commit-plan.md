# F14 新增修复提交计划

当前分支：`codex/fact-only-reply-vision`；基线：`26bfc58`。原四笔工作提交已完成；以下为用户追加F14的独立批次，待一次确认后执行，不amend、不push、不部署或操作真实workflow。

拟提交一笔：`fix(core): make internal context capacity advisory`

将内部引用容量改为告警，完整贯通两套capability冻结/prepared/结算路径，补齐模型前安全诊断及24+1实际结果引用回归。身份、授权、JSON/hash和模型参数合同保留。

## 源码与回归（11文件）

- `apps/core-go/internal/capability/capability.go`
- `apps/core-go/internal/core/action_outcome.go`
- `apps/core-go/internal/core/capability_core.go`
- `apps/core-go/internal/core/cognition.go`
- `apps/core-go/internal/core/context_reference.go`
- `apps/core-go/internal/core/context_reference_capacity.go`
- `apps/core-go/internal/core/context_reference_capacity_test.go`
- `apps/core-go/internal/core/intelligence.go`
- `apps/core-go/internal/core/provider_context.go`
- `apps/core-go/internal/core/reflection_runtime_v2.go`
- `apps/core-go/internal/core/tool_execution.go`

## 可执行契约（3文件）

- `.trellis/spec/backend/fluctlight-diagnostics-contract.md`
- `.trellis/spec/backend/quality-guidelines.md`
- `.trellis/spec/backend/structured-turn-contract.md`

## 交付说明

- `docs/context-capacity-availability.md`

## 任务、研究与检查证据

- `.trellis/tasks/10-14-fact-only-context-settlement/availability-followup.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/capacity-commit-plan.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/check.jsonl`
- `.trellis/tasks/10-14-fact-only-context-settlement/design.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/implement.jsonl`
- `.trellis/tasks/10-14-fact-only-context-settlement/implement.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/implementation-state.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/local-check.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/prd.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/capacity-diagnostics-audit.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/capacity-pipeline-audit.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/capacity-review.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-capacity-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-diagnostics-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-native-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/research/context-repair-contract.md`
- `.trellis/tasks/10-14-fact-only-context-settlement/task.json`
- `.trellis/tasks/10-14-fact-only-context-settlement/verification/capacity-check-summary.json`

共33个文件，全部为本轮主代理修改或已核对的探子研究产物。Unrecognized dirty files：无。

验证：八项顶层容量回归与完整Go test/race/vet/build通过；pnpm typecheck/test/build通过；gofmt、git diff --check、任务context manifests与原文excerpt/source hash核对通过。条件化SKIP、真实环境NOT_RUN详见verification/capacity-check-summary.json。

确认后只执行该清单git add与一笔本地git commit；任务继续in_progress，用户环境验收/归档独立处理。
