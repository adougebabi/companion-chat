# 人格层只读代码盘点（Persona / 多人格 takeover 上下文）

> 范围：仅人格层（profiles 定义与持久化、active profile、人格切换、forced_activation、Evolution Overlay、人格投影、System Persona vs Runtime Persona、traits、relationship/role、默认值来源、shared vs profile 作用域、时区/TOON 递归）。
> 方法：只读盘点。所有结论均带 `文件:行号`。区分「读到的」（直接证据）与「推断的」（基于代码的推导）。
> 仓库：`apps/core-go/internal/core` 与 `apps/core-go/internal/migrations`。

> **[2026-09-14 状态标注]** 本文件是**代码事实记录**。层级为 **request > 经评审的 `design.md` > `implement.md` > research 旧建议**。以下条目已被评审否决，**标 `superseded`**：
> - `superseded` §8.1 第 1 条「新增 takeover 应挂接 `preparePersonalityDecision` / `applyPersonalityDecisionPlanTx`」——**takeover 绝不写 persistent active**；只允许经授权的场景写（`design.md` §0.2/§0.4）。
> - `superseded` §8.2 第 3 条「takeover 若改写 `personality`，须同时维护 `core_persona.personality` + `effective_persona.personality` + `active_profile` 整对象」——与「唯一 Working Persona 出口」相反；改为**一个**由 Core 校验的 `working_persona` 投影（`design.md` §4.5/§4.6）。
> - **事实补充（R1 更正）**：本文件未指出的一点是，`testdata/initialization/dense_multi_expectations.json` 的断言 `id=forced` **要求产出的 `forced_activation` 含「公开质疑」**，且 `dense_multi_card.txt` 明确声明「遇到强烈的公开质疑时星火可强制接管」。因此「仓库内没有任何具体规则可迁移」的说法**不成立**：至少 1 条语义明确的规则来自仓库自带多头卡（详见 `design.md` §3.1/M6，`prd.md` 关键结论）。

---

## 1. 人格数据模型

### 1.1 顶层 Fluctlight 结构体
- `apps/core-go/internal/core/model.go:5-17` — `Fluctlight` 结构体。`CorePersona map[string]any`（json:"core_persona"）、`Identity`、`Personality`、`BehavioralPolicy`、`LifeProfile`、`Provenance` 均为 `map[string]any`；`Status string`、`CurrentRevision int`、`UnreadCount`、`LastConversationAt`。
- 说明：persona 全部以 `map[string]any`（JSONB）承载，没有强类型的 profile 结构体；profiles 是 `CorePersona.personality_system.profiles` 里的数组元素。

### 1.2 profiles 的真实定义
- profile 单条 schema：`provider_schemas.go:467-485`（`personalityProfile`），必填 `id`（`[]string{"id"}`，第 485 行）。字段含 `name/identity/personality/behavioral_policy/emotional_state/voice/body_language/behavior_state_machine/behavior_loops/scenario_behavior/secrets/intimacy_progression/output_preferences/fears/desires/extensions`。
- 完整已知字段契约：`app.go:533-539`（`personalityProfileFieldNames`）。
- 数组容器：`provider_schemas.go:488`（`"profiles": arraySchema(personalityProfile)`），置于 `personalitySystem`（`provider_schemas.go:486-499`）。

### 1.3 active profile 存哪里（持久化位置）
- 表 `public.fluctlight_personality_runtime`：`apps/core-go/internal/migrations/runner.go:172`
  `CREATE TABLE ... (fluctlight_id varchar(128) PRIMARY KEY, active_profile_id varchar(128) NOT NULL, previous_profile_id varchar(128), revision integer NOT NULL DEFAULT 0, switch_reason text NOT NULL DEFAULT '', switched_at timestamptz, cooldown_until timestamptz, updated_at ...)`
  - 每个 Fluctlight 一行（PK = fluctlight_id）。当前主导人格 = `active_profile_id`。
