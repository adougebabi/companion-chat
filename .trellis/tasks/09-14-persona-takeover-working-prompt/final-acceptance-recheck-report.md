# 多重人格接管、Working Persona 与 Prompt 收敛：第二轮历史复验报告

> **历史文件（不代表当前状态）**：本文记录整改前的第二轮复验结果。随后已完成 F-02、typed rule、Overlay fail-closed 和恢复字段整改；当前状态以 [`final-acceptance-report-v2.md`](final-acceptance-report-v2.md) 为准。

> 复验日期：2026-09-14  
> 复验基线：`master@7b26364` 之上的当前未提交工作区  
> 前次报告：`final-acceptance-report.md`  
> 复验范围：前次 F-01～F-06 整改、相关生产路径、专项测试、完整 Go/PostgreSQL 测试、Race Detector、交付文档与性能证据  
> 操作边界：本文件对应历史复验；后续复核已修改生产代码、测试和交付文档，不能继续用本文的阻断结论代表当前实现。

## 1. 复验结论

**结论：仍未通过最终验收，但整改取得了实质进展。**

前次 6 项发现的当前状态如下：

| 前次发现 | 第二轮结论 | 说明 |
|---|---|---|
| F-01 回复提案权威 | **按修订契约通过，文档需清理** | 当前 `request.md:15` 新增了 path b 产品修订；根字段优先、reply fallback、冲突 fail closed 已实现。新测试文件头注释仍声称 reply-only 应拒绝，与实际代码/测试相反 |
| F-02 Judge 前权限校验 | **部分整改，仍阻断** | surface、metadata owner、output target kind 已前置；真实 Capability 参数级权限与冻结 scope 仍在 Judge 后验证 |
| F-03 旧规则语义 heuristic | **heuristic 已移除，迁移未完成，仍阻断** | 无显式 kind 的 prose 不再产生 takeover；仓库真实 A/B 卡因此得到 0 条可执行规则，与设计、执行计划和交付报告的声明冲突 |
| F-04 pending verdict 恢复 | **通过** | 恢复按冻结 rule/target/condition/version 重建，不再重新选择当前规则 |
| F-05 B Overlay 进入 Judge | **happy path 通过，错误路径部分符合** | B accepted overlay 已合并；加载失败时静默回退声明基线并继续 Judge，可能基于过期 stance 接管 |
| F-06 `life_profile` 与成本对照 | **裁剪通过，性能验收仍部分通过** | 每轮只保留 `preferences` 与 `character_constraints`；新增报告只比较合成 life_profile/Working Persona 字节，不是同一真实卡的 full request 前后对照 |

本轮新增专项测试、完整无缓存测试和 Race Detector 全部通过，说明代码没有普通构建、测试或并发质量回归。最终验收仍被 F-02 与 F-03 阻断；F-05 的错误降级和性能/文档证据需要继续收口。

## 2. Standards 轴

**结论：部分通过。**

上一轮指出的 `forced_activation` kind 关键词/正则分类已经移除。当前 `declaredSwitchRuleKind` 只接受显式 enum，真实 prose 会变成 `unclassified`，并且新增了架构守卫。这个方向符合 `.trellis/spec/backend/fluctlight-cognitive-runtime.md:167-176` 与 `.trellis/spec/backend/persona-layer-contract.md:40`。

仍有一处边界需要收紧：`persona_switch_rules.go:333-370` 与 `:845-875` 会通过 `strings.Contains`/正则在 condition prose 中查找 profile id/name，并把唯一命中的 profile 当成可执行 target。对 `takeover_rules[]`，容器本身提供了 takeover kind；对显式 `forced_activation.kind=turn_takeover`，kind 也是结构化事实，因此这不再属于“从关键词判断 takeover/persistent kind”的原问题。但是 target 决定谁取得本轮发言权，也属于执行语义。为了完全满足“只执行类型化、授权规则”，可执行规则应要求显式 `target_profile_id`；自然语言 name mention 应只用于迁移诊断，不应补齐缺失 target。

## 3. Spec 轴

**结论：不通过。**

主链、阶段机、winner-only execution、scope matrix、调用预算、F-01 修订契约、F-04 恢复和 F-06 裁剪均有实现与测试证据。F-02 仍未覆盖原始要求中的真实权限和基本约束；F-03 移除 heuristic 后没有完成真实卡的 typed rule 迁移，导致仓库目标卡的接管机制不可执行。实现文档没有同步反映这一变化。

