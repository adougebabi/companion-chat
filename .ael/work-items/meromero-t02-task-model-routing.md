---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T02，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 5. T02：统一按任务选择模型\n\n## 5.1 路由粒度\n\n模型选择至少支持：\n- 默认文本模型；\n- Agent 级覆盖；\n- 任务变体级覆盖。\n\n基于当前已有配置机制实现明确的优先级，不增加多套相互竞争的配置源。\n\n配置应包含或引用现有：\n- Provider；\n- Model ID；\n- 端点与凭据引用；\n- 采样参数；\n- 输出长度；\n- 超时；\n- 能力需求，例如 Tools、结构化输出、图片输入。\n\nModel ID 不得写死在业务逻辑。\n不要因为本轮样本使用某个量化仓库就强制整个项目依赖该仓库名。\n\n## 5.2 实际生效\n\n保存配置后必须能证明：\n- 新请求使用新配置；\n- 其他任务没有被连带改变；\n- 修复/续接/后台入口没有漏掉；\n- 每次请求记录有效配置与版本。\n\n进行中的 run、异步 draft 和恢复任务，采用明确的配置快照策略：\n- 默认不因全局配置临时变化而静默换模型；\n- 若已有不同策略，保留并记录其一致性保证；\n- 不新增隐式模型切换。\n\n## 5.3 能力校验\n\n不能把“接口接受 response_format”视为已证明严格约束解码。\n仍须服务端验证结构和业务一致性。\n\nTools 与 response_format 的组合按实际 Provider 能力处理：\n- 不一刀切要求所有工具轮同时输出最终 JSON；\n- 不用解析普通正文中的伪 tool_calls 替代真实 ToolCall；\n- 不支持的能力应明确报错或使用显式配置的合适任务模型。\n\n真实视觉任务继续使用已经可用的视觉配置，直到替代模型通过真实图片输入验收。\n不能用文字问答测试替代视觉能力验证。\n\n## 5.4 界面与凭据\n\n后续管理入口必须显示：\n- 当前继承来源；\n- 最终生效模型；\n- 覆盖配置；\n- 兼容性或连接错误。\n\n凭据只保留服务端引用，不返回浏览器、不写入普通 Trace。\n\n验收：\n- Qwen 未配置时，已迁移文本主链能够运行；\n- 某个内部任务改为 Qwen，只影响该任务；\n- 角色拒绝不会触发换模型重试；\n- 视觉能力未验证时不会被默认文本配置误接管。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T02：按任务选择模型，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t02-task-model-routing",
  "title": "T02 按任务选择模型",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 5. T02：统一按任务选择模型",
    "原始 T02 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "默认文本模型、Agent 和任务变体覆盖具有唯一明确优先级，视觉等独立能力保持隔离",
    "新请求、修复、续接和后台异步恢复消费正确配置；进行中的任务按明确快照策略运行",
    "配置界面展示继承与生效来源，凭据不返回客户端或普通 Trace",
    "隔离验证配置持久化与任务隔离，真实 Provider 的生效按原要求留证或明确环境阻塞"
  ],
  "priority": 2,
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
      "id": "t02-review",
      "kind": "review",
      "command": null,
      "covers": [
        "默认文本模型、Agent 和任务变体覆盖具有唯一明确优先级，视觉等独立能力保持隔离",
        "新请求、修复、续接和后台异步恢复消费正确配置；进行中的任务按明确快照策略运行",
        "配置界面展示继承与生效来源，凭据不返回客户端或普通 Trace",
        "隔离验证配置持久化与任务隔离，真实 Provider 的生效按原要求留证或明确环境阻塞"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t02-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "默认文本模型、Agent 和任务变体覆盖具有唯一明确优先级，视觉等独立能力保持隔离",
        "新请求、修复、续接和后台异步恢复消费正确配置；进行中的任务按明确快照策略运行",
        "配置界面展示继承与生效来源，凭据不返回客户端或普通 Trace",
        "隔离验证配置持久化与任务隔离，真实 Provider 的生效按原要求留证或明确环境阻塞"
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
# T02 按任务选择模型
