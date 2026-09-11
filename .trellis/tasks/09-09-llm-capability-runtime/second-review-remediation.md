# Capability Runtime 第二轮复验整改清单

状态：remediation implemented / `accepted_as_prerequisite` / 等待最后统一验收。本文是第二版
direct Capability实现的整改入口；最终代码、测试和disposable真实环境证据已记录在
`remediation-evidence.md`。用户已确认可以启动后继任务；在统一验收前仍不提交、不合并、
不推送。

## 1. 执行顺序与任务边界

- Worktree：`/private/tmp/local-ai-companion-llm-capability-runtime`
- Branch：`codex/llm-capability-runtime`
- Base：`master@6b915bebf2516d27a89b3af834858ab0325895af`
- 所有实现仍未提交；不得在`master`修复，不得丢弃前序session改动。
- `09-10-project-health-evolution`严格依赖本任务完成。在本文全部P0/P1/P2修复、真实环境门禁
  运行、主session重新复验并向用户单独报告之前，不得开始后继任务业务代码或migration。

接手时按顺序读取：

1. `prd.md`
2. `design.md`
3. `implement.md`
4. `review-remediation.md`
5. 本文
6. `remediation-evidence.md`（只作为已通过阶段证据）
7. `research/current-tool-inventory.md`
8. `research/prompt-runtime-baseline.md`
9. 相关`.trellis/spec/backend/`契约

## 2. 第二轮结论

第二版已完成真正的direct Capability主干：11项内建能力直接实现`Capability`，Registry不再
保留legacy executor map，Provider直接消费`[]CapabilityDefinition`，旧adapter/executor生产
链已清理，Image/Schedule planner与ContextResolver主干已出现，visible callback也已移动到
事务提交之后。

但“direct主干存在”不等于Runtime闭环正确。当前仍存在Provider schema不可满足、模型可以
伪造runtime prepared字段、Image语义数据丢失、active migration/replay不安全、Schedule
crash-window重复副作用、structured-turn分裂事务以及Context/Result/error/log边界问题。

该段是第二轮复验时的历史结论。第三轮整改已关闭下文全部P0/P1/P2并通过真实环境门禁；
用户已确认本阶段可以作为Project Health/Evolution的前置条件收口。当前仍**不提交、不合并、
不推送**，后续两个阶段完成后由用户统一验收。

## 3. P0：Provider schema 必须先恢复可满足性

### P0.1 Scene/Presence required property未定义

证据：`apps/core-go/internal/core/native_capabilities.go`中的`sceneCapabilityDefinition`与
`presenceCapabilityDefinition`：

- `confidence`被列为required；
- `properties`没有`confidence`；
- root同时`additionalProperties:false`；
- strict JSON Schema下字段既必须出现又禁止出现。

Scene的`anyOf`第二分支只要求root已经required的`operation`，导致start/switch没有真正要求
`scene/activity`。

整改：

- 在Definition中显式定义`confidence`；
- 用条件schema或runtime语义validator保证`start|switch`要求scene/activity，`end`不要求；
- 增加遍历全部Definition的schema consistency/satisfiability测试；
- 每个内建Capability的最小合法Provider输入必须同时通过schema和runtime validator。

### P0.2 Runtime必须执行完整bounded schema

证据：`validateCapabilitySchemaValue`当前只遍历已知properties，不拒绝未知字段，也没有完整
覆盖array/item、string length、number/integer、enum、bounds、anyOf/oneOf语义。

结果：Provider可把`prepared_concept`塞进Image Arguments，或把`items/local_date/
expected_revision/idempotency_key`塞进Schedule Arguments；Capability看到这些字段后可能跳过
真实Prepare/Planner。

整改：

- Runtime严格执行Definition InputSchema的type/required/properties/additionalProperties、
  enum、minimum/maximum、length/items、anyOf/oneOf与总size；
- Provider Arguments保持immutable；任何未声明prepared/runtime字段直接`invalid_arguments`；
- OutputSchema使用同一完整validator或强类型Result DTO。

### P0.3 Arguments 与 PreparedPayload 物理分离

整改终态：

```text
Provider Arguments (immutable, Definition-validated)
  -> Capability Prepare outside tx
  -> schema-versioned PreparedPayload (Runtime-owned)
  -> freeze canonical invocation
  -> transactional apply / deferred settlement
```

- `PreparedPayload`不能再写回`Arguments`；
- Image/Schedule只信任Capability自己生成并冻结的prepared payload；
- replay读取frozen prepared payload，不能重新planner或接受Provider伪造值；
- snapshot/codec/migration分别校验Arguments与PreparedPayload schema/version。

