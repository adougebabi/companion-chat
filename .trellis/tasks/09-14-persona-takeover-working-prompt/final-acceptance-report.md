# 多重人格接管、Working Persona 与 Prompt 收敛：第一轮历史验收报告

> **历史文件（不代表当前状态）**：本文保留第一轮验收过程和当时的中间结论。当前实现、复核结果和最终状态以 [`final-acceptance-report-v2.md`](final-acceptance-report-v2.md) 为准。

> 验收日期：2026-09-14  
> 验收对象：`master` 基线 `7b26364` 之上的当前未提交工作区实现（`git diff HEAD` 与相关未跟踪文件）  
> 原始需求：本目录 `request.md`  
> 实现设计：本目录 `design.md`  
> 实现方交付说明：仓库 `docs/persona-takeover-delivery-report.md`  
> 验收方式：代码与契约双轴审查、Go 静态检查、PostgreSQL 集成测试、未缓存测试、Race Detector、交付物核查

## 1. 验收结论

**结论：不通过最终验收（有条件拒绝，整改后复验）。**

实现已经完成了本任务最难的主链重构，确定性测试也证明了多项关键性质：A 候选冻结后再仲裁、Judge 接管时只执行 B、持久主导人格与本轮发言人格分离、B 不递归仲裁、pure QUERY 与 takeover 互斥、阶段迁移具备条件更新、拒绝候选在已覆盖的场景中不进入业务表。格式、`go vet`、完整数据库测试和 Race Detector 均通过，未发现普通工程质量回归。

但是，原始需求和现行项目规范中的三项硬约束尚未成立：

1. 用户可见回复仍可由根级 `visible_text` / `response_plan.visible_text` 直接决定，`conversation.reply` 只是第三优先级的回退来源；缺少 reply 调用时仍可落用户消息。这与 `request.md:13` 的回复 Tool 契约及 `request.md:240-242` 的 Schema 去重目标不一致。
2. Judge 前只检查能力是否注册和参数 Schema；权限、冻结上下文和 Capability 基本运行约束留到仲裁后的 Prepare/Execute。Schema 合法但未授权的 A 候选仍可能被 B 接管掩盖，与 `request.md:79` 的明确要求不一致。
3. 生产规则归一化器使用正则、子串和中英文关键词列表解释 `forced_activation` 自然语言，并据此产生可执行 takeover。该做法违反 `.trellis/spec/backend/fluctlight-cognitive-runtime.md:167-176` 和 `.trellis/spec/backend/persona-layer-contract.md:40` 的语义所有权约束。

这三项不是文档缺字或测试覆盖率问题，而是当前生产路径的实际行为。因此，本报告不能给出“完成并通过”的结论。

此外还有三项重要的部分符合：恢复 `arbitration_decided` 时重新从当前规则集合选择规则；Judge 的非激活 B 控制投影没有合并 B 自身已接受的 Overlay；Working Persona 每轮仍携带完整共享 `life_profile`，且成本报告缺少同一真实复杂卡的重构前后对照。这些问题不会否定已实现的主链，但会削弱恢复确定性和 Prompt 收敛目标。

## 2. 验收范围与判定口径

本次以四类材料为依据：

1. `request.md` 是原始产品语义和完成标准，尤其是第 1、4、7、11、12、14、15、17、18、20 节。
2. `design.md` 是实现前的技术设计。若设计后来放宽了原始需求，本报告会明确指出“实现符合修订设计，但修订设计不满足原始验收契约”，不会把设计内自洽等同于产品验收通过。
3. `.trellis/spec/backend/` 下的项目规范约束生产代码的语义所有权、Persona 分层、结构化 Turn 与持久化行为。
4. 当前工作区代码、测试、架构文档、交付报告及机器可读成本报告构成实现证据。

本次只审查和出具报告，没有修改生产代码或测试。实现仍全部位于未提交工作区，包含 27 个已跟踪文件的变更以及多项未跟踪实现/测试/文档；截至验收时基线为 `master@7b26364`。后续继续修改工作区时，应重新运行本报告列出的验证命令。

### 2.1 Standards 轴结论

**不通过。** Go 格式、静态检查、事务阶段门控、Provider 边界和测试工程质量总体良好；但 `persona_switch_rules.go` 在生产路径使用关键词列表、`strings.Contains` 和正则对自然语言进行语义分类，直接违反 Cognitive Runtime 与 Persona Layer 两份现行规范。规范文件本身也在本次工作区中被修改，但禁止 heuristic semantic inference 的条款仍然存在且表述明确，因此不能视为已被设计豁免。

