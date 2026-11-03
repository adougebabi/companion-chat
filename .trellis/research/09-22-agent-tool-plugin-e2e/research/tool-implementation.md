# Tool 独立执行切片进度

更新：2026-09-22

## 已落盘的正式边界

生产入口为：

```go
func (a *App) ExecuteTool(
    ctx context.Context,
    request ToolExecutionRequest,
) (ToolExecutionReceipt, error)
```

`ToolExecutionRequest` 显式包含 `CapabilityName`、稳定业务 `OperationID`、`AuthorizationActorID`、`FluctlightID`、可选 `ConversationID`、业务 `EvidenceID`、参数 JSON，以及仅在模型真实调用时存在的 `NativeToolCallID` / `ProviderRequestID`。直接调用不构造 cognition inbox、ActionID 或 Provider 请求事实；本地 `ExecutionCallID` 只用于一次执行的结果关联，`Metadata.Source=direct` 清楚标识来源。

同一个入口也可由后续 Eino adapter 复用：传入真实 `NativeToolCallID` 与 `ProviderRequestID`，同时保留独立的稳定 `OperationID`。业务幂等优先使用 `OperationID`；旧冻结 payload 没有该字段时才回退到原 `CallID`。

执行顺序为：所有权校验 → 唯一参数 schema 校验和 context/Prepare → 按 execution class 执行。查询直接调用正式 capability；事务写在 `App.ExecuteTool` 内开启并提交短事务，调用方不再提供事务。Schedule 等 Prepare/Provider 工作发生在事务外。依赖错误通过原 error chain 返回，同时 receipt 保留规范化失败结果。

## 当前能力状态

已完成并有专门实现测试的第一条链路：

- `memory_event`：通过正式 `memoryEventCapability`、`prepareMemoryCapability` 和 `applyMemoryCommandTx` 执行，Tool 自己拥有短事务并真实提交 Memory、revision、embedding intent/outbox。业务幂等键绑定 `OperationID`，native ToolCall ID 改变不会重复写；同 operation 不同参数仍由既有 digest ledger 拒绝。
- `memory.recall`：通过正式 `memoryRecallCapability` / `MemoryRecallService` 查询刚提交的权威 Memory；不启动 Main，不构造 Agent 状态。
- 测试还断言直接调用不会创建同名 cognition inbox 事实，并为无 operation/授权主体/目标资源提供参数负例。

通用边界已经能为现有 pure query 和 transactional capability 提供自有事务，因此 `relationship.lookup`、`scene_event`、`presence_event`、`schedule.replan`、`affect_event`、`capability.request` 和 `visual_identity.initialize` 已有同一正式调用入口。`visual_identity.initialize` 的 `wake_up_` source 前缀门禁已移除；真实 WakeUp 仍记录 `wakeup` trigger，其他直接业务命令记录 `initialization` trigger。这些能力尚未在本切片补齐逐项真实数据库用例，不能计为 Tool E2E 已通过。

同时更新了 schedule、active-memory、capability-request 和 media intent 的业务身份派生，使后续迁移不再只绑定 native `CallID`。active-memory 的旧 Prepare 仍要求 frozen action/context-reference/cognition inbox，尚未完成独立输入改造，因此当前直接调用会明确失败，不能算完成。

## 尚未完成

- `conversation.reply` / `moment.publish`：仍需提取正式发布服务，在 Tool 自有短事务中创建 message/Moment 与 outbox，并防止后续生产调用方重复发布。
- `media.image.generate`：仍需让直接请求携带明确 target，由 Tool 自有事务创建 durable media intent；规范结果需使用真实 `accepted` 与 task/intent ID，而不是 `completed` 或 deferred。
- `active_memory_event`：仍需用显式 evidence time、actor 与 target ID/revision 替代 frozen action/context-reference/cognition inbox 前提。
- `persona.takeover` / `persona.switch`：仍需接入实际领域提交/拒绝及审计，当前 capability 仍只返回 `awaiting_domain_commit`。
- 其余已有通用入口的 Tool 仍需逐项成功、业务失败、独立产物、依赖异常、重复语义和 Eino adapter 等价测试。
- 后续 Agent 集成需要在不改参数解析/执行/结果序列化的前提下直接调用 `ExecuteTool`；模型 adapter 必须提供真实 native call/provider request identity 和独立 operation ID。

## 修改文件

- `apps/core-go/internal/capability/capability.go`
- `apps/core-go/internal/core/capability_core.go`
- `apps/core-go/internal/core/tool_execution.go`（新增）
- `apps/core-go/internal/core/tool_execution_test.go`（新增）
- `apps/core-go/internal/core/memory_intelligence.go`
- `apps/core-go/internal/core/active_memory.go`
- `apps/core-go/internal/core/builtin_capabilities.go`
- `apps/core-go/internal/core/capability_requests.go`

说明：新边界复用同 package 的 `CapabilityRuntime` 装配，但本轮没有修改 `capability_runtime.go` 的旧调用路径或 Main settlement。没有修改 `internal/ai/agent`、`eino_model_runtime.go`、`provider.go`、`model_tasks.go`、`conversation_runtime.go`、`mutations.go`、`wakeup.go`、`turn_takeover.go`。

## 验证记录

- `gofmt`：已执行，成功。
- `go build ./internal/core ./internal/capability`：exit 0。
- `go build ./...`（`apps/core-go`）：exit 0。
- `go vet ./internal/core ./internal/capability`：exit 0。
- `go test ./internal/core -count=1`：exit 0；普通 Core 测试通过，但数据库测试因当前环境未配置 `GO_CORE_TEST_DATABASE_URL` 而按既有 helper SKIP，不能作为真实 DB 通过证据。
- `go test ./internal/core -run 'Test(Capability|MemoryRecall|ToolExecution)' -count=1`：exit 0。
- `go test ./internal/core -run 'TestToolExecutionRequestRejectsMissingBusinessIdentity|TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay' -count=1 -v`：参数负例通过；真实 PostgreSQL 链路明确 SKIP，原因是 `GO_CORE_TEST_DATABASE_URL is not set`。该行验收状态仍为未执行，而非通过。

`TestCapabilityRuntimePreservesDirectDependencyCause` 另行验证了直接查询依赖错误可由 `errors.Is` 找回原 cause，不会被规范化结果吞掉。

真实 PostgreSQL 用例 `TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay` 使用现有 `isolatedCoreTestRepository` 创建一次性数据库，验证提交产物、operation replay、native call ID 分离、同键异参冲突、无伪 cognition fact 以及 recall 可见性。环境变量可用后应直接运行该用例并保留退出码/日志。
