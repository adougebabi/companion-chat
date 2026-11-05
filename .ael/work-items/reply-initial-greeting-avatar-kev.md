---
{
  "original_request": "修复回复失败时互相矛盾且入口不一致的反馈；初始化后的招呼出现旁白、附录或自称 AI；角色根据自身设定构思正式头像并让模型生图；Kev 超时设为一分钟；启用 Kev 时主动唤醒应先由 Kev 判断。",
  "target": "交付上述五项修复与对应回归，保持原始消息和执行安全语义，保留旧提示词与视觉身份版本栅栏。",
  "id": "reply-initial-greeting-avatar-kev",
  "title": "修复回复反馈、初始问候、头像构思与 Kev 唤醒",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/state/conversation.ts",
    "apps/web/src/react-app/pages/ChatPage.tsx",
    "apps/web/src/react-app/components/ChatTurnFeedback.tsx",
    "apps/web/src/v3/chat-feedback.mjs",
    "apps/web/test",
    "apps/core-go/internal/core/agent_result_adapter.go",
    "apps/core-go/internal/core/kev_",
    "apps/core-go/internal/core/reply_",
    "apps/core-go/internal/core/fact_only_reply_capability.go",
    "apps/core-go/internal/core/generated_avatar.go",
    "apps/core-go/internal/core/visual_identity.go",
    "apps/core-go/internal/core/visual_asset_review.go",
    "apps/core-go/internal/core/media.go",
    "apps/core-go/internal/core/model_tasks.go",
    "apps/core-go/internal/ai/decision",
    "apps/core-go/internal/migrations",
    "apps/core-go/internal/workflow",
    "docs/llm-prompt-composition.md"
  ],
  "non_goals": [
    "提交图片或生产环境发布",
    "改变用户资料和历史消息内容",
    "引入提示词 token 限额或绕过视觉审核",
    "更改缺少直接会话时是否创建会话的既有行为；该局部合同仍有未决冲突"
  ],
  "acceptance": [
    "失败或取消的回复状态不显示互相矛盾的输入、核对和错误提示，操作链接间距及同一行布局正确，诊断入口指向对应回合且重试安全不变",
    "首次主动问候按角色聊天表达，不把内部场景、状态附录或 AI 身份暴露为回复；有真实请求与异常输出回归",
    "正式头像基于角色设定由文本模型自主构思后交给图像生成，同时维护参考身份、审核及失败保护",
    "Kev 请求及适用阶段默认上限为一分钟，已有配置安全迁移，超时诊断保持真实",
    "启用 Kev 的主动唤醒在执行前进行决策，保留关闭、失败与手动交互的清晰行为并覆盖回归",
    "执行关联 Go、Web 回归及构建/类型检查，必要的真实页面交互留下证据"
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
      "id": "kev-unit",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/ai/decision"
      ],
      "covers": [
        "Kev 请求及适用阶段默认上限为一分钟，已有配置安全迁移，超时诊断保持真实"
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
        "失败或取消的回复状态不显示互相矛盾的输入、核对和错误提示，操作链接间距及同一行布局正确，诊断入口指向对应回合且重试安全不变"
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
        "执行关联 Go、Web 回归及构建/类型检查，必要的真实页面交互留下证据"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "regression-scenarios",
      "kind": "scenario",
      "command": null,
      "covers": [
        "失败或取消的回复状态不显示互相矛盾的输入、核对和错误提示，操作链接间距及同一行布局正确，诊断入口指向对应回合且重试安全不变",
        "首次主动问候按角色聊天表达，不把内部场景、状态附录或 AI 身份暴露为回复；有真实请求与异常输出回归",
        "正式头像基于角色设定由文本模型自主构思后交给图像生成，同时维护参考身份、审核及失败保护",
        "Kev 请求及适用阶段默认上限为一分钟，已有配置安全迁移，超时诊断保持真实",
        "启用 Kev 的主动唤醒在执行前进行决策，保留关闭、失败与手动交互的清晰行为并覆盖回归",
        "执行关联 Go、Web 回归及构建/类型检查，必要的真实页面交互留下证据"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "reply-ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "失败或取消的回复状态不显示互相矛盾的输入、核对和错误提示，操作链接间距及同一行布局正确，诊断入口指向对应回合且重试安全不变"
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
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 修复回复反馈、初始问候、头像构思与 Kev 唤醒
