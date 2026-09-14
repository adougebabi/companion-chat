# 执行计划：多重人格接管、Working Persona 与 Prompt 收敛

> 对应设计：同目录 `design.md`（**已修订至 R1，2026-09-14**）；需求：`prd.md`（R01–R12）与 `request.md`（1–20 节）
> 声明：本文件是执行清单，不是需求文档。每步的验收标准回溯到 PRD 的 R 编号。
> G0 的三个待定问题已在 `design.md` §11 **关闭**；不再重复确认，直接按已决议内容实施。

---

## 0. 前置约束（每步都必须遵守）

- 唯一回复发送路径仍是 `conversation.reply` → `conversation_messages`（`mutations.go:1149`）+ 既有同步流/异步 outbox。不新增旁路。
- **唯一文本权威**（`design.md` §4.3）：`resolveCanonicalVisibleReply` 生成期算一次并冻结；preview / Judge / 发送端读同一个冻结值。禁止在 preview 里重新从 reply 参数推导文本。
- 模型调用一律在数据库事务外；只有「校验版本 → 覆盖 payload → 结算」在短事务内。
- `cognition_frozen_actions` 状态机保持 `frozen/completed/failed`，不新增状态；**阶段用 payload 内的 `turn_stage`**。
- **权限执行脊 E1–E5 缺一不可**（`design.md` §0.4）：候选归一化丢弃 / 持久化授权标记 / 结算门 / 恢复门 / 覆盖时重置授权。**不得**只靠移除 Prompt 字段。
- 不修改 `forced_activation` / `switching.rules` / 角色卡原值；只做归一化与派生。
- 新增生产文件不得使用 `switch invocation.CapabilityName`、`switch call.Name`、`invocation.CapabilityName ==`。
- 不在 `mutations.go` 新增第二处主生成调用（见阶段 5）。
- 每个阶段结束都跑 `gofmt`；提交前跑 `go vet`。

### 验证命令（全流程复用）

> **F14.5 修正**：不要把多个 `cd apps/core-go && ...` 连在一起粘贴执行——它会改变后续命令的相对目录。用 `go -C`（Go 1.20+）或各自子 shell。

```bash
# 形式一：go -C（推荐）
go -C apps/core-go build ./... && go -C apps/core-go vet ./...
go -C apps/core-go test -race ./...
go -C apps/core-go test -race -run 'Takeover|WorkingPersona|CanonicalVisible|PromptAssembler|QueryContinuation|CapabilityRuntimeStaticGuards|ArchitectureGuard' ./internal/core/

# 形式二：子 shell（go -C 不可用时）
( cd apps/core-go && go build ./... && go vet ./... )
( cd apps/core-go && go test -race ./... )

# 格式门（CI 同款）
test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
```

**环境阻塞（如实标注，不得用整体绿色代替）**：

| 阻塞 | 门 | 影响 |
|---|---|---|
| 无 `GO_CORE_TEST_DATABASE_URL` | `project_health_transaction_integration_test.go:126` 等 `t.Skip` | 冻结/结算/回滚/投递链路断言**未真实执行** |
| 无 live provider（`FLUCTLIGHT_LIVE_PROVIDER_TEST=1`） | `provider_live_tool_test.go:443-447` | 真实模型行为、真实 token/延迟、**多头卡初始化的接管语义产出**未验证 |

**报告必须区分**：基线失败 / 环境阻塞 / 新增回归。

---

## G0 评审门（已关闭 — 决议见 `design.md` §11）

- [x] `persistent_deterministic`：**只分类 + 稳定 ID + 诊断**，不实现确定性求值器（选项 A）。
- [x] 接管声明入口：**`takeover_rules[]` 为唯一执行语义**；`forced_activation` 作为受控迁移输入，只有初始化/迁移 Provider 产出完整 typed 结果时才产生可执行规则；旧 prose 本身只保留并诊断。
- [x] 「无新表、状态存于 frozen payload」：**接受**，但叠加 `turn_stage` / `winner` / `expected` / `persistent_switch` 与执行资格守卫。
- [x] 跑 `python3 ./.trellis/scripts/task.py start <task-dir>` 将状态切到 `in_progress`。

**实施顺序（同意评审的收敛顺序）**：先修 **F01–F05 执行契约**（阶段 2.5–5），再定 **F06–F08 产品路径**（阶段 2、6），然后落 **F09–F11 配置与保真**（阶段 3、4），最后统一 research 口径（阶段 10）与基准（阶段 11）。

---

## 阶段 1：测试基座（先做，否则后续步骤无法写确定性测试）

对应 R12；依据 `research/inventory-tests-specs.md` §1.5–1.7、§7.1。

- [x] 新增测试辅助（`_test.go`，`package core`）：
  - `fakeProviderRouter`：按 request 的 model role + schema name 路由固定 JSON 响应，可脚本化「A 响应 / Judge 响应 / B 响应」；复用既有 `projectHealthRoundTripFunc`（`project_health_transaction_integration_test.go:37`）模式，不引入新依赖。
  - `newTestApp(t, repository, httpTransport)`：收敛 `&App{DB, Provider, ContextResolver, Capabilities, Runtime}` 的手工装配（现散落在 `conversation_delivery_regression_test.go:36-64`）。
  - `nowFunc` 注入点：生产代码当前直接 `time.Now()`（`mutations.go:948/1120`、`app.go:1421`）。新增可注入时钟字段（默认 `time.Now`），仅用于确定性断言；不改变既有语义。
  - **`captureProviderWirePayload`**：捕获真实 wire payload，用于 F09 的「实际参数」验收与阶段 11 的大小统计。
- [x] 复用既有计数型能力作 Spy Executor：`nativeReplayTestCapability`（`project_health_transaction_integration_test.go:43`）。
- **验证**：新增 helper 自身有测试；`go -C apps/core-go test -race ./internal/core/` 通过。
- **回滚点 R1**：只新增测试文件与一个时钟注入字段，删除即回到基线。

---

## 阶段 2：切换规则的归一化、稳定 ID 与授权（R07、F07）

新增 `apps/core-go/internal/core/persona_switch_rules.go`。

- [x] 定义：

```go
type personaSwitchRuleKind string
const (
    switchRulePersistentDeterministic personaSwitchRuleKind = "persistent_deterministic"
    switchRulePersistentSemantic      personaSwitchRuleKind = "persistent_semantic"
    switchRuleTurnTakeover            personaSwitchRuleKind = "turn_takeover"
    switchRuleUnclassified            personaSwitchRuleKind = "unclassified"
)
type personaSwitchRule struct {
    RuleID            string                 // switch:<id> | takeover:<id> | takeover:forced:<digest>
    RuleContentDigest string                 // 内容版本，与 RuleID 分离（F07）
    Kind              personaSwitchRuleKind
    Source            string                 // "switching.rules" | "takeover_rules" | "forced_activation"
    SourceProfileID   string                 // F07：来源人格资格判断
    TargetProfileID   string
    Condition         string
    Priority          float64
    CooldownSeconds   int
    Enabled           bool                   // F07：资格判断
    Raw               map[string]any         // 原值保留，不改写
}

type persistentSwitchGrant struct {
    Allowed       bool
    Scenario      string
    DeclaredRules []string
    Reason        string
}
```

- [x] `normalizePersonaSwitchRules(corePersona, runtime, subjectProfileID) []personaSwitchRule`：
  - 读 `personality_system.switching.rules[]` → `switch:<declared id>`（既有校验在 `personality_runtime.go:158-178`，不得重复实现第二套）；
  - 读 `personality_system.takeover_rules[]`（新增 typed 字段）→ `takeover:<id>`，`Kind = turn_takeover`；
  - 读 `personality_system.forced_activation`（兼容 `openObjectSchema` 的任意形状，含**字符串/数组/对象**）作为迁移证据：
    - 不从 prose 关键词、正则或 profile name 推断 kind/target；没有完整 typed migration 元数据时 → `unclassified`（**原样保留 + 诊断**）；
    - 只有显式 `kind=turn_takeover`、`version=turn-takeover.v1`、稳定 `id` 与声明 profile 的 `target_profile_id` 都存在且合法时，才产出可执行 `takeover:` 规则；
    - profile **display name** 或 condition 中的自然语言提及只用于迁移诊断，不能替代稳定 `target_profile_id`；
  - 全流程**不改写、不删除**原 map，只读取。
- [x] `resolvePersistentSwitchGrant(scope turnPersonaScope, scenario string, rules []personaSwitchRule) persistentSwitchGrant`：
  - `scenario == "takeover_reply"` → **无条件 `Allowed=false`**（结构性）；
  - `scenario == "query_continuation"` / `"takeover_judge"` → `false`；
  - `scenario == "cognitive_assessment"` → 仅当实例声明了 `switching` 入口（含 `default_profile_id`）时 `true`；
  - 其余 scenario（WakeUp / Reflection / autonomy）→ `false`。
- [x] `selectTurnTakeoverRule(rules []personaSwitchRule, scope turnPersonaScope) (personaSwitchRule, bool)`：
  - **Runtime 确定性选择唯一接管方**（`Kind == turn_takeover`、`Enabled`、`SourceProfileID` 资格匹配、`TargetProfileID` 存在于 `profiles`、非当前 reply owner）；
  - `cooldown` 资格判断（复用既有 `cooldown_until` 语义，`personality_runtime.go:143`）；
  - 并列时按 `Priority` 降序、`RuleID` 升序；Judge **不参与选择**（R08）。
