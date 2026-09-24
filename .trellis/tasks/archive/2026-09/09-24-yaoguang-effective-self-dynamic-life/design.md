# Design: 当前有效自我与持续虚拟生活

## 1. 边界与数据权威

沿用单一 Go Core composition root、Eino 原生循环和现有授权/receipt/事务机制。新增领域状态只表达业务事实；Prompt、摘要和图像均为读消费者。所有新增 Tool 使用 `App.ExecuteTool` 的正式 prepare、授权、短事务、operation ID 和 receipt 边界。模型请求期间无数据库事务。

| 语义 | 唯一当前权威 | 作用域 | 写入路径 | 历史/版本 | 读取位置 |
| --- | --- | --- | --- | --- | --- |
| 原文/结构化来源 | analysis source + Foundation | Fluctlight/Owner | 现有初始化/正式 Foundation edit | source link/foundation revisions | 原始/历史详情；不逐轮兜底 |
| 核心人格与受保护边界 | `core_persona` + accepted persona overlay | shared + speaking profile | 现有治理/Reflection 受控路径 | foundation/overlay revisions | Effective Persona → compiled portrait |
| 普通偏好/习惯 | Foundation 初值 + profile-scoped accepted habit revision | speaking profile（明确 shared 事实例外） | `habit.decide` 领域操作 | revision + evidence；可清空 | 编译源/画像与有效详情 |
| 身体外观、发长/色、临时发型、伤势 | shared appearance state | Fluctlight 共享身体 | 初始化一次、有效 event/行为结果 | revision/evidence/time；显式 unknown/cleared | Runtime、详情、media context |
| 物品和可用性 | wardrobe item | Fluctlight，归属/使用关系独立 | 初始化/成立的获得或失去结果 | item revision/source event/唯一键 | wardrobe Tool；紧凑当前穿着投影 |
| 搭配与当前穿着 | outfit refs + worn slot state | Fluctlight 共享身体 | outfit edit、`wardrobe.wear` | revision/CAS/事件 | Runtime、查询、media |
| 意愿和尝试 | existing Goal/Intention + attempt | speaking profile | intention Tool、Reflection、结果结算 | existing revisions/attempt/outcome | Runtime 持续事项、Tool、计划器 |
| 计划与实际活动 | accepted Schedule；activity run；Event | Fluctlight | planner/replan；activity start/resolve；Event | schedule version、run/result、Event | Life Context 与近期结果 |
| 情绪 | existing inner state | Fluctlight 当前状态 | cognition/affect_event | state revisions | Runtime/选择 |
| 历史对话与摘要 | messages + summary revisions | Conversation/Owner | existing publication/summary workflow | covered range/source digest | 较早摘要 + 近期原文 |
| 稳定视觉参考/生成图 | Visual Identity canonical + media task | Fluctlight | existing visual/media workflow | canonical revision + context binding | 图像生成；图像不回写状态 |

状态字段采用存在性与有效性标签，禁止用 `COALESCE(initial, current)` 恢复已清空值。Foundation 是初始化来源和历史；只有尚未初始化当前域时才可作为一次性初值输入。写入时把已成立结果、authority revision、source event/operation ID 原子关联。

## 2. 初始化、画像和存量

