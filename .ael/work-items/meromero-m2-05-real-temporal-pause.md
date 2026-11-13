---
{
  "original_request": "M2 要求真实工具、异步恢复和交付链路；用户明确图片服务可能已接单但响应丢失时直接暂停等待人工核对，不自动重新提交。M2-05 的跨进程 Core 和受控工作流证据已有，真实 Temporal 尚未验收。",
  "target": "",
  "id": "meromero-m2-05-real-temporal-pause",
  "title": "M2-05 真实 Temporal 图片不确定状态暂停",
  "type": "feature",
  "source": "user",
  "scope": [
    "一次性 Temporal 与 PostgreSQL 中正式 MediaWorkflow 和 ProcessMediaActivity 对已持久提交围栏的处理",
    "实际活动重试次数、持久不确定状态及人工核对前的拒绝"
  ],
  "non_goals": [
    "不调用真实渲染服务、真实模型或生产业务数据库",
    "不修改 Prompt、模型参数、图片恢复政策、Temporal 生产实现或四处原有草稿",
    "不把已持久围栏测试夸大为真实 ComfyUI 接单丢响应端到端"
  ],
  "acceptance": [
    "真实隔离 Temporal 执行正式媒体工作流与 Core 活动，已持久提交但无 Job ID 的任务进入 submission_unknown",
    "工作流活动按现有非重试策略停止，历史证明没有自动重复活动或生成请求",
    "未人工核对时不能自动恢复或重提；保留真实命令、版本、结果及测试是否执行",
    "若隔离服务不可用则记录实际错误和恢复条件，不用测试替身冒充真实 Temporal"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t06-delivery-consistency",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-05-real-temporal-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "真实隔离 Temporal 执行正式媒体工作流与 Core 活动，已持久提交但无 Job ID 的任务进入 submission_unknown",
        "工作流活动按现有非重试策略停止，历史证明没有自动重复活动或生成请求",
        "未人工核对时不能自动恢复或重提；保留真实命令、版本、结果及测试是否执行",
        "若隔离服务不可用则记录实际错误和恢复条件，不用测试替身冒充真实 Temporal"
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
# M2-05 真实 Temporal 图片不确定状态暂停
