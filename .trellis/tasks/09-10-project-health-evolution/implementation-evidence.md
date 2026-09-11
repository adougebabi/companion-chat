# Project Health Evolution 实施证据

## 状态

- Task：`09-10-project-health-evolution`
- Worktree：`/Users/vinson/Documents/project/个人/local-ai-companion-llm-capability-runtime`
- Branch：`codex/llm-capability-runtime`
- Base：`master@6b915bebf2516d27a89b3af834858ab0325895af`
- 当前阶段：S11确定性/本机PostgreSQL验收已完成；真实Redis、真实Temporal saved-history与live Provider因未配置保持未验证，Task继续`in_progress`。
- Git policy：最后统一验收前不commit、不merge、不push。
- Planning review：用户已于2026-09-10明确批准当前PRD、设计和实施计划继续实现；没有
  剩余阻塞性产品问题。

## AC00 / S00-S01 前置门禁（2026-09-10）

- `09-09-llm-capability-runtime`全部已知P0/P1/P2已关闭，证据见
  `../09-09-llm-capability-runtime/remediation-evidence.md`。
- 用户已确认该阶段可作为后继任务fixed point收口；Trellis current task已从`09-09`切换到本任务。
- Core/Gateway test、race、vet、build、gofmt、diff-check已在Capability fixed point通过。
- PostgreSQL 16 + pgvector空库迁移、`0025 -> 0026` active replay、malformed fail-closed、
  bounded snapshot、caller-owned transaction与Schedule幂等均已在disposable环境通过。
- Temporal已验证fresh runtime、drain/version fence、Worker Deployment与workflow testsuite；
  没有既存生产history，因此不声称执行过生产saved-history replay。
- `FLUCTLIGHT_LIVE_PROVIDER_TEST`未运行；没有配置真实Provider endpoint。
- 当前shell中`GO_CORE_TEST_DATABASE_URL`、`REDIS_URL`、`TEMPORAL_ADDRESS`与
  `FLUCTLIGHT_LIVE_PROVIDER_TEST`均未配置；后续真实环境门禁将使用独立disposable环境。
- `master`保持`master@6b915beb`，只存在另一session的未跟踪
  `.trellis/tasks/09-08-project-health-evolution/`；本任务没有修改它。

## S02 证据

### 已实现

- 新增`ContextReferenceKind`、`ContextReference`、`ContextReferenceIndex`与
  `DecisionInfluence`；ref按kind/entity/revision/frozen scope生成opaque token。
- Index绑定Fluctlight、Owner、当前speaker、conversation与active profile；只为当前
  Provider surface可见的Relationship/Goal/Intention生成ref，hidden profile row不可引用。
- Memory、Relationship、Goal、Intention、Schedule/item、Scene/Event、Presence、Current
  State、Developing Self与typed slot具备Core-owned mapping；Provider DTO保留opaque `ref`，
  不暴露index、数据库ID或scope。
- conversation、WakeUp、daily review、native cognition schema新增同一closed
  `influences[]`；Core校验unknown/stale/duplicate ref、role、confidence、note和size。
- Provider不能提交`context_reference_index`、版本或派生Goal/Intention refs；Core在freeze前
  安装mapping并按引用kind机械派生服务对象。
- conversation在freeze/replay中保留index/influences；WakeUp/daily review在自主行动与
  Capability planner前要求合法influence；native cognition在Capability执行前完成校验。
- 成功结算会把validated influences和仅被引用的mapping子集带入result fact，供Reflection
  evidence window读取；历史旧终态不会被回填伪造因果。
- daily review删除同一轮Provider失败后改成no-tools第二次Main LLM请求的路径；失败交由
  owning workflow使用稳定身份重试。

### 确定性验证

- `TestContextReferenceInfluenceRoundTrip`
- `TestValidateDecisionInfluencesFailsClosed`
- `TestFreezeDecisionInfluencesUsesOnlyCoreOwnedIndex`
- `TestContextReferenceIndexContainsOnlyCurrentProviderScope`
- `TestDecisionInfluenceSchemaIsClosedAcrossCognitionSurfaces`
- `GOCACHE=/tmp/lac-health-s02-actor-scope go test ./... -count=1`（沙箱外，本地端口测试）：通过。

## S03 阶段实现

- conversation冻结计划后不再提前提交appraisal/Current State；appraisal、personality
  transition、native Capability、assistant/claims、action result和inbox settlement已合并到
  caller-owned transaction。历史半完成记录只恢复一次，不重复state revision。
- Personality switch改为read-only prepare + frozen CAS plan + caller-owned transaction apply；
  Provider不能伪造transition plan。
- 新增`ActionOutcome`模型、Definition-owned `SuccessBoundary`以及独立
  `cognition_action_outcomes` authority；`(action_id,call_id)`为唯一身份。
- S03 migration曾在未提交worktree中推进为`0027_project_health_evolution`，严格位于
  `0026_capability_runtime`之后；S04为保持已应用revision不可变，继续新增
  `0028_affect_canonical`，active pre-reference action采用drain gate，不伪造influence。
- focused Core/migration tests与真实PostgreSQL migration/replay门禁均已通过。

## S02/S03 复验结果（2026-09-10）

- `ContextReferenceIndex`覆盖Memory、Relationship、Goal、Intention、Scene/Schedule、State、
  Developing Self、typed slots与真实Recent Outcome；Provider只能看到opaque ref。
