---
{
  "original_request": "2026-11-12 在 M2 图片提交保护变更和干净基线 b3f5cd40、相同一次性 PostgreSQL 与受控 HTTP 服务环境运行 TestGeneratedAvatarRealMediaHTTPWaitsForManualReview，均因 media_model_snapshot_failure: media_model_snapshot_invalid 失败。按用户要求非阻断问题先登记，不在本项顺手修改。",
  "target": "单独核实头像人工审核场景中模型快照失败的来源并恢复相应验收。",
  "id": "avatar-manual-review-snapshot-baseline-failure",
  "title": "头像人工审核回归的旧模型快照失败",
  "type": "bug",
  "source": "verification",
  "scope": [
    "头像人工审核真实 HTTP 夹具与模型快照契约"
  ],
  "non_goals": [
    "不把受控 HTTP 服务称为真实 ComfyUI 或真实模型",
    "不影响当前 M2-05 提交歧义修复"
  ],
  "acceptance": [
    "使用相同隔离环境复现并确认是测试夹具还是生产缺陷",
    "在保留模型快照和人工审核语义下修复并复跑相关测试"
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
        "使用相同隔离环境复现并确认是测试夹具还是生产缺陷",
        "在保留模型快照和人工审核语义下修复并复跑相关测试"
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
# 头像人工审核回归的旧模型快照失败
