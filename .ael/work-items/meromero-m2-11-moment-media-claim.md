---
{
  "original_request": "M2/T06 原始要求不允许模型把未完成动作说成已完成；实际附件由真实资源和统一交付产生，正文占位不能制造附件；Moment 允许可选媒体。",
  "target": "防止新发布的动态在没有对应真实附件或交付证据时，正文声称本次照片已经附上或发出。",
  "id": "meromero-m2-11-moment-media-claim",
  "title": "M2-11 动态正文中的无依据媒体完成声明",
  "type": "bug",
  "source": "user",
  "scope": [
    "正式 Moment Writer/独立工具的动态正文发布边界",
    "可核对的当前附件与历史媒体依据",
    "与现有发布核验机制一致的有界语义判断和真实事务校验"
  ],
  "non_goals": [
    "不改变角色是否选择发布的意愿，不禁止正常聊天、否认或未来意愿",
    "不固定引入 Qwen 审核，不调整模型选择、采样或 token 上限",
    "不把正则或模型自报一致当作所有自然语言正确的证明",
    "不修改四处用户原有草稿或连接生产业务库",
    "不假设真实模型、真实生图或整个 M2 已通过"
  ],
  "acceptance": [
    "无真实附件与交付依据时，声称本次照片已附在动态上的候选不能正式发布",
    "有可核对真实附件或匹配历史交付依据时的正确表达不被一律拦截",
    "普通文字、否认发送和未来意愿不因图片关键词被一律拦截；正文占位不能制造附件",
    "复用现有正式发布、权限和幂等边界，修复前红测、隔离库受控验证与相关回归均留证"
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
      "id": "m2-11-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "无真实附件与交付依据时，声称本次照片已附在动态上的候选不能正式发布",
        "有可核对真实附件或匹配历史交付依据时的正确表达不被一律拦截",
        "普通文字、否认发送和未来意愿不因图片关键词被一律拦截；正文占位不能制造附件",
        "复用现有正式发布、权限和幂等边界，修复前红测、隔离库受控验证与相关回归均留证"
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
# M2-11 动态正文中的无依据媒体完成声明
