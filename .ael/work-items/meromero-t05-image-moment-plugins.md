---
{
  "original_request": "原始主任务 meromero-first-humanization-plugin-governance 中的 T05，必须保持原始目标与边界。用户要求创建 12 个子任务并逐个处理。\n\n# 8. T05：恢复生图、动态的完整领域插件\n\n## 8.1 插件不是 Tool 的别名\n\n插件可以拥有：\n- Agent/Task；\n- Tool；\n- 配置；\n- Prompt/Context 声明；\n- 事件处理；\n- 持久任务；\n- 领域校验；\n- 必要的结果呈现注册。\n\n优先扩展现有 Capability/注册机制。\n不要创建一个并行的“新插件平台”绕开旧正式入口。\n\n所有 Tool 必须能脱离 Agent，通过同一正式实现独立调用和测试。\n权限、前置条件、幂等和领域校验在独立调用时也必须生效。\n\n## 8.2 最低生命周期能力\n\n实现或补齐：\n- 稳定 ID、版本；\n- 配置声明；\n- 必需/可选依赖；\n- 注册和初始化；\n- 启用/停用；\n- 状态查询；\n- 错误报告；\n- 工具与 Agent 发现；\n- 独立测试入口。\n\n停用策略必须明确：\n- 不删除已有数据；\n- 新请求不再获得已停用能力；\n- 在途任务是继续收尾、暂停还是取消；\n- 不重复提交或交付；\n- UI 不显示虚假可用状态。\n\n## 8.3 Image Plugin\n\n将现有生图领域逻辑归入清晰边界：\n- 角色拍摄意图；\n- 媒体主体与视角处理；\n- 冻结相关事实；\n- 提示词规范化；\n- 实际渲染请求；\n- 异步状态与恢复；\n- 质量判断；\n- 资源结果；\n- 与统一交付机制衔接。\n\n保留现有 ComfyUI/渲染适配、视觉身份及衣柜权威来源。\n\n媒体链必须明确：\n- 无人物画面不注入角色外貌与穿着；\n- 有人物时使用该入镜主体资料；\n- 自拍、他拍、镜前、设备是否入镜不能混淆；\n- 未知性别/外貌不能根据名字猜；\n- 当前画面不能为了“有氛围”擅自改变天气、时间、身份和穿着；\n- 创作性的场景设计与“当前真实生活照片”必须区分。\n\n保留现有 KEV 开关与接口。\n逐点核查 KEV 是推荐、筛选还是现有准入规则，不新增重复决策层。\n关闭/失败时沿明确的既有回退路径，不恢复固定自拍模板，也不放松 Core 权限。\n\n## 8.4 Moment Plugin\n\n覆盖现有：\n- 动态文案；\n- 可选媒体；\n- 发布；\n- 查询；\n- 已有评论/互动；\n- 状态和持久化。\n\n角色文案默认 MeroMero。\n不能把动态变成“直接发布一段随便拼的文本”。\n\n文字动态不能硬依赖生图插件。\n附图动态应明确对图片能力的可选依赖及失败处理。\n关闭动态插件不影响普通聊天和生活运行。\n\n## 8.5 Core 与插件边界\n\nCore/共享基础设施继续提供：\n- 身份与权限；\n- 事务；\n- 持久任务；\n- 事件；\n- 幂等；\n- 资源引用；\n- 统一发布/交付。\n\n领域规则放到对应插件或领域服务。\n不得把一致性全部挪进 Prompt，也不得让插件绕开 Core 直接改状态和发送消息。\n\n验收：\n- Image 与 Moment 都是实际可调用的完整能力，不是空壳注册；\n- 关闭 Image，文字聊天与纯文字动态仍可运行；\n- 关闭 Moment，其他能力仍正常；\n- Tool 独立调用和 Agent 调用经过同一实现与校验。\n\n---\n\n完整共同设计原则和最终自检见父任务 original_request。",
  "target": "完成原始 T05：Image 与 Moment 领域插件，保留已实现部分并补足未完成要求及对应证据。",
  "id": "meromero-t05-image-moment-plugins",
  "title": "T05 Image 与 Moment 领域插件",
  "type": "feature",
  "source": "user",
  "scope": [
    "# 8. T05：恢复生图、动态的完整领域插件",
    "原始 T05 的全部实施与验收要求，详见 original_request 与父任务"
  ],
  "non_goals": [
    "不修改或删除真实业务数据，不连接生产环境执行写操作",
    "不提交图片，不跳过 Git 提交钩子",
    "不替换现有技术栈、引入第二套运行时或凭空降低原始验收条件",
    "不新增或调整 prompt/response token 上限，不伪造外部服务验证"
  ],
  "acceptance": [
    "Image 与 Moment 复用现有 Capability 及领域服务，具备配置、依赖、注册、发现、启停、健康和错误报告",
    "各 Tool 独立调用与 Agent 调用使用同一权限、前置条件、幂等和领域校验",
    "停用插件不删除数据；明确在途恢复策略；文字动态不依赖生图",
    "核验主体、视角、衣柜来源、渲染、审核、交付和动态发布的真实链路"
  ],
  "priority": 5,
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
      "id": "t05-review",
      "kind": "review",
      "command": null,
      "covers": [
        "Image 与 Moment 复用现有 Capability 及领域服务，具备配置、依赖、注册、发现、启停、健康和错误报告",
        "各 Tool 独立调用与 Agent 调用使用同一权限、前置条件、幂等和领域校验",
        "停用插件不删除数据；明确在途恢复策略；文字动态不依赖生图",
        "核验主体、视角、衣柜来源、渲染、审核、交付和动态发布的真实链路"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "t05-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "Image 与 Moment 复用现有 Capability 及领域服务，具备配置、依赖、注册、发现、启停、健康和错误报告",
        "各 Tool 独立调用与 Agent 调用使用同一权限、前置条件、幂等和领域校验",
        "停用插件不删除数据；明确在途恢复策略；文字动态不依赖生图",
        "核验主体、视角、衣柜来源、渲染、审核、交付和动态发布的真实链路"
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
# T05 Image 与 Moment 领域插件