- conversation、WakeUp、daily review、native cognition共用closed `influences[]`和freeze前
  validator；foreign/stale/duplicate/oversized/actor-profile scope均fail closed。
- Capability执行被统一分类为`pure_query`、`transactional_mutation`、`deferred_output`、
  `external_async_intent`；分类只读取Definition/interface，不按Capability名称分支。
- Scene、Presence、Schedule、Memory、Affect与Capability Request均实现caller-owned
  `TransactionalCapability`；真实PostgreSQL outer rollback后没有任何领域行、fact或outbox残留。
- conversation normal/recovery/no-op共用事务结算：appraisal、personality transition、native
  mutations、assistant/claims、Capability result、ActionOutcome与inbox状态同commit。
- 新单一authority：`cognition_action_outcomes`，唯一`(action_id,call_id)`；fresh schema不再
  创建`cognition_action_results`，升级库只把旧表作为历史audit保留且生产零读写。
- 每个action有primary Outcome，每个Capability call有独立Outcome；Definition声明当前成功
  边界、可选异步完成边界和external ref字段。
- media/Visual Identity异步结果通过external ref CAS同一Outcome；已验证
  `pending -> unknown -> completed`、revision 1→2→3、每次revision一个ordered fact、相同终态
  重放no-op、不同终态observed payload冲突。
- Recent Outcomes进入`ContextProjection`、`ContextReferenceIndex`和最小Prompt section；最终
  Provider request仅包含opaque refs、Capability/status/success boundary/time/bounded error与
  白名单结果信号。原始文本、raw expected/observed、entity/action/call/external IDs、完整index、
  evidence/provenance/provider/workflow/idempotency均不外发。
- S03的`0027_project_health_evolution`严格位于`0026_capability_runtime`之后。真实PostgreSQL验证：
  empty→0027通过；malformed active pre-reference row以SQLSTATE P0001阻止且ledger保持0026；
  修正为valid reference envelope后0026→0027通过；fresh schema只有新Outcome authority。

### 确定性与工程门禁

- Core `go test ./... -count=1`：通过。
- Core `go test -race ./... -count=1`：通过。
- Core `go vet ./...`、`go build ./...`：通过。
- Gateway `go test ./... -count=1`、`go test -race ./... -count=1`、`go vet ./...`、
  `go build ./...`：通过。
- `gofmt -l`、`git diff --check master`：通过。
- 真实PostgreSQL：ActionOutcome幂等/异步CAS、structured-turn rollback、Memory/Affect rollback、
  Scene/Presence/Schedule/CapabilityRequest rollback与Schedule database idempotency均通过。

## Recent Outcome Provider 边界（用户已授权）

后续闭环要求“最近ActionOutcome进入下一次LLM cognition”。完整Outcome内部数据可能包含
assistant文本、数据库ID或其他非公开内容，因此不会原样外发。建议的Provider-safe投影仅含：

- opaque outcome/context/Goal/Intention refs；
- Capability name、status、success boundary、occurred-at；
- 白名单化结果信号（例如`delivery_status`、`target_kind`、bounded error code）；
- 不含原始文本、raw expected/observed、内部index/entity ID、provenance或完整evidence payload。

用户于2026-09-10确认先按上述严格裁剪方案实现；后续将另行整体重做提示词。因此本任务只
增加最小Recent Outcomes section和authority说明，不扩写或重排完整Prompt。

## S04 Affect / Drive canonical reducer（2026-09-10）

### 最终实现边界

- `affect.reducer.v2`成为appraisal、`affect_event`、lazy projection和Drive transition的唯一
  numeric policy。PAD、`momentum.value/trend/*_momentum`使用`-1..1`；mood intensity、
  regulation、Drive pressure/salience与conflict pressure使用`0..1`。`NaN/Inf`在通用数值入口
  fail closed，未知reducer version不得用当前算法冒充执行。
- 新增`fluctlight_affect_profiles`authority，包含baseline PAD、PAD/momentum/mood/Drive
  half-life、regulation、emotional summary、revision和policy version。完整profile仍为Core-only；
  Provider仅获得opaque AffectProfile ref、kind和`core_numeric_policy`authority。
- elapsed-time decay按真实wall time指数衰减，PAD回归profile baseline，momentum/Drive/conflict
  回归零；partition-invariance覆盖PAD、`momentum.value/trend`、Drive与conflict，不依赖Worker tick。
- built-in `exploration/rest/social`与active typed pressure Drive合并为Current State。Provider只输出
  closed semantic `drive_signals[{ref,direction,strength,confidence,evidence_refs}]`；Core解析frozen
  Drive ref、计算pressure与opposed-signal conflict并持久化requested/applied delta。inactive typed
  slot会从快状态移除；若曾覆盖built-in key，则恢复built-in neutral pressure。
- appraisal改为closed allowlist；unknown/raw numeric field、foreign/duplicate evidence、非有限值在
  freeze前拒绝。tool-only capability结果不再合成“neutral appraisal”，而是Core在freeze后写入
  `cognitive_state_transition=not_proposed`并跳过state mutation；Provider不能提交该marker。
- `affect_event`先锁state再检查event idempotency，canonical request digest阻止同key不同payload；
  同source相邻appraisal/event合并一个revision，A→B→迟到A fail closed；冻结的state与
  AffectProfile revision均在apply前CAS，冲突标记为不可盲重试。
