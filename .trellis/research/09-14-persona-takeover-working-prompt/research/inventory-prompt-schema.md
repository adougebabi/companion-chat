# 盘点：Prompt Context Assembler 与 Main Response Schema

> 范围：只读代码盘点，覆盖 Prompt 组装调用链、消息布局、Main `cognitive_assessment` 的 response schema、工具协议、预算/大小统计、重复序列化审计、provider 契约、对本次重构的约束。
> 所有结论均带 `文件:行号`。区分「读到的」（代码中明确存在）与「推断的」（由代码形态推导，需二次确认）。
> 未修改任何代码文件。

> **[2026-09-14 状态标注]** 本文件是**代码事实记录**，层级为 **request > 经评审的 `design.md` > `implement.md` > research 旧建议**：其中的「对本次重构的约束 / 建议」不是架构权威，实施以 `design.md` / `implement.md` 为准。两处已更正：
> - `superseded` §2 消息布局表原写「`2..n` = `user`（recent）」——实际 recent **保留真实 `role`（user/assistant）**（`provider_context.go:133-135/164`；`prompt_context_assembler.go:370-390`）；已就地修正。
> - §5 的结论**不要**读成「全仓没有请求大小设施」：已有 token 估算 + 三层 cap + `PromptAssemblyTrace` + 确定性 cap 守卫；缺的只是分节字节数、Judge 请求覆盖与机器可读报告（详见 §5 末注）。

---

## 1. Prompt 组装调用链（从入口到最终 messages 数组）

主交互主路径入口：`apps/core-go/internal/core/mutations.go:757-770`（`buildTurnProjection` → `assembleProjectionPrompt` → `StructuredAssembledWithToolsSchema`）。

逐层：

1. **`mutations.go:763`** `a.assembleProjectionPrompt(ctx, projection, "cognitive_assessment", []string{providerContextAuthorityRule, capabilityConversationPolicyInstruction}, text, definitions, "conversation_turn_response", schema)`
   - `schema = cognitiveTurnResponseSchema()`（`mutations.go:762`）
   - `definitions = capabilityCatalog(..., CapabilitySurfaceConversation)`（`mutations.go:761`）
   - 这是触发本次认知的唯一业务入口。

2. **`provider_context.go:14`** `assembleProjectionPrompt(...)`
   - `provider_context.go:24-31` 取 ActiveMemory（`retrieveActiveMemories`）
   - `provider_context.go:45-53` 取 ConversationSummary（`retrieveConversationSummaries`）
   - `provider_context.go:55` `workingMemoryInputFromProjection(projection, activeResult.Items, summaries)` → 由 `compactCognitionContext`（provider_context.go:182）投影出三层上下文，再 `ResolveWorkingMemory`（`provider_context.go:56`）解析为 `WorkingMemory{Active, RuntimeFacts, Recent, Retrieved, Summaries}`。
   - `provider_context.go:60-63` 由 `assignment`（provider 分配）构建 `PromptBudgetPolicy`：`DefaultPromptBudgetPolicy(assignment.TokenBudget)` 并覆盖 `ContextWindowTokens / MaxInputTokens / Version`。
   - `provider_context.go:64-68` 调用 `AssemblePromptContext(PromptAssemblyInput{..., Tools: RenderCapabilityTools(definitions), ResponseFormat: providerResponseFormatForSchema(role, schemaName, schema), Policy: policy})`。
   - `provider_context.go:70-76` 把 `PromptAssemblyTrace`（预算/section 统计）塞进 `result.Diagnostics`。

3. **`prompt_context_assembler.go:241`** `AssemblePromptContext(input)`
   - `prompt_context_assembler.go:249` 渲染 system：`renderProviderSystem(input.OperationRules, filterCorePersona(input.CorePersona), nil, input.Role)`（CorePersona 经 `filterCorePersona` 去 ID/元数据后只在 System 出现一次）。
   - `prompt_context_assembler.go:250` 渲染 current：`{role:"user", content: input.CurrentInput}`。
   - `prompt_context_assembler.go:251-261` 预算前置检查：`SystemTokensCap` / `CurrentInputTokensCap` / `ToolsSchemaTokensCap`，以及 `requiredTokens = system+current+tools+schema+16` 是否超 `MaxInputTokens`。
   - `prompt_context_assembler.go:262` `promptOptionalCandidates(...)` 把 WorkingMemory 分组排序成候选单元。
   - `prompt_context_assembler.go:274-302` 按 `MaxInputTokens` 贪心选取候选单元（整 unit 丢弃）。
   - `prompt_context_assembler.go:303` `assemblePromptMessages(system, current, selected)` → 生成最终 `[]map[string]any`。
   - `prompt_context_assembler.go:304-319` `estimatePromptWireInput(messages, tools, responseFormat)` 线长复核，必要时从尾部再丢弃。

