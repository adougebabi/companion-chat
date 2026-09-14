# 对 `yaoguang_persona_design_review.md` 的逐条核实与答复

> 核实方式：对 F01–F14 逐条回到当前工作树（2026-09-14，基线 `7b26364`）读源码验证，不采信任何一方文档的自陈。
> 结论口径：**成立** = 我读到的代码支持该指控；**部分成立** = 事实成立但严重性或归因需修正，或我不同意结论方向；**不成立** = 与代码不符。
> 所有行号为本次工作树实际行号。

## 总评

**这份审核基本成立，方向正确，应当采纳。** 14 条里 10 条我核实为「成立」，4 条「部分成立」（其中 2 条是审核方过度归因，1 条低估了既有防线，1 条是我确实超范围断言）。

其中 **F02、F05 是它抓对了、而我的设计真错了**：两条都不是文档措辞问题，是设计里写错了权威来源和校验位置。F01 的「结算路径仍可能改 persistent active」与 F04 的「B 全链路 profile scope」是我文档里**承诺了但结构上没保证**的，必须补。

我不同意的部分集中在两处，都是**归因**而非事实：审核认为「切换职责应彻底离开 Main」，但代码显示 Main 的 `personality_decision` 是今天**唯一能工作**的切换机制（见 F01/F08 核实）；以及 F12 的实际严重性——事务内并没有真正的网络 IO。

---

## P0 级

### F01 — Main 仍承载接管规则、切换职责未真正迁出 → **成立（我的设计有真实漏洞）**

审核的事实描述准确：

- 设计确实把 `takeover_rules` 放进普通 Main 的 System Persona：`design.md:288-290`；并保留条件化 `personality_decision`：`design.md:294-295`。
- **更严重的是它的第二段指控，我核实成立**：接管后 B 的 decision 会经过与 A 相同的归一化，其中包含 `preparePersonalityDecision`（`design.md:189`）。该函数在 `mutations.go:809` 被调用后会产出 `decision["personality_transition"]`，而结算事务对 `personalityPlan` 的落地是**无条件**的：

| 位置 | 行为 |
|---|---|
| `mutations.go:1126` | reply 结算：`applyPersonalityDecisionPlanTx(ctx, tx, fluctlightID, personalityPlan)` |
| `mutations.go:956` | no_op 结算：同上 |
| `mutations.go:1303` | 恢复结算：同上 |
| `personality_runtime.go:227-244` | `UPDATE/INSERT public.fluctlight_personality_runtime SET active_profile_id=...` |

即：**B 只要产出一个格式合法的 `personality_decision`，persistent active 就会被改写**，`takeover_once` 的语义就破了。而我的 `design.md:432` 守卫只断言「`turn_takeover.go` 内不直接调用 `applyPersonalityDecisionPlanTx`」——那只挡住了直接调用，挡不住共用的结算路径。**这是一条真实的结构性漏洞，我接受。**

### F02 — Judge 所见文本不一定是实际发送文本 → **成立（P0，我的设计写错了权威来源）**

这是本轮最危险的一条，我核实后确认是我错了。真实取值顺序在生成期就已确定：

- `mutations.go:830`：`visibleCandidate := normalizeVisibleReply(firstString(responsePlan["visible_text"], stringValue(decision["visible_text"])))`
- `mutations.go:863-870`：**只有当上面为空时**才回退到 `replyTextFromCapabilityInvocations`（即 `conversation.reply` 的参数 `text`），随后把结果**回写进** `responsePlan["visible_text"]` 与 `decision["visible_text"]`
- `mutations.go:1107`：结算前重新用同一优先级取值
- `mutations.go:1149`：`INSERT conversation_messages(..., text=visible)` ← 真正"发送"
- `builtin_capabilities.go:120-136`：`Execute` 直接返回 `deferred`；`ExecuteDeferredTx` 只校验 `args["text"]` 非空且 ≤32000，**它根本不参与文本选择**

