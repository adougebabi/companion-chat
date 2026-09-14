# 技术设计：多重人格接管、Working Persona 与 Prompt 收敛

> 对应 PRD：`.trellis/tasks/09-14-persona-takeover-working-prompt/prd.md`（R01–R12）
> 原始需求：同目录 `request.md`（1–20 节，全部为约束；其中 3 处关于「已有切换入口」的描述已按代码证据修订，见 §3.1）
> 代码盘点证据：同目录 `research/inventory-{persona,prompt-schema,turn-chain,tests-specs}.md`
> 评审与答复：同目录 `yaoguang_persona_design_review.md` + `review-response.md`
> 基线提交：`7b26364`（master，工作区干净）
> 所有行号以本次工作树（2026-09-14）为准；**行号是证据引用，不是实现要求**。

---

## 0. 修订记录与权限模型（R1 — 2026-09-14）

### 0.1 本次修订接受的事实纠正

评审（`yaoguang_persona_design_review.md`）与逐条复核（`review-response.md`）之后，本设计做了以下**事实层**修订，并刻意**不**按评审的过度归因扩大重构范围：

| 编号 | 修订 | 依据 |
|---|---|---|
| M1 | 可见文本的唯一权威**不是** `conversation.reply` 参数，而是生成期已定的 `response_plan.visible_text` / `decision.visible_text` 优先值 | `mutations.go:830`、`:863-870`、`:1109`、`:1149`；`builtin_capabilities.go:120-136` |
| M2 | 候选校验**不在**仲裁点之前发生：`prepareCapabilityInvocations` 由 `mutations.go:925` 调用（仲裁点之后） | `capability_runtime.go:168`、`:181-195`；`mutations.go:925` |
| M3 | `RowsAffected` **不是** CAS：条件只有 `id=$1 AND status='frozen'` | `cognition.go:474/488/502` |
| M4 | payload 覆盖范围漏了顶层 `capability_context_snapshot` / `context_reference_*` / `influences` / `goal_refs` / `intention_refs`；且 per-invocation `ContextSnapshot` 在 `PersistTurnDecision` 内部计算 | `cognition.go:414-444`，尤其 `:429-439` |
| M5 | `B` 的私有读写必须按**冻结的 reply owner** 作用域重建，不能沿用当前 active | `provider_context.go:102`、`:184-186`；`relationship_capability.go:68-73`；`relationship_edit.go:20-23`；`context_reference.go:167/245/252/258` |
| M6 | R07 的旧结论「仓库内 0 条具体规则数据」**不成立**：仓库自带多头卡明确声明了接管语义，且初始化期望要求产出它 | `testdata/initialization/dense_multi_card.txt`；`dense_multi_expectations.json` 断言 `id=forced`；`provider_live_tool_test.go:22` |
| M7 | 时区问题**成立**，此前我以「TOON 扁平渲染」推翻它不成立：历史时间戳确实丢掉时区 | `provider_context.go:626-629` |
| M8 | 我此前的 prompt 盘点写错一处：`recentPromptFragments` **保留** `role=user/assistant` | `provider_context.go:133-135`、`:164` |
| M9 | 「全仓无请求大小统计设施」**不成立**：已有 token 估算与三层 cap 及确定性守卫 | `prompt_context_assembler.go:14-19`、`:38-46`、`:255-259`；`provider_live_tool_test.go:34-47` |
| M10 | A 的审计行 `cognition_assessments` / `cognition_decision_proposals` **只有写入、全仓零读取方** → 被拒候选不可能经它们进入反思或记忆（实测证据，非承诺） | `cognition.go:455/458`；`migrations/runner.go:187/188` |

**不予采纳的评审归因（保留主设计方向）**：

- 评审 F01/F08 隐含「持久切换职责应彻底离开 Main」。代码事实是：`applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`）的调用方只有 `mutations.go:956/1126/1303` 三处结算路径，`applyPersonalityDecision`（`personality_runtime.go:260`）**无调用方**，WakeUp / Reflection / 任何 API **都不切换**（`reflection_runtime_v2.go:320/332` 只读 `active_profile_id`）。今天唯一能工作的切换机制就是 Main 读到 `switching` 规则后输出 `personality_decision`，且该规则被 `filterPersonalitySystem` 显式保留（`provider_prompt_composer.go:261-285`）。搬空 Main = 直接打断白天/晚上切换。
  **本设计的收敛方式**：保留 Main 驱动的持久切换，但收敛为**逐调用场景明确授权**的入口，并在**候选校验 / 结算 / 恢复**三处执行权限（§0.4），而不是只从 Prompt 里删字段。
- 评审 F12 的严重性被高估：事务内确实调 `ExecuteDeferred`，但内置 deferred 能力**没有网络 IO**。降为文档与注释修正（§4.10），**不改**现有正确的事务内数据库结算。

### 0.2 权限模型（核心，对应 F01）

两套机制**分离**，共用同一条权限执行脊：

| | **Persistent Switch** | **Turn Takeover** |
|---|---|---|
| 目的 | 改变持久主导人格 | 只改本轮发言人格（`reply_owner_profile_id`） |
| 声明来源 | `personality_system.switching`（既有，`provider_schemas.go:578-579`） | `personality_system.takeover_rules[]`（新增 typed）+ `forced_activation` 作为受控迁移输入（§3.2） |
| 提出者 | **有权的原始 Main**（`cognitive_assessment` 场景且该场景被授权） | **无提出者**：Runtime 从规范规则确定性选定唯一接管方；Judge 只判布尔 |
| 注入位置 | Main System Persona 的 `persistent_switch` 段（仅授权场景；仅 id/condition/target，**不含**非激活人格全文） | **只进 Judge 的 control view**，**不进 Main** |
| 写入 `active_profile_id` | **可以**（受授权门控，走既有 CAS） | **绝对不可以**（4 层执行，§0.4） |
| 生效范围 | 跨轮 | 单轮 |

派生规则（本设计的硬约束）：

1. **Judge 不判断持久切换**：Judge 的响应 schema 里根本没有持久切换字段（`takeoverJudgementSchema()` 只有 `takeover` + 有界诊断码）。
2. **接管后的 B 无权修改 persistent active**：B 的调用场景是 `takeover_reply`，结构性不授权；B 即使返回格式完全合法的切换提案也不得生效（§0.4 E1/E2/E3）。
3. **Main 保留必要的持久切换规则，但不重复注入 takeover rules**：`takeover_rules` 只出现在 Judge 输入；Main 的 System Persona 里不出现该字段。
4. **不因此恢复完整非激活人格**：Main 的 `profiles` 收敛为 `{id,name}` 名单，不再展开非激活 profile 的完整内容（§4.6）。

### 0.3 逐调用场景授权表（`persistentSwitchGrant`）

```go
type persistentSwitchGrant struct {
    Allowed       bool
    Scenario      string   // 本次调用的场景名
    DeclaredRules []string // 来自 personality_system.switching 的声明 ID，仅诊断
    Reason        string   // 未授权原因，仅诊断
}
```

| 调用 | scenario | `Allowed` | 说明 |
|---|---|---|---|
| A 主生成（普通 `reply` / `no_op` turn，且实例声明了 `switching` 入口） | `cognitive_assessment` | **true** | 与今天行为一致，保证白天/晚上切换不失效 |
| A 主生成（实例未声明任何 `switching` 入口） | `cognitive_assessment` | false | 响应 schema 不再含 `personality_decision`（R11） |
| **B 主生成（接管后）** | `takeover_reply` | **false（无条件）** | 结构性禁止，不看实例是否声明规则 |
| Judge | `takeover_judge` | n/a | schema 无该字段 |
| QUERY 无 Tools 合成 | `query_continuation` | false | 沿用既有路径 |
| WakeUp / Reflection / 其他 autonomy | 各自既有 scenario | false | **本次不新增切换**（已决议，§11） |

### 0.4 权限执行脊（E1–E5，缺一不可）

> 评审 F01 要求：「在候选校验和结算权限处禁止 takeover 产生 persistent switch，不只检查 Prompt 或函数名。」本设计的执行点如下，全部为**结构性**而非 Prompt 级。

| # | 层 | 位置 | 行为 |
|---|---|---|---|
| **E1** | 候选归一化（写入前） | 新增 `applyPersistentSwitchGrant(decision, grant)`，在 `mutations.go:888` `PersistTurnDecision` 之前 | `!grant.Allowed` 时**删除** `decision["personality_transition"]`（以及根级 `personality_decision` 提案残留），改写 `decision["persistent_switch_proposal"] = {proposed, applied:false, reason, scenario}`。**不是**整轮失败（避免一个多余字段毁掉用户可见回复），而是结构性丢弃 + 诊断事件 `persistent_switch_proposal_dropped` |
| **E2** | 持久化授权标记 | `cognition.go:398` `PersistTurnDecision`（扩展签名，接收 `grant`） | payload 写入 `persistent_switch: {authorized: bool, scenario, grant_revision}`。若 `personality_transition` 存在而 `authorized=false` → 返回 `personality_switch_not_authorized`（纵深防御；E1 正常运行时不可达） |
| **E3** | **结算权限门** | `mutations.go:956/1126/1303` 三处统一改为 `applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, payload, plan)` | `plan == nil` 或 `payload["persistent_switch"]["authorized"] != true` → **结构性 no-op**。**禁止**用「decision 里有 `personality_transition`」推断授权 |
| **E4** | **恢复路径** | `mutations.go:722-727` 重建 `personalityPlan` 之后 | 同样走 E3。恢复不得重判，也不得凭字段存在就写 active |
| **E5** | **覆盖写必须重置授权** | `ReplaceFrozenTurnDecision`（新增，`cognition.go`） | 覆盖时**必须**把 `persistent_switch` 重置为 `{authorized:false, scenario:"takeover_reply", reason:"takeover_reply_owner"}`，并清除 B 的 `personality_transition`。授权**绝不从被覆盖的 payload 继承**（否则 A 的授权会被 B 继承，这是最隐蔽的破法） |

