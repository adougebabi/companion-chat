---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T11，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 14. T11：必要管理入口、Trace 与重放\n\n沿现有新 UI 设计补功能，不重新设计全项目页面。\n不恢复旧 UI，不改默认主题和已确认布局。\n\n## 14.1 管理入口\n\n至少覆盖：\n- Agent/variant 有效模型与继承来源；\n- 插件启停、依赖、健康状态；\n- Prompt/Context 实际预览；\n- 表达候选及审核/启用/停用；\n- 记忆来源与纠错信息；\n- 请求 Trace 和执行状态。\n\n每个入口形成：\n页面 → API → 领域处理 → 持久化 → 回读 → 真实运行消费。\n\n不能只做前端 Mock。\n刷新页面之后配置应保留，下一次请求应能证明实际生效。\n\n## 14.2 Trace\n\n复用或统一现有 run_id，关联：\n- 触发来源；\n- actor/world/conversation；\n- Agent ID/variant；\n- 插件与版本；\n- 实际模型与配置版本；\n- Prompt 版本；\n- 上下文 Slot 和来源；\n- ToolCall/ToolResult；\n- 异步事件；\n- 状态 revision；\n- 候选正文；\n- 检查、修复和最终发布；\n- 耗时、错误与可获得的 token 指标。\n\n不要求隐藏推理。\n不要把 Trace 内容反过来全部注入聊天模型。\n\n保留失败前工具证据。\n不得只留最后一次成功结果掩盖失败过程。\n\n## 14.3 敏感信息\n\n凭据必须脱敏。\n完整 Prompt/历史记录按开发调试权限控制，不能无边界落生产日志。\n导出报告需去除真实用户隐私和访问凭据。\n\n## 14.4 重放\n\n默认只读或隔离环境。\n不能因重放再次购买、换衣、发消息或生图。\n需要真实副作用的集成测试应有显式测试环境与独立数据。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T11：管理、Trace 与安全重放，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t11-management-trace",
  "title": "T11 管理、Trace 与安全重放",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 14. T11：必要管理入口、Trace 与重放",
    "原始 T11 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "管理入口覆盖模型继承、插件状态与依赖、表达审核、记忆纠错和请求诊断",
    "配置通过页面、API、领域持久化、回读并影响下一次实际运行",
    "Trace 关联模型版本、来源 Slot、工具回执、异步状态、候选与发布且保护凭据及隐私",
    "只读或隔离重放不重复产生真实副作用，覆盖插件真实健康状态"
  ],
  "priority": 11,
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
      "id": "t11-review",
      "kind": "review",
      "command": null,
      "covers": [
        "管理入口覆盖模型继承、插件状态与依赖、表达审核、记忆纠错和请求诊断",
        "配置通过页面、API、领域持久化、回读并影响下一次实际运行",
        "Trace 关联模型版本、来源 Slot、工具回执、异步状态、候选与发布且保护凭据及隐私",
        "只读或隔离重放不重复产生真实副作用，覆盖插件真实健康状态"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t11-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "管理入口覆盖模型继承、插件状态与依赖、表达审核、记忆纠错和请求诊断",
        "配置通过页面、API、领域持久化、回读并影响下一次实际运行",
        "Trace 关联模型版本、来源 Slot、工具回执、异步状态、候选与发布且保护凭据及隐私",
        "只读或隔离重放不重复产生真实副作用，覆盖插件真实健康状态"
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
# T11 管理、Trace 与安全重放