- 每个Capability无论是否实现Capability-local Preparer，都先经过Runtime Prepare以冻结
  provenance、context snapshot与显式PreparedPayload envelope。autonomy message/Moment/capability
  action中的transactional mutation已移入同一个action settlement transaction；standalone
  `ExecuteCapabilities`只做query/plan，不再自提交transactional Capability。
- native cognition现在先冻结Main LLM decision，再prepare一次；retry复用同一decision、Provider
  identity、PreparedPayload和action-bound ContextSnapshot。成功只创建一个Reflection intent；
  required failure回滚state/effect/outcome，retryable保持frozen，terminal failure明确失败。
- reducer产生的post-transition state revision由Core生成新opaque state ref，写入frozen action、
  primary ActionOutcome/context-reference subset及`autonomy.result`fact；下一轮projection可引用该ref。
- Capability产生的递归`life.*`fact携带通用cognition depth；超过bounded depth时显式
  `native_cognition_cycle_guarded`结算并进入Reflection，不依赖Capability名称或场景关键词。

### Migration

- 当前Head：`0028_affect_canonical`；PreviousHead：`0027_project_health_evolution`；Capability
  Runtime Head：`0026_capability_runtime`。`0027`SQL已由digest guard冻结，后续不得回写。
- `0028`对旧合法`0..1`子集采用`preserve_numeric_subset_no_semantic_inference`，不猜旧情绪语义；
  backfill AffectProfile与Drive half-life，并追加source-head reconciliation audit。
- preflight覆盖PAD、全部momentum key、mood/regulation、Current State Drive/conflict、typed pressure
  Drive、AffectProfile与重复state source；malformed阻止cutover。Post-cutover约束显式要求required
  JSON keys，避免PostgreSQL `CHECK NULL`漏过空对象；constraint查询按目标`conrelid`限定。
- 新增`UNIQUE(fluctlight_id,source_event_id)`state-transition source边界；migration ledger含首尾空白
  直接拒绝，不会写成两个Head。

### 确定性与真实环境证据

- Pure/Core：`TestCanonicalAffectReducerPreservesNegativePADAndMomentum`、
  `TestCanonicalAffectReducerClampsBipolarAndUnitFieldsSeparately`、
  `TestAffectElapsedDecayIsPartitionInvariant`、
  `TestDriveSemanticSignalsResolveFrozenRefsAndCoreOwnsPressure`、
  `TestDriveSemanticSignalSchemaIsClosedOnAppraisalSurfaces`、
  `TestMergeEffectiveDriveStateDropsInactiveTypedSlotAndRestoresBuiltin`、
  `TestPrepareCapabilityInvocationsFreezesRuntimeProvenanceWithoutCapabilityPreparer`通过。
- disposable PostgreSQL 16 + pgvector（容器`lac-s04-test-pg-0910`，仅随机临时数据库）：
  - `TestAffectCanonicalMigrationUpgradesProjectHealthHeadAndPreservesState`；
  - `TestAffectCanonicalMigrationRejectsMalformedMomentumWithoutAdvancingLedger`；
  - `TestMigrationRoutesCapabilityRuntimeThroughProjectHealthAndAffectAtomically`，覆盖合法
    `0026→0027→0028`与0028失败时0027效果/ledger整体回滚到0026；
  - `TestMigrationRejectsWhitespacePaddedLedgerHead`；
  - `TestNativeCognitionRequiredFailureReplaysFrozenDecisionWithoutSecondProviderCall`，Provider=1、
    Prepare=1、事务执行=2、state revision=1、Reflection intent=1；
  - `TestAffectEventSerializesIdempotencySourceAndFrozenProfile`；
  - `TestAppraisalAndAffectEventCommitOneRevisionVisibleToNextProjection`，并证明下一decision/frozen
    Outcome引用新state ref、no-appraisal路径不写state；
  - `TestNativeCognitionCycleGuardBoundsCapabilityProducedLifeFacts`；
  - `TestAutonomyCapabilityActionRollsBackTransactionalSiblingOnRequiredFailure`；
  - 原S03 ActionOutcome、structured-turn rollback、native mutation rollback回归均通过。
- S04完成后的Core/Gateway test、race、vet、build、gofmt和`git diff --check master`全部通过；
  首次沙箱内Core/Gateway test仅因loopback bind限制失败，按规则在沙箱外复跑通过。

## S05 Memory lifecycle + operation-aware retrieval（2026-09-11）

### 单一写入 authority 与 Capability 边界

- 新增`memory.lifecycle.v2` typed command authority；`applyMemoryCommandTx`成为生产中唯一
  `memories/memory_revisions/memory_governance`写入者，静态guard禁止在其他Core文件恢复旁路SQL。
- `memory_event` Provider InputSchema仅含`type/content/confidence/importance`及optional
  `emotional_significance`，不含operation、ID/revision、profile、visibility、Actor/Event/
  Conversation/evidence、provenance或idempotency。Runtime Prepare冻结Core-owned`memory_plan`，
  apply只消费该plan；非事务执行返回`caller_transaction_required`。
- `memory_event`使用`required_for_visible_claim`，Memory写失败会回滚可见assistant/action/outcome，
  不允许“说记住但事实未提交”。Arguments保持Provider-owned immutable；rollback、commit和replay
  已在真实数据库验证。