4. **`prompt_context_assembler.go:370`** `assemblePromptMessages(...)`
   - system 原样放入；
   - 若 `runtimeContext` 非空，插入一条 `{role:"user", content:"[RUNTIME CONTEXT]\n"+jsonString(runtimeContext)+"\n[/RUNTIME CONTEXT]"}`（`prompt_context_assembler.go:389-391`）；
   - 追加 recent（`PromptFragmentRecentMessage`）；
   - 追加 current。
   - `renderProviderSystem` 实现见 **`provider_prompt_composer.go:156-188`**（写入 `# 运行协议` / `operation_rules` / `# Actor 与关系上下文` / `# 人格设定`）。

5. **`provider.go:195`** `StructuredAssembledWithToolsSchema(...)` → `provider.go:207` `completeWithToolsSchemaMode(..., assembled=true, continuation=true)`
   - 因 `assembled==true`，**跳过** `composeProviderMessages`（`provider.go:235-242`），直接 `validAssembledProviderMessages` 校验（`provider_prompt_composer.go:99`）。
   - `provider.go:245` `providerChatPayloadWithSchema(...)` 在已组装 messages 外补 `tools` + `response_format`（见第 7 节）。
   - 响应在 `provider.go:316-414` 解析：`NormalizeProviderToolCalls`（`provider.go:338`）和 `parseStructuredCandidatesForRole`（`provider.go:350`）。

> 推断：`composeProviderMessages`（`provider_prompt_composer.go:45`）仅用于非 assembled 角色（如 media_prompt、initialization 等普通路径）；cognitive_assessment 走 assembled 分支，其 System 由 `renderProviderSystem` 直接渲染、不再经 `composeProviderMessages`。

---

## 2. 消息布局表

下表针对 cognitive_assessment（conversation turn）的最终 messages 数组（由 `assemblePromptMessages` / `prompt_context_assembler.go:370` 决定）：

| # | role | 内容来源（代码） | 是否受预算裁剪 | 裁剪优先级 / unit |
|---|------|------------------|----------------|-------------------|
| 0 | system | `renderProviderSystem`：`providerContextAuthorityRule` + `capabilityConversationPolicyInstruction`（`operation_rules`）+ `filterCorePersona(CorePersona)`（`# 人格设定`）。`provider_prompt_composer.go:156-188`、`prompt_context_assembler.go:249` | 否（受 `SystemTokensCap=16384` 硬上限，`prompt_context_assembler.go:255`，但不会被候选选取逻辑丢弃） | — |
| 1（条件） | user | `[RUNTIME CONTEXT]`：聚合 `facts`(RuntimeFact) + `active_memory`(Active) + `retrieved_memory`(Retrieved) + `conversation_summaries`(Summary)，JSON 序列化。`prompt_context_assembler.go:370-391` | 是（作为若干 unit 参与贪心选取） | 见下方分组顺序 |
| 2..n | **user / assistant（保留真实 role）** | `recent`：`PromptFragmentRecentMessage`，按 turn 配对成 unit；`assemblePromptMessages`（`prompt_context_assembler.go:370-390`）**原样 append** 该 fragment，role 来自 fragment content（`provider_context.go:133-135`、`:164`：`{"role": role, "content": content}`，role ∈ {user, assistant}）。`prompt_context_assembler.go:342-354`、`370-390` | 是 | `order=2`（`promptOptionalCandidates` 分组 Recent） |
| last | user | `current`：`input.CurrentInput`。`prompt_context_assembler.go:250` | 否（受 `CurrentInputTokensCap=16384` 硬上限，`prompt_context_assembler.go:255`） | — |

候选 unit 分组顺序与优先级（`prompt_context_assembler.go:328-367` `promptOptionalCandidates`）：

- `order`：`Active=0` → `RuntimeFacts=1` → `Recent=2` → `Retrieved=3` → `Summaries=4`
- 同组内：`PromptFragmentRecentMessage` 排最后（按 turn 整体丢弃），其余按 `Priority` 降序（`prompt_context_assembler.go:358-366`）。
- 超出 `MaxInputTokens` 时从**尾部**（最低 priority / 最新 Recent 之外的末端）整体丢弃 unit（`prompt_context_assembler.go:289-302`、`305-319`）。