**验收（F01 原文）**：
- `TestTakeoverReplyCannotWritePersistentActive`：Fake Judge 返回 true，Fake Provider 让 B 返回**格式完全合法**的持久切换提案 → `fluctlight_personality_runtime` 逐字段不变，且 payload 的 `persistent_switch.authorized == false`。
- `TestPersistentSwitchStillWorksViaAuthorizedScenarios`：授权场景下持久切换照常生效（回归白天/晚上切换）。
- `TestAPromptHasNoTakeoverRules`：A 的 System Persona 不含 `takeover_rules`。
- 静态守卫：`applyPersonalityDecisionPlanTx` 在 `turn_takeover.go` 中出现 0 次，且三处结算点不再直接调用它（只经 E3 门）。

### 0.5 设计结论摘要（保留）

1. **A 的候选天然是「未执行提案」**：`PersistTurnDecision`（`cognition.go:398`）提交后、结算事务（`mutations.go:1122`）之前，`applyPersonalityDecisionPlanTx`、`applyFrozenCognitiveStagesTx`、`conversation_messages` 写入、`settleDeferredCapabilitiesTx` 都还没发生。
2. **结算事务以 frozen payload 为唯一事实源** → 拒绝 A 的全部副作用靠覆盖 payload 实现，无需在多个副作用点打补丁。
3. **Judge 插入点**：`mutations.go:888` 之后、`:917/:925` 之前。放在 Prepare 之前是硬约束（R02）。
4. **回复发送管线唯一**：`conversation.reply` 是唯一 `OutputRole == "conversation_message"` 的能力（`tool_contract.go:212`），无二次模型改写，全仓无 `visible_reply` 旁路。**但「唯一发送管线」不等于「唯一文本权威」**（M1），见 §4.3。
5. **Judge 复用既有模型绑定**：`providerBindingRole`（`diagnostics.go:166`）把除 embedding 外的一切角色绑到 `generic_llm`，`recordModelRun` / `provider_provenance` 按 role 分别落库。**接受复用当前模型绑定**，但必须显式配置与如实上报超时、输入预算、最大输出、采样参数（§4.4）。
6. **Working Persona 走唯一 System Persona 出口**：今天存在三处独立的人格序列化（`provider_context.go:188` `core_persona`、`:197` `effective_persona`、`:194` `personality_system.active_profile`），且 `core_persona.personality` 与 `effective_persona.personality` 是两种形状。本次合并为**一个由 Core 校验的 Working Persona**。

---

## 1. 现行调用链表（R01）

完整表格见 `research/inventory-turn-chain.md` 第 1 节（21 步，每步含 `文件:行号`）。此处只保留设计必须锚定的骨架：

| # | 阶段 | 位置 | 事务 | 关键性质 |
|---|------|------|------|----------|
| 1 | 用户消息落盘 | `mutations.go:527` `withTransaction` → `cognition.go:224` `enqueueTurnFactTx` | 短事务 | 用户消息 + inbox + workflow intent + outbox 原子提交 |
| 2 | 抢占/重放检查 | `mutations.go:650` `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`） | 内部事务 | 已有 assistant 时重放结算 |
| 3 | 读取已冻结候选 | `mutations.go:692` `LoadFrozenTurn`（`cognition.go:345`） | 否 | 重放时不再调模型 |
| — | **turn 快照（新增）** | `mutations.go:711` `buildTurnProjection` 之前 | 否 | **F04 修复点**：A 生成前固定 active/reply-owner/revision |
| 4 | **A Main Cognition** | `mutations.go:763` `assembleProjectionPrompt` → `:770` `StructuredAssembledWithToolsSchema` | 否 | **唯一主生成站点**（静态守卫断言 `mutations.go` 中该调用恰好 1 次） |
| 5 | 决策归一化 | `mutations.go:822/838/877` `normalizeResponsePlan` / `resolveCapabilityAction` / `normalizeCompositeAction` | 否 | 纯内存；`preparePersonalityDecision`（`personality_runtime.go:83`）也是只读 |
| 6 | **冻结候选** | `mutations.go:888` `PersistTurnDecision`（`cognition.go:398`，内部事务 `:446`） | 是 | 写 `cognition_assessments` / `cognition_decision_proposals` / `cognition_frozen_actions(status='frozen')` |
| 6.5 | **无副作用候选校验（新增）** | 复用 `validateCapabilityInvocationsForPersistence`（`capability_runtime.go:205-220`） | 否 | **F05 修复点**：A 在进入 Judge 前必须先过 registry 查表 + 参数 schema 校验 |
| — | **★ 仲裁窗口** | **`:892` 与 `:917` 之间** | — | **A 已持久，零副作用；Judge 的唯一插入点** |
| 7 | Prepare | `mutations.go:925` `prepareCapabilityInvocations`（`capability_runtime.go:168`） | 否 | 参数补全；部分能力有外部预检（**只对胜出候选**） |
| 8 | 持久化已准备调用 | `mutations.go:933` `persistFrozenCapabilityInvocations` | 单语句 | 崩溃/重放边界 |
| 9 | 执行纯查询能力 | `mutations.go:1012` `planCapabilitiesForTransaction` | 否 | pure query 真正执行 |
| 10 | QUERY continuation | `mutations.go:1043-1105` | 否 | 至多一次无 Tools 合成（`query_continuation.go:20` 限定 1–2 个 pure query） |
| 11 | **结算事务** | `mutations.go:1122` `withTransaction` | 是 | `:1126` 人格演化（**经 E3 门**）/ `:1129` appraisal+情绪+状态 / `:1149` 私聊消息行 / `:1157` 能力执行 / `:1182` 完成+outbox |
| 12 | 投递 | `mutations.go:1204` 同步 NDJSON 流；`platform/redis_pipeline.go:122/244` 异步 outbox→Redis→consumer | 否 | 两条通道，均在结算提交之后 |

**结论（R01）**：候选冻结与副作用提交之间存在一个物理窗口，窗口内的所有副作用都由同一个事务、同一份 payload 驱动。这是本次设计成立的前提。

---

## 2. 职责迁移表（R01）

| 现有职责 | 位置 | 新职责 | 处置 |
|---|---|---|---|
| 从 `profiles` 数组取 active profile 整对象塞进 Runtime Context | `provider_context.go:289-315` `compactPersonalitySystem`（`cloneMap(system)` + 追加 `active_profile` 整对象） | 不再传整组 profiles；只保留 `active_profile_id` + revision | **迁移** |
| **Main 的 `profiles` 展开为非激活 profile 的完整内容** | `provider_prompt_composer.go:261-285` `filterPersonalitySystem` → `:299-322` `filterSemanticProfileValue`（只对 `id/name/rules/edges` 特判，其余键回落 `filterCorePersonaValue` → **内容字段被保留**） | 收敛为 `{id,name}` 名单；**不恢复完整非激活人格** | **迁移（F01/R09）** |
| `effective_persona`（`personality` + `behavioral_policy`）同时出现在 System 与 Runtime Context | `provider_context.go:197-199,279-287` vs `provider_prompt_composer.go:246` | 合并为唯一 `working_persona`，只在 System Persona 段出现一次 | **迁移（去重）** |
| `core_persona.personality` 扁平键 vs `effective_persona.personality` 的 `traits.*` 嵌套 | `app.go:1255` / `evolution_overlay.go:36-40,452-458` | 统一规范形状；非对象 `traits` 不再静默变 `{}`（F11） | **迁移（修缺陷）** |
| `forced_activation` 开放对象，无结构、无校验、无 ID、**但仓库自带卡确实声明了接管语义** | `provider_schemas.go:497`；`app.go:657/874/1109-1113/1251`；`testdata/initialization/dense_multi_card.txt` | 保留原文作为迁移证据；初始化 Provider 另产出带稳定 ID、kind、version、显式 target 的 typed `takeover_rules[]`。Runtime 不从 prose 推断执行语义 | **迁移（R07/R2-02）** |
| `switching.rules[].id` 校验（LLM 决策的 trigger 白名单） | `personality_runtime.go:158-178` | **保持为持久切换的唯一执行入口**，纳入统一 rule ID 空间 `switch:<id>` | **保留 + 统一 ID** |
| 主生成的 `personality_decision` 每轮可输出 keep/switch | `mutations.go:808-816`；`provider_prompt_composer.go:261-285` | **保留**，但收敛为逐调用场景授权（§0.3）；未授权场景不出现在 schema 中 | **保留 + 授权门控** |
| `takeover_rules` 曾计划放进普通 Main 的 System Persona | 旧 `design.md:288-290` | **改为只进 Judge 的 control view**；Main 完全不出现该字段 | **迁移（F01）** |
| 单一主动人格决策 | `personality_runtime.go:83/227` | 持久 active 不变；新增 turn 级 `reply_owner_profile_id` | **扩展** |
| `cognition_frozen_actions` 状态机 `frozen/completed/failed` | `cognition.go:461/638/780` | 不新增状态；新增 **payload 内 `turn_stage`**（§4.8）；被拒候选记在 `takeover.rejected_candidate` | **保留状态机 + 扩展 payload** |
| 冻结 payload `decision` / `capability_invocations` | `cognition.go:414-444` | 接管时整体替换为 B 的候选，**并重置授权与阶段**；补 `capability_context_snapshot` / `context_reference_*` / `influences` / `goal_refs` / `intention_refs` | **扩展（M4）** |
| `visible_text` 与 `conversation.reply.arguments.text` 双来源 | `mutations.go:830/863-870/1109` | **生成期确定唯一 canonical 值并冻结**；preview / Judge / 发送端读同一个冻结值 | **修缺陷（M1/F02）** |
| `tool_calls` 原生通道 + JSON 侧车双通道 | `provider.go:338/396` | 保留（Provider 真实协议） | **保留** |
| 请求大小设施 | `prompt_context_assembler.go:14-19,38-46,255-259`；`provider_live_tool_test.go:34-47` | 已有 token 估算与三层 cap + 确定性守卫；**补**分节字节数、Judge 请求覆盖、机器可读报告 | **接通 + 扩展（M9）** |
| QUERY continuation 至多一次无 Tools 合成 | `query_continuation.go:20/92`；`mutations.go:1061/1088` | 与 takeover 互斥；接管后的 B 不进入该路径 | **保留 + 加互斥** |
| `role: "tool"` 仅允许出现在 `query_continuation.go` | 架构守卫 `project_health_architecture_guard_test.go:21` | Judge 与 B 都不引入 `role=tool` | **保留** |
| 历史消息时间戳 | `provider_context.go:626-629` `compactMessageTime`（`parsed.UTC().Format("01-02 15:04:05")`，**无时区标识**） | 保留时区偏移；最小改动，不重写格式系统 | **修缺陷（M7/R11）** |

