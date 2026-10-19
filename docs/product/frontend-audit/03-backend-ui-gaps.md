# 03 — 后端能力与前端入口缺口

本文按当前 Go Core 代码与浏览器边界核对可复用能力。这里的“有后端能力”只表示存在可证实的 service/route/tool，不代表有 owner 管理 API；“无 UI”不自动意味着应新增 UI。引用代码路径相对仓库根目录。

## 已形成前后端闭环的能力

| 能力 | 后端真实入口 | 当前 UI 覆盖 | 缺口与判断 |
|---|---|---|---|
| Conversation/Message | Go Core 注册 create/history/stream turn `apps/core-go/internal/httpapi/server.go:129,134-135`；创建 `core/operations.go:71`；turn `core/mutations.go:574`；Browser boundary turn/cancel `httpapi/browser/routes.go:255,290` | `ChatView` 发送/流式结果、取消、历史分页/已读、失败重试；`packages/browser-client/src/index.ts:176-204` | 已具备主要对话闭环。未发现任意修改/删除已提交 message 的用户管理接口。附件上传与引用签发另待核验，不能从 `attachmentRefs` 一字段推断。 |
| Runtime capabilities / Agent execution | 固定 registry `internal/core/builtin_capabilities.go:87,141-168`；catalog `tool_contract.go:122,150` | Diagnostics 展示 Agent/model runs；Governance 审 capability requests | 运行记录和 request review 不等于 Agent 定义编辑、Task 看板或动态插件安装。若只需要 operational inspection，已有 UI；用户定义能力管理 API 未证实。 |
| Actor、Persona、Relationship | detail projection `core/detail.go:25-74`；relationship edit `relationship_edit.go:113,148`；actor groups `operations.go:855,883,894` | 详情资料、关系编辑/回滚、Actor context、实例分组 | 有管理闭环；领域边界依赖 Core Persona、profile runtime、Actor 和有向 Relationship 契约。没有 NPC authoring API 证据。 |
| Goal、Intention、Goal planning | Goal routes `server.go:83-93`；planner `core/goal_planner.go:61,207,335,359,604`；Intention tools `intention_capabilities.go:35,145,291` | GoalPanel、GoalPlanningPanel 可读写、查看 history/evidence | 已相对完整。普通 Desire 是否是独立聚合没有证据；勿重复造一个 Goal/Intention 管理模块。 |
| Schedule/Event/Presence/Activity | Owner routes `server.go:114-117,133`；event create/cancel/presence `operations.go:998,1120,1192`；activity `life_activity_capabilities.go:76,335,494` | 详情上下文/日程；治理 JSON 全 Schedule 提交、Event create/cancel、presence 编辑 | API 可复用，UI 管理形态不完整（JSON 而非逐项编辑）；Scene/Location 是 context 字符串，不是资源 CRUD。 |
| Memory 与 Memory governance | revise/forget `operations.go:614,635`；routes `server.go:131-132`；detail active memory projection `detail.go:355` | 详情摘要、治理修订/忘却 | 可复用；详情最多 100 条且会过滤 pending/invalid provenance，不能把它当完整 memory ledger。 |
| Wardrobe / Item | owner CRUD `wardrobe_capabilities.go:35,587,711,788`；routes `server.go:97-100`；runtime wear/outfit `:279,442` | 治理增改删/可用状态；详情分页/穿着 | CRUD 有入口，但页面增加物品不等于穿戴；wear/borrow/return/outfit/item-use 的 runtime 语义没有统一管理面板。 |
| Visual Identity / Media | media service `media_service.go:58-73`；image/identity tools `tool_contract.go:253,314`；asset route `server.go:136`；prompt diagnostics/retry `:145-146` | 详情候选与状态；Settings workflow 配置；Diagnostics 查看/retry | 真实 pipeline 有入口，但入口分散；没有证据支持从工作流 JSON 编辑页面直接完成身份候选审查全流程。 |
| Model configuration / routing | settings 可见绑定角色 `core/settings.go:340-344`；provider assignment `provider.go:103,113,119-121`；endpoint/model routes `server.go:68-72` | Settings model roles、endpoint、bindings read-only | API/流程可复用。页面把绑定查看与编辑分成两个 section；旧 scenario role 兼容行不应当成独立产品角色。 |
| Diagnostics / workflow control | 路由集合 `server.go:137-155` | lifecycle、Agent/model runs、media prompts、events、KEV、workflow list/status/history/pause/resume/cancel/reset/restart、导出/清空 | 已有较完整 operational UI；需要维护 correlation、cursor 和危险 workflow command 确认约束。 |

## 有后端实现但缺少专门 UI 管理入口

