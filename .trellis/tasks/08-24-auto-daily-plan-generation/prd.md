# 自动生成后续生活安排

## Goal

优先修复新摇光实例跨本地日未生成 daily plan、baseline 和后续 job 的生产链路，并补齐幂等回归测试。用户当前最关心的是“为什么两天没有自动安排”，因此本子任务先恢复日计划的持续生成和可审计调度，不等待详情页改版。

## Confirmed facts

- 人格创建事务只写入创建日一条 `ready` daily plan，内容是全天 `daily_plan_baseline`，并入队一次 `daily_plan` job。
- `daily-plan-repository` 只有按最新日期读取、按 ID 读取和 `markReady()`，没有 `ensure/create` 或按目标本地日期读取能力。
- runtime 的 `daily_plan` handler 只读取已有行、调用 `markReady()`、同步已有 timeline slots；worker 只消费已存在的 durable jobs，不会扫描 persona 或跨时区边界补建计划。
- 当前 blueprint 的 `fixedTimeEvents`、`dailyFlexibleEvents`、随机候选字段没有生产消费路径；不能把它们误认为已经能生成个性化安排。
- 当前代码已经满足创建日 ready plan + baseline + payload `dailyPlanId` 的契约；本子任务要修复的是后续本地日的 ensure/enqueue 链路。

## Scope boundary

- 本子任务必须让每个启用 persona 在每个本地日最多有一条 ready daily plan、至少一个连续 baseline slot，并有一个带 `dailyPlanId`/`planDate` 的幂等 `daily_plan` job。
- 已存在计划必须复用；重复 worker tick、重启、租约恢复和旧 job 重放不得重复 plan、slot 或 job。
- 计划生成失败或 worker 离线时必须留下可诊断的 queued/retryable job，不得把“没有命中更新”伪装成 ready。
- 本子任务不定义新的 LLM planner/provider，也不把 routine 或空 blueprint 擅自改写成个性化活动；非 baseline 生活安排的内容来源另由后续任务接入。创建日已有的 baseline 语义必须保持兼容。

## Requirements

### R1. 本地日期 ensure

- 提供按 `personaId + planDate` 幂等复用的 ensure 能力，`planDate` 缺省时按 blueprint timezone 从给定时钟计算。
- 目标日期不存在时，在事务内创建 ready plan、连续 `daily_plan_baseline` slot，并返回完整计划标识和状态。
- 目标日期已存在时不得覆盖已有 items/timeline、状态、事件绑定或用户确认日程。
- 查询必须 persona-scoped，不能因“读取最新计划”把旧日期计划当作当前日期计划。

### R2. durable job 链

- 每个 persona/date 至少维护一个确定性 `daily_plan` job；payload 包含 `dailyPlanId`、`personaId`、`planDate`。
- job idempotency key 必须稳定，重复 ensure、重复完成回调和服务重启不能创建重复 job。
- daily-plan job 执行时先确保目标日期计划存在，再同步 slots；成功后计算下一个本地日期并幂等 ensure/enqueue 下一日 job。
- worker 离线跨过多个本地日后恢复时，补偿路径必须补齐缺失日期，而不是只生成下一天一份。

### R3. 状态、失败与兼容

- `ready` 只表示数据库中确实存在目标日期的 ready row；`markReady()` 未命中时不能默认返回 ready。
- 计划写入、baseline slot 写入和 job 入队遵循现有事务/幂等边界，保留 migration 6/7 schema 和旧行兼容。
- 保持现有 resolver 的权威顺序：当天 ready plan 优先，旧日期计划不得覆盖 routine/baseline fallback。
- 现有创建日测试、timeline candidate 流程和 worker lease/settlement 语义不得回归。

## Acceptance Criteria

- [ ] 固定 persona timezone 和时钟，创建日为 D；运行维护 job/tick 到 D+1、D+2 后，数据库中每个本地日恰好一条 ready plan、至少一条 baseline slot 和一条对应 daily-plan job。
- [ ] 重复执行同一日 job、重复调用 ensure、重启后恢复 lease，不增加 plan、baseline slot、daily-plan job 或 timeline candidate 的数量。
- [ ] 目标日期读取按 `personaId + planDate` 生效；D 的计划不会在 D+1 被 resolver 当作当天 ready plan 使用。
- [ ] worker 跨过多个本地日后一次恢复能补齐每个缺失日期，且 DST/非 UTC timezone 的本地日期边界计算可验证。
- [ ] daily-plan job 的成功结果只在目标 row 真正 ready 且 slots 同步成功时报告 `complete/ready`；缺失 row、错误 persona 或写入失败进入可重试/有界失败路径。
- [ ] 含既有非 baseline slots、用户确认日程或 event decision 的计划在 ensure/replay 时不被覆盖或删除；后续 candidate job 保持现有幂等规则。
- [ ] 现有后端全量测试和新增 daily-plan 跨日回归测试通过。

## Non-goals

- 本子任务不实现新的 LLM daily-plan planner，不从自然语言或 routine 猜测个性化活动。
- 本子任务不修改 PersonaDetail 的“最近安排”展示；父任务后续会把 daily-plan/timeline 与显式 schedule 合并呈现。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
