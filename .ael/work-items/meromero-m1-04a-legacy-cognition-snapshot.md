---
{
  "original_request": "M1 要求确认必要的任务配置快照能够支持后续主链迁移，并明确没有可靠模型快照的旧任务兼容策略，不能虚构原模型。",
  "target": "用正式私聊续接入口验证有快照时恢复原绑定、旧记录无快照时使用当前绑定并明确记录未知原模型。",
  "id": "meromero-m1-04a-legacy-cognition-snapshot",
  "title": "M1-04a 私聊续接模型快照及旧记录兼容",
  "type": "test",
  "source": "user",
  "scope": [
    "私聊回复续接的认知模型绑定快照和旧记录行为",
    "隔离 PostgreSQL、受控 Provider 物理请求和现有单元测试"
  ],
  "non_goals": [
    "不修改模型路由或旧任务业务策略",
    "不切换默认聊天模式、不声称已迁移主链",
    "不连接生产数据库、不覆盖原有草稿"
  ],
  "acceptance": [
    "已有认知模型快照的私聊续接在配置改变后继续使用原快照，不泄露凭据",
    "缺少认知模型快照的旧续接实际使用当前有效绑定并记录 legacy_unfrozen，不虚构原模型",
    "损坏快照明确失败而不是被当成旧任务重新选模"
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
      "id": "m1-04a-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "已有认知模型快照的私聊续接在配置改变后继续使用原快照，不泄露凭据",
        "缺少认知模型快照的旧续接实际使用当前有效绑定并记录 legacy_unfrozen，不虚构原模型",
        "损坏快照明确失败而不是被当成旧任务重新选模"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "meromero-t02-task-model-routing",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M1-04a 私聊续接模型快照及旧记录兼容