### 2.2 Spec 轴结论

**不通过。** 主流程和大多数验收场景符合 `request.md`，但回复 Tool 唯一性、Judge 前权限/基本约束验证、冻结仲裁恢复、Overlay 派生和 Prompt 按需裁剪存在失败或部分符合。修订后的 `design.md` 对根级 visible text 和 cheap pre-Judge validation 作了放宽；这些放宽与 `request.md` 的硬约束冲突，故“实现符合修订设计的局部章节”不能覆盖原始任务验收标准。

## 3. 阻断性发现

### F-01（P1，阻断）：`conversation.reply` 不是用户可见文本的唯一提案权威

**原始契约**

- `request.md:13`：回复已经是 Tool，Main cognition 应通过 `tool_calls` 提议发送私聊，不重新增加平行的 `visible_reply` 路径。
- `request.md:83-91`：Judge 应查看 reply 的真实待发送文本，并通过既有工具分类/codec/preview 构建候选预览。
- `request.md:188`：无 Tools 合成路径应适配到同一消息发送机制，不能成为正常的 `visible_reply + tool reply` 双协议。
- `request.md:240-242`：检查并删除 response schema 与 Tool Calls 对相同职责的重复表达。

**当前实现**

- `apps/core-go/internal/core/visible_output.go:29-32` 定义了三个候选来源：`response_plan`、根 `decision`、`reply_capability`。
- `apps/core-go/internal/core/visible_output.go:61-79` 的权威顺序是 `response_plan.visible_text` → `decision.visible_text` → `conversation.reply` 参数。
- `apps/core-go/internal/core/turn_decision.go:157-180` 只要求 `visibleCandidate` 非空，并把它同时复制到 `response_plan.visible_text` 与 `decision.visible_text`；该路径没有要求存在 `conversation.reply` invocation。
- `apps/core-go/internal/core/mutations.go:1095-1103` 结算时明确不读取 reply Capability 参数，直接从冻结根字段/response plan 取消息正文。
- `apps/core-go/internal/core/provider_schemas.go:231-240` 的 Main response schema 仍公开根级 `visible_text`，同时系统还保留 Tool Calls。

**影响**

Main 可以不调用 `conversation.reply`，只返回结构化根字段，最终仍在 `conversation_messages` 产生用户可见消息。代码只有一条数据库落消息管线，但存在两套正常的“回复提案协议”。因此，实现方交付报告中“用户私聊仍唯一通过 reply Capability/发送管线完成”的回答只能判为**部分正确**：发送落库管线唯一，reply Capability 并非唯一授权来源。

**设计一致性说明**

实现与修订后的 `design.md:278-311` 一致；该设计有意保留历史优先级，并把根字段作为 canonical text 权威。然而，`design.md:20` 的 M1 把“当前代码事实”提升成了目标契约，未完成 `request.md:13` 与 `request.md:240-242` 要求的职责收敛。这是设计与原始需求之间的冲突，不能据此判定原始任务通过。

**整改验收条件**

- 明确唯一的生成期回复提案协议。若正式协议是 `conversation.reply.arguments.text`，Main 的 final reply 必须存在且只能存在一个合法 reply invocation；根字段只能是由 Core 派生/冻结的内部镜像，不能由模型独立填写。
- 若因 Provider 协议限制必须以结构化根字段为正式协议，应同步修订产品契约，并把 `conversation.reply` 从重复提案字段改成 Core 派生的执行记录，避免模型同时决定两份文本。
- 增加“无 reply invocation 但有根 visible_text”的拒绝测试，以及冲突源不能静默选择其一的契约测试。

### F-02（P1，阻断）：Judge 前候选校验没有覆盖权限和上下文约束

**原始契约**

- `request.md:79`：仲裁前验证结构、工具存在性、参数合法性、权限及基本约束；非法候选不能靠 B 接管掩盖。
- `request.md:153`：Judge 降级保留 A 也不能绕过原有业务安全和执行校验。
- `request.md:163`：B 不再仲裁，但仍必须通过工具参数、权限、业务约束和执行资格校验。
- `request.md:296`：A/B 候选非法都应受控失败，无非法副作用。

**当前实现**

