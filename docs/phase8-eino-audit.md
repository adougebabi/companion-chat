# 摇光第八阶段：Eino Native 迁移前定向取证

## 0. 范围与结论

本报告是只读取证，不是重构方案。检查时间：2026-09-21；未修改生产代码、测试、配置、依赖、数据库或部署，也未运行生产/外部服务测试。工作区原有未提交修改已保留。

结论：当前仓库已经在正式路径使用 Eino `adk.NewChatModelAgent` + `adk.NewRunner`，但外层仍保留一个项目自有的 `RunADKLoop` 事件收集/结果投影层，以及 Provider 层的多形态 tool-call 规范化和结构化 sidecar 解析。实施范围应是“以 Eino ADK 事件与原生 Message/ToolCall 为唯一模型—工具循环，保留 CapabilityRuntime 的领域准备、冻结、事务、幂等、Outbox、审计和最终发布”，而不是再建第二个 Agent Framework，也不是把 Temporal durable workflow 替换成 Eino Workflow。

锁定版本：`github.com/cloudwego/eino v0.7.37`；ADK 属于该主模块的 `adk/`，不存在独立 ADK 版本。实际模块目录：`/Users/vinson/go/pkg/mod/github.com/cloudwego/eino@v0.7.37`。

## 1. 基线

| 项目 | 证据 |
|---|---|
| 分支/HEAD | `master`, `a237b611058f0791429f54c9f3b223a67154f52f`；与 `origin/master` 同步 |
| 未提交修改 | `apps/core-go/internal/core/wakeup.go`、`wakeup_intents_test.go`、`wakeup_test.go`；80 additions/1 deletion。修改是 WakeUp 保留 ADK trace tool calls/results 的修复，不能覆盖或回退。 |
| 模块 | `apps/core-go/go.mod:1-3`，Go `1.25.4`；无 `go.work`、`vendor`、`replace`。 |
| Eino 依赖 | `go.mod:5-10`：Eino `v0.7.37`、OpenAI model extension `v0.1.13`、embedding OpenAI 伪版本。 |
| 构造入口 | API `cmd/api/main.go:20-98`；Worker `cmd/worker/main.go:24-223`；App `internal/core/app.go:117-155`。 |
| Durable workflow | Temporal worker 注册在 `internal/workflow/workflow.go:920-1025`；Eino Workflow 未被用来替换 Temporal。 |

## 2. A. Agent / Task 清单

“Agent”只指实际使用 ADK Runner 的路径；其它是固定模型 Task，不强迫改成 Agent。

