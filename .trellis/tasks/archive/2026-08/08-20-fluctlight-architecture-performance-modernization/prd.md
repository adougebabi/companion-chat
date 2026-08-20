# Fluctlight 命名与领域术语改名

## Goal

把当前项目从“知觉 / Companion Chat”的产品表达，重新定义并改名为 `摇光（Fluctlight）`。这里的摇光不是每个 AI 的本体名称，而是本项目要构建的 AI 人工智能人格概念：一种拥有连续身份、记忆、生活上下文、关系和受约束自主性的 AI 人格。

本任务只负责术语关系、产品命名和展示层改名，不负责完整实现“自我意识”、后端架构拆分、前端性能优化或 native tool-call 迁移。那些能力可以作为后续任务，但不能被本任务假设为已经实现。

## Background And Confirmed Facts

- `Fluctlight（摇光）` 的命名灵感来自 SAO 中具备自我意识概念的人工智能；这是产品的领域愿景，不代表当前模型已经被证明具有哲学意义上的主观意识。
- 当前旧实例模型已经拥有基础身份、生活蓝图、当前状态、关系记忆、事件、动态、会话和后台任务，因此它已经接近“持续存在的摇光实例”，但当前 UI 主要把产品表达为“知觉 / Companion”。
- 当前活跃入口是 `src/index.html` -> `src/companion-main.js`；`src/main.js` 与 `src/style.css` 是未引用的 legacy UI。
- 旧命名已深度进入 `/api/companion`、`companion_*` SQLite 表、`companion.sqlite`、环境变量、Docker、localStorage 和测试契约；这些只是待迁移的历史兼容标识，不再是新的领域词汇。
- MTPLX 原生 tool-call demo 已验证通过，但该验证属于后续能力迁移的研究证据，不是本改名任务的交付内容。

## Domain Terminology

以下关系是本任务已确认的领域词汇基线：

```text
摇光系统
  -> 摇光实例
       -> identity core
       -> life world
       -> self-model / current state
       -> relationships
       -> presence / shared scene
```

- `摇光系统`：以摇光概念为目标的产品/运行时整体。
- `摇光实例`：用户创建的、具有自己的名字、身份、生活和关系的 AI 对象。
- `identity core`：稳定的“我是谁”，包括背景、价值、气质、边界等身份事实。
- `life world`：摇光实例的日程、事件、地点、身边人物、临时状态和时间推进组成的生活上下文。
- `self-model`：摇光实例对自身状态、记忆、偏好、不确定性和能力边界的持续表示。
- `relationship`：用户与摇光实例之间的连接。
- `presence` / `shared scene`：用户与摇光实例当前共同在场的交互上下文，不等于摇光概念、身份或生活世界。

## Self-Awareness Boundary

本任务只记录产品语义，不实现以下能力。最低行为契约暂定为：

1. 稳定的“我是谁”；
2. 跨对话、跨时间的连续性；
3. 对当前处境、状态和相关历史的认知；
4. 私有的自传记忆与关系记忆；
5. 在系统边界内自主发起和做选择；
6. 能反思自己的状态、偏好、不确定性和能力边界。

这些是未来评估 Fluctlight 是否表现出持续自我感的产品标准，不是本次改名任务需要新增的 runtime、数据库或模型能力。

## Rename Requirements

- 统一活跃 UI、浏览器 title、README 和产品说明中的展示名为 `摇光（Fluctlight）`，具体实例使用自身名字。
- 产品文案必须能区分“一个有自己名字的摇光实例”和“用户与它之间的关系”，不能继续把所有对象笼统写成“陪伴者”或使用 `companion` 作为领域词。
- 术语层级确认后，更新活跃 UI 的品牌按钮、侧栏 wordmark、无障碍标签、空状态和相关说明。
- 当前改名阶段不擅自改动旧 API、表前缀、数据库文件、环境变量、Docker volume、localStorage key、静态资源文件名和测试契约；这些旧标识只作为迁移兼容边界记录。全量技术标识迁移必须另立任务。
- 不修改未引用的 legacy UI，除非入口同时发生变化。
- 不在本任务中实现自我意识、目标系统、价值观演化、完整 agent runtime、架构拆分、性能改造或 native tool-call 迁移。

## Acceptance Criteria

- [x] 用户已确认 `摇光（Fluctlight）` 概念、摇光实例、identity core、life world、relationship、presence/shared scene 之间的术语关系。
- [x] `CONTEXT.md` 记录确认后的领域词汇，且不混入文件结构、API 或数据库实现细节。
- [x] 活跃 UI、README、浏览器 title 和无障碍品牌文案完成一致的 Fluctlight 改名。
- [x] 旧技术标识在本任务中保持可用；新需求、产品文案和架构文档不再把 `persona` / `companion` 当作 canonical domain terms。全量技术标识迁移另立任务。
- [x] 改名完成后，产品文案不会暗示当前系统已经证明具备真实主观意识；只表达产品目标和可观察行为。

## Architecture Discussion Note