## 4. P0：Image intent 与 Current Context 不得在media prompt前丢失

当前Image Prepare生成：

```text
intent
context_binding.current_life
context_binding.current_state
context_binding.visual_identity
context_binding.appearance
```

但`compactMediaConceptForProvider`仍主要识别旧`life_context/inner_state`键，allowlist也不包含
root`intent`。实际探针表明“在窗边读书”的intent、current_life和current_state在进入
`mediaPromptInput`前消失，只剩appearance和visual identity。

整改：

- 定义单一内部`PreparedMediaConcept` DTO；
- Prepare、frozen payload、`compactMediaConceptForProvider`、`mediaPromptInput`和Worker只使用
  同一字段名/结构；
- 增加以下端到端fixture，不允许只测Prepare局部map：

```text
thin intent
  -> Capability Prepare
  -> frozen PreparedMediaConcept
  -> compactMediaConceptForProvider
  -> mediaPromptInput
```

- 断言intent、current life、current state、visual identity、appearance逐项保留且不被live
  reread覆盖。

## 5. P0/P1：v1 -> v2 migration/replay必须具有独立revision

当前`migrations/runner.go`仍以旧`0025_llm_queue`作为Head，但本次又加入新的Capability
数据转换。migration ledger无法区分“已完成旧0025”和“已完成Capability cutover”。

其他缺陷：

- active set未覆盖`running` autonomy action；
- active row即使只有错误`capability_runtime_version` key也可能被跳过；
- 旧`tool_results`被删除却没有转换成`capability_results`；
- context projection没有真正转为bounded Slot snapshot；
- replay会重新执行已有completed result的immediate invocation。

整改：

- 创建新的线性Capability migration revision并推进Head/ledger测试；
- active预检覆盖pending/claimed/running cognition、autonomy、schedule/media/workflow payload；
- 完整验证v2 envelope，malformed/dual authority/错误version全部`RAISE EXCEPTION`；
- 转换legacy call/results和bounded snapshot，保留稳定call/action/provider/workflow/idempotency ID；
- completed audit row保持历史可读，不重写、不重新执行；
- replay按CallID/result status跳过已完成调用；
- 运行真实PostgreSQL empty->head、previous-head->new-head、active valid、active malformed、
  rerun与completed-history测试；纯SQL字符串/纯函数测试不能替代。

## 6. P1：Schedule prepare 与CallID幂等必须关闭crash window

当前Planner输出只存在内存中的Invocation更新。immediate schedule side effect可能先提交，而
prepared invocation/result尚未写回frozen payload；崩溃后replay重新读取`{intent}`、重新规划
并可能提交第二个schedule revision。`AcceptSchedule`又使用随机schedule ID，没有消费
Planner/CallID幂等身份。

整改：

- Planner output在任何schedule mutation前写入并冻结PreparedPayload；
- schedule apply使用CallID/ActionID派生stable idempotency boundary和数据库唯一约束；
- live revision/CAS与completed history保持Core-owned；
- crash测试覆盖：planner后freeze前、freeze后apply前、apply commit后result写回前、replay；
- 每个窗口只允许一个schedule revision，replay不得再次调用Planner或重复accept。

## 7. P1：Memory/Affect 与assistant facts必须同事务

当前immediate Capability在assistant caller-owned transaction之前执行。Memory/Affect各自开启
并提交事务；后续assistant/claims/output transaction失败时会留下：

```text
Memory/Affect committed
Assistant/claims absent
Frozen turn failed
```

整改：

- 引入generic transaction-aware Capability apply seam，不按Capability名称分支；
- Provider/Planner Prepare在事务外；Memory/Affect/Scene/Schedule accept/durable intent与
  assistant/frozen result在owning transaction中；
- normal与recovery共享settle/failure helper，required failure回滚visible output；
- optional internal failure保留结构化failed result，但不能留下半个成功turn；
- 增加assistant insert、claims、result persistence、commit failure和recovery集成测试。

## 8. P1：ContextResolver必须成为真实授权与类型边界

当前缺陷：

- `memory_event`只设置未消费的`memory_scope_present`，随后重新查DB；
- Visual Identity只检查resolved Persona非nil，又重新读取core persona；
- 多数Capability持有整个`*App`，继续service-locator模式；
- `relationship.lookup`在空relationship scope时fail open；
- wrapper type checker可能把`PersonaContext`静默重包装成`CurrentStateContext`。

整改：