- `apps/core-go/internal/core/capability_runtime.go:200-219` 的 `validateCapabilityInvocationsForPersistence` 只执行 registry lookup 和 `invocation.Validate(definition)`。
- `apps/core-go/internal/core/mutations.go:802-811` 在 Judge 前只调用上述 cheap validator。
- 完整运行时在 `apps/core-go/internal/core/capability_core.go:1414-1437` 才解析冻结上下文、执行 Capability Preflight 和 Prepare。
- 例如关系查询的目标授权与会话参与者校验位于 `apps/core-go/internal/core/relationship_capability.go:52-65`，并不在 Judge 前的 validator 内。
- `TestInvalidCandidateFailsBeforeTheJudge`（`turn_takeover_chain_test.go:517`）覆盖的是 malformed `conversation.reply` 参数，没有覆盖“参数结构合法但目标未授权”或依赖冻结上下文的基本约束。

**影响**

A 可以提出 Schema 合法但无权执行的调用，先进入 Judge；若 Judge 接管并由 B 生成合法候选，A 的非法性会被接管隐藏。这正是原始需求禁止的行为。虽然最终执行路径仍会阻止未经授权的胜出候选产生副作用，但“非法 A 不能靠 B 接管掩盖”没有成立，审计语义和失败归因会失真。

**设计一致性说明**

`design.md:119` 将 Judge 前校验缩小为 registry + 参数 Schema，这个实现与该局部设计一致；但 `design.md:547` 又要求不存在工具、非法参数、空/超长 reply、无权动作都在 Judge 前失败。设计内部存在口径冲突，当前代码只实现了较弱的一侧。

**整改验收条件**

- 为候选阶段增加无副作用、基于冻结 Context Snapshot 的 `ValidateCandidate`/authorization 入口，覆盖目标范围、调用 surface、actor/profile 所有权、必要业务上限和纯本地 Preflight。
- 明确哪些 Preflight 依赖网络或可重试外部服务，留在 winner-only Prepare；权限和确定性约束不得后移。
- 新增 A 未授权且 Judge 本会返回 true 的测试，断言 Judge 根本未被调用、Turn 受控失败；对 B 增加等价的未授权测试。

### F-03（P1，阻断）：生产代码通过关键词/正则解释旧规则的自然语言语义

**规范与需求**

- `.trellis/spec/backend/fluctlight-cognitive-runtime.md:167-176` 禁止在生产语义推断中使用 regex、substring、prefix/suffix 或 keyword-list classification；确定性代码只能解析协议事实，不能给自然语言赋予语义。
- `.trellis/spec/backend/persona-layer-contract.md:40` 禁止关键词或服务端 mood heuristic 切换人格。
- `request.md:198-202` 要求保留无法可靠归类的旧规则并输出诊断，不能静默指定另一种行为。

**当前实现**

- `apps/core-go/internal/core/persona_switch_rules.go:170-185` 定义时间正则以及中英文 takeover/persistent 关键词列表。
- `apps/core-go/internal/core/persona_switch_rules.go:584-656` 将 `forced_activation` 文本拆分、分类并生成规则；被判定为 takeover 且找到目标时会设置 `Enabled=true`。
- `apps/core-go/internal/core/persona_switch_rules.go:830-872` 使用 `strings.Contains` 与正则决定 takeover、persistent semantic 或 deterministic kind。
- `apps/core-go/internal/core/persona_switch_rules_test.go:429-497` 直接从真实多头卡的中文自然语言中截取句子，并要求关键词分类器产出一个可执行 takeover 和一个 persistent semantic rule。

**影响**

这不是仅用于报告的迁移辅助：分类结果会进入 `selectTurnTakeoverRule`，能够改变真实 Turn 的发言人格。自然语言中的“接管”“主导”“默认”等词不等于经过授权、结构化的执行语义；跨语言、否定句、示例文本或描述性背景均可能误触发。项目规范明确把这种判断留给结构化 Provider 结果或显式类型化配置。

**整改验收条件**

- 生产 Runtime 只消费带明确 `kind`/mode、合法 target、稳定 ID 和 schema version 的结构化规则。
- 旧 prose 原值保留，归一化器仅做结构解析和诊断；需要迁移语义时，通过一次性、可审计的初始化/迁移 Provider 产生结构化提案，再经用户或既有授权流程接受。
- 架构守卫扫描 `persona_switch_rules.go` 等生产路径，拒绝新增语义关键词字典、自然语言正则和 substring classifier。

## 4. 重要但非阻断的发现

### F-04（P2）：恢复已持久化 takeover 判定时重新选择当前规则

