# 技术设计：白纸人格涌现与自我模型 v2

## Boundary Model

```text
interaction facts
  -> appraisal candidate
  -> affect/drive reducer
  -> memory consolidation candidate
  -> self-model candidate
  -> agency intention candidate
  -> policy gate / freeze
  -> durable projection + prompt summary
```

各层只消费上游的结构化事实，不从用户可见回复重新解析意图。模型负责提出受限候选和理由，服务端负责校验范围、证据门槛、幂等、冲突、衰减、权限和事务。

## State Separation

- `interaction facts`: 用户消息、已确认生活事实、已完成能力调用、关系变化和时间边界。
- `appraisal`: 当前互动的短期主观评价候选，带事件类型、方向、confidence、sourceMessageId 和 idempotencyKey。
- `affect/drive`: 当前即时状态与近期趋势；沿用现有 reducer，不允许模型提交任意数值 delta。
- `memory`: 稳定事实、偏好、经验、防御策略和证据链；每次升级保留前后版本。
- `self-model`: 摇光实例对自身能力、偏好、限制、经历和关系位置的有界摘要，不与 persona blueprint 或用户资料混表。
- `agency`: 主动询问、继续、延迟、拒绝等意愿候选；必须经过资格门控和冻结。

## White-Paper Initialization

白纸实例只拥有必要的身份、时区、权限、安全边界和空的涌现容器。预定义人格可以有初始 blueprint，但两者在运行时都读取统一的 `affect`、`memory`、`self-model` 和 `agency` 端口。初始值与互动沉淀必须在数据上可区分，避免把设计者设定误标为实例亲自形成的经验。

## Prompt Contract

聊天 prompt 只接收经过预算控制的摘要：近期情绪趋势、已确认偏好、当前自我摘要、关系摘要、可用能动性边界和待确认候选。原始事件、内部分数、审计字段、推理文本和凭证不进入普通用户消息。

## Transaction And Recovery

- 候选计划阶段只读，不产生副作用。
- 应用阶段在 caller-owned transaction 中同时提交事实、状态投影、记忆/自我候选和幂等记录。
- 主动意愿在投递前再次执行资格门控，并以冻结 decision 防止重试重复调用模型。
- 所有 reducer、consolidation 和 self-model 更新支持 CAS 或 revision guard，服务重启后可从事件和快照恢复。

## Compatibility

新增能力通过现有 flow registry、capability dispatcher、debug trace 和 prompt-run 记录接入；不在聊天 SSE 中暴露内部候选或工具 JSON。预定义人格的静态字段继续由当前分析器和 persona lifecycle 负责，涌现层以增量投影叠加。
