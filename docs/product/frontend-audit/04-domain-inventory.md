# 04 — 核心领域实体与关系

本文区分**已证实关系**（当前 migration、Core model/service 或有效 API projection）与**未实现概念/建议关系**。全路径相对仓库根目录；`core/...`、`migrations/...` 等简写均相对 `apps/core-go/internal/`，`InstanceDetailsDialog.vue` 相对 `apps/web/src/components/instances/`。Schema/API 的存在不代表它们在 UI 中可编辑。

## 实体和权威数据概览

| 实体 | 已证实的当前形态 | 权威来源 / 持久化 | 当前 UI 表现 |
|---|---|---|---|
| Actor | 参与主体；已证实 actor types human、fluctlight；Fluctlight ID 同时作为其 Actor ID | PostgreSQL `actors` + `fluctlights`，创建逻辑 `apps/core-go/internal/core/app.go:308,1872`；schema `migrations/runner.go:323,337` | 实例列表/详情；Actor context 编辑；没有 NPC type 证据 |
| Fluctlight | 有 owner/creator Actor、foundation revision、persona groups 与 runtime state | `migrations/runner.go:337` 与 Core transaction；Detail 统一读模型 `core/detail.go:14-35` | Instances、Details、Governance、Chat |
| Core Persona | Foundation 中稳定的人格/身份/行为政策资料；persona groups 可多层 | persona contract `.trellis/spec/backend/persona-layer-contract.md:38-43`；detail `core/detail.go:25` | 详情 JSON、Foundation 提案治理 |
| Persona profile runtime | 独立记录 active/previous profile；profile 可作为部分 Goal/Intention/Relationship 的 scope | `migrations/runner.go:342,344,346,385` | 详情展示 runtime 与 profile 数据；没有一套独立全局 Actor/persona CRUD |
| Relationship | Fluctlight 拥有、可按 profile scope、目标为 Actor 的有向关系；含 role/metrics/trend/summary/emotional association/provenance/revision | `migrations/runner.go:385`；read projection `core/detail.go:329,345`；编辑 `relationship_edit.go:113,148` | 详情列表；治理可编辑/回滚 |
| Conversation / Message | Conversation 有 Actor participants；Message 记录 conversation、author actor、sequence、附件、时间/turn metadata、acting profile | model `core/model.go:40,49,58`；schema `migrations/runner.go:348-354`；acting profile `migrations/goal_closure.go:5` | Chat 历史/流式 turn；仅部分 Message provenance/attachments 可见 |
| Goal / Stage / Commitment | Goal 有生命周期/owner/profile/evidence；Stage/Commitment 细化执行 | Goal routes `server.go:83-93`；migrations `migrations/goal_closure.go:9-51,90-102` | GoalPanel 提供治理、修订、证据历史 |
| Intention | 可关联 Goal/profile；带执行状态和 Source；完成/恢复能力 | `migrations/runner.go:346`；runtime `intention_capabilities.go:35,145,291` | Detail 摘要、Goal/Planning 与 Governance |
| Schedule / Schedule item | Fluctlight 生活计划；item 文本字段可含 scene/location，且可关联 Intention 和 action_plan | `migrations/runner.go:394-395`；current reader joins intention `core/life_context.go:126,145-148` | 详情日程；治理全 Schedule JSON 提交/取消 |
| Event / Activity run | Life Event 有状态、时间和文本 context；Activity run 可连 Intention、结果与 authority Event | schema `migrations/effective_life.go:117-133`；authority `core/life_activity_authority.go:15-38` | 详情当前上下文/活动；治理 Event 创建/取消；workflow diagnostics |
| Presence overlay | 覆盖 current_task/user_presence 元信息；不是当前 scene/location 主权威 | reader `core/life_context.go:202-205`；兼容列见 `migrations/runner.go:430` | Governance presence 编辑、Detail context |
| Inventory Item | 单条持有项本身合并 description/category/slot/item kind/ownership/availability/provenance；没有已证实 global definition 表 | `migrations/effective_life.go:51-66`；`migrations/inventory_usage.go:6-10` | 详情库存快照、治理 CRUD |
| Worn items / Outfit / Item use | 穿着关系和 outfit item relation 与 inventory 分离；普通 object use 是另一 relation，并有 start/stop history | `migrations/effective_life.go:69-94`；`migrations/inventory_usage.go:14,24` | 详情展示当前穿着；详情未展现普通 object current-use 和完整 history |
| Effective body / wardrobe / habits | 初始 Foundation 与有效当前状态分开；body/wardrobe 跨 profile；habits 可 profile scoped；unknown/cleared 不回退旧 Foundation | `migrations/effective_life.go:3-5,27-49,96-115`；`core/effective_life.go:14-18,40-52` | 详情分开显示 Foundation 属性、effective appearance 与当前穿着 |
| Inner State / Developing Self | PAD/mood/drives等为动态当前状态；Developing Self 为有证据的软观察，不是 Persona 或临时 inner state | detail `core/detail.go:47,66,70,74,258`；model `core/model.go:19-37` | Details 此刻状态和 developing self claims；Governance 可治理 claims |
| Memory / Memory revision / provenance | Fluctlight 拥有 Memory，可连 Actor、Conversation、Event、evidence refs、personality perspective；有置信度、显著性、visibility、lifecycle、revision/supersession | `migrations/runner.go:388-390`；source links `migrations/memory_provenance.go:3-17,36-51` | 详情筛选后的 Memory 摘要；Governance revise/forget |
| Reflection window / proposal | Reflection V2 按 sequence window 对 evidence 形成候选，再分别评估/应用多个 domain；不是 Memory subtype | `core/reflection_v2.go:155-205`；schema `migrations/runner.go:360-361`；apply `core/reflection_runtime_v2.go:321-422,478-482` | 没有独立管理页面；只间接观察部分已应用状态/诊断 |
| Model/provider binding | 有 endpoint、model、公开 role 与 persisted binding；Core 当前公开角色包括 generic_llm、embedding、conversation_reply、vision_understanding | `core/settings.go:340-344`；assignment `provider.go:103,113,119-121` | Settings 模型角色、Endpoint、当前绑定 |
| Runtime diagnostics / Workflow | model/Agent runs、lifecycle、events、media prompts 与 durable workflow status/history | HTTP routes `server.go:137-155` | Diagnostics 多 section 运维页面 |