- 读取：`personality_runtime.go:43-65`（`readPersonalityRuntime`，SELECT active_profile_id, previous_profile_id, revision, switch_reason, switched_at, cooldown_until）。
- 写入/更新：`personality_runtime.go:236`（UPDATE，带 `revision` 乐观锁 CAS）与 `:244`（INSERT，行不存在时）。
- 初始写入：`app.go:1421`（`INSERT INTO public.fluctlight_personality_runtime ... VALUES($1,$2,0,$3)`，初始 `active_profile_id = initialPersonalityProfileID(corePersona)`）。

### 1.4 身份级（共享）personality 基线存哪里
- `fluctlights.core_persona` JSONB 内的 `personality_system.profiles[].personality` / `behavioral_policy` 是各 profile 的基线；
- evolution overlay 的「身份级 baseline」来自 `fluctlights.Personality` + 当前 profile 的 `personality`（`evolution_overlay.go:418-446`，见 §4）。
- evolution 专用表：`fluctlight_evolution_states`（`runner.go:1637`）、`fluctlight_evolution_overlays`（`runner.go:1647`）。

### 1.5 revision / 版本控制机制
- 切换版本：`fluctlight_personality_runtime.revision`，每次切换 +1（`personality_runtime.go:188-189` 计算 `newRevision`，`:236` 在 WHERE 中用 `revision=$7 AND active_profile_id=$8` 做 CAS，冲突返回 `personality_runtime_revision_conflict`）。
- Overlay 版本：`fluctlight_evolution_states.revision` + 每条 `fluctlight_evolution_overlays.base_revision/revision`；persist 用 `revision` CAS（`evolution_persistence.go:282-307`）。
- 关系版本：`relationships.revision` + `relationship_revisions`（`runner.go:216`）。

---

## 2. 人格切换入口清单

**唯一能改变 `active_profile_id` 的入口**：LLM 在 `cognitive_assessment` 返回的 `personality_decision`，经 `preparePersonalityDecision` → `applyPersonalityDecisionPlanTx` 持久化。

| 入口 | 文件:行号 | 触发条件 | 是否持久化 | 契约 |
|---|---|---|---|---|
| Cognition/Turn 的主路径 | `mutations.go:808-809`（`preparePersonalityDecision`）、`:956`、`:1126`、`:1303`（`applyPersonalityDecisionPlanTx`） | LLM `personality_decision.decision="switch"` + `from_profile_id` + `target_profile_id` + `trigger_id` | 是，UPDATE `fluctlight_personality_runtime`（带 revision CAS） | `personalityDecisionPlanVersion="fluctlight.personality-decision-plan.v1"`（`personality_runtime.go:67`）；schema `provider_schemas.go:200-209` |
| frozen turn 路径 | `cognition.go:409`（`personality_decision_plan_invalid` 校验） | 同上 | 是 | 同上 |
| 初始化（创建实例） | `app.go:1421` | 新建 Fluctlight，写入初始 runtime 行 | 是（INSERT） | `active_profile_id = initialPersonalityProfileID(...)`（`app.go:1547-1558`） |
| WakeUp | 消费 `projection.ReferenceIndex.ActiveProfileID`（`reflection_runtime_v2.go:320,332`） | **不切换**，只读当前主导人格 | 否 | — |
| Reflection | 消费 `ActiveProfileID`（`reflection_runtime_v2.go:320,332`） | **不切换** | 否 | — |
| 时间规则 / switching.rules | 由 LLM 在 cognition 中自行判断触发（`provider_prompt_composer.go:20`：「服务器只校验并保存你的结构化决定，不根据切换条件自行推导人格」） | 无 server 端 cron 强制切换（grep 全仓无 autonomous switch 调度） | — | — |

