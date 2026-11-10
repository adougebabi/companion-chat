---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T10，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 13. T10：现有领域能力保护与回归\n\n本项是保护性审计和必要适配，不是重新设计全部业务。\n\n逐项证明没有被模型迁移、上下文精简或插件拆分破坏。\n\n## 13.1 Goal\n\n保留：\n- 与固定人格解耦的动态 Goal；\n- 原成功条件；\n- 已完成记录；\n- 补全触发；\n- 默认约五个活动目标的现有上限，不强制填满；\n- 排序和依赖；\n- Owner 暂停/控制；\n- 原目标优先和去重规则。\n\n禁止：\n- 人格里旧愿望复活已完成目标；\n- 每轮聊天强制推进；\n- 送达当同意；\n- 模型自述当完成；\n- 为方便测试提高或降低成功标准；\n- 把目标规划拆成两套并行生成链。\n\n## 13.2 日程与虚拟活动\n\n保留：\n- 计划与实际到场分离；\n- 开始/进行中/完成分离；\n- 版本与时间约束；\n- 历史不可随意重写；\n- 购物、理发、染发等现有活动结算。\n\n不把普通愿望直接变成已安排。\n不把已安排直接变成已完成。\n\n## 13.3 物品与衣柜\n\n持有、搭配、穿着、借用、归还分别处理。\n缺少物品时可以由角色选择形成购买意愿或安排，不要只实现永久拒绝分支。\n入库必须来自已完成的获取过程。\n\n## 13.4 人格与视觉身份\n\n保留 Working Persona、profile、持久切换与本轮接管的区别。\n不因为取消旧主认知路径导致唯一有效的人格切换入口消失。\n\n当前有效外貌优先于初始化事实，但不改写未发生变化的属性。\n未知身份资料不得凭名字或常识补齐。\n\n## 13.5 其他\n\n保持并验证：\n- Actor 背景与关系配置的编辑和运行消费；\n- diary_material 与 diary_expression 的职责区分；\n- native_cognition / daily_review；\n- NPC 自身人格与知识范围；\n- 世界事件提议、准入和正式生效；\n- Studio 草稿与真实世界隔离；\n- 初始化、衣柜准备与视觉身份生命周期。\n\n创作任务允许生成候选设定。\n不得把“运行事实不能编造”扩展成“初始化或世界创作什么都不能生成”。\n候选经授权提交之前不能冒充已生效状态。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T10：现有领域能力保护性回归，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t10-domain-regression",
  "title": "T10 现有领域能力保护性回归",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 13. T10：现有领域能力保护与回归",
    "原始 T10 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "保护 Goal 的原成功条件、去重、活动上限与完成记录",
    "保护日程与虚拟活动的计划、到场、完成和历史版本语义，以及物品、衣柜的权威状态",
    "核对人格切换、视觉身份、Actor/NPC 知识隔离、世界事件、Studio 与日记关键路径",
    "发现范围内真实回退时修复并验证，不把未执行的回归算作通过"
  ],
  "priority": 10,
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
      "id": "t10-review",
      "kind": "review",
      "command": null,
      "covers": [
        "保护 Goal 的原成功条件、去重、活动上限与完成记录",
        "保护日程与虚拟活动的计划、到场、完成和历史版本语义，以及物品、衣柜的权威状态",
        "核对人格切换、视觉身份、Actor/NPC 知识隔离、世界事件、Studio 与日记关键路径",
        "发现范围内真实回退时修复并验证，不把未执行的回归算作通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t10-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "保护 Goal 的原成功条件、去重、活动上限与完成记录",
        "保护日程与虚拟活动的计划、到场、完成和历史版本语义，以及物品、衣柜的权威状态",
        "核对人格切换、视觉身份、Actor/NPC 知识隔离、世界事件、Studio 与日记关键路径",
        "发现范围内真实回退时修复并验证，不把未执行的回归算作通过"
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
# T10 现有领域能力保护性回归
