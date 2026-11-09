---
{
  "original_request": "修复分支媒体提示词规范化请求未向 Provider 发送 JSON Schema、导致模型漏掉 declared_human_count 并失败的问题。",
  "target": "分支媒体提示词规范化路径实际发送严格 JSON Schema，并在真实 Provider HTTP 请求回归中证明响应格式包含必需字段；既有自由文本媒体任务保持原契约。",
  "id": "media-normalizer-response-schema",
  "title": "修复媒体提示词结构化响应契约",
  "type": "bug",
  "source": "user",
  "scope": [
    "分支媒体提示词规范化 Agent 的输出契约及 Provider 请求",
    "Formal Agent 清单、lossless composition 回归与提示词组成文档",
    "通过真实 Provider HTTP JSON 请求验证 response_format 和声明字段"
  ],
  "non_goals": [
    "不改变 human_count 的来源与人数语义校验",
    "不改变旧版自由文本媒体任务的输出契约",
    "不新增、调整或发送任何 token 上限",
    "不修改或提交图片，不发布"
  ],
  "acceptance": [
    "media_prompt_normalized 的物理 Provider 请求包含 strict JSON Schema response_format，且 schema 要求 declared_human_count",
    "规范化模型返回符合 schema 的 JSON 后，后端验证通过并保留声明人数",
    "旧版纯文本媒体提示词任务仍按文本输出，不受结构化输出修复影响",
    "Formal Agent 提示词组成清单及 docs/llm-prompt-composition.md 与实现一致"
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
      "id": "media-normalizer-http-regression",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-run",
        "TestMediaNormalizedPromptUsesProviderSchema"
      ],
      "covers": [
        "media_prompt_normalized 的物理 Provider 请求包含 strict JSON Schema response_format，且 schema 要求 declared_human_count",
        "规范化模型返回符合 schema 的 JSON 后，后端验证通过并保留声明人数",
        "旧版纯文本媒体提示词任务仍按文本输出，不受结构化输出修复影响",
        "Formal Agent 提示词组成清单及 docs/llm-prompt-composition.md 与实现一致"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "legacy-9b89c41b4be449b963dcba95",
    "lac-project-entry"
  ],
  "feedback_of": "native-reply-image-prompt",
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 修复媒体提示词结构化响应契约
