# 摇光项目 Eino 基础层整体重构与 ADK 对话接入

## Goal

将 Core Go 中分散且自研的模型 HTTP、工具续接和 Prompt 组装路径收敛到 Eino 基础组件，并让直接对话由 Eino ADK 的 `ChatModelAgent`/`Runner` 负责模型—工具—结果回填—继续生成；同时保持现有人格权限、冻结决策、事务提交、Temporal、BFF 与领域状态边界不变。

交付目标不是“添加依赖”或“迁移一个样例”，而是完成生产入口、Worker 入口、Embedding、多模态和已有流能力的全量迁移，并删除被替代的旧 Provider HTTP/循环/Prompt 双轨。

## Confirmed repository facts

- 生产后端是 `apps/core-go`（Go 1.25.4）和 `apps/gateway-go`（Go 1.23）；当前两个模块均无 Eino/ADK 依赖。
- 现有模型调用集中在 `apps/core-go/internal/core/provider.go` 的 `ProviderClient`，通过自研 OpenAI-compatible HTTP、队列、Redis lease、预算、诊断、native/sidecar tool normalization 统一处理。
- 对话主流程在 `mutations.go`，人格接管在 `turn_takeover.go`，查询续接在 `mutations.go`/`query_continuation.go`；现有 CapabilityRuntime 已拥有权限、冻结、Prepare/Execute、事务和幂等边界。
- Prompt 运行时已有 `WorkingMemory`、`PromptFragment`、`PromptContextAssembler`、Working Persona 和 assembled-message 校验，但没有正式的任务声明式 Prompt Slot/Composer 抽象。
- 当前 Provider SSE `StreamText` 没有生产调用方；浏览器 token frame 在 assistant settlement 提交后发送，不能被 ADK streaming 提前破坏。
- 基线：在两个 Go module 分别运行 `go test -mod=readonly ./...` 通过。

## Requirements

### R1. Eino model/embedding foundation

- 使用锁定版本的 `github.com/cloudwego/eino` 与官方 `eino-ext` OpenAI ChatModel/Embedder；不把旧 HTTP client 包装成“Eino 接口”。
- 配置读取、角色绑定、能力校验、队列/取消/超时、Usage/审计与 Eino model 构造解耦；每次 Eino model call 必须经过现有队列、取消和诊断支撑。
- 保留 provider assignment 的 role/schema/scenario/预算/secret/endpoint/model provenance；不静默降级到 `generic_llm`、旧实现或不匹配模型。
- 文本、结构化 JSON、native tool call、多模态消息、Embedding 与已有流能力都通过新基础层；Embedding 保持独立接口和独立队列。

### R2. Prompt Slot / Fragment / Composer

- 建立唯一正式 Composer，提供显式的 Slot/Fragment 标识、输入、位置、顺序、必需性、预算策略与 trace。
- Composer 只负责 prompt 选择、预算和 Eino `schema.Message` 组装，不持有 `*core.App`，不访问 DB/Redis，不修改领域状态。
- 现有 Working Persona、WorkingMemory、recent turn 完整性、current input 去重、assembled-message 合同和预算算法迁移到该 Composer；旧 Composer 不再有生产调用路径。
- Prompt Slot、Capability ContextSlot、持久 typed evolution slot 保持不同类型和依赖边界；新增 Slot 只影响显式选择它的 Task。
- ADK 每次模型调用按当前阶段重建 system/context/tools；人格接管后的 B 必须重新构造 reply-owner prompt 与权限。

### R3. Model Tasks

- 为 Main conversation、query continuation、persistent switch assessment、takeover judge/reply、WakeUp、Reflection、Daily Review、Conversation Summary、Schedule、Media Prompt/Quality、Visual Identity Vision/Patch、Initialization、Memory Embedding/Retrieval Embedding 建立职责明确的任务入口。
- Task 接受业务输入并返回业务结果/规范化 completion，不把 Eino运行状态、ADK event 或 provider raw payload 传入领域实体、数据库或 BFF DTO。
- 单次摘要、视觉判断、反思提案、Embedding 等任务可直接使用 Eino model component，不强制套 Agent loop；所有模型调用仍共享同一调用支撑。

