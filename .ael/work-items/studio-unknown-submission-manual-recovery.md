---
{
  "original_request": "Studio image_preview 在可能已接单但响应丢失时会暂停，当前可核对后丢弃，但没有将可核对结果接回原任务的完整领域入口；没有本地同源 ready 资产时，外部结果归属和接管规则尚未确定。",
  "target": "明确并实现 Studio 原任务在人工核对后的安全处理，保留已记录的暂停与去重边界。",
  "id": "studio-unknown-submission-manual-recovery",
  "title": "Studio 图片提交不明后的人工处理缺口",
  "type": "bug",
  "source": "verification",
  "scope": [
    "Studio 任务的人工核对、已验证结果恢复或明确丢弃及相应状态"
  ],
  "non_goals": [
    "不自动查询图片服务或自动重新提交",
    "不凭人工输入的外部 ID 猜测结果归属",
    "不修改视觉身份、生产数据或原有质量验收"
  ],
  "acceptance": [
    "人工未确认时保持暂停且不重复提交",
    "对可证明同源的结果进行版本、权限、质量和幂等校验后安全处理；无法证明归属时继续暂停",
    "覆盖旧任务、迟到回调、并发和实际 UI/后端状态，并记录外部结果接管尚需的规则"
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
        "对可证明同源的结果进行版本、权限、质量和幂等校验后安全处理；无法证明归属时继续暂停",
        "覆盖旧任务、迟到回调、并发和实际 UI/后端状态，并记录外部结果接管尚需的规则"
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
# Studio 图片提交不明后的人工处理缺口
