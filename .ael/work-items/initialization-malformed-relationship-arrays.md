---
{
  "original_request": "core_persona.life_profile.relationship_seeds[0] type_invalid 经常出现，是否因为数组字段没有告诉 LLM 如何填充；如果字段说明无法避免类型错误，可以对类型错误的数组兜底为空数组。",
  "target": "补齐初始化物理模型请求的数组元素合同，并在模型仍返回类型错误时安全恢复，保留可用事实和原始错误数据。",
  "id": "initialization-malformed-relationship-arrays",
  "title": "修复初始化模型关系数组类型错误",
  "type": "bug",
  "source": "user",
  "scope": [
    "初始化描述分析的模型请求与实际响应处理",
    "relationship_seeds 和同类初始化数组字段的类型及恢复",
    "物理 HTTP、归一化、直接 JSON 边界和回归验证",
    "模型提示词组成清单",
    "初始化预览中的数组恢复提示与原值审阅"
  ],
  "non_goals": [
    "修改用户提交的原始描述、数据库或正式激活权限",
    "把直接导入 JSON 的严格校验整体改为静默容错",
    "新增或调整任何模型 token 上限",
    "提交图片、Git 提交或发布"
  ],
  "acceptance": [
    "真实模型 HTTP 请求明确说明 relationship_seeds 和相关数组的元素类型及必要字段，不编造关系事实",
    "模型生成的关系数组类型错误时可继续初始化；可保留的合法项目和其他事实不被覆盖，无法恢复的原值与警告可供审阅",
    "空值、混合类型和外层非数组等边界有确定性回归，直接 JSON 与正式激活的严格校验仍保留",
    "相关 Go 回归与提示词物理请求检查通过，必要的提示词组成清单同步",
    "初始化预览清楚提示哪些数组内容需要核对，并能查看保留的原值"
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
      "id": "core-initialization",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-run",
        "TestInitialization",
        "-count=1"
      ],
      "covers": [
        "真实模型 HTTP 请求明确说明 relationship_seeds 和相关数组的元素类型及必要字段，不编造关系事实",
        "模型生成的关系数组类型错误时可继续初始化；可保留的合法项目和其他事实不被覆盖，无法恢复的原值与警告可供审阅",
        "空值、混合类型和外层非数组等边界有确定性回归，直接 JSON 与正式激活的严格校验仍保留"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "http-initialization",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/httpapi",
        "-run",
        "TestCreationAnalysisRelationshipSeedsReturnsFieldValidation",
        "-count=1"
      ],
      "covers": [
        "空值、混合类型和外层非数组等边界有确定性回归，直接 JSON 与正式激活的严格校验仍保留",
        "相关 Go 回归与提示词物理请求检查通过，必要的提示词组成清单同步"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-array-warning",
      "kind": "command",
      "command": [
        "node",
        "--test",
        "apps/web/test/demo-replica-creation-ui.test.mjs"
      ],
      "covers": [
        "初始化预览清楚提示哪些数组内容需要核对，并能查看保留的原值"
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
        "初始化预览清楚提示哪些数组内容需要核对，并能查看保留的原值"
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
        "初始化预览清楚提示哪些数组内容需要核对，并能查看保留的原值"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry",
    "legacy-9b89c41b4be449b963dcba95",
    "legacy-cb41a9af66866f54759af7f1",
    "legacy-b98cb585876a94318684c053"
  ],
  "feedback_of": "initialization-preview-field-contract",
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 修复初始化模型关系数组类型错误
