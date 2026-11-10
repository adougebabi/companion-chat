---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T09，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 12. T09：记忆来源、纠错与派生失效\n\n复用现有记忆存储和检索，不整体替换。\n\n## 12.1 分类\n\n明确区分：\n- 已发生的交流事件；\n- 用户/角色的自述；\n- 已确认的世界事实；\n- 计划和承诺；\n- 推断；\n- 摘要；\n- 人物画像候选；\n- 表达习惯。\n\n“用户说买票，随后承认开玩笑”：\n- 两次发言是已发生的交流；\n- 不能记录为已购票；\n- 也不能把两次发言本身标成没有发生。\n\n当前状态不应因为被摘要就变成永久人格设定。\n\n## 12.2 来源和时间\n\n沿用现有引用体系，补足：\n- 来源消息/事件；\n- actor；\n- 发生时间与适用时间；\n- 来源类别；\n- 确认/推断状态；\n- 纠正或替代关系。\n\n不要新增一套与现有 evidence_refs 无关的平行事实系统。\n\n“正在穿”不等于“一直穿”。\n“过去长发”不覆盖“现在短发”。\n“没有检索到”不等于“从未发生”。\n\n## 12.3 纠错传播\n\n事实被纠正时，相关：\n- 摘要；\n- 人物画像；\n- 检索缓存；\n- 派生记忆；\n- 后续任务输入\n\n必须能够失效、更新或标记过时。\n\n不删除所有历史，也不篡改历史上实际说过的话。\n保留纠错链，避免旧摘要再次恢复错误事实。\n\n权限隔离：\n不能将一个角色、Actor、世界或会话的记忆错误注入另一个作用域。\n\n## 12.4 查询能力\n\n检查主 Agent 是否能通过现有能力按需查询真实记忆。\n不能只能被动接收预选摘要，也不能每轮强制查询。\n\n验收：\n覆盖纠正、撤回、未知、相互矛盾、主体不同、过期状态，以及纠错后的检索结果。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T09：记忆来源、纠错及派生失效，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t09-memory-correction",
  "title": "T09 记忆来源、纠错及派生失效",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 12. T09：记忆来源、纠错与派生失效",
    "原始 T09 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "沿用既有记忆来源与作用域，区分发言、外部事实、计划、推断和摘要",
    "纠正或撤回后摘要、画像、缓存、派生记忆和后续输入能够安全失效或正确重建",
    "处理未知旧来源、在途请求、并发锁序和不同 Actor 会话隔离，保留历史和纠错链",
    "以隔离数据库检验纠错、撤回、冲突、时效和纠错后检索"
  ],
  "priority": 9,
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
      "id": "t09-review",
      "kind": "review",
      "command": null,
      "covers": [
        "沿用既有记忆来源与作用域，区分发言、外部事实、计划、推断和摘要",
        "纠正或撤回后摘要、画像、缓存、派生记忆和后续输入能够安全失效或正确重建",
        "处理未知旧来源、在途请求、并发锁序和不同 Actor 会话隔离，保留历史和纠错链",
        "以隔离数据库检验纠错、撤回、冲突、时效和纠错后检索"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t09-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "沿用既有记忆来源与作用域，区分发言、外部事实、计划、推断和摘要",
        "纠正或撤回后摘要、画像、缓存、派生记忆和后续输入能够安全失效或正确重建",
        "处理未知旧来源、在途请求、并发锁序和不同 Actor 会话隔离，保留历史和纠错链",
        "以隔离数据库检验纠错、撤回、冲突、时效和纠错后检索"
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
# T09 记忆来源、纠错及派生失效
