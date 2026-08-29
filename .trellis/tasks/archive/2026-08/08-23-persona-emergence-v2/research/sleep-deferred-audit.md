# Research: Sleep And Deferred Reply Audit

- Query: 审计当前睡眠/延迟回复链路：为何“正在自己的空间里休息”会被判 sleeping；当前状态、场景、daily plan 的来源与优先级；新 persona 创建时 daily_plan 是否真正插入并 ready；relationship/intimacy/affect/random 是否进入 LLM deferred reply decision。
- Scope: internal
- Date: 2026-08-23

## Findings

### 1. “正在自己的空间里休息”被判 sleeping 的根因

有两个独立的睡眠判定器，导致同一生活状态可能得到不同结果。

- 默认 blueprint 的 `routine` 是空数组；persona 创建时仍会写入一个默认生活 blueprint，resolver 在没有 presence、life event、explicit schedule 或有效 daily plan 时走 `routineProjection()` 的 fallback，生成 `source: 'baseline'` 和 `situation: '正在自己的空间里休息'` (`server/infrastructure/persona-lifecycle-repository.js:25-43`, `server/domain/life-state-resolver.js:427-437`, `server/domain/life-state-resolver.js:493-494`)。
- `createLifeStateService().sleepAvailability()` 用 `/睡|休息|躺|寝室|卧室/i` 匹配整个 `situation`，所以只要状态文案包含“休息”，即返回 `sleeping: true`，不检查 `source`、`slotKind`、时间边界或明确睡眠事实 (`server/application/life-state-service.js:89-93`)。运行时探针已复现：baseline + “正在自己的空间里休息”在白天也返回 `sleeping: true`。
- 生产聊天的 deferred policy 当前没有复用上述 service，而是在 `createDefaultChatProductionPorts()` 注入 `resolveDeferredSleep()` (`server/runtime/runtime.js:257-312`)。该实现只把 `daily_plan_baseline` 且文案匹配 `/睡|赖床|自然醒|起床前/` 当作明确 plan sleep；否则 08:00-23:00 immediate，其他小时统一返回 `sleeping: true` (`server/runtime/runtime.js:276-305`)。因此该文案在生产聊天里夜间仍会被判 sleeping，白天则因时段规则 immediate；这是“按时间兜底”的根因，不是该文案本身的语义证明。
- `createDeferredChatPolicy()` 的通用 fallback 反而不会把普通“休息”判为睡眠，只接受显式 `state.sleeping/isSleeping` 或 `daily_plan_baseline` 的睡眠正则 (`server/application/deferred-chat-policy.js:159-173`)；它不是默认生产路径的实际判定器。

最小修复建议：只从结构化事实判定睡眠，例如 `source === 'daily_plan_baseline' && slotKind === 'baseline_sleep'` 或显式 `sleepPhase`/`interactionAvailability`；移除 `life-state-service.js:92` 的场景词正则，并取消 runtime 对所有非白天小时的 blanket sleep fallback。普通 `routine`/`baseline` 的“休息”只能表示休息/低活动，不能自动等价睡眠。若 v2 的 LLM-first 要求覆盖延迟语义，则应由结构化 LLM availability/agency candidate 提出立即或延迟，服务端只做安全、幂等、租约和时间边界校验。

### 2. 当前状态、场景和 daily plan 的实际来源与优先级

`createLifeWorldReader.readResolverInput()` 每次按 persona 和时间同步读取 blueprint、state、schedule items、life events、daily plan、presence，再交给纯 resolver；它会过滤跨 persona 行，并排除 appearance-only life events (`server/application/life-world-reader.js:299-315`, `server/application/life-world-reader.js:398-419`)。生产 chat context 与 deferred worker 均使用这个 reader/resolver 组合 (`server/runtime/runtime.js:257-274`, `server/runtime/runtime.js:320-324`, `server/infrastructure/production-proactive-ports.js:390-403`)。

实际 resolver 优先级是：

1. active presence/shared scene：候选为 `presence`, `presenceSnapshot`, `sharedScene`, `sharedSceneSnapshot`，要求 persona scope、active status、已开始且未结束，并且至少有 situation/activity/scene/location (`server/domain/life-state-resolver.js:273-291`)。
2. active life event：排除 `routine`/`schedule` 类型和 inactive/terminal 状态，按 priority、开始时间和稳定 id 选择 (`server/domain/life-state-resolver.js:294-310`)。
3. active explicit schedule：只接受非 generated source（generated/ai_daily_plan/daily_plan 等被排除），要求已开始且未结束；按 priority、开始时间选择 (`server/domain/life-state-resolver.js:317-337`, `server/domain/life-state-resolver.js:477-485`)。
4. ready daily plan：选择 plan/projection，要求 ready、当前 persona、plan date 等于 blueprint timezone 下的本地日期；先取当前 active slot，否则生成 daily-plan baseline idle/sleep slot (`server/domain/life-state-resolver.js:339-424`, `server/domain/life-state-resolver.js:487-490`)。
5. blueprint routine；没有匹配 routine 时使用 baseline fallback “正在自己的空间里休息” (`server/domain/life-state-resolver.js:427-437`, `server/domain/life-state-resolver.js:493-494`)。

