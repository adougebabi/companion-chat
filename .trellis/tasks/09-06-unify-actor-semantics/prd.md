# 统一 Actor 语义与多参与者提示词

## Goal

把 Human 用户与 Fluctlight 统一为会话语义中的 `Actor`，让每条消息都由明确的 Actor 发送，并让 Provider 提示词使用 Actor 发送者表达，而不是把 Human 固定成特殊的“用户消息”。这为 Human + Fluctlight 混合会话和后续群聊提供统一基础，同时保留现有认证、授权与 Provider 协议边界。

## Confirmed facts

- 数据库消息已经持久化 `author_actor_id`，Conversation participant 也使用 `actor_id`。
- `actors` 表已有 `human` 与 `fluctlight` 两类实体；当前认证会话解析出的是 Owner Human Actor。
- Relationship 当前表达 `Fluctlight -> Actor` 的有向关系，但提示词投影尚未把发送者 Actor 身份完整带入。
- `compactRecentMessages` 当前主要把消息压缩成 `user/assistant` 角色，可能丢失 `author_actor_id`；当前认知 turn payload 主要传递 `text`。
- Goals/intentions 已会进入 ContextProjection，并被 conversation/daily-review 提示词描述为决策输入；但当前数据主要是 Fluctlight-scoped 的描述文本，尚未明确绑定目标 Actor/关系，也缺少一条强制的“关系目标 → 行动筛选/执行”契约。
- 已确认：关系目标应支持显式 `scope: relationship` 与 `target_actor_id`，同时保留不绑定 Actor 的普通目标；关系目标不能只依赖 description 中的自然语言对象名。
- 现有 Reflection 已有 `relationship_candidates`，会在证据窗口内更新 directed Relationship；但现有 proposal/apply 契约没有明确的 goal/intention candidates，因此“关系变化 → 目标/意图调整”仍未形成闭环。
- 已确认：关系变化、升温/降温和由此产生的目标/行动调整属于 Reflection 学习过程，不属于每条消息的即时认知判断。
- 已确认：摇光可以在 Reflection 中自主演进关系、关系目标和意图，不需要 Owner 对每个语义变化逐项批准。
- 已确认：首版 Actor system 投影采用稳定别名、Actor 类型、关系角色/称谓和当前关系状态；显示名只作为可选展示字段，不作为身份依据。
- Relationship 必须保留来源和生命周期：初始化声明的关系种子，以及 Reflection 基于交互证据形成的关系判断不能在数据上混成同一种来源。`created_by_actor_id`/Owner account 只属于授权元数据，不自动生成社会关系。
- 已确认：关系治理首版允许编辑关系角色/称谓、辅助标签、metrics、trend、summary 和 emotional association；target Actor 不可修改，关系目标/意图保持单独治理。
- 当前 capability registry 有 memory、scene、presence、media 等能力，没有面向模型的只读 Relationship 查询能力。
- Provider Chat API 的 `system/user/assistant` 是传输协议角色，不能直接替代领域中的 Actor 身份。
- 当前 `composeProviderMessages` 会把普通 system 消息合并为运行协议/操作规则，只把 Core Persona 单独提取成系统段；关系快照若要进入 system，必须成为显式的系统上下文区块，不能依赖普通 operation rule 字符串拼接。
- 当前公共会话 API 仍限制最多一个外部 participant；本任务为群聊铺路，但不默认实现完整群聊产品。
- 当前关系详情只能显示 `target_actor_id`、trend、summary，Governance 页面主要提供回滚，没有 Actor 类型、当前用户标识或通用关系编辑表单。

## Requirements

### R1. Actor-first conversation semantics

- Human 与 Fluctlight 都必须作为 Actor 参与消息、参与者和关系语义。
- 领域上下文必须能表达当前自我 Actor、参与者 Actor、消息发送者 Actor 与其 Actor 类型/稳定引用。
- 不得用 Provider 的 `role=user` 推断真实发送者；真实发送者以服务端解析并持久化的 Actor ID 为准。

### R2. Actor-aware Provider context

- 当前输入和最近消息必须携带发送者 Actor 引用，能够渲染为类似“actor_a 给你发送：你好”的语义。
- 同一会话中 Human 与 Fluctlight 的历史消息必须采用同一套发送者表达，不得一类走“用户消息”、另一类走“助手消息”的领域特判。
- Actor 引用应使用稳定、可控、适合提示词的别名/投影；原始数据库 ID 不应被当成自然语言身份直接依赖。
- 提示词需要同时提供当前摇光的 self Actor 与其他参与者 Actor，便于模型判断“对方是谁”。

### R2a. Speaker-scoped relationship context