## 4. 阻断性发现

### R2-01（P1，阻断）：Judge 前的“权限校验”仍未检查真实 Capability 参数权限

本轮新增 `validateCandidateCapabilityInvocations`，见 `apps/core-go/internal/core/capability_runtime.go:233-271`。它前置验证了：

- Capability 是否允许当前 surface；
- invocation metadata 中的 Fluctlight ID 是否等于候选 ID；
- invocation metadata 中的 Conversation ID 是否等于候选 ID；
- OutputBinding target kind 是否包含在 Capability definition 中。

这些检查有价值，但没有读取 Capability arguments，也没有根据已冻结的 ContextProjection/ContextSnapshot 验证真实访问范围。

最直接的反例是 `relationship.lookup`：

- 输入参数是 `target_actor_id`，见 `relationship_capability.go:12-25`；
- target 是否在 `RelationshipScope.authorized_actor_ids` 中，要到 `relationship_capability.go:52-65` 的 Execute 阶段才检查；
- 完整 Context Resolve/Preflight/Prepare 仍位于 `capability_core.go:1414-1437`，发生在 Judge 和 winner 选择之后；
- Judge 前 validator 只看 Metadata/OutputBinding，不会解析 `target_actor_id`。

因此以下反例仍成立：

```text
A: final reply + relationship.lookup({target_actor_id: foreign_actor})
  → 参数 Schema 合法
  → surface 合法
  → metadata owner 合法
  → 无 OutputBinding，可跳过 target-kind 检查
  → 进入 Judge
  → Judge=true 后 B 覆盖 A
  → A 的真实未授权 target 被接管掩盖
```

最终 Execute 对胜出候选仍会拒绝未授权 target，所以这不是“未授权副作用已经发生”的结论；它仍违反 `request.md:79` 的“非法候选不能靠 B 接管掩盖”和 `request.md:163/296` 的 A/B 等价权限校验要求。

新增 `turn_takeover_f02_test.go` 只用合成 Capability 验证非法 surface/metadata/target kind，没有测试实际内置 Capability 的参数级授权，也没有测试缺少必要冻结 context 的基本约束。F-02 尚不能关闭。

**复验条件**：为 Capability 增加无副作用的 candidate validation 合约，接收已冻结的相关 ContextSlot；至少覆盖 `relationship.lookup.target_actor_id` 与其他存在本地权限/资格约束的内置 Capability。网络和可重试外部 Preflight 可以继续留在 winner-only Prepare。新增真实 `relationship.lookup` 越权 A 测试，断言 Judge 调用次数为 0。

### R2-02（P1，阻断）：移除 heuristic 后，仓库真实 A/B 卡没有完成 typed takeover 迁移

当前 `persona_switch_rules.go:560-633` 只有在 `forced_activation` 对象带显式合法 `kind` 时才分类；仓库真实卡 `dense_multi_card.txt` 的 `forced_activation` 是 prose，没有 `kind`。

新版 `TestPersonaSwitchRulesNormalizeTheRealMultiProfileCard` 在 `persona_switch_rules_test.go:492-509` 明确断言：

- 真实卡归一化得到 0 条 executable takeover；
- prose 只产生 `unclassified` 诊断；
- 所有保留规则 `Enabled=false`。

这证明 F-03 的关键词分类已删除，也证明目标卡当前没有可执行接管规则。现有 deterministic takeover 链路测试使用测试代码注入的 typed `takeover_rules`，不能证明真实卡已经完成迁移。

该行为与以下当前交付材料冲突：

- `design.md:140`：语义明确的 `forced_activation` 必须可执行；
- `design.md:208`：真实 dense card 应归一化出一条可执行 takeover；
- `implement.md:53/134/618`：声称真实卡接管语义可产出 executable rule；
- `docs/persona-takeover-architecture.md:53`：声称 forced_activation prose 按语义解析并可执行；
- `docs/persona-takeover-delivery-report.md:52/56`：声称散文/对象/列表可产出 executable rule，真实卡在归一化层已验证。

这不是单纯的文档过期：任务优先覆盖现有 A/B 卡，而该卡的核心 takeover declaration 现在不会执行。真实 Provider 测试又处于 SKIP 状态，没有证据证明初始化模型会额外产出 typed `takeover_rules`。