- `RelationshipScope`分离`AuthorizedActorIDs`和relationship rows；空授权集拒绝全部；
- Slot loader返回带不可伪造Slot tag的封闭value，wrapper不匹配直接`ErrContextResolve`；
- resolved context提供决策/prepare所需语义，live DB只做最终CAS、authorization和idempotency；
- Capability注入窄domain function/service，逐步移除为读取ambient state持有整个App；
- tests覆盖empty authorization、wrong wrapper、unknown/missing slot、cancel、snapshot roundtrip、
  only requested与no unrelated DB read。

## 9. P1：Memory schema不得暴露runtime-owned metadata

当前`memoryCapabilityDefinition`仍暴露：

- `personality_perspectives.profile_id`
- `evidence_refs`
- `provenance`

整改：Provider只决定content/type/confidence/importance/optional emotional significance。profile、
perspective binding、evidence、actor/conversation scope、visibility、provenance、idempotency与
embedding全部由Context/Runtime绑定。增加forbidden-field catalog与minimal-input成功测试。

## 10. P2：错误、结果与日志必须可解释

### P2.1 保留Prepare domain error

`ExecuteCapabilities`不能把所有Prepare错误抹成`capability_prepare_failed`。使用小型typed
`CapabilityError{Code, Retryable, Cause}`，让`schedule_replan_planner_failed`等bounded code
穿过Runtime、Result、workflow与diagnostics，同时保留`errors.Is/As`和`%w`。

### P2.2 完整校验OutputSchema

`ValidateOutput`必须检查类型、enum、bounds、nested items和additional properties；Capability
不能返回仅required key存在但语义类型错误的“成功”Result。

### P2.3 intent日志脱敏

普通`slog`不得记录最多160字符intent原文。只记录presence、rune/byte length、digest、call ID、
capability、duration、status和error code；可读内容只进入受控diagnostic/redaction路径。

## 11. 必需回归与完成门禁

### 11.1 确定性测试

- 全11项Definition schema consistency、最小合法输入和forbidden fields；
- strict additionalProperties、nested types/enum/bounds/anyOf/OutputSchema；
- Arguments不可变与PreparedPayload独立roundtrip/replay；
- Image intent/context完整到media prompt；
- Schedule stable idempotency与四个crash point；
- Memory/Affect/assistant caller-owned transaction rollback/recovery；
- Relationship empty authorization和Slot wrapper mismatch；
- migration active valid/malformed/completed/duplicate/result conversion；
- typed domain errors与intent log不含明文；
- single Main cognition、no`role=tool`continuation、no concrete-name dispatch、no legacy runtime。

### 11.2 全量门禁

```bash
GOCACHE=/tmp/lac-capability-final-test go -C apps/core-go test ./... -count=1
GOCACHE=/tmp/lac-capability-final-race go -C apps/core-go test -race ./... -count=1
GOCACHE=/tmp/lac-capability-final-vet go -C apps/core-go vet ./...
GOCACHE=/tmp/lac-capability-final-build go -C apps/core-go build ./...
GOCACHE=/tmp/lac-capability-gateway-test go -C apps/gateway-go test ./... -count=1
GOCACHE=/tmp/lac-capability-gateway-race go -C apps/gateway-go test -race ./... -count=1
GOCACHE=/tmp/lac-capability-gateway-vet go -C apps/gateway-go vet ./...
GOCACHE=/tmp/lac-capability-gateway-build go -C apps/gateway-go build ./...
test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
git diff --check master
```

### 11.3 真实环境门禁

- `GO_CORE_TEST_DATABASE_URL`：真实Capability migration/replay/CAS/crash-window；
- `TEMPORAL_ADDRESS`：active history replay或部署drain/version gate；
- `REDIS_URL`：outbox/reclaim/poison/duplicate相关回归；
- `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`：只作Provider contract补充，不替代fake确定性测试。

环境未配置时必须报告`NOT RUN`，不能把unit/string test写成通过。PostgreSQL migration与active
workflow证据未取得时，任务不能标记production-ready，也不能解除后继任务门禁。

## 12. 最终完成判定

只有以下条件同时成立，`09-09-llm-capability-runtime`才算处理完：

1. 本文全部P0/P1/P2逐项有代码、测试和disposition；
2. 原`review-remediation.md`的direct Runtime终态仍然成立；
3. Capability migration有独立revision与真实PostgreSQL证据；
4. active workflow有Temporal replay或明确drain/version证据；
5. Core/Gateway全量门禁和架构guards通过；
6. 主session重新检查源码数据流，而不是只复述测试；
7. 向用户单独报告已验证、未验证与remaining risk，并由用户确认前置任务已完成；
8. 然后才允许启动`09-10-project-health-evolution`。