1. 扩展现有结构化初始化任务及 schema，模型将外观现状、物品持有、正在穿着、审美偏好、普通习惯、长期约束按语义分开。接受前校验对象/数组、false/0/空值、provenance、同一 item 引用和所有权未知；重复 activation 使用稳定 source digest 防重复。
2. 保留原文和完整 Foundation。新 shared appearance/wardrobe 当前域从明确初始语义建立一次；未明确的字段写 unknown，不猜购买或穿着。`life_habits` 按实际 canonical key 进入覆盖检查。画像 compiler 输入改为稳定 allowlist + 当前有效 habit/overlay，带来源 ref；拒绝把当前发长/衣服、时段、瞬时情绪引入画像。修正规则版本与 source hash，让相关习惯更新使画像重编译，衣着/发长变化不触发。
3. 编译与发布保持一致版本：涉及习惯或稳定人格时，先在事务外编译候选，再以来源和 habit/overlay revision CAS 原子发布；失败保留前一套当前有效记录并报告，不静默发布降级、不以旧画像和新来源混用。普通身体/衣物变化无需编译。
4. 存量命令针对 0035 → 新 migration head：`--preview` 只统计和输出脱敏诊断；`--apply` 针对限定 Owner/ID 或显式 all 幂等写入。源优先级：已有有效当前领域修订 > 已确认 Event/行为结果 > Foundation 中可明确归类的初值 > unknown。旧画像里混入的可变事实只作为迁移线索，不能覆盖较新领域事实；无法判定时列人工/模型分类待处理。付费重编译须显式单独选择。支持中断后重跑与逐实例结果日志。API/Worker readiness 在未完成必要 backfill 时 fail closed。

## 3. 衣柜、搭配与穿着

新增窄领域表及 revision/unique 约束（具体 SQL 与当前 migration helper 保持一致）：items、item_sources、outfit definitions/ref rows、current worn slots、shared appearance revision。每个 item 有稳定 ID、category/description、available state、ownership/use relation、source kind/id、revision。初始化已知穿着可标 `ownership=unknown`，但仍有可追溯 item 和 worn relation。搭配只引用 item ID。

`wardrobe.query` 支持 bounded list/detail/outfits/wearing，返回 `has_more`、搜索覆盖范围和可否断言不存在。`wardrobe.wear` 接受 item IDs/部位与 mode=partial/full，prepare 读取当前 item/worn revision；事务中锁 Fluctlight、再次校验可用性/使用权/CAS，full 替换全部，partial 只触及指定部位。失去/不可用与当前穿着的结算在同一事务内清理或标出明确异常，不留下可正常穿的幽灵 item。普通换装不写 habit。

获得不开放任意 `create_owned_item` Tool。只有初始化、经校验的 gift/accepted Event 或 completed 虚拟购物结果通路能写 item；source event 必须存在且匹配主体、结果、类型和幂等键，unique source key 防重复入柜。购买完成不自动穿上。`wardrobe.outfit` 可保存/调整搭配引用，更新偏好须调用独立 `habit.decide`。

## 4. 意愿、活动、日程与结果

现有 Goal/Intention 是唯一意愿权威。新增/完善 `intention.manage` 查询、创建、调整、暂缓、取消；完成路径只接受绑定完成的 ActionOutcome。以 profile + normalized target + active lifecycle 查找可复用目标，模型显式选择复用或新建；取消后不永久去重。`intention.due` 的规范化 stages refs 必须写入 settlement/outcome，并在实际 Tool 操作中绑定已授权意愿。未调用 Tool 时保留 due/延期，不伪造完成。实际 ActionOutcome 的成功边界驱动 attempt settlement。

持续购物和外出剪发建窄 `life_activity_runs`：start 接受明确意愿/计划及时间约束，记录 accepted/scheduled；到达有效时间后由已有 workflow/trigger 驱动一个正式虚拟结果任务，输出 completed/failed/deferred/cancelled 与结构化结果，再由 Core 校验并原子写 Event、item/appearance、ActionOutcome、Intention settlement。成功必须有开始、经过的时间或已接受的事件来源，不能立即由“计划了”推出。模型结果任务不直接写 DB；拒绝现实支付。即时换装/扎发可在 Tool 本地事务完成。伤势只在有明确授权/事件的结果中写入，相关活动 preflight 读取身体约束；不自动制造事故。

初始 Schedule 任务输入增加当前有效生活约束、未完成 Goal/Intention、当前状态及近期结果，并区分已确立课程和模型生成设定。修改现有 instruction/validator，使空闲为明确合法时段，不按身份强制上课、自习或固定穿着。重排只修改 current/future，已有完成结果仍由 Event/Activity 而非 Schedule 证明。WakeUp 消费真实 event/due/activity result，无新变化时允许 no-op；状态更新不自触发无限循环。

