---
{
  "original_request": "一开始提到的头像、角色设定图和形象参考图的提示词还没给我吧。现在似乎直接将形象参考图作为头像了。这里应该是3张不一样的。但是现在的话，形象参考图和角色设定图先用同一段prompt（我后面再调整），但头像是不能用这个的。应该用一段专门生产头像的提示词。",
  "target": "让形象参考图、角色设定图和显示头像有独立产物；前两者保留共同提示词，头像使用专门的肖像提示词并由真实媒体任务生成。",
  "id": "independent-generated-avatar",
  "title": "独立生成显示头像并使用专用提示词",
  "type": "feature",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core/display_avatar.go",
    "apps/core-go/internal/core/generated_avatar.go",
    "apps/core-go/internal/core/visual_asset_review.go",
    "apps/core-go/internal/core/visual_identity.go",
    "apps/core-go/internal/core/visual_identity_agent.go",
    "apps/core-go/internal/core/visual_identity_tools.go",
    "apps/core-go/internal/core/visual_brief.go",
    "apps/core-go/internal/core/media.go",
    "apps/core-go/internal/core/media_quality.go",
    "apps/core-go/internal/core/model_tasks.go",
    "apps/core-go/internal/core/media_lifecycle.go",
    "apps/core-go/internal/core/owner_media_library.go",
    "apps/core-go/internal/migrations/runner.go",
    "apps/core-go/internal/httpapi/browser/surface_routes.go",
    "apps/core-go/internal/httpapi/browser/media_validation.go",
    "apps/core-go/internal/httpapi/server.go",
    "packages/browser-client/scripts/media-library-contract.mjs",
    "packages/browser-client/scripts/visual-brief-contract.mjs",
    "packages/browser-client/scripts/generate.mjs",
    "packages/browser-client/openapi.json",
    "packages/browser-client/src/index.ts",
    "apps/web/src/approved-ui/creation-progress.mjs",
    "apps/web/src/react-app/shared/VisualWorkspace.jsx",
    "apps/web/src/react-app/shared/AvatarWorkspace.jsx",
    "apps/web/src/styles/replica-media.css",
    "相关测试、页面文案、视觉规范和提示词清单"
  ],
  "non_goals": [
    "修改形象参考图或角色设定图的共同模板",
    "覆盖用户明确选择的显示头像",
    "实际修改用户业务数据库或自动批量生成历史角色头像",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "确认并说明三种图片的实际提示词及生成顺序",
    "接受形象参考后，角色设定图和专用头像作为独立媒体任务生成，产物与参考图相互独立",
    "头像提示词明确肖像构图并保留已确认外貌和可用的参考图；前两种图片的模板不变",
    "三张候选分别审核：视觉模型可用时使用真实图片审核，否则等待人工决定；通过无需额外理由，不通过保留实际修改要求",
    "创建进度和头像页面根据真实头像任务状态展示，不把参考图冒充生成头像；手动选图不被后台覆盖",
    "旧角色可在头像页安全地补生成，任务幂等并防止旧结果覆盖新参考",
    "隔离数据库、真实 HTTP 模拟媒体请求、前端交互与相关构建测试通过"
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
      "id": "avatar-core",
      "kind": "scenario",
      "command": null,
      "covers": [
        "确认并说明三种图片的实际提示词及生成顺序",
        "接受形象参考后，角色设定图和专用头像作为独立媒体任务生成，产物与参考图相互独立",
        "头像提示词明确肖像构图并保留已确认外貌和可用的参考图；前两种图片的模板不变",
        "三张候选分别审核：视觉模型可用时使用真实图片审核，否则等待人工决定；通过无需额外理由，不通过保留实际修改要求",
        "旧角色可在头像页安全地补生成，任务幂等并防止旧结果覆盖新参考"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "avatar-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "三张候选分别审核：视觉模型可用时使用真实图片审核，否则等待人工决定；通过无需额外理由，不通过保留实际修改要求",
        "创建进度和头像页面根据真实头像任务状态展示，不把参考图冒充生成头像；手动选图不被后台覆盖",
        "旧角色可在头像页安全地补生成，任务幂等并防止旧结果覆盖新参考"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-test",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "test"
      ],
      "covers": [
        "三张候选分别审核：视觉模型可用时使用真实图片审核，否则等待人工决定；通过无需额外理由，不通过保留实际修改要求",
        "创建进度和头像页面根据真实头像任务状态展示，不把参考图冒充生成头像；手动选图不被后台覆盖"
      ],
      "required": true,
      "timeout_seconds": 300,
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
        "隔离数据库、真实 HTTP 模拟媒体请求、前端交互与相关构建测试通过"
      ],
      "required": true,
      "timeout_seconds": 300,
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
        "隔离数据库、真实 HTTP 模拟媒体请求、前端交互与相关构建测试通过"
      ],
      "required": true,
      "timeout_seconds": 300,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "visual-prompts-manual-review",
  "definition_revision": 5,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 独立生成显示头像并使用专用提示词
