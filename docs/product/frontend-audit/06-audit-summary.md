# 06 — 前端功能审计总结

本总结仅归纳当前代码/项目契约中能找到证据的事实。功能分布按 `02-feature-inventory.md` 的 Feature ID 计数；Feature 的 A–F 是交互闭环状态，不是代码质量评分。

## 功能分布

按功能域归类当前清单（包含后端已有但无 UI 的 GAP 项）：

| 功能域 | 功能项 | A | B | C | D | E | F |
|---|---:|---:|---:|---:|---:|---:|---:|
| 认证、实例、对话、动态 | 10 | 8 | 1 | 0 | 0 | 0 | 1 |
| 详情与治理 | 19 | 15 | 4 | 0 | 0 | 0 | 0 |
| 设置 | 8 | 5 | 3 | 0 | 0 | 0 | 0 |
| Diagnostics | 6 | 6 | 0 | 0 | 0 | 0 | 0 |
| 后端 UI 缺口/领域边界 | 10 | 0 | 0 | 4 | 1 | 0 | 5 |
| **总计** | **53** | **34** | **8** | **0** | **4** | **1** | **6** |

> 计数依据：`02-feature-inventory.md` 中的 Feature ID（SYS/INS/CHAT/MOM/DET/GOV/SET/DIA/GAP）。GAP 表把“无已证实实现的领域概念”记为 E/F，并把“后端有运行能力但无管理入口”记 D；不代表所有 F 都是代码遗留，有些是证据待核实或模型概念未成立。

## 交互真实性与本地状态

- 已检查的生产页面主要读写动作均调用 BrowserClient/API；未发现用 mock array、延时成功提示代替生产后端操作的路径。该结论限定于当前 UI/调用链扫描，不代表未运行浏览器端到端验证。
- 本地行为包括：列表搜索/排序/筛选、tab/Dialog/disclosure 开关、实例创建预览与初始目标草稿、Media workflow `file.text()` 导入草稿、运行策略高级 JSON apply-to-draft。它们自身不代表持久化；实例 `activate`、Media settings `save`、Operational settings `save` 才触发 API（`InstancesView.vue:155-202,205-230`、`MediaSettings.vue:25-71`、`OperationalSettings.vue:63-74`）。
- 后端确认/重新查询证据应作为操作完成判断。仅有本地表单状态、Toast 或模拟数据不应被记为 A；本次未发现这样的成功造假调用链。

## 相对完整的模块

- **Chat 基本闭环**：NDJSON turn、消息历史与已读、取消、失败重试和 reconciliation 都接到后端；附件上传仍未核实。
- **Goal 与规划治理**：Goal 生命周期、revision/history/evidence、Goal Set 与 planner、依赖/来源审查都有前端和后端入口。
- **Diagnostics 运维面**：Lifecycle、Agent/model runs、Media prompts、Events、KEV、Workflow status/history/commands 以及 export/clear 均为真实 API 操作。
- **Provider/Settings 与认证**：Endpoint、模型角色、Reply/Vision、Kev、Media workflow 和 owner session 已有持久化配置操作；页面间编辑与查看有分区。
- **人格切换的运行时路径**：Chat turn 可触发 Core 授权的 `persona.switch` capability，Kev Diagnostics/settings 可配置/观察 decision point；前端没有独立 profile 切换控件或 active profile 视图，故管理面为部分覆盖。
- **Relationship/Memory/Actor context 与衣柜 CRUD**：存在 owner governance 操作；UI 数据密度、来源链和更细粒度状态展示不等同后端能力全量覆盖。

## 界面缺失或管理深度不足

- Reflection V2 是真实 Core runtime 和持久化 proposal/window，但没有独立 Reflection 查询/审查入口。
- Scene/location 是 Life Event/Schedule 的字符串上下文；没有证据证明有可复用 World/Scene/Location catalog 或资源 CRUD。
- Agent runs、Tool registry、Workflow controls 有运行观察/控制界面，但没有用户管理 Agent/Task/Tool 定义的证据。
- Schedule 编辑目前主要是完整 JSON submit；Item use、wear/borrow/use/outfit 与 inventory 的关系在 UI 中分散，普通 item current-use 展示不足。
- 详情的 Memory 列表是受限 projection，不展示完整 source/evidence/Conversation links；需要注意“可展示”不等于全量 Memory。
- Chat 暂无用户附件选择/上传入口；只找到 attachment rendering 和内部 `attachmentRefs` 发送槽位，上传与引用签发链待核验。

## 重复或分散的入口

- 实例详情可从列表行和 Chat 标题打开；治理是列表页内部切换，不是独立路由。
- Goal planning/GoalPanel 在详情以只读模式出现，在 Governance 以可写模式出现；若原型合并，需要继续维持权限/操作范围差异。
- Visual identity 状态在 Details，生成 workflow 在 Settings，media prompt 和失败重试在 Diagnostics。
- Provider bindings 的编辑和只读查看分属 model-role 与 binding 两个设置区；当前绑定不是第二份可写配置。

