# 摇光第八阶段：Eino Native 收敛、全量契约测试与故障定位

## Goal

在当前工作树和锁定依赖上，把 Eino ADK 变成正式模型—工具执行的唯一通道，清除会产生第二执行权威的外围协议猜测与重复回填，并为实际存在的 Agent、固定 Task、Tool、策略能力和阶段建立可执行的全量契约门禁。一次真实或受控故障必须能够定位到模型调用、原生 tool call、权限、Capability 执行/准备、结果回填、终止、提交或发布的明确阶段。

交付必须反映当前源码真实状态，不重做前七阶段，不重置或覆盖已有工作，不以新增目录、兼容别名、静态 PASS 或“接通一个工具”作为完成。

## Background and confirmed facts

- 审计记录的历史基线为 `a237b611058f0791429f54c9f3b223a67154f52f`，但当前 `HEAD` 已为 `50db532`；已有 `phase8-eino-audit.md`、`phase8-eino-audit-evidence/` 和 zip 是旧的静态取证，不能直接作为本阶段完成证据。
- 当前工作树中未提交内容应保留；尤其不能回退 WakeUp 已有的 ADK trace calls/results 修复。当前任务新增的规划、代码和证据必须与现有文件共存。
- 依赖锁定为 Go `1.25.4`、Eino `v0.7.37`（ADK 属于主模块）、OpenAI model extension `v0.1.13`；本阶段不升级依赖、Go 或扩展组件。
- Conversation Main、Takeover Reply、WakeUp 已使用 `adk.NewChatModelAgent`、`adk.NewRunner` 和 `Runner.Run`。`RunADKLoop` 只允许保留为事件消费/领域结果投影层，不得再执行工具、调用下一次模型或重建通用循环。
- Native Cognition、Daily Review、Reflection、Initialization、摘要、媒体/视觉、日程、Embedding 等按当前已运行的固定 Task 模式保留；不因“也是 Prompt”强制改造成 Agent 或 Eino Workflow。
- `ADKCapabilityTool` 是唯一薄工具适配器；`CapabilityRuntime` 继续负责领域权限所需的上下文、Prepare、冻结、事务、幂等、Outbox、异步受理和最终结算。Tool callback 的成功不等于业务提交或可见发布。
- 当前生产默认注册由 `apps/core-go/internal/core` 的 Registry 构造，`internal/capability` 中仍存在重复实现；本阶段必须对账并收敛为单一生产权威，不能继续维护两套可执行 Registry/Runtime。
- 当前没有已授权的真实故障 run；若无法找到真实样本，必须使用受控故障注入验证定位能力，并明确标记为受控验证，不声称已定位线上根因。

## Requirements

### P8-01 原生协议清理

- ADK 模型工具请求只从 Eino `schema.Message.ToolCalls` 读取真实 ID、名称和 arguments。
- ADK 路径不得从正文 JSON、Markdown、reasoning 或 sidecar 猜测可执行 ToolCall，不补造 ID、不猜工具名、不静默修复畸形参数、不把缺失调用转成成功。
- 固定 Task 的严格 DTO/JSON 解析、非 ADK Task 的既有原生 ToolCall 映射和恢复所需领域解码继续保留，但不能成为 ADK 工具执行的第二权威。
- 缺失/冲突 ID、畸形参数、未知或未授权工具必须 fail closed，且能区分诊断阶段。

### P8-02 ADK 事件与结果投影

- `RunADKLoop` 的职责限定为构造/调用已配置的 Runner、消费 AgentEvent、记录事件来源并投影业务结果。
- Eino 已完成的 Tool 执行、结果回填、下一次模型调用不得在 Core 再做一遍。
- 技术错误、取消、`ErrExceedMaxIterations` 必须保持失败语义；正常文本、合法 direct return、deferred/accepted pending、失败和中断必须分别识别。
- 工具事实、工具结果和最终可见文本分别保存；工具回合后的空文本/错误不能回退到旧候选。WakeUp trace calls/results 必须完整保留且只结算一次。
- `ReturnDirectly` 只可用于已有终止型工具契约，不能代替事务提交、正式发布或整批权限校验；多工具同轮不能因一个直接返回而丢失其它事实。