| 名称/入口与调用方 | 构造/模型/Prompt/工具 | 结果与现有测试 | 迁移判断 |
|---|---|---|---|
| 对话 `RunADKStructuredTask` → `conversation_runtime.go:66-151` → `Provider.StructuredAssembledWithToolsSchema` | schema `conversation_turn_response`；projection prompt；surface conversation；catalog capability tools；实际 Agent/Runner 在 `eino_model_runtime.go:392-521` | `eino_adk_runtime_test.go`、`conversation_delivery_regression_test.go`、`conversation_mixed_media_reply_test.go` | ADK 接管循环、tool binding、结果回填；Core 保留 context、权限、冻结/提交。 |
| WakeUp `ProcessWakeUp` → `wakeup.go:501-714` | schema `wake_up_response`；WakeUp catalog；ADK trace 合并；Temporal 外层负责 durable intent | `wakeup_test.go`、`wakeup_intents_test.go`、`settings_wakeup_test.go` | 保留 WakeUp action/freeze/repair/next clock；不得再做第二次 cognition loop。 |
| takeover/固定 Judge → `turn_takeover.go:609-793` | `RunPersistentSwitchTask` / Judge 为单次 structured task，无工具 catalog | `persistent_switch_assessment_test.go`, `takeover_scope_matrix_test.go` | 保留单次任务，不改造为 Agent；仅统一 Eino model/message 协议。 |
| Native Cognition → `model_tasks.go:192-205`, `cognition.go:31-82` | `native_cognition_response`，Native Cognition surface，允许 capability catalog | `cognition_*_test.go`, `eino_adk_runtime_test.go` | 若启用工具循环，使用同一 ADK 入口；settlement 与 inbox claim 保留。 |
| Daily Review → `model_tasks.go:213-226`, `autonomy.go` | `daily_review_response`，Autonomy surface，catalog；Temporal `DailyReviewWorkflow` | `autonomy_policy_test.go`, `schedule_replan_test.go` | 保留 durable review、治理、提交；Eino 不接管 Temporal。 |
| Reflection → `model_tasks.go:259-272`, `reflection_runtime_v2.go:132-422` | `reflection_proposal_v2`，definitions 明确为 nil；proposal→domain mutation | `reflection_*_test.go` | 这是固定 Task，不加工具循环；保留 evidence、proposal 编译、事务。 |
| Initialization → `RunInitializationTask:29-45` | 单次 `initialization` structured 输出 | `initialization_*_test.go` | 固定 Task。 |
| Summary → `RunConversationSummaryTask:140-155` | `conversation_summary_v1`；摘要窗口/预算由 Core 控制 | `conversation_summary_test.go` | 固定 Task，保留 chunk/budget/settlement。 |
| Media → `RunMediaPromptTask:48-63`, `RunMediaQualityTask:66-91` | prompt 文本任务 + quality structured task；Temporal Media workflow | `media_*_test.go` | 固定 Task；保留 media intent、quality retry、发布。 |
| Visual → `RunVisualIdentityVisionTask:94-118`, `RunVisualIdentityPatchTask:121-133` | vision/patch structured task；initializer capability 只由 WakeUp 暴露 | `visual_identity_test.go` | 固定 Task；保留资产、会话、workflow。 |
| Schedule → `RunScheduleGenerationTask:168-180`, `RunScheduleReplanTask:277-287` | structured schema；planner 业务约束 | `schedule_*_test.go` | 固定 Task；保留 revision/事务。 |
| Embedding → `RunEmbeddingTask:290-306` | model Embed；`RunFrozenEmbeddingTask` 使用 workflow pinned assignment | `memory_embedding/lifecycle/retrieval` tests | 保留 embedding queue、冻结 assignment、异步重试。 |

当前 ADK 允许 schema 白名单为 `conversation_turn_response`、takeover reply schema、`wake_up_response`（`adk_conversation_runtime.go:199-206`）；不能按名称把所有 Task 补成 Agent。

## 3. B. Tool / Capability 清单

注册唯一默认入口是 `builtinCapabilities:73-123`，registry 校验在 `capability/registry.go:17-165`。以下是模型可暴露的业务 capability；内部 persona action 不进入普通 catalog。

| 模型名称 | 定义/注册 | Eino 接口与业务处理 | 参数/权限/副作用/结果 |
|---|---|---|---|
| `conversation.reply` | `tool_contract.go:197-245`; builtin | `ADKCapabilityTool.Info/InvokableRun` → `appADKCapabilityInvoker.ExecuteWithID` → `CapabilityRuntime`; deferred tx | conversation surface；text 校验；先 deferred，绑定 conversation target 后事务提交 assistant message。 |
| `moment.publish` | `tool_contract.go:197-245`; builtin | 同上 | deferred output；moment target；提交失败不得等同 tool success。 |
| `media.image.generate` | `tool_contract.go:246-280`; builtin `builtin_capabilities.go:198-335` | Prepare 冻结 visual/life/appearance/state；deferred tx 创建 media intent | WakeUp/conversation/autonomy 由 catalog 决定；异步受理，Temporal Media workflow 负责后续。 |
| `visual_identity.initialize` | `builtin_capabilities.go:338-380` | internal capability，调用 visual identity initializer | 仅允许策略/指定 WakeUp surface；创建 workflow intent，不是普通模型工具。 |
| `scene_event`, `presence_event` | `native_capabilities.go:12-90` | Capability + transactional service | native cognition/conversation 等按 surface；写入 life context，需 revision/context。 |
| `schedule.replan` | `schedule_capability.go:13-38` | planner + transactional apply | revision、timezone、完成历史保护；事务/拒绝语义由 Core。 |
| `memory_event`, `active_memory_event` | `memory_intelligence.go:20-50`, `active_memory.go:19-61` | Prepare/transactional apply | memory scope/provenance/幂等；提交后才 completed。 |
| `memory.recall` | `memory_recall_capability.go:17-44` | pure query | 只读、范围过滤；返回查询结果或失败。 |
| `affect_event` | `affect_capability.go:33-70` | transactional apply | affect reducer/事务；结果不能覆盖其它工具调用。 |
| `relationship.lookup` | `relationship_capability.go:12-25` | query + candidate validator | 只读关系范围；未授权/非法 target 拒绝。 |
| `capability.request` | `capability_requests.go:13-33` | transactional proposal | 只提出权限请求，不伪造已授权执行。 |
| persona takeover/switch | `persona_action_capabilities.go:18-51` | `InternalOnly`；策略/Judge调用 | 不进入模型工具 catalog；由 takeover gate 控制。 |

