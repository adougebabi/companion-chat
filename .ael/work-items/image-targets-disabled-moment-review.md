---
{
  "original_request": "按用户要求将非阻断问题登记 AEL、暂不修复。隔离观察发现 Moment 停用且 Image 启用时，新图片任务仍可选择已有 Moment，已受理任务重放及普通会话图片可用。证据：docs/verification/meromero-m2-evidence/m2-moment-disabled-target-observation.log。是否违反原停用语义尚未确定。",
  "target": "核对 Moment 停用后的现有动态图片目标边界，依据既有业务合同处理。",
  "id": "image-targets-disabled-moment-review",
  "title": "Bug 待核：Moment 停用后的新图片目标",
  "type": "bug",
  "source": "verification",
  "scope": [
    "Moment 停用时新 Image 任务的目标准入与在途任务兼容"
  ],
  "non_goals": [
    "本次只登记，暂不修改业务行为",
    "不擅自决定历史动态可否修改的业务政策",
    "不影响 Image 普通会话目标或删除历史动态"
  ],
  "acceptance": [
    "核对 Moment 停用语义及已有动态的允许操作，明确新 Image 任务绑定已有动态是否符合原合同",
    "若确认不应受理，验证新请求拒绝且停用前已受理图片任务仍可恢复和重放；若确认允许，保留对应行为与证据",
    "Image 指向普通会话的调用和原有动态数据保持可用，不调整全局插件开关"
  ],
  "priority": 0,
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
      "id": "disabled-moment-target",
      "kind": "scenario",
      "command": null,
      "covers": [
        "核对 Moment 停用语义及已有动态的允许操作，明确新 Image 任务绑定已有动态是否符合原合同",
        "若确认不应受理，验证新请求拒绝且停用前已受理图片任务仍可恢复和重放；若确认允许，保留对应行为与证据",
        "Image 指向普通会话的调用和原有动态数据保持可用，不调整全局插件开关"
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
# Bug 待核：Moment 停用后的新图片目标
