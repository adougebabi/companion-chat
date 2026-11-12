---
{
  "original_request": "原 T05 要求 Moment 插件保留已有评论/互动、状态和持久化，并明确停用后的新任务与在途任务策略。现有 NPC 动态互动路径缺少针对性隔离集成验收。",
  "target": "",
  "id": "meromero-m2-13-moment-npc-comment-recovery",
  "title": "M2-13 动态 NPC 评论与停用恢复",
  "type": "feature",
  "source": "user",
  "scope": [
    "已确认的 NPC 结识、正式动态发布、NPC 评论及持久事实的现有生产调用路径",
    "Moment 停用时未产生决定和已持久化决定的处理与重放"
  ],
  "non_goals": [
    "不新增 NPC 业务、评论工具或第二套插件平台",
    "不改变模型设置、提示词、采样、token 限额或插件停用政策",
    "不连接生产数据库、真实用户数据或真实模型；不伪称受控 Provider 为真实模型",
    "不修改四处原有草稿或任务外非阻断问题"
  ],
  "acceptance": [
    "已有可核对结识的 NPC 对正式动态产生评论时，正式任务持久化唯一评论和事件，重放不重复生成或提交",
    "停用 Moment 后，尚未持久化决定的新互动不调用模型或发布评论",
    "停用前已持久化的有效互动决定可按既有策略完成，不重新调用模型且不重复评论",
    "隔离 PostgreSQL 与受控 Provider 实际执行相关场景和直接回归，保留命令、版本、无 SKIP 结果及限制"
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
      "id": "m2-13-npc-comment-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "已有可核对结识的 NPC 对正式动态产生评论时，正式任务持久化唯一评论和事件，重放不重复生成或提交",
        "停用 Moment 后，尚未持久化决定的新互动不调用模型或发布评论",
        "停用前已持久化的有效互动决定可按既有策略完成，不重新调用模型且不重复评论",
        "隔离 PostgreSQL 与受控 Provider 实际执行相关场景和直接回归，保留命令、版本、无 SKIP 结果及限制"
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
# M2-13 动态 NPC 评论与停用恢复