## 历史遗留与兼容点

- 没有 Vue Router；URL 前缀 fallback、`/settings/diagnostics*` 兼容 alias、correlation deep link 都是当前实际入口。
- `WorkspaceRoute` 类型声明的 instance/moments/mode 状态没有编码到 URL；不要把类型定义误认成当前 deep-link 功能。
- BrowserClient 有 `removeActorGroupMember`，UI 未见移出；client 有 `createConversation`，页面调用链未找到；这些需要继续核验消费者，不应静默删除或声称已提供界面。
- Frontend state management 旧 spec 含 persona/SSE/debug inspector/旧 API 路径等漂移；当前行为以运行代码为准。
- Detail / diagnostics 的 request epoch、cursor、分区轮询、store logout 清理不对称是迁移风险，不是“清理旧代码”即可解决的问题。

## 可在 UI 重构中补齐入口的能力

仅从现有 API /数据看，适合下一阶段评估补入口的项目：

1. **Actor Group 移出成员**：BrowserClient/store 已有 delete 操作，当前未见 UI 按钮。
2. **Schedule 分段编辑**：已有 Schedule API 和完整 JSON 提交，补更易理解的编辑器前需保留 revision、timezone 和 action_plan 契约。
3. **Reflection 只读 timeline/候选查看**：backend runtime 已有 proposal/window，但必须先确认是否存在可复用 browser query API；若无，则要新增受控 query route。
4. **Item use 状态/来源查看**：后端 item-use 与 start/stop 关系已存在；先查 owner 可读 API 是否足够，再评估 UI。
5. **Capability registry 只读目录**：当前 registry/catalog 在 Core；不能把它扩成动态 Agent/Tool 安装面板，除非新需求与后端治理模型明确。

## 目前只能规划或待核验的能力

- First-class World → Scene → Location 与资源 CRUD；
- NPC 与 Actor/Persona 的领域关系及创建流程；
- 独立 Desire 聚合及其与 Goal/Intention 的关系；
- global item definition/catalog 与持有项分离；
- Chat 文件/图片上传、引用签发和权限；
- 手动发布 Moment 是否存在其他 API；
- `createConversation` 的当前调用方，以及 Owner 是否可读写 item-use 详细 history；
- Reflection proposal 是否有未检索到的 owner-facing API。

这些不得在本轮原型中伪装成现存服务能力；原型阶段需标明是新产品能力还是待接接口。

## 原型阶段需要明确的问题

1. 主导航里的“聊天”是实例管理列表入口；是否调整名称/结构，需避免丢失列表与分组管理功能。
2. 治理视图是实例 scoped 且包含高风险操作；新导航如何表达 scope、退出与确认语义？
3. Profile 切换目前由 Chat/Core runtime 完成，前端没有手动切换入口；下一阶段是否需要新增可视的 active-profile 查看或切换控制？如果新增，如何保留 Kev/授权与 Core Runtime 的决策边界？
4. Core Persona、Developing Self 与 Current State 的职责边界怎样呈现，避免用户把初始设定当作当前事实？
5. Scene/location 为文本 context，而非资源 catalog；原型应保留当前自由文本还是新增受控词表？这是产品/领域决策。
6. Goal/Intention/Schedule/Event/Activity 的时间、authority 和证据来源怎样串联展示？
7. Inventory 页面是否同时覆盖 ownership、availability、worn outfit、borrow/return 与 current-use，或分步呈现？不得合并为单一 item status。
8. Memory 与 Reflection 的来源证据、候选态、采纳结果是否需要 owner review？当前 detail Memory list 不是全量账本。
9. Chat 是否需要用户上传附件；若需要，上传和撤销 API/权限由谁提供？
10. 哪些 workflow actions 面向普通 owner，哪些只属于诊断运维操作？
11. 旧路径、query IDs、`correlation_id` deep link 与 localStorage 选择状态的向后兼容期限是什么？

## 未核实事项

- Chat 附件 refs 的签发、上传、鉴权与生命周期闭环。
- `createConversation` 的当前消费者及 UI 是否通过其他路径间接创建会话。
- Moment 是否有不在已检查 BrowserClient route map 中的手动发布 API。
- item current-use 的 owner read/manage API 可见范围。
- Reflection proposal 是否有独立 browser API 但未被已检索 route/client 调用。
- 旧 docs/spec 对历史端点和状态的描述与当前运行版本之间的具体迁移时间。

## 结论边界

以上总结用于提供当前审计基线，不包含下一阶段视觉或组件方案，也不把“可在 UI 重构时评估”解释为已经批准新增功能。所有真实调用链与实体证据见 `01-current-pages.md` 至 `05-restructure-risks.md`。