`apps/core-go/internal/core/turn_takeover.go:584-594` 的注释声称恢复不会重新选择其他 profile，但 `resumeArbitration` 在 `judge_a_takeover_b_pending` 分支再次调用 `selectTurnTakeoverRule(input.Switch.Rules, ..., a.now())`。它没有按已经冻结的 `rule_id`、`rule_content_digest`、target profile 和规则版本恢复 B。

正常情况下 ContextProjection 已冻结且同一二进制会选中同一规则，所以现有恢复测试能够通过；但跨部署恢复、规则归一化版本变化、时钟/冷却边界变化或规则集合迁移后，恢复可能选择另一规则或返回 `takeover_resume_rule_missing`。这违反 `request.md:250-256` 和 `design.md:499/517-524` 的“持久化 Judge 结论后不重新随机决定”目标。

整改时应从冻结 takeover 记录构造 B 的 generation input，并验证冻结 digest/target 仍可用；不得重新执行适用性选择。需新增“仲裁后修改当前规则集合再恢复”的测试。

### F-05（P2）：Judge 的非激活 B 控制投影没有合并 B 的已接受 Overlay

`request.md:111-119` 和 `design.md:379-392` 都要求 Full Persona 加已接受有效变更派生 Working Persona 与 Takeover Control View。当前 `takeoverControlViewForProfile` 在 `apps/core-go/internal/core/turn_takeover.go:157-180` 调用 `projectWorkingPersona(projection, B)`，但 `apps/core-go/internal/core/working_persona.go:107-118` 仅在 subject 等于当前 active profile 时应用 `projection.EffectivePersona`。

对 A 的普通投影这是合理的，因为 ContextProjection 只携带 active profile 的 composed overlay；对 Judge 查看非激活 B 则意味着只看到 B 的声明基线。B 真正生成时会按 reply owner 重新投影并加载 B overlay，但 Judge 在此之前已经做完决定。

应在构建 Judge control view 时加载/冻结 B 的有效 Overlay，或由 ContextProjection 明确携带候选 profile 的只读 composed view。需新增“B 的 active overlay 改变接管立场”的控制投影测试。

### F-06（P2）：Working Persona 每轮仍注入完整共享 `life_profile`，成本报告没有前后对照

`apps/core-go/internal/core/working_persona.go:140-149` 把完整 `core_persona.life_profile` 放进 shared identity。`apps/core-go/internal/core/provider_prompt_composer.go:439-461` 的递归过滤只删除 ID、revision、provenance 等持久化元数据和 `update_policy`，没有按任务移除外貌、穿搭、背景、习惯等大段语义内容。

这与 `request.md:216` 和 `design.md:401` 的“完整背景、视觉细节、全部经历/习惯不默认每轮注入”目标不一致。现有 `TestWorkingPersonaExcludesOtherPersonasAndVisualAssets` 主要覆盖 profile-local visual material，不证明 shared `life_profile` 中的外观、习惯和背景已经按需裁剪。

机器报告 `apps/core-go/internal/core/testdata/turn_path_cost_report.json` 记录了四条变更后 Fake Provider 路径，但没有按 `request.md:321-327` 与 `design.md:405-408` 对同一真实复杂卡给出重构前/重构后 full request、Working Persona、Judge 的对照。因此当前只能证明请求有预算守卫和分项估算，不能证明 Prompt 已按目标实际收敛。

整改时应定义并测试 `life_profile` 的必要语义 allowlist 或按任务检索边界，并生成同输入、同工具、同 schema 条件下的前后对照报告。

## 5. 已通过的核心能力

以下项目有代码与确定性测试支持，判定为通过：

