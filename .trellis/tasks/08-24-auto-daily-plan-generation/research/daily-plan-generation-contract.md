# Research: Daily Plan Generation Contract

- Query: 仓库现有设计、任务和代码是否定义了“新 persona 每天自动生成安排”的具体来源和格式？重点审计 daily-plan repository、timeline-flow、runtime maintenance handler、blueprint 的 routine/fixed/flexible 事件，以及 08-18/08-19/08-23 相关任务与 specs。
- Scope: mixed (internal code, Trellis specs/tasks)
- Date: 2026-08-24

## Findings

### 1. 当前生产行为：只保证创建日，不保证跨本地日

- `createPersona()` 从输入 `blueprint`、`blueprintFactory` 或 deterministic fallback 得到生活模型，并以 blueprint 时区计算创建日的 `planDate` (`server/infrastructure/persona-lifecycle-repository.js:124-143`)。
- 创建事务内生成的初始 plan 形状是 `{schemaVersion: 1, timezone, planDate, items: [], timeline: [initialBaseline]}`；baseline 是覆盖当地日 `00:00` 至 `24:00` 的 `baseline_idle` 槽位，来源为 `daily_plan_baseline`，地点/房间取 blueprint 默认世界 (`server/infrastructure/persona-lifecycle-repository.js:60-92`, `server/infrastructure/persona-lifecycle-repository.js:144-150`)。
- 该 plan 当前已经以 `status='ready'` 写入 `companion_daily_plans`，同时把同一个 baseline 写入 `companion_timeline_slots`，因此新 persona 的首日状态不依赖 worker tick (`server/infrastructure/persona-lifecycle-repository.js:151-170`)。这与数据库 spec 的“创建时必须原子创建当天 ready plan 和至少一个 baseline”一致 (`.trellis/spec/backend/database-guidelines.md:87-104`)。
- 创建事务后只 enqueue 一条 `daily_plan` job，payload 为 `{dailyPlanId, planDate}`，`runAfter` 是创建时间 (`server/infrastructure/persona-lifecycle-repository.js:171-172`)。现有代码中没有按 persona、本地日期或日期边界创建后续 job 的其他生产调用；搜索到的 `timeline_reconcile` 只是在 timeline flow 中消费已有 slots (`server/application/timeline-flow.js:897-905`, `server/runtime/runtime.js:1040-1047`)。
- worker runtime 只回收 lease、立即/定时 claim 已存在的 durable jobs；它没有 persona 扫描、时区边界检测或“今天没有 plan 就创建”的职责 (`server/runtime/worker-runtime.js:109-116`, `server/runtime/worker-runtime.js:168-185`, `server/runtime/worker-runtime.js:236-255`, `server/runtime/worker-runtime.js:257-303`)。因此跨到下一个 persona 本地日后，当前不会自动生成新的 `companion_daily_plans` 行或 `daily_plan` job。

### 2. daily-plan repository 是读取/状态适配器，不是生成器

- `createDailyPlanRepository()` 的 `hydrate()` 会读取该 plan 日期的全部 timeline slots，解析 `plan_json`，并暴露 `{items, plan, dailyPlan, timeline}` 兼容形状 (`server/infrastructure/daily-plan-repository.js:3-15`)。
- 无 plan id 时，`read()` 只按 persona 的 `plan_date DESC LIMIT 1` 取最新一行；它不接收或使用 `planDate`/`at` 作为日期过滤条件 (`server/infrastructure/daily-plan-repository.js:16-22`)。这意味着日期边界前后，reader 可能先读到旧日期，随后由 resolver 发现日期不匹配并降级，而不是由 repository 补建当日 plan。
- repository 仅提供 `read/readById` 别名和 `markReady/complete`。`markReady()` 只把指定 id 从 `queued/processing` 更新为 `ready`，没有 `create/ensure/upsert`、plan 内容写入、planner 调用或 job 去重 (`server/infrastructure/daily-plan-repository.js:24-33`)。

### 3. runtime maintenance handler 只处理已有 plan，不生成下一日计划

