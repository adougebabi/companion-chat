# LLM结构化回合协议与人格状态记忆

## Goal

将 LLM 与后台之间的业务交互从“主要返回自然语言文本、后台再解析或猜测附加语义”升级为版本化、可校验、可审计的结构化回合协议，使一次对话回合可以同时表达：用户可见回复、有限的情绪状态事件、驱动力信号、记忆写入候选以及需要后台执行的能力调用。

协议必须服务于现有的本地 Node/SQLite/Vue companion 架构，不把模型输出直接当成数据库命令，也不要求第一阶段改变浏览器现有 SSE `token`/`done`/`error` 事件。

## Confirmed Facts

- 当前 provider 请求本身已经是 JSON，但模型主要回复仍从 `choices[0].delta.content` 或 `choices[0].message.content` 读取文本；只有原生 `tool_calls` 才是结构化业务输出。
- 后台已经有 normalized completion、`StepResult`、capability dispatcher 和 transaction commit boundary，但尚未有统一的严格 JSON turn envelope。
- 当前聊天外部协议是 SSE `token`、`done`、`error`；`done.messages` 是权威消息集合，`done.message` 是兼容别名。
- 当前 `<media-intent>`、`<pending-event>` 兼容路径依赖文本标记解析，不应成为新状态/记忆能力的长期协议。
- 当前 `companion_persona_states.mood` 是字符串生活状态，不是一等 PAD 情绪模型；当前没有 affect/drives snapshot 或 event 表。
- 当前普通聊天不会自动写入 `companion_memories`；已有 memory repository 支持 persona-scoped list/insert/delete，活动评论是现有生产写入来源。
- 当前所谓 10 分钟主要是关系演化 debounce 窗口；项目已有决策不做每 10 分钟全量扫描，也不在每条用户消息后额外调用 LLM 猜测记忆。
- 现有生活事件、主动消息、timeline、睡眠延迟、job lease、幂等和 persona-private 记忆边界应继续作为状态和副作用的治理边界。

## Requirements

### R1. Versioned structured turn contract

- 定义 `companion.turn.v1` 或等价的版本化 JSON Schema，至少覆盖 `messages`、`affectEvents`、`driveSignals`、`memoryWrites` 和 `capabilityCalls` 的边界。
- 模型提出的状态、记忆和能力调用只能作为候选/意图，必须经过后台 Schema、persona、长度、数量、枚举、幂等和安全策略验证。
- 不把模型完整推理过程、内部 prompt、API 凭据或未裁剪的调试信息放入业务协议。

### R2. Affect and drives integration boundary

- 为 PAD 和 drives 预留结构化事件边界，第一阶段优先使用有限事件类型，由服务器映射到受限 delta，不允许模型直接写任意数值。
- 第一阶段固定支持 `social`、`exploration`、`rest` 三项 drives；drives schema 和持久化字段必须可扩展，以后可以增加新类型而不破坏已有快照和事件。
- drives 统一使用 `0..1` 的 pressure 语义：越高表示需求越未满足；每个人格可以配置三项 drive 的权重、基线和半衰期。
- PAD 使用 persona-specific baseline、范围限制和惰性衰减；drives 具有明确的满足、累积和衰减规则。
- 状态事件必须关联 persona、来源消息/事件、因果 ID、幂等键和模型/规则版本。

### R3. In-turn memory capability

- 提供 persona-scoped 的 `memory_event` 或等价结构化能力，支持受限的 insert/upsert、confidence、source message 和 idempotency 信息。
- 普通聊天只有在模型显式调用 `memory_event` 时才产生长期记忆写入；后台不对每条消息追加隐式 LLM 记忆提取调用。
- 记忆写入不能跨人格，不能覆盖用户可审计/可删除的来源信息，不能因模型格式错误而伪造已保存结果。
- assistant message、memory event 和相关状态/effect 在可行时通过现有 commit boundary 同一事务提交。

### R4. Provider and compatibility migration

- provider/application 边界负责把严格 JSON、原生 tool call 和现有兼容文本路径归一化为同一个内部 turn result。
- 保持现有 SSE 事件名和 `done.messages`/`done.message` 兼容；结构化控制数据优先作为后台内部结果或 `done` 的安全可选扩展。
- 采用混合协议：用户可见回复继续使用现有文本流/`content`，情绪、驱动力、记忆和能力控制数据使用严格 JSON；控制 JSON 必须在完整、校验成功后才执行副作用。
- 结构化解析失败、Schema 不合法、provider 不支持或重复提交必须有界失败/回退行为，不得静默执行半截副作用。

### R5. Deterministic behavior policy

- PAD/drives 只影响回复姿态、延迟/长度倾向、主动候选权重和表达策略，不能绕过静默、屏蔽、预算、租约、内容安全和 persona 隔离规则。
- 服务器必须保留有限的 reason code、状态事件来源和治理拦截结果，供 development-only inspector 审计；普通 UI 不显示原始内部数值或推理。

## Acceptance Criteria

- [x] 存在版本化 turn envelope 的 JSON Schema、字段所有权、大小/数量上限和错误语义文档。
- [x] 旧 text completion、原生 tool call、结构化 JSON completion 和兼容 marker 都能被归一化，且 capability 副作用只由通过验证的结构化结果触发。
- [x] 现有 SSE `token`、`done`、`error` 以及 `done.messages`/`done.message` 兼容测试继续通过。
- [x] 结构化解析失败不会写入半条 memory、affect event、capability job 或用户可见的虚假成功结果。
- [x] `memory_event` 能以 persona-scoped、幂等、可审计方式提交，并能在提交失败时整体回滚。
- [x] PAD/drives 事件边界支持固定事件类型、基准线、范围限制、惰性衰减和状态快照的一致性测试。
- [x] 状态和记忆结果不跨人格，且不泄露内部 prompt、完整模型推理或 provider 凭据。
- [x] migration、provider 失败、模型返回 malformed JSON、重复调用、服务重启和旧客户端兼容均有验证方案。

## Out of Scope

- 第一阶段不替换浏览器现有 SSE 为新的顶层事件体系。
- 第一阶段不要求所有模型都支持复杂的增量 JSON streaming。
- 不实现每 10 分钟扫描所有人格或每条消息额外调用一个 LLM 的自动记忆提取器；普通聊天的长期记忆必须由显式 `memory_event` 产生。
- 不允许用户或模型直接编辑 foundation；不把 PAD/drives 写入现有关系 patch 字符串。
- 不在普通 UI 暴露原始 PAD 数值、完整内部决策链或 provider 请求细节。
- 不在本任务中实现图像编辑或媒体 prompt 业务改造；媒体能力只作为兼容 capability 的现有消费者。
