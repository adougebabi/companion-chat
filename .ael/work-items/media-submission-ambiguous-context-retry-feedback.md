---
{
  "original_request": "用户要求图片服务可能已接单但响应丢失时直接暂停，等待人工核对，不自动重新提交。静态核对发现 mediaActivityFailure 先判断 context.Canceled/DeadlineExceeded，而 media_submission_ambiguous 的 CapabilityError 可解包这些原因，因此此类错误可能交由 Temporal 按活动策略重试。提交 fence 仍在下一次执行入口阻止再次 POST；尚未运行能够证明实际重试行为的 Temporal 场景。",
  "target": "核验提交歧义包装上下文取消或超时的实际活动恢复行为，按直接暂停策略处理已证实的偏差。",
  "id": "media-submission-ambiguous-context-retry-feedback",
  "title": "图片提交不确定时的上下文取消活动重试待核验",
  "type": "bug",
  "source": "verification",
  "scope": [
    "media_submission_ambiguous 的 Activity 错误转换及其 Temporal 受控回归"
  ],
  "non_goals": [
    "当前 M2-11 仅登记，不修改生产代码",
    "不重提未核对的图片任务，不访问生产数据库或真实渲染服务",
    "不改变普通取消和已知安全错误的既有处理语义"
  ],
  "acceptance": [
    "使用受控环境核实歧义错误包装取消或超时后 Temporal 是否重试活动，保留证据",
    "若存在偏差，确保直接进入待人工核对且不会自动重新提交；保留普通取消和安全恢复对照",
    "记录真实 Temporal 与受控替身的边界"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "meromero-m2-05-image-submit-ambiguity",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "media-ambiguous-context-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "使用受控环境核实歧义错误包装取消或超时后 Temporal 是否重试活动，保留证据",
        "若存在偏差，确保直接进入待人工核对且不会自动重新提交；保留普通取消和安全恢复对照",
        "记录真实 Temporal 与受控替身的边界"
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
# 图片提交不确定时的上下文取消活动重试待核验
