---
{
  "original_request": "用户授权仅执行 M1；内部子任务串行收口，已满足依赖者自动推进；允许为当前交付行为增加必要的媒体证据引用、上下文投影、局部接口及测试，不改造 AEL、不跨 M2–M4，完成或不可解除阻塞时生成 M1-review-for-chatgpt.md 并停止。",
  "target": "按原 T02/T04/T06 收口 M1：可靠的媒体完成声明与交付证据、工具终止边界、必要模型快照和旧任务兼容策略，完成 M1 集成验收。",
  "id": "meromero-m1-reliability",
  "title": "M1 基础可靠性与主链阻塞消除",
  "type": "feature",
  "source": "user",
  "scope": [
    "原 T02/T04/T06 中 M1 必要的发布可靠性、证据、工具终止及模型恢复",
    "docs/verification/meromero-m1-progress-20261112.md",
    "M1-review-for-chatgpt.md"
  ],
  "non_goals": [
    "不切换默认聊天链、不整体重构插件、不实施 M2–M4",
    "不修改或删除真实业务数据，不连接生产业务库或执行真实不可逆操作",
    "不提交图片、不覆盖既有四处工作区草稿、不增加 token 上限"
  ],
  "acceptance": [
    "无真实交付依据的 direct 媒体完成声明不可发布，当前及历史真实交付表达可用",
    "queued、ready、failed 不冒充 delivered，工具权限拒绝和终止错误无无意义重复执行",
    "模型配置/快照及旧任务兼容策略明确，相关普通聊天和正式发布回归通过",
    "M1 相关真实与受控证据分层记录，未解决项不冒充通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-first-humanization-plugin-governance",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m1-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "无真实交付依据的 direct 媒体完成声明不可发布，当前及历史真实交付表达可用",
        "queued、ready、failed 不冒充 delivered，工具权限拒绝和终止错误无无意义重复执行",
        "模型配置/快照及旧任务兼容策略明确，相关普通聊天和正式发布回归通过",
        "M1 相关真实与受控证据分层记录，未解决项不冒充通过"
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
# M1 基础可靠性与主链阻塞消除
