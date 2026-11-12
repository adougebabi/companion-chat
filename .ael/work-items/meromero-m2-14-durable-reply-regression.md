---
{
  "original_request": "M2/T04 要求主链迁移后现有消息流、状态及 UI 交互不回退；既有任务 durable-chat-missing-live-token-stream 已登记浏览器在生成期间缺少真实增量，用户要求非任务外阻断问题先登记、不顺手修改。",
  "target": "",
  "id": "meromero-m2-14-durable-reply-regression",
  "title": "M2-14 已提交回复的持久流回归",
  "type": "feature",
  "source": "user",
  "scope": [
    "M2 起始版本与当前版本的同一 direct durable 消息观察场景",
    "已有浏览器 NDJSON 和 Web 控制器测试证据的版本核对"
  ],
  "non_goals": [
    "不实现新实时 token 通道或修改既有流式业务策略",
    "不把提交后的完整消息称为生成中的实时增量",
    "不修改模型选择、fact_only 开关、四处原有草稿或生产业务数据"
  ],
  "acceptance": [
    "M2 起始和当前代码在同一隔离 PostgreSQL 与受控 Provider 输入下执行 direct durable 场景，核对已提交用户消息、助手消息与终态顺序",
    "浏览器 NDJSON 转发及 Web 流控制已有测试的受检版本与当前相关代码一致，或按需复验",
    "如实记录真实生成期 token 缺失及已有 AEL 工单，不据终态测试声称实时增量已验收"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t04-single-main-chat",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-14-durable-regression",
      "kind": "scenario",
      "command": null,
      "covers": [
        "M2 起始和当前代码在同一隔离 PostgreSQL 与受控 Provider 输入下执行 direct durable 场景，核对已提交用户消息、助手消息与终态顺序",
        "浏览器 NDJSON 转发及 Web 流控制已有测试的受检版本与当前相关代码一致，或按需复验",
        "如实记录真实生成期 token 缺失及已有 AEL 工单，不据终态测试声称实时增量已验收"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M2-14 已提交回复的持久流回归