### P8-03 工具适配与权限

- 正式链路固定为 Eino Agent 内部工具机制 → `ADKCapabilityTool` → `CapabilityRuntime` → 既有领域服务/结算，不在 Agent 外重复调用 ToolsNode。
- 适配器只负责正式 `ToolInfo`、`compose.GetToolCallID(ctx)`、授权运行范围、既有校验/执行和一次结果序列化；缺失 call ID 失败，不随机生成。
- 模型可见集合、`InternalOnly`、surface、owner/context scope、candidate validation 与实际可执行集合必须一致；策略能力与模型 ToolCall 分开统计。Main 持久切换、B 当轮归属、WakeUp/Reflection 禁用规则不得改变。
- 只读查询可立即返回真实结果；写入/输出/外部异步能力继续 Prepare、冻结、事务、幂等、Outbox 和异步完成边界。`deferred/accepted_pending` 不得伪装成已提交或网络失败。
- 对 `visual_identity.initialize` 按当前正式策略和 `InternalOnly` 实际归属记录并测试，不为了填满矩阵而公开额外权限。

### P8-04 Trace

- 复用现有诊断存储、查询和导出入口，不新建观测平台或第二 Runtime。
- 一次运行至少能关联：run/correlation、业务范围/agent/阶段、物理模型调用 ID、原生 tool_call ID/名称/参数摘要、工具进入/拒绝/未调度、Capability prepare/execution/business 状态、工具返回/序列化错误、下一次实际模型输入是否含对应结果、终止、正式提交和发布。
- 模型回合、物理模型调用、ToolCall、消息事件和重试分别计数；“模型收到结果”只能由下一次真实模型输入证明，合法 direct return 标记为不适用。
- 未知工具或调度失败即使未进入 `InvokableRun` 也必须能与模型响应/Runner 错误关联。默认只保存脱敏摘要和必要字段。
- 提供按 `run_id` 的最小只读导出/开发查询入口，能直接回答工具是否请求、获准、执行、仅准备、结果是否回填及最终失败阶段。

### P8-05 全量契约门禁

- 从生产构造/注册和实际调用方生成实际存在项；测试维护独立、受审查的期望关系和夹具，两者不允许由同一个生产权限函数自证。
- 每个正式 Agent 使用真实构造函数、真实 Eino Runner 和受控模型替身覆盖：无工具、单工具反馈、合法多回合、同轮多工具、未知/未授权、非法参数、模型错误、工具错误、取消、循环上限和已有终止型结果。
- 每个模型可见 Tool 覆盖 `Info`/Schema、参数、权限、ContextSlot、序列化、错误/取消；写 Tool 另覆盖 Prepare、拒绝、回滚、幂等、异步受理和正式提交。
- 每个允许的 Agent/阶段 × Tool 关系覆盖真实适配、业务处理/准备、结果返回和后续模型消费或合法终止；禁止关系证明不可见且无业务副作用。策略能力单独覆盖获准调用与模型不可见/不可执行。
- 每个固定 Task 登记输入、Prompt/Schema、输出解析和失败；Embedding 采用向量契约；不得为套件方便增加模型回合。
- 必过反例至少包括正文伪 tool_calls、缺失/冲突 ID、畸形参数、工具成功但最终解析失败、deferred 不误报提交、工具回合后空文本、上限错误、WakeUp trace 不重复、同轮多工具配对、B/后台人格能力拒绝和唯一正式发布入口。
- 核心运行器、模型支撑、Prompt/Schema、Tool 定义/权限、注册/构造、共享模块或 Eino 依赖变化时必须运行全量套件；原始测试事件、完整叶子测试名、真实退出码和 SKIP 原因必须可审计。

### P8-06 回归、证据与交付