- 生产组合把 `dailyPlan` repository 注入 runtime (`server/runtime/runtime.js:767-834`)，但内置 maintenance handler 只有在 default production composition 且没有冲突的显式 repository/flow 配置时才注册 (`server/runtime/runtime.js:1013-1018`)。自定义 repositories 若没有显式 handler，`daily_plan` job 不会被消费。
- `daily_plan` handler 的实际步骤是：解析 job payload，按 payload id 或最新 plan 读取已有行，调用 `markReady()`，然后把已有 payload/stored plan 交给 `timelineFlow.syncDailyPlanSlots()` (`server/runtime/runtime.js:1019-1038`)。它没有 blueprint-to-plan 生成器、LLM planner、日期推进、`ensureDailyPlan` 调用或 next-day enqueue。
- handler 不能证明 `markReady()` 发生了状态转换：repository 返回的是更新后的 row，即使 `UPDATE ... WHERE status IN ('queued','processing')` 没有命中也可能返回原来的 `ready` row (`server/infrastructure/daily-plan-repository.js:27-31`)。当前新 persona 初始行本来就是 ready，所以该 job 的实际作用主要是重新同步首日 slots。
- `timeline-flow` 的 `handleJob()` 对 `daily_plan` 只是把 job 的 `personaId/planDate/plan` 转给 `syncDailyPlanSlots()`；它也不创建 plan (`server/application/timeline-flow.js:897-905`)。

### 4. 当前 plan/slot 格式和来源

#### 4.1 持久化首日 fallback 格式

当前实现的确定性首日来源是 lifecycle repository 的 blueprint world/default room，不是 LLM daily planner。初始 baseline 字段包括：

```json
{
  "schemaVersion": 1,
  "timezone": "<blueprint.timezone>",
  "planDate": "YYYY-MM-DD",
  "items": [],
  "timeline": [{
    "id": "<dailyPlanId>:baseline:initial",
    "slotKey": "<dailyPlanId>:baseline:initial",
    "slotKind": "baseline_idle",
    "title": "日常休息",
    "situation": "正在自己的空间里休息",
    "scene": "<default room scene>",
    "sceneRef": "<default location/room ref>",
    "location": "<default location>",
    "room": "<default room>",
    "startsAt": "<local day start as ISO>",
    "endsAt": "<next local day start as ISO>",
    "source": "daily_plan_baseline",
    "status": "confirmed",
    "priority": 0,
    "constraints": {},
    "outcome": {}
  }]
}
```

来源和字段实现分别见 `server/infrastructure/persona-lifecycle-repository.js:60-92`、`server/infrastructure/persona-lifecycle-repository.js:144-150`。数据库行还保存 `id/persona_id/plan_date/status/plan_json/source/created_at/updated_at`，并以 `(persona_id, plan_date)` 唯一 (`server/runtime/startup.js:273-282`)。

#### 4.2 timeline-flow 接受的输入格式

- `normalizeDailyPlan()` 优先使用非空 `plan.items`，否则使用 `plan.timeline`，最多保留 32 个槽位；空数组也被接受 (`server/application/timeline-flow.js:325-342`)。
- 每个非空槽位至少需要 title/situation/label 之一和有效 startsAt/endsAt；支持 `HH:mm`（按 plan timezone 转 ISO）或 ISO 时间。默认 `slotKind='planned'`、`source='daily_plan'`、`status='confirmed'`，并保留 scene/sceneRef/location/room/priority/constraints/outcome (`server/application/timeline-flow.js:291-322`)。
- 槽位不能重叠；同步时先删除同日期未被 decision/event 绑定的过期生成槽，再 upsert normalized slots (`server/application/timeline-flow.js:337-342`, `server/application/timeline-flow.js:819-843`)。
- `daily_plan_baseline` 或 `baseline_*` 槽位只做连续状态投影，不生成 candidate job；其它槽位会生成一个 `timeline_candidate`，其 idempotency key 为 `timeline:candidate:<personaId>:<planDate>:<slotKey>`，runAfter 为槽位开始时间 (`server/application/timeline-flow.js:345-385`, `server/application/timeline-flow.js:850-861`)。ready plan slot 到期后的 candidate 才进入 LLM-gated proactive decision，不能把 slot 入库等同于主动联系用户 (`.trellis/spec/backend/persona-analysis-and-media-jobs.md:136-168`)。

#### 4.3 读取时的权威和日期行为