状态字段不是直接信任 persisted `companion_persona_states.situation`：`life-state-service.project()` 保留持久化行作为底层字段，但用 resolver 的 situation/mood/appearance/source/time boundary 覆盖并附加 `resolved_*` 元数据 (`server/application/life-state-service.js:22-55`)。场景由 event/presence/plan 记录先提供 `scene`, `location`, `room`，缺失时按 blueprint `defaultSceneRef` 回退 (`server/domain/life-state-resolver.js:158-183`, `server/domain/life-state-resolver.js:225-263`)。

daily plan repository 查询该 persona 最新 `plan_date` 行，同时拼接 timeline slots、解析 `plan_json`，但不自行过滤 status/date；ready 和本地日期过滤由 resolver 完成 (`server/infrastructure/daily-plan-repository.js:5-16`, `server/domain/life-state-resolver.js:347-353`, `server/domain/life-state-resolver.js:405-424`)。context reader 把最终 resolved life state、可信时间边界、appearance、dailyPlan、presence 和 affect posture 放入 life layer；identity/life/memory/relationship/affect/self-model 再按 priority 100/90/50/40/45/55 进入 context pipeline (`server/runtime/runtime.js:320-385`)。

### 3. 新 persona 的 daily_plan 是否真正插入并 ready

插入是“是”，ready 是“否/不可靠”：

- `createPersonaLifecycleRepository.createPersona()` 的同一 SQLite transaction 会创建 persona、foundation、blueprint、initial state、conversation，以及一个当天 `companion_daily_plans` 行，初始 `status='queued'`, `plan_json='[]'`, `source='modular-default'` (`server/infrastructure/persona-lifecycle-repository.js:55-76`)。
- transaction 之后另 enqueue 一个 `daily_plan` job，但 payload 只有 `{planDate}`，没有新建 plan 的 `dailyPlanId`，也没有 plan slots/LLM 生成结果 (`server/infrastructure/persona-lifecycle-repository.js:75-78`)。运行时探针实际得到：`daily_plan.status=queued`, `plan_json=[]`，job payload 为 `{"planDate":"2026-08-23"}`。
- 生产 maintenance handler 通过 `payload.dailyPlanId ?? payload.id` 调 `markReady()`；由于创建 payload 没有这两个字段，`markReady()` 的 `UPDATE ... WHERE id = ?` 不会更新任何行 (`server/runtime/runtime.js:968-976`, `server/infrastructure/daily-plan-repository.js:18-24`)。当 handler 继续调用 timeline sync 时，plan repository 仍返回 queued 行，`syncDailyPlanSlots()` 按默认策略跳过 `plan_not_ready` (`server/application/timeline-flow.js:802-806`)。
- handler 的返回对象使用 `plan?.status ?? 'ready'`，所以即使 `markReady()` 没有找到/更新行，也可能对上层报告 `status: 'ready'`；这会掩盖数据库真实状态 (`server/runtime/runtime.js:971-976`)。当前创建流程也没有任何 daily-plan planner/provider 调用，不能证明“首日计划”已填充或 slots 已 ready。

最小修复建议：创建 queued row 时保留其 id，并把 `dailyPlanId` 放进 job payload；handler 必须检查 `markReady()` 返回行/changes，更新失败时返回明确 retry/failed，不能 `?? 'ready'`。如果目标是实际首日计划，则在 markReady 前完成受 schema 校验的 plan 生成并写入 `plan_json`，再同步 slots；如果空计划是合法 fallback，也必须显式写入 ready 空计划并由测试锁定该语义。当前创建事务确实保证了 row 与 persona 原子存在，但不保证 ready 或非空。

### 4. relationship/intimacy/affect/random 是否进入 deferred reply 的 LLM 决策

要区分“是否延迟”与“到期后如何回复”：

- 聊天 flow 先写 user message，再运行 `deferred-chat-policy`；policy 命中后写入 deferred batch 并直接短路后续 context/LLM steps (`server/application/chat-turn-flow.js:444-483`, `server/application/chat-turn-flow.js:489-521`)。因此 LLM 不参与首次 immediate/deferred 选择。
- 生产 `resolveDeferredSleep()` 在 policy 前置阶段读取最近 user message 数、relationship `activePatch()`、affect snapshot 的 social/rest drives，并计算 intimacy、deterministic draw 和 immediate threshold (`server/runtime/runtime.js:276-305`)。relationship 只要存在 communicationStyle 就加 1 intimacy；intimacy 主要来自 user message count 分段；affect 只通过 social/rest pressure 调整阈值；draw 是 persona/time/userCount 的稳定 hash，不是消息语义分析。传入的 `userMessage` 参数未被使用，所以 message importance、sleep phase、消息内容都没有进入该决策 (`server/application/deferred-chat-policy.js:347-367`, `server/runtime/runtime.js:276-305`)。
- batch 创建时将 `{intimacy, draw, reason:'sleep_deferred'}` 固化到 `decision`，并将 batch/job 持久化；后续消息只 append 到 active batch，不能改写首条决定 (`server/application/deferred-chat-policy.js:294-333`, `server/application/deferred-chat-policy.js:347-367`, `server/infrastructure/production-proactive-ports.js:300-335`)。
- 到期 worker 才会调用 reply composer；它读取 deliverAt 的 life-world，再以 `batch`, `messages`, `lifeWorld` 调 `replyComposer.compose()` (`server/application/proactive/worker-flows.js:512-565`)。生产 composer 另外调用 `contextReader`，把 identity/life/memory/relationship/affect/self-model context 和能力提示序列化给 reply LLM，同时把 batch（其中包含 intimacy/draw）、messages、lifeWorld 放入 user JSON (`server/infrastructure/production-proactive-ports.js:390-403`, `server/runtime/runtime.js:339-412`, `server/application/context-pipeline.js:85-92`, `server/application/context-contracts.js:65-87`)。