所以：**根级 / `response_plan.visible_text` 优先于 reply 参数文本**。我的 `design.md:174`、`:186` 把预览来源写成 reply 参数，于是「Judge 审 A、发出 B 的另一种文本」这条路径是真的存在的。审核的构造样例（审「我不是不在意你」、发「随便吧」）在代码上可复现。

### F03 — 只覆盖 frozen payload 不足以成立"胜出候选唯一执行" → **成立（P0）**

- `RowsAffected` 不是 CAS，成立：`cognition.go:474/488/502` 的更新条件只有 `WHERE id=$1 AND status='frozen'`，不改 status，因此两个并发更新都能各自拿到 1。我的 `design.md:345` 把它当并发保证，是错的。
- **覆盖字段不完整，成立，且比审核说的更严重**：`payload` 的真实字段（`cognition.go:414-444`）除 `decision` / `capability_invocations` 外还有 `capability_results`、`state_revision`、`capability_context_snapshot`、以及 `context_reference_version` / `context_reference_index` / `influences` / `goal_refs` / `intention_refs`。其中顶层 `context_reference_*` / `influences` / `goal_refs` 会**残留 A 的**。
- **我补一条审核没看到的**：每个 invocation 的 `ContextSnapshot` 是在 `PersistTurnDecision` 内部算出来的（`cognition.go:429-439`）。我的方案让 B 不走 `PersistTurnDecision`（只 `ReplaceFrozenTurnDecision`），那 **B 的 invocation 根本没有 context snapshot**，执行期会读空或读到 A 的。这条必须在设计里补上。
- 恢复阶段枚举不全、`implement.md:174-175` 的示意代码先读旧 frozen 再 `LoadFrozenTurn`、被拒候选的审计记录必须证明不进入反思/记忆——均成立。

### F04 — B 只换 Working Persona 不足以保证读写归属 → **成立（P0，我文档里没写）**

我核实了「多处读取/写入按 active profile 过滤」这一事实基础：

| 位置 | 行为 |
|---|---|
| `relationship_capability.go:68-73` | 读 `fluctlight_personality_runtime.active_profile_id` 后按 profile 过滤关系 |
| `relationship_edit.go:20-23` | 读写关系以 active profile 为 key（`profile_id=$3 OR profile_id IS NULL`） |
| `context_reference.go:167,245,252,258` | reference index / relationships / goals 均以 `ActiveProfileID` 作用域 |
| `evolution_overlay.go:419-421` | 从 runtime 解析 profile 再合成叠加 |

我的 `design.md`（§4.5）只写了「把 Working Persona 换成 B」，没有写「为 B 重建 profile-scoped 读上下文与写入主体」。审核的验收场景（A-only / B-only / shared 三组数据）成立。

另外 `resolveTurnPersonaScope` 被我放在 `design.md:363`（A 生成**之后**），确实无法证明它就是 A 生成时的 active/revision。审核要求「A 生成前固定 turn snapshot」是对的。

### F05 — 声称已完成 Candidate 校验，但校验仍在 Judge 之后 → **成立（P0，我的设计写错了）**

我核实了校验的真实位置：

| 校验 | 位置 | 相对仲裁点（`mutations.go:888` 与 `:925` 之间） |
|---|---|---|
| 工具存在性 + `invocation.Validate(definition)` | `capability_runtime.go:181-195`（`prepareCapabilityInvocations`），由 `mutations.go:925` 调用 | **之后** |
| `resolveCapabilityAction` | `capability_runtime.go:601-618` | 之前，但它**不做校验**：未知能力名直接 `continue`，最终返回 `no_op` 软失败 |
| 便宜的确定性校验器 | `capability_runtime.go:205-220` `validateCapabilityInvocationsForPersistence` | **它的唯一调用方是 `wakeup.go:530`**，turn 链完全没用 |

所以我的 `design.md:166`「A 与 B 都必须先过现有校验（…工具存在性、参数合法性…）」在当前代码里不成立——非法候选会先进 Judge。**审核正确。**