## 5. 正式 Tool/Agent 接线

新能力按最小 surface 暴露：conversation、WakeUp、native cognition 可读有效状态、衣柜和意愿；仅获授权的 surface 可做穿着、普通习惯、活动提交/外观行为。反思只按现有受控写入，不整包接入。每个 Tool 的 Request/Result 包含可验证 operation ID、主体、作用域、authority revision 和真实状态；model arguments 不接受 Owner ID 或任意 prepared state。`ExecuteTool` 也校验 `definition.SupportsSurface(request.Surface)`，独立调用不能凭 surface 字符串越权。

Main 正式入口是 `RunConversationCognitionAgent`/`handleTurn`；WakeUp 是 `ProcessWakeUp`，不是测试用 `ConversationRuntime.RunMain`。均通过 registry/catalog → `RunFormalAgent` → Eino ToolCall → `ExecuteTool` → Eino ToolResult。Tool 续接使用提交后的 receipt/fresh read，不重放原始 Foundation；候选未胜出时沿已有冻结与结算边界处理副作用。

## 6. Prompt、查询、摘要与媒体消费者

在 ContextProjection 增加共享身体、紧凑当前穿着、有效习惯版本、必要持续事项；普通 Runtime 不装完整库存。每个事实只由一个位置提供：稳定机制和有效习惯由画像，当前身体/穿着/活动/情绪由 Runtime，过去由摘要/原文。修复外观 slot 的权威来源，媒体 concept 读取当前 appearance/worn item 描述及 revision，保留 frozen capture time；若任务完成时状态已变，图像沿其拍摄版本保存，不提升为“最新状态”。Visual Identity canonical 只作身份参考，不反写当前身体。

`persona.detail` 默认合成当前有效人格/习惯，当前外观/衣着由领域读模型分节返回；raw/historical 明确版本和时点，不把旧卡片断言当前。摘要沿已有阈值和区间机制，完善任务说明和回归：意愿/计划/失败/获得/穿着与历史身体变化各有时间语义；摘要不写物品或当前状态。工具结果留在原生 Tool role，不能升级成 System。

Prompt assembler 用完整最终 messages + tool schemas + response format 实施输入预算。required 超限明确报错；optional 按完整 fragment/turn 裁剪，保留摘要/近期连续性和未结束 ToolCall/ToolResult。分项记录字符、字节、估算 Token 与 Provider usage，来源/版本/入选/排除原因；不记录全量私密文本。初始化、迁移一次性开销与每轮/续接累计开销分开。

## 7. 兼容、验证与回滚

新增 migration 同时进入 clean start 和 released-head upgrade，原迁移常量不改。实施时保留历史与旧媒体资产；删除会把 Foundation 当前外观回填、场景自动换装或双写的旧路径。可回滚部署到旧代码的前提由 migration 兼容性测试明确；数据不自动逆转，回滚时停止新 Tool/Worker 写入并保留事件、receipt、修订。独立 Tool/PostgreSQL E2E、正式 Main/WakeUp scripted Provider、真实 Provider/ComfyUI 验证分别报告，不互相代替。

## 8. 已知风险与决策

- `fluctlight-life-world-contract.md` 旧段称 Schedule 接受后才释放 WakeUp，当前代码允许 WakeUp 独立；以正式代码及用户要求的持续触发为准并同步规范。
- Provider/Persona 旧段有完整 Core Persona 逐轮注入和旧 A/Judge/B 叙述；以 09-22/09-23 原生 Tool/compiled portrait 正式实现为准，修改冲突规范，不复活旧路径。
- 旧初始化自由文本难以确定所有权、发长时态或历史与当前关系；迁移保留 unknown 并输出诊断，明确付费分类要单独显式执行。
- 真实模型或 ComfyUI 不可用时，交付标注 BLOCKED，受控测试证明协议与领域接线，不声称行为或视觉准确度通过。