- 切换校验核心：`preparePersonalityDecision`（`personality_runtime.go:83-201`）：
  - target 必须声明于 `profiles`（`personality_runtime.go:146-157`，否则 `personality_target_profile_not_found`）；
  - cooldown 检查（`:143-145`，`personality_switch_cooldown`）；
  - **trigger_id 必须匹配 `switching.rules` 或 `switching` 列表中的某个 `id`**（`:158-178`，否则 `personality_trigger_not_found`；switch 必须有 trigger，`:182-184` `personality_trigger_required`）；
  - revision CAS（`:215-224` `personalityDecisionPlanFromValue`）。
- 结论（读到）：人格切换是**单一点 + LLM 决策驱动**，WakeUp/Reflection 只消费 `ActiveProfileID`，不主动切换；时间规则没有 server 端自动切换引擎。

---

## 3. forced_activation 全链路

- **定义位置**：`provider_schemas.go:497` —— `"forced_activation": openObjectSchema()`（在 `personalitySystem` 内，紧邻 `core_relationship`/`core_conflict`）。
- **schema 形状**：`openObjectSchema()` —— 完全开放，**无任何内部字段 / 子结构校验**。对比 `switching` 有完整 rule schema（`personalitySwitchingSchema`，`provider_schemas.go:577-580`，含 `id/condition/target_profile_id/cooldown_seconds` 等必填）。
- **写入位置**：
  - 初始化必含键校验：`app.go:657`（`hasInitializationKeys` 要求 `forced_activation`）、`app.go:874`（knownPersona 白名单）、`app.go:1109-1113`（缺失则补默认值）。
  - 默认值：`app.go:1251` —— `defaultPersonalitySystem` 中 `"forced_activation": map[string]any{}`。
  - 即：只保证「这个键存在、是个 object」，内容完全不校验。
- **读取 / 校验位置**：**无**。全仓无任何针对 `forced_activation` 内部结构（如 trigger、target_profile）的读取、校验或消费代码。与之相对的 `switching.rules` 有 trigger 校验（`personality_runtime.go:158-178`）。
- **谁消费它**：**没有任何运行时代码消费其语义**。
  - prompt composer 指令只把 `profiles/switching/influence/conflict_resolution/integration` 列为判断输入（`provider_prompt_composer.go:20` 与 `:268` 投影分支），**不包含 `forced_activation`**；
  - `compactPersonalitySystem`（`provider_context.go:289-315`）仅 `cloneMap(system)` 透传整个 `personality_system`（含 `forced_activation`），但无任何逻辑读取它，不影响决策。
- **稳定 rule ID**：**无**。`forced_activation` 内没有 `id`/`rule` 概念；`switching.rules` 才有 `id`（`provider_schemas.go:578`）。
- **validator 是否识别**：**否**。它只是透传的开放对象。
- **搜索过的关键词**：`forced_activation`、`ForcedActivation`、`forced_activation`；覆盖 `internal/core` 与 `internal/migrations`。命中点仅为：`provider_schemas.go:497`、`app.go:657/874/1109/1251`、`initialization_contract_test.go`、`testdata/initialization/dense_multi_expectations.json`（测试与契约断言），**无任何非测试运行时代码消费**。
- **结论**：线索①「forced_activation 与 trigger 校验脱节」→ **成立**（更精确地说：forced_activation 压根没有结构，而 switching 才有 trigger 校验；两者之间没有连接代码。它是与切换机制脱节的「死字段」）。
- **[2026-09-14 补充，R1]** 上面的「无消费者」结论只说明**运行时**不读它，**不等于**「没有规则可迁移」。实测补充：
  - `testdata/initialization/dense_multi_card.txt`（300 字，**纯散文**）原文声明：「**遇到强烈的公开质疑时星火可强制接管；收到 actor_user 明确的安全确认后暮光可重新主导。**」
  - `testdata/initialization/dense_multi_expectations.json` 断言 `id=forced`：`path = core_persona.personality_system.forced_activation`、`contains = "公开质疑"`、`basis = "explicit"` → **初始化期望要求产出该内容**。
  - 该断言的**唯一校验路径是 live provider**：`provider_live_tool_test.go:21-23` → `liveProviderInitializationCase`（`:50`）→ `EvaluateInitializationCoverage`；而 `liveProviderConfig`（`:443-447`）在未设 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 时 `t.Skip` → **本地未验证**。
  - 结论修正：`forced_activation` 是「**无结构、无 ID、无校验、无消费者，但仓库自带卡确实有 1 条语义明确的接管声明**」；因此 R07 不能让语义明确的旧规则只落诊断（否则真实旧卡迁移后永不接管）。

