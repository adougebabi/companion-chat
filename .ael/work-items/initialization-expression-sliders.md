---
{
  "original_request": "目前初始化的时候，性格表达里面，有一些值，例如表情频率、调侃倾向，这些是允许小数吗？目前这样的输入框是可以输入小数，但左右的按钮却是整数来处理。假如支持小数，那就改为和上面 共同性格 那样的滑条。",
  "target": "确认表情频率等参数的小数语义，并将初始化页 0–1 表达倾向改为与共同性格一致的滑条，保持保存值和草稿语义。",
  "id": "initialization-expression-sliders",
  "title": "初始化性格表达的连续参数统一使用滑条",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/react-app/views/CharacterWizard.jsx",
    "apps/web/src/react-app/shared/ProfileField.tsx",
    "初始化表达倾向滑条的功能与页面验证"
  ],
  "non_goals": [
    "修改模型提示词、后端范围约定或数据库",
    "修改其他页面的资料控件行为",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "依据现有 schema 确认连续表达参数允许小数",
    "初始化的 0–1 表达倾向采用共同性格的滑条样式、0.01 步长和两位小数显示",
    "小数编辑、空值显式设定、草稿恢复和保存预览正确，其他字段维持原控件",
    "web 类型检查、构建和真实页面交互验证通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "id": "web-typecheck",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "typecheck"
      ],
      "covers": [
        "初始化的 0–1 表达倾向采用共同性格的滑条样式、0.01 步长和两位小数显示",
        "web 类型检查、构建和真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-build",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "build"
      ],
      "covers": [
        "web 类型检查、构建和真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "expression-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "依据现有 schema 确认连续表达参数允许小数",
        "初始化的 0–1 表达倾向采用共同性格的滑条样式、0.01 步长和两位小数显示",
        "小数编辑、空值显式设定、草稿恢复和保存预览正确，其他字段维持原控件",
        "web 类型检查、构建和真实页面交互验证通过"
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
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 初始化性格表达的连续参数统一使用滑条
