---
{
  "original_request": "帮我把现在各个agent的提示词整理到md里面。描述清楚该agent的作用是什么，提示词是什么",
  "target": "新增一份可核对源码的 Markdown 文档，完整列出当前正式 Agent、Writer/Kev 入口及共享 Agent 的任务变体，说明每个 Agent 的作用、稳定提示词协议、运行时动态输入、输出形式和源码位置；同步现有提示词组成清单入口。",
  "id": "document-current-agent-prompts",
  "title": "整理当前 Agent 的作用与提示词",
  "type": "feature",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core 中当前正式 Agent 注册表、任务专用提示词与共享构造器",
    "Writer、Kev、纠错/修复请求及正式 Agent 的 typed 子任务变体",
    "docs/llm-prompt-composition.md 及新增的 Agent 提示词说明文档"
  ],
  "non_goals": [
    "修改任何运行时 Agent 提示词或模型请求行为",
    "整理已废弃或历史版本的 Agent 为当前提示词",
    "新增、调整或恢复任何模型输入/输出 token 限制"
  ],
  "acceptance": [
    "覆盖当前注册表中的全部正式 Agent，并明确共享 Agent ID 下各个不同任务变体",
    "逐 Agent 说明用途、稳定提示词内容或协议、动态输入、输出形态和可追溯源码位置；明确区分源码原文与便于阅读的摘要",
    "覆盖独立 Writer、Kev 决策及修复/重试等有独立提示词协议的入口，说明它们与正式 Agent 的关系",
    "文档声明实际请求还会含运行时数据，不能将静态清单误读为每次调用的完整字节级 prompt",
    "现有提示词组成清单链接到新文档，且文档内容遵守当前提示词构造规范"
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
        "覆盖当前注册表中的全部正式 Agent，并明确共享 Agent ID 下各个不同任务变体",
        "逐 Agent 说明用途、稳定提示词内容或协议、动态输入、输出形态和可追溯源码位置；明确区分源码原文与便于阅读的摘要",
        "覆盖独立 Writer、Kev 决策及修复/重试等有独立提示词协议的入口，说明它们与正式 Agent 的关系",
        "文档声明实际请求还会含运行时数据，不能将静态清单误读为每次调用的完整字节级 prompt",
        "现有提示词组成清单链接到新文档，且文档内容遵守当前提示词构造规范"
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
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 整理当前 Agent 的作用与提示词