- [x] 诊断：`unclassified`、`target_profile_unresolved`、`target_profile_ambiguous` 产出结构化诊断事件（含规则位置与原文摘要），不静默丢弃（R11）。
- [x] schema：`provider_schemas.go` 的 `personalitySystem` 增加 `takeover_rules` 的 typed schema；`forced_activation` 保持 `openObjectSchema()`（**不收紧**，避免破坏既有角色卡）。
- **验证**：
  - 单元测试：三分类矩阵、稳定 ID 确定性（同输入两次归一化 ID 一致）、**RuleID 不随 `Condition` 文本改变而改变**（仅 `RuleContentDigest` 变）、`unclassified` 保留原文、`TargetProfileID` 按 name/id 解析与歧义处理、并列选择顺序、目标 profile 不存在时的处理、`resolvePersistentSwitchGrant` 全场景表。
  - **真实卡回归 fixture（F07 硬要求）**：用 `testdata/initialization/dense_multi_card.txt` + `dense_multi_expectations.json` 保留接管语义断言；真实 prose 路径必须得到 0 条 executable takeover 且有 bounded diagnostic。另用固定 `dense_multi_typed_migration.json` 模拟授权 Provider 迁移输出，证明同一语义可归一化为含 `rule_id`、`version` 与显式 `target_profile_id` 的 1 条可执行规则。
    - 真实 Provider 是否会在初始化时产出该 typed 字段仍由 opt-in live 测试验证；未配置时如实记录为环境未验证，不能以固定 fixture 代替。
- **回滚点 R2**：新文件独立；投影层尚未消费时无行为变化。

---

## 阶段 2.5：权限执行脊 E1–E5（R04、F01）— **执行契约，先于仲裁接入**

- [x] **E1** `applyPersistentSwitchGrant(decision map[string]any, grant persistentSwitchGrant) (map[string]any, []PromptDiagnostic)`：
  - `!grant.Allowed` → 删除 `decision["personality_transition"]` 与根级 `personality_decision` 残留，写 `decision["persistent_switch_proposal"] = {proposed, applied:false, reason, scenario}`；发诊断 `persistent_switch_proposal_dropped`；
  - 在 `mutations.go:888` `PersistTurnDecision` **之前**调用（A 与 B 都调用）。
- [x] **E2** `PersistTurnDecision` 扩展签名接收 `grant`：
  - payload 写 `persistent_switch: {authorized, scenario, grant_revision}`；
  - `personality_transition` 存在而 `authorized == false` → 返回 `personality_switch_not_authorized`（纵深防御）。
- [x] **E3** 新增 `applyPersistentSwitchIfAuthorizedTx(ctx, tx, fluctlightID, payload, plan)`：
  - `plan == nil` 或 `payload["persistent_switch"]["authorized"] != true` → **结构性 no-op**（返回 `nil, nil`）；
  - 否则调用既有 `applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`）；
  - `mutations.go:956/1126/1303` 三处**全部改走它**，不再直接调用 `applyPersonalityDecisionPlanTx`。
- [x] **E4** 恢复路径：`mutations.go:722-727` 重建 `personalityPlan` 后同样走 E3。
- [x] **E5** `ReplaceFrozenTurnDecision` 内部：覆盖时把 `persistent_switch` 重置为 `{authorized:false, scenario:"takeover_reply", reason:"takeover_reply_owner"}`，并清除 B 的 `personality_transition`。**授权绝不继承**。
- **验证**：
  - `TestTakeoverReplyCannotWritePersistentActive`：Fake Judge=true，Fake Provider 让 B 返回**格式完全合法**的持久切换提案 → `fluctlight_personality_runtime` 逐字段不变，`persistent_switch.authorized == false`。
  - `TestPersistentSwitchStillWorksViaAuthorizedScenarios`：授权场景下持久切换照常生效（回归白天/晚上切换）。
  - `TestUnauthorizedScenarioHasNoPersonalityDecisionField`：未授权场景的 response schema 不含 `personality_decision`。
  - `TestRecoveryDoesNotApplyUnauthorizedSwitch`：恢复路径下 E4 生效。
- **回滚点 R2.5**：E1–E5 可分别回退；回退 E3 即回到「字段存在即写入」的旧行为（不推荐，仅作紧急回退）。

---

## 阶段 3：Working Persona 投影与 Prompt 收敛（R08、R09、F11）

新增 `apps/core-go/internal/core/working_persona.go`；修改 `provider_context.go` / `provider_prompt_composer.go` / `provider_schemas.go`。

- [x] `projectWorkingPersona(projection ContextProjection, subjectProfileID string) (workingPersona map[string]any, trace workingPersonaTrace)`：
  - 合并顺序：Shared Identity（`identity` + `life_profile`）→ `profiles[subjectProfileID]` 基线 → `ComposeEffectivePersona`（`evolution_overlay.go:249`）应用 active overlay；
  - **保真（F11）**：`traits.*` 有数值才给数值；**描述性原文逐字保留**（`dense_multi_card.txt` 的「暮光安静、句子短、行事谨慎」等），**不得**臆造数字替代原词；`voice` / `expression` / `behavior_loops` / `scenario_behavior` / `behavioral_policy` 的关键语义必须保留；
  - 排除：完整背景、全部经历、未激活人格、视觉资产、`secrets` 全文；
  - 携带 `profile_id` / `persona_revision` / `overlay_revision` 以便追溯；
  - **确定性**：同输入同输出，不调用模型。
- [x] 修形状冲突（R11、F11）：`normalizeEvolutionPersonalityBaseline`（`evolution_overlay.go:452-458`）遇到非对象 `traits` 时**不再静默替换为 `{}`** → 显式兼容转换（保留原值到描述性载体）或返回可诊断错误。
- [x] 唯一 System Persona 出口：`filterCorePersona`（`provider_prompt_composer.go:241`）改为渲染
  `shared_identity` + `working_persona` + （条件）`persistent_switch`；**不得**渲染 `takeover_rules`。
- [x] **`profiles` 收敛（F01/R09）**：`filterSemanticProfileValue`（`provider_prompt_composer.go:299-322`）对 `profiles` 只保留 `{id, name}`，**不再通过 `filterCorePersonaValue` 漏出非激活 profile 的内容**。
- [x] 去重：`workingMemoryInputFromProjection`（`provider_context.go:80-84`）的 delete 列表扩展为 `core_persona` / `memories` / `recent_messages` / `effective_persona` / `personality_system`。
- [x] `compactPersonalitySystem`（`provider_context.go:289-315`）：不再 `cloneMap` 整组 `profiles`、不再追加 `active_profile` 整对象；只保留语义标识与 revision。
- [x] recent assistant 消息补 `reply_owner_profile_id`（`design.md` §4.1），来自该 turn 的冻结记录；`recentPromptFragments`（`provider_context.go:119`）渲染时带发言人格标注（**保留真实 `role`**，见 M8）。
- [x] 预算诊断与大小目标（F11）：生产投影按 allowlist 裁剪并执行可配置预算；当前机器可读报告覆盖 Fake Provider 的四条路径和合成 dense-shaped `life_profile` 局部前后字节。**同一真实卡的 full request 前后、真实 Token/缓存/延迟仍待 live 环境**，不能把合成报告写成完整性能验收；超限抛 `working_persona_over_system_budget`，**不从字符串中间截断**。
- [x] 复用并扩展既有预算设施（M9）：`AssemblePromptContext` 已算 `EstimatedInputTokens` 与三层 cap（`prompt_context_assembler.go:14-19`、`:38-46`、`:255-259`）；补分节**字节数**与 Judge 请求覆盖。
- [x] `cognitiveTurnResponseSchema(grant)` 条件化：`grant.Allowed == false` → 不含 `personality_decision`；有接管规则 → 可选 `internal_intent`（`string`, `maxLength 120`）。
  - **调用点同步**：`mutations.go:762` 与 `provider_live_tool_test.go:37` 需一起改（后者是确定性预算守卫）。
- **验证**：
  - `TestWorkingPersonaKeepsDescriptiveTraitsVerbatim`：散文特质原词逐字保留，且**未出现臆造数值**。
  - `TestWorkingPersonaExcludesFullProfilesAndOtherPersonas`：投影与 Main Prompt 均不含未激活人格内容。
  - `TestProviderSystemPersonaHasNoTakeoverRules`：A 的 System 段不含 `takeover_rules`。
  - `TestProviderSystemPromptHasSinglePersonaSource`：System 中人格只有一处；Runtime Context 无 `core_persona`/`effective_persona`/`personality_system`。
  - `TestAPromptDoesNotContainCompleteB` / `TestBPromptDoesNotContainCompleteA`（R09 验收）。
  - `TestProfileRosterIsIdentifierOnly`：`profiles` 只剩 `{id,name}`。
  - 回归 `prompt_context_assembler_test.go:23`、`provider_context_test.go:520/538/563`、`provider_live_tool_test.go:34-47`。
- **回滚点 R3**：投影、`profiles` 收敛、去重可**分别**回退。

---

## 阶段 4：唯一可见文本、候选预览与 Judge 契约（R05、F02、F09）

新增 `resolveCanonicalVisibleReply`（`visible_output.go`）；新增 `turn_takeover.go`（preview / control view）；修改 `provider.go` / `diagnostics.go` / `provider_schemas.go`。

- [x] `resolveCanonicalVisibleReply(responsePlan, decision, invocations) canonicalVisibleReply`：
  - 生成期调用**一次**，结果冻结进 `decision["visible_text"]` 与 `responsePlan["visible_text"]`（保持 `mutations.go:830` → `:863-870` 的既有优先级）；
  - 双来源同时非空且不同 → `Conflict=true` + 诊断 `visible_text_source_conflict`，`normalizeTurnDecision` 直接 fail closed；不再静默选择 winner；
  - 根字段为空而 `conversation.reply.text` 非空 → reply-only fallback 合法，由 Core 派生 canonical；
  - **preview / Judge / `mutations.go:1149` 全部读该冻结值**，不得各自重推导。