- owner revise/forget/rollback均委托同一authority。command digest排除wall time但保留全部语义/
  scope/target，使同请求可重放、不同payload冲突。rollback读取完整immutable snapshot并以新revision
  恢复；forgotten/deprecated可恢复，merge会同时恢复未继续演化的related Memory，supersede会恢复旧
  Memory并把本次创建的replacement追加deprecated revision。related row后续已演化时fail closed。

### Reflection `MemoryCandidateV2`

- Reflection Memory schema改为closed：
  `operation=create|confirm|revise|merge|supersede|deprecate`、operation-specific opaque
  `target_ref/merge_refs`、semantic fields、`evidence_refs`、`semantic_reason`。Provider不能提交
  raw Memory ID、expected revision、profile/visibility/perspective、Actor/Event/Conversation、
  provenance/idempotency或embedding metadata。
- raw Reflection response在alias normalization前执行完整schema validator；empty/scalar/
  wrong-container/unknown-field candidate使proposal invalid且watermark不推进，不再被normalizer吞掉。
- compiler在事务外验证frozen`ContextReferenceIndex`，把Memory ref解析为internal ID + expected
  revision，绑定owner/profile/scope/evidence/proposal/candidate/idempotency，并拒绝wrong-kind、foreign、
  duplicate、overlapping target/merge footprint。
- Memory evidence只接受当前sequence observation/appraisal、strict ActionOutcome projection和
  authorized Memory/Outcome ref。`autonomy.result`发给Reflection时物理删除assistant visible text、
  raw expected/observed、数据库/Provider/workflow/idempotency信息；完整内部事实仍用于audit/replay。
- sequence、Memory和Outcome evidence均携带Core-only Conversation scope；conflicting/unknown scope
  拒绝candidate，不再用空字符串隐式创建global Memory。target operation也要求evidence scope不越界。
- proposal、Memory mutations、full revisions、governance、embedding intents、outbox与watermark在
  一个事务提交；watermark必须对已claim row完成exactly-one CAS，watermark=0也没有
  `INSERT ... ON CONFLICT DO NOTHING`成功旁路。返回包含Memory-level disposition counts/results。

### Operation-aware retrieval 与 Provider边界

- 新增显式operation：conversation、wake_up、daily_review、native_cognition、reflection、
  capability_planner；Conversation scope必须是`exact|allowed_set|global_only`，空值不再是wildcard。
- authorization actor与viewer audience分离；owner/status/type/visibility/viewer Actor/Conversation
  filters在PostgreSQL中先于200-candidate limit、FTS/lexical和vector ranking执行。Fluctlight self
  actor ref不会授权无关viewer，wrong conversation与terminal Memory不会进入ranking。
- typed query plan机械组合current message、Life Context、active Goal/Intention、strict Recent Outcome、
  unresolved hypothesis、Reflection fact/appraisal和Capability semantic intent，不按关键词推断意义。
  空cue=`salience_recent`；有本地cue但无embedding egress=`lexical_salience`；只有明确授权的
  `AllowEmbedding` cue才是`semantic_hybrid`。
- 当前生产内部cue全部local-only，避免Scene/Goal/Outcome/Reflection内容新增外发到Embedding
  Provider；测试用non-sensitive synthetic cue验证vector seam。vector SQL只接收已经授权的candidate
  IDs并重检active/current revision；缺失/不兼容vector保留bounded fallback trace。单条oversized
  Memory不会突破budget，trace保持Core-only。
- Provider Memory DTO只含opaque ref、type/content/confidence/importance/emotional significance/
  semantic created-at及active profile的bounded interpretation；raw evidence IDs、revision、profile ID、
  perspective evidence/provenance、visibility和embedding metadata不外发。

### Embedding lifecycle 与`0029_memory_lifecycle`

- canonical tuple为`UNIQUE(memory_id,memory_revision,model_id)`；revision `0`是精确revision，
  不是unspecified sentinel。tuple使用同一row完成`pending|failed -> ready`，更新dimensions/vector/
  error/embedded_at；ready replay按Memory/revision/endpoint/model完整匹配且不会被失败重试降级。
- endpoint/model在embedding workflow intent冻结；Activity input必须与持久化binding一致，existing
  tuple不得被rebind。Provider assignment只解析一次并直接传入实际Embed request。
- Provider I/O前后均检查Memory仍`active`且revision精确匹配；并发revise/deprecate/supersede使
  pending/failed tuple变stale，不能恢复obsolete ready。Provider failure不改变Memory authority。
- Head推进为`0029_memory_lifecycle`，严格位于digest-frozen`0028_affect_canonical`之后；`0027/0028`
  未改写。migration只ADD table/column/index/constraint，不UPDATE/DELETE历史Memory或embedding。
- preflight拒绝pre-lifecycle/durable working/malformed Memory、duplicate active canonical key、duplicate
  revision/embedding tuple、orphan/malformed embedding和active intent。既存Memory只有携带
  `memory.lifecycle.repair.v1` audit，且source snapshot与type/content/Conversation/visibility/sorted
  Actor/Event scope逐字段一致、key/digest与row一致，才能cutover；unverifiable repair阻止ledger推进。

### 确定性与真实环境证据

- pure/contract：MemoryCandidateV2六种operation shape、closed/runtime-field禁止、opaque ref compile、
  cross-Conversation/unknown scope、same-proposal overlap、local-vs-embedding cue、salience/lexical/hybrid
  mode、single SQL authority、non-transactional rejection、Provider Memory/Reflection redaction均通过。
