# 交互与 LLM 请求调度优化

## Goal

让聊天输入、模型请求调度和诊断信息在桌面端与移动端都可预测、可观察、可恢复：用户发送后立即得到完整回复，所有 LLM 请求按统一队列和优先级执行，并能在诊断中心看到从排队到完成的全过程。

## Background / confirmed facts

- 当前浏览器客户端是 Vue 3 + TypeScript，聊天输入位于 `apps/web/src/views/ChatView.vue`，发送按钮在独立的 `.composer-footer` 行；`apps/web/src/styles/app.css:183-190` 也把 footer 作为单独布局。
- `ChatView.vue:33-39` 只有在 `store.sending` 已经结束且不可重试时才清空 `draft`，发送期间或失败时不会清空。
- `apps/web/src/stores/conversations.ts:236-435` 通过 NDJSON 读取 `token`/`message`/`completed` 事件并增量更新消息，流结束后再次读取完整消息页；目前没有对不完整/终止帧做客户端可见的“流已完成”保护。
- Go Core 的模型调用集中在 `apps/core-go/internal/core/provider.go`：`Structured*`、`Text`、`StreamText`、`Embed` 通过 `assignment(ctx, role)` 查找 `public.model_roles`。
- 当前数据库和设置页面把 6 个业务场景当成可绑定角色：`initialization`、`cognitive_assessment`、`action_realization`、`reflection`、`embedding`、`media_prompt`（见 `provider.go:51-67`、`SettingsView.vue`）。
- 当前实际触发场景至少包括：回复生成（`action_realization`）、认知判断/对话决策（`cognitive_assessment`）、原生事件认知（`native_cognition`）、媒体提示词（`media_prompt`）、反思（`reflection`）、定期唤醒（复用 `cognitive_assessment`）、初始化（`initialization`）、Embedding（`embedding`）、日评/计划生成（复用 `cognitive_assessment`），以及自治动作的可见回复（复用 `action_realization`）。
- 诊断模型运行只在 Provider 调用完成或失败后由 `recordModelRun` 写入 `diagnostic_model_runs`（`apps/core-go/internal/core/diagnostics.go:54-85`），因此当前无法显示排队中或执行中的请求。
- 已有工作流 dispatcher 的业务优先顺序（计划/唤醒/日评/自治/能力/媒体/反思），但它只调度 Temporal 工作流，不限制或观察 Provider 内部的 LLM 并发。
- `ProcessReflection` 在 `apps/core-go/internal/core/workflow_ops.go:255-375` 中负责消费已处理 cognition fact 并推进 reflection window；`ProcessWakeUp` 在 `apps/core-go/internal/core/wakeup.go` 中负责内部唤醒、动作冻结及创建 reflection intent。唤醒失败后的 durable retry/reconcile 已存在，但需要验证失败路径是否会被卡成不可再次唤醒。

## Requirements

### R1. 聊天输入交互

- 桌面端和移动端输入框与发送按钮必须在同一行；输入框在可用宽度不足时可收缩，按钮保持可点击的触摸目标。
- 用户提交非空消息后立即清空输入框，不因后端正在执行、排队或最终失败而把已提交文本留在编辑框中；失败内容仍由现有“可重试”状态保留。
- 保留 Enter 发送、Shift+Enter 换行、取消和重试能力。

### R2. 模型绑定与场景可见性

- 设置页面的“模型角色绑定”只展示两个绑定目标：`通用 LLM` 与 `Embedding`。
- 所有生成式场景统一使用 `通用 LLM` 绑定；Embedding 继续使用独立绑定。禁止因某个场景失败而隐式切换到另一个模型。
- 每次 LLM 请求必须携带并持久化可读的触发场景（至少覆盖：回复、认知判断、原生事件认知、媒体提示词、反思、唤醒、初始化、日评/计划；如发现新的实际场景需在实现说明中列出并归类）。场景信息在诊断中心可见，不改变绑定目标。
- 旧数据库中按业务场景命名的绑定必须有兼容/迁移策略，不能让升级后请求因找不到 `generic_llm` 而静默失败。