- [x]（F-01 path b 适度，已默认启用）**已启用**：`strict_visible_text_reconciliation`——「双来源冲突」（根 visible_text 与 reply.text 不一致）直接按契约 fail closed（`normalizeTurnDecision` 返回 `visible_text_source_conflict`，Turn 受控失败），不再静默选 winner。reply-only（无根字段但有 reply 参数）合法：reply 作为 fallback 提案源，Core 从 reply.text 派生 canonical（保留 request.md:13「回复已经是 Tool」原契约）。根 visible_text 是优先正式协议，冲突 fail closed 避免模型同时决定两份文本（见 design.md §4.3 修订与 request.md:13 path b 加注）。
- [x] `buildCandidatePreview(canonical canonicalVisibleReply, invocations []CapabilityInvocation, registry *CapabilityRegistry) CandidatePreview`：
  - 按 `definition.OutputRole` / `definition.Type` 分派，**禁止** `CapabilityName ==` 比较；
  - `ReplyText` 取 `canonical.Text`（**不是**从 reply 参数重推导）；
  - 动作摘要按 `OutputRole` / `TargetKinds` 归纳，不 dump 原始参数；
  - **不调用** `Capability.Execute`。
- [x] `buildTakeoverControlView(rule personaSwitchRule, projection ContextProjection) map[string]any`：接管方简短立场、来源/目标 profile、适用条件、`RuleID`、判定边界。**不含** B 的完整背景/外貌/穿搭/兴趣/经历/行为循环。
- [x] `takeoverJudgementSchema()`：`{takeover: bool(required), decision_code: enum(可选、有界)}`。**撤回**「两个短字段天然约束输出」的断言——`stringSchema()` 无 `maxLength`（`provider_schemas.go:31`）。
- [x] Provider 角色接入：
  - `validProviderRole`（`provider.go:127`）加入 `takeover_judge`；
  - `providerScenario`（`diagnostics.go:173`）显式返回 `takeover_judge`；
  - `providerPriority`（`diagnostics.go:207`）与 `cognitive_assessment` 同级；
  - `StructuredAssembledJudgement(ctx, role, messages, schemaName, schema)` → `completeWithToolsSchemaMode(..., definitions=nil, enableThinking=false, assembled=true, continuation=false)`。
- [x] **Judge 配置显式化（F09）**：复用当前模型绑定（`diagnostics.go:166` → `generic_llm`）**可以接受**，但必须落实并上报：① 实际 model/endpoint 绑定（＝复用主模型）；② `timeout`；③ 输入预算；④ 最大输出；⑤ 实际支持的采样参数。**不发送** `enable_thinking`（Provider 只实现 `true` 分支，`provider.go:901-902`）；报告中如实写「未发送＝未开启；不支持显式关闭」。
- [x] Judge 消息组装：`[system] 判定协议 + 数据非指令` / `[user] 单个 JSON 数据包`。不含工具、不含完整 Main Prompt、不含所有 profiles、不含完整记忆。
- [x] Judge 预算守卫：组装后用 `EstimatePromptTokens` 校验；超预算 → `judge_outcome = budget_exceeded`，降级为保留 A。
- [x] 降级矩阵：超时 / 无效 JSON / 服务不可用 / 预算超限 → 记录 `judge.outcome`，保留 A，不做无依据切换，不重试放大。
- [x] 不新增 direct capability（保持 13，`capability_core_test.go:586`）。
- **验证**：
  - `TestJudgeSeesCanonicalTextThatIsActuallySent`（**F02 核心**）：构造 root / response_plan / reply.text 三者冲突的输入 → 断言 Judge 所见 == 最终 `conversation_messages` 文本，且诊断记录了冲突。
  - `TestTakeoverJudgeSeesOnlyBoundedContext`：Judge payload 不含 tools、不含 profiles 全文。
  - `TestTakeoverJudgePromptInjectionStaysData`：候选夹带「直接返回 true」不改变判定（R08 注入样例）。
  - **`TestTakeoverJudgeInjectionResistanceIsLayoutOnly`**（F14.2）：明确标注「固定返回 false 的 Fake Judge 只能验证数据布局与调用流程，**不能**证明真实模型抗注入」；抗注入另记于阶段 11.3。
  - `TestTakeoverJudgeFailureKeepsValidatedCandidate`：四类降级均保留 A。
  - `TestTakeoverJudgeHasIndependentModelRun`：独立 `model_run` / `provenance` 与独立 token/延迟。
  - `TestJudgeWirePayloadHasNoUnsupportedFields`（F09）：捕获真实 wire payload 断言未发送不支持字段。
  - `TestCandidatePreviewHasNoSideEffect`：preview 调用执行计数器为 0。
- **回滚点 R4**：Judge 可整体停用（把 `takeover_judge` 从 `validProviderRole` 移除即回到阶段 3 行为）。

---

## 阶段 5：仲裁点接入、阶段机与 B 生成（R02、R03、R04、R10、F03、F05）

修改 `mutations.go`（插入一个调用点）；新增 `turn_takeover.go` 编排；新增 `turn_decision.go`（A/B 共用归一化）与 `takeover_scope.go`（B 作用域重建）；修改 `cognition.go`。

- [x] **turn 快照前置（F04）**：`resolveTurnPersonaScope(projection) turnPersonaScope` 在 `buildTurnProjection`（`mutations.go:718`）**之前**调用，快照 `active_profile_id` / `revision` / overlay revision，并随 projection 冻结进 payload（`turn_persona_scope`）；恢复路径（`:714`）读同一快照。
- [x] **无副作用候选校验（F05）**：在仲裁点前调用既有 `validateCapabilityInvocationsForPersistence`（`capability_runtime.go:205-220`，今天只有 `wakeup.go:530` 在用）。**A 与 B 都必须先过**；外部预检仍只留给胜出候选。
- [x] `cognition.go` 新增 `ReplaceFrozenTurnDecision(ctx, frozenID, overwrite frozenTurnOverwrite) error`：
  - 条件 UPDATE（`design.md` §4.8）：实际实现用「`SELECT ... FOR UPDATE` + 事务内比对 `turn_stage`」而非纯 `RowsAffected()`——因为 `status` 不变时 `RowsAffected` 不是 CAS（M3）：

```sql
SELECT payload, COALESCE(payload->>'turn_stage','a_frozen')
  FROM public.cognition_frozen_actions WHERE id=$1 AND status='frozen' FOR UPDATE
-- stage != overwrite.ExpectedStage → ErrConflict
UPDATE public.cognition_frozen_actions SET payload=$2::jsonb WHERE id=$1 AND status='frozen'
```

  - 断言 `RowsAffected()==1`，否则 `ErrConflict`；
  - **必须同时**：写 `turn_stage` / `winner` / `expected`、重置 `persistent_switch`（E5）、覆盖 `capability_context_snapshot` 与 `context_reference_*` / `influences` / `goal_refs` / `intention_refs`（M4）、**用 B 的 projection 重算每条 invocation 的 `ContextSnapshot` 与 `ActionID`**（M4）、并经 `stripFrozenDecisionSidecars` 剥离 provider codec 旁挂字段。
- [x] `cognition.go` 新增 `AdvanceTurnStage(ctx, frozenID, from, to, patch) error`：用于 `a_frozen` → `arbitration_decided` → `b_frozen` → `winner_ready`；`COALESCE(payload->>'turn_stage',$2)=$2` 守卫。
- [x] `LoadFrozenTurn`（`cognition.go:347`）扩展：`status=='frozen'` 时同时校验 `turn_stage` 合法（`validateFrozenTurnStage`，紧邻 `validateExecutableCapabilityPayload`）。
- [x] `mutations.go` 在 `PersistTurnDecision` 之后、`capabilityInvocationsFromValue` 之前插入**唯一**调用（`mutations.go:816`）：

```go
if handled, err := a.applyTurnTakeover(ctx, turnTakeoverInput{...}); err != nil {
    return TurnResult{}, err
} else if handled {
    // 从 payload 重读最终胜出候选与全部派生变量（不要复用 A 的本地变量）
    reloaded, reloadFound, reloadErr := a.LoadFrozenTurn(ctx, inboxID)
    hydrated, hydrateErr := a.hydrateFrozenTurn(reloaded, fluctlightID)
    ...
}
```

- [x] `applyTurnTakeover` 编排（全部在 `turn_takeover.go`）：
  1. `responseMode == "query_continuation"` → 直接返回（互斥，不调 Judge；R06/F06）；
  2. 归一化规则 + 选唯一接管方；无规则 → `takeover.decision = "skipped"`、`winner_ready`，返回；
  3. **先持久化仲裁结论**：`judge_a_takeover_b_pending` / `judge_kept_a` 写库（F03：保证「Judge 已完成、B 未生成」可恢复）；
  4. 用 `buildCandidatePreview`（读 canonical 文本）+ `buildTakeoverControlView` 组装 Judge 输入；
  5. 调 `StructuredAssembledJudgement`（`role="takeover_judge"`）；
  6. `takeover == false` / 降级 → `judge_kept_a` → `winner_ready(A)`，返回；
  7. `takeover == true`：
     - `projectWorkingPersona(targetProfile)`；
     - B 主生成：`assembleProjectionPrompt`（subject profile = reply owner）+ `StructuredAssembledWithToolsSchema`（**在 `turn_takeover.go` 内，使 `mutations.go` 仍只有 1 处**）→ `b_frozen`；
     - 用与 A **完全相同**的归一化/校验函数处理 B 的 decision（`normalizeTurnDecision`，见下）；
     - B 携带 takeover context（明确「A 以下内容未发送、动作未执行」+ A 原计划 + 命中规则），但**禁止再次仲裁**（结构上不调用 Judge）；
     - `ReplaceFrozenTurnDecision(B)`；写 `reply_owner_profile_id = B`、`rejected_candidate = A` → `winner_ready(B)`；
  8. B 生成失败 → `FailTurnCognition`，**绝不发送被拒的 A**，不启动第三次生成。