- disposable PostgreSQL 16 + pgvector（`lac-s05-test-pg-0910`，每项创建并删除随机`lac_core_*`/
  `lac_affect_migration_*`数据库）通过：
  - `TestMemoryEventFreezesRuntimePlanAndSharesCallerTransaction`；
  - `TestMemoryLifecycleCreateConfirmReviseMergeSupersedeDeprecate`；
  - `TestOwnerMemoryReviseRollbackAndForgetUseLifecycleAuthority`；
  - `TestOwnerMemoryRollbackCompensatesMergeAndSupersedeLineage`；
  - revision-zero/terminal-state、failed→ready one-tuple、frozen-assignment embedding tests；
  - authorization-before-ranking、exact/global-only、oversized budget、authorized current vector tests；
  - create→retrieve→Memory influence→ActionOutcome→Reflection revise→next revision/ref闭环；
  - 一个Reflection proposal原子执行create/confirm/revise/merge/supersede/deprecate；
  - forced watermark failure后Memory/revision/governance/embedding intent/outbox/proposal/watermark全回滚；
  - empty/repaired`0028→0029`、rerun、unrepaired/unverifiable repair、duplicate canonical/embedding、
    durable working、malformed active embedding intent fail-closed且ledger保持`0028`。
- 独立只读post-implementation audit最初报告0个P0、7个P1；上述schema吞候选、watermark CAS、
  rollback lineage、Conversation scope、embedding binding、retrieval mode与migration repair audit问题
  已逐项修复并复验。
- 审计修复后Core与Gateway`test ./... -count=1`、`test -race ./... -count=1`、`vet ./...`、
  `build ./...`全部通过；`gofmt -l`、`git diff --check master`通过；verify-change/quality/security通过，
  security为0 Critical/High/Medium/Low。首次全量复跑曾因系统临时卷不足失败，随后只删除本任务
  明确GOCACHE目录并复用单一gate cache，最终门禁真实通过。

## S06 Life Context / Scene reference closure（2026-09-11）

### 主线收口

- Life Context以同一Repeatable Read快照解析`confirmed Event > inferred Event > accepted Schedule
  item > pending`，Presence只覆盖user/current-task；opaque Event/Schedule-item/Presence refs、source、
  effective/expiry和`life_ctx_*` composite revision进入frozen projection。
- composite revision同时包含当前Event、底层accepted Schedule及当前item时间边界、Presence和timezone。
  即使Event当前具有最高authority，Schedule替换也会使旧frozen action发生stale conflict；
  `conversation.reply`显式冻结`SlotCurrentLife`。
- Schedule item可携带location并贯通migration、Core读取/写入、Provider-safe projection、BFF和生成的
  Browser contract；没有location时不由Core猜测。
- Scene/Presence/Schedule mutation继续使用caller-owned transaction、per-Fluctlight advisory lock、
  canonical digest、stable idempotency和stored replay result；stale revision不提交事实、outbox或Outcome。
- 0030 clean-start拒绝已有业务authority的旧库，Event/Presence/Schedule要求NOT NULL replay identity；
  deferred constraint trigger在COMMIT读取最终row，允许同事务暂存`{}`后补齐，但拒绝历史/过期/当前
  authority以不完整result首次写入。owner cancel command ledger也要求closed replay result。
- direct conversation在Provider前发送已提交user frame，在assistant/effects事务提交后发送assistant
  frame；两种frame都携带数据库`created_at`。缺少可见回复返回`cognition_visible_text_missing`，不把
  cognition标记completed。
- settlement事务锁定cognition inbox；已被新turn标记`superseded_by_newer_turn`的旧turn不能写入迟到
  assistant、不能把inbox恢复为processed。
- Web queued turn拥有独立local message/turn/idempotency identity，不再猜测服务端sequence；旧retry的
  assistant不能覆盖queued user。stream-confirmed消息优先于较旧history，queued turn保留会话绑定。

### 用户限定的focused退出证据

- Life Context：
  `GO_CORE_TEST_DATABASE_URL=... go test -run TestLifeContext ./internal/core/... -count=1 -v`
  通过，3项：snapshot priority/boundary、no scene heuristic guard、Provider opaque authority projection。
- 0030 migration（仓库实际路径为`./internal/migrations/...`）：
  `GO_CORE_TEST_DATABASE_URL=... go test -run '^(TestLifeContextRevision|TestMemoryLifecycleMigrationPreservesExplicitlyRepairedMemoryAt0029And0030RejectsIt)' ./internal/migrations/... -count=1 -v`
  通过；覆盖empty 0029→0030、非空Actor/Goal/Provider/workflow拒绝、rollback、malformed authority、
  deferred staged/commit检查、历史/过期row检查和0029 repaired Memory被0030拒绝。
- Chat Bug目标测试：
  `GO_CORE_TEST_DATABASE_URL=... go test -run 'Test(DirectConversationMessageIsDurableDuringCognitionAndNoReplyCannotComplete|DirectConversationStreamsCommittedUserBeforeProviderAndAssistantAfterCommit|SupersededConversationTurnCannotCommitLateAssistantOrReviveInbox)$' ./internal/core/... -count=1 -v`
  通过，Core 3/3；`node --test apps/web/test/conversation-delivery.test.mjs`通过，Web 3/3。
