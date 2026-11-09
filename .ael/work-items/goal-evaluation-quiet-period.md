---
{
  "target": "自动目标进展评估从最近一次相关活动结束时开始等待 30 分钟；若这段时间有新活动则重新计时，每个静默窗口只评估一次。",
  "id": "goal-evaluation-quiet-period",
  "title": "目标评估按活动后的静默周期触发",
  "type": "feature",
  "source": "user",
  "scope": [
    "apps/core-go 目标评估请求入队、周期修复、准入与来源活动水位",
    "相关数据库迁移与恢复语义"
  ],
  "non_goals": [
    "修改目标评估 Agent 提示词或模型输入/输出预算",
    "改变目标评估判据、手动核对按钮的产品语义、部署数据库或发布服务",
    "新增或运行自动化测试"
  ],
  "acceptance": [
    "相关活动完成后计算 30 分钟静默截止时间；截止前的新活动会重置截止时间",
    "静默截止后只为该活动水位触发一次自动评估；没有新活动时不因后续周期重复入队",
    "已排队或恢复中的旧请求遵循最新静默截止时间，评估期间的新活动不会被旧轮次消费",
    "显式 Owner 核对仍按既有语义执行，不被自动活动静默计时误延后"
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
        "相关活动完成后计算 30 分钟静默截止时间；截止前的新活动会重置截止时间",
        "静默截止后只为该活动水位触发一次自动评估；没有新活动时不因后续周期重复入队",
        "已排队或恢复中的旧请求遵循最新静默截止时间，评估期间的新活动不会被旧轮次消费",
        "显式 Owner 核对仍按既有语义执行，不被自动活动静默计时误延后"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "",
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 目标评估按活动后的静默周期触发
