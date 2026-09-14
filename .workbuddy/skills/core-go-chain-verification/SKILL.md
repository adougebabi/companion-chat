---
name: core-go-chain-verification
description: 为 apps/core-go 的 turn 管线写链路级端到端测试（脚本化 Provider + 隔离 DB），并在改动后跑全量回归与静态守卫。当需要验证「一次 HandleTurn 的实际行为」（接管/仲裁、多阶段生成、候选有效性、profile 作用域、可见文本、副作用落点），或为某阶段补 verification 用例与源码守卫时使用。
agent_created: true
---

# core-go 链路级验证

适用范围：`apps/core-go/internal/core`。目标是把「声称的行为」变成对**持久行**与**真实 wire payload** 的断言，而不是对单元 helper 的断言。阶段 2–4 的单测曾全绿却漏掉三个只有端到端才能发现的真实缺陷（见文末）。

## 环境（一次性）

```bash
# 容器（本机无 Docker Desktop，用 OrbStack）
~/.orbstack/bin/orbctl start
docker run -d --name lac-test-pg -e POSTGRES_PASSWORD=fluctlight -e POSTGRES_USER=fluctlight \
  -e POSTGRES_DB=fluctlight -p 55432:5432 pgvector/pgvector:pg16

# 迁移（不跑 → 全红 relation does not exist）
cd <repo> && CORE_GO_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go -C apps/core-go run ./cmd/migrate
```

## 验证命令（每次改动后）

```bash
cd <repo>
GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55432/lac_test?sslmode=disable' \
  go -C apps/core-go test -count=1 ./internal/core/          # 全量，~70-90s；后台跑
go -C apps/core-go build ./... && go -C apps/core-go vet ./...
gofmt -l apps/core-go/internal/core/                          # 必须无输出
```

`isolatedCoreTestRepository(t)` 每次测试**新建隔离库**，测试可并发；单包全量比单测慢但能抓回归，改动核心链后**必须**跑全量。

## 写一个链路级用例

1. **seed**：自己写 helper（不要复用 `seedTurnConversation`，它写死单 profile）。需要 `actors` / `fluctlights(core_persona, identity, personality, behavioral_policy, life_profile, provenance)` / `fluctlight_inner_states` / `fluctlight_affect_profiles` / `fluctlight_personality_runtime` / `conversations` / `conversation_heads` / `conversation_participants` / `provider_endpoints` + `model_roles('cognitive_assessment')`。`core_persona` 用 `jsonString(map)` 传入即可（列是 jsonb）。
2. **脚本化 Provider**：`newTestApp(t, repository, router)`，`router.on(schemaName, func(payload map[string]any) fakeProviderResult {...})`。
   - schema 名 = wire `response_format.json_schema.name`：Main 轮 `conversation_turn_response`（常量 `workingPersonaMainTurnSchema`）、接管生成 `takeover_reply_response`、Judge `takeover_judge_response`、QUERY 续写 `query_continuation_response`。
   - **不同阶段用不同 schema 名 → 生成次数可精确计数**（`router.requestCount(schema)` / `router.payloads(schema)`）。
   - 失败注入：`fakeProviderResult{Status: 503}` 或 `{Err: ...}`；注意 `http.ErrHandlerTimeout` 文本含 "timeout"，会被 `isProviderTimeout` 判为超时（测通用失败要用别的错误）。
   - 想「只让第 N 次调用失败」，用按调用序号递进的**状态闭包**；**不要**在闭包里反引 `router` 自身（会读到未赋值变量）。