- 按用户2026-09-11 S06执行约束，本阶段不运行Gateway、`-race`、全仓`gofmt`或3.1全量门禁，
  不以这些未运行项声明通过。上述三组focused验证全绿即为S06退出条件。

S06六项focused checklist已于2026-09-11完成；此后已进入并完成S07-S11的本机可验证主线。

## S07 Goal / Intention / Trigger / Outcome（2026-09-11）

- Goal/Intention V2 authority、全生命周期、revision/governance、typed trigger、stable
  attempt/action/outcome identity和CAS均由`goal_intention.go`统一定义。
- `persistIntentionAuthorityTx`在Intention进入`qualified`时创建revision-scoped
  `intention.trigger` durable workflow intent；`IntentionTriggerWorkflow`对time trigger使用
  Temporal durable timer，对event/semantic trigger只等待新的processed fact并交给Core做typed
  match，不读取自然语言关键词。per-Intention cursor按sequence持久推进，不匹配fact不会饿死
  后续匹配fact；到期只生成stable `agency.intention_due` cognition fact。
- primary ActionOutcome在`persistActionOutcomesTx`同一事务机械结算一个Intention attempt；
  failed/suppressed回到qualified并建立新revision trigger intent，cancelled关闭。存在async call时
  aggregate primary保持pending，只有final completion boundary结算后才完成Intention；replay复用
  同一digest/attempt。
- Reflection V2 Goal candidate新增`outcome_refs/criterion_indexes/complete`；Core解析frozen
  Outcome authority，只有completed且绑定同一Goal的Outcome可经`ApplyGoalProgress`推进。
- 证据：
  - `TestGoalIntentionStageS07` PASS（4/4）；
  - `TestIntentionTriggerProductionFlowCreatesDueFactAndSettlesFromOutcome` PASS；
  - `TestIntentionTriggerWorkflowUsesDurableTemporalTimerBeforeActivity` PASS；
  - `TestProcessReflectionV2AdvancesGoalOnlyFromBoundCompletedOutcome` PASS。

## S08 Reflection V2 生产闭环（2026-09-11）

- `ProcessReflection`只有`processReflectionV2`生产出口；V1 schema、normalizer、validator、
  DB-ID SQL apply和对应旧测试已删除。generic reflection role也解析为V2 closed schema。
- Provider egress只包含opaque `sequence:*` refs、bounded event type/用户observation、scalar
  semantic observation allowlist、semantic appraisal allowlist、strict ActionOutcome DTO和
  Provider-safe context。嵌套object/array、oversized/invalid numeric值、assistant visible text、
  hidden reasoning、transcript、raw expected/observed、数据库/Provider/workflow/idempotency ID、
  provenance和内部ContextReferenceIndex不会进入请求。
- 完整ActionOutcome只进入Core-owned `EvolutionContext.Outcomes`；Provider仍只看到裁剪投影。
- 单个caller-owned transaction应用Memory、Relationship、Goal、Intention、Affect summary与
  recalibration、Drive、Preference、Trigger、Developing Self、Personality/Behavior overlay，
  并提交proposal、逐候选disposition、domain revisions和watermark。Memory exact duplicate及
  Developing Self threshold/no-change会回写真实disposition；foreign/nested ref、malformed
  proposal不推进watermark，CAS/domain failure整笔rollback。
- `TestReflectionV2AppliesMemoryGoalIntentionAndAffectThenReprojects` PASS：同一窗口真实修改
  Memory/Relationship/Goal/Intention/Affect/Drive/Preference/Trigger/Developing Self；单窗口
  Personality/Behavior候选正确deferred。
- Provider boundary证据：
  - `TestReflectionV2ProviderSchemaOwnsOnlySemanticFields` PASS；
  - `TestCompactReflectionEvidenceV2RejectsNestedAndOversizedValues` PASS；
  - `TestActorRelationshipContextCompactsTargetedIntention` PASS。

## S09 Personality / Behavior Overlay（2026-09-11）

- Reflection只可写allowlisted、profile-scoped overlay；Identity/Core Persona、Owner、安全、
  权限、Provider、基础设施和hard character constraints保持不可自动修改。
- numeric trait由Core计算requested/applied delta并限幅；categorical behavior要求更高置信度和
  3个独立历史Reflection windows；当前窗口无论多少facts只贡献一个window。
  cooldown/CAS/append-only rollback primitive继续由overlay policy治理。
- `BuildContextProjection`读取持久化overlay，组合baseline为bounded `effective_persona`，并为
  active overlays生成opaque refs；baseline列从不被overlay回写。
- `TestEvolutionOverlayStageS09` PASS（5/5）；
  `TestReflectionOverlayEvidenceCountsHistoricalWindowsNotFacts` PASS。Owner-authorized rollback
  API/e2e尚未实现，留待下一轮治理任务。

## S10 0031 Evolution Authority（2026-09-11）

- migration Head=`0031_evolution_authority`，PreviousHead=`0030_life_context_revision`；
  加入per-Intention trigger cursor后的0031 digest为
  `0ded302af878cd0c5b5c4d544b61b9c1f976b1004061ac9ecd55655e900bb4b5`。
- 0031与0030相同采用clean-start：只接受fresh/empty-business database；任意既存业务authority
  fail closed，不truncate、不repair、不编造语义。
