# 阶段二迁移与验收记录：ADK 对话运行接入

## 1. 前置条件

阶段一基线已在分支 `codex/yaoguang-eino-adk` 完成，阶段二工作树从该基线继续：

- 阶段一主要提交：`89764bc`、`d82daf8`。
- 阶段二开始前基线提交：`de2d73e`。
- 阶段二分支：`codex/yaoguang-adk-phase2`。
- 工作树：`/Users/vinson/Documents/project/个人/local-ai-companion-yaoguang-eino-adk`。
- `master` 当前仍位于独立工作树；本阶段没有覆盖或重置用户已有修改。

阶段一已经提供 Eino factory、Prompt Composer、ModelTask、生成/Embedding queue、取消超时和 diagnostics。本阶段没有重新实现这些基础设施，也没有改动 BFF/Temporal/前端协议。

## 2. 真实调用路径

```text
HandleTurn / takeover stage
  -> projection + authorization + Composer
  -> ConversationRuntime (request-scoped)
  -> Provider Eino boundary
  -> ADK ChatModelAgent + Runner
  -> formal ToolCall
  -> CapabilityRegistry -> ADK tool adapter -> CapabilityRuntime
  -> bounded result + matching tool_call_id/name
  -> ADK next Generate (fresh queue/diagnostic/provider attempt)
  -> normalized ProviderCompletion
  -> frozen decision / Prepare / settlement / single publish
```

`RunQueryContinuation` 和 `RunTakeoverJudge` 仍是一次、无工具、无 ADK bridge 的专用模型任务；它们不参与通用模型—工具循环。Main 和 takeover B 都通过同一 `isADKConversationSchema` gate 进入 ADK，B 的 `takeover_reply_response` 不再退回普通单次 Generate。

## 3. 代码迁移清单

| 旧入口/职责 | 当前入口 | 证据 |
| --- | --- | --- |
| Main `RunMainConversationTask` facade | `ConversationRuntime.RunMain` | `apps/core-go/internal/core/conversation_runtime.go`, `mutations.go` |
| Query continuation facade | `ConversationRuntime.RunQueryContinuation` | `conversation_runtime.go`, `mutations.go` |
| Takeover Judge facade | `ConversationRuntime.RunTakeoverJudge` | `conversation_runtime.go`, `turn_takeover.go` |
| Takeover reply facade | `ConversationRuntime.RunTakeoverReply` | `conversation_runtime.go`, `turn_takeover.go` |
| 应用层通用 tool loop | Eino ADK `ChatModelAgent`/`Runner` | `adk_conversation_runtime.go`, `eino_model_runtime.go` |
| 并行 Capability 目录 | `CapabilityRegistry.Catalog` | `tool_contract.go`; `InternalOnly` entry 不进入 model catalog |
| Eino ToolMessage wire name 丢失 | `normalizeEinoToolMessageNames` | `eino_model_runtime.go` |

Reflection、WakeUp、media、summary、Embedding 等独立任务没有被强制改成 Agent；它们继续使用阶段一 ModelTask/Eino 路径。

## 4. Capability 与人格动作

- ADK adapter 只接受 Registry 提供的 definition，并在 runtime 边界校验 caller definition 与 Registry 的完整一致性。
- `compose.GetToolCallID(ctx)` 的正式 ID 原样写入 `CapabilityInvocation.CallID`、`CapabilityResult.CallID` 和后续 ToolMessage；缺失 ID fail-closed，不按 name/arguments 补造。
- 只读 capability 在 adapter 内执行 bounded query 并返回真实结果；mutation/external capability 返回 `deferred`，后续仍由现有 frozen/Prepare/transaction/intent 机制提交。
- `persona.takeover` 与 `persona.switch` 是 `CapabilityTypeInternal + SideEffectClass=policy + InternalOnly`。确定性策略通过 `executePersonaPolicyAction` 发起，`Metadata.Source=policy`，稳定 policy call ID，不伪造模型原生 ToolCall。
- Takeover Judge、persistent-switch grant、B 禁止持久切换、A 废弃候选不提交等既有规则保持不变。Policy audit 嵌套在 takeover/decision frozen payload，native model trace 与 policy trace 不混用。

