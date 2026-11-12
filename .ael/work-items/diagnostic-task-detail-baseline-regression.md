---
{
  "original_request": "用户要求本轮非阻断且不属于原 MeroMero 任务的问题只登记，不顺手修复。2026-11-12 在当前改动和干净基线 b3f5cd40 使用相同 Node 测试环境运行 diagnostic-task-detail.test.mjs，均有相同 5 项失败。",
  "target": "单独核对并处理诊断任务详情页面已有的 5 项测试失败。",
  "id": "diagnostic-task-detail-baseline-regression",
  "title": "诊断任务详情的既有页面回归失败",
  "type": "bug",
  "source": "verification",
  "scope": [
    "诊断任务详情的通用生命周期、重置和标题/原因展示及对应测试"
  ],
  "non_goals": [
    "不属于当前 M2 图片提交保护实施",
    "不伪造通过或删除现有失败证据"
  ],
  "acceptance": [
    "在相同环境重现并分类这 5 项失败",
    "按照用户确认的诊断页面行为修复并执行相应回归"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "model",
      "id": "review",
      "kind": "review",
      "command": null,
      "covers": [
        "在相同环境重现并分类这 5 项失败",
        "按照用户确认的诊断页面行为修复并执行相应回归"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "diagnostics-task-order-failure-detail",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 诊断任务详情的既有页面回归失败
