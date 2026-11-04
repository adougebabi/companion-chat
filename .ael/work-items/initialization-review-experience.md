---
{
  "original_request": "通过描述让 LLM 分析的初始化流程中：female 被识别为自定义性别而非女；澄清标点习惯、幽默方式、回应节奏等数值的语义并统一合适的滑块，给抽象数值增加可理解的说明；回复风格、消息长短及其他类似下拉框检查并使用 shadcn Select；生活设定、人格与关系的常规编辑不应要求用户编辑 JSON，完整设定可以保留 JSON 编辑。",
  "target": "修复初始化分析预览中的识别映射，并让数值、选择项和结构化设定可理解、可直接编辑且保留原有数据。",
  "id": "initialization-review-experience",
  "title": "完善初始化分析预览和结构化编辑体验",
  "type": "bug",
  "source": "user",
  "scope": [
    "初始化分析后的第三步预览与编辑",
    "性别映射、数值语义与表单控件",
    "生活设定、人格与关系的结构化字段编辑",
    "相关初始化页面测试与交互验证"
  ],
  "non_goals": [
    "修改用户未提出的其他页面",
    "编造未知人物事实或破坏已有扩展字段",
    "提交图片、Git 提交或发布"
  ],
  "acceptance": [
    "常见模型性别值 female、male 在初始化预览中正确对应女、男，保留未知与自定义值",
    "按真实字段类型和范围为适合的数值提供一致的滑块与易理解的端点说明，保留未设置及保存语义",
    "检查初始化流程的选择控件并让适用的下拉选择使用项目 shadcn Select，保留值和自定义选项",
    "生活设定、人格与关系支持直观的结构化编辑且不丢失未知数据；仅完整设定保留 JSON 编辑",
    "相关回归、类型检查、构建及实际页面交互验证通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": true,
  "ux": true,
  "checks": [
    {
      "id": "web-initialization",
      "kind": "command",
      "command": [
        "node",
        "--test",
        "apps/web/test/demo-replica-creation-ui.test.mjs"
      ],
      "covers": [
        "常见模型性别值 female、male 在初始化预览中正确对应女、男，保留未知与自定义值",
        "生活设定、人格与关系支持直观的结构化编辑且不丢失未知数据；仅完整设定保留 JSON 编辑"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
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
        "相关回归、类型检查、构建及实际页面交互验证通过"
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
        "相关回归、类型检查、构建及实际页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "initialization-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "按真实字段类型和范围为适合的数值提供一致的滑块与易理解的端点说明，保留未设置及保存语义",
        "检查初始化流程的选择控件并让适用的下拉选择使用项目 shadcn Select，保留值和自定义选项",
        "生活设定、人格与关系支持直观的结构化编辑且不丢失未知数据；仅完整设定保留 JSON 编辑",
        "相关回归、类型检查、构建及实际页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "相关回归、类型检查、构建及实际页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "ux",
      "kind": "ux",
      "command": null,
      "covers": [
        "相关回归、类型检查、构建及实际页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "initialization-preview-field-contract",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 完善初始化分析预览和结构化编辑体验