### R3. LLM 队列、优先级与并发

- 所有生成式和 Embedding Provider 请求都必须经过统一队列；队列至少保证 FIFO 同优先级、全局有界并发、取消/超时后释放执行槽。
- 优先级固定为：回复 > 认知判断 > 媒体提示词 > 反思 = 唤醒 > 初始化；日评/计划归入认知判断，Embedding 作为独立低优先级场景（具体数值和是否与生成式共享并发由设计文档落定）。
- 允许通过运行设置查看和修改并发数量，设置有安全上下限和默认值；非法值不应导致无限并发或服务不可用。
- 排队、执行、完成、失败、取消、超时均需保持请求关联 ID 和场景信息，且取消/失败不能阻塞同队列后续请求。

### R4. 诊断中心模型运行队列

- 诊断中心模型运行列表必须展示尚未返回的请求，并显示状态：`排队中`、`执行中`、`已完成`；失败、取消、超时也要有明确状态。
- 每条记录至少显示：触发场景、绑定目标/实际模型、状态、创建时间、开始/结束时间（若有）、关联 ID；已有脱敏 Prompt/Response 展示继续保留。
- 页面刷新或重新进入诊断中心后，队列记录仍以服务端状态为准；不能只依赖浏览器内存。

### R5. 对话流完整性

- 发送后页面无需刷新即可展示完整 assistant 句子；不得只显示最初 2–3 个字符或依赖最终刷新补齐。
- 客户端必须正确处理拆分 UTF-8、拆分 NDJSON 行、终止帧和错误帧；流异常时显示可重试状态并避免留下半截 assistant 草稿。
- 服务端流式响应继续保持单调 sequence 和明确 terminal event，排队状态不得破坏现有取消传播。

### R6. 反思、唤醒与失败恢复排查

- 在代码/文档中明确反思与唤醒的职责边界：唤醒是周期性内部生命触发（注意/想法/愿望/行动判断），反思是基于已处理 cognition evidence 的窗口化长期学习/候选记忆更新；唤醒可触发反思，但二者不是同一请求。
- 修复或补强“唤醒一次失败后不再唤醒”的路径：活跃/暂停实例的 wake-up intent 在 Provider/工作流失败后必须可重试，重试不能重复提交同一 cycle 的已持久化事实或动作。
- 为上述失败恢复和区别添加自动化测试，覆盖 Provider 错误、Temporal 终态、dispatcher/reconcile 和重复 cycle。

## Acceptance Criteria

- [x] ChatView 在桌面和移动断点下的输入框/发送按钮同一行，提交后编辑框立即为空；现有键盘、取消、重试测试通过。
- [x] 设置页只出现“通用 LLM”和“Embedding”两个绑定项；生成式调用均能在诊断记录中显示真实触发场景；旧角色数据有明确兼容行为。
- [x] 并发设置可读写且受上下限约束；并发超过上限时新请求稳定处于排队状态，优先级顺序和同优先级 FIFO 可通过测试观察。
- [x] 诊断中心能在请求尚未返回时显示排队中/执行中，完成后转为已完成，失败/取消/超时状态可见；刷新后记录不丢失。
- [x] 聊天流在真实或测试用 NDJSON 分片（含中文 UTF-8）下无需刷新即可得到完整句子，流错误可重试且不会残留半截回复。
- [x] 反思/唤醒职责说明落入项目文档或代码注释；唤醒失败后下一次调度可再次执行，且 cycle/事实/反思 intent 不重复。
- [x] 运行相关 Go、BFF、浏览器测试及生产构建/静态预览检查通过，未改动用户已有的未提交 Trellis spec 文件。

## Product decision

- 队列并发数量分别配置：生成式 LLM 与 Embedding 各有独立上限。默认值、上下限和设置键由技术设计确定；两类队列互不占用对方的执行槽。
