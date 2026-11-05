---
{
  "original_request": "调整头像生成提示词：由角色依据性格经历等自主选择符合角色心智的头像主体，包括非人物；若包含本人保留已确认外貌；正方形且适合圆裁、不出现文字标志或拼图，只输出英文生图提示词。考虑按任务动态传递生图 ratio 和 resolution，例如头像 1:1、普通图片 9:16 或 16:9。",
  "target": "让头像构思、渲染及审核一致支持角色自主选择头像主体，并按任务把有依据的尺寸传给可配置生图工作流，保留已有任务和工作流兼容性。",
  "id": "avatar-creative-prompt-image-dimensions",
  "title": "头像自主构思提示词与生图尺寸传递",
  "type": "feature",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core 中头像提示词、视觉审核与媒体生成尺寸解析/工作流替换",
    "相关受控物理 HTTP 与回归测试",
    "docs/llm-prompt-composition.md 及必要的工作流配置说明"
  ],
  "non_goals": [
    "改变角色设定图与形象参考图既有画面语义",
    "覆盖已提交或已冻结的历史媒体任务",
    "引入提示词 token 限额或绕过视觉审核",
    "提交图片、Git 提交或发布"
  ],
  "acceptance": [
    "头像文本模型使用用户提供的完整运行协议及角色动态资料，仅输出英文生图提示词，非人物主体不被后续人像约束覆盖",
    "头像审核按所选主体及画面用途判断，人物时保持已确认外貌，非人物不因没有人脸失败；参考图和版本栅栏保持",
    "头像采用 1:1，普通图片可以按任务要求选择竖版或横版，尺寸可传入 ComfyUI 工作流实际请求，兼容现有固定尺寸工作流及恢复语义",
    "物理请求和针对性回归覆盖角色画像、提示词、审核及实际生图工作流中的尺寸与类型，相关 Go 测试通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "id": "core-tests",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-count=1"
      ],
      "covers": [
        "头像文本模型使用用户提供的完整运行协议及角色动态资料，仅输出英文生图提示词，非人物主体不被后续人像约束覆盖",
        "头像审核按所选主体及画面用途判断，人物时保持已确认外貌，非人物不因没有人脸失败；参考图和版本栅栏保持",
        "头像采用 1:1，普通图片可以按任务要求选择竖版或横版，尺寸可传入 ComfyUI 工作流实际请求，兼容现有固定尺寸工作流及恢复语义",
        "物理请求和针对性回归覆盖角色画像、提示词、审核及实际生图工作流中的尺寸与类型，相关 Go 测试通过"
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
  "feedback_of": "independent-generated-avatar",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 头像自主构思提示词与生图尺寸传递
