---
{
  "original_request": "先修复能独立修复的，需要Owner验证的先跳过。诊断详情旧恢复弹窗遗漏submission_reviewed且不显示submission_unknown入口。",
  "target": "诊断详情使用已实现的原媒体任务恢复流程，提交不确定记录可进入人工核对。",
  "id": "diagnostic-media-original-recovery-entry",
  "title": "诊断媒体详情接入原任务人工核对入口",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/v3-domain/diagnostics.js",
    "针对诊断详情入口的自动回归及证据"
  ],
  "non_goals": [
    "不改后端恢复政策、Studio或视觉身份恢复",
    "不新增外部结果导入或自动重新提交",
    "不触及四原稿或真实用户数据"
  ],
  "acceptance": [
    "failed和submission_unknown媒体诊断使用当前execution.intent_id指向正式原任务页",
    "无当前任务身份时只提示核对，不构造虚假任务或提交恢复",
    "目标原任务页既有人工核对、ready资产、版本和幂等回归保持"
  ],
  "priority": 1,
  "dependencies": null,
  "parent": "meromero-m2-05-image-submit-ambiguity",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "original-recovery-entry",
      "kind": "scenario",
      "command": null,
      "covers": [
        "failed和submission_unknown媒体诊断使用当前execution.intent_id指向正式原任务页",
        "无当前任务身份时只提示核对，不构造虚假任务或提交恢复",
        "目标原任务页既有人工核对、ready资产、版本和幂等回归保持"
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
# 诊断媒体详情接入原任务人工核对入口
