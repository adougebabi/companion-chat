# Project Health Evolution Implementation Plan

状态：in_progress。前置任务已独立完成整改和复验，用户已确认可以继续；本任务已运行
`task.py start`，S02-S10已完成可运行主线，当前执行S11最终验收。
所有实施与验证只能在 `/Users/vinson/Documents/project/个人/local-ai-companion-llm-capability-runtime`、分支
`codex/llm-capability-runtime` 中进行；`master` 只允许只读检查。

## 1. 执行原则与依赖

- 本计划为 inline workflow：主 session 直接实施和校验，不使用 implement/check 子代理，
  不需要 `implement.jsonl/check.jsonl`。
- S01属于原`09-09-llm-capability-runtime`任务的收尾，不属于本任务的新功能实现。
  Capability Runtime全部已知P0/P1/P2未关闭、未复验前，不开始新的Project Health/
  Evolution领域实现。
- 每个批次先加入失败回归，再修改生产代码，局部通过后才进入下一批；不通过时回到当前
  批次，不用兼容 adapter绕过。
- Provider I/O和 planner在数据库事务外；PreparedPayload先冻结。所有 authority mutation、
  result/outcome、assistant facts与watermark在 owning transaction中。
- 普通 conversation始终一次 Main LLM cognition；任何测试或实现不得引入同轮第二回复。
- 当前 worktree承载前置任务和本任务的未提交改动。实施前已记录精确dirty-path/diff基线，
  后续持续保留用户/前序
  session改动；不得 `reset --hard`、`checkout --` 或覆盖整文件来“重来”。

## 2. Ordered checklist

### S00. Worktree、规划与基线门禁

- [x] 断言当前目录、branch、HEAD与worktree映射；确认`master`工作区未承载本任务写入。
- [x] 记录`git status --short`、`git diff --stat master`、`git diff --check master`和当前
  untracked task/docs；不得把前序修改误判为新回归。
- [x] 完整读取本任务`prd.md/design.md/implement.md`、原`09-09`任务文档（包含最新
  `second-review-remediation.md`）和相关backend specs；
  进入开发阶段时运行`trellis-before-dev`。
- [x] 运行当前 Core/Gateway unit-race-vet-build/gofmt基线；分别记录真实失败、环境限制和skip。
- [x] 检查`GO_CORE_TEST_DATABASE_URL`、`REDIS_URL`、`TEMPORAL_ADDRESS`、
  `FLUCTLIGHT_LIVE_PROVIDER_TEST`是否配置，不因未配置而宣称integration通过。
- [x] S01期间继续更新原任务`remediation-evidence.md`；本任务只有在AC00通过并正式start后
  才建立`implementation-evidence.md`。不混写两个任务的migration或完成证据。

退出条件：已满足。worktree安全、基线已知，原`09-09`任务已独立收口并由用户确认。

### S01. 前置任务：Capability Runtime P0/P1/P2 独立收口

归属：`09-09-llm-capability-runtime`。依赖：S00。主要文件：`capability_core.go`、`capability_runtime.go`、
`builtin_capabilities.go`、`native_capabilities.go`、`provider_context.go`、
`memory_intelligence.go`、`mutations.go`、`workflow_ops.go`、`migrations/runner.go`及测试。

#### S01.1 Definition/schema correctness

- [x] 添加全catalog schema-consistency测试：required property必须存在、anyOf/oneOf有实际
  约束、`additionalProperties:false`与runtime validator一致。
- [x] 修复scene/presence缺少`confidence`property与scene start/switch条件失效。
- [x] 完整实现bounded schema validator：object/array/string/number/integer/boolean、required、
  enum、min/max、length/items、anyOf/oneOf和additionalProperties。
- [x] `ValidateOutput`复用完整validator或强类型Result DTO，不只检查required key。

#### S01.2 Provider Arguments / PreparedPayload isolation

- [x] `CapabilityInvocation.Arguments`保持Provider-owned不可变；新增schema-versioned
  `PreparedPayload`，删除在Arguments中注入`prepared_concept/items/revision/idempotency`。
- [x] Image/Schedule的Prepare只读取Arguments + declared Context；prepared payload在任何
  side effect前原子冻结，replay不重新planner。