- disposable PostgreSQL 16 + pgvector：
  - `go test ./internal/migrations -count=1` PASS（25.606s）；
  - `TestEvolutionPersistenceS10` PASS；
  - Reflection多域transaction/CAS/watermark、Intention due/attempt replay均PASS。

## S11 全量确定性门禁（2026-09-11）

- `pnpm generate` PASS；四个生成物前后SHA-256完全一致。
- `pnpm typecheck` PASS；`pnpm test` PASS（Browser Client 8，Web 35）；
  `pnpm build` PASS（Vite production build）。
- Core：`go test ./... -count=1` PASS；`go test -race ./... -count=1` PASS；
  `go vet ./...` PASS；`go build ./...` PASS。两次test均显式使用本机disposable PostgreSQL。
- Gateway：`go test ./... -count=1` PASS；`go test -race ./... -count=1` PASS；
  `go vet ./...` PASS；`go build ./...` PASS。
- `gofmt -l`输出为空；`git diff --check master` PASS。
- `go-core-reference-guard.sh`、`legacy-scope-guard.sh`、`check-core-openapi.sh`、
  `check-compose-bind-sources.sh`和`docker compose ... config --quiet`全部PASS。
- 新S11 guards PASS：Reflection V1 production surface=0、`role=tool`=0、legacy decision
  effect fallback=0、Schedule semantic splitter=0、active executable runtime/context authority
  fail-closed、PAD/momentum保持bipolar clamp。

### 五条可回放closure trace

1. Memory：observation/ref -> authorized retrieval -> decision influence -> ActionOutcome ->
   Reflection create/confirm/revise/merge/supersede/deprecate -> next Memory revision/ref。
2. Scene：Event/Schedule/Presence refs -> frozen `life_ctx_*` -> scene Capability transaction ->
   Outcome -> next authoritative Life Context；stale revision整笔rollback。
3. Affect：semantic appraisal/ref -> `affect.reducer.v2` -> state/outcome -> Reflection emotional
   summary + bounded profile recalibration -> next AffectProfile/state projection。
4. Goal/Intention主逻辑：Goal/Intention refs -> `intention.trigger` -> `agency.intention_due` ->
   cognition/Capability authority -> sync/async final Outcome -> attempt settlement -> evidence-bound
   Goal progress。单条贯通Temporal dispatcher与真实Capability的组合fixture留待下一轮。
5. Reflection主逻辑：bounded evidence/outcomes + frozen refs/revisions -> closed V2 proposal ->
   one UoW applies accepted domains + truthful dispositions/watermark -> next projection observes
   changed values。mutation后可直接解析的精确`changed_refs`仍待增强。

### 外部环境实况与未验证项

```text
REDIS_URL=unset
TEMPORAL_ADDRESS=unset
FLUCTLIGHT_LIVE_PROVIDER_TEST=unset
```

- PostgreSQL已通过真实本机disposable环境；默认shell未导出URL，但所有上述PG命令显式传入
  `postgres://postgres:postgres@127.0.0.1:32771/postgres?sslmode=disable`。
- Redis只有miniredis deterministic tests和Compose配置/smoke入口，没有真实Redis
  expiry/reconnect/duplicate/startup-repair/PG-fallback证据。
- Temporal有SDK durable-timer/workflow tests与Worker build/version静态/配置门禁，但没有真实
  Server上的intention/reflection/daily-review/media/schedule saved-history replay或drain证据。
- live Provider未配置，未执行概率性兼容probe；这不影响fake Provider确定性闭环，但不能宣称
  live Provider门禁通过。
- `master` worktree仍在`master@6b915beb...`；其既有未跟踪
  `.trellis/tasks/09-08-project-health-evolution/`未被本任务修改。本任务全部代码/文档位于专用
  `codex/llm-capability-runtime` worktree。
- 未commit、未merge、未push，等待用户统一验收。

### PRD AC00-AC14 mapping

| AC | 状态 | 主要证据 |
| --- | --- | --- |
| AC00-AC01 | PASS | 前置Capability Runtime evidence；全量Core/Gateway/race与static guards未回归双Runtime/PreparedPayload/transaction边界。 |
| AC02 | PARTIAL | Provider-safe projection、opaque ref/freeze validator及foreign/stale/duplicate rejection已验证；单条conversation fixture同时贯通Memory/Scene/Affect/Goal/Intention/Outcome influences留待下一轮。 |
| AC03 | PASS | S05 Memory lifecycle/retrieval全闭环与本轮完整Core/race回归。 |
| AC04 | PASS | S06 Life Context focused证据与本轮全量回归；Schedule自然语言split heuristic已删除并有反向测试。 |
| AC05 | PASS | Affect reducer、atomic transaction、profile recalibration和next projection tests。 |
| AC06 | PARTIAL（主逻辑PASS） | time/event/semantic cursor、expire、re-arm、sync/async final Outcome settlement与Outcome-bound Goal progress均有production tests；单条Temporal dispatcher→Main→真实Capability→Goal组合fixture待补。 |
| AC07 | PASS | Reflection V2多域happy path、单事务、watermark/CAS、foreign/nested ref与next projection通过。 |
| AC08 | PARTIAL | Memory/Developing Self真实no-change/deferred统计已修；mutation后精确可解析changed refs仍待增强。 |
| AC09-AC09a | PASS | ActionOutcome authority/async CAS/replay与Provider strict allowlist/request fixtures。 |
| AC10 | PASS（deterministic） | conversation/WakeUp/daily/native/Reflection fake Provider与static single-Main/zero-role-tool guards；live Provider未运行。 |
| AC11 | PASS | generic Capability dispatch、Reflection V1=0、semantic splitter=0、runtime-field/PAD/replay guards。 |
| AC12 | PARTIAL | Core/Gateway/Web/race/vet/build/gofmt/diff/PG PASS；真实Redis、真实Temporal saved-history和live Provider未配置。 |
| AC13 | PASS | 专用worktree/branch检查；master仅有既存未跟踪task目录，本任务无写入。 |
| AC14 | PARTIAL（主逻辑PASS） | 真实历史window计数、allowlist/confidence/max-delta/cooldown/profile/effective projection及rollback primitive已验证；Owner-authorized rollback API/e2e待补。 |