仅由策略调用的能力、未注册实现和 InternalOnly 能力必须与模型可见工具分开统计。`CapabilitySurface` catalog 会排除 `InternalOnly`；当前代码同时在 ADK bridge 做 registry lookup、surface、definition equality 和 candidate validation（`adk_conversation_runtime.go:70-165,237-305`）。

## 4. C. 权限矩阵（当前实际值 vs 规范期望）

| Agent/阶段 | 当前代码实际允许 | 规范期望/差异 |
|---|---|---|
| Conversation turn | `capabilityCatalog(...Conversation)`；ADK bridge 再校验 surface | 允许的 conversation 工具可见且可执行；需补全每个关系的实际执行+结果消费测试。 |
| WakeUp assessment | `CapabilitySurfaceWakeUp` catalog；trace 中的调用先 candidate validate，再 freeze | WakeUp 可 proactive output/native state；内部 persona action 不应暴露。工作区未提交修复正是保留 trace calls/results。 |
| Native Cognition | `CapabilitySurfaceNativeCognition` catalog | 仅 native cognition 允许关系；不得用 conversation policy 生成期望值。 |
| Daily Review/Autonomy | `CapabilitySurfaceAutonomy` catalog | 允许 autonomy 关系；Temporal review/治理仍由 Core。 |
| Reflection | `definitions=nil`，没有模型工具 | 规范要求 proposal-only；当前一致。 |
| Persistent switch/Judge | nil tools，单次 structured task | 规范要求 Judge 不重复调用；当前一致。 |
| Internal persona action | registry 中存在但 `InternalOnly` | 只能策略调用；必须有“不可见且不可执行”门禁。 |

这是代码实际静态矩阵；现有测试多数检查定义/边界，尚未形成完整 Agent×阶段×Tool 对账门禁，因此“规范要求”与“当前允许”不能由同一策略生成。

## 5. 普通对话与 WakeUp 的完整 Tool 链路

1. Provider 返回 Eino `schema.Message`；`schema.ToolCall` 含 `ID`、function name、arguments（Eino `schema/message.go:121-125,648-682`）。项目 `generateWithADK` 在 `eino_model_runtime.go:392-521` 绑定 `schema.ToolInfo` 与 ADK model。
2. ADK `ChatModelAgent` 使用 `compose.ToolsNode` 调用 `InvokableRun(ctx,args)`；Eino 将当前 call ID 放入 context，`compose.GetToolCallID`（Eino `components/tool/interface.go:25-58`, `compose/tool_node.go:1038-1050`）。
3. 项目 `ADKCapabilityTool.InvokableRun`（`ai/agent/loop.go:93-110`）规范空参数、取得 call ID，调用 `ExecuteWithID`。
4. `appADKCapabilityInvoker.ExecuteWithID`（`adk_conversation_runtime.go:70-165`）做 registry lookup、同 ID 幂等/复用检查、CapabilityInvocation 构造、ContextSnapshot、candidate validation；pure query 可立即 `CapabilityRuntime.Execute`，写入/输出类保持 deferred。
5. CapabilityRuntime（`capability_core.go:1224+`; `capability_runtime.go`）负责 Prepare/context、事务、幂等、deferred settlement、错误状态和领域审计；它不是 Eino 的职责。
6. 返回字符串是 Eino Tool result；ADK 将 tool message 回填下一次模型输入。官方 Runner `Run` 返回 `AsyncIterator[AgentEvent]`（Eino `adk/runner.go:71-99`），项目 `RunADKLoop` 消费事件、去重 call ID、记录 tool messages，最终取最后 assistant message（`loop.go:172-230`）。
7. 对话最终由 `conversation_runtime.go`/`turn_takeover.go` 解析 completion，冻结 candidate、Judge/提交；WakeUp 在 `wakeup.go:664-714` 合并 trace calls/results，后续 `:864+` 冻结 action，`ProcessAutonomyAction` 再提交/发布。

