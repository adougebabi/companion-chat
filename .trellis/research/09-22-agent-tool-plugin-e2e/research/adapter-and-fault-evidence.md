# Formal Tool adapter 与故障敏感性证据

日期：2026-09-22  
基线 HEAD：`32276a1c20ad68462b966c45c16ab6126230b297`  
测试类型：controlled regression + 隔离 PostgreSQL。没有调用 live LLM、GPU 或真实媒体 Provider。`FLUCTLIGHT_LIVE_PROVIDER_*` 在执行前显式移除；数据库仅通过 `GO_CORE_TEST_DATABASE_URL` 创建每用例随机临时库并在 cleanup 中删除。

## 实现文件

- `apps/core-go/internal/core/formal_tool_adapter_e2e_test.go`
  - SHA-256：`e724fe063154d6ec6489ce399c05ce5be91004820be21ad91e3a5e27179ddb2d`
  - 固定写死 18 项产品 Tool 清单，不从 registry 生成期望值。
  - 每一行先用同一 `ToolExecutionRequest` 直接调用正式 `App.ExecuteTool`，再由受控 OpenAI-compatible HTTP Provider 发出原生 `tool_calls`，经过正式 Eino `ChatModelAgent` / Runner、`NewADKCapabilityTools`、`appADKCapabilityInvoker` 回到 `App.ExecuteTool`。
  - mutation Tool 使用同一稳定 operation ID，adapter 路径必须 replay；两个 QUERY Tool 实际再次查询。所有行校验原始 native call ID、真实物理 Provider request ID、同一参数、正式 trace、第二次物理请求中的 `role=tool` 序列化 receipt，并用独立 SQL/查询结果核验业务资源。
  - `schedule.replan` 的 capability-local planner 也使用同一受控 HTTP Provider；响应依据当前日期和近实时 `completed_before` 构造，未使用 live 模型。
- `apps/core-go/internal/core/e2e_fault_sensitivity_test.go`
  - SHA-256：`e14565a760f7eeba17c4d9b187549b6dbd5f299a2e835fa32d9605fbfdcd1a41`
  - 所有 mutation 开关、故障 service 和 RoundTripper 只存在于 `_test.go`；生产文件没有故障 flag。

## 18 项正式 adapter 矩阵

固定子测试：

1. `active_memory_event`
2. `affect_event`
3. `capability.request`
4. `conversation.reply`
5. `media.image.generate`
6. `memory.recall`
7. `memory_event`
8. `moment.publish`
9. `persona.switch`
10. `persona.takeover`
11. `schedule.replan`
12. `presence_event`
13. `relationship.lookup`
14. `scene_event`
15. `visual_identity.initialize`
16. `visual_identity.generate_candidate`
17. `visual_identity.commit_review`
18. `visual_identity.finalize`

每项均成功观测两次物理模型请求。第一请求产生 native ToolCall；第二请求包含 Eino 回填的匹配 assistant ToolCall 与 `role=tool` receipt。物理 request ID 非空且两次不同；invocation/result 的 ProviderRequestID 对应产生 ToolCall 的第一请求。视觉四行按真实状态依赖串行执行：初始化 session、创建候选 media intent、将真实候选 asset 状态置为 ready 后提交 review、将 character-sheet asset 状态置为 ready 后 finalize；所有晋升与完成事实由正式 Tool 提交。

## 假成功写入破坏实验

父测试：`TestIndependentToolE2ERejectsFalseSuccessWithoutWrite`  
probe：`TestIndependentToolE2EFalseSuccessProbe`

同一 probe 始终调用正式 `memory_event` Tool 并要求 `completed`，随后独立执行：

```sql
SELECT count(*)
FROM public.memories
WHERE owner_fluctlight_id=$1 AND content=$2
```

故障子进程只在测试侧以 `testOnlyFalseSuccessMemoryService` 替换 `memoryEventCapability` 的业务依赖：Prepare 仍走生产实现，apply 跳过 Memory 写入却返回结构合法的 `completed`。因此 receipt 和 `tool_executions` 可看似成功，但原成功产物断言真实触发 `t.Fatalf`，子进程退出非零；父测试只在输出包含 `independent memory product assertion failed` 时接受该失败。随后移除 fault env，以生产 Memory service 运行完全相同 probe，必须退出 0。

## Tool result 回填破坏实验

父测试：`TestFormalAgentE2ERejectsBrokenToolResultFeedback`  
probe：`TestFormalAgentE2EBrokenToolResultFeedbackProbe`

probe 使用正式 `RunConversationCognitionAgent`、正式 prompt/context/model assignment、正式 Eino Runner、正式 `memory.recall`、随机隔离 PG 内容和受控 OpenAI-compatible HTTP Provider。首次模型请求已经被受控 Provider 捕获后，测试才通过正式 `memory_event` 写入随机 guide/secret；因此初始请求不可能预装答案。受控 Provider 仅在实际第二请求包含 `role=tool` 且 receipt 含数据库随机 secret 时返回该 secret；最终输出断言必须引用它。

故障子进程只在测试侧用 `testOnlyToolFeedbackDroppingTransport` 删除 HTTP wire 的 `role=tool` message。受控 Provider 因看不到真实查询结果返回缺失标记，原最终答案断言真实失败并使子进程退出非零；父测试只在输出包含 `formal Agent final answer did not use the database-only Tool result` 时接受该失败。随后移除 mutation transport，完全相同 probe 必须退出 0，并额外核验两次物理 request ID、正式 invocation/result trace 与第二请求实际 Tool result。

## 执行结果

正式最终命令（敏感值未写入日志）：

```sh
go -C apps/core-go test ./internal/core \
  -run '^(TestFormalToolEinoAdapterE2E|TestIndependentToolE2ERejectsFalseSuccessWithoutWrite|TestFormalAgentE2ERejectsBrokenToolResultFeedback)$' \
  -count=1 -v
```

退出码：`0`。18 个 adapter 子测试及两个父破坏实验全部 PASS。日志：`research/runs/adapter-fault-evidence-20260922-final.log`。

Race 命令：

```sh
go -C apps/core-go test -race ./internal/core \
  -run '^(TestFormalToolEinoAdapterE2E|TestIndependentToolE2ERejectsFalseSuccessWithoutWrite|TestFormalAgentE2ERejectsBrokenToolResultFeedback)$' \
  -count=1
```

退出码：`0`。日志：`research/runs/adapter-fault-evidence-20260922-race.log`。

静态检查：`go -C apps/core-go vet ./internal/core`，退出码 `0`。日志：`research/runs/adapter-fault-evidence-20260922-vet.log`。

保留失败尝试：

- `research/runs/adapter-fault-evidence-20260922-adapter.log`：共享工作树并行视觉测试一度缺少 helper，包编译退出 `1`；对应并行切片随后修复，本切片未修改该文件。
- `research/runs/adapter-fault-evidence-20260922-faults.log`：早期 controlled feedback fixture 的最终 schema/初始投影约束未收敛，退出 `1`；最终实现改为捕获初始请求后再经正式 Tool 写随机 PG 内容，消除了答案预装可能。

这些测试是受控模型回归与故障敏感性证据，不冒充 live Provider Agent E2E。用户可在最终整套验收时串行运行 live 模型/媒体矩阵。