---

## 3. 切换入口与 forced_activation（R07）

### 3.1 盘点事实（更正 M6）

**更正**：旧设计写「仓库内 0 条具体规则数据」，**不成立**。仓库自带的双重人格卡用自然语言明确声明了接管语义：

- `testdata/initialization/dense_multi_card.txt`（300 字，**纯散文，不是 JSON**）原文：
  > 「人格系统包含"暮光"与"星火"：暮光安静、句子短、行事谨慎，会反复检查门锁；星火热烈、语速快、爱用反问，画画时会哼歌。**遇到强烈的公开质疑时星火可强制接管；收到 actor_user 明确的安全确认后暮光可重新主导。**」
- `testdata/initialization/dense_multi_expectations.json` 的断言 `id=forced`：
  `path = core_persona.personality_system.forced_activation`，`contains = "公开质疑"`，`basis = "explicit"`。
  即**初始化期望要求产出的 `forced_activation` 里必须含有该接管条件**。

**但**（这才是可执行的现状）：

| 事实 | 位置 |
|---|---|
| schema 无内部结构 | `provider_schemas.go:497` `openObjectSchema()` |
| 是必含键 | `app.go:657` `hasInitializationKeys` |
| 初始化白名单含它 | `app.go:874` |
| 缺失时补默认 | `app.go:1109-1113` |
| 默认值空对象 | `app.go:1251` |
| 契约测试只要求键存在 | `initialization_contract_test.go:123/142/225` |
| **零读取方** | 全仓无非测试代码读取其内部语义 |
| **唯一校验路径是 live provider** | `provider_live_tool_test.go:21-23` → `liveProviderInitializationCase`（`:50`）→ `EvaluateInitializationCoverage`；而 `liveProviderConfig`（`:443-447`）在未设 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 时 `t.Skip` |

**结论**：R07 的真实形态是「**≥1 条语义明确的历史规则（来自仓库自带卡）+ 无机器可读结构 / 无 ID / 无校验 / 无消费者**」。因此本次迁移保留原始 prose，同时把可执行语义的责任放在初始化/迁移 Provider：只有 Provider 产出完整 typed `takeover_rules[]`（含稳定 ID、版本和显式 target）时，Runtime 才会执行；没有 typed 结果的旧 prose 只生成 bounded diagnostic，不会被 Runtime 猜测执行。

仓库固定 fixture 已覆盖两条边界：真实 dense card prose 归一化为 0 条 executable takeover 并保留诊断；同一语义的 `dense_multi_typed_migration.json` 代表授权迁移 Provider 输出，可归一化为 1 条 executable takeover。该卡是否在真实 Provider 初始化时产出 typed 字段仍未验证（live provider 路径被 skip），报告不得把 fixture 结果写成 live 结果。

### 3.2 三类规则与唯一执行语义

| 类别 | 声明来源 | 执行语义 | 本次是否实现执行 |
|---|---|---|---|
| Persistent Switch（语义持续切换） | `personality_system.switching` | 由**授权的 Main** 输出 `personality_decision` → `preparePersonalityDecision` → 结算经 E3 门写入 | **是**（保留既有） |
| Persistent Switch（确定性/时间窗） | `switching.rules[].condition` 携带机器可读时间窗 | 需要 Runtime 求值器 | **否**（已决议选项 A，§3.5） |
| **Turn Takeover** | `personality_system.takeover_rules[]`（**唯一执行语义**）；`forced_activation` 仅作**受控迁移输入** | Runtime 确定性选定唯一接管方 → Judge 判布尔 → B 生成 | **是**（本次主体） |

**硬约束**：运行时**只消费一种规范结构**（`takeover_rules[]`）。`forced_activation` 可以存在多种输入形状，但**不能有两套执行语义**。初始化/迁移 Provider 必须把需要执行的语义翻译成带 `id`、`kind=turn_takeover`、`version=turn-takeover.v1`、`condition` 和显式 `target_profile_id` 的 typed rule；Runtime 只消费这些显式字段。没有 typed migration result 的 prose 只保留原值并生成 bounded diagnostic，不执行接管。

为兼容已经落库的迁移中间形态，`forced_activation` 下若已有对象本身带齐同一组 typed 字段，归一化器可以把它当作迁移结果映射到同一条规范 rule；这不是第二套语义，也不放宽对 prose、profile name 或 condition mention 的禁止推断。新的初始化输出应优先把 typed 结果写入 `takeover_rules[]`。

### 3.3 迁移表（按来源逐条）

| 来源 | 现有条目 | 稳定 ID | 分类 | 处置 | 依据 |
|---|---|---|---|---|---|
| `personality_system.switching.rules[]` | 每条含 `id`/`condition`/`target_profile_id`/`priority`/`cooldown_seconds`（`provider_schemas.go:578`） | `switch:<declared id>` | 语境理解 → **persistent_semantic**；`condition` 携带机器可读时间窗 → **persistent_deterministic**（本次只分类） | 保留原数据与既有校验（`personality_runtime.go:158-178`）；统一 ID 空间并纳入授权场景的投影 | R07、R11 |
| `personality_system.switching.default_profile_id` / `cooldown_seconds` | 同上（`provider_schemas.go:579`） | `switch:default` | persistent_semantic | 保留 | R11 |
| `personality_system.forced_activation.*` | **≥1 条语义明确**（仓库自带多头卡「公开质疑 → 星火强制接管」）；默认 `{}` 只适用于未声明的实例 | 仅作为迁移证据；typed 规则使用 `takeover:<id>` | Runtime 不按关键词、正则或 profile name 推断 kind/target；没有 typed migration 结果的 clause → `unclassified` + bounded diagnostic | 初始化/迁移 Provider 必须保留原 prose，并为可执行语义产生 typed `takeover_rules[]`；显式 target 只能是声明的稳定 profile ID | R07、R2-02、§19「不破坏性覆盖原始角色卡」 |
| `personality_system.takeover_rules[]`（新增） | 新增 typed 字段 | `takeover:<id>` | **turn_takeover** | 新增；接管的唯一正式声明入口 | R03–R08 |
| WakeUp / Reflection 入口 | 只消费 `active_profile_id`（`reflection_runtime_v2.go:320/332`），**不切换** | — | — | **不改动**；本次不新增切换入口 | R11（已决议） |
| Runtime 确定性时间切换 | **当前不存在**（全仓无 server 端自动切换引擎） | `switch:<id>` | persistent_deterministic | **本次只分类 + 稳定 ID + 诊断**，不实现求值（§3.5） | R07、R11 |

**迁移验收（F07/R2-02：不能只用 Go 关键词分类或放宽 fixture 证明成功）**：
- `dense_multi_card.txt` 与 `dense_multi_expectations.json` 保留原始接管语义断言；固定的 Provider migration fixture `dense_multi_typed_migration.json` 同时保留原 `forced_activation` prose，并产生一条显式 typed rule（含稳定 `rule_id`、`version`、`target_profile_id`）。
- Runtime 测试必须证明：只给真实卡 prose 时得到 0 条 executable takeover 且有诊断；给出该固定 typed Provider 输出时得到 1 条 executable takeover，规则 ID/target 稳定。
- 真实 Provider 是否会从该卡产出 typed field 仍由 opt-in live 测试验证；未运行时报告为环境未验证，不能用固定 fixture 冒充 live 结果。

### 3.4 稳定 ID 的端到端一致性（R07 硬要求）