---

## 4. Evolution Overlay 全链路

### 4.1 数据结构
- `EvolutionOverlay`（`evolution_overlay.go:62-86`）：`ID, Ref, FluctlightID, ProfileID, Kind(personality|behavior_policy), FieldPath, ValueKind(numeric|categorical), SemanticDirection, RequestedDelta, AppliedDelta, BeforeValue, AfterValue, Confidence, EvidenceRefs, EvidenceWindows, PolicyVersion, BaseRevision, Revision, Status(active|superseded), Supersedes, RollbackOf, CooldownUntil, CreatedAt`。
- 类型常量：`evolution_overlay.go:15-50`（kind / valueKind / status / 禁止前缀）。

### 4.2 合并顺序（共享身份 → 当前 profile → overlay）
- 基线构建 `personaEvolutionBaseline`（`evolution_overlay.go:418-446`）：
  1. 先取 `fluctlight.Personality`（身份级 / 共享 baseline，`fallbackPersonality`）与 `fluctlight.BehavioralPolicy`；
  2. 再按 `active_profile_id` 覆盖 `profile["personality"]` / `profile["behavioral_policy"]`（`:428-440`）；
  3. 最后 `ComposeEffectivePersona`（`evolution_overlay.go:249-268`）按 `revision` 升序把 `status=active` 且 `ProfileID`/`FluctlightID` 匹配的 overlay 应用到 `personality` / `behavioral_policy`（`:257-266`）。
- 加载：`loadPersonaEvolutionState`（`evolution_persistence.go:314-348`）从 `fluctlight_evolution_states` + `fluctlight_evolution_overlays`（按 revision 排序）载入，再交给 `ComposeEffectivePersona`。

### 4.3 允许覆盖哪些字段
- 数值（personality）：`personalityEvolutionNumericPaths`（`evolution_overlay.go:36-40`）—— `traits.openness/conscientiousness/extraversion/agreeableness/emotional_stability`、`expression.warmth/initiative/playfulness`。
- 类别（behavior_policy）：`behaviorEvolutionCategoricalPaths`（`evolution_overlay.go:42-45`）—— `communication.tone/response.style/initiative.mode/conflict.approach/support.style`。
- 禁止前缀：`forbiddenEvolutionPathPrefixes`（`evolution_overlay.go:47-50`）—— `identity./core_persona./owner./permissions./security./provider./infrastructure./character_constraints./personality_system.active_profile_id`。
- 校验：`CompileEvolutionOverlay`（`evolution_overlay.go:132-219`）做路径白名单、置信度阈值、跨窗口证据、cooldown、数值夹紧（`:194` `MaxNumericDelta=0.1`）。

### 4.4 revision 追溯
- `state.Revision` + 每条 overlay 的 `BaseRevision`/`Revision`（`evolution_overlay.go:209,215`）；
- persist CAS：`evolution_persistence.go:286-307`（先 UPDATE 旧 active overlay 为 superseded，再 UPDATE `fluctlight_evolution_states.revision`）；
- rollback：`RollbackEvolutionOverlay`（`evolution_overlay.go:283-320`，`RollbackOf` 指向原 overlay）。

### 4.5 投影到 prompt 的出口
- `readEffectivePersonaProjection`（`evolution_overlay.go:391-416`）→ `compactEffectivePersonaForProvider`（`provider_context.go:279-287`）→ 输出 `effective_persona: {profile_ref, authority_revision, personality(traits.*), behavioral_policy}`。
- 在 `compactCognitionContext` 中与 `core_persona`、`personality_system` 一同进入模型上下文（`provider_context.go:187-199`）。