**复验条件**：选择一个不违反语义所有权规范的迁移边界，并形成实际交付。例如：

1. 在初始化/迁移 Provider 的结构化输出中产生 `takeover_rules[]`，保存带 `id/kind/target_profile_id/condition/version` 的结果，并用固定 Provider fixture 验证；或
2. 为仓库目标卡提供经授权、版本化的 typed sidecar/config migration，不修改原始 prose，并证明运行时能加载；或
3. 明确将真实卡迁移排除出本版范围，同时修改原始任务和所有交付声明；当前任务文本并未允许这一缩减。

生产 Go Runtime 不应恢复关键词/正则语义判断。typed rule 必须带显式 target；profile name mention 可保留为迁移诊断。

## 5. 重要发现

### R2-03（P2）：B Overlay 加载失败时静默使用声明基线继续 Judge

`apps/core-go/internal/core/turn_takeover.go:157-179` 的 `projectTakeoverControlView` 会调用 `rescopeEffectivePersona`。当 clone 或 overlay composition 失败时，它直接返回 `buildTakeoverControlView(rule, projection)`，没有错误、诊断或降级标志。

结果是：

- 正常数据库路径能让 Judge 看到 B accepted overlay，专项测试已经证明；
- 数据库/overlay composition 出错时，Judge 仍会运行；
- Judge 看到 B 的旧声明基线，可能批准一个基于过期 stance 的 takeover；
- 该情况不会走既有“Judge 输入不可用时保留已验证 A”的降级策略。

`request.md:111-119` 和 `design.md:379-392` 要求 Full Persona 加 accepted changes 派生 control view。更安全的实现是让 `projectTakeoverControlView` 返回 `(view, error)`；候选 B overlay 无法可靠组成时，记录 bounded diagnostic 并保留 A，不以旧基线继续判断。无 Overlay 记录与 Overlay 读取失败必须区分。

### R2-04（P2）：`life_profile` 裁剪成立，但新增“前后报告”不满足完整性能验收

生产代码已实质修复：

- `working_persona.go:46-56` 定义 life_profile allowlist；
- `working_persona.go:167-170/227-256` 每轮只保留 `preferences` 与 `character_constraints`；
- 外貌、社交背景、生活习惯、周期承诺、关系种子和媒体偏好不再默认进入 Working Persona；
- 新测试验证内容保留与排除。

`life_profile_before_after_report.json` 给出的局部结果是：

| 指标 | Before | After | 变化 |
|---|---:|---:|---:|
| life_profile keys | 8 | 2 | -75% |
| life_profile bytes | 753 | 174 | -76.9% |
| Working Persona bytes | 990 | 432 | -56.4% |

不过，该报告由 `working_persona_test.go:352-389` 手工构造的 `denseLifeProfileFixture` 生成，属于“real-shaped”合成 fixture，不是读取仓库真实卡经初始化后得到的 Persona。它只比较 life_profile 和 Working Persona JSON 字节，没有比较：

- 基线与当前 full request；
- System、Runtime facts、history、memory、tools、response_format、output reserve；
- Judge request；
- 字符、字节、估算/真实 Token 的完整分项；
- 同一输入下的四条路径前后变化；
- 真实缓存、首条消息延迟和总耗时。

因此 F-06 的代码裁剪可以关闭，但 `request.md:321-327` 与 `design.md:405-408` 的完整性能验收仍是**部分通过**。`implement.md:180` 的“同一真实复杂卡 full request / Working Persona / Judge 三组大小已完成”目前没有对应证据。

### R2-05（P2，交付一致性）：现有报告和代码注释互相矛盾

当前 `final-acceptance-report.md` 前半仍写“不通过”和旧 F-01～F-06，后半又追加“全部整改完成”，同一文档不能作为最终验收凭证。建议保留它作为历史第一轮报告，并以本复验报告记录第二轮结论。

F-01 相关材料也有直接冲突：

- 当前 `request.md:15`、`design.md:305-309` 和 `visible_output.go:61-82` 允许 reply-only fallback；
- `TestResolveCanonicalVisibleReplyFallsBackToReplyCapability` 明确验证 fallback；
- `turn_takeover_f01_test.go:15-17` 的文件头却写“canonical 只从根字段解析，reply 不再作为提案源，reply-only 拒绝”；
- 同文件实际只有 root-only 和冲突失败测试，没有 end-to-end reply-only 测试。