## 5. Frozen payload 与失败边界

- B overwrite 写入 B 自己的 `capability_invocations` / `capability_results`，并清理 A 的候选衍生上下文。
- overwrite 对空 slice 做 non-nil normalization，因此 JSON 中始终是 `[]` 而不是 `null`。
- ADK iteration 上限为 2；model/tool failure、取消或空 final 都返回 bounded error，不发布 assistant，不制造成功结果。
- 每个 ADK 物理 Eino `Generate`/`Stream` 独立进入生成 queue、独立诊断 row、独立 provider attempt/request ID，同时保留 parent turn correlation。

## 6. 证据测试

已加入/保留的关键测试：

- `TestProviderConversationUsesADKLoopAndReturnsCanonicalTrace`
- `TestProviderConversationWithoutToolsStillUsesADKRunner`
- `TestProviderTakeoverReplyUsesADKLoopAndPreservesTrace`
- `TestRunADKConversationExecutesCapabilityAndFeedsResultBack`
- `TestRunADKConversationToolFailureHasNoFinalMessage`
- `TestRunADKConversationStopsLoopAtTwoGenerations`
- `TestNewADKCapabilityToolsRejectsInvokerWithoutFormalIdentity`
- `TestConversationRuntimeRequiresCapabilityContextForModelCatalog`
- `TestConversationRuntimeRejectsCallerSchemaDivergenceBeforeProviderIO`
- `TestPersonaPolicyCapabilitiesStayOutOfConversationCatalog`
- `TestPersonaPolicyInvocationUsesStablePolicyIdentity`

真实 PostgreSQL 入口/settlement/diagnostics 测试由 `GO_CORE_TEST_DATABASE_URL` 控制；未配置时会按项目既有规则 skip，不能把 skip 解释为真实数据库全链路通过。真实 Provider、ComfyUI、计费/缓存服务仍需外部凭据，不以 Fake 测试替代兼容性结论。

## 7. 删除与静态扫描

已删除旧的 `RunMainConversationTask`、`RunTakeoverReplyTask`、`RunTakeoverJudgeTask` facade 及其生产调用。`HandleTurn` 仍是业务入口，但不再拥有通用模型—工具交替循环。最终扫描只允许业务 `HandleTurn`、明确的 `StructuredQueryContinuation` boundary 和 ADK 内部事件迭代。

## 8. 最终验证记录

2026-09-19 最终门禁已执行并通过：

```bash
gofmt -w apps/core-go/internal/core
go -C apps/core-go test -mod=readonly ./...
go -C apps/core-go test -mod=readonly -race ./internal/core
go -C apps/core-go vet ./...
go -C apps/gateway-go test -mod=readonly ./...
go -C apps/gateway-go test -mod=readonly -race ./internal/bff
go -C apps/gateway-go vet ./...
go -C apps/core-go mod tidy -diff
git diff --check
```

结果摘要：

- Core `go test ./...`：通过。
- Core `go test -race ./internal/core`：通过。
- Core `go vet ./...`、Gateway 全量 test、Gateway BFF race、Gateway vet：通过。
- Core `go mod tidy -diff`、`gofmt`、`git diff --check`：无差异/通过。
- 未设置 `GO_CORE_TEST_DATABASE_URL` 的 PostgreSQL 集成用例按既有规则 skip；没有将 skip 记为真实 PostgreSQL 全链路通过。

旧路径扫描：

```bash
rg -n --glob '*.go' --glob '!**/*_test.go' \
  'for .*Tool|StructuredQueryContinuation|HandleTurn\(|Provider\.(Structured|Text|Embed|StreamText)|RunMainConversationTask|RunTakeoverReplyTask|RunTakeoverJudgeTask' \
  apps/core-go/internal/core
```

扫描结果：生产源码中没有旧三类 task facade，也没有旧 Provider raw `/chat/completions` 或 `/embeddings` 请求构造；保留的 `HandleTurn` 是业务入口，`StructuredQueryContinuation` 是明确的 query boundary，ADK 事件迭代位于 ADK runtime 内部。
