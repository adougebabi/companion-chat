# Phase 8 技术设计：Eino Native 收敛、契约门禁与故障定位

## 1. 设计目标与边界

本设计把 Eino v0.7.37 的 `ChatModelAgent`/`Runner` 作为唯一通用模型—工具循环，同时保留摇光已经形成的领域边界。它不把所有结构化模型任务都改造成 Agent，也不替换 Temporal、CapabilityRuntime 或正式发布入口。

当前生产权威和职责如下：

```text
ModelTask / ConversationRuntime
        │
        ├─ fixed structured task ──> Eino ChatModel Generate（单次 DTO 契约）
        │
        └─ ADK allowlist ─────────> ChatModelAgent + Runner
                                     │ AgentEvent / native ToolCalls
                                     ▼
                                ADKCapabilityTool
                                     │ formal call ID
                                     ▼
                            appADKCapabilityInvoker
                                     │ registry/surface/candidate gate
                                     ▼
                              CapabilityRuntime
                                     │ Prepare / freeze / tx / outbox
                                     ▼
                            领域结算与唯一正式发布入口
```

`RunADKLoop` 位于 Runner 和 Core 之间，只消费事件并形成 `ADKCapabilityTrace`/completion 投影；它不执行第二次工具、不主动调用第二次模型、不持有事务。

## 2. 协议与数据流

### 2.1 ADK 原生 ToolCall

`generateWithADK` 直接把 Eino `schema.Message.ToolCalls` 的 ID、函数名和 arguments 传给 `RunADKLoop`/invoker。ADK 路径不再把正文、Markdown、reasoning 或结构化 sidecar 当作执行来源。

Provider 层区分两种解析模式：

1. ADK completion：原生 Eino message/AgentEvent 是唯一 ToolCall authority；sidecar 中出现的 `tool_calls` 只能作为诊断冲突记录，不能进入业务 invocation。
2. Fixed Task/non-ADK completion：继续使用明确 DTO/Schema 的严格解析；只有既有契约声明允许的原生 ToolCall 映射才进入领域动作，不新增循环。

原生 call 必须有非空 ID、名称和合法 JSON arguments。相同 ID 只能对应同一名称和参数；缺失、冲突、未知、未授权或 schema 无效均返回可定位错误，不生成替代 ID、不静默丢弃为成功。固定 Task 的合法 JSON 仍按原 DTO 和领域校验处理。

### 2.2 Agent 事件与业务结果

`RunADKLoop` 对 Runner iterator 做单次消费：记录 AgentEvent 来源、物理迭代、assistant/tool message、ToolCall 和 Tool result；最后依据终止类型选择 completion。它不从“最后一个非空 assistant 文本”推断结果，必须区分：

- 正常最终文本；
- 合法终止型工具的 direct result；
- deferred/accepted-pending 的待提交结果；
- 工具错误、模型错误、取消；
- `ErrExceedMaxIterations`。

工具事实、结果和最终文本在内存 trace 与领域投影中分别保存。`filterADKTraceToToolCalls`/`mergeADKTraceInvocations` 只按真实 call ID 合并同一执行事实；不同 call ID 即使名称和参数相同也不合并。WakeUp 的 trace 合并仍进入现有 freeze/settle 流程，不能再次准备或提交。

### 2.3 Tool 适配和领域状态

`ADKCapabilityTool` 只实现 `Info` 和 `InvokableRun`：

- `Info` 从同一正式 `CapabilityDefinition` 生成 `schema.ToolInfo`；
- `InvokableRun` 使用 `compose.GetToolCallID(ctx)`，缺 ID 失败；
- invoker 传递获准 surface、owner、ContextSlot 和 request identity；
- pure query 可立即返回真实序列化结果；写入/输出/外部异步能力返回真实 deferred/accepted-pending 或 error；
- 适配器不构造下一轮模型消息、不调用 ToolsNode、不直接发布。

Registry 的 App 构造路径是生产权威。`internal/capability` 中仍被 Core 使用的值对象、接口和类型别名保留；独立的可执行 Registry/Runtime 实现若经全仓引用核对确认无生产调用，则移除其执行入口或改为共享类型实现，避免两套注册/执行语义。不能以兼容别名保留第二条正式运行路径。

