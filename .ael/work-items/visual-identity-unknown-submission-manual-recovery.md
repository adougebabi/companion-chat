---
{
  "original_request": "视觉身份候选和设定图在可能已接单但响应丢失时会暂停并提示人工核对，但尚无完整领域入口将核对后的处理接回原任务。",
  "target": "明确并实现视觉身份任务人工核对后的安全处理与原工作流衔接。",
  "id": "visual-identity-unknown-submission-manual-recovery",
  "title": "视觉身份提交不明后的人工处理缺口",
  "type": "bug",
  "source": "verification",
  "scope": [
    "视觉身份候选及设定图的人工核对状态、版本围栏和原工作流恢复"
  ],
  "non_goals": [
    "不自动查询图片服务或自动重新提交",
    "不凭外部 ID 或未经验证的图片推断归属",
    "不修改 Studio、生产数据或用户现有视觉审核结论"
  ],
  "acceptance": [
    "人工未确认时保持暂停且不重复提交",
    "明确用户选择并校验版本、执行状态及原任务身份后安全处理，不重复副作用",
    "覆盖原视觉工作流、旧尝试、失败重放及相关 UI 状态，并记录无法验证的外部结果"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "meromero-t05-image-moment-plugins",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "manual-recovery-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "人工未确认时保持暂停且不重复提交",
        "明确用户选择并校验版本、执行状态及原任务身份后安全处理，不重复副作用",
        "覆盖原视觉工作流、旧尝试、失败重放及相关 UI 状态，并记录无法验证的外部结果"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "meromero-m2-05-image-submit-ambiguity",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 视觉身份提交不明后的人工处理缺口
