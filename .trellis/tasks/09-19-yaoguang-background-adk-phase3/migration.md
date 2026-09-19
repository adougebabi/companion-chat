# 摇光项目第三阶段迁移与验收记录

日期：2026-09-19

## 1. 前置基线

- 第一阶段 Eino 基础提交：`89764bc`。
- 第二阶段 ADK 对话实现：`7a47b66`；任务归档：`bcf83ad`；session journal：`26aaca2`。
- 当前实现基于第二阶段工作树 `codex/yaoguang-adk-phase2`，没有重建 Provider、Composer、CapabilityRegistry 或 Temporal。
- 第二阶段已有真实 Eino/ADK model → tool → ToolResult → model 闭环测试；本阶段在同一运行基础上增加 WakeUp schema 和 surface-aware structured-task 边界。

## 2. 后台任务分类

| 任务 | 真实入口 | 是否需要工具反馈 | 当前限制 | 本阶段决定 | 代码依据 |
| --- | --- | --- | --- | --- | --- |
| WakeUp | `WakeUpWorkflow → ProcessWakeUpActivity → App.ProcessWakeUp` | 是；允许模型选择已授权 Capability，并在纯查询结果返回后继续一次 bounded decision | `wake_up_response`、WakeUp surface、自治策略、冻结/intent/outbox | 迁移到共享 ADK | `internal/workflow/workflow.go`、`internal/core/wakeup.go` |
| Daily Review | `DailyReviewWorkflow → ProcessDailyReviewActivity → App.ProcessDailyReview` | 否；工具调用是冻结后的执行声明，不把结果回填给模型 | advisory lock、Continue-As-New、daily-review policy | 保留单次 `RunStructuredToolsTask` | `internal/core/autonomy.go` |
| Native Cognition | `ProcessCognitionActivity → ProcessCognitionInbox → ProcessNativeCognitionFact` | 否；Capability 结果在 freeze 后由 Core settlement | depth guard、frozen replay、native fact | 保留单次 Task | `internal/core/cognition.go`、`cognition_growth.go` |
| Reflection | `ProcessReflectionActivity → ProcessReflection` | 否；Provider contract 明确拒绝 ToolCall | evidence window、watermark/CAS、no-tools | 保留单次无工具 Task | `internal/core/workflow_ops.go`、`reflection_runtime_v2.go` |
| Summary/Schedule/Media/Visual Identity/Embedding | 各自 ModelTask 或固定 workflow | 未发现真实模型反馈需求 | 持久调度、长任务和独立队列 | 保留既有 Task/workflow | 对应 `internal/core` 与 `internal/workflow` 模块 |
| Autonomy/Capability action worker | `ProcessAutonomyAction` / `ProcessCapabilityAction` | 不是模型规划；负责 durable side effect settlement | intent、租约、重试、幂等 | 保留固定异步 worker | `internal/core/workflow_ops.go` |

因此本阶段没有把“声明了 tools schema”或“拥有副作用”的任务机械改成 Agent。

## 3. WakeUp 真实调用路径

```text
WakeUpWorkflow
  -> ProcessWakeUpActivity
  -> App.ProcessWakeUp(ctx, fluctlightID, cycle)
  -> BuildContextProjectionFor(MemoryForWakeUp)
  -> assembleProjectionPromptForSurface(ProviderContextSurfaceWakeUp)
  -> App.RunADKStructuredTask
  -> Provider.StructuredAssembledWithToolsSchema
  -> Eino ChatModel + ADK Runner
  -> WakeUp-scoped Capability adapter
  -> WakeUp assessment normalization
  -> autonomy policy / bind / validate / freeze
  -> persistWakeUp transaction
  -> existing intent/outbox/action worker/next clock/reflection hint
```

`ProcessWakeUpActivity` 仍负责 activity lifecycle correlation、durable intent
绑定、cancellation marker、timeout 和 Temporal retry。`ProcessWakeUp` 仍负责
`wakeID = wake_up_<digest(fluctlightID:cycle)>`、稳定 action ID、Owner/direct
conversation、Projection、权限和提交。

## 4. 共享 ADK 边界与权限矩阵

共享边界位于 `internal/core/adk_conversation_runtime.go`：

- `ADKCapabilityRequest` 只带 Fluctlight/Conversation/source/action/correlation、surface 和 Projection；不把 `*App`、数据库或事务传给 Eino。
- `RunADKStructuredTask` 在 Provider I/O 前校验 Registry canonical definition、surface、重复项和 `InternalOnly`。
- `RunADKLoop` 是唯一的 Eino model/tool event iterator，最大两代、无隐式 retry/fallback，保留原始 ToolCall ID 和 ToolResult 配对。
- Provider 的 `queuedToolCallingChatModel` 为每个物理 Generate/Stream 重新取得 queue lease、attempt/request identity 和诊断行。

| Schema | Surface | ADK loop | 工具/结果规则 |
| --- | --- | --- | --- |
| `conversation_turn_response` | Conversation | 是 | Main 既有 Capability catalog；按既有 conversation settlement |
| `takeover_reply_response` | Conversation | 是 | B 重新构建 scope，不继承 A 的 trace/结果 |
| `wake_up_response` | WakeUp | 是 | 仅 `Catalog(CapabilitySurfaceWakeUp)`；纯查询可回填，deferred/mutation 不在 callback 提交 |
| `daily_review_response` | Autonomy | 否 | 保留单次冻结声明 |
| `native_cognition_response` | Native Cognition | 否 | 保留 cycle guard/frozen settlement |
| `reflection_proposal_v2` | Reflection | 否 | no-tools provider contract |
| `query_continuation_response` / `takeover_judgement_response` | 专用 | 否 | 既有 bounded no-tools path |

