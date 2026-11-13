---
{
  "original_request": "用户要求非阻断且偏离当前修复的问题登记AEL，不顺手修改。恢复入口回归时在修改前测试及修改后测试同环境复现5项既有失败。",
  "target": "核对任务详情对话框、检查点与状态文案的现行合同，修复五项基线失败并保留原证据。",
  "id": "diagnostic-task-detail-baseline-regressions",
  "title": "任务详情既有五项UI合同回归待核对",
  "type": "bug",
  "source": "verification",
  "scope": [
    "apps/web/test/diagnostic-task-detail.test.mjs相关5项失败及对应正式UI入口"
  ],
  "non_goals": [
    "不阻挡本轮媒体恢复入口和候选预览修复",
    "不直接放宽断言或改业务政策以通过"
  ],
  "acceptance": [
    "区分实际UI缺陷与过期测试，并提供现行合同依据",
    "五项既有失败在原输入下有明确修复或批准的合同变更并复验"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "baseline-five",
      "kind": "scenario",
      "command": null,
      "covers": [
        "区分实际UI缺陷与过期测试，并提供现行合同依据",
        "五项既有失败在原输入下有明确修复或批准的合同变更并复验"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "media-task-failure-recovery",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 任务详情既有五项UI合同回归待核对