- Relationship 不是仅供详情页展示的静态数据，而是当前认知与行动决策的运行时上下文。
- 在私聊、群聊以及 Fluctlight-to-Fluctlight 会话中，默认只注入当前摇光与当前说话者（或当前需要回应的参与者）之间的关系，而不是无差别注入完整关系列表。
- 当前说话者可以是 Human 或 Fluctlight；关系查询不得依赖“用户”这一特殊概念。
- 关系上下文至少要能表达关系角色/称谓、关系状态、关系摘要、证据/可信度和适用的边界信息。
- 当决策需要了解非当前说话者的关系时，运行时应提供受权限约束的只读关系查询能力（可作为内部 capability/MCP-compatible tool），而不是让模型自行猜测或读取全量关系。

### R2b. System relationship snapshot

- 当前摇光与当前说话者/需要回应的参与者之间的 Actor 与 Relationship 快照必须注入 Provider 调用的 leading `system` block。
- 快照在单次认知、工具决策和回应生成调用内保持一致，并携带足以重建/审计的 relationship revision 或 context revision。
- Relationship revision、会话参与者或当前需要回应的 Actor 发生变化时，下一次 Provider 调用必须重新生成快照；不能把“理论上稳定”当成永久缓存或绕过 revision。
- 当前消息仍可通过 Provider 的 `user` transport role 传递，但其内容必须引用已解析的 Actor sender，不得重新引入“当前用户”作为唯一语义。

### R2c. Cognition versus Reflection boundary

- Conversation cognition 只处理当前消息的感知、appraisal、即时回应/行动决策；不得在该阶段直接修改 Relationship、Goal 或 Intention 的长期投影。
- Reflection 读取带证据的交互窗口，判断当前 Fluctlight 与目标 Actor 的关系是否变化（包括升温、降温、稳定、角色/称谓变化），再提出可审计的 Relationship revision。
- Reflection 还应能基于关系 revision 与重复证据提出关系目标/意图的新增、调整、完成、暂停或降级候选；这些候选必须通过 Runtime 的验证、CAS、权限和副作用门禁。
- Reflection 的关系/目标/意图演进默认自动落地，不设置 Owner 审批门；revision、证据窗口、幂等和审计仍由 Runtime 保证。

### R2d. Relationship origin and lifecycle

- Fluctlight 创建时可以从初始化输入物化 relationship seeds；种子可以声明目标 Actor、初始角色/称谓、初始状态和初始关系目标。
- Runtime 必须保留必要的系统授权关联（例如 `created_by_actor_id` 与 Owner account），但不能把它自动写成 Relationship，也不能把 Human Actor 的身份特殊化成“用户关系”。
- 没有初始化种子的普通 Actor 关系从 `unknown`/未建立开始；摇光可以在后续 Reflection 中依据证据自主形成关系角色、状态和关系目标。
- 初始化种子不是永久不可变的人格事实。除系统授权边界外，关系角色、状态和关系目标可以被后续 Reflection 修正、升级、降级或结束，并保留来源、证据和 revision。
- Relationship 提示词应区分 `source=initialization` 与 `source=reflection`；授权事实单独放在 authorization context，不能伪装成社会关系来源。

### R3. Relationship integration

- Relationship 查询和提示词投影必须以当前摇光为主体、以任意 Actor 为目标。
- 关系中的“对方是谁”（如 owner、creator、peer、collaborator 等）与信任/亲密度等关系状态必须保持可区分；本任务至少要为后续 typed relationship role 留出明确契约。
- 系统授权关系（Owner/认证）与人格语义关系不能混为一谈。
- 关系可以影响回应方式、行动资格与行动优先级；例如“Actor A 与 Actor B 是情侣”应能支撑“维持感情、持续升温”这类面向关系的长期目标。
- 长期目标/意图需要能明确绑定到目标 Actor 或关系上下文；仅把目标作为无主文本注入 Provider 不足以保证它影响具体行动。
- 关系目标必须以当前 Fluctlight 为主体、以目标 Actor 为对象，例如“你就是 actor_a；actor_b 是你的伴侣；你的目标是维持与 actor_b 的感情并持续升温”，不能建模成当前摇光作为第三方帮助 actor_a 与 actor_b 维持感情。
- 目标驱动的关系行动必须经过当前关系状态、当前说话者、行为策略、权限和证据约束，不能因为目标存在就自动执行副作用动作。

### R4. Compatibility and authorization

- 保留 Provider 所需的 `system/user/assistant` 传输角色；Actor 发送者信息通过结构化 payload 或渲染后的内容补充。
- 浏览器提交的 Actor ID 不得建立权限；消息发送者必须来自已授权 session 或内部自治流程。
- 保持现有消息、Conversation、Relationship revision 与回放兼容，避免在本任务中无必要地重命名历史 `kind` 或数据库列。
- 关系读取工具必须是只读、最小暴露和 Actor-scoped；不得因为模型请求查询关系而绕过 Owner/session 授权或泄露无关 Actor 的关系资料。