- life-world reader 每次按 persona 读取 blueprint/state/schedules/life events/daily plan/presence，再交给 resolver；daily plan 通过 repository 的 `read()` 读取并拆成 plan/projection (`server/application/life-world-reader.js:299-355`, `server/application/life-world-reader.js:398-419`)。
- resolver 只有在 plan ready 且 `planDate` 等于 blueprint timezone 的当前本地日期时才使用它；没有当前活动 slot 时生成 `daily_plan_baseline` 覆盖日间空隙 (`server/domain/life-state-resolver.js:346-451`)。优先级是 presence/event/explicit schedule/daily plan/routine (`server/domain/life-state-resolver.js:492-520`)。
- 因而没有下一日本地日期的 ready row 时，旧 plan 不会被错误用于新日，但会回落到 routine/default baseline；这解释了“跨本地日未生成 daily plan”而不是格式解析错误。

### 5. blueprint routine/fixed/flexible 的实际定义和缺口

- fallback blueprint 同时定义 `routine`、`fixedTimeEvents`、`dailyFlexibleEvents`、`randomPositiveEvents`、`randomNegativeEvents`，并提供默认世界/房间 (`server/infrastructure/persona-lifecycle-repository.js:94-112`; read-time fallback 也保留这些数组于 `server/infrastructure/blueprint-repository.js:5-22`)。
- 当前生产 resolver 只消费 `blueprint.routine` 作为没有 ready plan 时的 legacy fallback；没有发现将四类 blueprint event 数组展开成当天 slots 的 composer/normalizer (`server/domain/life-state-resolver.js:453-464`)。timeline repository 只在删除生成槽时把 `life_model_fixed/life_model_flexible/life_model_opportunity` 列为可删除 source，并不从 blueprint 生成这些槽 (`server/infrastructure/timeline-repository.js:117-132`)。
- 08-18 的设计要求当天编排顺序为“用户确认日程 -> fixed -> flexible -> free windows -> opportunities”，并把 daily plan、slots、候选决策持久化 (`.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/design.md:170-188`)；实施清单仍把 fixed/flexible normalizer 和 daily timeline composer 列为工作项 (`.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/implement.md:19-28`, `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/implement.md:46-57`)。因此这些字段是设计目标/兼容形状，不是当前生成来源。

### 6. 历史任务/spec 对“每天自动生成”的明确契约

- 08-18 PRD 要求 LLM 生活模型包含固定时间、每日可偏移、随机正/负候选，并“每天基于生活模型编排完整时间线” (`.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/prd.md:3-12`, `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/prd.md:33-55`)。其兼容要求保留 `companion_daily_plans`/`companion_jobs`，且计划、事件、job 幂等 (`.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/prd.md:91-97`)。
- 08-19 状态一致性任务进一步规定 ready plan 是当天全天唯一权威；legacy routine 仅在 plan 尚未生成、失败或不存在时 fallback，并要求计划首块前后也有连续 baseline (`.trellis/tasks/archive/2026-08/08-19-state-plan-consistency/prd.md:7-20`, `.trellis/tasks/archive/2026-08/08-19-state-plan-consistency/prd.md:42-55`)。当前首日 baseline 满足这条读取契约，但没有跨日生成器。
- backend `persona-analysis-and-media-jobs` spec 给出最明确的 daily-plan 生成契约：每个 persona 每个本地日一个 `daily_plan` durable job；本地模型返回 2-6 条普通、可逆安排；签名为 `ensureDailyPlan(personaId, date?) -> {id, personaId, planDate, status}`；JSON 仅允许 `items[{title, scene, situation, startsAt, endsAt}]`；已有同 persona/date plan 必须复用且不得重复 enqueue (`.trellis/spec/backend/persona-analysis-and-media-jobs.md:147-169`)。
- 同一 spec 只定义 ready slot 到期后的 proactive candidate/LLM 门控，不定义如何从 blueprint 四类模板具体合成 2-6 个 item (`.trellis/spec/backend/persona-analysis-and-media-jobs.md:136-168`)。所以“每天生成”的触发、幂等和粗格式已定义，安排语义来源仍需 planner/composer 设计。
- 08-23 emergence 任务明确两种初始化模式共享生活状态等运行时能力，`llm_defined` 可以携带确认后的初始 blueprint，`blank_slate` 只建立最小身份锚点 (`.trellis/tasks/08-23-persona-emergence-v2/prd.md:5-24`)；它没有新增 daily-plan 生成器。其已有 sleep/deferred audit 记录过 daily-plan handler/ready 状态问题，但该研究基于 2026-08-23 代码快照，不能覆盖当前已改为首日 ready、payload 带 `dailyPlanId` 的实现 (`.trellis/tasks/08-23-persona-emergence-v2/research/sleep-deferred-audit.md:36-45`)。

