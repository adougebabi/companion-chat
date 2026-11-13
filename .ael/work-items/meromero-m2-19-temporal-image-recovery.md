---
{
  "original_request": "原 M2/T05/T06 要求真实工具调用、异步状态、恢复、发布及附件具有对应集成证据。现有真实 Temporal 验证仅覆盖未知提交暂停，已保存 Job 的成功恢复及交付尚缺真实 Temporal Worker 证据。",
  "target": "验证真实 Temporal 对已保存渲染 Job 的同任务重试、资产落库和唯一附件交付。",
  "id": "meromero-m2-19-temporal-image-recovery",
  "title": "M2-19 真实 Temporal 的图片任务恢复交付",
  "type": "feature",
  "source": "user",
  "scope": [
    "任务专用 Temporal 与隔离 PostgreSQL，正式 Image 入口、媒体工作流及交付回归"
  ],
  "non_goals": [
    "不修复非阻断的 LoRA 或 Moment 目标待核问题",
    "不修改模型提示词、采样、默认模式或用户四处草稿",
    "不把受控 Provider、渲染器或对象存储说成真实模型、真实 ComfyUI 或真实 MinIO"
  ],
  "acceptance": [
    "隔离 PostgreSQL 通过正式 Image 入口受理任务，真实 Temporal 执行正式媒体活动；受控渲染器接单后的首次轮询失败并保留原 Job ID",
    "Temporal 重试同一任务时只轮询原 Job、没有第二次提交，完成后保存可核对的图片字节并向原会话唯一交付",
    "同一任务重放不重复上传或附件，媒体状态、工作流活动次数及任务身份均可核对",
    "对直接相关工作流回归实际执行并保存无 SKIP 的记录；真实模型、ComfyUI 和对象存储结果不得由替身冒充"
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
      "id": "temporal-image-recovery",
      "kind": "scenario",
      "command": null,
      "covers": [
        "隔离 PostgreSQL 通过正式 Image 入口受理任务，真实 Temporal 执行正式媒体活动；受控渲染器接单后的首次轮询失败并保留原 Job ID",
        "Temporal 重试同一任务时只轮询原 Job、没有第二次提交，完成后保存可核对的图片字节并向原会话唯一交付",
        "同一任务重放不重复上传或附件，媒体状态、工作流活动次数及任务身份均可核对",
        "对直接相关工作流回归实际执行并保存无 SKIP 的记录；真实模型、ComfyUI 和对象存储结果不得由替身冒充"
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
# M2-19 真实 Temporal 的图片任务恢复交付
