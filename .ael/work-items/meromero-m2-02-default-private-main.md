---
{
  "original_request": "M2/T04：默认新私聊由设置中选择的模型直接完成决策和回复，取消固定二段式；MeroMero 是配置的模型而不是写死的名称。用户明确要求保留 fact_only 可选开关及已有显式选择。",
  "target": "让未显式选择回复模式的新正式私聊默认使用已有配置模型的直接回复链，同时保留明确选择的 fact_only/legacy 和旧草稿。",
  "id": "meromero-m2-02-default-private-main",
  "title": "M2-02 新私聊使用配置的主模型直接回复",
  "type": "feature",
  "source": "user",
  "scope": [
    "未显式选择模式时的私聊默认值及设置显示",
    "明确选择的旧模式和旧草稿恢复回归",
    "隔离数据库、受控 Provider 与本机真实 Provider 验证"
  ],
  "non_goals": [
    "不写死模型 ID 或建立新的模型配置源",
    "不覆盖用户已保存的模式或移除 fact_only 开关",
    "不更改显式 Agent/variant 模型覆盖优先级",
    "不修改 Prompt、采样、Token 限额或媒体/插件业务",
    "不处理已登记的非阻断问题，不覆盖四处原有未提交草稿"
  ],
  "acceptance": [
    "无回复设置或缺省 mode 时，新正式私聊由当前配置的主模型直接发布，不固定调用独立 Writer",
    "用户显式选择 fact_only 或 legacy 时保持现有模式及设置，界面继续可选择",
    "模型绑定、显式 Agent/variant 覆盖、原生工具结果及认知结算和发布幂等有效",
    "同源旧草稿保留原合同，其他非主私聊入口保留自身既有语义",
    "隔离库和受控 Provider 正式路径验证，真实本机模型结果单独记录"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t04-single-main-chat",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-02-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "无回复设置或缺省 mode 时，新正式私聊由当前配置的主模型直接发布，不固定调用独立 Writer",
        "用户显式选择 fact_only 或 legacy 时保持现有模式及设置，界面继续可选择",
        "模型绑定、显式 Agent/variant 覆盖、原生工具结果及认知结算和发布幂等有效",
        "同源旧草稿保留原合同，其他非主私聊入口保留自身既有语义",
        "隔离库和受控 Provider 正式路径验证，真实本机模型结果单独记录"
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
# M2-02 新私聊使用配置的主模型直接回复