**但有一个好消息可以降低修复成本**：F05 要求的「无副作用 Candidate Validation」在仓库里**已经存在**（`validateCapabilityInvocationsForPersistence`，只做 registry 查表 + 参数 schema 校验，不碰 IO），只是没接到 turn 链上。修复只需在仲裁点前调用它，不需要新写一层。

---

## P1 级

### F06 — QUERY+ACTION 混合轮是否允许仲裁，文档前后不同 → **成立**

`normalizeConversationResponseMode`（`query_continuation.go:38-47`）只有在 `structuredFallback && visibleText=="" && validatePureQueryContinuation()==nil` 时才返回 `query_continuation`；混合批次含非 pure query 能力 → 返回 `final`。因此 `design.md:313`（只跳过 `query_continuation`）不会跳过混合轮，而 `design.md:308` 又写「混合候选不叠加 takeover」。**确实自相矛盾**，审核的收敛建议（互斥的是「结果依赖型 continuation 路径」而非「任何带 QUERY 工具的轮次」）我接受。

### F07 — 仓库无实例数据 ≠ 用户角色卡无规则可迁移 → **成立（且我用仓库内的证据支持了它）**

审核引用的 `conditions/target_profile/trigger_profile` 示例**不在本仓库**，我无法核实其具体形状，这点我如实标注。

但审核的**结论方向**我用仓库内证据证实了：本仓库自带的多重人格测试角色卡就是自然语言形态，并且明确写着重接管的语义——

`apps/core-go/internal/core/testdata/initialization/dense_multi_card.txt`：
> 「人格系统包含"暮光"与"星火"…**遇到强烈的公开质疑时星火可强制接管**；**收到 actor_user 明确的安全确认后暮光可重新主导**。」

而代码侧：`forced_activation` 是 `openObjectSchema()`（`provider_schemas.go:497`）、默认 `{}`（`app.go:1251`）、**全仓无任何非测试代码读取其语义**。于是我的 `design.md:89`「缺 `mode` → unclassified → 只保留 + 诊断」意味着**真实旧卡永远不会接管**——这是产品性失败，不只是迁移表不完整。

审核另外两点也成立：我的规则结构与选择函数只检查 target（`implement.md:73-83`、`:90`），没有 SourceProfileID / enabled / cooldown 资格判断；同一 B 有多条可触发条件时只取一条。稳定 ID 与规则修订的区分、`switch:<id>` 前缀必须与 validator 同源，也都成立。

### F08 — 持续切换入口不存在，却在验收里写成"保持原有即可" → **成立（并要求我纠正需求本身的假设）**

我核实了全部 `applyPersonalityDecisionPlanTx` 调用方：只有 `mutations.go:956` / `:1126` / `:1303`（均为 turn 结算路径）。而 `personality_runtime.go:260` 的 `applyPersonalityDecision` **是死代码，无任何调用方**。WakeUp / Reflection / 任何 API 都不写 active。

结论：`implement.md:201`「时间/唤醒/反思入口仍能更新 active」这条验收**写不出来**。

同时这也暴露了需求侧的事实错误：`request.md:192` 假设「需要理解语境的持续切换仍由已有 WakeUp/Reflection 处理」，但代码里**不是**——今天唯一的切换机制是 Main 模型读到规则后输出 `personality_decision`，而规则文本确实进了 System Persona：`provider_prompt_composer.go:261-285`（`filterPersonalitySystem` 显式保留 `switching` / `profiles`，注释原话是「Keep them so cognition can name the active profile and reference a declared switch trigger」），`condition` 经 `:318` 原样保留。

**这一点是我要为主设计辩护的地方**：审核 F01/F08 隐含「切换应彻底离开 Main」，但按代码，把 `personality_decision` 从 Main 移除 = 直接打断用户今天赖以工作的白天/晚上切换。正确的收敛是「保留持久切换、但把它变成逐场景命名的显式入口，并禁止 takeover 路径写 active」，而不是「搬空 Main」。