- [x] 修复Image canonical media DTO：`intent/current_life/current_state/visual_identity/appearance`
  在Prepare、provider compaction和media prompt间同名、同shape、端到端不丢失。
- [x] Schedule以CallID/ActionID建立数据库唯一幂等边界；`AcceptSchedule`不得生成每次不同的
  schedule identity或忽略planner idempotency。

#### S01.3 canonical migration/replay

- [x] 创建新migration revision并推进runner Head；不能复用`0025_llm_queue` ledger状态。
- [x] active预检覆盖pending/claimed/running cognition、autonomy、schedule/media/workflow rows。
- [x] 完整验证v2 envelope，不因已有错误version key静默跳过。
- [x] 转换legacy results为`capability_results`，projection为bounded Slot snapshot；completed
  result在replay中跳过，completed audit rows不重写。
- [x] 对真实PostgreSQL跑upgrade/replay/malformed fail-closed；Temporal history无环境时明确
  保留部署drain/version gate，不能以pure helper替代。

#### S01.4 transaction、Context与错误

- [x] 引入transaction-aware capability apply seam；Memory/Affect/Scene/Schedule/durable intent
  mutation可参与caller-owned Unit of Work，删除先提交后assistant失败的窗口。
- [x] ContextResolver严格slot tag/type、Relationship authorized actor set与空授权fail-closed；
  Capability消费resolved context，live DB只做CAS/authorization/idempotency guard。
- [x] Memory provider schema移除profile/evidence/provenance等runtime-owned metadata。
- [x] typed `CapabilityError`保留schedule planner等bounded domain code/retryability。
- [x] intent普通日志改为presence/length/digest；原文仅可进入受控redacted diagnostics。

退出条件：原第二轮复验全部P0/P1/P2消失；Runtime focused/full/race/vet/build通过；
Capability migration使用新revision并在真实PostgreSQL验证；Temporal active history有
saved-history replay或明确drain/version gate；主session重新审查并单独向用户报告。用户确认
前置任务已处理完后，将原任务标记完成，才允许运行本任务`task.py start`并进入S02。
该退出条件已经满足；后续批次继续把Runtime回归作为组合验收门禁。

### S02. ContextReferenceIndex 与 DecisionInfluence

依赖：AC00与本任务`task.py start`。进入本步骤时建立本任务`implementation-evidence.md`。

- [x] 定义`ContextReferenceKind/ContextReference/ContextReferenceIndex`，为Memory、
  Relationship、Goal、Intention、Scene/Schedule、State/Affect profile、Outcome和evolution
  overlay生成bounded opaque ref。
- [x] 内部snapshot保留ref->entity/revision/scope mapping；Provider DTO只保留ref和semantic
  fields。snapshot round-trip必须保留mapping且不能被Provider覆盖。
- [x] 重写Goal/Intention/Memory等compactor，保留ref而非删除所有身份；继续剔除DB/workflow/
  provider/idempotency/embedding metadata。
- [x] conversation/WakeUp/daily review/native cognition schema增加`influences[]`；实现closed
  schema与validator。
- [x] freeze前验证ref存在、scope/revision匹配、role/confidence/note bounded和duplicate。
- [ ] Goal/Intention lifecycle candidate、proactive action与服务对象之间建立必需引用规则；
  普通reply/no-op允许空数组。
- [x] assessment/frozen action/outcome/reflection evidence持久化validated influences。

测试：provider DTO snapshot、foreign/stale/duplicate/oversized ref、actor/profile scope、
freeze/replay、同一语义文本不依赖DB ID。

退出条件：AC02的reference/influence数据链在fake Provider fixture中贯通。

### S03. Transactional action plan 与 ActionOutcome authority

依赖：S01、S02。

- [x] 把Capability batch分成pure query、prepared transactional mutation、deferred output和
  external async intent；分类来自interface/Definition metadata而非name。
- [x] interactive事务统一提交appraisal transition、personality decision、native mutations、
  frozen invocation/result、assistant/claims、output binding和initial outcome。
- [x] normal/recovery共用一个settle helper；required failure回滚visible output，callback仅
  commit后触发；optional failure结构化保留。
