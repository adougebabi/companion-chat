---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T01，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 4. T01：完整盘点真实运行入口\n\n查找本地的 llm-agent-prompts.md 及相关文档，但以当前源码为准。\n文档不存在时直接从源码盘点，不等待用户补文件。\n\n盘点所有：\n- FormalAgent；\n- 共享 Agent ID 下的任务变体；\n- 非 FormalAgent 模型请求；\n- 修复、重试、恢复请求；\n- 插件内部模型调用；\n- 直接调用模型客户端的旁路；\n- 后台调度入口；\n- 用户可见内容生成入口；\n- Native Tool 注册与独立调用入口。\n\n历史清单约有以下 30 个 Agent，需逐项核对，但实际数量以源码为准：\n\nconversation_cognition\nconversation_reply\nreply_action_check\nwake_up\ntakeover_judge\ntakeover_reply\ninitialization\npersona_compilation\npersistent_switch\nwardrobe_preparation\nmedia_prompt\nmedia_quality\nvisual_identity_vision\nvisual_identity_patch\nvisual_identity\nconversation_summary\nschedule_generation\nnative_cognition\ndaily_review\ndiary_material\ndiary_expression\nreflection\nschedule_replan\ngoal_planner\ngoal_evaluation\nvirtual_activity_result\nstudio_assistant\nstudio_text_preview\nnpc_response\nworld_event_generation\n\n每个入口记录：\n- Agent ID / variant；\n- 实际 caller；\n- 是否生产使用；\n- 所属领域/插件；\n- 单次 Task 或 Agent Loop；\n- 当前模型来源；\n- Prompt 来源和版本；\n- 输入 Slot；\n- 工具集合；\n- 输出与校验；\n- 副作用；\n- 发布渠道；\n- 本轮处理方式：保留、迁移、修复、移除或待核实。\n\n重要：\n- 不把已注册但无生产 caller 的入口当成正常运行。\n- 不擅自激活 takeover、持久切换或视觉分支。\n- 不为了统一清单新增实际不需要的 Agent。\n- 全量盘点不等于全量重写。\n\n验收：\n任何正式 LLM 请求都能找到任务归属、模型配置来源和执行入口。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T01：运行入口完整盘点，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t01-agent-inventory",
  "title": "T01 运行入口完整盘点",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 4. T01：完整盘点真实运行入口",
    "原始 T01 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "覆盖所有正式 Agent 及其真实任务变体，并区分生产上游、已注册但未启用的分支",
    "覆盖非 FormalAgent、插件内部、模型客户端旁路、后台调度、修复重试恢复和独立 Tool 调用入口",
    "每个入口记录 caller、模型配置来源、Prompt 来源与版本、输入 Slot、工具、输出校验、副作用和发布渠道，并给出当前源码定位",
    "以源码及实际绑定为依据核对清单，标明不确定项和未验证项，不把注册等同于生产使用"
  ],
  "priority": 1,
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
      "id": "t01-review",
      "kind": "review",
      "command": null,
      "covers": [
        "覆盖所有正式 Agent 及其真实任务变体，并区分生产上游、已注册但未启用的分支",
        "覆盖非 FormalAgent、插件内部、模型客户端旁路、后台调度、修复重试恢复和独立 Tool 调用入口",
        "每个入口记录 caller、模型配置来源、Prompt 来源与版本、输入 Slot、工具、输出校验、副作用和发布渠道，并给出当前源码定位",
        "以源码及实际绑定为依据核对清单，标明不确定项和未验证项，不把注册等同于生产使用"
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
# T01 运行入口完整盘点
