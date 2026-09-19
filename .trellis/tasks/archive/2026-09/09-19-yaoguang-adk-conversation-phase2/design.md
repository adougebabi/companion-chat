# 第二阶段技术设计：ADK 对话运行与人格动作接线

## 1. 前置与边界

第一阶段已经提供 Eino factory、Eino-backed Provider completion、Prompt Composer、ModelTask wrapper、per-call queue/diagnostics 和 request-scoped ADK loop。第二阶段只改变对话编排边界：

```text
HTTP/Worker business entry
  -> ConversationRuntime request input
  -> ADK Runner/ChatModelAgent
  -> CapabilityRegistry adapter
  -> CapabilityRuntime / deferred plan
  -> existing freeze/Prepare/settlement/publish
```

`ConversationRuntime` 不拥有数据库事务、持久状态或全局 Agent；它接收窄的业务 input/context，并返回规范化的候选、tool trace、最终消息和运行结果。

## 2. ConversationRuntime contracts

```go
type ConversationRuntime interface {
    RunMain(context.Context, ConversationInput) (ConversationRunResult, error)
    RunQueryContinuation(context.Context, QueryContinuationInput) (ConversationRunResult, error)
    RunTakeoverReply(context.Context, TakeoverReplyInput) (ConversationRunResult, error)
}
```

Judge 与 persistent-switch assessment 继续是明确的非递归 ModelTask/阶段调用，不变成模型可自愿调用的通用工具。

`ConversationRunResult` 只包含 request-scoped `schema.Message`、canonical `CapabilityInvocation`/`CapabilityResult` trace、final structured candidate 和 bounded diagnostics；不会进入领域实体或 BFF DTO。

## 3. ADK tool adapter and identity

- `CapabilityDefinition` 仍是唯一 tool schema/catalog owner。
- `ADKCapabilityTool` 从 Eino `compose.GetToolCallID(ctx)` 取得正式 ID，并通过 optional `ADKCapabilityInvokerWithID` 传递；不能根据 name/arguments 重造 ID。
- Tool adapter 负责调用窄 `CapabilityInvoker`，不持有 App/DB/transaction。
- Query capability 可以立即执行并返回 bounded output；mutation/deferred/external capability 只返回明确 deferred/rejected/failed 状态，后续交由现有 frozen action/Prepare/settlement。
- trace source 显式区分 `model_tool` 与 `policy`；native ToolCall 不由策略伪造。

## 4. Persona action bridge

新增的 persona action capability 只作为现有领域操作的正式入口：

- `persona.takeover`：确定性 Judge 通过 policy source 发起，更新本轮 reply owner/frozen candidate，不改 active profile。
- `persona.switch`（内部执行入口，不暴露给普通模型 catalog）：仅 Main/assessment 授权后在原有 transaction gate 应用 active profile 变更。

这两个入口复用 `applyTurnTakeover`、`preparePersonalityDecision`、`applyPersistentSwitchIfAuthorizedTx` 和已有审计/冻结记录，不复制人格规则，也不允许 B/WakeUp/Reflection 获得权限。

## 5. Queue, diagnostics and cancellation

Outer ADK Runner 不持有 model queue lease。每次 ChatModel Generate/Stream：

1. 派生 child request/attempt ID，保留 parent turn correlation；
2. 插入独立 queued model-run row；
3. 通过第一阶段 queue/Redis/cancellation/timeout 执行一次模型调用；
4. 写 usage/latency/terminal state；
5. 释放 lease 后才允许下一次 ADK model call。

ADK cancellation、tool failure、iteration limit 和 parent supersession 必须让当前 model/tool context 结束，并在 Core 侧检查 frozen/inbox authority。

## 6. Prompt/message rebuild

每个 ADK model call 都通过第一阶段 Composer 重新选择 current stage slots。Tool result message 必须使用正式 assistant tool-call 的 `ID`，随后使用同 ID 的 `tool_call_id` 和 tool name。重建时 system 只出现一次，user/current input 不重复，工具调用和结果成对保留，rejected A 的消息/trace/候选/结果不进入 B，B 使用 reply-owner persona/context/tool catalog，预算裁剪以完整 tool-call/result 对为单位。

## 7. Rollback/compatibility

不添加 runtime feature flag 或新旧双轨。每个迁移 slice 以现有 ProviderCompletion/CapabilityInvocation v2/frozen payload 为回滚边界；失败时保留原有业务提交契约。只删除确认不再被任何生产入口引用的旧控制流。