- [x] 定义/迁移单一`ActionOutcome` authority与`(action_id,call_id)`唯一身份；记录success
  boundary、expected/observed、Goal/Intention refs、evidence、revision。
- [x] async worker用CAS更新同一outcome并追加ordered inbox fact；ambiguous result标记
  `unknown`并reconcile，不能盲目重投。
- [x] 新增`SlotRecentOutcomes`及bounded projection/retrieval；下一次cognition和Reflection
  看到相关真实结果，不能依赖assistant prose。
- [x] Recent Outcome Provider DTO使用字段级allowlist并在最终request上测试：只保留opaque
  refs、Capability/status/success boundary/time、bounded error和白名单化结果信号；严禁原始
  文本、raw expected/observed、内部index/entity ID、provenance/evidence及workflow/provider/
  idempotency标识外发。

测试：commit/rollback、crash before/after freeze/intent/assistant/outcome、duplicate retry、
required/optional、async final result、media intent-created与asset-ready语义。

退出条件：AC09通过，Memory/Affect分裂事务回归变为不可能。

### S04. Affect / Drive canonical reducer

依赖：S02、S03。

- [x] 将PAD与momentum range统一为`-1..1`，intensity/pressure仍为`0..1`；拆分
  `clampBipolar`与`clampUnit`，禁止复用错误clamp。
- [x] 设计`AffectProfile` authority：baseline PAD、decay、regulation、emotional summary、
  revision/policy version；初始化neutral baseline不依赖magic positive midpoint。
- [x] 合并appraisal与`affect_event`到一个versioned reducer API；同source语义事件只产生
  一个state revision，记录requested/applied delta。
- [x] 实现elapsed wall-time lazy decay、momentum和regulation；结果与Worker tick次数无关。
- [x] 将built-in drives与typed Drive slots接入semantic signals/conflict projection，保持
  model semantic ownership与Core numeric ownership。
- [x] 编写旧0..1数据reconciliation migration：合法0..1值不能凭空推断为旧/新语义；使用
  migration version/created-at和明确mapping policy，无法安全判断的row fail/defer审计。

测试：positive/negative/extreme/mixed events、负PAD、decay partition invariance、CAS、source
dedupe、atomic turn、projection/influence、migration fixture。

退出条件：AC05通过。

### S05. Memory lifecycle 与 operation-aware retrieval

依赖：S02、S03。

- [x] 将`memory_event`收敛为semantic Arguments + runtime preparation/apply；resolved Persona/
  Memory scope真正参与owner/profile/evidence绑定。
- [x] 实现Memory create/confirm/revise/merge/supersede/deprecate命令、revision和governance；
  duplicate/repetition不创建第二条近似Memory。
- [x] Reflection Memory候选只返回semantic fields、target/merge refs和evidence；Core补充
  revision/provenance/idempotency/embedding。
- [x] 新建bounded retrieval query plan，按surface组合current message/topic、scene、Goal/
  Intention、unresolved items与recent outcomes；空query明确标记salience/recency。
- [x] hard authorization/visibility/conversation/Actor filter先于lexical/vector ranking；
  embedding failure保留bounded deterministic fallback与诊断，不影响事实authority。
- [x] Memory revision、embedding rebuild intent、outbox与apply result原子提交。

测试：AC03完整e2e、visibility leak、profile perspective、revision conflict、merge/supersede、
embedding stale/failure、budget与下一轮引用。

退出条件：Memory从写入、检索、引用、演化到再消费闭环。

### S06. Life Context / Scene reference closure

依赖：S02、S03。

- [x] `resolveContext`返回active event/schedule item/presence refs、source、effective/expiry和
  context revision；不只返回scene/activity文本。
- [x] ContextSnapshot为reply、Image、Schedule、WakeUp和autonomy提供同一frozen revision；
  live变化在apply时按policy conflict/re-assess/defer。
- [x] scene_event在transactional capability path中结算event、native fact、outbox和outcome；
  retry不截断当前scene。
- [x] 删除/阻止所有基于scene字符串自动选择action的路径；Prompt只说明事实authority，
  Definition只说明能力。