---

## 5. Persona 投影（System Persona vs Runtime effective_persona）

存在**三处**互相独立的人格序列化出口：

1. **System Persona（core_persona）**：`compactCorePersona`（`provider_context.go:472-504`）—— 投影 `core_persona.identity / personality / behavioral_policy`。其中 `personality` 是**扁平键**形态（`openness/conscientiousness/...`，见 `app.go:1255` `defaultPersonality`、契约 `app.go:632`）。
2. **Runtime effective_persona**：`compactEffectivePersonaForProvider`（`provider_context.go:279-287`）—— `effective_persona.personality` 是 **`traits.openness` 嵌套**形态（`evolution_overlay.go:454` `normalizeEvolutionPersonalityBaseline`）。
3. **personality_system.active_profile**：`compactPersonalitySystem`（`provider_context.go:289-315`，`:303` 把当前 profile 整对象作为 `active_profile` 放入）。

- 结论（读到）：**线索③「System Persona 与 Runtime effective_persona 存在重复」→ 成立**。两者同时进入 prompt（`provider_context.go:187-199`），且对同一底层人格数据给出**两种形状**（扁平 vs `traits.*` 嵌套）+ 再加 `active_profile` 整对象，模型需自行对齐，存在矛盾/重复风险。

---

## 6. 作用域审计：shared vs profile

### 6.1 关系（relationship）
- 播种：`insertRelationshipSeeds`（`app.go:1753-1805`）。
- 作用域决定：`app.go:1761` —— `profileID, _ := normalizeProfileID(stringValue(item["profile_id"]), defaultProfileID)`。
- `defaultProfileID` 来源：`app.go:1437` —— `defaultProfileID := initialPersonalityProfileID(corePersona)`，即声明的 `active_profile_id`（或首 profile 或 `"default"`，见 `app.go:1547-1558`）。
- 持久化：`app.go:1797` INSERT `relationships(...,profile_id,...)` 用上述 `profileID`（非 NULL）。
- 表结构：`relationships.profile_id varchar(128)` **可空**（`runner.go:279`）。读取端把 `profile_id IS NULL` 视为「共享/兜底」：`relationship_edit.go:23`（`(profile_id=$3 OR profile_id IS NULL)`）、`provider_context.go:333-339`（`fallbackRelationship` 用空 profile_id）。

### 6.2 目标 / 意图（goal / intention）
- `insertAgency`（`app.go:1608-1700`）：goal `profile_id`（`app.go:1614`）、intention `profile_id`（`app.go:1662`）同样回退到 `defaultProfileID`（初始主导 profile）。
- 表结构：`fluctlight_goals.profile_id varchar(128)`（`runner.go:286`），`scope` 字段（`general`/`relationship`，`app.go:1618-1621`）。

### 6.3 推断与结论
- 「shared」在存储模型里用 **`profile_id IS NULL`** 表示；但初始化路径在 `profile_id` 缺省时落到 `initialPersonalityProfileID`（声明的初始 profile），**不会**写 NULL（`normalizeProfileID` 把空串归为 `defaultProfileID`，而非 NULL：`personality_runtime.go:32-41`）。
- 结论（读到 + 推断）：**线索④「共享关系和目标误绑到初始 profile，而不是 shared 作用域」→ 成立**。未声明 `profile_id` 的关系/目标/意图被私有化绑定到声明的初始主导 profile；若设计意图是「未声明即共享」，则当前实现与之相悖（除非调用方显式传 `profile_id: null`，而初始化契约并不鼓励这么做）。

---

## 7. 第 16 节线索核实表