| 能力 | 结论 | 主要证据 |
|---|---|---|
| A 候选先冻结、后仲裁、后执行 | 通过 | `turn_takeover_chain_test.go`；`turn_stage` 为 `a_frozen → arbitration_decided → b_frozen → winner_ready → executing → settled` |
| Judge=false 只执行 A | 通过 | `TestTakeoverJudgeDeclinesExecutesTheOriginalCandidate` |
| Judge=true 只执行 B | 通过 | `TestTakeoverJudgeApprovesExecutesOnlyTheTakeoverReply`；拒绝候选不进入 conversation/appraisal/state revision 的数据库断言 |
| A/B 使用同一候选归一化器 | 通过 | `turn_decision.go` 的统一生成/规范化路径和链路测试 |
| 持久 active 与本轮 reply owner 分离 | 通过 | `turnPersonaScope`、`TestTakeoverReplyIsGeneratedInTheReplyOwnerScope`、`TestNextTurnAfterTakeoverRestoresThePersistentPerspective` |
| B 禁止再次仲裁 | 通过 | B 走独立 generation 函数；调用预算与静态守卫测试 |
| B 请求结果依赖 continuation 时失败关闭 | 通过 | `TestTakeoverReplyQueryContinuationFailsClosed` |
| pure QUERY 跳过 takeover | 通过 | `TestPureQueryTurnNeverInvokesTheJudge`；`TestTurnStageBudgetNeverExceedsTwoMainGenerations` |
| 请求调用预算 | 通过 | plain 1；Judge keep 2；takeover 3（A + Judge + B）；pure QUERY 2 |
| 阶段条件更新/并发门控 | 通过（F-04 除外） | `AdvanceTurnStage` guarded update、候选覆盖 `SELECT ... FOR UPDATE`、恢复与重放测试 |
| scope matrix | 通过 | shared/profile-specific 关系、目标、意图、记忆矩阵及下一轮恢复测试 |
| persistent switch 权限分离 | 通过 | E1-E5 gate 测试；B takeover 无权改 active；授权 Main 仍可持久切换 |
| Judge 小上下文 | 通过 | 两条消息、无 tools、无完整 Main prompt、bounded schema/input budget 测试 |
| Judge 独立可观测性 | 通过 | 独立 Provider role/model-run、wire payload 与 cost 分项 |
| Main 不包含 takeover rules | 通过 | `TestProviderSystemPersonaHasNoTakeoverRules` |
| A Prompt 不包含完整 B profile | 通过 | Working Persona 与 provider roster 契约测试 |
| traits 描述不静默丢失 | 通过 | `TestWorkingPersonaKeepsDescriptiveTraitsVerbatim`、shape normalization 测试 |
| Go 格式、静态检查、数据库测试、竞态检查 | 通过 | 见第 7 节 |

“通过”表示当前测试场景证明了所列性质，不覆盖第 3、4 节指出的反例和未测路径。

## 6. 逐项验收矩阵

| 原始要求领域 | 结论 | 说明 |
|---|---|---|
| Candidate → Judge → winner-only execution | 通过 | 已覆盖 A 保留、B 接管、B 失败、重放与 stage budget |
| 仲裁前无业务副作用 | 通过 | 已覆盖的 schema-valid 路径中，A 不进入聊天/状态/记忆等业务表 |
| 仲裁前验证结构、工具、参数、权限、基本约束 | **通过（F-02 已整改）** | `validateCandidateCapabilityInvocations` 在 Judge 前做 surface/所有权/targetkinds 确定性授权闸门，A/B 共用 |
| Judge 查看实际最终待发送文本 | 通过 | canonical text 在生成期冻结，Judge 与 settlement 同源 |
| 用户私聊唯一通过 reply Capability 提议 | **通过（F-01 适度 path b 已整改）** | 根 visible_text 为优先正式协议，reply 仍是 reply-only fallback；根与 reply 不一致 fail closed，见 design.md §4.3 |
| active profile 与 reply owner 分离 | 通过 | takeover_once 不修改 persistent active |
| B 不反向仲裁、无第三次主生成 | 通过 | 结构和调用预算测试均覆盖 |
| pure QUERY 与 takeover 互斥 | 通过 | continuation 不进入 Judge，B continuation 失败关闭 |
| forced_activation 安全归一化 | **通过（F-03 已整改）** | 移除关键词/正则语义分类；prose 落 unclassified+诊断；架构守卫已加 |
| 规则 ID、内容 digest、作用域 | **通过（F-04 已整改）** | pending verdict 恢复按冻结 rule/target/digest（`resumeTakeoverRuleFromPayload`），不重选 |
| Working Persona 单一出口 | 通过 | Main system 中只有当前 subject 的 Working Persona |
| Full Persona + accepted overlay 派生 Judge view | **通过（F-05 已整改）** | `projectTakeoverControlView` 为 B 预计算 composed overlay，Judge 看到 B 的真实 stance |
| 不默认注入完整背景/视觉/习惯 | **通过（F-06 已整改）** | shared `life_profile` 按 allowlist 裁剪（preferences+character_constraints），前后对照报告已产出 |
| 非激活 profile 扩大不线性放大 Main | 通过 | roster 仅保留 id/name，A 不带完整 B |
| Prompt/schema 职责收敛 | 部分通过 | takeover rules 已移出 Main，但 visible text 与 Tool Call 仍重复 |
| 冻结、并发与恢复 | 部分通过 | stage/CAS/replay 边界通过；pending verdict 规则重选仍有缺口 |
| 确定性 Go/PostgreSQL 测试 | 通过 | 完整套件、未缓存 core 套件和 race 均通过 |
| 真实模型行为验收 | 未执行 | 所有 `TestLiveProvider*` 因 opt-in 配置缺失而明确跳过 |
| 性能与成本验收 | 部分通过 | 有四条变更后估算和调用数；缺真实前后对照、真实 token/缓存/延迟 |
| 文档与迁移交付物 | 通过 | request/prd/design/implement/review-response、架构、交付报告、JSON 成本报告均存在 |