### 7. 最小可行修复建议

按当前边界，最小修复应先补“每 persona、每本地日的幂等计划确保和 job 链”，再决定是否接入 LLM planner：

1. 在 daily-plan repository 或独立 application port 增加 `ensureDailyPlan({personaId, planDate, blueprint, ...})`，以现有 `(persona_id, plan_date)` unique 为幂等边界；已存在行直接返回，不能重复创建/入队。创建时至少写入 ready 的连续 baseline（保持当前首日 fallback 语义），并创建对应 `daily_plan` job，payload 必须含 `dailyPlanId`、`personaId`、`planDate`。
2. 让 runtime maintenance handler 调用该 ensure port，并在 job 成功后确保目标日期的 plan/slots 已经同步；若 mark/update 没有命中或 plan 仍不 ready，返回 retry/failed，不能把默认字符串 `ready` 当作真实状态 (`server/runtime/runtime.js:1030-1038`)。
3. 在每日 job 完成后按 persona blueprint timezone 计算 next local date，并幂等 ensure/enqueue 下一日；同时保留一个 worker tick 的补偿扫描或启动恢复路径，覆盖服务离线跨日、job 丢失和 lease recovery。仅依赖固定 24h timer 会在时区/DST/停机时漏日。
4. 第二阶段再接入显式 planner/composer port：若遵循现有 spec，planner 输出受限的 2-6 条 `items[{title,scene,situation,startsAt,endsAt}]`，服务端校验时间、重叠、地点、可逆性后写 plan，并由 `timelineFlow.syncDailyPlanSlots()` 投影；planner 不可用时保留可审计 baseline fallback或 retry policy，不能静默把 routine 当成新日 plan。
5. 若要真正使用 blueprint 的 fixed/flexible/random 四类事件，应先实现 normalizer/composer，把它们映射为 `slotKind/source`（如 `life_model_fixed`、`life_model_flexible`、`life_model_opportunity`），再复用现有 slot upsert/decision/candidate 幂等链；当前仅有数组字段和 source allowlist，不足以推断时间窗口、时长、冲突、预算或风险规则。

### 8. 潜在兼容风险

- 把 `read()` 改成严格按 `planDate` 查询会影响 life-world reader、timeline-flow 和旧调用者；当前 repository 忽略 `at/planDate`，新增日期参数应保留最新 plan 的兼容别名或同步改所有调用点 (`server/infrastructure/daily-plan-repository.js:16-25`, `server/application/life-world-reader.js:314-355`)。
- 将新日 plan 从 `queued` 改成 `ready` 的时机影响 resolver：ready plan 会成为全天权威并屏蔽 legacy routine；空 plan 必须先有 baseline，否则会制造状态空洞 (`.trellis/spec/backend/database-guidelines.md:98-113`, `server/domain/life-state-resolver.js:431-450`)。
- 直接采用 spec 的 2-6 item JSON 会改变当前 `timeline` baseline-only 形状；需要同时保留 `items`/`timeline` 读取兼容，并明确 ordinary slots 是否创建 `timeline_candidate`/proactive job (`server/application/timeline-flow.js:325-342`, `server/application/timeline-flow.js:850-861`)。
- planner 输出若覆盖 fixed/flexible slots，必须维护现有用户确认日程优先级、不可重叠约束和删除保护；`deleteGeneratedSlots()` 不删除已有 decision/event 绑定的槽位 (`server/infrastructure/timeline-repository.js:117-132`)。
- 新增跨日补偿扫描若无 persona scope、timezone 固化和 unique/job idempotency，会重复创建 plans/jobs；必须遵守 spec 的 `(persona_id, plan_date)` 复用规则和现有 job lease/settlement 语义 (`.trellis/spec/backend/persona-analysis-and-media-jobs.md:153-169`, `server/infrastructure/job-repository.js:211-247`, `server/infrastructure/job-repository.js:250-280`)。
- 仅修复 job payload 或 markReady 不能满足“自动生成安排”：当前首日 plan 的 `items` 为空，blueprint 四类事件也为空且没有消费路径；若产品要求非空人格化安排，必须另行定义 planner 的 provider、schema、失败/重试和安全边界，不能把 baseline 误称为 LLM 计划。