3. **断言对象**
   - 持久行：`conversation_messages`（`kind='assistant'` + `turn_id`）、`cognition_appraisals`（`source_fact_id=inboxID`）、`fluctlight_state_revisions`（`source_event_id=inboxID`）、`fluctlight_personality_runtime.active_profile_id`。
   - frozen payload：`SELECT payload FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE idempotency_key=$1 AND event_type='conversation.turn' ORDER BY created_at DESC LIMIT 1)`；断言 `turn_stage` / `winner` / `takeover` / `turn_persona_scope`。
   - 真实 wire：`payload["messages"]` 里 system content 的 profile 私有标记（roster 只留 `{id,name}`，所以标记只能经 Working Persona 到达）；`payload["response_format"]["json_schema"]["schema"]["properties"]` 验字段是否被条件化。
4. **静态守卫**：读源码文本钉住结构性属性（单插入点、无反向调用、单调用点）。只断言**真实语义**，不要用「出现次数」猜字符串（同名表达式可能合法出现多处）。
   - **顺序型不变量用 `strings.Index` 比较**：例如「互斥判定必须早于规则选择 / Judge / 生成」断言 `mutexIndex < ruleIndex`。比计数更能表达「先后」。
   - **反向守卫**：用 `!strings.Contains(src, X)` 禁止某个退化实现（如禁止仲裁点扫 `CapabilityExecutionPureQuery`，避免语义退化成「候选里有 QUERY 就禁止仲裁」）。
5. **证明断言有牙（反向验证）**：新写的失败路径断言，先临时把生产代码**退回旧分支**、只跑那一个用例、确认它**精确失败**（记下实际 got 值），再恢复。`cp file /tmp/x.bak` → 改 → `go test -run <一个用例>` → `cp /tmp/x.bak file`。不这么做，很容易写出「永远为真」的断言。
   - **seed 参数化**：同一 seed 需要「有规则 / 无规则」等变体时，让主 helper 调 `...WithX(rules []any)`，`len(rules)==0` 时**省略该 key**（不是写空数组），再给一个返回新切片的 `defaultRules()`，避免测试间共享可变 seed。

## 关键陷阱（都踩过）

- **`projection.CorePersona` 是信封**：`{"authority":"hard_constraint","data":<core_persona>}`。读声明人格语义必须经 `corePersonaData()` 拆包；直接读 `["personality_system"]` 会静默拿到空 map，让整条规则链失效（而单测因为直接传未拆包的 map 全绿）。
- **frozen payload 只允许一个能力权威**：`decision` 里不得再带 `capability_invocations` / `tool_calls` / `tool_results`。写 payload 的每个路径都要经 `stripFrozenDecisionSidecars()`；违反不会在写入时报错，而是在 `LoadFrozenTurn` 报 `capability_runtime_dual_authority`。
- **schema 名同时是授权开关**：`workingPersonaMainTurnSchema` 是唯一被授权「持久主导人格切换」的 schema 名；非 Main 生成不要复用它，否则 prompt 会多出 `persistent_switch` 段而它自己的 schema 已禁掉该字段。
- **`HandleTurn` 的错误可能来自重载阶段**，不一定是生成阶段；定位时先看是哪个 `LoadFrozenTurn` / hydrate 校验。
- **静态守卫测试会因代码搬迁而失败**：搬走逻辑后必须同步改守卫读的文件名与调用签名（例如归一化块从 `mutations.go` 搬到 `turn_decision.go`）。
- **失败路径不落失败码 = 静默可重试**：`HandleTurn` 的错误分支若只 `return err` 而不调 `FailTurnCognition`，frozen 行会停在中间 `turn_stage` 且 `status='frozen'`，可被反复重试。凡是「结构性无法成功」的失败（如接管回复被禁止续调用），必须落一个稳定 `error_code`。断言要读 `cognition_frozen_actions.status / error_code`，只断言「返回了错误」是不够的。
- **错误字符串与错误码会漂移**：让 `errors.New(<常量>)` 复用同一个常量，代码里再无字面量，就不可能出现「日志里是 A、库里是 B」。
- **prompt 常量有字符预算门**：`provider_prompts_test.go` 对 `capabilityConversationPolicyInstruction` 等断言了 `max` 字符数。要加提示规则时先看有没有预算门与共用方；被 Main 轮共用的常量扩了会同时放大主 prompt 尺寸。只服务单一阶段（如接管生成）的规则应放该阶段自己的上下文规则里。