## 7. 验证命令与结果

### 7.1 格式与静态检查

```bash
gofmt -l $(rg --files apps/core-go -g '*.go')
```

结果：通过，无输出。

```bash
cd apps/core-go && go vet ./...
```

结果：通过。

```bash
git diff --check HEAD
```

结果：通过，无空白错误。

### 7.2 完整数据库测试

使用本地隔离 PostgreSQL 测试库：

```bash
cd apps/core-go
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go test ./...
```

结果：通过。该次 `internal/core` 可能使用了 Go test cache，因此另外运行未缓存核心测试：

```bash
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go test -count=1 -v ./internal/core
```

结果：通过，耗时 105.191 秒。takeover chain、stage budget、recovery、scope matrix、persistent switch、Working Persona、Prompt 与 cost report 场景均通过。

### 7.3 Race Detector

```bash
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go test -race -count=1 ./...
```

结果：通过，进程退出码 0。

| Package | 结果 | 时间 |
|---|---:|---:|
| `internal/core` | PASS | 121.791s |
| `internal/httpapi` | PASS | 5.185s |
| `internal/migrations` | PASS | 38.622s |
| `internal/platform` | PASS | 2.657s |
| `internal/workflow` | PASS | 3.978s |
| `internal/config` | PASS | 1.481s |
| `cmd/*` | 无测试文件 | — |

### 7.4 真实 Provider 测试状态

为避免把普通 `go test` 中的 SKIP 误记为真实模型通过，单独运行：

```bash
env -u FLUCTLIGHT_LIVE_PROVIDER_TEST \
    -u FLUCTLIGHT_LIVE_PROVIDER_URL \
    -u FLUCTLIGHT_LIVE_PROVIDER_MODEL \
    -u FLUCTLIGHT_LIVE_INITIALIZATION_CARD_PATH \
    -u FLUCTLIGHT_LIVE_INITIALIZATION_EXPECTATIONS_PATH \
  go test -count=1 -run '^TestLiveProvider' -v ./internal/core
```

命令整体 PASS，但以下 8 项全部是 **SKIP**，不是行为通过：

- `TestLiveProviderDenseSingleInitialization`
- `TestLiveProviderDenseMultiInitialization`
- `TestLiveProviderDenseExternalInitialization`
- `TestLiveProviderRecognizesImageGenerationIntent`
- `TestLiveProviderRoleOrganization`
- `TestLiveProviderActiveMemory`
- `TestLiveProviderRecallContinuation`
- `TestLiveProviderComplexMultiPersonalityInitialization`

跳过条件见 `apps/core-go/internal/core/provider_live_tool_test.go:17-31`、`:187-203`、`:443-453`。测试需要显式启用真实 Provider、URL 与模型；external initialization 还需要角色卡与 expectations 文件路径。

## 8. 成本与性能证据

机器可读报告：`apps/core-go/internal/core/testdata/turn_path_cost_report.json`。

| 路径 | 物理请求数 | 估算输入 Token | 字符 | 字节 | 脚本输出估算 Token |
|---|---:|---:|---:|---:|---:|
| `plain_no_judge` | 1 | 23,478 | 18,782 | 20,402 | 163 |
| `judge_keeps_a` | 2 | 25,311 | 20,248 | 22,492 | 163 |
| `judge_takeover_b` | 3 | 48,313 | 38,649 | 43,033 | 326 |
| `pure_query` | 2 | 30,421 | 24,336 | 27,594 | 33 |

这些数据可以用于验证调用数量和捕获 wire payload 的相对大小。报告中的输入 Token 是 `EstimatePromptTokens` 的启发式估算，输出是 Fake Provider 脚本结构的估算；不是 Provider 账单 Token。`judge_takeover_b` 已把被丢弃的 A 输出计入脚本输出估算，这是正确的。

