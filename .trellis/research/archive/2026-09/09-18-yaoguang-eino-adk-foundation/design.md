# 技术设计：Eino 基础层与 ADK 对话运行时

## 1. 边界与目标

实现位于 `apps/core-go/internal/core`，对外仍暴露现有 Core/BFF 业务协议。新基础层分成四个窄边界：

1. `ModelConfig/ModelFactory`：从已解析的 `providerAssignment` 构造 Eino ChatModel/Embedder，负责 capability 校验、provider-specific payload modifiers 和 model identity。
2. `ModelCallSupport`：包住一次 Eino Generate/Stream/EmbedStrings 调用，统一队列许可、取消 marker、timeout、usage/diagnostic correlation、preflight 和失败语义；不保存 Agent 长生命周期状态。
3. `PromptComposer`：把业务提供的 `PromptTaskInput` 和显式 Slot/Fragment 选择转换成 Eino `schema.Message`，保留现有预算和 redaction trace；不访问 App/DB。
4. `ConversationRuntime`：以 ADK Runner/ChatModelAgent 推进一轮对话，将 ADK tool schema/event 适配到现有 CapabilityRuntime，并把最终结果交回当前冻结/settlement 管线。

单次任务通过 `ModelTask` 使用 1/2/3；只有直接对话使用 4。WakeUp、Reflection、Summary、Media、Visual Identity 等不得为了形式统一而进入 Agent loop。

## 2. 模型配置和调用支撑

`providerAssignment` 继续是配置读取层的业务快照，新增 `EinoModelConfig` 只承载构造所需的 endpoint/model/secret/timeout/budget/response-format/provider extras。`ModelFactory`：

- 调用官方 `openai.NewChatModel` / `openai.NewEmbedder`；不自己 POST 或解析 OpenAI wire。
- 用 `openai.WithRequestPayloadModifier`/`WithExtraFields` 注入已验证的 `enable_thinking`、strict schema、idempotency/correlation headers 所需字段；任何 provider 不支持的能力返回明确错误。
- 对 `schema.Message.ToolCalls` 只做一次 bounded adapter，再交给现有 `NormalizeProviderToolCalls` 与 `CapabilityInvocation.Validate`。
- 将 `schema.Message`、usage 和 stream close/error 收敛成内部 `ProviderCompletion`/`TaskResult`，但不新增另一套可持久化 tool envelope。

调用支撑每次 acquire 一次本地/Redis queue lease，设置单次 timeout 和 cancellation watcher；ADK Agent 不持有 lease，工具回合结束后释放，下一次模型调用重新申请。retry/failover 默认关闭，避免与 Temporal、ADK、Provider 自身重试叠加。

## 3. Prompt Slot/Fragment/Composer

新增值对象：

```text
PromptSlotID, PromptSlotPosition(system/runtime/recent/current/tools/schema),
PromptSlot{ID, Required, Order, Budget, ContentSource},
PromptFragment{SlotID, GroupKey, Role, Content, Priority, EstimatedTokens, SourceRefs}
```

`PromptComposer.Compose(ctx, input)` 的 `ctx` 只用于取消/预算，不含 App。Task 显式声明 slots，例如：

- Main：core persona → operation rules → bounded runtime facts/memory → recent whole turns → current input → tools/schema。
- Takeover B：reply-owner persona + takeover control context + current input + authorized tools。
- Reflection/Summary：各自的 evidence slots，不带 Main persona/context。
- Media Quality/Visual Identity：text + one bounded image part，不共享 Main conversation slots。

Composer 输出 `[]*schema.Message` 与 bounded `PromptAssemblyTrace`。现有 `PromptContextAssembler` 的预算选择、dedup、role 保留和 required overflow 错误迁移到 Composer 内部；旧 `composeProviderMessages` 只能保留为测试/迁移辅助，不能被生产 Task 调用。

## 4. Task 与调用清单