> 读到的：`RuntimeFacts` 来自 `workingMemoryInputFromProjection`（`provider_context.go:97` 把 compact 后除 core_persona/memories/recent_messages 外的 key 都做成 RuntimeFact，priority 默认 50，current_state/schedule/presence 等 100）。

---

## 3. Main response schema 字段表（cognitiveTurnResponseSchema，provider_schemas.go:200-236）

必填（root required）：`response_mode`, `action_type`, `response_intent`, `tool_calls`, `influences`（`provider_schemas.go:235`）。

| 字段 | 类型 | 必填 | 真实消费者（文件:行号） | 与 tool_calls 是否重复职责 | 备注 |
|------|------|------|--------------------------|----------------------------|------|
| `response_mode` | enum(final\|query_continuation) | 是 | `mutations.go:752`（取默认）、`mutations.go:831` `normalizeConversationResponseMode`、`mutations.go:852-873`（分支 final / query_continuation 校验） | 否 | 决定是否需要 query continuation |
| `action_type` | enum(reply) | 是 | `mutations.go:846` `normalizeConversationActionType`、`mutations.go:873`（强制置 "reply"） | 否 | 交互回合固定 reply |
| `response_intent` | string | 是 | `mutations.go:748`（`decision["response_intent"]` 进入 frozen）、`normalizationComposite`/`composite_actions.go:44` 读取 `decision["response_intent"]` | 否 | 仅作 frozen 存证/组合动作意图 |
| `tool_calls` | array(openObject) | 是 | 原生通道：`provider.go:338` `NormalizeProviderToolCalls`；JSON 侧车：`provider.go:396` 再次 `NormalizeProviderToolCalls(completion.Structured["tool_calls"])`；下游 `mutations.go:782` `capabilityInvocations`、`mutations.go:838` `resolveCapabilityAction` | ——（见第 4 节：与消息级 tool_calls 存在双通道） | schema 里 `tool_calls` 是 `openObject`（`provider_schemas.go:152`）以规避 strict json schema 卡死 |
| `influences` | decisionInfluencesSchema | 是 | `mutations.go:802` `freezeDecisionInfluences(decision, projection, false)`、`mutations.go:717-719` `validateFrozenDecisionInfluences` | 否 | 与 provider 所见 projection 强绑定冻结 |
| `visible_text` | string | 否（schema 非必填，但业务强约束） | `mutations.go:830` `normalizeVisibleReply(firstString(responsePlan["visible_text"], decision["visible_text"]))`、`mutations.go:869-870`（写入回 decision/responsePlan）、`mutations.go:1107`（最终取可见文本）、`query_continuation.go:115` `continuationVisibleText` | 与 `conversation.reply` text 重复（见第 4 节） | 业务上若 `response_mode=final` 且无 conversation.reply，则 visible_text 是唯一可见文本；缺失即 `cognition_visible_text_missing`（`mutations.go:867`、`:1114`、`:914`） |
| `response_plan` | responsePlanSchema | 否 | `mutations.go:748` `responsePlan = decision["response_plan"]`、`mutations.go:822` `normalizeResponsePlan`、`:828` 写回 | 否 | 含 personality_decision / output_preference_decision / visible_text / claims / self_evaluation / core_alignment / state_expression 等子字段（provider_schemas.go:166-182） |
| `personality_decision` | object（decision/keep/switch + profile ids + confidence + evidence_refs） | 否 | `mutations.go:808-815` `preparePersonalityDecision`；错误校验 `cognition.go:409`、`personality_runtime.go:89,209-222,232` | 否 | 多重人格切换的唯一决策入口；删除前需保留 `preparePersonalityDecision` 调用链 |
| `output_preference_decision` | outputPreferenceDecisionSchema | 否 | `mutations.go:874-875` `evaluateOutputPreferenceAction` | 否 | 通道/画像选择 |
| `core_alignment` | openObject | 否 | `normalizeResponsePlan` 透传；动作实现手递 `compactResponsePlanForProvider`（`provider_context.go:1012`） | 否 | 仅投影/存证 |
| `state_expression` | openObject | 否 | `compactResponsePlanForProvider`（`provider_context.go:1012`）；`stateExpressionSchema`（`provider_schemas.go:192`） | 否 | 表情/情绪外显 |
| `claims` | array(claimSchema) | 否 | `normalizeResponsePlan` 透传；`claimSchema`（`provider_schemas.go:138`，持久化语义契约） | 否 | 持久化到 cognition fact 层 |
| `appraisal` | appraisalResponseSchema | 否 | `mutations.go:794` `skipCognitiveStateTransition = len(decision["appraisal"])==0`；`cognition_growth.go:205` 写入 `cognition_appraisals` 表 | 否 | 状态提案关键字段：为空表示「不提议状态转移」 |
| `attention` | cognitiveStageSchema(anyOf string\|object) | 否 | 进入 frozen decision，下游无专门行为消费者（推断：仅存证/回放） | 否 | 认知阶段短摘 |
| `thought` | 同上 | 否 | 同上 | 否 | 认知阶段短摘 |
| `desire` | 同上 | 否 | 同上 | 否 | 认知阶段短摘 |
| `agency` | 同上 | 否 | 同上 | 否 | 认知阶段短摘 |
| `self_evaluation` | selfEvaluationSchema | 否 | `compactResponseEvaluation`（`provider_context.go:1037-1045`）动作实现手递 | 否 | |
| `evidence_refs` | array(string) | 否 | `freezeDecisionInfluences`（mutations.go:802）及 frozen 存证 | 否 | |