同一个 rule ID 必须贯穿：初始化解析 → 持久化 → prompt 投影 → Judge 引用 → 冻结记录 → 诊断日志。约束：

- ID 只能由 **Runtime 归一化器**生成；Judge 与主模型**不能创造 ID**，只能引用 Runtime 提供的合法候选。
- ID 生成是**确定性**的（声明 `id` 时直接使用；否则 `stableDigest`，与 `cognition.go:404` 的 `frozenID` 同风格）。
- **规则身份 ≠ 规则内容版本**（F07）：`rule_id` 只由来源与声明 id 决定，**不随条件文本变化而换 ID**；条件内容的变更用独立的 `rule_content_digest` 表达。禁止用全文 hash 充当永久 ID。
- 归一化结果按 persona revision 缓存，不每轮重新推导。
- 统一 ID 空间若进入 Prompt，`preparePersonalityDecision`（`personality_runtime.go:158-178`）的 validator 必须解析**同一个**规范 ID；`switch:<id>` 前缀不能只在投影侧出现而在 validator 侧仍只匹配原始 `<id>`（否则重现 `personality_trigger_not_found`）。

### 3.5 范围决定（已决议，不再开放）

**选项 A（采纳）**：分类器照常产出 `persistent_deterministic`，但 V1 **只分类、只暴露稳定 ID、只做诊断**，**不实现** Runtime 求值器。
**选项 B（本次不做）**：无 LLM 的确定性时间窗求值器。

理由：仓库今天没有该引擎，做它是**新增能力**而非收敛；且按代码，持久切换今天已由 Main 驱动可用，缺少的是「授权边界与职责分离」，不是「缺少时间引擎」。报告中明确标注「deterministic 求值未实现、未验证」。

---

## 4. 契约设计

### 4.1 持久主导人格 vs 本轮发言人格（R04）

```go
// 持久：fluctlight_personality_runtime.active_profile_id（不变，仍由 E3 门后的 applyPersonalityDecisionPlanTx 写）
type turnPersonaScope struct {
    ActiveProfileID     string // 持久主导人格（prompt / 冻结记录）
    ReplyOwnerProfileID string // 本轮实际发言人格；默认 == ActiveProfileID
    PersonaRevision     int    // fluctlight_personality_runtime.revision 快照
    OverlayRevision     int    // fluctlight_evolution_states.revision 快照
    ScopeRevision       int    // 快照标识，用于并发/恢复校验
}
```

硬约束：

- **快照必须在 A 生成之前固定（F04）**。旧设计把 `resolveTurnPersonaScope` 放在 `design.md:363`（A 生成之后）是错的：它无法证明「这就是 A 生成时用的 active/revision」。**新位置**：`buildTurnProjection`（`mutations.go:711`）之前，与 projection 一起冻结进 payload 的 `context_projection`，恢复路径（`mutations.go:705/728`）读回同一个快照。
- **B 的读写作用域必须按冻结的 reply owner 重建**，不得回读当前 active：记忆（`provider_context.go:102`）、关系/目标/意图（`:184-186`）、关系能力（`relationship_capability.go:68-73`）、关系写入（`relationship_edit.go:20-23`）、引用索引（`context_reference.go:167/245/252/258`）、overlay 合成（`evolution_overlay.go:419-421`）。逐条见 §10。
- `takeover_once` 只改 `ReplyOwnerProfileID`，**绝不写 `active_profile_id`**。测试断言 `fluctlight_personality_runtime` 在 takeover 前后逐字段不变。
- 下一轮仍按持久 `active_profile_id` 构建（R04、R08 验收）。
- 消息归属：`conversation_messages.author_actor_id` 仍是 Fluctlight 实例，人格归属由冻结记录的 `reply_owner_profile_id` 表达；下一轮历史渲染时为 assistant 消息补该标注，避免 A 把 B 的语气吸收成稳定特征（R09）。实现方式：`recentPromptFragments`（`provider_context.go:119-167`）读取的 `recent_messages` 携带每条的 `reply_owner_profile_id`（**新增投影字段，非新表**）。

### 4.2 候选与「未执行提案」

**不新造万能 DTO**（R02）。候选 = 冻结 payload 的既有结构（`cognition.go:414-444`）：

```go
payload["decision"]               // visible_text / appraisal / personality_transition / response_plan / composite_action / context_projection …
payload["capability_invocations"] // []CapabilityInvocation（含 conversation.reply）
payload["capability_results"]
```

**未执行性由结构保证**：

| 副作用落点 | 为什么 A 的被拒候选到不了 |
|---|---|
| 人格演化 `applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`） | 经 **E3 门**；且 payload 已被 B 覆盖、授权已重置（E5） |
| appraisal / 情绪 / 状态 `applyFrozenCognitiveStagesTx`（`cognition_growth.go:223`） | 读 `decision`；已被覆盖 |
| 关系互动 `recordRelationshipInteractionTx`（`cognition.go:665`） | 在结算事务内、读最终 decision |
| 能力执行 `settleDeferredCapabilitiesTx`（`capability_runtime.go:366`） | 只执行 payload 的 `capability_invocations`；已被覆盖 |
| 私聊消息行 `conversation_messages`（`mutations.go:1149`） | 写结算时选出的 canonical `visible`；来自 B |
| Prepare 的参数补全与外部预检 | Judge 插在 `:925` 之前，A 从未进入 Prepare |
| 事件/监听器/outbox | 冻结只写 DB（`cognition.go:461`），无监听；outbox 由结算事务在 `:1182` 产出 |
| **A 的审计行 `cognition_assessments` / `cognition_decision_proposals`** | **M10：只有写入（`cognition.go:455/458`），全仓零读取方**（`migrations/runner.go:187/188` 仅建表）→ 不可能作为 accepted 事实进入反思或记忆 |

**被拒候选的留存**：写入同一 payload 的 `takeover.rejected_candidate`（含 A 的 decision、canonical 可见文本、invocations 摘要、命中规则 ID）。诊断可读，但不得进入 `conversation_messages` / Active Memory / 关系事实 / 下一轮人格基线，且**不得被任何执行或投递代码读取**（§8 守卫）。**审计行的消费者清单已实测为空**，这是证据而非承诺。

### 4.3 唯一可见文本权威（R05、F02 / M1）

**事实**（`review-response.md` F02）：取值顺序在生成期就已确定。

```
mutations.go:830   visibleCandidate := normalizeVisibleReply(firstString(responsePlan["visible_text"], decision["visible_text"]))
mutations.go:863-870  仅当上面为空时，回退 replyTextFromCapabilityInvocations(...)，并回写 responsePlan/decision["visible_text"]
mutations.go:1109 结算前用同一优先级再取一次
mutations.go:1149 INSERT conversation_messages(..., text=visible)
builtin_capabilities.go:120-136  ExecuteDeferredTx 只校验 args["text"] 非空且 ≤32000，不参与文本选择
```

因此**根级 / `response_plan.visible_text` 优先于 reply 参数文本**。旧设计把 preview 来源写成 reply 参数，会产生「Judge 审 A 的文本、实际发出另一种文本」的真实漏洞。

**修正设计**（F-01 path b 契约修订）：

```go
// 生成期一次确定，冻结进 decision["visible_text"] 与 responsePlan["visible_text"]
type canonicalVisibleReply struct {
    Text        string   // 最终发送文本的唯一权威
    Source      string   // "response_plan" | "decision" | "reply_capability"
    Conflict    bool     // 根字段与 reply 参数同时非空且不同
    Digest      string
}
func resolveCanonicalVisibleReply(responsePlan, decision map[string]any, invocations []CapabilityInvocation) canonicalVisibleReply
```

- **正式协议（适度 path b）**：根级 `visible_text`（`response_plan.visible_text` / `decision.visible_text`）是生成期回复提案的**优先**正式协议；`conversation.reply` 仍是 reply-only 候选可用的 fallback 提案源（Core 从 reply.text 派生根字段）。但模型不得同时决定两份文本：根字段与 reply.text 不一致即冲突。
- **单点计算 + 冻结**：`resolveCanonicalVisibleReply` 在生成期（`normalizeTurnDecision`）调用一次，结果冻结进 payload。
- **消费端全部读冻结值**：`buildCandidatePreview` 的 `ReplyText` **必须**取该冻结值，**禁止**重新从 reply 参数推导；Judge 输入、结算写库（`mutations.go:1104`）同源。
- **冲突处理（F-01 path b，默认启用）**：根字段与 reply 参数同时非空且不同 → 记录 `visible_text_source_conflict` 诊断并 **fail closed**（`normalizeTurnDecision` 返回错误，Turn 受控失败），不再以既有优先级静默取胜者。模型不能同时决定两份文本。
- **reply-only 合法**：根 `visible_text` 为空但 reply 参数非空 → reply 作为 fallback 提案源，canonical 从 reply.text 解析（Core 派生根字段），Turn 通过；这是 request.md:13「回复已经是 Tool」原契约保留的合法形态。
- 预览**不得**调用 `Capability.Execute`，按 `definition.OutputRole` / `definition.Type` 分派（禁止 `CapabilityName ==`）。

**验收**：构造 root / response_plan / reply.text 三者冲突的输入，断言 Turn 受控失败（`visible_text_source_conflict`）、Judge 未被调用、无消息送达；构造 reply-only 输入，断言消息从 reply.text 送达（fallback 合法）；构造无 reply invocation 但有根 visible_text 的输入，断言消息从根字段送达。