所有模型可见入口（ADK、Native Cognition、Daily Review 等 fixed task）在冻结/Prepare 前共用 canonical candidate gate：registry definition、`InternalOnly`、surface、owner/scope、ContextSlot、参数和 source policy 都必须验证。策略能力（`persona.takeover`/`persona.switch`）继续走 policy bridge，不进入模型 catalog；`visual_identity.initialize` 按当前代码实际 `InternalOnly`/surface/策略归属固定并用反例锁定，不能为了矩阵完整而扩大权限。

## 3. Trace 与诊断设计

不新建观测平台或第二 Runtime，复用 `diagnostic_events`、`diagnostic_model_runs`、现有 provider attempt/request identity、lifecycle diagnostic 和 `DiagnosticsExportFiltered`。

一次运行的关联链使用同一 context/correlation：

```text
run_id / correlation_id / surface / agent / stage
  → physical provider request + attempt + model_run
  → native tool_call_id/name/arguments_digest
  → tool requested / authorized / dispatched / rejected
  → capability prepare / execution / business status
  → tool result / serialization error
  → next physical model input contains matching assistant+tool pair (true/false/n/a)
  → termination reason
  → freeze / transaction / outbox / publish status
```

每个物理模型调用继续拥有独立 request/attempt/model-run identity；在现有脱敏 metrics/payload 中补充 bounded stage、tool-call digest、message-event count、retry count 和 next-input evidence。ADK tool 进入、拒绝、未调度、返回、终止等写入现有 `diagnostic_events`（事件类型带 `adk.`/`tool.` 前缀，payload 仅保留脱敏摘要）。不增加 prompt/response 明文或私人内容。

`DiagnosticsExportFiltered` 扩展为按 `run_id`/correlation 导出这些事件和 model-run 证据；现有 owner 授权、limit、redaction、lifecycle filter 保持不变。若现有导出字段无法表达某个阶段，优先在事件 payload/metrics 中补充受约束字段，而不是增加另一张持久化表。导出必须能区分“未进入 InvokableRun 的未知/调度错误”和“进入后执行/序列化错误”。

## 4. 全量契约矩阵

矩阵由两份来源组成：

1. 实际存在项：扫描生产构造/注册、正式调用方和各 `ModelTask` 入口生成；它用于发现漂移，不直接生成期望答案。
2. 审查期望夹具：固定登记 Conversation Main、Takeover Reply、WakeUp、Native Cognition、Daily Review、Reflection 等 Agent/Task，以及 13 个 direct capability、2 个 policy capability、每个 surface 的允许关系。期望关系独立于生产 catalog 函数。

每个矩阵叶子记录 source、Agent/Task、surface、Tool、可见性、执行类别、终止/回填方式、测试名和条件环境。测试目标不存在、零匹配、关键子测试 SKIP、未登记生产入口和证据文件缺失均为非零。DB、真实 provider、Temporal 等条件层必须显示 `conditional`/`blocked` 与原因，不能改写成 PASS。

套件使用真实生产构造、真实 Eino Runner 和 controllable fake model/provider，只替换明确外部网络/数据库依赖；不替换整个 Runner，也不把每个真实 capability 的业务处理替换为统一成功桩。写能力的数据库原子性在隔离 PostgreSQL 集成层验证，纯适配/状态机在单元层验证。

## 5. 兼容、回滚与安全

- 不改变 Eino、Go、OpenAI extension、Temporal 或数据库字段的既有版本；如确需持久化字段，先证明现有事件/metrics 无法表达，再按项目迁移/回滚规则增加最小字段。
- 不启用默认 retry、failover 或模型降级。模型错误、Tool 错误、取消、迭代上限和领域 settlement 错误保持原阶段与错误码。
- 先补反例和契约，再替换协议路径；每个 P8 项目保持可独立回退的提交边界。若一次收敛导致既有对话/WakeUp 行为变化，回退该边界的代码/测试，不恢复被证明为第二 authority 的兼容路径。
- 所有新诊断字段经过现有 `redactDiagnostic`/bounded payload；证据 bundle 运行密钥/个人数据扫描。真实消息、生产数据库和共享卷不作为测试目标。

## 6. 交付结构

主报告和 evidence bundle 以本阶段最终快照为准，最少包含：命令输出、原始 Go JSON events、matrix、执行摘要、成功/失败/解析失败 trace、关键连续源码/测试摘录、环境与依赖版本、HEAD/工作树指纹、每个文件字节数和 SHA-256。证据路径使用 bundle 内相对路径；重新解压验证引用与 hash。
