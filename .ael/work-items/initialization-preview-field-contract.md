---
{
  "original_request": "因为这一部分，之前也有遇到过类似的问题，就是让llm分析描述的时候，提供的字段不够多（或者说没告诉llm需要返回那些字段），导致字段缺漏。假如是这个问题，那就要进行修复。最起码第三步中预览的字段，让llm返回。但同时，也支持llm没返回，让后端进行兜底（也就是现在的空状态，让用户填写）。",
  "target": "核对第三步预览字段与初始化物理模型请求，修复未告知的字段及相关预览映射，保留语义未知时缺省和用户编辑兜底。",
  "id": "initialization-preview-field-contract",
  "title": "初始化模型补全预览字段合同并保留空值兜底",
  "type": "bug",
  "source": "user",
  "scope": [
    "初始化模型字段提示与结构化请求",
    "相关 Go 物理 HTTP 和缺项兜底回归",
    "初始化预览对应字段映射和必要验证",
    "模型提示词组成清单"
  ],
  "non_goals": [
    "臆造用户未提供的人物事实",
    "新增或调整模型 token 上限",
    "业务数据库迁移、发布、提交图片或 Git 提交"
  ],
  "acceptance": [
    "初始化物理模型请求明确提供第三步身份、共同性格、表达与相处和生活板块的有效字段与所属路径",
    "模型省略未知字段时后端保留空值和空容器，已有真实值包括零和 false 不被覆盖",
    "已提取的生活资料在对应第三步预览栏可读且可编辑",
    "真实 HTTP 请求回归和相应后端与前端验证通过；可用时执行真实模型样本回归"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "id": "core-preview-contract",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-run",
        "TestInitializationPreview",
        "-count=1"
      ],
      "covers": [
        "初始化物理模型请求明确提供第三步身份、共同性格、表达与相处和生活板块的有效字段与所属路径",
        "模型省略未知字段时后端保留空值和空容器，已有真实值包括零和 false 不被覆盖",
        "真实 HTTP 请求回归和相应后端与前端验证通过；可用时执行真实模型样本回归"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-preview",
      "kind": "command",
      "command": [
        "node",
        "--test",
        "apps/web/test/demo-replica-creation-ui.test.mjs"
      ],
      "covers": [
        "已提取的生活资料在对应第三步预览栏可读且可编辑",
        "真实 HTTP 请求回归和相应后端与前端验证通过；可用时执行真实模型样本回归"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry",
    "legacy-9b89c41b4be449b963dcba95",
    "legacy-cb41a9af66866f54759af7f1"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 初始化模型补全预览字段合同并保留空值兜底
