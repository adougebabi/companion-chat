---
{
  "original_request": "用户要求任务外、非阻断性的 bug 先建任务而不在当前任务修改。T03 来源追踪验证中，TestGoalAssessmentMemoSkipsDurablyAndOwnerForceOrNewProofReassesses 在首次评估后预期模型调用一次，实际调用为 0。单独运行复现；使用 HEAD 原始 goal_evaluation_task.go 和原始测试夹具在本机隔离 PostgreSQL 复跑仍为 0。尚未定位根因，也未证明其与 goal-planner-original-evidence-chain-regression 同因或真实生产必然受影响。原始脱敏输出见 docs/verification/goal-assessment-first-run-baseline-20261111.txt。",
  "target": "后续领域回归中定位首次 Goal 评估被跳过的原因，按原契约修复或证明测试预期与已批准行为不符。",
  "id": "goal-assessment-first-run-not-invoked",
  "title": "Bug：Goal 评估首次用例未触发模型",
  "type": "bug",
  "source": "user",
  "scope": [
    "首次 Goal 评估的请求排队、来源选择、跳过判定与原有集成测试"
  ],
  "non_goals": [
    "T03 不追修该任务外失败",
    "不伪造模型调用、证据或完成状态",
    "不连接生产数据库或提交图片"
  ],
  "acceptance": [
    "定位首次模型调用为零的直接原因，并保留原始失败日志",
    "在隔离 PostgreSQL 回归首次评估、重复来源跳过、Owner 强制和新证据重评",
    "不降低原 Goal 成功条件或伪造模型结果"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "meromero-t10-domain-regression",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "goal-assessment-first-run-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "定位首次模型调用为零的直接原因，并保留原始失败日志",
        "在隔离 PostgreSQL 回归首次评估、重复来源跳过、Owner 强制和新证据重评",
        "不降低原 Goal 成功条件或伪造模型结果"
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
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# Bug：Goal 评估首次用例未触发模型