另外，`docs/persona-takeover-architecture.md` 与 `docs/persona-takeover-delivery-report.md` 仍描述旧的 prose heuristic 行为。交付文档应按最终生产行为统一重写，删除已放弃的声明和过度结论。

## 6. 已关闭的整改项

### F-01：按当前修订契约通过

当前 task request 新增了 path b 修订：根 visible text 是优先协议，`conversation.reply` 可作为 reply-only fallback，多个来源不一致则 fail closed。代码在 `turn_decision.go:125-145` 检测 `canonical.Conflict` 并返回 `visible_text_source_conflict`；专项链路测试证明冲突发生时 Judge 不调用、消息不送达。

本结论以 `request.md:15` 记录的产品修订为前提。若该行没有真实产品授权、仍要求原始 `request.md:13` 的“Main 必须通过 tool_calls 提议私聊”，那么 root-only accepted 仍是原始 F-01；届时需要重新打开该阻断项。

### F-04：通过

`turn_takeover.go:612-661` 的恢复路径调用 `resumeTakeoverRuleFromPayload`，从冻结记录重建 rule，不再调用 `selectTurnTakeoverRule`。正向和 corrupted-target 反向测试均通过，Judge 不重调，B generation 只调用一次。

建议的小型加固：`resumeTakeoverRuleFromPayload` 当前允许空 `rule_version` 和空 `rule_content_digest`。新路径正常会写入这些字段，但既然注释承诺验证 version/digest，恢复函数应把缺失字段视为无效，而不是接受空值。

### F-05 happy path：通过

`projectTakeoverControlView` 会为非激活 B 重新组合 accepted Overlay；数据库测试证明 Judge serialized control view 得到 overlay 后的 `traits.openness=0.55`，persistent active/reply owner 仍保持 A。错误路径问题见 R2-03。

### F-06 生产裁剪：通过

Working Persona 不再完整携带 shared `life_profile`，allowlist 和内容测试均通过。完整性能报告问题见 R2-04。

## 7. 验证结果

### 7.1 格式、静态检查与构建

以下命令全部退出 0：

```bash
gofmt -l $(rg --files apps/core-go -g '*.go')
git diff --check HEAD
cd apps/core-go && go vet ./...
cd apps/core-go && go build ./...
```

`gofmt -l` 与 `git diff --check` 均无输出。

### 7.2 整改专项测试

使用隔离 PostgreSQL 测试库运行 13 个 F-01～F-06 专项测试，全部通过，耗时 11.171 秒：

- F-01：root-only、文本冲突 fail closed；
- F-02：deterministic synthetic authorization、非法 A、非法 B；
- F-03：架构守卫、真实卡 prose unclassified；
- F-04：冻结规则恢复正反例；
- F-05：B overlay 与 no-overlay baseline；
- F-06：life_profile allowlist 与局部前后比较。

### 7.3 无缓存完整测试

```bash
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go test -count=1 ./...
```

结果：全部通过，退出码 0。

| Package | 结果 | 时间 |
|---|---:|---:|
| `internal/core` | PASS | 150.717s |
| `internal/httpapi` | PASS | 2.124s |
| `internal/migrations` | PASS | 31.835s |
| `internal/platform` | PASS | 1.391s |
| `internal/workflow` | PASS | 1.941s |
| `internal/config` | PASS | 0.381s |
| `cmd/*` | 无测试文件 | — |

### 7.4 Race Detector

```bash
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go test -race -count=1 ./...
```

结果：全部通过，退出码 0。

| Package | 结果 | 时间 |
|---|---:|---:|
| `internal/core` | PASS | 171.967s |
| `internal/httpapi` | PASS | 5.231s |
| `internal/migrations` | PASS | 34.244s |
| `internal/platform` | PASS | 2.719s |
| `internal/workflow` | PASS | 3.294s |
| `internal/config` | PASS | 1.634s |

### 7.5 真实 Provider 测试

再次显式清空 live 环境变量并运行 `TestLiveProvider*`。命令整体 PASS，但以下 8 项全部为 **SKIP**：

- `TestLiveProviderDenseSingleInitialization`
- `TestLiveProviderDenseMultiInitialization`
- `TestLiveProviderDenseExternalInitialization`
- `TestLiveProviderRecognizesImageGenerationIntent`
- `TestLiveProviderRoleOrganization`
- `TestLiveProviderActiveMemory`
- `TestLiveProviderRecallContinuation`
- `TestLiveProviderComplexMultiPersonalityInitialization`