- [x] direct conversation提交user后立即stream authoritative user row；assistant提交后stream
  authoritative assistant row；no-visible-reply cognition必须失败并保持retry，Browser history
  merge不得擦除queued/streamed messages。
- [x] 0030按用户确认的clean-start策略拒绝非空旧业务数据库；新Event/Presence/Schedule使用
  NOT NULL idempotency/digest和deferred replay-ready constraint，不再保留repair兼容分支。

测试：event>schedule>pending、presence overlay、expiry、same observation/different scene
influence、stale context、Image/Schedule context一致、无关键词行为。

退出条件：AC04通过。

### S07. Goal / Intention / trigger / action closure

依赖：S02、S03、S04、S06。

- [x] 扩展Goal authority：desired outcome、success criteria、motivation、完整lifecycle、
  importance/urgency/progress/deadline/scope/target、revision/governance。
- [x] Reflection Goal update按frozen ref/expected revision更新允许字段；0031采用clean-start，
  不迁移既存Goal。新Goal缺少criteria时标为needs-reflection，不编造成功标准。
- [x] 扩展Intention：action intent、expected outcome、optional capability constraints、typed
  trigger、preferred time、expiration、attempt identity和完整lifecycle/revision。
- [x] 建立`agency.intention_due` durable fact：time用Temporal timer，event用inbox source，
  semantic仅在新事实时交给LLM；不实现keyword listener。
- [x] cognition response/frozen action绑定served Goal/Intention refs；permission、budget、quiet
  hours、cooldown、context revision和expiration在freeze前重检。
- [x] Outcome机械结算Intention attempt；成功/失败/取消/suppressed分别处理。Goal progress只
  由引用真实Outcome与success criteria的semantic proposal推进，Core计算bounded delta。
- [x] restart/replay复用stable workflow/action/outcome/attempt identity；pause/cancel保留历史。

测试：AC06完整e2e、time/event/semantic trigger、expiration、pause/resume/cancel、budget/quiet、
failed/suppressed no progress、duplicate timer/retry、cross-profile/Actor隔离。

退出条件：Goal/Intention从语义形成到真实动作与结果反馈闭环。

### S08. Reflection V2 与可解释演化事务

依赖：S02-S07。

- [x] 定义closed `ReflectionProposalV2`：summary、Memory、Relationship observations、Goal/
  Intention、emotional summary/recalibration、Drive/Preference/Trigger、Developing Self、
  Personality和Behavioral Policy evolution candidates。
- [x] 删除Provider对profile ID、idempotency、provenance、完整relationship metrics snapshot、
  raw numeric delta的所有权；用refs + semantic direction/strength/confidence/evidence。
- [x] build frozen EvolutionContext：window facts/appraisals/outcomes/current refs与base revisions；
  Provider call在事务外。
- [x] validation分层：malformed/foreign ref whole-proposal invalid；policy threshold/cooldown不足
  candidate deferred/rejected；CAS stale/apply error whole transaction rollback。
- [x] narrow domain services在一个tx应用所有accepted mutations，并同时写proposal、逐候选
  disposition、revision history、outcome和watermark。
- [ ] Reflection outcome已按真实Memory/Developing Self reducer结果返回逐类计数、revisions、
  reason codes与`no_change`；`changed_refs`仍是changed target/audit token，尚未全部变成下一次
  ContextReferenceIndex可直接解析的新revision ref。该可观察性增强留待下一轮。
- [x] WakeUp保持action assessment only；它只写fact和reflection intent，不直接更新Current
  State/personality，不再承担旧legacy cognition阶段。

测试：AC07/AC08、multi-domain atomic commit、empty no_change、bad ref、stale CAS、policy
deferred、partial accepted with explicit dispositions、retry/idempotency和next projection。

退出条件：Reflection不再只有proposal/watermark，而能证明状态变更和后续消费。

### S09. Personality / Behavioral Policy evolution overlays

依赖：S08。

- [x] 新建profile-scoped overlay/revision authority；不修改Identity/Core Persona baseline。
- [x] allowlist Personality trait与Behavioral Policy表达字段；禁止anchor、安全、Owner权限、
  provider/infrastructure和hard character constraints。
- [x] 数值字段应用baseline update policy（window/min confidence/max delta/cooldown）；categorical
  字段要求多窗口一致证据和更高阈值。
