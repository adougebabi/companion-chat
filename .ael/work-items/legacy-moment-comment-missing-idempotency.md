---
{
  "original_request": "用户要求任务外非阻断问题先创建 bug，不在当前任务修改。T05 只读核对发现旧 /api/moments/:momentId/comments 路由只转发 text；CommentMoment 每次生成随机评论 ID 并插入，没有接收或持久校验操作幂等键。网络结果不明后客户端重试可能产生重复评论。新版 moment-feed 评论使用 worldCommand 幂等账本。当前为源码级发现，尚未复现真实用户受影响。",
  "target": "核实旧动态评论的实际调用和兼容要求，使未知结果重试不重复创建评论，同时保留鉴权和插件停用边界。",
  "id": "legacy-moment-comment-missing-idempotency",
  "title": "Bug：旧动态评论接口缺少幂等身份",
  "type": "bug",
  "source": "user",
  "scope": [
    "旧评论路由、Core 写入、操作身份和与现有动态命令账本的兼容"
  ],
  "non_goals": [
    "当前 T05 不修改旧评论接口",
    "不删除已有评论或真实业务数据",
    "不提交图片"
  ],
  "acceptance": [
    "隔离环境复现同一旧评论请求结果不明后的重复风险",
    "修复后相同操作重放只返回原回执，不同正文或身份不能复用",
    "旧调用方兼容、Owner 鉴权、插件停用和新接口回归通过"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "meromero-t06-delivery-consistency",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "legacy-moment-comment-idempotency-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "隔离环境复现同一旧评论请求结果不明后的重复风险",
        "修复后相同操作重放只返回原回执，不同正文或身份不能复用",
        "旧调用方兼容、Owner 鉴权、插件停用和新接口回归通过"
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
# Bug：旧动态评论接口缺少幂等身份
