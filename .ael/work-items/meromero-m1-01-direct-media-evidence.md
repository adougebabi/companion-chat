---
{
  "original_request": "用户要求从既有 direct_media_delivery_claim_test.go 红测开始；为完整行为可增加必要的媒体证据引用、上下文投影、正式发布接口和局部状态字段。不得仅凭任意历史附件或单条正则证明一致，不固定 Qwen 审核，不更改默认聊天链；按当前真实证据区分 queued、ready、failed 与 delivered。",
  "target": "在正式 direct 发布路径中阻止无真实媒体交付依据的完成声明，同时允许与指定任务匹配的当前或历史真实交付、普通闲聊、否认及未来意愿。",
  "id": "meromero-m1-01-direct-media-evidence",
  "title": "M1-01 direct 媒体完成声明与真实交付依据",
  "type": "feature",
  "source": "user",
  "scope": [
    "direct 会话回复、其授权媒体交付证据与必需的模型上下文/接口",
    "相关隔离 PostgreSQL、真实 ToolCall 和受控 Provider 回归"
  ],
  "non_goals": [
    "不切换默认对话模式、不整体重构 Image/Moment 或建立新 Runtime",
    "不连接生产业务库、不提交图片、不覆盖已有四处草稿",
    "不新增输入/输出 token 上限或固定 Qwen 审核"
  ],
  "acceptance": [
    "无对应真实交付依据的照片已发送声明不能正式发布，保留失败前真实证据",
    "有明确任务引用且权威记录证明交付的当前/历史媒体表达不被误拦，无关历史照片不能替代",
    "普通闲聊、否认发送、未来意愿不因关键词被一律拦截，queued/ready/failed 不当成 delivered",
    "原同批工具保护、正常 direct 发布及既有稳定聊天回归通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-m1-reliability",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m1-01-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "无对应真实交付依据的照片已发送声明不能正式发布，保留失败前真实证据",
        "有明确任务引用且权威记录证明交付的当前/历史媒体表达不被误拦，无关历史照片不能替代",
        "普通闲聊、否认发送、未来意愿不因关键词被一律拦截，queued/ready/failed 不当成 delivered",
        "原同批工具保护、正常 direct 发布及既有稳定聊天回归通过"
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
  "feedback_of": "meromero-t06-delivery-consistency",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M1-01 direct 媒体完成声明与真实交付依据
