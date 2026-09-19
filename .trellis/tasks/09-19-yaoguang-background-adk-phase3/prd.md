# 摇光项目第三阶段：后台主动行为接入已有 ADK 运行基础

## Goal

在第一、二阶段已经验收的 Eino/Composer/Capability/ADK 基础上，将 WakeUp
后台主动行为的模型决策接入共享 ADK 执行机制，同时保留自治策略、冻结、事务、
intent/outbox、Temporal、Redis、幂等和发布边界。只把真正需要共享 Agent 运行支撑
的部分抽取为通用能力，不把没有工具反馈需求的后台单次 Task 机械改造成 Agent。

## 前置证据

- 阶段一基线：`89764bc`。
- 阶段二实现提交：`7a47b66`。
- 阶段二归档提交：`bcf83ad`，session journal：`26aaca2`。
- 当前阶段二源码已经有真实 ADK model → tool → result → model 测试，且 Core/Gateway
  test、race、vet、gofmt、mod tidy 和旧控制流扫描通过。
- 当前 worktree：`codex/yaoguang-adk-phase2`，从阶段一 Eino 基础分支继续，工作区干净。

## Background task classification

| Task | Real entry | Tool feedback needed | Decision |
| --- | --- | --- | --- |
| WakeUp | `WakeUpWorkflow → ProcessWakeUpActivity → App.ProcessWakeUp` | Must share ADK execution support; current action tools are frozen and settled asynchronously, so no new unbounded feedback policy | Migrate model decision to generic ADK task boundary; preserve bounded/terminal behavior and existing async settlement |
| Daily Review | `DailyReviewWorkflow → ProcessDailyReviewActivity → App.ProcessDailyReview` | Current tool calls are frozen execution declarations; no model consumption of results | Keep `RunStructuredToolsTask` single task |
| Native Cognition | `ProcessCognitionActivity → ProcessCognitionInbox → ProcessNativeCognitionFact` | Tool-only/native effects are settled after freeze; no second model decision | Keep single task |
| Reflection | `ProcessReflectionActivity → ProcessReflection` | Explicitly rejects ToolCall and fallback | Keep single task |
| Summary/Schedule/Media/Visual/Embedding | Existing ModelTask or fixed workflow | No background tool-feedback loop | Keep existing task/workflow |
| Autonomy/Capability action workers | `ProcessAutonomyAction` / `ProcessCapabilityAction` | Durable side-effect execution, not model planning | Keep fixed async worker |

## Requirements

### R1. Shared ADK runtime

- Extract a request-scoped, surface-aware ADK structured-task boundary from the
  conversation-specific bridge.
- Preserve Eino factory, model role assignment, queue lease, timeout/cancellation,
  diagnostics, provider attempt identity and usage for every physical model call.
- WakeUp must enter this shared ADK boundary through its real Worker/Activity/Core path.
- ADK loop has an explicit maximum of two model generations and no hidden retry/fallback.
- No new BackgroundAgent engine, Provider, Prompt Composer, CapabilityRegistry or global mutable Agent.

### R2. WakeUp behavior and scope

- Preserve WakeUp cycle identity, `wake_up.current` recurrence, `wakeID`, action IDs,
  owner/Fluctlight scope, `MemoryForWakeUp`, autonomy policy, cancellation marker,
  authority revision checks and existing output routing.
- WakeUp model-visible tools come only from `CapabilityRegistry.Catalog(CapabilitySurfaceWakeUp)`.
- Conversation-only and `InternalOnly` capabilities must not leak into WakeUp.
- Pure query capability results may be returned to ADK when a model call actually requests
  them; mutation/deferred/external capabilities remain deferred and cannot commit from the
  ADK callback.
- No fake user message, no conversation-history pollution, no direct publish from ADK events.
- A valid no-op remains a successful lifecycle result; model/tool/cancel/iteration failures
  must not be converted into no-op success.

### R3. Non-migrated tasks

- Daily Review, Native Cognition, Reflection, Summary, Schedule, Media, Visual Identity and
  Embedding remain their existing operation-owned single Task or fixed async workflow unless
  an actual feedback requirement is discovered in code.
- Add regression guards proving their schemas do not silently enter the WakeUp/Conversation
  ADK loop.

### R4. Temporal/Worker/lifecycle preservation

- Do not change Workflow/Activity names, queues, timeouts, retry budgets, Redis hints,
  PostgreSQL recurrence authority, outbox or intent contracts.
- Propagate Activity context cancellation into the shared ADK request and each model/tool call.
- Preserve current preemption semantics; do not add a new checkpoint or recovery store.
- Report the existing Daily Review/Native Cognition Activity lifecycle-marker asymmetry if it
  remains outside this phase's scope; do not silently broaden this task into lifecycle redesign.

### R5. Evidence and documentation

- Add a real WakeUp ADK test from the provider/runtime boundary and, where PostgreSQL is
  available, the real `ProcessWakeUp` entry.
- Cover no-op, tool/result pairing, unauthorized/internal tool rejection, deferred side-effect
  behavior, model/tool failure, cancellation, iteration cap, idempotent retry and output scope.
- Document the classification table, exact call path, preserved business rules, removed/changed
  control flow, tests and external unverified items.

## Out of scope

- No new autonomy thresholds, scheduling rules, personality rules, memory/evolution rules,
  BFF/API protocol, Temporal replacement, multi-agent platform or background checkpoint store.
- No forced ADK migration for Reflection, Summary, Embedding, Media, Visual Identity, Schedule
  or Daily Review/Native Cognition single-task flows.
- No real-user message, dynamic post, production database mutation or external-provider smoke
  without an explicitly isolated environment and credentials.

## Acceptance criteria

- [x] WakeUp's real model decision enters the shared request-scoped ADK path.
- [x] Tool catalog and server execution surface are identical and WakeUp-scoped.
- [x] Formal ToolCall identity/result pairing is preserved; any actual feedback round is visible
  in the next model request and is bounded to two generations.
- [x] WakeUp no-op, policy rejection, deferred action, cancellation and failures retain their
  existing lifecycle/frozen/intent semantics without fabricated success.
- [x] Daily Review, Native Cognition, Reflection and other classified single tasks remain single
  tasks and do not gain accidental ADK rounds or permissions.
- [x] Worker/Activity/Temporal/Redis/PostgreSQL contracts remain unchanged except for the minimal
  shared runtime wiring.
- [x] Core/Gateway tests, race, vet, formatting, tidy, static scans and task documentation pass.