- [x] 读取时组合baseline + active overlays为effective persona；Provider接收bounded effective
  view与authority标记，realization继续使用frozen effective snapshot。
- [x] rollback primitive与PostgreSQL补偿revision恢复effective value且不删除历史；跨profile共享
  Memory不被overlay复制。
- [ ] 增加Owner-authorized production rollback API/e2e；当前仅pure + persistence primitive，
  留待下一轮治理接口任务。

测试：阈值未达deferred、多窗口accepted、max-delta/cooldown、forbidden path、profile isolation、
effective projection、frozen realization、rollback。

退出条件：用户确认的慢演化边界通过确定性测试和治理API检查。

### S10. Project Health migration、replay 与 operational gate

依赖：S01-S09。

- [x] 从已验收的Capability migration Head之后创建更高的独立Project Health/Evolution
  revision；只包含S02-S09状态，不重复或改写前置Capability cutover。更新Head与ledger测试，
  禁止一个revision号表达两种已完成状态。
- [x] 真实PostgreSQL运行empty->0031、empty-business 0030->0031、nonempty拒绝、
  closed constraints、rerun/idempotent、transaction/CAS/replay与失败rollback；0031不迁移
  active/completed旧业务row。
- [ ] 运行Temporal saved history/replay或明确部署drain + Worker build/version gate，覆盖
  intention timer、reflection、daily review、media/schedule active histories。
- [ ] 运行Redis expiry/reconnect/duplicate/startup repair与PostgreSQL fallback；Redis不成为
  business authority。
- [x] 验证async action result、outcome、reflection window在process restart后不重复或丢失。

环境记录：S10执行时`REDIS_URL`、`TEMPORAL_ADDRESS`与`FLUCTLIGHT_LIVE_PROVIDER_TEST`均未配置；
Redis/Temporal两项保持未勾选，不以pure helper或PostgreSQL测试替代真实外部环境证据。

退出条件：migration/replay环境证据真实存在；无法运行的外部环境项明确标红且不能宣称
production-ready。

### S11. Prompt、文档、架构 guard 与最终验收

依赖：S10。

- [x] Prompt只保留全局semantic ownership、context authority、real capability、influence refs、
  Reflection operation rules；schema细节不重复写进Prompt，Definition保持一句能力说明。
- [x] 本任务只增加最小Recent Outcomes section/authority说明；不重写整体Prompt结构与语气，
  完整提示词重构留给用户后续独立任务。
- [x] 更新`docs/capability-architecture.md`、README和backend specs，使其描述真实最终代码；
  删除“future/placeholder”冒充当前闭环的描述。
- [x] 增加static guards：legacy symbols/concrete-name dispatch/v1-first replay/semantic regex/
  runtime-owned reflection fields/PAD wrong clamp/second cognition。
- [ ] 运行全量fake e2e矩阵和真实环境门禁；输出Memory/Scene/Affect/Goal/Reflection五条
  closure trace，每条包含refs/revisions/outcomes。
- [x] PRD acceptance逐项映射到测试/命令/证据；未验证项不勾选。
- [x] 检查`master` status、worktree diff、generated artifacts与untracked文件；不自动commit/
  merge/push，除非用户随后明确要求。

退出条件：AC00-AC14全部有证据，或明确报告仍阻塞的外部环境项并保持task in_progress。

## 3. Validation commands

### 3.0 Requirements traceability

| Requirement | Owning batches | Acceptance |
| --- | --- | --- |
| R00 Capability Runtime closure | S00、S01 | AC00、AC01 |
| R01 Context refs/influences | S02、S03 | AC02、AC09、AC11 |
| R02 Memory | S05、S08 | AC03、AC07-AC09 |
| R03 Life Context/Scene | S06 | AC02、AC04 |
| R04 Affect/Drive | S04、S08 | AC05、AC07 |
| R05 Goal/Intention/Action | S07、S08 | AC06-AC09 |
| R06 ActionOutcome | S03、S07、S08 | AC02、AC06、AC09 |
| R07 Reflection | S08、S09 | AC07、AC08、AC14 |
| R08 Governance/observability | S08-S11 | AC08、AC11、AC14 |
| R09 one cognition/no heuristic | S01、S11 | AC10、AC11 |
| R10 migration/replay | S01、S10 | AC00、AC01、AC09、AC12 |

