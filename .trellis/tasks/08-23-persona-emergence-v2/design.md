# 技术设计：白纸人格涌现与自我模型 v2

## Boundary Model

```text
shared runtime (both initialization modes)
  -> interaction facts
  -> appraisal candidate
  -> affect/drive reducer
  -> memory consolidation candidate
  -> self-model candidate
  -> agency intention candidate
  -> policy gate / freeze
  -> durable projection + prompt summary
```

各层只消费上游的结构化事实，不从用户可见回复重新解析意图。模型负责提出受限候选和理由，服务端负责校验范围、证据门槛、幂等、冲突、衰减、权限和事务。

情感反馈、记忆沉淀、自我模型和主体性属于共享运行时能力，不属于某一种初始化模式。初始化模式只决定初始 blueprint/identity anchors、初始涌现容器和候选证据门槛。

## State Separation

- `interaction facts`: 用户消息、已确认生活事实、已完成能力调用、关系变化和时间边界。
- `appraisal`: 当前互动的短期主观评价候选，带事件类型、方向、confidence、sourceMessageId 和 idempotencyKey。
- `affect/drive`: 当前即时状态与近期趋势；沿用现有 reducer，不允许模型提交任意数值 delta。
- `memory`: 稳定事实、偏好、经验、防御策略和证据链；每次升级保留前后版本。
- `self-model`: 摇光实例对自身能力、偏好、限制、经历和关系位置的有界投影；长期人格特征以带 `category/polarity/strength/confidence/evidenceRefs/revision/decayPolicy/status` 的结构化特征主张保存，自然语言摘要由主张派生，不与 persona blueprint 或用户资料混表。
- `agency`: 主动询问、继续、延迟、拒绝等意愿候选；必须经过资格门控和冻结。

## User Governance

- 运行时沉淀默认全自动，不要求逐条用户确认，不因等待确认而阻塞聊天、记忆或主动行为流程。
- 普通用户界面提供已沉淀内容的查看、修改、删除和回滚入口；debug 视图额外显示证据链、候选来源、revision、衰减和冲突状态。
- 用户修改、删除或回滚不覆盖原始模型候选，而是写入新的 user-authored revision，并保留因果链和审计历史；prompt/context 只读取最新有效 revision。
- foundation、权限、安全边界和用户事实仍不受自动涌现写入影响，用户治理入口也必须经过对应的显式版本化流程。

## Agency Explanation

- LLM 可以自由生成符合上下文的拒绝/延迟解释，但必须与结构化 agency decision 同步提交，不能让自然语言单独触发副作用。
- 服务端保存有限的 `reasonCategory`、`evidenceRefs`、`gateResults`、`status` 和 redacted `explanationSummary`；不保存完整 hidden reasoning 或内部提示词。
- 普通聊天仅展示经过安全筛选的 LLM 解释；专用查看入口可以展示原因摘要、证据和门控结果，但仍不展示凭证、内部数值或完整推理。

## Initialization Modes

- `llm_defined` 继续使用现有严格分析器和预定义人格创建流程，产出的 blueprint/foundation 作为外部初始资料。
- `blank_slate` 复用现有顶层 `name`、`role`、`timezone`、`language`、`permissions` 和 `safetyBoundaries` 字段并允许身份字段为空；不使用服务端默认人格，不把创建请求中的性格描述写入初始自我模型。
- `initializationMode` 是显式模式字段，不额外引入 `identityAnchors` 嵌套容器，以保持 persona DTO、prompt/context 和调试入口的兼容性。
- 两种模式在运行时都读取统一的 `affect`、`memory`、`self-model` 和 `agency` 端口。初始值与互动沉淀必须在数据上可区分，避免把设计者设定误标为实例亲自形成的经验。

## Prompt Contract

聊天 prompt 只接收经过预算控制的摘要：近期情绪趋势、已确认偏好、当前自我摘要、关系摘要、可用能动性边界和待确认候选。人格摘要由结构化特征主张派生；Big Five/MBTI 仅可作为可撤销的视图，不作为 prompt 中的权威事实。原始事件、内部分数、审计字段、推理文本和凭证不进入普通用户消息。

v2 首版普通 UI 不提供 Big Five/MBTI 坐标视图，只提供自然语言摘要、结构化特征主张和证据回溯；坐标视图属于后续可选分析能力。

## Transaction And Recovery

- 候选计划阶段只读，不产生副作用。
- 应用阶段在 caller-owned transaction 中同时提交事实、状态投影、记忆/自我候选和幂等记录。
- 主动意愿在投递前再次执行资格门控，并以冻结 decision 防止重试重复调用模型。
- 所有 reducer、consolidation 和 self-model 更新支持 CAS 或 revision guard，服务重启后可从事件和快照恢复。

## Compatibility

新增能力通过现有 flow registry、capability dispatcher、debug trace 和 prompt-run 记录接入；不在聊天 SSE 中暴露内部候选或工具 JSON。预定义人格的静态字段继续由当前分析器和 persona lifecycle 负责，涌现层以增量投影叠加。
