---
{
  "original_request": "用户要求任务外非阻断性问题只登记 bug，不在当前任务调整。T08 盘点时发现 ExpressionLearningPanel 在候选读取失败后仍用 !loading \u0026\u0026 !items.length 显示“还没有表达候选”，来源读取失败后也同样显示“没有可用于学习的真实消息”。useDomainRead/useCursorPage 保留错误且初始列表为空，因而错误与空状态可能同时出现；目前依据源码确认，尚未做浏览器复现。",
  "target": "后续修正表达学习面板错误与空状态的区分，并验证读取失败和重试恢复。",
  "id": "expression-panel-error-shows-empty",
  "title": "Bug：表达学习读取失败时同时显示空列表",
  "type": "bug",
  "source": "user",
  "scope": [
    "表达学习候选与来源列表初始读取失败、真实空结果以及重试恢复的展示"
  ],
  "non_goals": [
    "当前 T08 不修改该显示问题",
    "不更改表达学习后端或数据",
    "不提交图片"
  ],
  "acceptance": [
    "候选和来源初次读取失败时显示错误，不同时声称列表为空",
    "读取成功且确为空时显示空状态；重试成功后正确恢复"
  ],
  "priority": 3,
  "dependencies": null,
  "parent": "meromero-t08-expression-learning",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": true,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "expression-error-empty-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "候选和来源初次读取失败时显示错误，不同时声称列表为空",
        "读取成功且确为空时显示空状态；重试成功后正确恢复"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "model",
      "id": "expression-error-empty-ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "候选和来源初次读取失败时显示错误，不同时声称列表为空",
        "读取成功且确为空时显示空状态；重试成功后正确恢复"
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
# Bug：表达学习读取失败时同时显示空列表