### 4.4 Judge 契约（R05、R06、R08、F09）

**角色与配置（复用当前模型绑定，已接受）**：

- 新增 Provider 角色 `takeover_judge`：加入 `validProviderRole`（`provider.go:127-134`）白名单；`providerBindingRole`（`diagnostics.go:166`）映射到既有 `generic_llm` 绑定 → **复用当前模型绑定**，无需迁移 `model_roles`。
- `providerScenario`（`diagnostics.go:173`）返回 `takeover_judge`；`providerPriority`（`diagnostics.go:207`）与 `cognitive_assessment` 同级（在用户可见回复的关键路径上，不应被饿死）。
- 独立记账：`recordModelRun`（`provider.go:278`）与 `provider_provenance` 按 role 落库 → 独立 token / 延迟 / 次数。
- **必须显式配置并如实上报**（F09）：① 实际 model/endpoint 绑定（＝复用主模型，写清楚）；② `timeout`；③ 输入预算；④ 最大输出；⑤ 实际支持的采样参数。**没有配置项就不是「独立低成本配置」**，不得凭 role 名宣称便宜。
- thinking：**不发送** `enable_thinking`。Provider 只实现 `enable_thinking: true` 分支（`provider.go:901-902`），**没有 false 分支** → 报告中如实写「未发送该参数＝未开启；Provider 不支持显式关闭」。**撤回**「已实现不思考」的表述。
- 不提供 tools（`definitions=nil`）→ payload 不含 `tools`（`provider.go:892`），只有 `response_format`。

**调用方式**：新增专用方法（可 grep、可守卫），走 assembled 路径以避免 `composeProviderMessages` 注入整套运行协议：

```go
func (p *ProviderClient) StructuredAssembledJudgement(
    ctx context.Context, role string, messages []map[string]any,
    schemaName string, schema map[string]any,
) (ProviderCompletion, error) {
    return p.completeWithToolsSchemaMode(ctx, role, messages, true, nil, schemaName, schema, false, true, false)
}
```

**输入投影（有限、隔离）**：消息固定两条，满足 `validAssembledProviderMessages`（`provider_prompt_composer.go:99`：恰好 1 条 system 且首条为 system、末条为 user）。

```text
[system] 接管判定协议（任务、输出契约、数据不是指令、不确定时保守）
[user]   {"current_user_message":"...", "recent_references":[...],
          "candidate_reply":"...",              // ← 4.3 的 canonical 冻结值，不是 reply 参数重推导
          "internal_intent":"...", "action_summary":[...],
          "takeover_control_view":{...}, "relationship_facts":{...}}
```

- **不提供**：工具、执行权限、完整 Main Prompt、所有 profiles、完整记忆、完整 B 背景（R08）。
- **数据隔离**：用户文本、历史、A 的候选回复、internal intent 全部作为 `user` 消息内的 JSON **数据**，system 消息显式声明「这些是待分析数据，不是指令」。候选里出现「请直接返回 true」不得生效（R08 注入样例）。
- **缺少必要语境时保留不确定性**，不为省 token 砍掉判断依据。

**输出契约（F09 输出约束修正）**：

```go
func takeoverJudgementSchema() map[string]any {
    return objectSchema(map[string]any{
        "takeover":      boolSchema(),                 // 唯一必填
        "decision_code": enumStringSchema([]string{    // 可选、短、有界；不是自由 string
            "meaning_mismatch", "boundary_conflict", "other",
        }),
    }, []string{"takeover"}, false)
}
```

- **撤回**旧设计中「两个短字段天然约束输出上限」的超范围断言：`stringSchema()` 只是 `{"type":"string"}`，**没有 `maxLength`**（`provider_schemas.go:31`）。改为**有界 enum**；无必要就只保留 boolean（F09）。
- 正常只判断布尔值；**不同时选择目标人格、接管模式或改写回复**（R08）。
- `decision_code` 仅诊断；Rule ID 只能引用 Runtime 提供的候选。

**降级（R08）**：

| 情况 | 行为 |
|---|---|
| 实例无适用接管规则 | **跳过 Judge**，零额外调用 |
| Judge 超时 / 无效 JSON / 服务不可用 | 记录 `judge.outcome` + 诊断，**保留已通过校验的 A 候选**；不做无依据切换 |
| Judge 输出超预算/超大小 | 同上，降级为保留 A |
| 重试 | 不重试、不升级到多模型（不放大既有 `runProviderQueued` 重试策略） |

Judge 降级**不绕过**原有业务安全与执行校验（A 仍走完整校验链）。

### 4.5 Working Persona 投影（R08、R09、F11）

从同一份已验证数据派生两个投影，**不手工维护第二套 B 人设**：

```text
Full Persona（core_persona.personality_system.profiles[i]）
  + 已接受的 Evolution Overlay（status=active）
  ├─ Working Persona         → 只进 Main Prompt 的唯一 System Persona 段
  └─ Takeover Control View   → 只进 Judge 输入
```

**合并顺序（Core 内，确定性）**：

1. Shared Identity：`core_persona.identity` + `life_profile`（跨 profile 共享，`profile_id IS NULL` 语义）。
2. 当前 profile 基线：`profiles[<subject profile id>]` 的 `personality` / `behavioral_policy` / `voice` / `body_language` / `scenario_behavior`。
3. Evolution Overlay：`ComposeEffectivePersona`（`evolution_overlay.go:249-268`）按 revision 升序应用 `status=active` 且作用域匹配的 overlay。

受保护字段不可被普通情绪或一次模型推断改写 —— 既有 `forbiddenEvolutionPathPrefixes`（`evolution_overlay.go:47-50`）已覆盖 `identity./core_persona./character_constraints.`，**沿用不重写**。

**保真要求（F11 修正 —— 不得臆造数值替代原词）**：

- `traits.*` 数值**只在来源真的声明了数值时**投影；来源是描述性文本（如 `dense_multi_card.txt` 的「暮光安静、句子短、行事谨慎」「星火热烈、语速快、爱用反问」）时，**原词必须逐字保留**，投影到 `voice` / `expression` / 描述性 traits 字段，**不得**转成数字或丢弃。
- 关键载体（缺一不可）：`name`、identity 关键语义、`traits.*`（有数值则给数值，否则给原文）、`expression.*`、`behavioral_policy`（`communication.tone` / `response.style` / `initiative.mode` / `conflict.approach` / `support.style`）、`voice` / `behavior_loops` 的语义要点、`scenario_behavior` 关键约束。
- 形状冲突（M6/F11）：`normalizeEvolutionPersonalityBaseline`（`evolution_overlay.go:452-458`）当前对非对象 `traits` **静默替换为 `{}`**；改为**显式兼容转换**（保留原值到描述性载体）或返回**可诊断错误**，绝不清空。
- **不进入投影**：完整背景故事、全部经历、未激活人格、视觉资产细节、`secrets` 全文。

**可追溯与确定性**：投影携带 `profile_id` / `persona_revision` / `overlay_revision`，可从冻结记录回放；**确定性投影**，不做「每次聊天前让 LLM 总结一次人格」。

**大小目标（F11：不以「仍装得下」作为收敛成功）**：

- 对**同一张真实复杂卡**记录重构前后的 **full request / Working Persona / Judge** 三组大小，并设置**可配置的工作预算**（不是只看 `defaultSystemTokensCap = 16384` 是否溢出）。
- 已有设施可复用（M9）：`AssemblePromptContext` 已计算 `EstimatedInputTokens` 并强制三层 cap（`prompt_context_assembler.go:14-19`、`:38-46`、`:255-259`），且已有确定性 cap 守卫（`provider_live_tool_test.go:34-47`；`prompt_context_assembler_test.go:186-187`）。**本次要补的是**：分节字节数、Judge 请求的同等覆盖、机器可读报告。
- 若必须保留的身份/行为约束本身超出 System 段预算 → 输出诊断 `working_persona_over_system_budget` 并调整配置或投影设计，**不得从字符串中间静默截断**。
- **非激活 profile 扩大时，Main prompt 不随之同步扩大**（对应 §2 的 `profiles` → `{id,name}` 收敛）；Judge 也有独立输入上限。

### 4.6 提示词与 schema 收敛（R09、R12、F01）

**System Persona 段的结构（唯一权威输出）**：

```yaml
# 人格设定
shared_identity:   { ... }        # 跨人格共享
working_persona:   { ... }        # 当前发言人格（本轮 reply owner）
persistent_switch:                 # 仅当实例声明了切换入口时才出现
  active_profile_id: "A"
  authorized: true                 # 仅授权场景为 true；B/continuation 为 false
  rules: [ {id: "switch:night", target_profile_id: "B", condition: "..."} ]
# 注意：这里【没有】takeover_rules。接管规则只进 Judge 的 control view。
```