## 崩溃恢复测试（阶段 7 起）

不要试图用 Provider 脚本「把轮次停在中间阶段」——`applyTurnTakeover` 的阶段推进是同一调用内完成的，只有 `arbitration_decided + judge_a_takeover_b_pending` 天然可达（让接管生成失败）。其余阶段用**回拨注入**：

```go
// 把已结算轮次还原成「在 targetStage 崩溃」的持久状态，再跑 HandleTurn。
DELETE FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND turn_id=$2  -- 崩溃前未投递
DELETE FROM public.cognition_action_outcomes WHERE action_id=<frozenID>                              -- 结算前无 outcome
UPDATE public.cognition_inbox SET status='pending',claimed_by=NULL,claimed_at=NULL,processed_at=NULL,error_code=NULL ...
UPDATE public.cognition_frozen_actions SET status='frozen',error_code=NULL,payload=$2::jsonb,realization_payload=NULL,completed_at=NULL ...
```

四个必须记住的点：

1. **必须删 `cognition_action_outcomes`**：它按 `(action_id, call_id)` + `request_digest` 幂等，重放一个内容不同的结算会报 `action_outcome_replay_conflict`——那是另一条守卫，不是被测对象。
2. **必须复用完全相同的用户文本**：`enqueueTurnFactTx` 把「存档 text == 本次 text」当幂等校验，不同文本直接 `ErrConflict`（表现为 `resource revision conflict`，不指向真因）。把文本提成常量，别在用例里重复字面量。
3. **基线要选对**：`a_frozen` 的基线必须是**被拒（decline）轮**（其冻结 decision 就是原始候选）。用接管（approve）轮当基线会把 B 的 decision 冒充候选 A，且 scope 的 reply owner 变成 B，`takeover_rules` 的 `source_profile_id` 不匹配 → 规则选不中、Judge 零调用。
4. **断言「零模型调用」要能计数**：`router.requestCount(schema)` 只统计**匹配到脚本**的请求，所以必须给该轮能触达的每个 schema 都注册脚本（哪怕是返回 503 的脚本），计数才有牙。`router.totalRequests()` 统计所有 HTTP attempt（物理），用于 F10 的逻辑/物理分离。

失效快照（并发）测试：回拨到 `winner_ready` 后 `UPDATE fluctlight_inner_states SET revision=revision+1` 并改 `fluctlight_personality_runtime.active_profile_id`；恢复必须被 `current_state_revision_stale` 拒绝、零 assistant 消息、持久 active 与 persona revision 都不被回写。

## 链路测试实证（供后续阶段参照）

**阶段 5** — `turn_takeover_chain_test.go`：Judge 拒/准的分支、只执行胜出者、`takeover_once` 不改持久 active、A/B profile 作用域（prompt 侧标记）、B 生成后不再仲裁、纯 QUERY 不调 Judge、非法候选在 Judge 前失败、接管生成失败不回退发被拒候选、阶段机语义 + 静态守卫。首跑即撞出 3 个单测照不到的真实缺陷（信封未拆包、覆盖候选双权威、接管复用 Main schema 名）。

**阶段 6** — 同文件追加：混合 `QUERY + ACTION` 批次的 `final` 派生与「可仲裁但不续调用」（对**冻结后的真实 invocations** 调 `validatePureQueryContinuation` 断言其非 pure query）、接管回复要求续调用 → 受控失败 + `error_code` 落库、`TestMainGenerationBudgetNeverExceedsTwo`（4 子场景表：无规则 1 次 / 拒 1+Judge / 准 1+1+Judge / 纯 QUERY 1+续调用；断 A+B ≤ 2 且两条路径同轮不共存）。