- [x] **A/B 完全同一条归一化链**：把原先内联在 `mutations.go` 的归一化块抽成 `turn_decision.go: normalizeTurnDecision`（`freezeDecisionInfluences` → `applyPersistentSwitchGrant` → `preparePersonalityDecision` → `normalizeResponsePlan` → `resolveCanonicalVisibleReply` → `normalizeConversationResponseMode` → `resolveCapabilityAction` → `normalizeCompositeAction`），A（`mutations.go:778`）与 B（`turn_takeover.go:791`）共用，使「B 通过同一校验链」成为结构事实而非承诺。
- [x] **B 作用域重建（F04/§10）**：`takeover_scope.go: resumeProjectionForReplyOwner`（克隆 projection → 改 `PersonalityRuntime.active_profile_id` → `rescopeEffectivePersona` 重算 owner 的 accepted overlay → `buildContextReferenceIndex` 重 token 化）；`capability_core.go` 的 `relationship_scope` 快照与 `relationship_edit.go` / `relationship_capability.go` 的读写主体均跟随 `reply_owner_profile_id`；`fluctlight_personality_runtime` **从不回写**。
- [x] **禁止反向仲裁**：B 的路径不回调 `applyTurnTakeover`；静态守卫断言该函数在 `turn_takeover.go` 内不出现（`TestTakeoverChainStaticGuards`）。
- [x] **执行资格门（F03）**：只有 `turn_stage == "winner_ready"` 才允许进入 `prepareCapabilityInvocations` 与结算（`mutations.go:856`）。
- [x] **不提前流给前端**：仲裁块（`mutations.go:802-861`）内无 `onChunk` / `onActionResult` / `emitAssistantFrame`；`onChunk` 只在结算之后（`:1181`）用最终 `visible`。
- [x] 冻结记录写入 `active_profile_id` / `reply_owner_profile_id` / `persona_revision` / `overlay_revision` / `scope_revision` / `rule_id` / `rule_content_digest` / `rule_version` / `turn_stage`（R10）。
- [x] **B 自身 schema 名与 Main 区分**：`takeover_reply_response`（`turn_takeover.go:takeoverReplySchemaName`）。Main 的 schema 名 `conversation_turn_response` 同时是「持久切换授权」的开关，接管方结构上禁止写持久主导人格，故其 prompt 不得出现 `persistent_switch` 段。

### 阶段 5 执行记录：链路级测试发现并修复的 3 个真实缺陷

新增 `turn_takeover_chain_test.go`（9 例，全部真实执行、非跳过；见 `### 5.1 链路级验证覆盖`）。首跑即暴露三个只有端到端才会出现、单元测试与静态守卫都照不到的缺陷：

| # | 缺陷 | 症状 | 修复 |
|---|---|---|---|
| D1 | **Core Persona 信封未拆包**：`buildPersonaProfileIndex` / `normalizePersonaSwitchRules` / `initialPersonalityProfileID` / `projectWorkingPersona` / `systemPersonaForProjection` 直接读 `projection.CorePersona["personality_system"]`，但投影把它存成 `{"authority":"hard_constraint","data":<core_persona>}` | 真实环境下 `switching.rules` / `takeover_rules` **全部不可见** → Main 永远拿不到持久切换授权 → 阶段 5d 的条件化 schema 会把 `personality_decision` 从 Main 的 response schema 中**整个移除**（回归既有能力）；接管规则永远选不中，`takeover` 永远 `skipped`。单元测试全绿是因为它们直接传**未拆包**的 `CorePersona` | 新增单一访问器 `corePersonaData()`（`persona_switch_rules.go`，`authority` 标记存在时拆 `data`，无标记时原样返回、幂等），在上述 5 处 persona 语义读取点统一使用 |
| D2 | **覆盖候选未剥离 provider codec 旁挂字段**：`PersistTurnDecision` 会 `delete(decision, "capability_invocations"/"tool_calls")`，`ReplaceFrozenTurnDecision` 没有 | 接管后的 `payload.decision.tool_calls` 与 payload 根级 `capability_invocations` 形成双权威 → `handled` 分支的 `LoadFrozenTurn` 报 `capability_runtime_dual_authority` → **每一次接管都会失败** | 抽出 `stripFrozenDecisionSidecars()`（`cognition.go`），两处写入共用 |
| D3 | **接管生成复用 Main schema 名** | `systemPersonaForProjection` 按 schema 名注入 `persistent_switch` 段，接管生成传 `conversation_turn_response` → 接管 prompt 拿到了「持久主导人格切换」规则段，而它自己的 schema 已把 `personality_decision` 禁掉（自相矛盾，且泄漏 switching 规则给接管方） | 引入 `takeoverReplySchemaName = "takeover_reply_response"`，接管生成用它；`TestTakeoverReplyIsGeneratedInTheReplyOwnerScope` 断言接管 system prompt 不含 `persistent_switch` |

### 5.1 链路级验证覆盖（`turn_takeover_chain_test.go`）

| 验证项（阶段 5 原文） | 覆盖用例 |
|---|---|
| `Judge=false` → 只执行 A，实际内容与候选一致 | `TestTakeoverJudgeDeclinesExecutesTheOriginalCandidate`（`conversation_messages.text` 逐字相等；Judge 恰 1 次；无第二/第三次生成） |
| `Judge=true` → 仅执行 B；A 的根级 state 提案全部未执行 | `TestTakeoverJudgeApprovesExecutesOnlyTheTakeoverReply`（A 提出 appraisal + 已授权的持久切换；断言 `cognition_appraisals`=0、`fluctlight_state_revisions`=0、无 A 文本落库） |
| `takeover_once` → `fluctlight_personality_runtime` 不变；`reply_owner_profile_id`=B | 同上（DB 逐字段校验 active 仍为 `spark`；frozen `turn_persona_scope` 分离 `active` 与 `reply_owner`） |
| **A/B 作用域** | `TestTakeoverReplyIsGeneratedInTheReplyOwnerScope`（A 的 wire system prompt 含 spark 私有标记且不含 twilight 的；B 反之；B 含「未发送」边界规则） |
| `B 生成完成` → 不再仲裁，无第三次人格生成 | 所有接管用例断言 `requestCount(judge)==1`、`requestCount(main)==1`、`requestCount(takeover_reply)==1` |
| `A/B 候选非法` → 进入 Judge 前失败；B 失败不偷偷送 A | `TestInvalidCandidateFailsBeforeTheJudge`、`TestFailingTakeoverGenerationNeverSendsTheRejectedCandidate` |
| **声明规则真的能到达生产轮** | `TestDeclaredRulesReachTheProductionTurn`（Main schema 含 `personality_decision`；`takeover.rule_id` 为声明的那条） |
| query 路径不调 Judge | `TestPureQueryTurnNeverInvokesTheJudge`（`takeover.decision=not_applicable`、`skip_reason=query_continuation`、judge=0） |
| 阶段机与静态守卫 | `TestTurnStageMachineSemantics`、`TestTakeoverChainStaticGuards`（单插入点、无反向仲裁、Judge 单调用、`winner_ready` 执行资格、E5 重置、M3 `FOR UPDATE`、M4 派生字段重建、信封拆包不回归） |

**仍未覆盖（顺延到阶段 6/7，非本次范围）**：混合 `final` 轮的可仲裁语义与 `takeover_reply_budget_exhausted`（阶段 6）；`a_frozen`/`arbitration_decided`/`b_frozen`/`executing` 各边界的崩溃注入与并发版本竞争（阶段 7）；多 profile 记忆/关系 A-only·B-only·shared 的完整读写矩阵（阶段 7，本阶段只验证了 prompt 侧作用域与 projection 重建）。

- **回滚点 R5**：移除 `mutations.go` 中的唯一插入点即回到基线；`ReplaceFrozenTurnDecision` / `AdvanceTurnStage` / `turn_takeover.go` 无其他调用者。

---

## 阶段 6：QUERY 互斥与调用预算守卫（R06、F06）

- [x] 互斥语义按 `design.md` §4.7 修正：互斥的是「**结果依赖型 QUERY continuation 路径**」与「takeover 路径」，**不是**「出现任意 QUERY 工具就禁止 takeover」。
- [x] `responseMode == "query_continuation"` 时跳过整个仲裁块（阶段 5 已实现）；补**静态断言**而非仅运行时判断。
- [x] 混合 `final` 轮：**允许仲裁**；未返回结果的查询结果不得被当成已知事实。
- [x] 接管后的 B 若产出 `responseMode == "query_continuation"` → 受控失败 `takeover_reply_budget_exhausted`：不伪造答案、不调第三次模型；已冻结且适用的真实查询结果可复用为输入。
- [x] 无 Tools 合成路径的返回文本仍汇入同一 `visible` → `conversation_messages` 机制（回归验证，不新增旁路）。
- **验证**：
  - [x] `TestPureQueryTurnNeverInvokesTheJudge`（原计划名 `TestPureQueryPathNeverInvokesJudge`）。
  - [x] `TestMixedQueryActionBatchMayBeArbitratedButNotContinued`（修正后的语义）。
  - [x] `TestTakeoverReplyQueryContinuationFailsClosed`（原计划名 `TestTakeoverBYouQueryContinuationFailsClosed`）。
  - [x] `TestMainGenerationBudgetNeverExceedsTwo`：A + B 合计 ≤ 2；Judge 不计数在内。
  - [x] 回归 `query_continuation_test.go`（全 6 例通过）。

### 6.1 执行记录（2026-09-14）

