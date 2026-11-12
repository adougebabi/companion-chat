---
{
  "original_request": "M2-06 受控图片请求边界探针中，PrimitiveString 的正向文本未包含既有无字规则；保留探针日志，按照用户要求登记范围外非阻断问题，不在 M2-06 修改。",
  "target": "核对 PrimitiveString 连接的图片正向文本是否遵循现行无字规则，并在独立任务中验证与修复。",
  "id": "image-no-text-primitive-string-boundary",
  "title": "图片无字规则未覆盖 PrimitiveString 正向文本",
  "type": "bug",
  "source": "verification",
  "scope": [
    "图片提交时的无字规则和 PrimitiveString 接线及相关回归"
  ],
  "non_goals": [
    "不在 M2-06 顺手修改图片文字策略",
    "不把受控请求当成真实渲染"
  ],
  "acceptance": [
    "复现并确认 PrimitiveString 与正负向接线的无字规则行为",
    "修复后验证正向规则、负向质量、原始文本与其他现有工作流不回退"
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
      "id": "image-no-text-test",
      "kind": "scenario",
      "command": null,
      "covers": [
        "复现并确认 PrimitiveString 与正负向接线的无字规则行为",
        "修复后验证正向规则、负向质量、原始文本与其他现有工作流不回退"
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
# 图片无字规则未覆盖 PrimitiveString 正向文本
