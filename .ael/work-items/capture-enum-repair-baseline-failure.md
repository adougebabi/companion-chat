---
{
  "original_request": "2026-11-12 在 M2-06 改动和干净基线 b8204c55、相同一次性 PostgreSQL 与受控 Provider 环境下，TestCurrentCaptureFinalMediaWorkerSubmitsOnlyFrozenFactsAndRejectsOverrides/enum-rejected 均失败：模型结构化输出两次 framing 枚举不合法，现有测试断言与实际修复调用次数不一致。按用户要求先登记，不在 SDXL 正向校验子项内顺手修改。",
  "target": "核对拍摄枚举错误的既有业务约定与测试，确定并修复该基线失败。",
  "id": "capture-enum-repair-baseline-failure",
  "title": "拍摄枚举无效时已有回归断言失败",
  "type": "bug",
  "source": "verification",
  "scope": [
    "拍摄计划 framing 枚举校验、结构化纠错次数及对应测试"
  ],
  "non_goals": [
    "不在 M2-06 中修改受保护的媒体草稿",
    "不将受控 Provider 结果称为真实模型验收"
  ],
  "acceptance": [
    "用相同输入和隔离环境确认当前行为与原约定",
    "在保留现有授权边界和用户草稿的前提下修复并验证"
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
      "method": "model",
      "id": "review",
      "kind": "review",
      "command": null,
      "covers": [
        "用相同输入和隔离环境确认当前行为与原约定",
        "在保留现有授权边界和用户草稿的前提下修复并验证"
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
# 拍摄枚举无效时已有回归断言失败