## Bug Analysis：conversation.reply 无 appraisal 导致消息不展示（2026-09-11）

### 1. Root Cause Category

- **Category**：B（Cross-Layer Contract）+ D（Test Coverage Gap）。
- **Specific Cause**：`cognitiveTurnResponseSchema`允许appraisal缺省；Provider合法返回空content、
  一条`conversation.reply` Tool call和无appraisal时，`handleTurn`只对“无reply的tool-only”设置
  `cognitive_state_transition=not_proposed`。可见reply路径随后进入`persistCognitiveStagesTx`并以
  `appraisal_required`回滚assistant/effects。现场turn
  `turn_47d7fc03-d804-4183-9b0e-591cf2e625c0`的数据库证据为：user row已提交，assistant row=0，
  inbox/frozen action=`failed/capability_settlement_failed`；Core日志保留原始根因`appraisal_required`。

### 2. Why Earlier Fixes Failed

1. S06修复了user-before-provider、assistant-after-commit与no-visible-reply，但成功fixture始终携带
   完整appraisal，未覆盖“可见reply Tool存在、appraisal完全缺省”的Provider-native shape。
2. tool-only no-appraisal测试只覆盖没有conversation.reply的background/no-op分支，错误地把
   “是否有可见reply”和“是否提出appraisal”耦合成一个布尔条件。
3. Web queued/history回归能保护已经收到的message frame，却无法让Core替一个被
   `appraisal_required`回滚的assistant生成权威frame。

### 3. Prevention Mechanisms

| Priority | Mechanism | Specific Action | Status |
| --- | --- | --- | --- |
| P0 | Architecture | appraisal缺省独立映射为Core-owned `not_proposed`；present-but-invalid仍fail closed | DONE |
| P0 | Integration test | `TestDirectConversationReplyToolWithoutAppraisalCommitsBothMessages`复现真实Provider envelope | DONE |
| P0 | Browser regression | terminal error后authoritative user message/retry identity仍可见 | DONE |
| P1 | Contract | structured-turn spec明确visible output与appraisal是正交可选边界 | DONE |
| P1 | Runtime verification | clean rebuild本地Compose到0031并确认BFF/Core/Worker/Web healthy | DONE |

### 4. Systematic Expansion

- **Similar Issues**：任何optional semantic candidate都不能因为“完全未提出”阻断独立、有效的
  visible output；但候选一旦存在且malformed/foreign，仍必须按其安全契约fail closed。
- **Design Improvement**：Provider response normalization应显式区分`absent`、`present-valid`、
  `present-invalid`，避免以空map和错误混合表达不同状态。
- **Process Improvement**：每个optional cognition section都必须至少有三种fixture：缺省、合法、
  非法；可见交付测试必须覆盖text字段与output Capability两种通道。

### 5. Knowledge Capture / Verification

- `.trellis/spec/backend/structured-turn-contract.md`已增加正交边界、error matrix和required test。
- 红灯命令在修复前稳定得到：`appraisal_required user=1 assistant=0 events=[user]`；同一命令修复后PASS。
- Chat focused Core 4/4 PASS；Web conversation-delivery 4/4 PASS；Web全量36项PASS；Core全仓
  `go test ./... -count=1`在迁移后的独立PostgreSQL上PASS。
- 没有`src/templates/markdown/spec/`目录可同步；遵照用户策略未commit。

## Chat follow-up：clean-start 后孤立 retry 阻塞新消息（2026-09-11）

- 根因是跨层状态生命周期不一致：clean-start 删除了旧 PostgreSQL conversation，但浏览器
  `localStorage`仍保留旧`retryTurn`；`send()`只看到“存在另一个 retry 且不属于当前 conversation”，
  就阻止新发送。它不是 Core inbox 锁或 BFF 丢帧。
- Web 现在在 bootstrap 时清理不再存在于 Fluctlight 列表的 retry/queued identity；加载同一
  Fluctlight 返回新 direct conversation 时，再清理 conversation-id 不匹配的本地 identity。
  发送前如果 retry 属于另一个仍存在的会话，会读取该会话的权威历史：已经有对应 assistant
  的 retry 会被清掉，仍未完成的 retry 继续保留，不会误删。
- 回归：`a retry from a discarded conversation is pruned when the server returns a new conversation`
  、`a completed retry from another conversation no longer blocks the selected conversation` 与原有
  conversation-delivery tests 全部 PASS；Web bundle 已重建部署到当前 disposable Compose。
