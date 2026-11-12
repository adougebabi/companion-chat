---
{
  "original_request": "M2/T05 原验收：关闭 Image，文字聊天与纯文字动态仍可运行；Tool 独立调用与 Agent 调用经过正式权限、幂等与领域校验。",
  "target": "验证并在必要时修复 Image 停用时默认 direct 私聊与纯文字 Moment 的正式运行隔离。",
  "id": "meromero-m2-03-image-off-text-moment",
  "title": "M2-03 关闭图片后私聊与纯文字动态保持可用",
  "type": "feature",
  "source": "user",
  "scope": [
    "Image 停用时默认私聊、Moment 文本发布、工具发现与准入",
    "本项隔离 PostgreSQL 及受控 Provider 正式入口回归"
  ],
  "non_goals": [
    "不改变 fact_only 模式开关和模型绑定",
    "不修改 Prompt、采样、Token 上限或真实模型合同",
    "不处理伤情数据来源等其他待办，不访问生产数据库或真实发布",
    "不重构 Image/Moment 插件和 UI，不覆盖四处原有未提交草稿"
  ],
  "acceptance": [
    "通过正式设置关闭 Image、保留 Moment 后，新的默认 direct 私聊仍可提交文本回复",
    "Agent 可调用纯文字 Moment 的正式工具并消费结果，独立调用也可发布并保持幂等",
    "已停用的 Image 不出现在新模型工具清单，独立图片新请求被拒绝且不创建媒体任务",
    "相关已有启停、重放和回复测试在隔离库执行，区分受控 Provider 与真实模型证据"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t05-image-moment-plugins",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-03-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "通过正式设置关闭 Image、保留 Moment 后，新的默认 direct 私聊仍可提交文本回复",
        "Agent 可调用纯文字 Moment 的正式工具并消费结果，独立调用也可发布并保持幂等",
        "已停用的 Image 不出现在新模型工具清单，独立图片新请求被拒绝且不创建媒体任务",
        "相关已有启停、重放和回复测试在隔离库执行，区分受控 Provider 与真实模型证据"
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
# M2-03 关闭图片后私聊与纯文字动态保持可用
