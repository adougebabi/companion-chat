---
{
  "original_request": "原 T02 要求进行中 run 和恢复任务默认不因配置变化静默换模型；M1 要求确认必要快照支持主链迁移并明确旧任务无快照策略。",
  "target": "为新的正式 direct 私聊逻辑 turn 保存并在恢复重试时恢复模型绑定，旧 turn 无快照时明确按兼容行为处理。",
  "id": "meromero-m1-04b-direct-turn-model-snapshot",
  "title": "M1-04b direct 私聊恢复保持模型绑定",
  "type": "feature",
  "source": "user",
  "scope": [
    "direct 私聊 cognition turn 的模型配置快照和正式恢复入口",
    "隔离 PostgreSQL、受控 Provider 和必要的直接回归"
  ],
  "non_goals": [
    "不切换默认聊天模式、不修改 Prompt 或 token 限额",
    "不改造其他后台任务或 UI、不引入并行 Runtime",
    "不把密钥写入快照、不连接生产库、不覆盖原有草稿"
  ],
  "acceptance": [
    "新的 direct 私聊在首次 Provider 请求前持久化实际模型绑定，同一逻辑 turn 恢复或重试时不因配置变化静默换模型",
    "同一配置变化后新 turn 使用新绑定，已完成回复重放不重复执行",
    "无可靠快照的旧 turn 明确标为 legacy_unfrozen，不虚构原模型；损坏或越权快照明确失败",
    "隔离 PostgreSQL 和受控 Provider 正式入口测试覆盖重试、凭据保护及直接发布回归"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-m1-reliability",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m1-04b-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "新的 direct 私聊在首次 Provider 请求前持久化实际模型绑定，同一逻辑 turn 恢复或重试时不因配置变化静默换模型",
        "同一配置变化后新 turn 使用新绑定，已完成回复重放不重复执行",
        "无可靠快照的旧 turn 明确标为 legacy_unfrozen，不虚构原模型；损坏或越权快照明确失败",
        "隔离 PostgreSQL 和受控 Provider 正式入口测试覆盖重试、凭据保护及直接发布回归"
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
  "feedback_of": "meromero-t02-task-model-routing",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M1-04b direct 私聊恢复保持模型绑定
