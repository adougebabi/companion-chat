---
{
  "original_request": "用户要求任务外非阻断 bug 先建任务，不在当前任务顺手调整。T04 核对交互时发现浏览器入口使用 StreamDurableTurn；该观察器当前仅发送用户和已提交助手消息的 action_result 及 completed/error，不发送模型生成期间的 token 帧。隔离 PostgreSQL + 受控 direct Provider 的 TestDurableObserverReceivesDirectMainReply 实际观察到三帧：用户 action_result、已提交助手 action_result、completed。旧的持久观察器测试也是同样合同；尚未证明前端没有对完整消息做视觉渐显，也未证明该现象由 direct 改动引入。",
  "target": "核实并恢复浏览器持久聊天的实时流式体验，同时保持断线恢复、消息状态、提交校验和幂等。",
  "id": "durable-chat-missing-live-token-stream",
  "title": "Bug：浏览器持久聊天流不发送模型增量",
  "type": "bug",
  "source": "user",
  "scope": [
    "浏览器持久 turn 观察器、Worker 到观察器的增量传递及前端真实消费"
  ],
  "non_goals": [
    "当前 T04 不新增第二套工作流或绕过正式发布",
    "不把未校验文本或附件宣称为已提交",
    "不提交图片或修改真实业务数据"
  ],
  "acceptance": [
    "隔离环境证明 Worker 生成中浏览器可收到真实增量，并区分临时内容与正式提交",
    "断线、取消、失败、校验拒绝及重放不重复发布或伪造完成",
    "保留消息状态、前端交互和原有持久恢复语义"
  ],
  "priority": 2,
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
      "id": "durable-live-stream-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "隔离环境证明 Worker 生成中浏览器可收到真实增量，并区分临时内容与正式提交",
        "断线、取消、失败、校验拒绝及重放不重复发布或伪造完成",
        "保留消息状态、前端交互和原有持久恢复语义"
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
# Bug：浏览器持久聊天流不发送模型增量