- 回归普通对话、查询、人格接管/持久切换、WakeUp、Reflection、正式提交/Outbox、取消/幂等、Stream 生命周期、API 和 Worker 接线；不发真实消息、不修改生产、不清理共享数据卷。
- 按项目格式运行格式化、构建、单测、相关集成测试、`go vet` 和适用 race；最后一次源码改动后重跑对应测试，证据必须来自同一快照。
- 生成 `phase8-eino-native-report.md`、完整 Agent/Task/Tool/阶段覆盖矩阵、Conversation/WakeUp 成功 trace、工具失败与模型解析失败可区分的 trace、关键源码/测试连续摘录、原始脱敏测试事件、命令与退出码、HEAD/未提交变更指纹、依赖版本、文件大小和 SHA-256。
- 生成 `phase8-eino-native-evidence.zip`，包含主报告及实际证据文件；重新解压验证报告、相对引用、哈希、矩阵可读且无密钥/真实私人数据。哈希一致不等同于功能通过。
- 关键验收失败或环境缺失时返回明确非通过状态、已完成部分和具体阻塞，不恢复兼容分支、不伪造 PASS。

## Acceptance Criteria

- [x] 规划和实现均以当前 `HEAD=50db532` 与工作树快照为准，未覆盖或回退既有 WakeUp ADK trace 修复；锁定依赖未被升级。
- [x] 生产路径不存在第二个模型—工具循环、ADK 外重复 ToolsNode、正文/Markdown/reasoning 猜测执行 ToolCall、缺 ID 派生执行或旧 Runtime 回退；固定 Task 的合法 DTO 解析仍通过。
- [x] Conversation Main、Takeover Reply、WakeUp 的 Eino Agent/Runner 事件、ToolCall ID、Tool result、最终文本、空结果、工具/模型错误、取消和迭代上限语义有测试证据；WakeUp trace calls/results 可见且只结算一次。
- [x] 实际生产 Agent、固定 Task、模型可见 Tool、InternalOnly/策略能力和阶段关系形成独立期望矩阵；新增/缺失项、零匹配、叶子 SKIP、管道吞错均使门禁非零。
- [ ] 每个允许 Tool 关系至少有真实 Eino 适配和业务结果消费证据；写能力有隔离存储/集成层的 Prepare、回滚、幂等、异步受理和提交证据；不以统一 `return success` 替代业务处理。
- [ ] 同一受控运行可从 `run_id` 导出物理模型调用、原生 tool_call、授权/调度、Capability 状态、下一次模型输入回填、终止和正式提交/发布；工具失败与模型解析失败可区分，未知工具未进入 InvokableRun 也可定位。
- [x] 现有 Go/Web/静态质量门禁和本阶段全量矩阵按真实退出码通过；数据库/真实模型等缺失环境明确为 conditional/blocking，不被写成 PASS。
- [x] 主报告、矩阵、trace、原始事件和 hash manifest 打包为可重解压读取的 `phase8-eino-native-evidence.zip`，完成无密钥/私人数据扫描并记录证据快照。

## Out of scope

- 不升级 Eino、扩展组件或 Go；不 fork、复制或改写框架源码。
- 不迁移 Temporal，不把 Reflection、Judge、初始化、摘要、媒体/视觉、日程、Embedding 或其它固定 Task 为了“也是 Prompt”强制改成 Agent/Graph/Workflow。
- 不新增 Checkpoint、自由 Agent 转交、自主性机制、默认重试、模型降级、failover 或新的观测平台。
- 不重做前七阶段、不重命名/新增包装来制造完成、不删除合法领域解码、冻结、事务、幂等、Outbox、审计或发布机制。
- 不发真实消息、不改生产服务、不清理共享数据卷；真实模型兼容单独分层，不得阻塞可由受控模型验证的逻辑。

## Planning status

需求已由当前任务正文、现有源码、测试和历史 Phase 8 取证交叉核对；没有需要用户再决定的产品范围问题。`design.md` 和 `implement.md` 将把上述边界落实为可执行的代码/测试/证据顺序。规划完成后需先由用户审核，再运行 `task.py start` 进入实施。