Conversation-only `memory.recall`、Autonomy-only `persona.*` 和所有
`InternalOnly` definition 都不会泄漏到 WakeUp。WakeUp 的 `conversation.reply`、
`moment.publish`、已安装的媒体/scene/presence 等能力仍通过现有 bind/prepare/
freeze/worker 边界执行。

## 5. 保持不变的业务规则

- 不改变 WakeUp 周期、`wake_up.current` recurrence、Temporal workflow/activity 名称、queue、timeout、retry budget、Redis hint 或 PostgreSQL authority。
- 不写 fake user message，不把 WakeUp 内部事件写入 conversation history，不从 ADK 中间事件直接发布消息或动态。
- `no_op` 仍是成功的生命周期结果，并继续写 wake fact、reflection intent、next clock；Provider/tool/cancel/iteration failure 不会被 catch 成 no-op。
- 自治策略、目标接收者、source fact、action ID、frozen invocation、intent/outbox、幂等 replay 和 cancellation fence 仍由 Core/worker 负责。
- deferred output 只表示待提交；最终私聊/Moment/media 仍由既有 action workflow settlement。
- Daily Review、Native Cognition、Reflection、Summary、Schedule、Media、Visual Identity、Embedding 没有新增 ADK 回合或权限。

## 6. 代码变化与清理

- 将 Conversation 专用命名的 ADK bridge 提取为 `ADKLoop*`、`ADKCapabilityRequest` 和 `RunADKStructuredTask`；Conversation Main/B 通过同一边界继续运行。
- WakeUp 从旧的单次 `RunStructuredToolsTask` 入口切换到共享边界；没有保留旧 WakeUp Provider fallback 或第二套 BackgroundAgent engine。
- Provider/Eino gate 的 allowlist 增加 `wake_up_response`；Query/Judge、Daily Review、Native Cognition、Reflection 仍走各自原有 Task。
- 更新 prompt-composer 源码守卫，使 WakeUp 明确要求 `RunADKStructuredTask`，其 Composer/Projection 仍由 WakeUp 自己拥有。

## 7. 已执行验证

在 `apps/core-go`：

```text
go test -mod=readonly ./internal/core                         PASS
go test -mod=readonly ./...                                   PASS
go test -mod=readonly -race ./internal/core                   PASS
go vet ./...                                                   PASS
go test -mod=readonly ./internal/core -run 'TestWakeUp|TestProviderWakeUp|TestAppADKInvokerUsesWakeUpSurface|TestADKLoop|TestProductionMainCallersUseOnlyPromptContextAssembler' -count=1  PASS
```

专项证据包括：

- Provider-level WakeUp 两次生成：第一轮正式 ToolCall，第二轮消息包含同一 assistant call 与匹配 `tool_call_id` 的 ToolResult。
- WakeUp surface/correlation/正式 call ID 保留，以及 deferred result 不在 callback 内提交。
- ADK model failure、tool failure、context cancellation、iteration cap 不产生 fabricated final message。
- WakeUp/Conversation ADK schema allowlist，以及 Daily Review/Native Cognition/Reflection 明确不进入 loop。
- Registry unknown、duplicate/mismatch、cross-surface、`InternalOnly` definition fail-closed。
- 既有 Conversation Main/B ADK 回归和 full Core package tests 通过。

当前环境 `GO_CORE_TEST_DATABASE_URL` 未设置，因此依赖临时 PostgreSQL 的真实
`ProcessWakeUp` 事务入口、冻结/intent/outbox replay 和两轮诊断行集成测试按项目
测试基座 skip；没有把 Fake Provider 结果描述成 PostgreSQL 已验证。已有
WakeUp/Core integration tests 与真实入口测试在设置该变量后应作为最终环境门禁运行。

补充工程门禁结果（2026-09-19）：

- Core `go test -mod=readonly ./...`、Core `go test -mod=readonly -race ./internal/core`、Core `go vet ./...`：通过。
- Gateway `go test -mod=readonly ./...`、Gateway `go test -mod=readonly -race ./internal/bff`、Gateway `go vet ./...`：通过。
- Core `go mod tidy -diff`、`gofmt -l apps/core-go/internal/core apps/gateway-go`、`git diff --check`：通过。
- 旧路径静态扫描仅保留 `RunADKLoop`、三类 ADK schema 和既有单次任务 schema 的合法引用；没有 `RunADKConversation`、`BackgroundAgent` 或 WakeUp 旧 fallback。
- `GO_CORE_TEST_DATABASE_URL` 当前未设置；依赖临时 PostgreSQL 的真实 `ProcessWakeUp` 事务/冻结/intent/outbox/replay 门禁仍按测试基座 skip，不能由 Fake Provider 结果替代。

## 8. 新增后台行为的扩展点

新增后台行为只需在自身模块声明：

1. operation-owned input 与 `ProviderContextSurface`；
2. Composer rules/slot 和 schema；
3. `CapabilityRegistry` surface catalog 与服务端 candidate validation；
4. 领域结果 normalization、policy、freeze、transaction/intent settlement。

只有出现真实的“工具结果决定下一次模型判断”需求时，才把 schema 加入
`isADKLoopSchema` allowlist，并补齐两代上限、ToolCall pairing、取消、失败、
权限和不发布中间事件的测试。否则继续使用既有单次 ModelTask/fixed workflow。