| 检查项 | 结论 |
|---|---|
| 互斥判定位置 | `turn_takeover.go:applyTurnTakeover` 内 `a_frozen` 分支的**第一条**判断，先于 `selectTurnTakeoverRule` / `StructuredAssembledJudgement` / `StructuredAssembledWithToolsSchema`；跳过时写 `takeover.decision=not_applicable` + `skip_reason=query_continuation` 并直接到 `winner_ready`，**零额外调用**。 |
| 互斥判据 | 只读 `input.ResponseMode`（归一化后的结果），**不**扫能力清单。静态守卫显式禁止 `turn_takeover.go` 出现 `CapabilityExecutionPureQuery` / `validatePureQueryContinuation(`，防止有人把语义退化成「候选里有 QUERY 就禁止仲裁」。 |
| 混合 `final` 轮 | 由 `normalizeConversationResponseMode` 派生（候选不声明 `response_mode` 也成立）：`conversation.reply + memory.recall` 的批次 `validatePureQueryContinuation` 失败 → `final` → **可仲裁**。链路测试断言 Judge 恰 1 次、续调用 0 次、冻结 `response_plan.response_mode == "final"`，并对**冻结后的真实 invocations** 调用同一 validator 断言其非 pure query。 |
| 未返回查询结果 | 因「不 continuation」，模型从未收到任何查询结果。此边界由 `takeoverReplyContextRule` **以系统指令**声明（新增一行「本轮【没有返回任何查询结果】…不得把未返回结果的查询当成已知事实」），不依赖模型从「计划已丢弃」自行推断。放在 takeover 上下文而非 `capabilityConversationPolicyInstruction`：后者有 900 字符预算门（`provider_prompts_test.go:18`）且被 Main 轮共用，扩它会同时放大 Main prompt（F11）。 |
| 预算耗尽受控失败 | `errTakeoverReplyBudgetExhausted` 文本改用常量 `takeoverReplyBudgetExhaustedCode`，使「记录的错误码」与「错误字符串」不可能漂移。`mutations.go` 原先对该错误**直接 return 且不落失败码** → frozen 行停在 `arbitration_decided`（`judge_a_takeover_b_pending`）可被无限重试，而 B 的结构性禁止使其永远无法成功。现改为：`errCognitionTurnSuperseded` 独占「不动本行」分支；其余仲裁失败一律 `FailTurnCognition(code=takeoverFailureCode(err))`，预算耗尽落 `error_code=takeover_reply_budget_exhausted`。 |
| 反向验证 | 曾临时把 `mutations.go` 还原为旧分支（只 return 不记码）→ `TestTakeoverReplyQueryContinuationFailsClosed` 精确失败：`got "frozen" / ""`。断言有牙。 |
| 发送路径 | 未新增旁路：续调用的 `visible` 仍经既有 `conversation_messages` INSERT；失败时**零** assistant 消息（断言 `len(texts)==0`）。 |

- **回滚点 R6**：`applyTurnTakeover` 内删除互斥首判 → 回到「QUERY 轮也走仲裁」；`mutations.go` 删除 `takeoverFailureCode` 调用 → 回到「预算耗尽可重试」。两者均无其他调用者。

---

## 阶段 7：恢复、并发与幂等（R10、F03）

- [x] `LoadFrozenTurn` 读回路径（`mutations.go:703-756`）按 `design.md` §4.8 的**全枚举**处理：

| 读到 | 行为 |
|---|---|
| `winner_ready` | 直接执行胜出候选，**不重调 Judge** |
| `arbitration_decided` + `judge_kept_a` | 置 `winner_ready(A)`，继续 |
| `arbitration_decided` + `judge_a_takeover_b_pending` | 按已持久化结论**继续生成 B**（不重判） |
| `a_frozen`（无仲裁记录） | 正常进入仲裁块 |
| `judge.outcome != ok` 且 `judge_kept_a` | 用 A，不重判 |
| `executing` / `settled` | 拒绝覆盖；已执行动作不重演 |
| 已有 assistant 消息 | 走既有 `recoverFrozenTurnAfterAssistant`（`cognition.go:1262`） |

- [x] 覆盖用「`turn_stage` 条件 UPDATE + `RowsAffected`」**并叠加**既有版本门控（`requireCognitionAuthorityRevisionsTx`，`mutations.go:1123`）；**明确记录 `RowsAffected` 单独不是 CAS**（M3）。
- [x] 胜出者确定后**统一重建派生变量**（`responseMode` / `visible` / `personalityPlan` / `composite` / `capabilityInvocations` / `continuationBaseMessages`）。
- [x] 幂等不变：`frozenID = "frozen_"+stableDigest(inboxID)`（`cognition.go:404`）、`assistant:`+turnID 唯一索引（`migrations/runner.go:2169`）、outbox `ON CONFLICT DO NOTHING`。
- [x] 结算事务回滚语义不变（`mutations.go:1185-1200`）；失败时写 `capability_settlement_failed` 并 quarantine。
- [~] **逻辑调用 vs 物理请求分离计数**（F10）：已提供分离的计数面（`fakeProviderRouter.requestCount` 按 wire schema 计逻辑阶段、`totalRequests()` 计物理 attempt），并在每条恢复断言中要求「零物理请求」；**有界重试策略**归入阶段 10（§8 运行时预算），本阶段不宣称恰好一次。
- **验证**（全部落地，见下方执行记录）：
  - [x] 崩溃恢复测试：在 `a_frozen` / `arbitration_decided`（kept_a 与 b_pending 两支）/ `b_frozen` / `winner_ready` / `executing` 各边界注入中断 → 同一裁决、不重调 Judge、不重复投递。
  - [x] 并发测试：WakeUp 变更 state revision 与 active profile 后，旧快照不覆盖新状态。
  - [x] 被拒候选不进入事实记忆：`cognition_appraisals` / `fluctlight_state_revisions` / `conversation_messages` 无 A 痕迹；`cognition_assessments` / `cognition_decision_proposals` 消费者清单仍为 0（M10，按生产源码扫描断言）。

- **回滚点 R7**：把 `turnStageExecutable` 改回「只认 winner_ready」并删掉 `BeginTurnExecution` 调用 → 回到「飞行中崩溃的轮次被隔离」；两者均无其他调用者。

### 7.1 执行记录（2026-09-14）

| 检查项 | 结论 |
|---|---|
| `executing` 阶段真实化（**缺陷 D5**） | 设计 §4.8 的阶段表把 `executing`/`settled` 列为真实阶段，但生产代码**只有定义与读取、没有写入者** → `applyTurnTakeover` 的 `case turnStageExecuting, turnStageSettled: return error` 是死分支，「在 executing 边界崩溃」这一行**不可达、不可测**。更严重的是：若真写入了 `executing`，恢复会把它当错误 → `FailTurnCognition` → **把可恢复的轮次永久隔离**。现改为：`winner_ready` 过资格门后写 `executing`（`BeginTurnExecution`），`turnStageExecutable` 接受 `winner_ready \| executing`，`executing` 时「不重判、不覆盖、继续执行」，`settled` 时拒绝。 |
| 写入时机 | 资格门通过之后、`prepareCapabilityInvocations` 之前，唯一一处；`frozen.Payload` 同步镜像，避免后续读到旧阶段。 |
| 覆盖拒绝仍结构化 | `ReplaceFrozenTurnDecision` 只接受 `ExpectedStage = arbitration_decided`，而 `executing` 是再往后的阶段 → 一旦进入执行窗口，覆盖**在 SQL 层面不可能命中**（不靠额外判断）。 |
| 回拨注入原语 | 新增 `takeoverChainRewind`：删 assistant 消息 + inbox 复位为 `pending` + 删 `cognition_action_outcomes` + `status='frozen'` + 重写 payload 阶段。**删 action outcome 是必需的**：`persistActionOutcomesTx` 以 `(action_id, call_id)` + `request_digest` 幂等，重放一个内容不同的结算会（正确地）报 `action_outcome_replay_conflict`，那是另一条守卫，不是被测对象。 |
| 回拨必须复用同一用户文本 | `enqueueTurnFactTx` 把「存档文本 == 本次文本」作为幂等校验的一部分，文本不同直接 `ErrConflict`。因此文本提为常量（`takeoverChainApproveUserText` / `DeclineUserText` / `PendingUserText`），防止复制字面量再次踩坑。 |
| `a_frozen` 的基线选择 | 必须用**被拒（decline）轮**做基线：其冻结 decision 就是原始候选。用接管（approve）轮做基线会把 B 的 decision 当成「候选 A」，且 scope 的 reply owner 变成 B，`takeover_rules` 的 `source_profile_id` 不匹配 → 规则选不中、Judge 零调用。 |
| 失效快照 | 回拨到 `winner_ready` 后并发推进 `fluctlight_inner_states.revision` 与 `fluctlight_personality_runtime.active_profile_id` → 结算被 `current_state_revision_stale` 拒绝，零 assistant 消息，持久 active 与 persona revision 均未被回写。 |
| M10 | 按 `productionSourceFiles()` 扫描整个包的非测试源码：`cognition_assessments` / `cognition_decision_proposals` **只有 INSERT，零 `FROM`/`JOIN` 消费者**（证据而非承诺）；被拒候选在 `turn_takeover.go` 内只有 2 处写入（诊断记录 + payload 镜像），其它生产文件 0 引用。 |
| 反向验证 | 把 `turnStageExecutable` 临时改回「只认 winner_ready」→ `TestRecoveryFromExecutingResumesWithoutANewVerdict` 精确失败（`turn_stage_not_executable`），`TestTurnStageMachineSemantics` 同样失败。旧行为会把可恢复轮次隔离，断言有牙。 |

**验证**：新增 `turn_takeover_recovery_test.go` 10 例全 PASS；`go build` / `go vet` 干净；`gofmt -l` 无输出。

---

## 阶段 8：基础缺陷复核与修复（R11、F14）

仅修复**在本次分支复核成立**的缺陷；每个缺陷必须有**精确字段测试**，不得用全对象关键词匹配代替契约断言。

