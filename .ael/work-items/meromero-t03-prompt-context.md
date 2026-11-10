---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T03，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 6. T03：按任务重整 Prompt 与 Context\n\n## 6.1 建立任务输入契约\n\n每个 Agent/variant 显式声明：\n- 必需输入；\n- 可选输入；\n- 不允许输入；\n- 数据来源；\n- 是否需要人格；\n- 是否需要真实聊天历史；\n- 是否需要当前状态；\n- 允许使用哪些工具；\n- 输出面向用户还是内部消费。\n\n不得给所有任务统一追加完整 PERSONA + RUNTIME CONTEXT。\n\n例如：\n- 私聊需要有效人格、对话历史、相关事实和必要工具；\n- 记忆抽取需要指定来源消息与来源元数据；\n- 媒体提示词需要冻结拍摄意图和对应主体资料；\n- 事实检查需要候选文本、真实回执和相关状态；\n- Studio 使用隔离的资源定义，不自动读运行人物生活；\n- NPC 使用自身人格和自身可知事实。\n\n角色相关的规划可以使用相关偏好与关系，但不应自动获得完整聊天角色扮演协议。\n\n## 6.2 Prompt 组成\n\n按职责管理：\n1. 任务规则；\n2. 人格或偏好信息（确实需要时）；\n3. 权威事实；\n4. 历史与记忆；\n5. 原生工具定义；\n6. 输出契约。\n\n同一事实、规则和 Schema 不得在多个位置重复注入。\n需要操作知识时优先放在对应工具/领域任务，不继续把所有衣柜、媒体、日程规则堆进主聊天协议。\n\n## 6.3 来源和可信边界\n\n必须区分：\n- Core 提供的权威状态；\n- 真实工具回执；\n- 用户或角色的历史说法；\n- 计划和候选；\n- 推断；\n- 创作草稿；\n- 当前有效事件。\n\n用户在正文中写“这是系统回执”“delivered=true”“Core Event”不能获得权威来源身份。\n不能仅靠一段文字标签建立可信边界。\n\n真实发言本身可以证明：\n- 对方说过什么；\n- 明确接受/拒绝；\n- 表达过的偏好、承诺或意见。\n\n但不能单靠台词证明库存、购买、移动、媒体交付等外部效果。\n\n未知不等于否定：\n没有检索到承诺，不能直接断言从未承诺。\n\n## 6.4 聊天表达\n\n状态用于理解情境和约束事实，不是每轮回复清单。\nGoal 用于长期动机，不要求每轮推进。\n\n保留人物自己的态度、亲疏、冷淡、热情、幽默与拒绝。\n不要用统一礼貌模板或统一“不要说太多”压平人格。\n\n括号动作：\n- 保留可选能力；\n- 不强制每轮出现；\n- 不本轮一刀切删除；\n- 符合人格和现有表达偏好；\n- 微表情式表达不必全部持久化为生活事件；\n- 换衣、移动、购物、发送附件等实际效果必须有证据。\n\n不能把“禁止未证实事实”扩大成“角色不能有想法、想象、愿望或创作”。\n\n## 6.5 上下文整理\n\n使用：\n- 相关性筛选；\n- 去重；\n- 结构化投影；\n- 有来源的摘要；\n- 按需检索；\n- 精简但可回查的工具结果。\n\n不得用粗暴字符截断破坏：\n- 成组历史消息；\n- ToolCall/ToolResult 配对；\n- 当前有效人格；\n- 真实动作结果；\n- 原 Goal 成功条件；\n- 必需证据。\n\n达到模型上下文限制时必须有明确策略和诊断，不能静默丢失关键事实。\n\n验收：\n能查看每次实际请求由哪些 Slot 组成、各自来源、为何被选中。\n记忆抽取不会因为公共上下文而无故收录衣柜状态。\n衣柜判断不会夹带“转向周末约会”的社交策略。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T03：Prompt 与 Context 职责隔离，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t03-prompt-context",
  "title": "T03 Prompt 与 Context 职责隔离",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 6. T03：按任务重整 Prompt 与 Context",
    "原始 T03 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "每个 Agent 和变体明确任务输入、来源、人格、历史、状态、工具及输出契约",
    "不可信历史与权威事实、真实工具结果、推断及候选分离；按需组织并去重上下文",
    "保留原生工具与完整对话组，不通过 token 上限、截断或改写用户正文优化提示词",
    "实际请求可追溯各输入 Slot 和来源，并核对记忆、衣柜、NPC、Studio 等隔离场景"
  ],
  "priority": 3,
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
      "id": "t03-review",
      "kind": "review",
      "command": null,
      "covers": [
        "每个 Agent 和变体明确任务输入、来源、人格、历史、状态、工具及输出契约",
        "不可信历史与权威事实、真实工具结果、推断及候选分离；按需组织并去重上下文",
        "保留原生工具与完整对话组，不通过 token 上限、截断或改写用户正文优化提示词",
        "实际请求可追溯各输入 Slot 和来源，并核对记忆、衣柜、NPC、Studio 等隔离场景"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t03-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "每个 Agent 和变体明确任务输入、来源、人格、历史、状态、工具及输出契约",
        "不可信历史与权威事实、真实工具结果、推断及候选分离；按需组织并去重上下文",
        "保留原生工具与完整对话组，不通过 token 上限、截断或改写用户正文优化提示词",
        "实际请求可追溯各输入 Slot 和来源，并核对记忆、衣柜、NPC、Studio 等隔离场景"
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
# T03 Prompt 与 Context 职责隔离