### F09 — Judge 独立记账 ≠ 独立低成本配置 → **部分成立**

- **成立**：`stringSchema()` 就是 `{"type":"string"}`，**没有 `maxLength`**（`provider_schemas.go:31`）。我 `design.md:235`「输出上限由 schema 形状天然约束（两个短字段）」是超范围断言，应改为 `enumStringSchema(...)` 或带 `maxLength`。
- **成立**：role 映射到 `generic_llm`（`diagnostics.go:166`）只解决分类记账，不提供小模型 / 独立 timeout / 输入预算 / 最大输出。设计里确实缺这几项。
- **不成立（审核过度归因）**：审核说「不得称已实现不思考」——我的 `design.md:193` 原文已经写明「Provider 不支持显式关闭，仅未开启」，并没有宣称实现了不思考。

### F10 — 静态函数出现次数不能代替运行时调用预算 → **成立**

守卫就是源码文本计数（`capability_core_test.go:1032` 数 `StructuredAssembledWithToolsSchema(` / `StructuredQueryContinuation(`）。我 `design.md:434` 说新契约「是可静态检查的」属于超范围断言：静态计数约束不了循环、重入与 provider 重试。`implement.md:217` 确实还有运行时测试，所以不是完全缺失，但方向要对：**补 turn 级运行时阶段/调用预算，并把 logical invocation 与物理 HTTP attempt 分开计数**。

### F11 — Working Persona 需要语义保真与明确大小目标 → **成立**

核实：`evolution_overlay.go:452-458` 对 `traits` / `expression` 是 `if len(mapValue(...))==0 { ...= map[string]any{} }` —— **非对象（如数组）会被静默替换成空对象**。而数值路径只有大五类（`evolution_overlay.go:36-40`），真实角色卡却用散文描述特质（`dense_multi_card.txt`：「暮光安静、句子短、行事谨慎」）。于是我的 `design.md:267`「`traits.*`（数值）」对这类卡会**投影不出东西**，语义全丢。审核的「不能臆造数值替代原词」成立。另需补：重构前后 full request / Working Persona / Judge 的大小实测，Judge 独立输入上限。

### F12 — 事务内外结论自相矛盾 → **成立（但严重性低于审核表述）**

矛盾是真的，而且**我的盘点文件自己也打架**：`research/inventory-turn-chain.md` 的 18e 写「外部调用发生在事务窗口内」，同一文件 §6 又写「能力 Execute 均在事务之外」。设计 `design.md:347`「能力 Execute 全部在事务外」重复了后者。

代码事实：`settleDeferredCapabilitiesTx` 由 `mutations.go:1157` 调用，而它**在** `withTransaction`（`:1122`）之内；`capability_runtime.go:363-365` 的注释声称「keeps external/provider work out of the transaction」，但 `:408-433` 确实带着 `tx` 调 `ExecuteTransactional` / `ExecuteDeferred`。

**我不同意的是严重性**：内置 deferred 能力只写本行数据、不发起网络 IO——`conversation.reply` 是纯校验（`builtin_capabilities.go:123-136`），`media.image.generate` 只落一条 `media_intent` 行（`:243-295` → `createMediaIntentTargetTx`），真正的外部生成在后面由工作流/outbox 做。所以不存在「持事务做网络请求」的现状；需要修的是**措辞准确性**，并额外说明 `PreflightTx` 这类入口的行为。

### F13 — research 建议与主设计冲突，且被再次加载为实施上下文 → **成立**

已核实：`implement.jsonl` 与 `check.jsonl` 的**第 1 行仍是 `_example` 占位行**（它自己写着「Delete this line once real entries are added」）；research 文件又作为权威上下文被登记（`implement.jsonl` 第 9-11 条、`check.jsonl` 第 8-9 条）。两处都要处理。

### F14 — 降低过度断言 → **成立（其中一条是我的盘点真错了）**