> 删除任何字段前必须能指出消费者：本表已逐行标注。最危险的字段是 `appraisal`（状态提案，cognition_growth.go:205）、`personality_decision`（mutations.go:808）、`visible_text` 与 `conversation.reply`（mutations.go:830/1107）、`tool_calls`（provider.go:338/396）、`influences`（mutations.go:802）。

---

## 4. 工具协议（conversation.reply）

- **真实参数结构（必需字段是 `text`，不是 content/数组）**：`tool_contract.go:191-214` `conversationReplyCapabilityDefinition`
  - `InputSchema`: `{type:object, additionalProperties:false, required:["text"], properties:{text:{type:string,minLength:1,maxLength:32000}}}`（`tool_contract.go:198-202`）
  - `OutputSchema` 要求 `text`, `target_kind`("conversation_message"), `target_ref`（`tool_contract.go:203-211`）
  - `CapabilityTypeAction`、`FailurePolicyRequiredForVisibleClaim`（`tool_contract.go:212`）。即：作为**正式发送私聊**的工具，文本放在 `arguments.text`。
- **工具定义生成位置与形式**：`tool_contract.go:158-178` `RenderCapabilityTools(definitions)` → 输出 `[{type:"function", function:{name, description, parameters}}]`。
- **传给 provider 的形式**：`provider.go:892-894` 当 `len(definitions)>0` 时 `payload["tools"] = RenderCapabilityTools(...)` 且 `payload["tool_choice"]="auto"`。即 OpenAI 兼容原生 tool calling。
- **模型是否需要在文本 JSON 里复制 tool_calls**：**不需要，但代码允许双通道并存**。
  - 原生通道：`message["tool_calls"]` → `provider.go:338` 归一化。
  - JSON 侧车通道：`cognitiveTurnResponseSchema` 的 `tool_calls` 字段（`provider_schemas.go:227`）位于 `response_format` 的 strict json schema 内；`provider.go:390-402` 若 `len(definitions)>0` 会**再次**从 `completion.Structured["tool_calls"]` 解析并覆盖 `completion.ToolCalls`。
  - 两者都汇入 `completion.ToolCalls`（`provider.go:358`、`provider.go:401`）。即同一 capability 调用可能被模型以原生 tool_calls 与 JSON 内 tool_calls 两种方式表达，Core 都接受（后者覆盖前者）。这是「重复表达同一职责」的实例，但通常只走原生通道。
- **关于 visible_text 与 conversation.reply 的重复**：`visible_text`（schema 字段）与 `conversation.reply.arguments.text` 都能携带可见文本。`mutations.go:830` 优先取 `responsePlan["visible_text"]`/`decision["visible_text"]`；`mutations.go:1109` 兜底取 `replyTextFromCapabilityInvocations(...)`（即从 conversation.reply 的 text 抽取）。即可见文本有两个合法来源，消费端已做兜底合并。

---

## 5. 预算与大小统计设施