### R4. ADK conversation runtime

- 直接对话必须经 Eino ADK `ChatModelAgent` 和 `Runner`，由 ADK 真正推进模型、正式工具调用、工具结果回填和继续生成/结束。
- ADK 工具只能是现有 CapabilityDefinition/CapabilityRuntime 的窄适配；执行仍经过 context snapshot、权限、冻结、Prepare/Execute、事务/CAS、幂等和审计。
- ADK 不得重复发送 user/system/tool 消息，不得伪造 tool call ID/name/arguments，不得自动重试或无限循环；Main generation、query continuation、takeover A/B 次数和 response mode 约束由显式 runtime policy 保证。
- 普通最终回复与 `conversation.reply` 汇入唯一正式发布入口；工具回合、最终结构化输出和 settlement 后 token frame 分离。
- ADK checkpoint/事件只保存 bounded runtime metadata，不把 raw prompt、raw response、reasoning、凭据或完整领域状态写入诊断/数据库。

### R5. Persona/capability invariants

- 当轮 reply owner takeover 与持久 active profile switch 继续分离；takeover B 不获得持久切换权限，不递归进入 Judge。
- Judge 仍由确定性策略触发；A 未获准候选和其动作不得执行/发布。B 必须重新创建 prompt、context reference index、tool catalog 与作用域。
- invocation/result 的 source（native model tool vs deterministic policy）如实记录，保留现有 `CapabilityInvocation`/`CapabilityResult` v2 authority，不引入第二套持久 tool envelope。
- 不跨越 LLM 调用持有数据库事务；外部媒体/Temporal/Redis 调用不进入 domain settlement transaction。

### R6. Removal, docs and verification

- 删除旧 Provider HTTP 请求/响应解析、旧通用模型循环、旧 Prompt Composer/兼容分支、畸形 tool 修复/伪 tool JSON/旧回退路径；不移动到 `legacy`/`backup` 留存。
- 更新模块依赖、模型配置/能力要求说明、架构与迁移清单文档；不改变 BFF 协议、Temporal 类型、领域事务/Outbox 或记忆/情绪/日程规则。
- 使用 Fake/Mock Eino model 与 tools 覆盖普通 ADK 对话、真实 tool result 回填、query 次数、takeover 权限/重建、候选拒绝、失败/取消/上限、单次发布、Slot 预算/隔离、Reflection 禁用上下文、文本/结构化/多模态/Embedding/streaming 和历史格式 fail-closed。
- 运行两个 Go module 的格式化、构建、单测、相关 `-race` 和全仓旧符号扫描；真实 provider/ComfyUI/计费等无凭据项明确列为未验证，不以 Mock 冒充。

## Out of scope

- 不删除或重做 BFF、Temporal、记忆/情绪/日程领域系统、领域事务/Outbox/异步状态机。
- 不改变人格切换条件、权限、反思演化规则或前端接口协议。
- 不建设通用 Agent 平台、在线工作流设计器、Prompt 配置平台，也不顺带开放浏览器 token 级流式输出。

## Acceptance criteria

- [x] `go.mod`/`go.sum` 锁定 Eino、ADK、官方 OpenAI ChatModel/Embedder；生产模型和 Embedding 请求不再直接使用旧 `/chat/completions`/`/embeddings` HTTP 解析器。
- [x] 所有已盘点模型入口（Main、continuation、takeover、WakeUp、Reflection、Summary、Schedule、Media、Visual Identity、Initialization、Embedding、已有流）都有迁移清单、Task/Composer 路径和测试证据。
- [x] 直接对话测试能证明 ADK Runner 发起模型调用、执行正式 Capability tool、将结果送回下一次模型输入并只发布一次最终回复。
- [x] Prompt Slot/Fragment Composer 测试覆盖选择、顺序、预算、current input 去重、完整 recent turn、B 重建和任务隔离；旧 Composer/循环无生产引用。
- [x] Capability、冻结 payload v2、query continuation、takeover/persistent switch、事务/取消/审计边界与现有回归测试保持通过。
- [x] 两个 Go module `go test -mod=readonly ./...`、相关 `go test -race`、`go vet`/静态扫描通过；真实模型未验证项显式记录。