| 能力 | 可复用的后端实现 | 当前入口 | 需要补齐什么才能闭环 |
|---|---|---|---|
| Reflection V2 观察/审查 | Active runtime `reflection_runtime_v2.go:136`；proposal/evaluation/apply `:321-422,478-482`；持久化 reflection window/proposal `migrations/runner.go:360-361` | 间接呈现已结算的 Memory/Goal/Relationship/affect 与 diagnostics；未发现 Reflection 专页/API | 需先确认产品是否需要面向 owner 的 Reflection 时间线/候选审查，再决定新增只读 query 与 action API；当前不应把 Reflection 伪装成 Memory CRUD。 |
| 内置 capability/Tool 清单与定义 | 当前注册表 `builtin_capabilities.go:87,141-168`；catalog `tool_contract.go:122,150` | Capability request review；Agent runtime diagnostics | 如果需要看清运行时可用工具，可评估复用 catalog read。编辑 Agent/Tool 定义或安装代码没有已证实的 owner API，必须新增服务端治理契约且不属于本轮审计。 |
| Item current-use 查看/治理 | Item use tables `migrations/inventory_usage.go:14,24`；runtime borrow/return/use capabilities `builtin_capabilities.go:156` | 当前详情展示 inventory、worn items 和 Schedule；未见普通 object use history/manage surface | 可先核对是否存在 owner-query API；如没有需新增只读/管理接口，不能拿 inventory row 的 ownership 字段代替当前使用关系。 |
| Actor Group 移出成员 | 已有 `BrowserClient.removeActorGroupMember` `packages/browser-client/src/index.ts:122`，store `apps/web/src/stores/control-center.ts:203-206` | UI 目前能加入组，未发现移出按钮 | 端点可复用；需要补交互及确认移出后实例默认分组/当前筛选状态。 |

## 没有证实的后端资源或操作闭环

以下不应记为“后端已有但缺 UI”的成熟功能。当前证据只支持字段、运行时语义或文档契约，不能据此杜撰可复用 CRUD API。

| 想象中的管理能力 | 当前证据 | 结论 |
|---|---|---|
| World / Scene / Location catalog | `life_events`、`life_schedule_items` 保存文本 scene/location (`migrations/runner.go:393-395`)；context resolver 把它们折叠为当前上下文 (`core/life_context.go:168-226`) | 没有 World/Scene/Location 资源表或 CRUD API 证据。现有 Event/Schedule 输入可以复用，但资源目录需要新的领域/API 决策。 |
| NPC authoring | Actor schema 存在，当前创建写入 human/fluctlight (`core/app.go:308,1872`) | 没有 NPC type 或 NPC authoring API 证据。NPC 与 Actor/Persona 的关系需产品建模，不可直接列为 UI 缺口。 |
| Desire 独立编辑 | Intentions 有 inspect/decide/complete 能力；Goal planning 等接口存在 | 没有独立 Desire 表/接口证据；历史 trace 不能替代。先确认 Desire 是否是 Goal/Intention 的别名、来源或独立实体。 |
| 独立物品定义库 | inventory item row 同时记录 description/category/ownership/availability (`migrations/effective_life.go:51-66`) | 未发现 global item definition/catalog。若未来拆分为 catalog 与 instance holding，属于领域模型扩展，不是当前漏掉的 UI。 |
| 手动发布 Moment | 当前可证实的 BrowserClient actions 是 comment/reaction/hide/restore (`index.ts:173-175`) | 未发现创建/发布 API 或前端发布表单。需以全 route registry 再次确认后才能定性。 |
| Chat attachment 上传 | UI 有媒体展示，turn request 内含 `attachmentRefs` | 未发现用户上传控件；目前缺少对 ref 签发/上传 API、权限和存储闭环的核验。暂记待核验，不推断后端不存在。 |
| 前端调用 createConversation | BrowserClient 当前有 `createConversation` 方法；Core 有创建 conversation service/route | 当前页面调用链未找到显式新建 conversation 操作；可能由其它初始化路径确保，应在代码里进一步追查调用点。 |

## 对当前前端功能入口的建议分类

- **可直接沿用既有 API 补入口：** Actor Group member removal；如确认有用户需求，可用 capability catalog read 做只读说明。
- **已有 API 但交互闭环需补齐：** Schedule 的逐项编辑、Item use 状态、Reflection 查询/审查（需先验证现有 read/action API 是否足够）。
- **必须先补后端治理接口或数据模型：** Agent/Tool 定义管理、Reflection proposal 管理（若现有 route 不提供）、World/Scene/Location catalog、NPC authoring、item definition catalog、手动发布 Moment（若最终需求成立）。
- **本轮只能登记待核验：** Chat 附件上传/引用签发、createConversation 实际消费者、Moment publish route、独立 Desire 聚合。
