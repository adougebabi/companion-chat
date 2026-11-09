---
{
  "original_request": "不是只整理文档吗？我要将所有 agent 的名字、作用、提示词（完整的组成，固定的 + 动态补充了什么东西进去）、调用时间列出来。",
  "target": "完善 docs/llm-agent-prompts.md，按当前实际调用列出每个 Agent/入口的名字、作用、完整提示词组成、固定协议内容、动态追加的数据字段、历史/人格/工具/schema/图片等组成，以及何时由哪个调用点触发。",
  "id": "document-agent-prompt-composition-and-timing",
  "title": "补全 Agent 完整提示词组成与调用时机",
  "type": "feature",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core 中 FormalAgent 注册、提示词构造、动态上下文与调用点",
    "Kev、Writer、修复重试及其他不在 FormalAgent 注册表中的模型提示词入口",
    "docs/llm-agent-prompts.md 与 docs/llm-prompt-composition.md"
  ],
  "non_goals": [
    "修改任何运行时提示词、模型请求、调用时机或产品行为",
    "恢复或整理已废弃历史版本为当前提示词",
    "新增或调整模型输入/输出 token 上限、截断或预算拒绝"
  ],
  "acceptance": [
    "当前30个正式 Agent 逐项列出名称、作用、固定协议组成、动态输入字段、输出/schema、工具/图片/历史组成（如适用）和真实调用时机/条件",
    "共享 Agent ID 的每个实际 typed task variant 分开列出差异，不把 schema 名或辅助函数误列为新 Agent",
    "Kev 与 Writer/修复等非正式注册表入口单独说明实际协议组成和触发位置",
    "每项提示词组成与调用时机都有可定位的当前源码引用；静态固定内容与运行时动态数据明确分开",
    "文档明确哪些来源为源码摘要、哪些是精确文本；不把最终动态请求误写成固定完整字符串",
    "提示词组成清单与新目录互相链接，符合当前提示词构造规范"
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
      "method": "model",
      "id": "review",
      "kind": "review",
      "command": null,
      "covers": [
        "当前30个正式 Agent 逐项列出名称、作用、固定协议组成、动态输入字段、输出/schema、工具/图片/历史组成（如适用）和真实调用时机/条件",
        "共享 Agent ID 的每个实际 typed task variant 分开列出差异，不把 schema 名或辅助函数误列为新 Agent",
        "Kev 与 Writer/修复等非正式注册表入口单独说明实际协议组成和触发位置",
        "每项提示词组成与调用时机都有可定位的当前源码引用；静态固定内容与运行时动态数据明确分开",
        "文档明确哪些来源为源码摘要、哪些是精确文本；不把最终动态请求误写成固定完整字符串",
        "提示词组成清单与新目录互相链接，符合当前提示词构造规范"
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
  "feedback_of": "document-current-agent-prompts",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 补全 Agent 完整提示词组成与调用时机