| 线索 | 复核结论（含 R1 修正） | 修复要求 |
|---|---|---|
| traits 形状冲突 | **成立**（`evolution_overlay.go:452-458` 非对象 `traits` → `{}`；两出口形状不一致） | 显式兼容转换或可诊断错误；统一规范形状（与阶段 3 合并）。**描述性原文逐字保留，不臆造数值**（F11） |
| `relationship.role` 归一化丢失 | **部分成立**（`app.go:947-955` 字符串 role 且无 type → `{label:"unknown"}` 丢原值） | 精确保留原 label；断言准确字段路径 |
| 无来源 0.5 默认值 | **成立**（`app.go:907-908/1688/1255/1260`，`app.go:1641-1642`） | 区分默认 / 用户提供 / 模型推导 |
| shared vs profile 作用域 | **成立**（`app.go:1437/1761/1614/1662` 缺省绑到初始 profile，不写 NULL） | 缺省走 `profile_id IS NULL` 共享作用域；断言切换与接管后共享关系/目标仍存在 |
| **时区（历史时间戳）** | **成立 — R1 修正**：此前我以「TOON 扁平渲染」推翻它**不成立**。真实缺陷在 `provider_context.go:626-629`：`parsed.UTC().Format("01-02 15:04:05")` **强制 UTC 且无时区标识** | **小范围修复**：保留/输出时区偏移（最小改动）；加回归断言。**不重写格式系统** |
| TOON 递归 | **未复核**（此前「不成立」的判据不充分） | **改为「尚未充分复核」**；要做就先拿递归调用链证据，否则如实标注未复核 |
| assistant 历史自我举证 | **部分成立**（`provider_context.go:142-151` 把 assistant 标 `actor_self`） | 「曾经说过」不得当作作品/动作/外部事实已存在；补窄断言 |

- [x] **附加（F12，文档/注释级）**：
  - [x] 修正 `capability_runtime.go:363-365` 注释措辞：函数**在结算事务内**；deferred 能力只做本行数据写入与 intent 登记、**无网络 IO**；真正外部执行在事务提交后由 outbox/工作流完成；注明 `PreflightTx` 类入口的实际行为（`preflightImageCapabilityTx` 只读事务内配置，不发起外部调用）。**未改** `mutations.go:1122` 的事务边界与结算语义。
  - [x] 修正 `research/inventory-turn-chain.md` 内部矛盾（18e 行 vs §6 末条）：两处统一为「在结算事务内，但只做 DB 行写入 + intent 登记、无网络 IO」。
- [x] **验证**：每个成立的缺陷一条精确字段测试；未成立/未复核的线索在报告中明确标注（见 8.1）。

### 8.1 执行记录（2026-09-14）

**修复（代码）**

| 缺陷 | 代码改动 | 精确字段测试 |
|---|---|---|
| relationship.role 丢原值 | `app.go` `normalizeInitializationAliases`：原 `role` 字符串成为**首个**候选 label，不再因缺 `type` 退化为 `"unknown"` | `TestInitializationRelationshipRoleKeepsDeclaredStringLabel`（断言 `role.label` 三条：字符串原值 / 结构化对象优先 / 真缺失才 unknown） |
| shared vs profile 作用域 | `app.go` 新增 `initializationScopeProfileID`：缺省 `profile_id` → **共享作用域（SQL NULL）**，显式值仍走 membership 校验；`insertAgency`/`insertRelationshipSeeds` 去掉 `defaultProfileID` 形参；`GoalAuthority`/`IntentionAuthority.Validate` 允许空 ProfileID；`evolution_persistence.go` 四处 `$n` 改 `nullableString(...)` | `TestInitializationScopeDefaultsToSharedRow`（goals/intentions/relationships 逐行 `profile_id` NULL vs 声明值）、`TestInitializationRejectsUndeclaredProfileScope` |
| 无来源 0.5 默认值 | `app.go` 新增 `defaultFieldSource`/`usesDefaultUnit`/`recordInitializationDefaultFieldSources`：未被来源声明的 personality/policy 键与目标 `importance`/`urgency`、意图 `confidence` 按稳定字段路径记入 `provenance.field_sources = "server_default"` | `TestInitializationDefaultFieldSourcesAreExplicit`（9 条路径逐条断言，含「已声明键不得被标记」）、`TestCreateFluctlightRecordsPersonalityDefaultsEndToEnd`（blank_slate 端到端读回 `provenance`） |
| traits 形状冲突（两出口） | `working_persona.go`：Working Persona 的 `personality` 统一走 `normalizeEvolutionPersonalityBaseline`（与 evolution 层同一 reconciler），把扁平 trait 补进规范 `traits.*`（**加法**，保留扁平键），随后剔除空载体 | `TestWorkingPersonaCanonicalizesFlatTraitsShape`、`TestWorkingPersonaDoesNotInventTraitsFromProse`（散文 traits 逐字保留、不臆造数值） |
| 历史时间戳时区 | `provider_context.go` `compactMessageTime`：`parsed.UTC().Format("01-02 15:04:05")` → `parsed.Format("01-02 15:04:05Z07:00")`，保留原偏移并显式标注 | `TestCompactMessageTimePreservesTheOriginalOffset`（`+08:00` / `-05:00` 两个方向）、`TestCompactMessageTimeKeepsDateAndSecondsWithoutSequence`（`Z`） |
| assistant 历史自我举证 | **不改代码**：渲染风格（自身消息用 fluctlight display_name）是设计意图，由既有 `TestCompactRecentMessagesUsesActorUserAndFluctlightDisplayName` 钉死（F14.4：改盘点，不改代码） | 新增窄断言 `TestRecentHistoryNeverAttributesSelfUtteranceToTheUser`：两条渲染路径下自身历史**永不**归属 `actor_user`，保留 `actor_type=fluctlight` 与时间戳 |

**逐条复核结论**

- traits 形状 → **成立**，已修（阶段 3 的 F11 描述性保留 + 本阶段统一出口形状）。
- relationship.role → **成立**，已修。
- 无来源 0.5 默认值 → **成立**，已修（区分手段落在 `provenance.field_sources`）。
- shared vs profile 作用域 → **成立**，已修（缺省写 NULL）。
- 时区（历史时间戳）→ **成立**（R1 修正），已按「最小改动、不重写格式系统」修复。
- TOON 递归 → **尚未充分复核**，如实标注，不声称成立或不成立。
- assistant 历史自我举证 → **部分成立**，补窄断言，不改渲染。

**影响面说明**：`insertAgency` 签名去掉 `defaultProfileID`（`evolution_persistence_test.go` 调用点同步）；共享作用域使 goals/intentions 的 `profile_id` 可为 NULL，读取侧（`filterActiveProfileRows`/`selectActiveProfileRelationships`/`relationship_edit.go`）本就以空 profile_id 为「始终可见」，无需改动。

---

## 阶段 9：~~确定性时间规则求值~~（**已取消**）

**本次不执行**（`design.md` §3.5、§11）。不新增 WakeUp/Reflection 切换，不实现确定性时间窗求值器。

- R07 的 `persistent_deterministic` 以「**分类 + 稳定 ID + 诊断**」交付。
- 报告中必须标注：「deterministic 求值**未实现、未验证**」。

---

## 阶段 10：静态守卫、运行时预算与规范同步（§8、F10、F13）

- [x] 改写 `capability_core_test.go:1026` 守卫为跨文件精确计数（见 `design.md` §8 表格）。
- [x] 新增守卫：
  - 生产代码读取 `takeover.rejected_candidate` 仅限诊断；
  - **三处结算点直接调用 `applyPersonalityDecisionPlanTx` 为 0**（只经 E3 门）；
  - `turn_takeover.go` 内无仲裁入口递归调用；
  - `mutations.go` 中 `StructuredAssembledWithToolsSchema(` 仍为 1。
- [x] **新增运行时预算/阶段测试**（F10，静态守卫的补充）：`TestTurnStageBudgetNeverExceedsTwoMainGenerations`、`TestRecoveryDoesNotReplayCompletedStages`、`TestLogicalInvocationsVsPhysicalAttempts`。
- [x] 扩展 `project_health_architecture_guard_test.go:21` 的白名单语义（若 Judge 消息引入新的 role 位置需显式登记；预期不需要）——核对结论：确无需登记，13 能力计数不变。
- [x] 确认 `TestBuiltinRegistryContainsExactlyThirteenDirectCapabilities`（`capability_core_test.go:586`）仍为 13。
- [x] **统一 research 口径（F13）**：research 只保留代码事实；被否决的「建议/推断」标 superseded 并写明替代设计：
  - `research/inventory-turn-chain.md`：① 「Judge 生成 B 的 decision」→ superseded（Judge 只返回布尔，B 独立主生成）；② 「接管时向正在进行的 A 发取消」→ superseded（A 已生成完毕）；③ 「接管需覆盖 query_continuation 子状态」→ superseded（两者互斥）；④ 18e/§6 事务内外矛盾 → 按 F12 修正措辞；⑤ recent 消息 role 写错 → 按 M8 修正。
  - `research/inventory-persona.md`：① 「takeover 应挂接 `preparePersonalityDecision`/`applyPersonalityDecisionPlanTx`」→ superseded（takeover 绝不写 active）；② 「须同时维护三个 Persona 出口」→ superseded（唯一 Working Persona）；③ 「仓库内 0 条 forced_activation 规则」→ 按 M6 更正。
  - `research/inventory-prompt-schema.md`：① 「无请求大小统计设施」→ 按 M9 更正；② recent role 表 → 按 M8 更正。
  - 明示层级：**request > 经评审 design > implement**；research 的旧建议不是另一个架构权威。
- [x] 清理 `implement.jsonl` / `check.jsonl` 的 `_example` 占位行（F13）——核对：两文件均无 `_example` 行。
- [x] 更新 `.trellis/spec/backend/` 下受影响契约：`persona-layer-contract.md`（reply owner、takeover、**persistent switch 授权模型**）、`structured-turn-contract.md`（调用预算矩阵、QUERY 互斥语义）、`fluctlight-cognitive-runtime.md`（A + 可选 Judge + 可选 B、`turn_stage`）——各追加一个 Scenario 小节（含阶段机/预算/恢复、reply-owner 读写矩阵、结算权威与失败码）。
- [x] 补齐多 profile 读写矩阵（F04，阶段 7/8 顺延项）：新增 `takeover_scope_matrix_test.go` 三例（读矩阵 wire 断言 / 结算写矩阵 / 下一轮恢复持久视角）。