真实模型行为、初始化 typed takeover 产出、误报/漏报、提示注入、真实 Token/缓存/延迟仍未验证。

## 8. 第二轮验收矩阵

| 验收领域 | 结论 |
|---|---|
| Candidate → Judge → winner-only execution | 通过 |
| A/B 根级状态与副作用隔离 | 通过（已覆盖路径） |
| F-01 当前 path b 文本契约 | 通过，需清理矛盾注释并补 reply-only E2E |
| Judge 前结构/Schema/surface/owner/target-kind | 通过 |
| Judge 前真实参数级权限与本地资格约束 | **失败** |
| Persistent active 与 turn reply owner 分离 | 通过 |
| B 不递归仲裁/调用预算 | 通过 |
| pure QUERY 与 takeover 互斥 | 通过 |
| legacy prose 不做 kind heuristic | 通过 |
| 仓库真实 A/B 卡迁移为 typed executable rule | **失败** |
| 冻结 verdict 恢复不重新选规则 | 通过 |
| B accepted Overlay 进入 Judge happy path | 通过 |
| Overlay 加载失败的安全降级 | 部分通过；当前静默使用 baseline |
| Working Persona life_profile 裁剪 | 通过 |
| 同一真实卡 full-request 前后成本报告 | 部分通过；当前仅局部合成 bytes |
| Go build/vet/format/test/race | 通过 |
| 真实 Provider 行为 | 未执行，全部 SKIP |
| 架构/交付/验收文档一致性 | **失败** |

## 9. 对原始八个问题的第二轮回答

1. **A 是否只是未执行提案，B 接管后 A 根状态是否不提交？**  
   是，限当前确定性测试覆盖的合法候选路径。真实参数级非法 A 仍可能先进入 Judge，但不会在被拒前执行。

2. **私聊是否只有一个正式回复契约？**  
   按当前 `request.md:15` 的 path b 修订，根 visible text 与 reply-only fallback 都会被 Core 归一化成一个 frozen canonical，冲突 fail closed，判定通过。文档必须删除“reply-only 拒绝”和“reply-only 合法”的矛盾表述。

3. **persistent active 与 turn reply owner 是否分离，B 是否禁止再仲裁？**  
   是。

4. **Judge 是否只看到小上下文？**  
   正常路径是；B accepted overlay 已进入 control view。Overlay 加载失败会静默使用基线，仍需安全降级。

5. **Main 是否只有当前发言人格的 Working Persona？**  
   是；shared `life_profile` 也已按 allowlist 裁剪。

6. **pure QUERY 与 takeover 是否互斥并有测试？**  
   是。

7. **全请求大小、调用数和真实延迟是否完整测量？**  
   调用数和变更后 Fake Provider wire 大小已测；life_profile 局部前后 bytes 已新增。真实卡 full request 前后比较、真实 Token/缓存/延迟仍未完成。

8. **能否通过注册/配置扩展 profile 和规则？**  
   typed `takeover_rules` 可以；仓库现有 A/B 卡仍是 prose，当前没有迁移成 typed executable rule，因此实际目标卡扩展闭环未通过。

## 10. 下一次复验门槛

1. 为 Judge 前 candidate validation 增加真实参数级、基于冻结 ContextSlot 的无副作用权限检查；补 `relationship.lookup` foreign actor 反例。
2. 完成仓库真实 A/B 卡从 prose 到 typed `takeover_rules` 的授权迁移，不恢复 Go 关键词/正则语义推断；可执行规则要求显式 target。
3. 让 B Overlay compose 错误传播到 Judge 降级路径，记录诊断并保留 A；不得静默以声明基线继续。
4. 生成同一真实复杂卡、同一输入的基线/当前 full request、Working Persona、Judge 前后对照，包含 chars/bytes/estimated tokens/output reserve/tools/schema；真实 Provider 指标可继续诚实标为未验证。
5. 同步 `request.md`、`design.md`、`implement.md`、architecture、delivery report、F-01 测试注释和历史验收状态，确保每项只有一套最终契约。
6. 重跑本报告第 7 节全部质量门禁。

在上述第 1、2 项完成前，不建议把 Trellis task 从 `in_progress` 改为 completed。
