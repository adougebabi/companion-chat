---
{
  "original_request": "用户要求任务外、非阻断性 bug 先建任务而不在当前任务修改。T03 长目标输入验证时，TestGoalPlannerNativeLoopConsumesQueryCommitAndReread 在提交新 Goal、真实回复和评估后，预期目标 completed 且存在一条 resolution，实际得到 active、resolution 数为 0。使用当前已提交 HEAD 的原始 goal_planner.go 和原始测试夹具，在本机独立临时 PostgreSQL 复跑两次得到同一失败；因此该失败不由尚未提交的长输入变更引起。根因尚未定位，不能据此断言真实生产服务已出现同样故障。原始脱敏输出见 docs/verification/goal-planner-e2e-baseline-20261111.txt。",
  "target": "后续领域回归中定位并解决该用例的目标评估证据链失败，或以可核实证据明确测试夹具与既有契约的偏差。",
  "id": "goal-planner-original-evidence-chain-regression",
  "title": "Bug：Goal Planner 端到端用例未完成原始证据链",
  "type": "bug",
  "source": "user",
  "scope": [
    "Goal Planner 提交、真实对话证据、Goal Evaluation 与完成结算的既有端到端场景"
  ],
  "non_goals": [
    "T03 不追修该任务外问题",
    "不修改原 Goal 成功条件或伪造证据",
    "不连接生产数据库或调用真实模型",
    "不提交图片"
  ],
  "acceptance": [
    "定位 active 且无 resolution 的直接原因，保留原始失败证据",
    "在隔离 PostgreSQL 中真实回归原始工具链，按既定成功条件验证状态和 resolution",
    "修复不通过绕过原业务语义或将失败改写为成功实现"
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
      "id": "goal-original-evidence-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "定位 active 且无 resolution 的直接原因，保留原始失败证据",
        "在隔离 PostgreSQL 中真实回归原始工具链，按既定成功条件验证状态和 resolution",
        "修复不通过绕过原业务语义或将失败改写为成功实现"
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
# Bug：Goal Planner 端到端用例未完成原始证据链