**存在**：
- 预算常量（写在代码常量/默认值，非外部配置）：
  - `prompt_context_assembler.go:12-23`：`defaultContextWindowTokens=131072`、`defaultMaxInputTokens=98304`、`defaultOutputReserveTokens=4096`、`defaultPromptSafetyMarginTokens=4096`、`defaultSystemTokensCap=16384`、`defaultToolsSchemaTokensCap=24576`、`defaultCurrentInputTokensCap=16384`、`defaultPromptImageTokens=1536`、`defaultPromptLowDetailImage=85`。
  - `DefaultPromptBudgetPolicy`（`prompt_context_assembler.go:38-48`）生成策略；真实数值由 provider `assignment` 覆盖（`provider_context.go:60-63`：TokenBudget / ContextWindowTokens / MaxInputTokens / PromptBudgetPolicyVersion）。
- Token 估算器：`EstimatePromptTokens`（`prompt_context_assembler.go:173`）、`estimateRawTextTokens`（`:164`，启发式 `bytes/3*5/4`）、`estimateImagePartTokens`（`:90`）、`estimateProviderMessageTokens`（`:193`）。
- **覆盖 tools 与 response_format**：`estimatePromptWireInput(messages, tools, responseFormat)`（`prompt_context_assembler.go:397-399`）= `EstimatePromptTokens(messages)+EstimatePromptTokens(tools)+EstimatePromptTokens(responseFormat)+16`，在 `AssemblePromptContext`（`:304`、`:318`、`provider_context.go:251`）与 `provider.go:251` 均被调用。另 `prompt_context_assembler.go:255` 单独校验 `ToolsSchemaTokensCap`。
- **统计/日志**：`PromptAssemblyTrace`（`prompt_context_assembler.go:215-225`，含 `EstimatedInputTokens`、`SectionTokens`、`Selected`/`Dropped`）；`provider.go:847-878` `mergeProviderPromptBudgetDiagnostics` 输出 `estimated_input_tokens`、`section_tokens{tools,response_schema,...}`；`provider.go:259` 写入 diagnostics。
- **字节/字符统计设施（tools 专用）**：`CapabilityToolSchemaStats`（`tool_contract.go:183-189`）返回 `(bytes, chars)` = `json.Marshal(RenderCapabilityTools(definitions))` 的 len。但其**仅在生产之外被使用**：仅出现在测试 `capability_core_test.go:916`、`:920`、`memory_recall_capability_test.go:90-92`。**生产代码无断言**。

**不存在（按搜索关键词确认）**：
- 搜索过：`request size`、`bytes`、`max_bytes`、`prompt_size`、`prompt_bytes`、`MaxBytes`、`byte limit`、`字符统计`（针对完整 chat/completions payload）。结果：无任何对**完整请求**（messages+tools+response_format 合计）的字节/字符级硬上限或断言；预算完全是 token 估算驱动的。也没有把 `CapabilityToolSchemaStats` 接入 `completeWithToolsSchemaMode` 的断言路径。
- 没有把「真实 tokenizer」接入的代码（注释明确：`tool_contract.go:180-182`「当前 Provider envelope 不暴露 tokenizer 用量，调用方应报告 bytes/chars 而非臆造 token」）。`estimateRawTextTokens` 是纯启发式，非真实 tokenizer。

结论：**有 token 估算 + 日志 + 分区统计；无字节/字符级生产断言；tools 字节统计函数存在但仅测试使用。**

> **[2026-09-14 状态标注，R1/M9]** 本节结论准确，但**不要**把它读成「全仓没有请求大小设施」——那是我在 `design.md` 初稿里的过度概括，已更正。准确表述是：**已有** token 估算、三层 cap 与 `PromptAssemblyTrace` 分区统计（`prompt_context_assembler.go:14-19`、`:38-48`、`:215-225`、`:255-259`、`:397-399`），且已有**确定性 cap 守卫** `TestConversationCapabilityCatalogFitsDefaultPromptBudget`（`provider_live_tool_test.go:34-47`，本身不调用 provider）与 `prompt_context_assembler_test.go:186-187` 的 `wireEstimate <= defaultMaxInputTokens` 断言；**缺的是**：① 分节**字节数**（非仅 token）；② **Judge 请求**的同等覆盖；③ 机器可读的总体大小报告。三者在 `implement.md` 阶段 3 / 11.5 补齐。

---

## 6. 重复序列化审计

