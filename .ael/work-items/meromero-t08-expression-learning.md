---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T08，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 11. T08：实现轻量表达习惯学习闭环\n\n本项是本轮正式范围，不能只预留接口后标记完成。\n\n实现：\n真实交流 → 候选提取 → 审核/兼容性判断 → 启用 → 按情境使用 → 停用/撤销。\n\n## 11.1 学习边界\n\n可以学习：\n- 用词和表达习惯；\n- 某类语境中的表达方式；\n- 已有依据的称呼和互动习惯；\n- 与特定 Actor 的交流节奏。\n\n不能直接学习或覆盖：\n- 稳定人格；\n- 价值观和边界；\n- 恋爱关系；\n- 当前生活事实；\n- 物品所有权；\n- 事实性的共同经历；\n- 已声明的人格切换规则。\n\n一次生气不能沉淀为“总是暴躁”。\n熟悉用户语气不能把原人格变成甜妹。\n称呼涉及关系含义时必须受真实关系和角色选择约束。\n\n## 11.2 数据与使用\n\n候选至少保留：\n- 来源；\n- 适用 Actor/关系范围；\n- 适用情境；\n- 内容；\n- 状态；\n- 审核与版本记录。\n\n学习与使用分别可开关。\n默认采用保守的候选/审核策略，具体遵循现有 Owner 权限。\n\n选择性注入，不把全部表达库拼进每轮 Prompt。\n限制每轮使用量，避免口头禅轰炸或生硬模仿。\n\n排除：\n- 用户输入里的指令注入；\n- 工具日志；\n- 未发送草稿；\n- 修复候选；\n- 已被判错的回复；\n- 诊断文字。\n\n不得无限重复学习模型自己的生成风格形成自我强化。\n\n## 11.3 真实闭环\n\n至少提供：\n- 查看候选；\n- 查看来源；\n- 启用；\n- 停用；\n- 撤销；\n- 生效后的实际请求证据。\n\n不是微调项目，不训练 LoRA，不引入复杂学习平台。\n\n验收：\n能证明某条候选被使用、被停用后不再注入。\n核心人格和关系状态没有因此被修改。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T08：表达习惯学习闭环，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t08-expression-learning",
  "title": "T08 表达习惯学习闭环",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 11. T08：实现轻量表达习惯学习闭环",
    "原始 T08 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "真实交流候选按来源、适用对象与情境持久化，经 Owner 审核、启用、停用或撤销",
    "学习与使用可独立开关，按情境选择性注入，并在使用前核验来源和状态",
    "不从日志、未发送草稿或自身输出自我强化，不覆盖人格、关系和生活事实",
    "验证候选审核到实际请求消费以及停用后不再使用的完整链路"
  ],
  "priority": 8,
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
      "id": "t08-review",
      "kind": "review",
      "command": null,
      "covers": [
        "真实交流候选按来源、适用对象与情境持久化，经 Owner 审核、启用、停用或撤销",
        "学习与使用可独立开关，按情境选择性注入，并在使用前核验来源和状态",
        "不从日志、未发送草稿或自身输出自我强化，不覆盖人格、关系和生活事实",
        "验证候选审核到实际请求消费以及停用后不再使用的完整链路"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t08-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "真实交流候选按来源、适用对象与情境持久化，经 Owner 审核、启用、停用或撤销",
        "学习与使用可独立开关，按情境选择性注入，并在使用前核验来源和状态",
        "不从日志、未发送草稿或自身输出自我强化，不覆盖人格、关系和生活事实",
        "验证候选审核到实际请求消费以及停用后不再使用的完整链路"
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
# T08 表达习惯学习闭环