## Files Found

- `server/infrastructure/persona-lifecycle-repository.js`: 创建 persona、fallback blueprint、首日 ready plan/baseline slot 和初始 daily_plan job。
- `server/infrastructure/daily-plan-repository.js`: daily plan 最新行读取、timeline hydrate 和 markReady；无生成/ensure API。
- `server/application/timeline-flow.js`: daily plan slot normalization、upsert/prune、candidate job 和 job dispatch。
- `server/runtime/runtime.js`: repositories composition、default production maintenance handler 和 job dispatcher wiring。
- `server/runtime/worker-runtime.js`: durable job lease recovery、claim、poll timer、start/stop；无本地日计划调度。
- `server/infrastructure/blueprint-repository.js`: blueprint read-time v2 fallback 和 routine/fixed/flexible 字段默认值。
- `server/infrastructure/timeline-repository.js`: timeline slot persistence/pruning；只把 life-model source 列为可删除来源。
- `server/application/life-world-reader.js`: persona-scoped life inputs and daily-plan projection read.
- `server/domain/life-state-resolver.js`: ready plan、本地日期、baseline 与 routine 的读取优先级。
- `server/runtime/startup.js`: `companion_daily_plans` schema and `(persona_id, plan_date)` unique constraint。
- `.trellis/spec/backend/database-guidelines.md`: ready daily-plan state authority and day-one baseline contract。
- `.trellis/spec/backend/persona-analysis-and-media-jobs.md`: per-local-date job, `ensureDailyPlan`, planner JSON and proactive slot contract。
- `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/prd.md`: intended v2 life-model and daily timeline requirements。
- `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/design.md`: intended composer ordering and slot/decision boundaries。
- `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/implement.md`: composer and fixed/flexible normalizer work items。
- `.trellis/tasks/archive/2026-08/08-19-state-plan-consistency/prd.md`: ready plan as all-day authority and baseline fallback。
- `.trellis/tasks/08-23-persona-emergence-v2/prd.md`: shared runtime/initialization-mode boundary; no daily-plan generator。
- `.trellis/tasks/08-23-persona-emergence-v2/research/sleep-deferred-audit.md`: prior audit of daily-plan initial status/job payload; partially stale against current code.

## External References

- None. The contract and findings are based on the current repository, Trellis specs, and task artifacts.

## Related Specs

- `.trellis/spec/backend/database-guidelines.md:87-135` — ready daily-plan state authority, baseline, local-day fallback.
- `.trellis/spec/backend/persona-analysis-and-media-jobs.md:136-198` — daily-plan job/format and LLM-gated candidate behavior.
- `.trellis/spec/backend/emergence-contract.md:1-40` — LLM-first candidate boundary and no semantic fallback (relevant if planner is LLM-backed).
- `.trellis/tasks/archive/2026-08/08-18-persona-life-timeline/prd.md:17-55` — life-model event classes and daily timeline requirements.
- `.trellis/tasks/archive/2026-08/08-19-state-plan-consistency/prd.md:9-25` — plan/state/chat same-source and continuous baseline requirements.

## Caveats / Not Found

- No current planner/composer/provider call was found for daily plans, and no current `ensureDailyPlan` implementation was found. The only current daily-plan writer is persona initialization; the only current daily-plan updater is `markReady()`.
- No current production enqueue path was found for a next-local-date `daily_plan` job or a scheduled `timeline_reconcile` job. Worker polling cannot create absent jobs.
- `fixedTimeEvents`, `dailyFlexibleEvents`, `randomPositiveEvents`, and `randomNegativeEvents` exist in fallback blueprint shapes but have no current consumer that creates slots; their actual semantics remain unspecified in executable code.
- The 2026-08-23 `sleep-deferred-audit.md` accurately documents the earlier queued-row/payload bug, but current code has changed to an initial ready row and payload with `dailyPlanId`; use current source lines for implementation decisions.
- The archived 08-18 PRD marks broad acceptance items `[x]` while its implementation checklist still contains unchecked composer/normalizer work. Treat executable current code and explicit specs as stronger evidence than those stale status markers.
