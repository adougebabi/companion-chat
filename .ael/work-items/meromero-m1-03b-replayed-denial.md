---
{
  "original_request": "M1 要求权限拒绝不导致无意义重复工具循环，并保留真实 ToolCall/ToolResult 和幂等；本工作项只覆盖重复原生调用 ID。",
  "target": "让正式私聊中同一无权限工具重复使用相同原生调用 ID 时有界进入正常回复，保留真实回执的幂等性。",
  "id": "meromero-m1-03b-replayed-denial",
  "title": "M1-03b 识别同一调用 ID 的跨轮拒绝重放",
  "type": "bug",
  "source": "user",
  "scope": [
    "会话权限拒绝和已有模型响应批次记录",
    "隔离数据库及受控 Provider 正式入口和相关回归"
  ],
  "non_goals": [
    "不改变授权策略、模型 prompt 或默认聊天模式",
    "不重复执行业务效果、不伪造工具回执",
    "不修改四处原有草稿或其他里程碑"
  ],
  "acceptance": [
    "同一无权限工具跨不同模型响应重复使用同一调用 ID 时有界结束并正常回复",
    "重复 ID 不重复执行业务效果或追加伪造回执，同一响应内多个调用不被当成多轮",
    "正式 HandleTurn、已授权工具和现有发布回归在隔离 PostgreSQL 中通过"
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
      "id": "m1-03b-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同一无权限工具跨不同模型响应重复使用同一调用 ID 时有界结束并正常回复",
        "重复 ID 不重复执行业务效果或追加伪造回执，同一响应内多个调用不被当成多轮",
        "正式 HandleTurn、已授权工具和现有发布回归在隔离 PostgreSQL 中通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "meromero-t06-delivery-consistency",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M1-03b 识别同一调用 ID 的跨轮拒绝重放