边界结论：Tool 函数返回 ≠ 业务动作提交 ≠ 用户回复发布。当前失败可发生在 native ToolCall 未生成、工具集合/名称/schema 不一致、call ID 缺失/复用、candidate hook 拒绝、InvokableRun 返回错误、tool result 序列化、下一轮模型输入/最终解析、deferred settlement 或最终发布任一边界。未发现当前授权日志可供本次串联的 run；因此没有故障复现证据，只有静态风险。尤其不能凭 Tool 返回成功标记“模型已收到结果”，需捕获下一次实际模型输入。

## 6. 重叠机制与正式替换点

| 当前文件/符号 | 正式职责/是否正式路径 | Eino 接替点 | 迁移后删除/保留与语义差异 |
|---|---|---|---|
| `ai/agent/RunADKLoop` | ADK 已负责 Agent loop；项目负责事件聚合、去重、最终消息判定 | `adk.NewChatModelAgent` + `adk.NewRunner` + `AgentEvent` | 删除重复的模型/工具循环，不删除 Core 的 trace/settlement 投影；验证 max iteration、tool-only、empty final。 |
| `ADKCapabilityTool` | 正式 Eino `tool.BaseTool/InvokableTool` adapter | `Info`, `InvokableRun` | 保留薄适配器：schema、call ID、CapabilityRuntime bridge；删除旧 tool-call executor。 |
| `normalizeProviderToolCalls*` / `einoToolCalls` | Provider 协议兼容/sidecar 解析，仍在 `provider.go:353-460`, `eino_model_runtime.go:830+` | Eino `schema.Message.ToolCalls` 原生通道 | 原生 ADK 路径不得再从正文猜 tool call、派生 ID 或重复注入；仅为非 ADK 固定 DTO 做明确、受测映射。 |
| `StructuredAssembledWithToolsSchema` | 统一 provider boundary；正式调用方 | ADK only for allowlisted loop schemas | 保留 fixed Task 的 provider DTO；ADK schema 禁止外层再次手调 ToolsNode。 |
| `ToolOnlyTermination` + `ErrExceedMaxIterations` 特判 | ADK 已定义 max iterations/error；Core 有业务“tool-only completion” | Eino `ErrExceedMaxIterations`, `AgentAction/Exit` | 只保留与 WakeUp/Conversation 终止语义相符的业务判定；不扩大为全局成功。 |
| `CapabilityRuntime.Execute/Prepare/ExecuteDeferred/ExecuteTransactional` | 领域业务机制 | 无 Eino 替换 | 必须完整保留：权限、context、冻结、事务、幂等、Outbox、审计、deferred/rejected。 |
| `ProviderClient` retry/failover/queue | provider/调用预算 | Eino `ModelRetryConfig` 仅是可选 model retry（Eino `chatmodel.go:210-253`） | 不默认打开或扩大 retry；领域拒绝、网络错误、Temporal retry 分层。 |
| StreamReader/EnableStreaming | Eino transport/event stream | Eino `EnableStreaming`, `StreamReader` | 不能替代队列许可、取消、审计生命周期；补取消/关闭/计数测试。 |
| Temporal workflows | durable task/workflow | 不迁移到 Eino Workflow | 保留 Temporal；同名不等价。 |

