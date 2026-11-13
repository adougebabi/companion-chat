---
{
  "original_request": "M2 期间仅执行无生图预检，用户提供的 Krea2 多输入 LoRA 工作流在隔离场景和假设性风格分类下均被现行分类策略拒绝。按用户要求作为非阻断 bug 登记，不在当前批次修复。证据：docs/verification/meromero-m2-evidence/m2-comfy-policy-preflight-20261113.md。",
  "target": "核实并处理正式媒体策略对多输入 LoRA 节点的分类兼容，保持原有身份隔离规则。",
  "id": "image-krea2-multi-lora-policy-compatibility",
  "title": "Bug：多输入 LoRA 工作流分类兼容",
  "type": "bug",
  "source": "verification",
  "scope": [
    "媒体工作流 LoRA 多输入分类及其正式 Image 调用验证"
  ],
  "non_goals": [
    "不猜测用户 LoRA 的真实分类，不通过空字符串或关闭规则绕过保护",
    "不修改提示词、模型选择、用户工作流或真实业务数据",
    "本次仅登记，不立即实施"
  ],
  "acceptance": [
    "隔离复现 Krea2 多输入 LoRA 工作流的分类结果，明确各实际输入的识别依据",
    "已明确分类的风格 LoRA 可通过；未分类或身份 LoRA 对无人物场景继续按现行规则拒绝，不能以空分类绕过",
    "通过正式媒体处理入口验证允许与拒绝对照，并回归现有普通 LoRA 工作流及无人物身份隔离"
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
      "id": "multi-lora-classification",
      "kind": "scenario",
      "command": null,
      "covers": [
        "隔离复现 Krea2 多输入 LoRA 工作流的分类结果，明确各实际输入的识别依据",
        "已明确分类的风格 LoRA 可通过；未分类或身份 LoRA 对无人物场景继续按现行规则拒绝，不能以空分类绕过",
        "通过正式媒体处理入口验证允许与拒绝对照，并回归现有普通 LoRA 工作流及无人物身份隔离"
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
# Bug：多输入 LoRA 工作流分类兼容