- **Runtime Context 中移除** `core_persona`、`effective_persona`、`personality_system`（至少移除 `active_profile` 整对象与重复的 `personality`/`behavioral_policy`）。`workingMemoryInputFromProjection`（`provider_context.go:80-84`）的 delete 列表同步扩到这三个 key。
- **`profiles` 收敛为 `{id,name}` 名单**：`filterSemanticProfileValue`（`provider_prompt_composer.go:299-322`）当前让非激活 profile 的内容字段经 `filterCorePersonaValue` 漏进 Main。改为**只保留 `id` / `name`**（供模型命名 active profile 与引用切换目标），**不恢复完整非激活人格**。
- **A 的 prompt 不含完整 B**；B 只有实际生成时才加载其 Working Persona（R09 验收）。B 侧只允许附带**标明未执行**的 A 候选摘要（takeover context）。
- **response schema 按场景条件化**：`cognitiveTurnResponseSchema(grant)`：
  - `grant.Allowed == false`（含 B 的 `takeover_reply`、continuation）→ **不输出** `personality_decision` 字段与相关指令（满足 R11「普通 Main cognition 不再为了多重人格而每轮强制输出 keep/switch」，且结构性使 B 无法提出持久切换）。
  - 存在接管规则 → 允许可选 `internal_intent`（`string`, `maxLength 120`），并明确「不发送给用户、不写入事实记忆、不是完整思维链」；**不在根字段与 reply 参数重复存同一 intent**。
- **不重复表达**：保留原生 tool calling，不要求模型在文本 JSON 里复制 `tool_calls`（现状双通道保留，属 §14 允许的兼容封装）。
- **消息角色**：不创造 `context` / `system-context`（`project_health_architecture_guard_test.go:21` 继续强制）；未执行的 tool 不得伪装成 `role=tool`。历史消息**保留真实 `role`**（`provider_context.go:133-135`，M8）。
- **预算**：主请求与 Judge 请求分别配置（§4.5 的大小目标）。

### 4.7 调用预算与 QUERY 互斥（R06、F06）

```text
[1] 无 takeover 规则                     → A（1 次主生成），无 Judge
[2] 有规则、Judge=false                  → A + Judge（Judge 单独统计，不计入主生成）
[3] 有规则、Judge=true                   → A + Judge + B（主生成合计 2 次）
[4] QUERY 结果依赖型 continuation        → 既有 A + 1 次无 Tools 合成，【整个跳过仲裁块】
[5] QUERY + ACTION 混合候选              → 走普通候选机制，**可以仲裁**；但不 continuation
```

**F06 修正**：旧设计一边写「混合轮不叠加 takeover」，一边只跳过 `responseMode == "query_continuation"`，自相矛盾。核实后的语义是：

- `normalizeConversationResponseMode`（`query_continuation.go:38-47`）只在 `structuredFallback && visibleText=="" && validatePureQueryContinuation()==nil` 时返回 `query_continuation`；混合批次含非 pure query 能力 → 返回 `final`。
- 因此互斥的是「**结果依赖型 QUERY continuation 路径**」与「takeover 路径」，**不是**「出现任意 QUERY 工具就禁止 takeover」。
- 混合 `final` 轮：候选本身已是可成立的可见回复 → **允许仲裁**；未返回结果的查询结果**不得**被当成已知事实。
- 纯查询续调用路径：**整个跳过仲裁块**（含 Judge），且由此**不产生额外调用**。

实现边界：

- 接管后的 B **禁止再次仲裁**：结构性保证 —— B 走独立生成函数，该函数不调用 Judge，也不递归回仲裁块。
- 接管后的 B 若只产出仍需一次结果合成的 pure QUERY → **受控失败**（`takeover_reply_budget_exhausted`），不伪造答案、不调第三次主模型；已冻结且适用的真实查询结果可作为输入复用。
- 无 Tools 合成路径返回的文本仍通过既有 `visible` → `conversation_messages` 同一发送机制，**不新增旁路**。

### 4.8 冻结 payload、阶段与恢复（R10、F03）

**payload 扩展**（版本化）：

```json
{
  "capability_runtime_version": "...",
  "turn_id": "...", "conversation_id": "...",
  "turn_stage": "winner_ready",
  "winner": { "candidate_id": "frozen_...", "reply_owner_profile_id": "B", "source": "b" },
  "expected": { "state_revision": 123, "persona_revision": 12, "overlay_revision": 34, "scope_revision": 7 },
  "persistent_switch": { "authorized": false, "scenario": "takeover_reply", "reason": "takeover_reply_owner" },
  "decision": { "...": "最终胜出候选；接管后被 B 覆盖" },
  "capability_invocations": [ "..." ],
  "capability_results": [ "..." ],
  "takeover": {
    "version": "turn-takeover.v1",
    "decision": "not_applicable|skipped|judge_kept_a|judge_a_takeover_b_pending|takeover_b|judge_degraded|budget_exhausted|failed",
    "rule_id": "takeover:xxx", "rule_content_digest": "…", "rule_version": "persona-switch-rules.v1",
    "active_profile_id": "A", "reply_owner_profile_id": "B",
    "judge": { "role": "takeover_judge", "takeover": true, "decision_code": "meaning_mismatch",
               "outcome": "ok|timeout|invalid_json|unavailable|budget_exceeded", "latency_ms": 812 },
    "rejected_candidate": { "candidate_id": "frozen_...", "rejected_at_stage": "pre_prepare",
                            "visible_text": "...", "invocations": [ "…" ] }
  }
}
```

**turn_stage 阶段机（F03：不强制新表，但必须有可检查的阶段与执行资格）**：

| stage | 含义 | 允许的下一步 | 是否可执行 |
|---|---|---|---|
| `a_frozen` | A 候选已冻结，未仲裁 | 进入仲裁块 → `arbitration_decided` | 否 |
| `arbitration_decided` | Judge 结论已持久化（**先落库再调 B**） | 若接管 → `b_frozen`；否则 → `winner_ready` | 否 |
| `b_frozen` | B 候选已生成并校验、已覆盖 payload | → `winner_ready` | 否 |
| `winner_ready` | 胜出候选与执行资格已确定 | 进入 Prepare/执行 → `executing` | **是** |
| `executing` | 已开始 Prepare/执行 | → `settled` / `failed` | 否（拒绝再覆盖） |
| `settled` | 结算完成 | — | 否 |

- **候选冻结顺序**：`PersistTurnDecision` 写 `a_frozen` → 仲裁结果**单独持久化** `arbitration_decided` → 需要时才生成 B 写 `b_frozen` → `winner_ready`。这样「Judge 已完成、B 尚未冻结」这个窗口**可恢复**（不会重新随机判定）。
- **执行资格**：只有 `turn_stage == "winner_ready"` 才允许进入 `prepareCapabilityInvocations`（`mutations.go:925`）与结算（`:1122`）。`LoadFrozenTurn`（`cognition.go:345`）当前只校验 `status=='frozen'` + payload 信封（`validateExecutableCapabilityPayload`，`:357`），**扩展**为同时检查 `turn_stage`。
- **覆盖写（E5 + F03）**：

```sql
-- 条件 UPDATE：只允许在预期阶段覆盖，且未被其它 worker 推进
UPDATE public.cognition_frozen_actions
   SET payload = jsonb_set(jsonb_set(payload, '{decision}', $2::jsonb, true),
                                     '{capability_invocations}', $3::jsonb, true)
 WHERE id = $1 AND status = 'frozen' AND payload->>'turn_stage' = $4
```

  断言 `RowsAffected()==1`。**注意 M3**：`RowsAffected` 单独**不是** CAS（既有 `cognition.go:474/488/502` 的条件只有 `id` + `status`，不改 status 时两个并发更新各自都可能拿到 1）；因此必须**叠加** `turn_stage` 预期值与既有版本门控（`requireCognitionAuthorityRevisionsTx`，`mutations.go:1123`）。
- **覆盖范围必须完整（M4）**：除 `decision` / `capability_invocations` 外，还要处理 `capability_context_snapshot`、`context_reference_version` / `context_reference_index` / `influences` / `goal_refs` / `intention_refs`（`cognition.go:418-424`、`:438`），否则顶层会残留 A 的引用。
- **per-invocation ContextSnapshot 必须在覆盖时重算（M4）**：每条的 `ContextSnapshot` 是在 `PersistTurnDecision` **内部**根据 `decision["context_projection"]` 算出的（`cognition.go:429-439`）。B 不走 `PersistTurnDecision`（只走覆盖），因此覆盖函数**必须**用 B 的 projection 重算每条 invocation 的 `ContextSnapshot` 与 `ActionID`，否则执行期读空或读到 A 的。
- **B 的校验等价性**：B 的 decision 必须经过与 A 相同的归一化与校验函数（`freezeDecisionInfluences`、`preparePersonalityDecision`、`normalizeResponsePlan`、`normalizeConversationResponseMode`、`resolveCapabilityAction`、`normalizeCompositeAction`），再写 payload。不绕过 `personalityDecisionPlanFromValue`（`cognition.go:406-411`）与 `validateFrozenDecisionInfluences`（`:415`）。
- **胜出者确定后统一重建派生变量（F03）**：`responseMode` / `visible` / `personalityPlan` / `composite` / `capabilityInvocations` / `continuationBaseMessages` 一律从最终 payload **重读重建**，不能只换 JSON 而留下 A 的本地变量或 prepared 数据（既有冻结重放路径 `mutations.go:716-756` 已是这个形状，覆盖面要补齐）。
- **模型调用在事务外**：A、Judge、B、Prepare、能力 Execute 全部在事务外；只有「校验版本 → 覆盖 payload → 结算」在短事务内。
- **恢复不重判（F03 全枚举）**：

