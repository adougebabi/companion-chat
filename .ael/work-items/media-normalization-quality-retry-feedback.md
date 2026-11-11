---
{
  "original_request": "用户要求：不要偏移原来的任务设定；任务外、非阻断性的 bug 先创建 bug 任务，不在当前任务修改。T03 盘点模型输入时发现，media.go 在质量结果为 retry 时重新调用媒体提示词任务；model_tasks.go 对 branched media 进入 runBranchedMediaPrompt，但 mediaNormalizationInput 仅接收 concept，未投影上次 provider prompt、quality feedback 或 retry guidance。现有非分支媒体路径通过 mediaPromptInput 显式传入这些信息。当前结论是重试输入缺失反馈，尚未证明具体输出或交付失败。",
  "target": "在后续媒体工作中核实并修复分支媒体质量重试的输入，使对应任务能使用真实审核反馈，同时保留冻结意图和既有交付校验。",
  "id": "media-normalization-quality-retry-feedback",
  "title": "Bug：分支媒体质量重试未传入审核反馈",
  "type": "bug",
  "source": "user",
  "scope": [
    "branched media 的质量审核重试、规范化请求与必要的恢复回归"
  ],
  "non_goals": [
    "当前 T03 不修改媒体重试",
    "不伪造视觉审核通过或交付",
    "不更改原始意图或引入 token 限制",
    "不提交图片"
  ],
  "acceptance": [
    "受控物理请求证明分支媒体质量重试携带本次真实审核反馈和此前提示词，并保留冻结意图",
    "恢复和重试不重复提交已有副作用，失败仍按现有质量和交付语义处理",
    "非分支媒体及首次规范化行为不回退"
  ],
  "priority": 2,
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
      "id": "media-quality-retry-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "受控物理请求证明分支媒体质量重试携带本次真实审核反馈和此前提示词，并保留冻结意图",
        "恢复和重试不重复提交已有副作用，失败仍按现有质量和交付语义处理",
        "非分支媒体及首次规范化行为不回退"
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
# Bug：分支媒体质量重试未传入审核反馈
