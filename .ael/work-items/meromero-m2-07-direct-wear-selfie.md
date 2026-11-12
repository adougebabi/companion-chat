---
{
  "original_request": "M2 原验收要求换衣自拍按照真实穿着和媒体结果执行；普通私聊从现有配置读取主模型，经过正式 direct 与真实 ToolCall/ToolResult，不固定进入 Writer。",
  "target": "验证并在必要时修复 direct 主链中先真实换衣、再请求自拍时的穿着事实冻结与媒体结果交付顺序。",
  "id": "meromero-m2-07-direct-wear-selfie",
  "title": "M2-07 默认主链换衣后自拍的事实与交付",
  "type": "feature",
  "source": "user",
  "scope": [
    "direct 私聊的衣柜发现、检查与换衣工具回执",
    "自拍媒体请求的当前穿着快照、受控渲染及正式交付和重放",
    "本场景必要的最小生产修复及隔离数据库测试"
  ],
  "non_goals": [
    "不写死模型、不改变 fact_only 开关及默认模型配置",
    "不修改提示词、采样参数、token 上限或回退策略来换取通过",
    "不修改四处原有未暂存草稿，不连接生产业务库",
    "不将受控 Provider、renderer、对象存储称为真实模型或真实生图",
    "不处理任务外非阻断缺陷，不完成全部 Image/Moment 验收"
  ],
  "acceptance": [
    "direct 正式对话通过原生工具发现、查询衣柜并使用实际查询结果完成换衣，再独立请求自拍并消费实际工具回执",
    "持久穿着在自拍受理前已改变；自拍媒体 intent 和实际渲染请求使用当时真实穿着，不使用未穿或编造衣物",
    "受控图片未真实交付前不声称已发出；受控流程完成后正式附件与资源引用只交付一次并可重放",
    "隔离数据库和直接相关既有回归实际通过；清楚保留真实模型、真实渲染与 M2 父任务未验收事项"
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
      "id": "m2-07-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "direct 正式对话通过原生工具发现、查询衣柜并使用实际查询结果完成换衣，再独立请求自拍并消费实际工具回执",
        "持久穿着在自拍受理前已改变；自拍媒体 intent 和实际渲染请求使用当时真实穿着，不使用未穿或编造衣物",
        "受控图片未真实交付前不声称已发出；受控流程完成后正式附件与资源引用只交付一次并可重放",
        "隔离数据库和直接相关既有回归实际通过；清楚保留真实模型、真实渲染与 M2 父任务未验收事项"
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
# M2-07 默认主链换衣后自拍的事实与交付