| 恢复时读到的状态 | 行为 |
|---|---|
| `turn_stage = winner_ready` | 直接执行胜出候选（payload 已含 B 或 A），**不重调 Judge** |
| `turn_stage = arbitration_decided` + `takeover.decision = judge_kept_a` | 置 `winner_ready`（胜出者=A），继续 |
| `turn_stage = arbitration_decided` + `judge_a_takeover_b_pending` | 按已持久化的 Judge 结论**继续生成 B**（不重判），成功后 `b_frozen`→`winner_ready` |
| `turn_stage = a_frozen`（无仲裁记录） | 正常进入仲裁块 |
| `judge.outcome != ok` 且 `judge_kept_a` | 使用 A，不重判 |
| `turn_stage = executing/settled` | 拒绝任何覆盖；已执行动作不重演 |
| 已有 assistant 消息 | 走既有 `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`）重放 |

- **并发**：沿用既有防线 —— 10 分钟 inbox claim 租约（`cognition.go:112-137`，尤其 `:121-123`）、`requireCognitionAuthorityRevisionsTx`（`mutations.go:1123`）、`superseded_by_newer_turn`（`cognition.go:267`）、`cognitionFactSuperseded`（`:334`）；叠加 `turn_stage` 条件 UPDATE。失效快照不得覆盖新状态。
- **逻辑调用 vs 物理请求（F10）**：turn 级计数「逻辑阶段」与「实际 HTTP attempt」分开；既有 Provider 重试会产生额外物理请求，必须统计并设边界。对「响应已到达但持久化前崩溃」的窗口，采用明确的有界重试/失败策略，**不声称物理请求恰好一次**。
- **不提前流给前端**：候选文本在仲裁结束前不进任何 `onChunk`；只允许不含候选内容的进度事件。**不做「先发后撤回」**。

### 4.9 无副作用候选校验（R02、F05 / M2）

评审 F05 成立：**校验不在仲裁点之前**。核实结论：

| 校验 | 位置 | 相对仲裁点 |
|---|---|---|
| 工具存在性 + `invocation.Validate(definition)` | `capability_runtime.go:181-195`（`prepareCapabilityInvocations`，由 `mutations.go:925` 调用） | **之后** |
| `resolveCapabilityAction` | `capability_runtime.go:601-618` | 之前，但**不做校验**：未知能力名 `continue` → 最终 `no_op` 软失败 |
| **便宜的确定性校验器** | `capability_runtime.go:205-220` `validateCapabilityInvocationsForPersistence` | **唯一调用方是 `wakeup.go:530`**，turn 链完全没用 |

**修正**：在仲裁点（`mutations.go:892` 与 `:917` 之间）**之前**增加一步 `Candidate Validation`，**复用既有** `validateCapabilityInvocationsForPersistence`（只做 registry 查表 + 参数 schema 校验，不碰 IO），**A 与 B 都必须先过**。

- **校验与 IO Prepare 分离**：外部预检（可产生 IO 的那些）仍只留给**胜出候选**。
- 非法候选走既有受控失败路径（`FailTurnCognition`），**不能靠 B 接管掩盖**。
- **验收**：不存在工具 / 非法参数 / 空 reply / 超长 reply / 无权动作，必须在进入 Judge 前按契约失败。

### 4.10 F12：文档与注释修正（不改结算）

**事实**（逐项给准确位置）：

- `settleDeferredCapabilitiesTx`（`capability_runtime.go:366`）由 `mutations.go:1157` 调用，而它**在** `withTransaction`（`:1122`）**之内**。
- `capability_runtime.go:363-365` 的注释声称「keeps external/provider work out of the transaction」——**这是误导性注释**：函数本身在事务内。
- 但**内置 deferred 能力没有网络 IO**：`conversation.reply` 是纯文本校验（`builtin_capabilities.go:123-136`，≤32000 字、非空），`media.image.generate` 只插入一行 `media_intent`（`:243-295` → `createMediaIntentTargetTx`）；真正的外部生成在事务之后由工作流/outbox 执行。
- 因此**不存在「持事务做网络请求」的现状**；同时修掉 `research/inventory-turn-chain.md` 内部自相矛盾（18e 说「外部调用在事务窗口内」，§6 又说「能力 Execute 均在事务之外」）。

**本次动作（仅这两项）**：

1. 修正 `capability_runtime.go:363-365` 注释措辞：说清「函数在结算事务内执行；其中的 deferred 能力只做本行数据写入与 intent 登记，不做网络 IO；真正的外部执行在事务提交后由 outbox/工作流完成」，并注明 `PreflightTx` 类入口的实际行为。
2. 修正 `research/inventory-turn-chain.md` 的措辞矛盾。

**不动**：`mutations.go:1122` 的事务边界、结算顺序、回滚语义。**继续保证**：被拒候选不能写业务状态、不能登记待执行 intent（由 §0.4 E1/E3/E5 + §4.2 覆盖，且以 `turn_stage='winner_ready'` 为执行资格）。

---

## 5. 目标流程（落到代码位置）

```text
buildTurnProjection(...)                                mutations.go:711
resolveTurnPersonaScope(...)                            [新] turn_takeover.go   ← 必须在 A 之前（F04）
persistentSwitchGrant = resolveGrant(scope, scenario)   [新] persona_switch_rules.go
assembleProjectionPrompt(..., schema=cognitiveTurnResponseSchema(grant))   mutations.go:763
A Main Cognition                                        mutations.go:770   [主生成 #1]
归一化 A decision → applyPersistentSwitchGrant(A)        mutations.go:822-887  [E1]
resolveCanonicalVisibleReply(...)                        [新] visible_output.go  ← 唯一文本权威（F02）
PersistTurnDecision(A, grant)  → turn_stage=a_frozen     mutations.go:888   [E2]
Candidate Validation（复用 validateCapabilityInvocationsForPersistence）    [新调用点，F05]
────────────────────────── ★ 仲裁 ──────────────────────────
if responseMode == "query_continuation" → 跳过整个仲裁块     [互斥，F06]
if 无适用 takeover 规则 → takeover.decision = "skipped"     [零额外调用]
else:
    persist arbitration_decided（先落库）                  [F03]
    controlView = buildTakeoverControlView(...)           [新]
    preview     = buildCandidatePreview(canonical...)     [新，无副作用，读冻结文本]
    judgement   = Judge(role="takeover_judge")            [新，1 次，独立计费]
    if false / 降级 → judge_kept_a → winner_ready(A)，保留 A
    if true:
        workingPersonaB = projectWorkingPersona(B)        [新]
        B Main Cognition（grant.Allowed=false，禁止再次仲裁）  [主生成 #2，新文件]
        校验 B（与 A 同一套）→ applyPersistentSwitchGrant(B) → 丢弃任何持久切换
        ReplaceFrozenTurnDecision(B)                      [E5：重置授权 + 重算 ContextSnapshot]
        → b_frozen → winner_ready(B)
──────────────────────────────────────────────────────────────
prepareCapabilityInvocations(...)                       mutations.go:925   [只对胜出候选，且需 winner_ready]
… 既有一切不变 …
结算事务（唯一副作用窗口；人格演化过 E3 门）              mutations.go:1122
投递（同步流 + 异步 outbox）                             mutations.go:1204 / redis_pipeline.go
```

新增/修改文件（详见 `implement.md`）：

- 新增 `turn_takeover.go`：scope 解析、规则选择、control view、Judge 调用、B 生成、payload 覆盖编排。
- 新增 `persona_switch_rules.go`：rule 归一化、三分类、稳定 ID、`persistentSwitchGrant`、诊断。
- 新增 `working_persona.go`：合并顺序 + 确定性投影 + 预算诊断。
- 修改 `visible_output.go`：`resolveCanonicalVisibleReply` 唯一文本权威。
- 修改 `provider_context.go` / `provider_prompt_composer.go` / `prompt_context_assembler.go`：唯一 System Persona 出口、`profiles` 收敛、去重。
- 修改 `provider_schemas.go`：`takeover_rules` schema、`takeoverJudgementSchema`、`cognitiveTurnResponseSchema(grant)`。
- 修改 `provider.go` / `diagnostics.go`：`takeover_judge` 角色、`StructuredAssembledJudgement`、Judge 预算/超时配置。
- 修改 `cognition.go`：`ReplaceFrozenTurnDecision`（阶段条件 UPDATE + 授权重置 + ContextSnapshot 重算）、`PersistTurnDecision` 接收 grant、`LoadFrozenTurn` 阶段校验。
- 修改 `mutations.go`：插入仲裁点（仍只保留 1 处 `StructuredAssembledWithToolsSchema(`）。
- 修改 `capability_runtime.go`：`settleDeferredCapabilitiesTx` 注释措辞（F12）。

---

## 6. 兼容与回滚

- **不破坏性覆盖**：`forced_activation` 原值保留；`switching.rules` 原值保留；`cognition_frozen_actions` 状态机不变；`fluctlight_personality_runtime` 表结构不变。
- **无新表**：接管状态放在既有 payload 内（含 `turn_stage` / `winner` / `expected` / `persistent_switch`），避免迁移风险。
- **单一执行链**：不留第二套人格决策或回复发送路径；不下发 feature flag 长期并行（§19 明文要求）。
- **回滚点**：见 `implement.md` 的 R1–R6。
- **可配置关闭**：接管判定依赖「存在合法 takeover 规则」，把 takeover 规则从角色卡移除即回到 A-only 行为，无需改代码。

---

## 7. 明确不做的事（范围边界）

