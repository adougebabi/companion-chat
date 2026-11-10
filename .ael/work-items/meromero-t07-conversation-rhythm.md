---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T07，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 10. T07：拟人发言节奏与主动行为\n\n不只优化文风，也优化何时说、为什么说。\n\n## 10.1 消息接收\n\n复用现有收件、superseded、取消和会话机制，检查并完善：\n- 连续短消息合并；\n- 新消息到达后的旧候选处理；\n- 避免逐条重复回答；\n- 避免重放历史最后一句作为当前请求。\n\n不得靠随机长延迟假装真人。\n不得破坏现有流式反馈。\n\n## 10.2 WakeUp\n\n复用现有调度，不新增第二套周期时钟。\n\n区分：\n- 后台生活执行机会；\n- 主动社交机会；\n- 需要发动态的机会；\n- 可以静默的情况。\n\n触发输入应带真实来源：\n- 新事件；\n- 到期任务；\n- 已接受计划的执行义务；\n- 未完成交流；\n- 当前兴趣或长期目标的合理机会；\n- 是否存在新用户消息。\n\n后台任务到期不等于必须通知用户。\n有新分享理由也不应被“用户没发新消息”永久压制。\n\n保留：\n- 安静时段；\n- 自治权限；\n- 明确拒绝；\n- 重复联系抑制；\n- 主题冷却；\n- 无事可做时的退避；\n- 原 Goal 和日程义务。\n\n主动消息由 MeroMero 根据事实和自身人格决定。\n内部任务只能提供触发事实，不能提前规定“你应该撒娇/安慰/强推关系”。\n\n验收：\n既有应静默场景，也有真实新事件的正向场景。\n不将“全部不发”当作主动交流能力通过。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T07：拟人交流节奏与主动行为，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t07-conversation-rhythm",
  "title": "T07 拟人交流节奏与主动行为",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 10. T07：拟人发言节奏与主动行为",
    "原始 T07 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "连续短消息和追发基于真实受理、superseded 与会话机制合并处理，旧候选不得重复或覆盖新消息",
    "WakeUp 区分新事件、未回应交流、执行义务与静默机会，保留安静时段、冷却及自治权限",
    "真实理由正向主动与无理由静默均可成立，不用随机延迟或强迫人格达成",
    "隔离并发验证已执行动作保留及最新轮上下文；主观交流效果可由用户验收"
  ],
  "priority": 7,
  "dependencies": null,
  "parent": "meromero-first-humanization-plugin-governance",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "model",
      "id": "t07-review",
      "kind": "review",
      "command": null,
      "covers": [
        "连续短消息和追发基于真实受理、superseded 与会话机制合并处理，旧候选不得重复或覆盖新消息",
        "WakeUp 区分新事件、未回应交流、执行义务与静默机会，保留安静时段、冷却及自治权限",
        "真实理由正向主动与无理由静默均可成立，不用随机延迟或强迫人格达成",
        "隔离并发验证已执行动作保留及最新轮上下文；主观交流效果可由用户验收"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t07-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "连续短消息和追发基于真实受理、superseded 与会话机制合并处理，旧候选不得重复或覆盖新消息",
        "WakeUp 区分新事件、未回应交流、执行义务与静默机会，保留安静时段、冷却及自治权限",
        "真实理由正向主动与无理由静默均可成立，不用随机延迟或强迫人格达成",
        "隔离并发验证已执行动作保留及最新轮上下文；主观交流效果可由用户验收"
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
# T07 拟人交流节奏与主动行为