### R5. Relationship governance UI

- 关系列表/详情必须同时展示目标 Actor 的类型（Human/Fluctlight）和 `is_current_user` 标识；该标识由服务端根据当前 authenticated Human Actor ID 计算，不能由浏览器传入。
- UI 必须支持查看关系角色、关系状态、摘要、来源、证据与 revision，并明确标注“当前用户”与其他 Actor 的区别。
- UI 必须支持编辑关系的可编辑语义字段，并通过 expected revision、证据引用和治理审计提交；目标 Actor 身份本身不可在编辑关系时被改写。
- 关系编辑与 Reflection 自动演进共用同一 Relationship revision/rollback 语义；人工编辑不是覆盖历史，而是创建新的审计 revision。

## Acceptance Criteria

- [ ] Provider 认知输入包含明确的 `self_actor`、参与者 Actor 投影和当前消息发送者，而不是只有裸 `text`/“当前用户输入”。
- [ ] 最近消息投影保留发送者 Actor 引用，至少能区分 Human、Fluctlight 与不同 Actor 实例。
- [ ] 以同一渲染规则表示 Human 和 Fluctlight 消息；示例输出可表达 `actor_a 给你发送：你好`。
- [ ] Provider transport role 与领域 Actor 身份的职责边界有测试覆盖。
- [ ] 当前单 Owner + 单 Fluctlight 对话行为保持兼容，现有认证授权路径不回归。
- [ ] Relationship 上下文不会把 Owner 权限自动等同为唯一人格关系；其角色扩展点和当前限制有文档/测试说明。
- [ ] 初始化 relationship seed、授权元数据和 Reflection 形成的关系判断在存储、提示词和 revision provenance 中可区分；授权元数据不会自动生成社会关系。
- [ ] 没有 seed 的任意 Actor 对关系从 unknown/未建立开始，并可在后续 Reflection 中基于证据自主形成；seed 关系也可被 Reflection 修正但不突破授权边界。
- [ ] 当前说话者的关系会被按会话上下文选择性注入；不再默认把完整关系列表作为每次对话的静态背景。
- [ ] Relationship snapshot 出现在 leading system block，并在同一次 Provider 调用的认知/工具/回应链路中保持一致；关系 revision 变化后下一次调用可观察到新快照。
- [ ] 至少有一条可验证的数据流说明“关系目标 + 关系状态 + 长期目标/意图”如何影响决策输入或行动筛选，并保留安全的 no-op/不支持路径。
- [ ] 提供只读、权限受控的关系查询能力，覆盖私聊、群聊和 Fluctlight-to-Fluctlight 的当前 Actor 查询场景；非当前说话者的查询边界有测试。
- [ ] 为后续多人 Conversation 保留可验证的参与者/发送者数据流，但不要求本任务交付完整群聊 UI、通知或多账号治理。
- [ ] 关系治理页面能明确显示哪个 target Actor 是当前用户，哪些是其他 Human/Fluctlight，并能查看其关系来源和 revision。
- [ ] 关系编辑请求使用当前 revision 做 CAS，成功后可在关系历史/治理记录中追溯编辑者、原因和证据。

## Out of scope

- 完整群聊产品（群聊 UI、通知、邀请、群管理、多人消息编排）。
- 当前任务不实现“当前摇光作为第三方调节另外两个 Actor 关系”的群聊编排或关系调解能力；这属于后续多人会话/社交协调工作。
- 当前任务不把任意两个非当前 Fluctlight Actor 的关系事实作为第一版 Relationship 主模型；首版关系目标以当前 Fluctlight → 目标 Actor 为边界。
- 当前任务不要求所有关系在 Fluctlight 创建时由任何 Human 手动逐条配置；初始化 seeds 是可选输入，Reflection 可以负责后续关系发现和演进。
- 当前任务的关系编辑 UI 不允许把目标 Actor 改成另一个 Actor；如需新关系，应创建/编辑对应的关系边。
- “不做限制”仅适用于语义成长和目标/意图演进；不解除认证、授权、证据、revision/CAS、能力调用和外部副作用的 Runtime 边界。
- 多 Human 账号、账号邀请、角色治理或匿名模式。
- 立即实现完整的社会图谱查询、推荐、关系路径分析。
- 直接把 Provider 原生 `role` 字段改成 `actor_a` 等非标准值。

## Open questions

无。后续技术取舍以 `design.md` 和 `implement.md` 为准；进入实现前需由用户审阅规划 artifacts。