- **F14.4 我核实为我的盘点错误**：`recentPromptFragments`（`provider_context.go:133-135`、`:164`）**保留** `role=user/assistant`，我的 prompt 盘点写成「recent 全写成 user」是错的。这条要改盘点，不能改代码。
- F14.1 时区/TOON：我不该用 `provider_prompt_format.go:232` 的扁平渲染与 `mutations.go:474-484` 的 switch 去推翻「历史 timestamp 格式化丢时区」。改为「该具体路径尚未充分复核」。
- F14.2 / F14.5 成立：Fake Judge 固定返回 false 只能验证数据布局，不能证明抗提示注入，两类测试要分开报告；`implement.md:22-24` 的 `cd apps/core-go && ...` 连续粘贴会污染后续相对目录，改用 `go -C` 或各自子 shell。
- F14.3 成立且我已如实写过（`implement.md:29-32`、`project_health_transaction_integration_test.go:13-15` 确认 skip 门），保留该诚实标注。

---

## 我核实后新增的既有防线（用于修正严重性判断，审核未提及）

1. **同 inbox 并发由租约串行化**：`claimCognitionInbox`（`cognition.go:112-137`）有 10 分钟 claim 租约（`:121-123`）与更早 pending 检查（`:129-135`），因此 F03 的「两个 worker 同时改同一 frozen」在正常情况下不会发生。
2. **结算前有版本门控**：`requireCognitionAuthorityRevisionsTx`（`mutations.go:1123`）会校验 `ContextRevision/CurrentStateRevision/LifeContextRevision`。
3. **冷启动/重放已有稳定身份**：`frozenID = "frozen_"+stableDigest(inboxID)`（`cognition.go:404`）、`assistant:`+turnID 唯一索引、outbox `ON CONFLICT DO NOTHING`。

所以 F03 的正确结论是：**`RowsAffected` 不是 CAS 这一点成立，必须补预期阶段/版本；但不要把它说成"唯一执行保证已经失效"** —— 既有租约与版本门控仍在兜底，新增的是纵深防御与恢复阶段枚举。

## 收敛顺序（同意审核的下一步）

1. 先修 F01–F05 的执行契约（权限边界、唯一回复文本权威、恢复阶段表、B 的全链路 scope、无副作用候选校验）。
2. 再定 F06–F08 的产品路径（QUERY 互斥矩阵、真实角色卡迁移、持久切换的真实入口）。
3. 把 F09/F10/F11 落成可测试配置（Judge 参数、运行时预算、投影大小目标），不用 role 名 / 函数次数 / 大窗口不溢出代替。
4. 统一 design / implement / research 的冲突建议（F13），research 只留事实，旧建议标 superseded。
5. 补两张表：**阶段恢复表**、**A/B 作用域读写表**。

---

## 用户决议（R1 — 2026-09-14）与采纳结果

用户确认：**接受事实纠正，但不按上一版的过度归因扩大重构范围**。以下为逐条落地位置。