| 线索 | 判定 | 证据（文件:行号） |
|---|---|---|
| traits 形状（数组被转换成空对象 / 形状冲突） | **成立（已在阶段 8 修复）** | `evolution_overlay.go:454-458` `normalizeEvolutionPersonalityBaseline`：若 `traits`/`expression` 非对象（如数组），`mapValue` 返回空 map（`app.go:167-172`），于是 `result[root] = map[string]any{}` —— 数组被静默替换为 `{}`。另：`core_persona.personality` 用扁平键（`app.go:1255,632`），`effective_persona.personality` 用 `traits.*` 嵌套（`evolution_overlay.go:36-40,454`），两处形状不一致。**阶段 8 修复**：非对象值改走描述性载体逐字保留（F11）；Working Persona 出口统一走同一 reconciler，把扁平 trait 补进规范 `traits.*`（加法，不删扁平键），空载体再剔除。 |
| relationship.role 归一化丢失 | **成立（已在阶段 8 修复）** | `normalizeRelationshipRole`（`relationship_edit.go:35-56`）只对 map 输入产出 `{label,addressing}`；当 `role` 是字符串且无可识别 `type` 时，`app.go:947-955` 把原值替换为 `{label:"unknown", addressing:{}}`（原 label 丢失）；若以字符串持久化，读回 `decodeObject(role)`（`app.go:154-158`，非对象 JSON 解析失败→`{}`）会丢失 role。**阶段 8 修复**：`normalizeInitializationAliases` 把原 `role` 字符串作为首个候选，逐字保留为 `role.label`。 |
| 无来源 0.5 默认值 | **成立（已在阶段 8 修复）** | 目标 `importance/urgency` 默认 0.5：`app.go:907-908`、`app.go:1641-1642`；意图 `confidence` 默认 0.5：`app.go:1688`；人格默认值全 0.5：`app.go:1255` `defaultPersonality`；`defaultPolicy` `initiative=0.5`：`app.go:1260`。均为无 evidence 来源的兜底。**阶段 8 修复**：`recordInitializationDefaultFieldSources` 把未被来源声明的数值字段按稳定字段路径写入 `provenance.field_sources = "server_default"`，使默认值可区分于用户提供 / 模型推导。 |
| shared / profile 作用域 | **成立（已在阶段 8 修复）** | 见 §6：`app.go:1437,1761,1614,1662` 把缺省 `profile_id` 绑到 `initialPersonalityProfileID`；表 `relationships.profile_id`/`fluctlight_goals.profile_id` 可空（`runner.go:279,286`）但初始化不写 NULL。**阶段 8 修复**：新增 `initializationScopeProfileID`，缺省解析为共享作用域（写 SQL NULL），显式声明仍走 membership 校验；`GoalAuthority/IntentionAuthority.Validate` 允许空 ProfileID 表示共享。 |
| 时区（历史时间戳） | **成立（R1 修正）** | `provider_context.go:626-629` `parsed.UTC().Format("01-02 15:04:05")`：**强制 UTC 且无时区标识**，13:27+08:00 的消息渲染成无标记的 "05:27"。已在阶段 8 修复为 `01-02 15:04:05Z07:00`（保留偏移并显式标注）。 |
| TOON 递归 | **尚未充分复核** | 此前以 `provider_prompt_format.go:232` `providerTOONFields`、`:121` `renderProviderYAMLValueWithMode` 的「扁平表格渲染」推翻它，**判据不充分**（评审 F14.1 的同类错误）。需先取得递归调用链证据再下结论；本次不声称成立或不成立。 |
| assistant 历史自我举证 | **部分成立（阶段 8 补窄断言，不改渲染）** | `recentPromptFragments`（`provider_context.go:142-151`）与 `compactRecentMessagesForActors`（`provider_context.go:605-607`）把 `kind=="assistant"` 消息归属到 fluctlight actor（actor 行存在时渲染为 display_name）。**渲染风格是设计意图**，由既有测试 `TestCompactRecentMessagesUsesActorUserAndFluctlightDisplayName` 钉死，阶段 8 不改代码；改为补窄断言 `TestRecentHistoryNeverAttributesSelfUtteranceToTheUser`：自身历史永不归属为 `actor_user`，且保留 `actor_type=fluctlight` 与时间戳，使「曾经说过」始终可辨识为**发言**而非「作品/动作/外部事实已存在」。 |