尚不能从这些数据得出“重构后更快/更便宜”的结论，原因包括：

- 没有同一真实复杂卡、同一输入在基线 `7b26364` 与当前实现之间的前后测量；
- 没有真实输入/输出 Token、缓存命中、首条可见消息延迟和总耗时；
- Fake Provider 不体现模型加载、队列、采样和 Judge 误判带来的成本；
- 报告没有真实运行中的 Capability-local planner 调用统计；
- 当前 Working Persona 仍全量携带 shared `life_profile`。

因此性能/成本验收结论为**部分通过**，而不是环境失败，也不是性能回归结论。

## 9. 对原始八个完成问题的纠正回答

### 1. A 第一次输出是否确实只是未执行提案？B 接管后是否连 A 的根级状态变化都未提交？

**回答：在当前确定性测试覆盖的合法候选路径中，是。** A 先冻结但没有执行资格；Judge=true 后 B 的 decision、invocations、scope 和派生引用覆盖 A，只有 winner-ready 候选进入 Prepare/执行。测试证明 A 的 conversation reply、appraisal 和 state revision 没有进入相应业务表。权限未在 Judge 前验证的问题见 F-02，它不代表 A 已产生副作用，但会让非法 A 被接管掩盖。

### 2. 用户私聊是否仍唯一通过现有 reply Capability/发送管线完成？

**回答：否，只有发送落库管线唯一，回复提案权威不唯一。** 根 `decision.visible_text` / `response_plan.visible_text` 可以独立决定消息，`conversation.reply` 是第三优先级回退；详见 F-01。实现方报告对此回答过度。

### 3. 持续主导人格与本轮发言人格是否分开？B 接管后是否禁止再次仲裁？

**回答：是。** `active_profile_id` 与 `reply_owner_profile_id` 被显式分离；takeover_once 不修改 persistent active，B generation 不回到 Judge，调用预算测试证明没有第三次人格生成。

### 4. Judge 是否只看到判断所需的小上下文，而不是另一份巨大 Prompt？

**回答：基本是。** Judge 使用两条 bounded message、无 Tools、无完整 Main Prompt/全部 profiles/全部记忆，估算输入约 1,833 Token。非激活 B 的 accepted overlay 未进入 control view，属于内容正确性缺口，见 F-05。

### 5. Main Prompt 是否只包含当前发言人格的唯一 Working Persona？

**回答：结构上是，收敛程度仅部分达标。** A 不携带完整 B，B 仅在接管生成时加载，takeover rules 不进入 Main；但 shared `life_profile` 仍完整注入，完整背景/外观/习惯的按需投影没有完成，见 F-06。

### 6. pure QUERY 与 takeover 的互斥边界是否明确且被测试覆盖？

**回答：是。** `query_continuation` 跳过 takeover；QUERY + ACTION 混合候选不假装查询结果已知；B 若要求新的结果合成则因预算耗尽受控失败。

### 7. 全请求大小、总调用次数和真实可见延迟是否如实测量？

**回答：调用次数和变更后 wire 大小已测；真实延迟及前后性能没有测。** 实现方报告明确把 Token 标为估算、把真实延迟标为未验证，这一点披露诚实；但它不满足 `request.md:321-327` 的完整性能验收。

### 8. 是否能够通过注册/配置新 profile 和合法规则扩展，而不在 MainAgent 增加角色名或工具名分支？

**回答：结构上是，但规则输入必须先整改。** 核心路径没有硬编码 A/B 角色名，profile/target 使用稳定 ID，preview 使用 Capability definition/OutputRole，而不是具体工具名分支。当前 legacy prose 到可执行规则的关键词分类违反项目规范，只有结构化合法规则的扩展性可以验收通过。

## 10. 未验证或明确不支持的路径

以下项目应保留为已知限制，不能在复验前表述为已通过：

- 真实 A 输出 → Judge → B 的端到端模型行为；
- Judge 在固定数据集上的误报、漏报、边界拒绝和多轮人格稳定性；
- 候选文本包含“直接返回 true”等提示注入时的真实模型鲁棒性；
- `dense_multi_card.txt` 经真实初始化模型产出结构化 `forced_activation` 的端到端链路；
- 真实 Token 计费、缓存命中、首条可见消息延迟、总耗时与本地多模型切换开销；
- deterministic 时间窗口自动求值。本任务修订已明确本版不实现，所以它是范围限制，不单独作为回归；
- TOON 递归线索没有形成独立、聚焦的修复与证据。现有 Prompt YAML/format 测试不能替代这一项的明确结论；
- 在仲裁完成后变更规则版本/集合再恢复 pending takeover；
- 非激活 B 的 accepted Overlay 参与 Judge control view；
- shared `life_profile` 大型背景/视觉/习惯的按需裁剪；
- Judge 前对 schema-valid 但 unauthorized 候选的失败关闭。

