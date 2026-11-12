---
{
  "original_request": "M1 要求权限拒绝不能导致无意义的重复工具循环，且不破坏正常聊天和相关发布；此工作项只覆盖真实私聊的能力范围拒绝。",
  "target": "正式私聊中同一无权限工具改变参数和调用 ID 的重复尝试必须有界，随后允许正常回复。",
  "id": "meromero-m1-03a-rejected-reply-loop",
  "title": "M1-03a 限制重复无权限工具调用",
  "type": "bug",
  "source": "user",
  "scope": [
    "私聊原生工具循环对 capability_not_authorized 无效果回执的重复处理",
    "隔离 PostgreSQL 和受控 Provider 的 HandleTurn 正式入口测试"
  ],
  "non_goals": [
    "不修改授权策略、不扩大工具权限",
    "不处理 WakeUp 独立循环或其他模型任务",
    "不修改 M1-01 未提交文件、不切换默认聊天模式"
  ],
  "acceptance": [
    "同一工具遭能力范围拒绝后，即使参数和调用 ID 改变也能识别重复且有界进入正常回复",
    "不同授权工具和正常私聊、已有防重复行为不回退",
    "正式 HandleTurn 与隔离 PostgreSQL 测试保留修复前失败和修复后证据"
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
      "id": "m1-03a-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同一工具遭能力范围拒绝后，即使参数和调用 ID 改变也能识别重复且有界进入正常回复",
        "不同授权工具和正常私聊、已有防重复行为不回退",
        "正式 HandleTurn 与隔离 PostgreSQL 测试保留修复前失败和修复后证据"
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
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M1-03a 限制重复无权限工具调用