| # | 用户指令 | 落地 |
|---|---|---|
| 1 | 保留现有 Main 驱动的持久切换，收敛为**逐调用场景明确授权**的入口；**不要求**本次新增 WakeUp/Reflection 切换或确定性时间引擎 | `design.md` §0.2/§0.3（`persistentSwitchGrant` + 场景表）、§3.5（选项 A，已决议）、§11-1/§11-7；`implement.md` 阶段 2；阶段 9 **已取消** |
| 2 | 同步修正 `request` 和 `implement` 中对「已有切换入口」的错误描述 | `request.md` §1 表下更正、§11 首段更正、§15 并发段更正、§17.1 场景表更正、§19 清理段更正（5 处，均带 `[更正 2026-09-14，基于代码证据]` 标记）；`implement.md` 阶段 12 + §20 八问第 3/8 条 |
| 3 | 持久切换与接管分开：**有权的原始 Main** 可提出 persistent switch；Judge 只判断 takeover；**接管后的 B 无权修改 persistent active**；权限须在**候选校验、结算、恢复**路径执行，不能只靠移除 Prompt 字段 | `design.md` §0.2 权限模型 + **§0.4 E1–E5 权限执行脊**（候选归一化丢弃 / 持久化授权标记 / **结算门 E3** / **恢复门 E4** / 覆盖时重置授权 E5）；`implement.md` **阶段 2.5**（先于仲裁接入） |
| 4 | Main 保留必要的持久切换规则，但**不重复注入**交由 Judge 判断的 takeover rules，也不因此恢复完整非激活人格 | `design.md` §4.6（System 段**不再有** `takeover_rules`；`persistent_switch` 仅在授权场景带 rules）、§2（`profiles` 收敛为 `{id,name}`）；`implement.md` 阶段 3 |
| 5 | F12 降为**文档和注释修正**，不改现有正确的事务内数据库结算；继续保证被拒候选不能写业务状态或登记待执行 intent | `design.md` §4.10（准确位置 + 仅修注释与文档）、§7（明确不改结算事务）；`implement.md` 阶段 8「附加」；被拒候选保证由 §0.4 E1/E3/E5 + `turn_stage='winner_ready'` 执行资格 + §4.2 覆盖 |
| 6 | F09 撤回「声称已关闭 thinking」的归因，保留输出约束修正；复用当前模型绑定可接受，实际参数/限制/成本如实报告 | `design.md` §4.4（撤回原断言、`decision_code` 改有界 enum、显式列出 timeout/输入预算/最大输出/采样参数、如实写「未发送＝未开启」）；`implement.md` 阶段 4 + `TestJudgeWirePayloadHasNoUnsupportedFields` |
| 7 | 其他审核项**按具体证据继续核对**，不因没有反驳就视为已确认；不为关闭审核项额外引入新架构 | 见下「补充核实」新增 6 条；`design.md` §0.1 表 M1–M10；无新增框架/服务/表 |

**采纳结果**：14 条评审项中，10 条「成立」→ 全部落入 `design.md`；4 条「部分成立」→ 事实部分采纳、归因部分按用户指令修正（F01/F08 的「切换离开 Main」不采纳，F12 降为文档级，F09 撤回其中一条归因）。

---

## 补充核实（R1 新增证据 — 不因「没有反驳」而视为确认）

以下 6 条是本轮为**避免把未反驳当作已确认**而补做的代码核实，全部改变了原结论或原文档表述：

### C1 — `forced_activation` 不是「0 条规则」（更正 P0 级事实错误）

我此前写「仓库内不存在任何具体规则数据」，**错误**。仓库自带多头卡用散文明确声明了接管语义：

- `testdata/initialization/dense_multi_card.txt`（**300 字纯散文，不是 JSON**）：「**遇到强烈的公开质疑时星火可强制接管；收到 actor_user 明确的安全确认后暮光可重新主导。**」
- `testdata/initialization/dense_multi_expectations.json` 断言 `id=forced`：`path = core_persona.personality_system.forced_activation`，`contains = "公开质疑"`，`basis = "explicit"` → **初始化期望要求产出该内容**（不是「只断言键存在」）。
- 该断言只经 `provider_live_tool_test.go:21-23` → `liveProviderInitializationCase`（`:50`）→ `EvaluateInitializationCoverage` 校验；`liveProviderConfig`（`:443-447`）未设 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 时 `t.Skip` → **本地未验证**。
- 影响：R07 的真实形态是「≥1 条语义明确的历史规则 + 无结构/无 ID/无校验/无消费者」→ 语义明确者**必须**产出可执行 takeover 规则，不能整体归入诊断（否则真实旧卡迁移后永不接管）。已写入 `design.md` §3.1/§3.2/§3.3 与 `implement.md` 阶段 2 的真实卡 fixture 要求。

### C2 — 时区缺陷**成立**（推翻我自己的否证）

