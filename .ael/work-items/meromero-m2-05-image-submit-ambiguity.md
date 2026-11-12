---
{
  "original_request": "T05 要求在途任务不重复提交或交付；T06 要求不得为恢复上下文重新执行已提交副作用，明确未知与重试状态。",
  "target": "复现并处理渲染服务已接单、本地尚未保存 Job ID 时自动恢复可能重复提交的窗口。",
  "id": "meromero-m2-05-image-submit-ambiguity",
  "title": "M2-05 图片已接单但 Job ID 未保存时避免重复提交",
  "type": "bug",
  "source": "user",
  "scope": [
    "正式图片 intent 的 renderer 提交、持久状态、失败恢复与去重",
    "一次性 PostgreSQL 和受控本机 HTTP renderer 的失败与恢复测试"
  ],
  "non_goals": [
    "不修改 Prompt、采样、token 限额、默认模型或 fact_only 开关",
    "不改变真实服务或访问生产数据库，不伪称受控服务是真实 ComfyUI",
    "不改写既有已完成任务或其他领域，不覆盖四处原有草稿",
    "不擅自改变未明确的业务恢复策略"
  ],
  "acceptance": [
    "隔离场景复现 renderer 接单但回复丢失后，原任务自动恢复可能重复提交的事实",
    "安全处理该窗口而不伪称未发生或已交付，既有 job ID 的恢复和明确安全的首次请求继续有效",
    "持久状态、诊断与重放可复核，不修改用户已有媒体数据",
    "覆盖故障和正常对照，未验证的真实 renderer 能力及策略限制明确记录"
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
      "id": "m2-05-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "隔离场景复现 renderer 接单但回复丢失后，原任务自动恢复可能重复提交的事实",
        "安全处理该窗口而不伪称未发生或已交付，既有 job ID 的恢复和明确安全的首次请求继续有效",
        "持久状态、诊断与重放可复核，不修改用户已有媒体数据",
        "覆盖故障和正常对照，未验证的真实 renderer 能力及策略限制明确记录"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry",
    "legacy-9b89c41b4be449b963dcba95"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M2-05 图片已接单但 Job ID 未保存时避免重复提交