### 3.1 Pure/full Go gates

```bash
GOCACHE=/tmp/lac-health-core-test go -C apps/core-go test ./... -count=1
GOCACHE=/tmp/lac-health-core-race go -C apps/core-go test -race ./... -count=1
GOCACHE=/tmp/lac-health-core-vet go -C apps/core-go vet ./...
GOCACHE=/tmp/lac-health-core-build go -C apps/core-go build ./...
GOCACHE=/tmp/lac-health-gateway-test go -C apps/gateway-go test ./... -count=1
GOCACHE=/tmp/lac-health-gateway-race go -C apps/gateway-go test -race ./... -count=1
GOCACHE=/tmp/lac-health-gateway-vet go -C apps/gateway-go vet ./...
GOCACHE=/tmp/lac-health-gateway-build go -C apps/gateway-go build ./...
test -z "$(find apps/core-go apps/gateway-go -name '*.go' -print0 | xargs -0 gofmt -l)"
git diff --check master
```

### 3.2 Focused deterministic gates

实际测试名在实现时固定，但必须覆盖以下分组：

```text
capability schema / prepared payload / replay / transaction / outcome
context reference / influence validation / projection round-trip
affect reducer / decay / migration
memory lifecycle / retrieval / visibility
scene context priority / stale snapshot
goal intention trigger action outcome progress
reflection V2 / evolution overlay / watermark CAS
single-cognition and no-semantic-heuristic architecture guards
```

### 3.3 Opt-in real environment gates

```text
GO_CORE_TEST_DATABASE_URL: PostgreSQL migration, transaction, CAS, replay
REDIS_URL: expiration listener, reconnect, duplicate hint, PG fallback
TEMPORAL_ADDRESS: intention timer, active history replay/drain/versioning
FLUCTLIGHT_LIVE_PROVIDER_TEST=1: schema/prompt compatibility probes only
```

每项运行时记录endpoint类型/版本、命令、pass/fail/skip和时间；不得在文档中只写“integration
passed”而没有配置与结果。

## 4. 风险文件与回滚点

| 批次 | 高风险文件/区域 | 主要风险 | 回滚点 |
| --- | --- | --- | --- |
| S01 | capability core/runtime、mutations、workflow、migration | 重复side effect或active replay损坏 | 仅保留新增失败测试，回到当前direct Runtime基线 |
| S02-S03 | provider context/schema、frozen payload、action result | ref丢失、可见提交与状态分裂 | 保留v2 migration未启用，回退到pre-reference schema |
| S04 | cognition reducer、inner state、affect migration | 负值错误、双revision、旧数据误解释 | 停止migration cutover，恢复旧reader并保留审计脚本 |
| S05-S07 | Memory、Life Context、Goal/Intention、Temporal | 隐私泄漏、重复action、错误progress | 关闭新workflow dispatch，不删除authority history |
| S08-S09 | Reflection、evolution overlays | watermark越过坏窗口、人格漂移 | rollback transaction/overlay revision；不改Core baseline |
| S10 | migration runner、active histories | 无法回放或两种Head语义 | 阻止部署；drain/repair后重新升级，不加永久compat |

任何回滚都不能删除已提交的用户事实、Memory revision、Outcome或governance history；通过新
revision/compensation恢复状态。实现过程中不使用破坏性Git命令。

## 5. 完成报告必须包含

- 修复后的最终Capability表与原`09-09`全部P0/P1/P2 disposition及独立复验结论；
- Memory/Scene/Affect/Goal/Reflection各一条完整closure trace；
- 新增/修改的schema、table、migration revision和active replay结果；
- 删除的legacy/compat/name switch/runtime-owned Provider字段；
- Prompt/Definition/Implementation责任变化；
- Core/Gateway与真实PostgreSQL/Redis/Temporal/live Provider测试结果；
- schema bytes/chars变化和普通conversation Main LLM调用次数证明；
- 未验证环境、remaining risk和是否允许commit/merge的明确结论。