### 6.1 tool schema 是否在 tools 与 System 中重复 —— **否（明确不重复）**
- `provider_context.go:176-182` 注释明确：capability definitions 故意不进入 user content；能执行能力的调用已通过独立的原生 `tools` catalog 发送，重复会浪费 prompt 且让模型有两份契约要对齐。
- 证据：`compactCognitionContext`（`provider_context.go:182-277`）的产物**不含**任何 capability/tool schema；`assembleProjectionPrompt`（`provider_context.go:64-67`）只把 `RenderCapabilityTools(definitions)` 作为 `Tools` 字段传递，`System` 内容（`renderProviderSystem`）只含 operation rules + Core Persona + actor relationship（provider_context.go:249）。
- 因此 tools 仅出现一次（payload.tools）。

### 6.2 完整人格是否同时在 System 与 Runtime Context 中重复 —— **完整人格不重复，但存在「部分人格」重复**
- System 人格：`prompt_context_assembler.go:249` `filterCorePersona(input.CorePersona)` → 仅写入 System（`provider_prompt_composer.go:177-187` `# 人格设定`）。
- Runtime Context：`assemblePromptMessages`（`prompt_context_assembler.go:370-391`）只聚合 WorkingMemory 的 `facts/active_memory/retrieved_memory/conversation_summaries`。
- 关键去重证据：`workingMemoryInputFromProjection`（`provider_context.go:80-84`）先 `compact := compactCognitionContext(projection)` **再 `delete(compact,"core_persona")`**、`delete(compact,"memories")`、`delete(compact,"recent_messages")`。即 `core_persona` 不会进入 RuntimeFacts → **完整人格不重复**。
- **但存在部分人格重复（需关注）**：`compactCognitionContext`（`provider_context.go:197-199`）把 `effective_persona` 加入 compact，而 `workingMemoryInputFromProjection` 只删 `core_persona/memories/recent_messages`，未删 `effective_persona`，于是 `effective_persona` 进入 `input.RuntimeFacts`（provider_context.go:97，priority 50）。`effective_persona` 内容由 `compactEffectivePersonaForProvider`（`provider_context.go:279-287`）定义，含 `personality` 与 `behavioral_policy`。这两块同时存在于 System 人格（`filterCorePersona` 的 `personality`/`behavioral_policy` 组，provider_prompt_composer.go:246）。→ **personality/behavioral_policy 在 System 与 Runtime Context 部分重复**。
  - 推断风险：本次若再加「Working Persona 投影」进 RuntimeFacts，会进一步放大与 System 人格的重叠。
- 其余字段（current_state、schedule、presence、developing_self、memories、relationships、drive_slots 等）均在 Runtime Context 单向出现，System 不含 → 无重复。

> 读到的重复：`effective_persona` 的 personality/behavioral_policy 双处出现（provider_context.go:197-199 + 279-287 vs provider_prompt_composer.go:246/177-187）。

---

## 7. Provider 契约

- **原生 tool calling**：支持。当 `len(definitions)>0` 时 payload 带 `tools` + `tool_choice:"auto"`（`provider.go:892-894`）。conversation turn 走 `CapabilitySurfaceConversation` 目录（`mutations.go:761`），必然带 tools。
- **关闭 thinking**：**不支持显式关闭参数**；仅支持开启。`enable_thinking=true` 通过 `payload["enable_thinking"]=true` 设置（`provider.go:901-902`），且仅在 `role=="cognitive_assessment"` 时 `enableThinking=true`（`provider.go:881` `providerChatPayloadForRole`；`mutations.go:770` 也传 true）。无 `enable_thinking:false` 分支——即 thinking 由 provider/模型侧决定，Core 只选择「开」。
  - 注意 mlx-serve 行为：`provider.go:346-349` 注释：thinking 开启时结构化 JSON 可能落在 `reasoning_content`，Core 把它当结构化控制通道、绝不作为可见文本。
- **response_format 用法与限制**：
  - 生成函数 `providerResponseFormatForSchema`（`provider.go:914-932`）：`initialization` 返回 `{"type":"json_object"}`；其余返回 `{"type":"json_schema","json_schema":{"name":schemaName,"strict":true,"schema":schema}}`（strict 模式）。
  - 注入条件（`provider.go:892-900`）：仅当 `jsonMode==true` 时附加，且与 tools 是否非空无关（tools 非空或 jsonMode 为真都会加）。cognitive_assessment 调用 `StructuredAssembledWithToolsSchema(..., jsonMode=true)`（`provider.go:195`），故带 strict json schema。
  - 限制/坑：`provider_schemas.go:102-106` 注释——MLX strict-json-schema 会对 `{}` 卡死编译 grammar，故 persona 值用 `openObjectSchema()` 渲染首片；`tool_calls` 字段用 `openObjectSchema()`（`provider_schemas.go:152`）以兼容 strict 模式。
  - `providerSchemaForRole`（`provider.go:941-956`）把 role 映射到 schema：cognitive_assessment→`cognitiveTurnResponseSchema`，reflection→`reflectionProposalV2ProviderSchema`，initialization→`initializationResponseSchema` 等。

