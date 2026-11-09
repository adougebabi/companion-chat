---
{
  "original_request": "我要的是能单独审核的文档，而不是你现在的类似证据形态的东西。我要所有 Agent 的名字、作用、提示词完整组成（固定和动态补充内容）、调用时间。",
  "target": "将 Agent 提示词目录改成可离开源码独立审核的 Markdown：直接展示各正式 Agent 当前固定提示词原文及适用分支，明确动态输入结构、原生工具/图片/schema、调用触发时机和角色作用；共享部分的原文也完整保留在同一文档中。",
  "id": "make-agent-prompts-standalone-reviewable",
  "title": "把 Agent 提示词整理为可独立审核的正文",
  "type": "feature",
  "source": "user",
  "scope": [
    "docs/llm-agent-prompts.md 及必要的目录链接",
    "只读追踪 FormalAgent、Writer、Kev、修复和共享提示词的实际构造与调用点"
  ],
  "non_goals": [
    "修改 Agent 提示词、模型调用或产品行为",
    "复制用户私人对话、密钥或真实运行数据到文档",
    "增加或调整任何输入输出 token 限制或截断"
  ],
  "acceptance": [
    "30 个正式 Agent 逐项列出名字、作用、调用时间/条件和其生产任务变体",
    "文档中直接包含所有需要审核的固定提示词正文和适用条件；读者不需要打开源码才能读到固定指令",
    "每项明确实际消息构造顺序、动态注入的字段/人格/历史/工具/图片与输出协议；动态值用占位符而非真实隐私数据",
    "共享固定协议、Kev 与同 Agent 的修复重试提示词也在同一文档中给出正文及触发条件；未接入生产的注册入口明确状态",
    "所复制的固定文本与当前源码一致，文档明确区分原文、变量占位与说明；保留必要出处供维护"
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
      "method": "model",
      "id": "review",
      "kind": "review",
      "command": null,
      "covers": [
        "30 个正式 Agent 逐项列出名字、作用、调用时间/条件和其生产任务变体",
        "文档中直接包含所有需要审核的固定提示词正文和适用条件；读者不需要打开源码才能读到固定指令",
        "每项明确实际消息构造顺序、动态注入的字段/人格/历史/工具/图片与输出协议；动态值用占位符而非真实隐私数据",
        "共享固定协议、Kev 与同 Agent 的修复重试提示词也在同一文档中给出正文及触发条件；未接入生产的注册入口明确状态",
        "所复制的固定文本与当前源码一致，文档明确区分原文、变量占位与说明；保留必要出处供维护"
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
  "feedback_of": "document-agent-prompt-composition-and-timing",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 把 Agent 提示词整理为可独立审核的正文