结论：relationship、affect、intimacy、draw 会在“已延迟、到期生成回复”的 LLM prompt 中出现（relationship/affect 经 context layer；intimacy/draw 经 batch JSON），但它们不是 LLM 的 deferred decision 输入源；真正的延迟决策已被服务端规则提前冻结。若 v2 要求拒绝/延迟的语义判断 LLM-first，当前实现不满足，且没有结构化 `message importance`、`sleep phase` 或 agency/refusal candidate 通道。最小修复是把睡眠可用性/延迟候选作为版本化 LLM structured decision 输入，并让服务端只校验边界、保存冻结结果；在此之前至少不要把 relationship/affect/random 的 server heuristic 描述成 LLM 决策。

## Files Found

- `server/domain/life-state-resolver.js`: 纯生活状态 resolver、来源优先级、plan readiness 和 baseline 文案。
- `server/application/life-state-service.js`: 对外 state/sleep projection；包含过宽的 situation 正则睡眠判定。
- `server/application/life-world-reader.js`: 从 repositories 读取并 persona-scope blueprint/state/schedule/events/plan/presence。
- `server/application/deferred-chat-policy.js`: 聊天 preflight、active batch 合并、deferred batch 决策持久化边界。
- `server/application/chat-turn-flow.js`: user-message boundary、policy short-circuit 与后续 LLM steps 的顺序。
- `server/runtime/runtime.js`: production reader、runtime sleep heuristic、context layer、maintenance daily_plan handler。
- `server/infrastructure/persona-lifecycle-repository.js`: persona 与 queued daily plan/job 的创建事务。
- `server/infrastructure/daily-plan-repository.js`: latest-plan read 与 markReady 更新。
- `server/application/timeline-flow.js`: ready plan 的 slot sync gate。
- `server/application/proactive/worker-flows.js`: deferred batch 到期、lease、reply projection。
- `server/infrastructure/production-proactive-ports.js`: 到期 deferred reply 的 context + provider LLM composer。
- `server/application/context-pipeline.js`, `server/application/context-contracts.js`: LLM prompt context 序列化与层级。
- `test/life-state-resolver.test.mjs`, `test/life-state-integration.test.mjs`, `test/life-proactive-worker-flows.test.mjs`: precedence, integration, deferred worker regression coverage。

## External References

- None. Findings are based on the current repository and its persisted Trellis design/contracts; targeted Node test execution passed 21 tests.

## Related Specs

- `.trellis/tasks/08-23-persona-emergence-v2/prd.md`: LLM-first semantic decisions; service-side rules limited to safety, integrity, idempotency and resource governance.
- `.trellis/tasks/08-23-persona-emergence-v2/design.md`: shared interaction → appraisal → affect/memory/self-model/agency boundary and LLM-first failure behavior.
- `.trellis/spec/backend/structured-turn-contract.md`: structured control, server-owned affect reducer, source ownership and no text-regex side effects.
- `.trellis/spec/backend/persona-analysis-and-media-jobs.md`: ready daily-plan proactive trigger contract and LLM-gated semantic decision boundary.
- `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/prd.md:67-81`: prior sleep contract: chat allowed in all scenes; intimacy may affect immediate probability but deferred decision is durable and non-interruptible.
- `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/design.md:211-232`: prior intended sleep decision inputs (intimacy, message importance, sleep phase, random draw) and batch lifecycle.

## Caveats / Not Found

- No current v2 implementation of a structured agency/refusal/deferred-availability candidate was found; the repository has structured affect/appraisal/self-model channels, but this audit found no LLM-owned sleep/defer schema.
- `lifeStateService.sleepAvailability()` is not the production chat port in the default composition; production chat injects `resolveDeferredSleep()`. Both paths are reported because they disagree and either can be consumed by callers/tests.
- `daily_plan` maintenance registration is enabled only in the default production composition path; custom `options.repositories`/handler composition can omit it. In those compositions the queued row remains queued unless an explicit handler is supplied (`server/runtime/runtime.js:962-976`).
- Relevant targeted tests pass, but they do not assert persona creation's daily-plan job payload contains `dailyPlanId`, nor that executing that job changes the row from queued to ready; they also do not assert ordinary “rest” is distinct from sleeping.
