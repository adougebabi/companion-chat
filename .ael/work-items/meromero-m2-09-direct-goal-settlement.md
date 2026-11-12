---
{
  "original_request": "M2/T04 原始要求迁移主链时保留 claims、influences、goal_event_candidates 等认知结算和实际下游业务效果；候选不等于目标已经完成。",
  "target": "验证 direct 正式回复产生的真实消息证据能经持久 Goal 评估任务完成原有结算，重放不重复。",
  "id": "meromero-m2-09-direct-goal-settlement",
  "title": "M2-09 direct 主链 Goal 候选的实际下游结算",
  "type": "feature",
  "source": "user",
  "scope": [
    "direct 主链的非空 Goal 引用、消息与评估请求",
    "正式 Goal 评估任务对已提交消息证据的消费、结算和重放",
    "必要的本范围局部修复及隔离数据库回归"
  ],
  "non_goals": [
    "不修改 Goal 成功判定规则或从候选直接推定完成",
    "不修改默认模型配置、fact_only 开关、Prompt、采样参数和 token 上限",
    "不修改四处原有未暂存草稿，不连接真实业务库",
    "不将受控 Provider 或直接调用工作入口称为真实模型或真实 Temporal"
  ],
  "acceptance": [
    "direct 回复真实提交前提供有效 Goal 引用，结算后同一消息关联目标候选并产生持久评估任务；评估前目标仍未完成",
    "正式评估入口只消费真实已提交消息及原始 Goal 条件，受控评估满足条件后产生对应真实领域结算",
    "重复执行已完成的回复与评估不会重复生成消息、评估、目标结算或模型请求",
    "相关引用校验、回滚、fact_only 开关和 Goal 原有回归通过；不宣称真实模型或整个 M2 完成"
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
      "id": "m2-09-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "direct 回复真实提交前提供有效 Goal 引用，结算后同一消息关联目标候选并产生持久评估任务；评估前目标仍未完成",
        "正式评估入口只消费真实已提交消息及原始 Goal 条件，受控评估满足条件后产生对应真实领域结算",
        "重复执行已完成的回复与评估不会重复生成消息、评估、目标结算或模型请求",
        "相关引用校验、回滚、fact_only 开关和 Goal 原有回归通过；不宣称真实模型或整个 M2 完成"
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
# M2-09 direct 主链 Goal 候选的实际下游结算
