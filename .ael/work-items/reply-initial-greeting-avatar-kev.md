---
{
  "original_request": "修复回复失败时互相矛盾且入口不一致的反馈；初始化后的招呼出现旁白、附录或自称 AI；角色根据自身设定构思正式头像并让模型生图；Kev 超时设为一分钟；启用 Kev 时主动唤醒应先由 Kev 判断。",
  "target": "将默认聊天回复 agent 的固定运行协议替换为用户提供的内容，尽量保留原文，同时维护协议版本兼容、真实请求回归与提示词组成文档。",
  "id": "reply-initial-greeting-avatar-kev",
  "title": "更新聊天回复 Agent 运行协议",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core/reply_expression.go",
    "apps/core-go/internal/core/reply_context.go",
    "apps/core-go/internal/core/reply_image_request.go",
    "apps/core-go/internal/core/reply_identity_test.go",
    "apps/core-go/internal/core/reply_expression_test.go",
    "apps/core-go/internal/core/reply_provider_test.go",
    "apps/web/src/v3/proto/components/configuration-fields.js",
    "apps/web/test/reply-expression-version.test.mjs",
    "docs/llm-prompt-composition.md"
  ],
  "non_goals": [
    "提交图片或生产环境发布",
    "头像构思与生成、Kev 唤醒及回复反馈 UI 等原工作项剩余内容",
    "改变用户资料和历史消息内容",
    "修改已冻结的 v4–v7 回复协议或旧草稿版本语义",
    "引入提示词 token 限额或改变当前工具/图片传输契约"
  ],
  "acceptance": [
    "默认聊天回复 agent 的运行协议替换为用户提供的段落内容，除排版整理外尽量不改动文字含义",
    "协议通过新版本挂载，现有 v4–v7 协议及已持久化草稿恢复语义保持有效，新版本成为默认且版本选择器同步",
    "新增或更新真实 Provider HTTP 请求回归，确认新协议进入 system 请求且原生图片工具和 persona.detail 工具仍按真实协议传递",
    "同步 docs/llm-prompt-composition.md 的正式 Agent 条目、默认版本与兼容说明"
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
      "id": "reply-prompt-regression",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core"
      ],
      "covers": [
        "默认聊天回复 agent 的运行协议替换为用户提供的段落内容，除排版整理外尽量不改动文字含义",
        "协议通过新版本挂载，现有 v4–v7 协议及已持久化草稿恢复语义保持有效，新版本成为默认且版本选择器同步",
        "新增或更新真实 Provider HTTP 请求回归，确认新协议进入 system 请求且原生图片工具和 persona.detail 工具仍按真实协议传递"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "reply-prompt-ui",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "test"
      ],
      "covers": [
        "协议通过新版本挂载，现有 v4–v7 协议及已持久化草稿恢复语义保持有效，新版本成为默认且版本选择器同步"
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
        "协议通过新版本挂载，现有 v4–v7 协议及已持久化草稿恢复语义保持有效，新版本成为默认且版本选择器同步"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "prompt-inventory-review",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同步 docs/llm-prompt-composition.md 的正式 Agent 条目、默认版本与兼容说明"
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
  "definition_revision": 4,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 更新聊天回复 Agent 运行协议