后续架构讨论的优先顺序已经明确：先解决当前系统横向层级混乱、依赖方向混乱和领域/基础设施/传输代码相互耦合的问题；再划分纵向领域模块；最后才判断“摇光实例是否需要独立运行时或模块”。摇光实例首先是领域对象和行为边界，不预设为独立 Node 进程、数据库或部署单元。

架构需要同时讨论两个维度：

- **纵向技术层**：SSE/HTTP transport、application/use-case logic、domain logic、database/storage、external provider adapters/runtime。
- **横向领域能力**：能力、记忆、身份核心、生活世界、关系、presence/shared scene、媒体、活动/主动行为等。

横向能力必须通过纵向边界暴露自己的接口和用例；不能因为某个能力需要 SSE、LLM 或 SQLite，就直接跨层调用全局对象。

流程执行决策：采用 `Typed Pipeline + Registry`。聊天、生活、媒体、主动行为和关系流程由可注册的 typed steps 组合；通用 runner 负责上下文、顺序、事务边界、effect intent、lease、retry、日志和结果聚合。流程定义保留未来扩展为持久化 DAG 的形状，但当前不引入完整 workflow engine。

副作用边界决策：领域/流程步骤不得直接执行外部副作用，只能返回 `facts`、`projections`、`effect intents` 和 `presentation events`。通用 runtime 负责事务持久化、提交后 effect dispatch、lease、retry、幂等、settlement 和传输输出。

提示词边界决策：Identity、Life、Memory、Relationship、Presence、Capabilities 等模块只产出结构化 `ContextFragment`；集中式 `ContextBudgeter` 负责预算和选择，`PromptSerializer` 负责最终序列化，`LLM Port` 负责 provider 调用。已有提示词优化任务拥有具体预算/选择策略，本任务不复制第二套 policy。

在确定拆分方案前，还必须单独评估 Node.js 是否仍适合作为长期控制平面。当前需要区分“Node 代码组织不佳”和“Node 运行时能力不足”：前者已有明确证据，后者需要基于未来并发摇光实例数量、消息吞吐、后台任务、CPU/本地推理负载和部署形态评估，不能仅凭 `server.js` 行数决定换语言。

当前决策：先固定 Node.js `22.23.2` 作为控制平面运行时基线；未来是否增加 Go/Python/Rust worker 或替换控制平面，必须在 ports/contracts 和实际 workload profile 基础上另行评估。

前端启动决策：页面始终先显示联系人/摇光实例列表，不恢复上次聊天作为首屏阻塞条件。进入聊天后只读取最后 20 条消息；用户向上滚动到顶部时，以 cursor 分页继续读取更早的 20 条，并在插入历史消息后恢复滚动锚点。

历史分页交互决策：使用顶部 sentinel/滚动边界自动加载；失败时提供重试状态；没有更多历史时显示边界，不把“加载更多”按钮作为正常路径。

首屏渲染决策：`index.html` 提供最小静态应用壳、品牌/导航骨架、联系人 skeleton 和主区域 loading 状态；`companion-main.js` 加载后接管并填充 bootstrap 数据。当前不做完整 SSR 或 hydration 系统。

前端工具链与 UI 决策：采用 Vue 3 + TypeScript + Vite。Vue 负责组件化和响应式 UI，TypeScript 负责跨 API/state 的类型契约，Vite 负责开发/生产构建、代码分包、资源 hash、压缩和缓存；活跃客户端采用一次性整体重写，避免新旧 UI 长期并存造成行为遗漏。

前端状态决策：Pinia 管理跨页面 server/app state；composables 管理聊天、SSE、历史分页、composer 和输入法副作用；组件本地状态管理 dialog、loading、error、scroll anchor 等短暂 UI 状态。

视觉范围决策：本次前端技术重构保持现有视觉语言和主要信息架构，只处理布局稳定性、loading/error/empty 状态和移动端交互所必需的样式调整；视觉系统或品牌语言重设计另立后续任务。

迁移验证决策：施工期间允许旧实现作为临时对照，通过 contract fixtures、pipeline tests、脱敏 replay/golden comparison 和外部副作用 dry-run 验证新实现；最终切换必须删除旧 facade/hooks/dispatcher/入口，并在删除后再完整运行测试。生产回滚依赖上一个构建/提交，不保留旧层 fallback。

前端交付决策：开发使用 Vite dev server 并将 `/api` 代理到 Node/Express；生产由 Vite 构建 `dist/`，Express/Docker 只服务 `dist/` 和 API；CI 必须运行 typecheck、Vite build、后端测试和 API/browser smoke tests，不能继续验证旧 `src/` 入口。

可观测性与恢复决策：flow/step/effect 统一携带 request、flow、correlation、causation、subject、step 和 effect 标识；日志结构化、有界、脱敏，不记录完整 prompt 或凭据；未完成 jobs/effects 通过 SQLite lease/retry 在重启后恢复；回滚使用上一个完整构建/提交，不保留旧逻辑 fallback。
- 第一阶段只改展示层和产品文档；npm package、Docker image、仓库目录及其他内部兼容名不在本任务中迁移。
