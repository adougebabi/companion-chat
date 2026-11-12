---
{
  "original_request": "M2-09 隔离 PostgreSQL 回归中，TestGoalAgentCadenceEvaluationPendingFrozenAndProfiles、TestGoalEvaluationGhostStageFailsBeforeDomainCommit、TestGoalSubmissionRecoveryPersistenceHandledCycleAndReplay 失败。干净提交 f74fbc1c、无源码覆盖及同一数据库环境分别复现；前两种报告包含 evaluation_cadence 延期，GhostStage 只报告未出现预期错误，具体原因待核查。证据 docs/verification/meromero-m2-evidence/m2-09-precommit.log 与 m2-09-baseline-goal.log。",
  "target": "独立核查三项既有 Goal 回归与静默期契约的关系，恢复其实际覆盖并验证原有语义。",
  "id": "goal-cadence-additional-baseline-regressions",
  "title": "Goal 静默期相关的三项基线回归未通过",
  "type": "bug",
  "source": "verification",
  "scope": [
    "上述 Goal 节奏、无效引用和恢复测试及必要的对应领域实现"
  ],
  "non_goals": [
    "不在 M2-09 顺手更改既有 Goal 规则或测试",
    "不把等待状态直接视为通过或生产缺陷已确定"
  ],
  "acceptance": [
    "同环境复现并分别确定各失败原因及与已有节奏问题是否同因",
    "依据既有业务契约恢复三项测试的真实覆盖并验证重放与静默期"
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
      "id": "goal-cadence-recovery-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同环境复现并分别确定各失败原因及与已有节奏问题是否同因",
        "依据既有业务契约恢复三项测试的真实覆盖并验证重放与静默期"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "goal-evaluation-cadence-domain-failures",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# Goal 静默期相关的三项基线回归未通过