- `provider_context.go:626-629` `compactMessageTime`：`parsed.UTC().Format("01-02 15:04:05")` → **强制 UTC 且无时区标识**。历史消息进入 prompt 的时间戳对 GMT+8 用户偏移 8 小时且看不出偏移。
- 我此前用「TOON 是扁平渲染」「时区是简单 switch」去推翻它，**依据不成立**（评审 F14.1 正确）。
- 影响：`design.md` §0.1 M7、§2 末行；`implement.md` 阶段 8 由「复核不成立」改为**成立 + 小范围修复**。TOON 递归改为**「尚未充分复核」**（不声称不成立）。

### C3 — A 的审计行消费者清单实测为**空**

- `cognition_assessments` / `cognition_decision_proposals` 全仓仅见**写入**：`cognition.go:455`、`:458`；表定义在 `migrations/runner.go:187`、`:188`；**没有任何非测试读者**。
- 影响：评审 F03 最后一条（「必须检查消费者，证明它们不会以 accepted 事实进入反思和记忆」）由**实测证据**关闭，而非靠禁止读取 `takeover.rejected_candidate` 的承诺。已写入 `design.md` §4.2 与 §0.1 M10。

### C4 — 请求大小设施**已存在**（更正我的概括）

- `prompt_context_assembler.go:14-19`（cap 常量）、`:38-48`（`DefaultPromptBudgetPolicy`）、`:215-225`（`PromptAssemblyTrace`，含 `EstimatedInputTokens` / `SectionTokens`）、`:255-259`（三层 cap 强制）、`:397-399`（`estimatePromptWireInput` **覆盖 tools 与 response_format**）。
- **确定性**守卫：`provider_live_tool_test.go:34-47`（不调用 provider）；`prompt_context_assembler_test.go:186-187`。
- 影响：我「全仓无请求大小统计设施」的说法更正为「已有 token 设施；缺分节字节数、Judge 覆盖、机器可读报告」（`design.md` §0.1 M9、§4.5/§4.6；`implement.md` 阶段 3/11.5）。

### C5 — 静态守卫是**源码文本计数**

- `capability_core_test.go:1026-1035`：`strings.Count(source, "StructuredAssembledWithToolsSchema(") != 1` / `StructuredQueryContinuation(` != 2 / 无 `invocation.CapabilityName ==`，并要求 `decision["cognitive_state_transition"] = "not_proposed"` 存在；另对 `wakeup.go`/`workflow_ops.go`/`capability_runtime.go` 断言无具体能力分派。
- 影响：确认 F10 —— 静态计数**不能**替代运行时调用预算。已补运行时阶段/预算测试（`design.md` §8；`implement.md` 阶段 10）。

### C6 — recent 消息**保留真实 role**（我的盘点写错了）

- `provider_context.go:133-135` 过滤 `role ∈ {user, assistant}`，`:164` 写入 `Content{"role": role, "content": content}`；`prompt_context_assembler.go:370-390` `assemblePromptMessages` **原样 append** recent fragment → prompt 里 recent 的 role 是真实的 user/assistant。
- 影响：**改盘点，不改代码**。已修 `research/inventory-prompt-schema.md` §2 消息布局表（原写 `2..n = user`），并在 `design.md` §0.1 M8 记录。

---

## 仍未解决 / 未验证（不得当作已完成）

| # | 项 | 状态 |
|---|---|---|
| U1 | 真实多头卡初始化是否真产出 `forced_activation` 含「公开质疑」 | **未验证**（live provider 未运行，路径 skip） |
| U2 | 真实模型抗提示注入能力 | **未验证**（Fake Judge 只证明数据布局） |
| U3 | 真实 Token 计费与可见延迟 | **未测量** |
| U4 | `persistent_deterministic` 求值 | **未实现**（已决议不做） |
| U5 | TOON 递归丢标志 | **尚未充分复核**（需递归调用链证据） |
| U6 | 无 `GO_CORE_TEST_DATABASE_URL` 时的链路级断言 | **未真实执行**（测试 skip） |