**阶段 7** — `turn_takeover_recovery_test.go`：§4.8 恢复全枚举逐行（`winner_ready` / `b_frozen` / `arbitration_decided`+`kept_a` / `arbitration_decided`+`b_pending` / `executing` / `a_frozen` / 已有 assistant 消息→重放）+ 失效快照拒绝 + 被拒候选零事实痕迹 + M10 消费者扫描。撞出的真实缺陷：`executing` 只有定义与读取、**没有写入者**，且一旦真的写入会把可恢复轮次当错误隔离。

**「定义了但没写入者的状态」是一类系统性盲区**：阶段机 / 状态枚举里出现的每个值，都要问「谁会写它？谁能从它恢复？」。只被 `switch` 读取、没有任何 `UPDATE` 产出的状态，等于「不可达分支 + 不可测行为」；而如果它位于恢复路径的错误分支上，还会把本可继续的轮次直接隔离。改动这类状态时必须补一条**可达性测试**（回拨注入）与一条**反向验证**。

**阶段 8（基础缺陷复核）** — 交付物是「每个成立缺陷一条**精确字段测试**」，测试文件：`initialization_scope_test.go`（role label / 共享作用域）、`initialization_defaults_test.go`（默认值来源）、`working_persona_test.go`（traits 形状）、`provider_context_test.go`（时区 / 自我举证）。

## 基础缺陷复核的六条经验（阶段 8）

1. **同文件批量 Edit 会静默丢改动**：一次消息里对**同一个文件**发多个 Edit，可能只有部分落盘（工具仍报成功）。**同一文件一次只发一个 Edit，改完 Read 回读确认**，再发下一个。跨文件的批量 Edit 没有这个问题。
2. **「缺陷」与既有测试冲突时，先读测试的名字**：`TestCompactRecentMessagesUsesActorUserAndFluctlightDisplayName` 把「自身消息渲染成 display_name」明确写成了**意图**。此时正确动作是**改盘点而不是改代码**（F14.4 的教训），交付物变成「锁住身份/归属字段的窄断言」。判据：把现有测试改名会不会让它的断言看起来荒谬？会 → 它是意图。
3. **「缺省即共享」是读取侧先行、写入侧补齐**：读侧（`filterActiveProfileRows`、`selectActiveProfileRelationships`、`relationship_edit.go`）早已把空 `profile_id` 当「始终可见」。所以缺省 scope 的修复只在**写入侧**：新增 `initializationScopeProfileID`，空值不过 membership 校验、SQL 写 `nullableString(...)`。别忘了 `Validate()` 里 `ProfileID == ""` 这类「identity 非空」断言——它会把共享作用域误判为缺身份，必须同步放宽并加注释。
4. **默认值要能被区分，就必须落到稳定路径**：`provenance.field_sources["<table>.<entity_id>.<field>"] = "server_default"`（或 `core_persona.personality.<trait>`）。判断「是否兜底」要**复用写入侧同一个谓词**（`boundedNumber` 的 `!ok || <0 || >1`），否则标记与实际写入会不一致。
5. **改格式字符串必须扫全部固定字符串断言**：把 `"01-02 15:04:05"` 改成 `"01-02 15:04:05Z07:00"` 后，用 Grep 找遍 `_test.go` 里的旧字面量（本例 2 处 `"09-03 00:00:00"` 需要同步）。**不要**只依赖编译器——字符串断言不会被编译发现。
6. **两个「出口」形状不一致时，复用同一个 reconciler**：`normalizeEvolutionPersonalityBaseline` 已经能把扁平 `openness` 补成嵌套 `traits.openness`（**加法**，不删扁平键）。在另一个出口调它即可自动保持「一处定义形状」，但要注意它会**主动创建空载体**（`traits: {}` / `expression: {}`），需在出口把空载体再删掉；`filterCorePersonaValue` 不会替你删（它对空嵌套 map 走 `result[key] = child` 直接保留）。