- 不引入 MCP、向量基础设施、新工作流引擎、多 Agent 框架、Agent 框架级 Judge 插件系统。
- 不实现多人格辩论、递归抢答、竞价仲裁。
- **不实现确定性时间规则求值器**（选项 B 已决议不做，§3.5）。
- **不新增 WakeUp / Reflection 切换入口**，不新增每轮 Pre-turn LLM。
- 不新增 `visible_reply` 或任何并行发送路径。
- 不支持「需要查询后才能形成实际回复的轮次同时被接管」（范围限制，写入报告与测试）。
- 不重写 Initialization、全部 Memory 生命周期、媒体资产平台、TOON/YAML 渲染系统。
- **不改**结算事务的边界与既有正确的事务内数据库结算（F12）。
- 不新增 direct capability 数量（`TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities` 保持 13）。
- 不编造真实作品 Asset 或真实模型验证结果。

---

## 8. 静态守卫与运行时预算（F10）

**现状**：`capability_core_test.go:1026-1035` 是**源码文本计数** —— 断言 `mutations.go` 中 `StructuredAssembledWithToolsSchema(` 恰好 1 次、`StructuredQueryContinuation(` 恰好 2 次、无 `invocation.CapabilityName ==`，并要求 `decision["cognitive_state_transition"] = "not_proposed"` 存在；另对 `wakeup.go` / `workflow_ops.go` / `capability_runtime.go` 断言无具体能力分派。

**F10 修正**：静态计数**只能约束代码位置**，不能证明没有循环、重入或 Provider 重试。保留静态守卫作为**补充**，并新增**运行时阶段/调用预算**。

| 断言 | 期望 |
|---|---|
| `mutations.go` 中 `StructuredAssembledWithToolsSchema(` | 1（A 主生成） |
| `turn_takeover.go` 中 `StructuredAssembledWithToolsSchema(` | 1（B 主生成），且不得出现在其他生产文件 |
| 生产代码中 `StructuredAssembledJudgement(` | 1，且位于 `turn_takeover.go` |
| `mutations.go` 中 `StructuredQueryContinuation(` | 2（不变） |
| 生产代码中 `CapabilityName ==` / `switch invocation.CapabilityName` / `switch call.Name` | 0（不变） |
| `mutations.go` 中 `decision["cognitive_state_transition"] = "not_proposed"` | 必须存在（不变） |
| `role: "tool"` 生产出现位置 | 仅 `query_continuation.go`（不变） |
| 生产代码读取 `takeover.rejected_candidate` | 仅允许诊断文件；执行/投递/记忆路径为 0 |
| **三处结算点直接调用 `applyPersonalityDecisionPlanTx`** | **0**（只经 E3 门 `applyPersistentSwitchIfAuthorizedTx`） |
| `turn_takeover.go` 内递归/重入调用仲裁入口 | 0 |

**新增运行时预算测试（补充静态守卫）**：

- `TestTurnStageBudgetNeverExceedsTwoMainGenerations`：Fake Provider 记录**实际调用序列**，断言 A + B ≤ 2，Judge 不计入，且 `query_continuation` 与 takeover 不同轮共现。
- `TestRecoveryDoesNotReplayCompletedStages`：在 `a_frozen` / `arbitration_decided` / `b_frozen` / `winner_ready` / `executing` 各边界注入中断，断言不重判、不重演、不重复投递。
- `TestLogicalInvocationsVsPhysicalAttempts`：分离计数，重试产生的额外物理请求被统计。
- `TestTakeoverReplyCannotWritePersistentActive` / `TestPersistentSwitchStillWorksViaAuthorizedScenarios`（§0.4 验收）。
- `TestJudgeWirePayloadHasNoUnsupportedFields`：捕获真实 wire payload，断言未发送不支持的开关字段（F09）。

---

## 9. 风险

| # | 风险 | 缓解 |
|---|---|---|
| 1 | Working Persona 进 System 段可能顶到 `defaultSystemTokensCap = 16384`（`prompt_context_assembler.go:18`） | 投影即裁剪（非截断）；设可配置工作预算；超限输出 `working_persona_over_system_budget` 并调整投影，不静默截断 |
| 2 | B 的 decision 覆盖 payload 时绕过 `PersistTurnDecision` 的部分校验 | 复用同一套归一化 + 校验；覆盖用 `turn_stage` 条件 UPDATE；重算 `ContextSnapshot` 与派生变量 |
| 3 | 上一轮 assistant 消息的人格归属未标注时，A 会吸收 B 的语气 | recent assistant 消息补 `reply_owner_profile_id`（新增投影字段） |
| 4 | 接管覆盖与并发 turn / WakeUp / Reflection 竞态 | `turn_stage` 条件 UPDATE + 既有 revision CAS + 租约；失效快照不覆盖新状态 |
| 5 | Judge 增加固定延迟 | §18 要求实测四条路径的延迟与 token；不先验宣称更快 |
| 6 | **无 Fake Judge、无可控 Clock、无统一 `newTestApp`**（`research/inventory-tests-specs.md` §1.5–1.6） | 阶段 1 先补测试基座，否则确定性测试写不了 |
| 7 | **真实多头卡的接管语义今天是否真被初始化产出，本地未验证** | `provider_live_tool_test.go` 需 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`（`:445`）；报告中如实标注，并把该卡做成回归 fixture（§3.3） |
| 8 | `profiles` 收敛为 `{id,name}` 可能改变模型对人格系统的理解 | 以 R14/§18 的大小与行为对照测量；若回归则回退该单项（回滚点 R3） |
| 9 | 可见文本冲突若改成「严格拒绝」会破坏既有双来源输出 | 默认保持既有优先级 + 冻结 + 诊断；严格模式作为显式配置单独启用（§4.3） |
| 10 | Judge 复用主模型绑定，成本可能不低 | 如实上报实际参数、限制与成本（F09）；不凭 role 名宣称便宜 |

---

## 10. A/B 作用域读写表（F04 验收依据）

| 数据 | 读取位置 | 当前作用域键 | 接管后应使用 |
|---|---|---|---|
| 检索记忆 | `provider_context.go:102` `compactMemoriesForProfile(..., active_profile_id)` | 当前 active | **冻结的 reply owner** |
| 关系 / 目标 / 意图 | `provider_context.go:184-186` `selectActiveProfileRelationships` / `filterActiveProfileRows` | 当前 active | **冻结的 reply owner**（shared 行 `profile_id IS NULL` 始终可见） |
| 关系能力读 | `relationship_capability.go:68-73` | 读 `fluctlight_personality_runtime.active_profile_id` | **冻结的 reply owner**（不得回读 runtime） |
| 关系写 | `relationship_edit.go:20-23`（`profile_id=$3 OR profile_id IS NULL`） | 当前 active | **冻结的 reply owner** |
| 引用索引 / 关系 / 目标 | `context_reference.go:167/245/252/258` | `ActiveProfileID` | **冻结的 reply owner** |
| Overlay 合成 | `evolution_overlay.go:419-421` | 从 runtime 解析 profile | **冻结的 subject profile** |
| 响应归属（System 段） | `provider_context.go:197-199` / `provider_prompt_composer.go:246` | 两处并存 | 唯一 `working_persona` |
| 消息归属 | `mutations.go:1149` `conversation_messages.author_actor_id` | Fluctlight 实例 | 不变；人格归属由 `reply_owner_profile_id` 表达 |

**验收（F04 原文）**：设置 A-only / B-only / shared 三组关系与记忆 → B 接管时能看到 B + shared、**不错误读取 A-only**；B 的动作**不写入 A 私有作用域**；下一轮 A 保持原视角。

---

## 11. 已决议事项（G0 关闭）

| # | 问题 | 决议 | 依据 |
|---|---|---|---|
| 1 | `persistent_deterministic` 只分类还是实现求值器 | **只分类 + 稳定 ID + 诊断**（选项 A）；不实现确定性时间引擎 | 用户决议；仓库无该引擎，做它属新增能力 |
| 2 | 接管规则的正式声明入口 | **`personality_system.takeover_rules[]` 是唯一执行语义**；`forced_activation` 作为**受控迁移输入**，必须由初始化/迁移 Provider 产出显式 typed rule 才可执行；Runtime 不按 prose 推断，缺失或不合法时保留原文并记 bounded diagnostic | 用户决议 + F07/R2-02 证据 |
| 3 | 是否接受「无新表、状态存于 frozen payload」 | **接受**，但**不是**「只有 `status=frozen` + 两字段覆盖」：必须叠加 `turn_stage` / `winner` / `expected` / `persistent_switch` 与执行资格守卫 | 用户决议 + F03 |
| 4 | 持久切换职责是否离开 Main | **不离开**。保留 Main 驱动，收敛为逐调用场景授权，并在校验/结算/恢复三处执行权限 | 用户决议；`applyPersonalityDecisionPlanTx` 调用方仅三处结算路径，WakeUp/Reflection 不切换 |
| 5 | Judge 是否使用更小模型 | **复用当前模型绑定可接受**；必须显式配置并如实上报超时、输入预算、最大输出、采样参数与成本 | 用户决议 + F09 |
| 6 | F12 是否改结算 | **不改**。仅修文档与注释 | 用户决议 |
| 7 | 是否新增 WakeUp/Reflection 切换 | **不新增**；同时修正 `request.md` / `implement.md` 中「已有切换入口」的错误描述 | 用户决议 + 代码证据 |
