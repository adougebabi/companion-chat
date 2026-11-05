---
{
  "original_request": "所以的话，先按照我现在的思路，一个是强化tool的描述，另外一个是提示词部分。先把这两个优化了吧。我实际再测试记录。",
  "target": "按用户已验证的方向优化聊天回复图片工具描述和媒体提示词，允许角色在恰当场景主动请求图片并保持真实回执与旧协议恢复；完成实际请求回归，用户后续自行实测效果。",
  "id": "native-reply-image-prompt",
  "title": "优化聊天回复的图片工具与媒体提示词",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core/reply_image_request.go",
    "apps/core-go/internal/core/reply_provider.go",
    "apps/core-go/internal/core/reply_expression.go",
    "apps/core-go/internal/core/reply_context.go",
    "apps/core-go/internal/core/reply_vision_settings.go",
    "apps/core-go/internal/core/*reply*test.go",
    "apps/web/src/v3/proto/components/configuration-fields.js",
    "apps/web/test/reply-expression-version.test.mjs",
    "docs/llm-prompt-composition.md",
    ".ael/knowledge/spec/legacy-4bc80c68710c2ca549e16e7e.md"
  ],
  "non_goals": [
    "恢复或实施用户暂缓的文本 JSON 工具兼容模式",
    "修改历史聊天、角色设定或用户正文",
    "强制每次主动唤醒发送图片",
    "增加输入输出 token 限制、提交图片或发布部署"
  ],
  "acceptance": [
    "图片请求工具采用用户提出的强直接调用描述，并覆盖角色自主分享图片的实际场景",
    "聊天回复系统媒体规则简洁明确，保留真实工具结果、未发送不得声称完成及视觉边界",
    "旧版 v4/v5/v6 的提示词合同和已冻持久草稿保持可恢复，新版本可供新回复使用",
    "真实序列化模型请求及工具回执回归通过，历史消息、原生调用和无 token 上限契约保持",
    "表达版本设置选项、关联测试和提示词组成清单同步"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": true,
  "ux": false,
  "checks": [
    {
      "id": "reply-go",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-run",
        "TestReply|TestPrivateChat|TestFactOnlyReply|TestProactiveReply",
        "-count=1"
      ],
      "covers": [
        "图片请求工具采用用户提出的强直接调用描述，并覆盖角色自主分享图片的实际场景",
        "聊天回复系统媒体规则简洁明确，保留真实工具结果、未发送不得声称完成及视觉边界",
        "旧版 v4/v5/v6 的提示词合同和已冻持久草稿保持可恢复，新版本可供新回复使用",
        "真实序列化模型请求及工具回执回归通过，历史消息、原生调用和无 token 上限契约保持"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "reply-web",
      "kind": "command",
      "command": [
        "node",
        "--test",
        "apps/web/test/reply-expression-version.test.mjs"
      ],
      "covers": [
        "表达版本设置选项、关联测试和提示词组成清单同步"
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
        "表达版本设置选项、关联测试和提示词组成清单同步"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "settings-ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "表达版本设置选项、关联测试和提示词组成清单同步"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry",
    "legacy-9b89c41b4be449b963dcba95",
    "legacy-4bc80c68710c2ca549e16e7e"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 优化聊天回复的图片工具与媒体提示词
