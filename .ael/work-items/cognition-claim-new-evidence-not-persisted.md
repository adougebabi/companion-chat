---
{
  "original_request": "用户要求任务外非阻断 bug 先建任务，不在当前任务顺手调整。T04 核对认知结算时静态发现：intelligence.go 的 evaluateClaims 对已有 repetition_key 且证据不同的 claim 继续归入 approved/uncertain；persistClaimsTx 随后使用 ON CONFLICT(fluctlight_id,repetition_key) DO NOTHING，因此新证据可能未写入 cognition_claims。当前是源码级发现，尚未验证实际生产数据受影响，也未确定应合并、版本化还是另建来源记录。",
  "target": "核实并修复重复 claim 获得新证据时的持久化与后续读取语义，保留原证据和历史。",
  "id": "cognition-claim-new-evidence-not-persisted",
  "title": "Bug：重复 claim 的新证据可能未持久化",
  "type": "bug",
  "source": "user",
  "scope": [
    "claim 去重、证据持久化、后续 hypothesis/记忆读取及重放"
  ],
  "non_goals": [
    "当前 T04 不改通用 claim 持久化",
    "不删除或批量重写真实业务数据",
    "不提交图片"
  ],
  "acceptance": [
    "在隔离数据库复现重复 claim 携带不同证据的现状并确定原契约",
    "修复后旧证据与新证据均可追溯，重放不重复且不同事实不错误合并",
    "相关 claim、认知结算和记忆读取回归通过"
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
      "id": "claim-new-evidence-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "在隔离数据库复现重复 claim 携带不同证据的现状并确定原契约",
        "修复后旧证据与新证据均可追溯，重放不重复且不同事实不错误合并",
        "相关 claim、认知结算和记忆读取回归通过"
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
# Bug：重复 claim 的新证据可能未持久化