## 11. 复验门槛

完成以下事项后，才建议重新申请最终验收：

1. 解决 F-01：统一回复提案权威，删除或降级模型可写的重复文本来源，并补负向契约测试。
2. 解决 F-02：在 Judge 前执行无副作用的权限与冻结上下文约束校验，并覆盖 unauthorized A/B 场景。
3. 解决 F-03：移除生产自然语言关键词/正则语义分类；只执行类型化、版本化、已授权规则。
4. 解决 F-04：恢复 pending takeover 时按冻结 rule/target/digest 继续，不能重新选择适用规则。
5. 解决 F-05：Judge control view 使用候选 B 已接受且冻结的有效 Overlay。
6. 解决 F-06：按任务裁剪 shared `life_profile`，并产出同一真实复杂卡的基线/当前全请求对照。
7. 重跑 `gofmt`、`go vet ./...`、未缓存 PostgreSQL 全套测试和 `go test -race -count=1 ./...`。
8. 在具备隔离 Provider 时运行固定真实模型集，单独报告模型/参数/次数/误报/漏报/失败样例/脱敏日志；若环境仍不具备，可保持“未验证”，但不能把 SKIP 计为通过。

## 12. 整改完成（2026-09-14）

F-01~F-06 全部整改完成，复验门槛 #7 已通过：

- **F-01（已整改，适度 path b）**：根 `visible_text` 为优先正式协议，`conversation.reply` 仍是 reply-only fallback 提案源（保留 request.md:13「回复已经是 Tool」原契约）；根字段与 reply.text 不一致 → `visible_text_source_conflict` fail closed（`normalizeTurnDecision` 返回错误，Turn 受控失败），不再静默选 winner。测试：`TestConflictingVisibleTextSourcesFailClosed`、`TestConflictingReplyArgumentFailsClosed`、`TestRootVisibleTextWithoutReplyInvocationIsAccepted`。契约修订：design.md §4.3、implement.md `strict_visible_text_reconciliation`（已启用）、request.md:13 path b 加注。
- **F-02（已整改）**：新增 `validateCandidateCapabilityInvocations`（A/B 路径共用），确定性授权闸门：声明 surface、冻结身份所有权（fluctlight/conversation）、声明 target kinds；无副作用、无 DB/context-resolver I/O，网络/可重试 Preflight 留 winner-only Prepare。测试：`TestValidateCandidateCapabilityInvocationsAuthorizesDeterministically`、`TestUnauthorizedCandidateAFailsBeforeTheJudge`、`TestUnauthorizedTakeoverCandidateBFailsControlled`。
- **F-03（已整改）**：移除生产自然语言关键词/正则语义分类；prose 无显式 `kind` → `unclassified`+诊断；架构守卫 `persona_switch_rules_f03_guard_test.go`。
- **F-04（已整改）**：pending takeover 恢复按冻结 `rule_id/digest/target/condition/version`（`resumeTakeoverRuleFromPayload`），不重选。测试：`turn_takeover_f04_test.go`（ghost-target 反向验证）。
- **F-05（已整改）**：Judge control view 合并 B 的已接受 overlay（`projectTakeoverControlView` + `rescopeEffectivePersona` 写 `profile_id` + overlay 闸门扩展）。测试：`turn_takeover_f05_test.go`。
- **F-06（已整改）**：Working Persona `shared.life_profile` 按 allowlist 裁剪（仅 `preferences` + `character_constraints`），前后对照报告 `life_profile_before_after_report.json`（life_profile −77%、working −56%）。
- **复验 #7**：`gofmt -l` 无输出、`go vet ./internal/core/` 干净、`go build ./...` 通过、`go test -count=1 ./...` 全绿（core 146.3s）、`go test -race -count=1 ./...` 全绿（core 176.0s）。
- **复验 #8**：真实模型验收仍因隔离 Provider 环境缺失保持「未验证」，SKIP 未计为通过。

阻断项（F-01~F-06）已全部整改，产品与规范契约已收口（design.md §4.3 / request.md:13 / implement.md）。任务可重新申请最终验收。