## 已证实关系

```text
Actor(human) ── owns/participates ── Fluctlight(actor id = fluctlight id)
Fluctlight ── has ── Core Persona / profile runtime / Developing Self / Current State
Fluctlight ── owns directed Relationship ── target Actor [optional profile scope]
Actor participants ── Conversation ── authored Messages [optional acting profile]
Goal ── evidence links ── source events / messages / outcomes / inventory / activity …
Goal ── linked ── Intention ── optionally referenced by Schedule item
Schedule item ── time/context/action_plan ── Event/Schedule-derived effective Life Context
Event ── authority ── Activity run; result Event provenance is a separate source link
Fluctlight ── owns Memory revisions ── Memory source links ── source identity/revision/fingerprint
Conversation + local date/timezone ── daily summary ── Memory
Conversation/fact/outcome → Reflection window/proposal → candidate disposition → committed domain revision
Inventory Item ── separate relations ── current worn item / outfit slot / current item use
Foundation baseline ── initializes/anchors ── effective current appearance/body/habits (authority-specific)
```

证据：Conversation/Message `core/model.go:40-58`、schema `migrations/runner.go:348-354`；Goal/Intention/Schedule `migrations/runner.go:344-347` 与 `core/life_context.go:126-148`；Activity authority `core/life_activity_authority.go:15-38`；Memory provenance `migrations/memory_provenance.go:36-84`；daily memory `migrations/conversation_daily_memory.go:7-31,56-96`；Reflection `reflection_runtime_v2.go:321-422`；Item relations `migrations/effective_life.go:51-94`。

## 重点问题核实

### World、Scene、Location

- 当前没有证实的 World、Scene 或 Location 独立资源表/实体 ID。Migration 中 `life_events` 和 `life_schedule_items` 的 scene/activity/location 是文本字段（`migrations/runner.go:393-395`）。
- 当前上下文 resolver 选 active confirmed 或 inferred Event，优先 confirmed、再按最近 start；没有 Event 时取 accepted Schedule item；否则返回 pending/null context（`core/life_context.go:176,191,195,226`）。因此 inferred Event 也是当前来源，不应只写“confirmed Event > Schedule”。
- Presence overlay 只覆盖 current_task/user_presence metadata，不替代当前 Fluctlight scene/location（`core/life_context.go:202-205`）；旧 presence 表的兼容 scene/location 列见 `migrations/runner.go:430`。
- “生活世界/生活脉络”是 UI pane 名称（`InstanceDetailsDialog.vue:505`），不是 World entity 证据。

### Actor、NPC、Persona

- 当前 Actor 类型创建证据为 `human` 与 `fluctlight`（`core/app.go:308,1872`）；没有 NPC 类型或 NPC authoring schema 证据。
- Persona profiles 属于 Fluctlight；不是另一种 Actor。runtime active profile 与 Core Persona group 是分离数据。Relationship 目标为 Actor 且可 profile-scoped。
- **待产品定义：**NPC 将来是 Actor 子类型、独立 persona，还是不持久化的场景角色，不能从现有 schema 推断。