---

## 8. 对本次重构的约束

**必须复用的唯一出口（不可绕过，否则布局会分裂）**：
- `AssemblePromptContext`（`prompt_context_assembler.go:241`）：唯一把 WorkingMemory + System + Current 组装成 messages 的函数。新增内容应通过扩展 `PromptAssemblyInput` / `WorkingMemory`，而非另写组装函数。
- `assembleProjectionPrompt`（`provider_context.go:14`）：唯一构建 WorkingMemory + PromptBudgetPolicy 并调用 `AssemblePromptContext` 的业务入口（cognitive_assessment / wake_up / daily_review / native_cognition / reflection_v2 都经此）。
- `renderProviderSystem`（`provider_prompt_composer.go:156`）：System 消息唯一渲染器（assembled 路径直接调用；非 assembled 路径经 `composeProviderMessages` 仍调它）。
- `StructuredAssembledWithToolsSchema`（`provider.go:195`）：cognitive_assessment 等角色唯一构造带 tools+response_format 的线 payload 的出口。
- `compactCognitionContext`（`provider_context.go:182`）与 `workingMemoryInputFromProjection`（`provider_context.go:80`）：唯一决定「哪些字段进入 RuntimeFacts / 不进入」的投影层；**新增 Working Persona 投影必须在此层做去重**，否则会与 System 人格重复。

**新增「Working Persona 投影」最容易破坏现有布局的位置**：
1. 若把它塞进 `CorePersona`（System 路径 `filterCorePersona`）：会膨胀 System、并可能触发 `SystemTokensCap=16384` 上限（`prompt_context_assembler.go:255`）。而且 System 已经承载完整 Core Persona，再加 Working 投影属语义叠加，违反「System=硬约束」的协议（`provider_prompt_composer.go:9-21` 第 2 条优先级）。
2. 若把它作为新的 `RuntimeFact` 加入 WorkingMemory：必须确认不与 System 人格、`effective_persona` 已存在的 personality/behavioral_policy 重复（见第 6.2 节）。推荐做法是**新增一个独立的 `PromptFragmentKind`**（如 `PromptFragmentWorkingPersona`），而不是混入 `RuntimeFacts`，以便独立裁剪优先级（类似 `order` 分组，`prompt_context_assembler.go:333`）。
3. 不要同时写入 System 与 Runtime Context（参照第 6.2 节 `effective_persona` 的教训）。
4. 若投影需要进入 `response_format` schema 或 `response_plan`：需同步更新消费者（`normalizeResponsePlan`、`mutations.go:822` 起）与 `cognitiveTurnResponseSchema`（`provider_schemas.go:200`），并确认 strict json schema 不会因新字段卡死（参考 `openObjectSchema` 用法）。

---

## 附：搜索过的关键词（用于「不存在」声明的佐证）

- Prompt 组装/布局：`cognitive_assessment`、`response_format`、`ResponseFormat`、`tools`、`budget`、`Budget`、`token`、`Token`、`System`、`role`、`Role`、`persona`、`Persona`、`appraisal`、`personality_decision`、`visible_text`、`visible_reply`、`tool_calls`、`TOON`、`toon`。
- 大小/字节：`request size`、`bytes`、`MaxBytes`、`max_bytes`、`prompt_size`、`prompt_bytes`、`byte limit`、`字符统计`、`CapabilityToolSchemaStats`。
- 结论：工具 schema 重复 = 不存在（provider_context.go:176-182 明确否定）；完整人格双写 = 不存在（core_persona 在 provider_context.go:81-84 被删）；部分人格（effective_persona 的 personality/behavioral_policy）双写 = 存在（provider_context.go:197-199,279-287 vs provider_prompt_composer.go:246）；完整请求字节级断言 = 不存在（仅 token 估算 + tools 字节统计函数仅在测试中使用）。