另有 5 条早期「背景线索」核实：
- ① forced_activation 与 trigger 校验脱节 → **成立**（§3，forced_activation 是死字段，无 trigger 校验）。
- ② traits 数组被转换成空对象 → **部分成立**（§7 上表，数组→`{}` 仅发生在 evolution baseline 的非对象输入，且两 persona 出口形状不一致）。
- ③ System Persona 与 Runtime effective_persona 重复 → **成立**（§5）。
- ④ 共享关系/目标误绑初始 profile → **成立**（§6）。
- ⑤ relationship.role 归一化丢失 → **部分成立**（§7 上表）。

---

## 8. 对本次重构（takeover 逻辑）的约束

### 8.1 可复用的基础（稳定、低耦合）
- **单一干净切换入口**：`preparePersonalityDecision` + `applyPersonalityDecisionPlanTx`（`personality_runtime.go:83,227`），带 `revision` CAS，已是全仓唯一改变 `active_profile_id` 的路径。**[superseded]** ~~新增 takeover 应挂接这里而非另起通道。~~ → 替代设计：takeover **不写** `active_profile_id`；该入口只由**授权的持久切换场景**调用，并经结算权限门（`design.md` §0.4 E3）。
- **稳定的 overlay 合并层**：`PersonaEvolutionState` + `ComposeEffectivePersona`（`evolution_overlay.go:249-268`）与 `fluctlight_evolution_states/overlays` 表，提供版本可追溯的「共享身份→profile→overlay」合并。
- **role 规范化锚点**：`normalizeRelationshipRole`（`relationship_edit.go:35`）与 `compactRelationship`（`provider_context.go:356`）是关系 role 的唯一规范出口。

### 8.2 耦合风险（新增 takeover 逻辑需警惕）
1. **与 LLM 决策竞态**：切换目前仅由 LLM `personality_decision` 触发（§2），无 server 端强制接管。若 takeover 想「强制接管」，必须与 LLM 决策做互斥/仲裁，否则出现双写竞态（`revision` CAS 会拒绝旧 plan，但需明确谁优先）。
2. **forced_activation 是天然落点但当前是死字段**：可把 forced_activation 作为 takeover 触发器来源，但**必须先定义其 schema 与校验**，并接入 `personality_runtime.go:158-178` 的 trigger 校验（目前该处只认 `switching.rules` 的 `id`）。
3. **[superseded]** **双 persona 出口形状不一致（§5）**：~~takeover 若改写 `personality`，须**同时维护** `core_persona.personality`（扁平）与 `effective_persona.personality`（`traits.*` 嵌套）+ `active_profile` 整对象，否则模型看到矛盾人格。~~ → 替代设计：**只保留一个**由 Core 校验的 `working_persona` 投影，Runtime Context 移除 `core_persona` / `effective_persona` / `personality_system`；形状冲突由 `normalizeEvolutionPersonalityBaseline` 显式兼容转换解决，不再静默变 `{}`（`design.md` §4.5/§4.6，F11）。
4. **作用域陷阱（§6）**：涉及「共享 vs profile」数据时，takeover 必须**显式设 `profile_id` 或 NULL**；依赖缺省会落入 `initialPersonalityProfileID`（初始 profile），把本应共享的数据私有化。
5. **role 写入形状（§7）**：takeover 写入 `relationships.role` 必须用 `{label, addressing}` 对象形；传字符串会在 `app.go:947-955` 被替换为 `{label:"unknown"}` 而丢失原值。

---

### 附：本次未改动任何代码（只读）。所有引用均为当前分支实际存在的符号与行号（基于 2026-09-14 工作树）。