| 旧入口 | 新 Task/运行模块 | Eino 路径 | 关键验证 |
| --- | --- | --- | --- |
| `mutations.go` Main | `ConversationRuntime.RunMain` | ADK Runner + ChatModelAgent | tool 回填、一次发布、冻结 |
| query continuation | `QueryContinuationTask` | ChatModel.Generate，无 tools | 1 次、pure query、digest |
| persistent switch assessment | `PersistentSwitchTask` | ChatModel.Generate，独立 schema | 当前 reply 不替换 |
| takeover Judge | `TakeoverJudgeTask` | ChatModel.Generate，无 tools | deterministic trigger、A fallback |
| takeover B | `TakeoverReplyTask` | bounded ADK/Generate policy | B scope、无 persistent switch |
| WakeUp/Daily Review/native cognition | `StructuredTask` | ChatModel.Generate + tool schema adapter | durable action owner |
| Reflection/Summary/Schedule | `StructuredTask` | ChatModel.Generate | evidence/schema/CAS |
| Media Prompt | `TextTask` | ChatModel.Generate | prompt persistence |
| Media Quality/Vision/Patch | `MultimodalStructuredTask` | ChatModel.Generate | image bounds/checkpoint |
| Initialization | `InitializationTask` | ChatModel.Generate json schema | large budget/source digest |
| memory embedding/retrieval | `EmbeddingTask` | Eino Embedder.EmbedStrings | pin/model/dimensions/fallback |
| existing Provider SSE | `StreamTask` | ChatModel.Stream aggregation | settlement-after-stream semantics |

## 5. ADK conversation flow

`ConversationRuntime` builds a request-scoped agent with the current PromptComposer output and a narrow list of allowed Eino `tool.BaseTool` schemas. Each adapter receives a request-scoped `CapabilityExecutionContext` containing frozen identity and a callback into `CapabilityRuntime`; it never receives `*App` or raw DB handles.

Flow:

```text
HandleTurn auth/idempotency/projection
  -> PromptComposer(Main)
  -> ModelCallSupport acquires one lease
  -> ADK Runner.Query/Run
  -> ChatModelAgent emits model event
  -> approved tool adapter validates + executes CapabilityRuntime
  -> tool result appended by ADK to the next model input
  -> runtime policy enforces max generation/continuation
  -> canonical visible output + invocations/results
  -> existing frozen decision + settlement transaction
  -> post-commit single token/completed frame
```

ADK checkpoint stores only bounded event metadata and are scoped to the turn; no global mutable Agent stores current persona/candidate. Tool adapters return rejected/failed results as explicit errors/events, never empty success. `conversation.reply` is converted to the same canonical visible-output authority used by existing settlement and is not independently published.

Takeover remains an explicit deterministic composition: validate A → Judge Task (if required) → rebuild Composer for B → one B generation → winner-only freeze/prepare/settle. It is not an ADK nested Agent and B cannot call Judge or persistent switch.

## 6. Migration/removal strategy

First replace the body of the current provider completion boundary with Eino model calls while retaining the existing `ProviderCompletion` contract and test construction seam. Then move prompt assembly and conversation orchestration behind the new Composer/Runtime. Migrate each non-conversation task through the shared support. Finally remove direct `http.Client` model requests, old SSE parser, generic hand-written tool loop/JSON sidecar compatibility and unused prompt composition helpers; keep only the narrow test fake/model seam.

No BFF/Temporal/domain schema change is required. Configuration docs must state required ChatModel tool-calling, strict structured-output, multimodal and embedding capabilities; unsupported assignments fail at preflight.

## 7. Failure, rollback and observability

- Before each production call, record bounded metadata-only preflight (role, scenario, assignment/model digest, prompt digest, slot trace digest, budget); never raw prompt/response/reasoning.
- Any Eino error, malformed message, tool rejection, cancellation, timeout or iteration limit returns a typed failure and leaves frozen/settlement state retryable; no empty success.
- Rollback point is per task boundary: keep existing domain settlement and switch back to the previous commit if adapter tests fail. Do not add runtime feature flags or dual provider paths.
- Real provider and external media acceptance remain opt-in and are reported separately from Fake Eino tests.

## 8. Compatibility decisions

- `ProviderClient` remains the internal composition-root name only where existing tests and queue wiring require it; its production completion implementation becomes Eino-backed and no longer owns raw model HTTP parsing. New code consumes `ModelRuntime`/Task interfaces; no new business caller is added to legacy provider methods.
- Existing `ProviderCompletion`, `CapabilityInvocation` v2, frozen stages and BFF frames remain the authoritative contracts during the mechanical migration. They are not a second model protocol.
