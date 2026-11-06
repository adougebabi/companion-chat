---
{
  "original_request": "现在主动唤醒附带的聊天回复几乎都是有类似“我理解。让我按照当前有效人格独立表达，不写第三人称旁白”的语句输出出来。是否是将思考模式的东西输出出来了。要在提示词优化这点吧。",
  "target": "定位并修复主动唤醒私聊把内部规则当作回复内容的问题，区分推理字段与模型正文并以真实请求路径回归。",
  "id": "proactive-reply-protocol-leak",
  "title": "修复主动唤醒回复复述内部指令",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core/agent_result_adapter.go",
    "apps/core-go/internal/core/fact_only_reply_capability.go",
    "apps/core-go/internal/core/private_reply_run.go",
    "apps/core-go/internal/core/reply_context.go",
    "apps/core-go/internal/core/reply_style.go",
    "apps/core-go/internal/core/reply_provider.go",
    "apps/core-go/internal/core/prompt_context_assembler.go",
    "apps/core-go/internal/core 相关回归测试",
    "docs/llm-prompt-composition.md"
  ],
  "non_goals": [
    "改写历史消息或用户正文",
    "修改旧版 Writer 协议字节和持久草稿 hash",
    "改变主动唤醒决策、权限或会话创建行为",
    "引入 token 限额、提交图片、发布或合并"
  ],
  "acceptance": [
    "主动唤醒触发的私聊不把内部任务指令伪装成用户新消息；模型收到清晰的主动任务语义与真实历史，同时不改写普通用户输入和历史。",
    "主动回复避免复述内部运行规则或表达计划，对高置信的无标签规则复述使用受限修复，保留正常对话和用户明确要求。",
    "通过实际 Provider HTTP 请求及针对性回归核对主动/普通聊天、异常文本修复、独立推理字段隔离与旧版持久协议；不引入 token 上限，真实模型效果有条件才宣称。"
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
      "id": "proactive-regression",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-run",
        "TestProactiveReply|TestPrivateChatStyle|TestPromptAssembler|TestProviderStructuredContent|TestADKReasoningOnly|TestEinoStreamObservation|TestReplyExpression",
        "-count=1"
      ],
      "covers": [
        "主动唤醒触发的私聊不把内部任务指令伪装成用户新消息；模型收到清晰的主动任务语义与真实历史，同时不改写普通用户输入和历史。",
        "主动回复避免复述内部运行规则或表达计划，对高置信的无标签规则复述使用受限修复，保留正常对话和用户明确要求。",
        "通过实际 Provider HTTP 请求及针对性回归核对主动/普通聊天、异常文本修复、独立推理字段隔离与旧版持久协议；不引入 token 上限，真实模型效果有条件才宣称。"
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
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 修复主动唤醒回复复述内部指令