## 7. 测试缺口与 Trace 门禁（本次不实现）

现有测试已经覆盖 ADK tool result 回填、模型/工具失败、取消、max iterations、WakeUp trace 修复（`eino_adk_runtime_test.go:244-345`；未提交 WakeUp 测试）。但仍缺少以下可直接锁定实施的门禁：

| 范围 | 当前状态 | 必须补齐的断言 |
|---|---|---|
| 每个正式 ADK Agent | 已有对话/WakeUp 样例，但非全清单对账 | 实际生产构造 + 实际 Runner + controllable model；正常、tool feedback、unknown/unauthorized、bad args、tool/model failure、cancel、limit、合法 direct return。 |
| 每个 Tool | 分散在 capability/contract tests；部分只测底层 runtime | 经 Eino adapter 检验 Info/Schema、参数、权限、context、返回/错误/取消；写工具补 Prepare、事务、幂等、拒绝、异步受理。 |
| 固定 Task | 多数已有单元测试，未统一清单 | 输入、Prompt、schema/output contract、失败；不得把 Task 强制变 Agent。 |
| Agent/阶段×Tool | 有 surface/definition 测试，缺全矩阵 | 允许关系实际执行并消费结果；终止工具验证终止；禁止关系验证不可见且不可执行。 |
| 正式入口/提交 | 有大量集成测试，缺端到端接线 manifest | 选定 Agent→Tool→CapabilityRuntime→最终副作用；不可只测底层样例。 |
| 重试/回填 | 有幂等测试但无统一计数门禁 | 无重试确保调用/回填不重复；有重试按 call ID/领域幂等；不要宣称 Eino 提供 exactly-once。 |
| Trace | Diagnostics 已保存 provider response（近期 commit），但未证明下一次输入关联 | 复用现有日志/诊断，最小链：`run_id → physical model call → tool_call_id → tool invocation → Capability state → next request contains result → termination → final submit/publish`；分别计 model calls/message events/tool calls/retries。 |

零匹配、关键叶子 SKIP、子进程失败不可被父 PASS/管道码掩盖；Fake、数据库集成、浏览器、真实模型分别统计。不要运行生产或外部服务测试凑结果。

## 8. 能力差异与待决范围

目前没有足够证据声称锁定 Eino 缺能力。已核实的正式扩展点包括 `ChatModelAgentConfig.Middlewares`（before/after model、tool middleware）、`ReturnDirectly`/`Exit`、`Runner.CheckPointStore`、ADK workflow agents 与 compose Workflow；项目当前没有使用其中大多数。最小待决范围只有：

1. 是否把 `RunADKLoop` 的事件聚合收敛到一个薄的 ADK event-to-domain projector；
2. 对 ADK allowlisted schema，哪些旧的 provider sidecar/派生 ID 代码可以删除，哪些固定 Task DTO 仍需保留；
3. `ReturnDirectly` 仅在已提交/可直接发布的工具上是否适用；conversation.reply、media、moment 等 deferred 工具不能提前发布；
4. 在不改变业务预算、权限和 Temporal 重试的前提下，是否配置 Eino model retry；
5. 用真实 Eino Runner + controllable model 建立完整 manifest/matrix 门禁。

## 9. 实施锁定建议

先锁定一条正式路径：Conversation 与 WakeUp 继续通过 Eino ADK；删除/禁止 ADK 路径的正文猜测和重复 tool loop；保留 `ADKCapabilityTool` 作为唯一薄适配器，保留 CapabilityRuntime 与领域提交；固定 Task、Reflection、Embedding、Media/Visual workflow 不因名称相似改成 Eino Workflow。实施前先生成 Agent/Task/Tool/阶段矩阵和 Trace 关联测试清单，任何新增入口未覆盖即失败。

## 10. 证据限制

本次没有读取到可授权的单个故障 run 日志，也没有执行可能访问生产/外部服务的测试；“Tool 返回但失败/没有调用”的结论是静态边界风险，不是本次复现。模块源码证据来自本机 Go module cache，路径和版本已在本文及 zip manifest 固定。