### Goal、Desire、Schedule、Event

- 已证实链路是 Goal → Intention → Schedule item（可执行 future item 有 action_plan）。Schedule item 的普通文本描述不等于 action_plan；reader 会联结 Intention status（`core/life_context.go:126,145-148`）。
- Goal source event 与 `life_events` 不是同一个结构：goal evidence source event 是独立 provenance（`migrations/goal_closure.go:53-66,90-102`）。
- Activity run 有自身执行时间与 authority Event；result Event provenance 与 active-scene authority 分别保存/检查（`migrations/effective_life.go:117-133`、`life_activity_authority.go:15-38`）。
- 没有证据证明独立 Desire 聚合。wake-up desire trace 或用户描述不能直接等同 Goal、Intention 或 Desire 表。

### Item 定义、持有、穿着与使用

- 当前单条 item row 同时记录物品描述与持有/可用性；没有已证实 global item definition/catalog（`migrations/effective_life.go:51-66`）。所以“定义与持有分离”是未来建模可能，不是当前实现。
- 穿着是独立 instance/slot→item 关系，Outfit 有自己的 per-slot item relation；普通 object use 有 current-use relation 与 start/stop event（`migrations/effective_life.go:69-94`、`migrations/inventory_usage.go:14,24`）。拥有、穿着、使用不能合并成一个状态字段。
- 详情页面提示“未列出的物品不代表不存在”（`InstanceDetailsDialog.vue:530`）；详情衣柜是分页 partial snapshot，不是完整 ledger。

### 静态初始设定与动态有效状态

- Foundation/Persona baseline 与 effective body/wardrobe/profile habits 分开；有效状态允许 `unknown`/`cleared`，不会回退为旧初始设定（`core/effective_life.go:14-18,40-52`）。
- body/wardrobe 跨 persona profile；habits 显式 profile scoped（`migrations/effective_life.go:27-49,96-115`）。因此“persona 切换 → 全部当前外观重置”不是正确兼容假设。
- Core detail 读取 Foundation、Goal/Intention/Relationship/Memory/Schedule、effective appearance/activity/current state 并处理 stale projection（`core/detail.go:14-35,86-118,143-157`）。前端不应自行合成权威领域状态。

### Memory、Reflection 与对话来源

- Memory source link 记录 source kind、身份/修订/fingerprint 与 valid/unknown/invalid；provenance kind 包括 user_statement、action_result、self_report、inference、summary 等（`migrations/memory_provenance.go:7-17,36-84`）。
- Conversation daily memory 以 Fluctlight、Conversation、本地日期、timezone 关联 summary 与 Memory；摘要保存时间范围、结束状态、open threads、core events（`migrations/conversation_daily_memory.go:7-31`）。
- Reflection proposal 候选包括 Active Memory、Memory、Relationship、Goal、Intention、affect、drives/preferences、Developing Self、Persona/policy overlay（`core/reflection_v2.go:155-205`）；proposal 经 evaluation/application 写回不同领域，不应显示成单一 Memory 类型。
- Owner 可 revise/forget Memory；detail projection 只返回 active、provenance 非 pending/invalid 的最新最多 100 条（`core/operations.go:614,635`；`core/detail.go:355`）。

## 尚未证实 / 建议关系（不属于当前模型）

1. First-class World → Scene → Location catalog：当前字段字符串化；新增实体、ID 和复用语义需产品/领域决策。
2. NPC → Actor/Persona 关系：当前无 NPC 类型证据，需先定持久化主体和参与对话方式。
3. Global Item Definition → owner-specific Inventory Item：当前一个 item row 包含描述与持有状态；拆表属于领域模型扩展。
4. Desire → Goal/Intention 的映射：当前没有单独 Desire 聚合的证据；不要按历史词汇增加重复对象。
5. UI relation graph 可使用已存在的 IDs（Actor、Fluctlight、Profile、Conversation、Message、Memory、Reflection proposal/window、Goal/Stage/Commitment、Intention、Schedule item、Event、Activity、Item）；Scene/Location IDs、NPC IDs 不应伪造。

## 权威持久化与前端信任边界

当前这些 domain authorities 由 PostgreSQL tables 持久化、Go Core 负责读取与 transaction；browser API 是投影与命令边界。Life snapshot 使用 PostgreSQL Repeatable Read/read-only transaction（`core/life_context.go:24-40`）。跨 pane 的 detail 聚合来自 Core read model，前端短缓存/轮询不是权威来源。审计未把只存在于 loose JSON 的字段自动认定为 schema 领域实体。
