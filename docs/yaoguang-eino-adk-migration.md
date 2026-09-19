# 摇光 Eino 基础层与 ADK 对话接入

## 交付边界

Core Go 现在以官方 Eino 组件作为模型传输边界：

- `github.com/cloudwego/eino` `v0.7.37`
- `github.com/cloudwego/eino-ext/components/model/openai` `v0.1.13`
- `github.com/cloudwego/eino-ext/components/embedding/openai` `v0.0.0-20260916065400-2607f61e807f`
- `github.com/eino-contrib/jsonschema` `v1.0.3`

由于当前构建环境是 Go 1.26，Eino 的 JSON runtime 依赖固定到兼容 Go 1.26 runtime layout 的 `github.com/bytedance/sonic v1.15.4`（以及 loader `v0.5.2`）。Core module 仍以 `go 1.25.4` 声明兼容下限。

Eino 只负责模型/消息/工具/Embedding 协议；Core 仍拥有配置角色、预算、队列、取消、权限、冻结、事务、Outbox/Temporal 和诊断。浏览器接入由同一 API 进程的 `internal/httpapi/browser` 边界负责，没有新增网关进程或领域状态协议。

## 旧入口到新路径

| 旧入口 | 新任务/运行模块 | 新路径 |
| --- | --- | --- |
| `mutations.go` Main | request-scoped ADK conversation runtime | Eino `ChatModelAgent` + `Runner` → Capability tool adapter → tool result → next model call → existing frozen/settlement |
| Query continuation | `StructuredQueryContinuation` task | Eino ChatModel `Generate`，无 tools，最多一次 |
| Takeover Judge / B reply | `StructuredAssembledJudgement` / assembled task | Eino ChatModel，Judge 无 tools；B 重新使用 takeover prompt scope |
| WakeUp / native cognition / Daily Review | `RunStructuredToolsTask` | Eino ChatModel + bounded Capability schema |
| Reflection / Summary / Schedule | `RunStructuredTask`/assembled task | Eino ChatModel，按各自 schema/scenario |
| Media Prompt | `RunTextTask` | Eino ChatModel 文本生成 |
| Media Quality / Visual Identity Vision/Patch | multimodal `RunStructuredTask` | Eino Message `UserInputMultiContent` + structured output |
| Initialization | `RunInitializationTask` | Eino ChatModel JSON-object 模式，保留初始化预算/诊断规则 |
| Memory Embedding / retrieval embedding | `RunEmbeddingTask` / embedding task | 官方 Eino `Embedder.EmbedStrings`，独立 embedding queue |
| Provider SSE | `StreamText` | 官方 Eino `ChatModel.Stream` 聚合；仍在 settlement 后向 Core NDJSON 发布 |

## Prompt Slot / Composer

`prompt_slots.go` 增加纯函数式 `PromptComposer`、`PromptSlot` 和显式位置/顺序/预算字段。Composer 只接收已经解析的业务事实，不持有 `*App`，不访问数据库或 Redis。已有 `WorkingMemory` 的 whole-turn 选择、current input 去重、预算和 trace 仍是唯一选择算法；Prompt Fragment、Capability ContextSlot、持久 typed evolution slot 不是同一个 map。

Main 与 takeover B 每次调用都重新生成 Eino `schema.Message`；B 不沿用 A 的 system prompt 或工具权限。

## ADK 工具与事务边界

`NewADKCapabilityTools` 将 `CapabilityDefinition` 转成 Eino `tool.BaseTool`，只接收 request-scoped `ADKCapabilityInvoker`。它不接收 `*App`、DB、Redis 或事务：

- 纯查询能力可以在 ADK tool 回合执行并将真实 bounded result 回填给下一次模型输入；结果进入 canonical `ADKCapabilityTrace`，后续被 Core 的 `CapabilityResult` 去重/settlement 使用。
- 变更/外部能力返回明确 `deferred` result，继续由 frozen action 的 Prepare、事务/CAS 和外部 intent 结算；ADK 不跨 LLM 调用持有数据库事务。
- tool call ID、schema version、source/action identity 仍由 `CapabilityInvocation` v2 约束；模型不会直接写领域状态。

ADK Runner 禁用隐式 retry/failover，Main 迭代上限为 2。浏览器仍只在提交后的 Core NDJSON boundary 收到最终 token/completed frame。

ADK tool adapter 从 `compose.GetToolCallID(ctx)` 保留模型正式的 tool-call ID；每个底层 Generate/Stream 都有独立 queue lease、Provider request ID 和 `diagnostic_model_runs` row，共享的只是父 turn correlation。

## 删除/替代内容

- `ProviderClient` 的生产结构化、文本、流式和 Embedding 请求不再手写 `/chat/completions` 或 `/embeddings` HTTP/JSON 解析；统一由 Eino OpenAI ChatModel/Embedder 发起。
- 旧 Provider SSE scanner、旧 embedding response envelope 和旧 chat response envelope 已从生产调用路径移除。
- Main 的模型—工具续接由 ADK Runner 推进，Core 不再在对话入口外另写通用 tool loop。
- 旧的 CapabilityRuntime、冻结 payload、权限/事务和现有 redacted diagnostics 没有被复制成第二套协议；它们仍是正式业务 authority。

## 模型与配置要求

模型角色必须显式配置并通过既有 preflight：

- Main/工具回合：ChatModel tool calling + structured output；多工具 payload 使用 `parallel_tool_calls`。
- Query continuation / Judge：ChatModel structured output，禁止 tools/thinking。
- Media Quality / Visual Identity：ChatModel multimodal input + structured output。
- Embedding：独立 Eino Embedder、固定 model/dimensions/revision；失败继续按 retrieval fallback 处理。
- Initialization：继续使用 JSON-object 模式及 8,192 output reserve / ten-minute floor。

不支持所需能力时返回明确配置错误；没有 generic/model 静默回退、旧实现回退或隐式重试。

## 验证证据

已运行：

```text
cd apps/core-go && go test -mod=readonly ./...
go -C apps/core-go test -mod=readonly ./internal/httpapi/browser
cd apps/core-go && go vet ./...
go -C apps/core-go vet ./...
```

另有 Eino/Fake 契约测试覆盖：官方 ChatModel 工具调用、官方 Embedder、ADK tool 回填到第二次模型输入、canonical invocation/result trace、Prompt Slot 选择与 whole-turn/current-input 约束。

尚未验证：需要真实凭据或外部服务的真实 OpenAI-compatible provider、真实 takeover Judge、ComfyUI 生成/投递、计费/cache/latency 线上行为。这些不能由 Fake Eino 测试冒充通过。
