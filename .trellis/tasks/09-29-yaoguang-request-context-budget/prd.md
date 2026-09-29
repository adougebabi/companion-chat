# 摇光完整请求上下文收敛与 Token 预算闭环

## Goal

沿既有 Prompt/Slot、投影、摘要、检索与 Eino 原生 Agent Loop，约束每次真正发送给模型的完整请求，解决 16K 环境中输入贴近窗口上限的问题。交付代码、自动化测试和可复核的前后对比报告；保留人格、生活连续性、工具真实性与独立调用能力。

## Confirmed background

- 会话经 `RunConversationCognitionAgent`、`BuildContextProjectionFor`、`assembleProjectionPromptForSurface`、`RunADKStructuredTask` 到 Eino `RunADKLoop`；每次物理调用由 `queuedToolCallingChatModel` 发出。证据：`apps/core-go/internal/core/cognition_agent.go:61`、`provider_context.go:57`、`eino_model_runtime.go:72`。
- Eino 固定为 v0.7.37，原生循环维护工具调用与结果。现有刷新器在模型调用前替换 System/Runtime Slot，且已有物理调用预算估算。证据：`apps/core-go/go.mod:7`、`runtime_context_refresh.go:147`、`eino_model_runtime.go:176`。
- 当前输出预留只进入预算计算，直接与 ADK 调用的 `MaxCompletionTokens` 均为 0；物理输出上限未与预留一致。证据：`eino_model_runtime.go:578`、`:654`、`internal/ai/model/eino_runtime.go:53`。
- 计数为启发式而非实际 tokenizer，默认窗口 131072；16K 必须按实际部署角色配置检验。证据：`prompt_context_assembler.go:14`、`:166`。
- 摘要有来源窗口校验，选中后才排除完整覆盖原文；记忆去重主要依赖来源 ref 或精确内容键。证据：`conversation_summary.go:544`、`prompt_context_assembler.go:308`、`working_memory.go:250`、`memory_lifecycle.go:131`。
- 工作区已有用户未提交的协议、上下文、引用编码和工具适配修改；保留这些修改并在其基础上工作。

## Requirements

1. 对所有支持入口，按实际模型/模板/窗口/输出上限核算完整请求：System、Persona、Runtime、当前输入、历史、摘要、记忆、Tool Call/Result、Tool Schema、实际进入模板的输出约束及模板开销。每个 Eino 续接轮次复核。估算须标明模式，不能称作精确计数。
2. 按现有配置优先级计算硬限。16K 起始校验为窗口 16384、生成预留 4096、安全余量 512、有效输入硬限 11776，普通首轮约 8000 为目标。实发 Provider 输出上限与预留一致，显式冲突返回错误，不静默缩小。
3. 保留 Eino 原生循环、Agent/Tool 权限和独立调用。状态刷新只替换稳定 Slot，保护完整工具调用/结果单元及真实操作 ID，不重复执行副作用 Tool。
4. Runtime 使用任务相关当前状态投影，不完整 dump 领域对象；保留有效外观/穿着/活动、相关关系、动态目标与意愿、期限和推断属性。Working Persona 承担稳定人格及切换规则，Current State 承担动态事实。不修改原始业务状态。
5. 近期原文、适用滚动摘要和检索记忆有来源覆盖边界。保留用户纠正、承诺、有效待办与操作引用；不同真实事件不按文本相似合并；摘要失败不推进覆盖；无变化 WakeUp 不持续制造同义可召回记忆。
6. 大工具结果提供真实且授权可读取的精简视图或分页，保留状态、错误、必要 ID 与继续读取方式。原始收据不变。
7. 保留人格接管与持久切换、主动性、生活连续性和行动真实性。不得升级依赖、替换 Agent Loop、修改模型服务内存保护、删除用户数据、自动发布或推送。
8. 记录各物理请求的输入前后、分项、模型与计数模式、预算、收敛动作、可得 Provider usage；默认日志不泄露完整聊天或敏感结果。分类报告预算错误，并交付 Markdown 验收报告。

## Acceptance criteria

- [ ] 中英混合、长 ID、时间戳、结构化字段、Schema/结果/输出约束都计入最终预算；边界/略超限、巨大当前输入和配置冲突有明确结果。
- [ ] 相同输入重复组装不增长协议/Runtime；至少三轮工具续接逐轮受控；状态刷新只替换 Slot，调用与结果配对合法且不重复执行。
- [ ] 30 轮历史输入不无界增长；摘要/检索去重可由来源核验；跨天有效事项仍能进入上下文。
- [ ] 当前短发覆盖旧长发初始化；购买意愿不等于持有；已生成未发送图片不等于已发送；推断不等于确认；人格切换/接管仍有效。
- [ ] 大列表可继续读取，ID 和 failed/pending/completed/sent 状态保真；独立 Tool 调用通过回归。
- [ ] 同一样本报告协议、Persona、Runtime、历史、Schema、工具结果及总输入前后对比；目标不通过删当前输入、禁工具或删必要事实达成。
- [ ] 报告链路、根因证据、修改职责、旧机制复用/移除、预算配置、计数限制、测试命令与层级、未执行项、剩余绕过路径和风险。

## Out of scope

整体框架或工作流重构；人格、关系、生活状态机重写；独立记忆/工具发现平台；默认新增相关性 LLM 调用；mlx-serve 参数、模型、并发或内存保护修改。
