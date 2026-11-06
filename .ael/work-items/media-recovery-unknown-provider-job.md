---
{
  "original_request": "现在媒体任务失败了，完全不能恢复（看图1）。应该要像图2、3、4那样。处理这个问题。",
  "target": "从失败处恢复时，如生成服务任务编号未保存，清楚提示可能重新提交及重复生成，仍保留原恢复协议。",
  "id": "media-recovery-unknown-provider-job",
  "title": "提示媒体恢复时未知的生成服务任务",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/react-app/shared/MediaTaskRecovery.tsx",
    "apps/web/test/diagnostic-task-detail.test.mjs",
    "媒体恢复确认页面"
  ],
  "non_goals": [
    "改动媒体服务执行或重写已有任务",
    "提交图片或发布"
  ],
  "acceptance": [
    "没有已保存的生成服务任务编号时，恢复确认说明可能再次提交以及上次已受理时的重复风险；已有编号时不显示此提醒",
    "确认、取消、版本与幂等恢复行为保持正确，自动回归和真实页面确认通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": true,
  "ux": false,
  "checks": [
    {
      "id": "ambiguity-regression",
      "kind": "command",
      "command": [
        "node",
        "--test",
        "--test-name-pattern=media recovery warns",
        "apps/web/test/diagnostic-task-detail.test.mjs"
      ],
      "covers": [
        "没有已保存的生成服务任务编号时，恢复确认说明可能再次提交以及上次已受理时的重复风险；已有编号时不显示此提醒",
        "确认、取消、版本与幂等恢复行为保持正确，自动回归和真实页面确认通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "ambiguity-ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "确认、取消、版本与幂等恢复行为保持正确，自动回归和真实页面确认通过"
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
# 提示媒体恢复时未知的生成服务任务