### 阶段 10 执行记录（2026-09-14）

| 事项 | 改动 | 验证 |
|---|---|---|
| 静态守卫重构 | `TestCapabilityRuntimeStaticGuardsPreserveActionSingleCognitionAndGenericQueryContinuation` 改为跨文件精确计数：A/B 主生成各 1 处、Judge 仅 `turn_takeover.go` 1 处（`StructuredAssembledJudgement(`）、`mutations.go` 续调用恰 2 次、`not_proposed` 唯一出口、四个流程文件无具体能力分派、`rejected_candidate` 仅诊断、三处结算点 0 处直调 `applyPersonalityDecisionPlanTx`、`turn_takeover.go` 无递归仲裁。共享扫描助手 `assertProductionOnlyIn`/`productionSourceFiles` 落在 `turn_takeover_recovery_test.go` | 单测通过；**反向验证**：在 `composite_actions.go` 注入 `invocation.CapabilityName ==` 分派后守卫变红，回退后恢复绿 |
| 运行时预算（F10） | 新增 `turn_chain_budget_test.go` 三例：阶段序列预算（final/接管/纯查询三场景的完整调用序列 + 恰好送达一次 + 阶段落库）、逻辑调用 vs 物理请求分离（`schemaSequence()` 断言顺序、Judge 失败三形态 timeout/unavailable/invalid output 均恰 1 次 Judge、0 次 B、`judge_degraded` + `outcome` 落库、0 无归属请求）、五阶段边界恢复不重演 | 单测通过；**反向验证**：在 `turn_takeover.go` 接管路径注入第二次 `judgeTurnTakeover` 调用，预算与静态守卫双双变红，回退后恢复绿 |
| 架构守卫核对 | `TestProjectHealthArchitectureGuardAllowsToolRoleOnlyInQueryContinuation` 白名单语义无需为新 role 登记；13 能力计数不变 | `-v` 单测确认 |
| 多 profile 读写矩阵（F04） | 新增 `takeover_scope_matrix_test.go`：`takeoverScopeMatrixSeed` 播种关系/目标/意图/记忆的 shared + A-only + B-only 全量行；① 读矩阵：A 与 B 的 wire payload 各含「自有行 + shared 行」、互不含对方行、自有关系行遮蔽 shared 行、`current_profile_perspective` 跟随发言者；② 写矩阵 4 子场景：Judge 拒绝→A 行 +1、Judge 批准→B 行 +1、A/B 无自有行→shared 行 +1 且不建新行；③ 下一轮：接管后第二轮 Main 回到 spark 视角、持久 `active_profile_id` 仍 spark、两轮分别写对各自行 | 单测通过；**反向验证 ×3**：结算忽略 reply owner（写错行，抓到）、`selectActiveProfileRelationships` 忽略 profile（读越界，抓到）、`compactMemoriesForProfile` 丢 profile（视角错位，抓到）；均回退并 grep 确认无残留 |
| F13 spec 同步 | `structured-turn-contract.md` 增「Turn Takeover Arbitration, Stage Machine, And Per-Turn Budget」Scenario；`persona-layer-contract.md` 增「Takeover Reply Scope And The Multi-Profile Data Matrix」；`fluctlight-cognitive-runtime.md` 增「Turn Settlement Authority And Reply-Owner Writes」；三份 inventory 的 superseded 标注（5/3/2 条）与层级声明在早前阶段已完成，本次核对无遗漏 | 文档核对 |

**回归**：`go -C apps/core-go test ./internal/core/` 全绿（113.9s，含新增矩阵与预算测试）；`gofmt -l` 无输出；`go vet` 干净。阶段 10（含顺延的 F04 矩阵）关闭；下一阶段为阶段 11（验证与基准）。
- **验证**：全部守卫与契约测试通过；`go -C apps/core-go test -race ./...` 无新增失败。

---

## 阶段 11：验证与基准（R12、§17、§18）

### 11.1 确定性 Runtime 测试

按 §17.1 场景表逐条落地（Fake Provider / Fake Judge / Spy Executor / 可控时钟）。新增/修正的场景：

- `pure QUERY 路径` → 整个跳过仲裁块；
- `QUERY + ACTION 混合候选` → 可仲裁、不 continuation（F06 语义）；
- `persistent switch` → **授权场景**（`cognitive_assessment`）下仍能更新 active，下一轮加载对应人格；
- `B 的持久切换提案` → 不写 active（F01）。

### 11.2 Persona / Prompt 契约测试（§17.2）

A Prompt 不含完整 B；B Prompt 不含完整 A；唯一权威投影；**描述性 traits 原词不丢、无臆造数值**；shared scope 不丢；规则 ID 端到端一致；`profiles` 只剩 `{id,name}`；A 的 System 段不含 `takeover_rules`；请求预算含 tools 与 response_format；被拒候选不成为事实记忆。为当前 A/B 角色卡建立**精确字段测试**，不使用「整个对象某处出现某个词」。

### 11.3 真实模型测试（可选，需可用模型）

`FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 时执行；执行器隔离或 mock，避免真实发送/生图。

- 固定测试集：A 克制恰当→不接管；A 表达困难致误解→接管；A 明确拒绝/边界→不接管；普通技术讨论→不接管；多轮后 A/B 风格保持。
- 先用固定 Candidate 测 Judge，再测 A→Judge→B 全链。含无害注入样例。
- **F14.2**：抗注入结论**只能**来自本节；Fake Judge 的布局测试不构成抗注入证据。
- **多头卡初始化**：确认 `dense_multi_card.txt` 的接管语义是否真被产出（`provider_live_tool_test.go:21-23`）；未运行时如实标注未验证。

### 11.4 性能与成本（§18）

四条路径的变更后 Fake Provider wire 测量已完成：**普通无需 Judge / Judge 保留 A / Judge 接管 B / pure QUERY**。报告统计 A、Judge、B、查询合成的输入估算 Token、脚本输出估算 Token、字符数、字节数和请求次数；A 被丢弃的输出也计入接管路径成本。真实 Provider 的缓存命中、首条可见消息延迟、总耗时和基线 full-request 前后对照仍需要 live 环境，不能由 Fake Provider 估算替代。

### 11.5 序列化大小报告

主请求与 Judge 的**字符数 / 字节数 / 估算 Token 分开给**；说明压缩来源与关键语义是否保留；复用并扩展既有 `AssemblePromptContext` 估算与 wire capture 产出机器可读字节报告。当前报告是 Fake Provider / synthetic fixture 证据，不宣称真实模型 Token 或延迟。

### 11.6 如实标注未实测项

真实模型行为、真实 Token 计费、真实可见延迟、**deterministic 求值（未实现）**、**多头卡接管语义产出（live provider 未运行）**、无 `GO_CORE_TEST_DATABASE_URL` 时未执行的链路断言。

区分**基线失败 / 环境阻塞 / 新增回归**。

### 11.7 执行记录（阶段 11 完成）

**11.1 确定性场景（§17.1）**——覆盖映射：
- pure QUERY 跳过仲裁 → `TestPureQueryTurnNeverInvokesTheJudge`（已有）；
- QUERY+ACTION 混合可仲裁不 continuation → `TestMixedQueryActionBatchMayBeArbitratedButNotContinued`（已有，F06 修正语义）；
- **persistent switch 授权场景** → 既有 `TestPersistentSwitchStillWorksViaAuthorizedScenarios` 只验 active 写入；新增端到端 `TestAuthorizedPersistentSwitchLoadsTheNewPersonaNextTurn`（`phase11_verification_test.go`）：Main 候选带 `personality_decision{switch,spark→twilight,trigger_id=safety}` → Judge 拒绝 → 结算 E3 写 active → **下一轮 Main wire 含 TWILIGHT_SCOPE_MARKER、不含 SPARK_SCOPE_MARKER**，且后续普通轮不动持久 profile。反向验证：`applyPersistentSwitchIfAuthorizedTx` 改为结构空操作 → 测试红（active 仍 spark）；
- B 的持久切换提案不写 active → `TestTakeoverReplyCannotWritePersistentActive`（已有，F01）。

**11.2 Persona/Prompt 契约（§17.2）**——精确字段级断言，覆盖映射：
- A Prompt 不含完整 B / B 不含完整 A、shared 不丢、自有遮蔽 shared → `TestMultiProfileScopeMatrixReadFollowsTheSpeaker`（已有，wire 级 marker）；
- traits 原词不丢、无臆造数值 → `TestWorkingPersonaKeepsDescriptiveTraitsVerbatim` / `DoesNotInventTraitsFromProse` / `CanonicalizesFlatTraitsShape`（已有）；
- 规则 ID 端到端 → `TestDeclaredRulesReachTheProductionTurn`（已有，rule_id 落 frozen）；
- A 的 System 段无 takeover_rules → `TestProviderSystemPersonaHasNoTakeoverRules`（已有）；
- 唯一权威投影 → `TestProviderSystemPromptHasSinglePersonaSource` / `TestProviderRuntimeContextCarriesNoSecondPersonaCopy`（已有）；
- 被拒候选不成事实记忆 → `TestRejectedCandidateLeavesNoFactTrail`（已有）；
- **profiles 只剩 {id,name}** → 新增 `TestProviderRosterCarriesOnlyIdentifierFields`：对唯一 System Persona 出口 `filterCorePersona` 用当前 A/B 卡逐字段断言 roster 每个 entry 的键 ⊆ {id,name,profile_id}、语义 switching 段保留 id/target、active 内容仍经 Working Persona body 到达。反向验证：`filterProfileRosterValue` 放行全部键 → 测试红（泄漏 `personality` 字段）；
- **请求预算含 tools 与 response_format** → 新增 `TestWireBudgetAccountsForToolsAndResponseFormat`：`AssemblePromptContext` 的 `SectionTokens["tools"]/["response_schema"]` 与 `EstimatePromptTokens` 精确相等，`EstimatedInputTokens` 增量 = tools+schema（减空 schema 常数）——messages-only 估算无法替代全 wire 预算。

**11.3 真实模型测试**：未运行（无可用 live provider）。**F14.2 抗注入结论不可用**——Fake Judge 布局测试不构成抗注入证据；`dense_multi_card.txt` 多头卡接管语义产出**未验证**。`FLUCTLIGHT_LIVE_PROVIDER_TEST=1` 门与固定测试集设计保留待环境可用时执行。

**11.4/11.5 性能与成本**：新增 `turn_path_cost_report_test.go`（`TestTurnPathCostMeasurement`）——四路径全 wire 捕获测量，同时断言预算不变量（各路径请求次数、takeover 路径恰 A→Judge→B 三次物理请求）。测量结果（估算 Token≈1.25/字符启发式；字节为重序列化 payload）：

| 路径 | 物理请求 | 输入 Token（估算） | 脚本输出 Token（估算） | 字节 |
|---|---|---|---|---|
| plain_no_judge | 1（A） | 23,439 | 163 | 20,367 |
| judge_keeps_a | 2（A+Judge） | 25,379 | 163 | 22,543 |
| judge_takeover_b | 3（A+Judge+B） | 48,342 | 326（含被丢弃的 A 输出 163） | 43,049 |
| pure_query | 2（A+续合成） | 30,343 | 33 | 27,524 |

Judge 请求为 1,940 估算 Token / 2,176 bytes（主请求约 8.3%）；接管路径输入成本约为普通路径 2.06×。机器可读报告：`apps/core-go/internal/core/testdata/turn_path_cost_report.json`。这些是 Fake Provider wire estimates。

**11.6 未实测项（如实标注）**：
- **环境阻塞**（非基线失败、非回归）：真实模型行为、真实 Token 计费、真实首条可见延迟、缓存命中、F14.2 抗注入、多头卡接管语义产出（live provider）、`dense_multi_card.txt` 产出核对；
- **未实现**：deterministic 求值器；
- Fake Provider 下输出侧为脚本值，仅标注为 `scripted_output_estimated_tokens`，不代表真实模型输出长度。

**回归**：核心包专项和全包测试均须以当前复核命令的实际输出为准；此前的阶段记录不替代本轮最终证据。

---

## 阶段 12：清理与交付物（§19、§20、R12）

- [x] 清理普通 Main Prompt / Schema 中已迁出的切换职责：
  - 未授权场景**不再出现** `personality_decision` 与相关指令；
  - `takeover_rules` 不进入 Main；
  - `profiles` 收敛为 `{id,name}`；
  - **保留**持久切换在**授权场景**下的真实入口（不得一起删掉，否则打断白天/晚上切换）。
- [x] 确认只保留一条正常对话执行链（不留两套人格决策或回复发送方式，不做长期 feature flag 并行）。
- [x] 确认未破坏性覆盖角色卡、未清空历史、未删除旧数据字段。
- [x] 交付物：
  1. 代码（新增/修改/删除文件清单）；
  2. 简洁架构文档（可并入 `design.md` 或 `docs/`）；
  3. 规则迁移表（`design.md` §3 + 实现后的**实际数据核对**，含真实多头卡结论）；
  4. 测试与性能报告。
- [x] 报告需说明：运行方式、旧逻辑清理情况、无法验证或暂未支持的路径。
- [x] 逐项回答 `request.md` §20 的八个问题（见下）。

| # | §20 问题 | 回答依据 |
|---|---|---|
| 1 | A 第一次输出是否确实只是未执行提案？B 接管后连 A 的根级状态变化都未提交？ | 阶段 5 验证；`design.md` §4.2 |
| 2 | 私聊是否仍唯一通过现有 reply Capability/发送管线完成？ | `design.md` §0.5-4；且**唯一文本权威**见 §4.3 |
| 3 | 持续主导人格与本轮发言人格是否分开？B 接管后是否禁止再次仲裁？ | `design.md` §4.1 + §0.4 E1–E5；阶段 2.5/5 验证 |
| 4 | Judge 是否只看到判断所需的小上下文？ | `design.md` §4.4；`TestTakeoverJudgeSeesOnlyBoundedContext` |
| 5 | Main Prompt 是否只包含当前发言人格的唯一 Working Persona？ | `design.md` §4.6；`TestProviderSystemPromptHasSinglePersonaSource` |
| 6 | pure QUERY 与 takeover 的互斥边界是否明确且被测试覆盖？ | `design.md` §4.7（**修正后的语义**）；阶段 6 验证 |
| 7 | 全请求大小、总调用次数和真实可见延迟是否如实测量？ | 阶段 11.4/11.5，含未实测项标注 |
| 8 | 能否通过注册/配置新 profile 和合法规则扩展，而不在 MainAgent 增加角色名或工具名分支？ | 阶段 2 规则归一化 + 静态守卫「无具体能力分派」 |

---

## 检查点总览

| 检查点 | 内容 | 门 |
|---|---|---|
| G0 | 评审 `design.md`（三个待定问题已在 §11 关闭），`task.py start` | **阻塞实施** |
| R1 | 测试基座（仅测试文件 + 时钟注入） | 可回滚 |
| R2 | 规则归一化与授权（独立新文件，未接投影时无行为变化） | 可回滚 |
| R2.5 | **权限执行脊 E1–E5**（执行契约；先于仲裁接入） | 可回滚 |
| R3 | Working Persona 投影 / `profiles` 收敛 / 去重（可分段回退） | 可回滚 |
| R4 | 唯一文本权威 + Judge（Judge 可从角色白名单移除即停用） | 可回滚 |
| R5 | 仲裁点接入 + 阶段机（移除唯一插入点即回基线） | 可回滚 |
| G1 | 阶段 5+6+7 完成后：链路级测试与守卫全绿 | **进入基准前必过** |
| G2 | 基准与报告完成后：逐项回答 §20 八问 | **交付前必过** |

### 12.1 执行记录（阶段 12 初版记录，已由本轮复核更新）

**清理核对（12a）**：
- 未授权场景 schema 无 `personality_decision`（`cognitiveTurnResponseSchemaForGrant` + E1 双保险，`TestUnauthorizedScenarioHasNoPersonalityDecisionField`）；
- `takeover_rules` 不进 Main（`TestProviderSystemPersonaHasNoTakeoverRules`）；`profiles` 只剩 `{id,name}`（阶段 11 roster 精确字段测试，反向验证有牙）；
- 授权场景持久切换入口保留（`TestPersistentSwitchStillWorksViaAuthorizedScenarios` + 阶段 11 端到端下一轮加载新人格）；
- **单一执行链**：一次 `HandleTurn` 内完成 A→Judge→B，全走既有 Provider/结算管线；无并行人格决策、无双发送路径、**零 feature flag**（生产代码无 `os.Getenv` 开关、无 `FeatureFlag` 标识）；能力注册表恒 13、静态守卫禁具体能力分派；
- **无破坏性改动**：迁移全为 `CREATE TABLE IF NOT EXISTS`（零 DROP/DELETE）；阶段状态存既有 `cognition_frozen_actions.payload` JSONB，未删旧字段；角色卡与历史数据未覆盖清空。

**交付物（12b）**：
1. 代码清单：24 个既有文件修改（+1088/−275）+ 6 个新生产文件（turn_takeover / turn_decision / persistent_switch_gate / persona_switch_rules / takeover_scope / working_persona）+ 16 个新测试文件（见 `docs/persona-takeover-delivery-report.md` §1）；
2. 架构文档：`docs/persona-takeover-architecture.md`（链路、权威模型、作用域、预算、规则体系、回滚）；
3. 规则迁移表 + 实际数据核对：`docs/persona-takeover-delivery-report.md` §4——四来源逐条核对到测试；真实 `dense_multi_card.txt` prose 路径只保留诊断，固定 `dense_multi_typed_migration.json` 验证授权 typed Provider 输出可产出可执行规则；**live provider 端到端未运行**（如实标注，未放宽 fixture）；
4. 测试与性能报告：同文档 §5——运行方式、四路径成本（plain 1 请求 23.5k tok / judge_keeps_a 2/25.3k / judge_takeover_b 3/48.3k（≈2.06×，含被丢弃 A 输出）/ pure_query 2/30.4k；Judge≈主请求 9%）、JSON 报告路径、未验证路径清单（真实模型/计费/延迟/缓存/F14.2 抗注入/多头卡端到端/deterministic 求值）。

**§20 八问（12c，G2 门）**：逐项回答见 `docs/persona-takeover-delivery-report.md` §6，每问给出结论 + 测试依据 + 设计章节；性能与真实 Provider 项目明确区分“已测 Fake/synthetic”与“环境未验证”。

**成功标准（request.md §20 结尾）**：`A 候选 → Judge → 选择最终发言人格 → 只执行胜出候选` 在副作用、恢复、人格归属与调用预算四维度均成立并被测试锁定；普通 Main LLM 不再承载完整 A+B 人格与全部切换规则的解释负担。

**回归**：`go -C apps/core-go test ./internal/core/` 全绿（118.3s）；`gofmt -l` 无输出；`go vet` 干净。

**状态记录已过时**：本轮复核在最终测试和文档同步完成后才更新 `task.json`；当前剩余的 live provider、真实 Token/缓存/延迟和 deterministic 时间窗求值属于明确未验证/未实现边界，不能写成已完成证据。
