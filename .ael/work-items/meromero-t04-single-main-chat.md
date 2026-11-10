---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T04，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 7. T04：调整主对话链，迁移而不是丢弃认知结算\n\n## 7.1 主链\n\n普通聊天：\n用户消息 → MeroMero 主对话 Agent → 正式发布。\n\n需要能力：\nMeroMero → 真实 ToolCall → Plugin/Core → 真实结果 → 按需继续决策和表达。\n\n不再固定：\nQwen 认知 → Writer；\nMeroMero 意图 → Qwen 代理 → MeroMero 文案。\n\n确定性插件不必再调用语言模型。\n插件内部确有提示词规范化等任务时，按其独立配置调用。\n\n## 7.2 正文生成与发布边界\n\n核对 conversation.reply、moment.publish 和现有 Writer 的真实职责。\n\n如果正文已经由负责表达的 Agent 生成：\n- 发布能力不得无条件再调用另一个 Writer 改写一次；\n- 不为了沿用旧接口保留隐式二段生成；\n- 仍复用已有唯一发布入口、持久草稿、媒体关联与幂等机制。\n\n允许独立的动态、日记、异步媒体附言 Agent。\n它们是任务不同，不是每条聊天的固定后处理。\n\n## 7.3 保留认知结算\n\n逐项核对现有：\n- claims；\n- influences；\n- output_preference_decision；\n- goal_event_candidates；\n- 其他认知、偏好、关系、Goal 和事件字段。\n\n为每项明确：\n- 原生产者；\n- 原消费者；\n- 迁移后生产者；\n- 触发时机；\n- 证据来源；\n- 一致性和幂等。\n\n不能因为旧认知 Agent 不再主导聊天，就让这些字段永久为空或丢掉下游消费。\n\n不需要阻塞聊天的派生分析，可以使用现有持久任务机制在已提交消息/事件之后处理。\n必须真实落地，不是无持久化保障的随意 goroutine。\n结果不能反过来伪造先前已经发生的行动或关系。\n\n## 7.4 保持交互\n\n保留现有流式显示、消息状态和新 UI 交互。\n涉及完成声明或附件的部分，在通过对应核验前不得作为正式内容落库/发布。\n\n不要为了安全，把所有普通聊天都改成固定模板或多模型审核。\n异步任务可以用真实任务状态展示进度，不伪造角色的动作和完成台词。\n\n验收：\n- 不需要工具的聊天不经过 Qwen；\n- 查询和行动仍能通过原生 ADK Loop；\n- 同一正文不会生成两遍、发布两遍；\n- 旧认知结算的业务效果没有丢失。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T04：主对话链与认知结算迁移，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t04-single-main-chat",
  "title": "T04 主对话链与认知结算迁移",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 7. T04：调整主对话链，迁移而不是丢弃认知结算",
    "原始 T04 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "默认普通私聊使用主 Agent 和原生工具循环，不再固定进入独立 Writer 或第二认知模型",
    "真实动作、消息、回执与旧认知结算保持一致、持久及幂等，旧草稿和在途恢复兼容",
    "保留 claims、influences、偏好、Goal 候选及关系等实际消费者，不由台词伪造动作",
    "用隔离数据库及真实 MeroMero 分别验证普通私聊、工具循环、事务和恢复"
  ],
  "priority": 4,
  "dependencies": null,
  "parent": "meromero-first-humanization-plugin-governance",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "model",
      "id": "t04-review",
      "kind": "review",
      "command": null,
      "covers": [
        "默认普通私聊使用主 Agent 和原生工具循环，不再固定进入独立 Writer 或第二认知模型",
        "真实动作、消息、回执与旧认知结算保持一致、持久及幂等，旧草稿和在途恢复兼容",
        "保留 claims、influences、偏好、Goal 候选及关系等实际消费者，不由台词伪造动作",
        "用隔离数据库及真实 MeroMero 分别验证普通私聊、工具循环、事务和恢复"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t04-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "默认普通私聊使用主 Agent 和原生工具循环，不再固定进入独立 Writer 或第二认知模型",
        "真实动作、消息、回执与旧认知结算保持一致、持久及幂等，旧草稿和在途恢复兼容",
        "保留 claims、influences、偏好、Goal 候选及关系等实际消费者，不由台词伪造动作",
        "用隔离数据库及真实 MeroMero 分别验证普通私聊、工具循环、事务和恢复"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry",
    "legacy-9b89c41b4be449b963dcba95"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# T04 主对话链与认知结算迁移
