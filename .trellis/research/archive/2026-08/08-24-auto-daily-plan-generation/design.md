# 自动生成后续生活安排：技术设计

## 目标与边界

本设计只修复 daily plan 的生命周期和调度连续性：每个 persona 的每个本地日都能按需创建/复用一份 ready plan、连续 baseline slot 和一个可审计的 `daily_plan` durable job。它不实现新的 LLM planner，也不从空的 `routine` 或生活模型字段猜测个性化活动。

## 组件边界

- `server/domain/daily-plan-defaults.js`：集中本地日期、时区时间和 baseline plan 的纯函数，供 persona 创建和后续日计划复用，避免两套边界算法漂移。
- `server/infrastructure/daily-plan-repository.js`：增加按 `personaId + planDate` 的精确读取和 `ensure()` 持久化端口；保留旧的 latest/readById 兼容别名。
- `server/runtime/runtime.js`：`daily_plan` handler 负责从 job 的起始日期追赶到当前 persona 本地日期，逐日 ensure、同步 timeline，并为下一日幂等入队 job。
- `server/runtime/worker-runtime.js`：不改变 claim/lease 语义；恢复后由尚未完成的 daily-plan job 触发追赶。
- `server/infrastructure/job-repository.js`：复用 `findByPayload()` 做 persona/date/jobType 去重；不新增 migration。

## 数据流

```text
persona 创建日 D
  -> ready plan + baseline slot(D) + daily_plan job(D)

worker claim job(D) at now
  -> 读取 blueprint timezone
  -> 计算 [payload.planDate .. localDate(now)]
  -> 对每个日期 ensure plan/date + baseline slot
  -> timelineFlow.syncDailyPlanSlots()
  -> findByPayload(personaId, jobType, $.planDate)
  -> 缺失时 enqueue daily_plan(next date)
  -> 当前 job complete
```

`ensure()` 使用 `(persona_id, plan_date)` 唯一约束作为最终幂等边界，事务内先查再插入；已存在计划不覆盖 `plan_json`，也不删除与用户 schedule/event decision 绑定的 timeline slot。job 去重使用稳定 payload 查询，竞态下以唯一 job id 冲突后的重新读取收敛。

## 日期与追赶

- 计划日期来自 blueprint timezone；`at` 缺省时保持原有 latest-read 兼容行为，life-world reader 传入时间时按 persona timezone 精确读取当天计划。
- handler 使用 dispatcher context 的 `now`，不能使用 job 的旧 `updated_at` 作为当前时间。
- 追赶跨度设置有界（最多 366 个本地日）；超过上限返回 retryable 结果，避免停机多年后单次 worker tick 无界写入。
- 每个已确保日期都同步 slots；baseline 不创建 `timeline_candidate`，非 baseline 内容沿用现有 timeline-flow 幂等逻辑。

## 失败与兼容

- 缺失 persona/blueprint/目标日期或写入失败抛出 bounded error，由通用 dispatcher 进入 retry/terminal settlement。
- handler 只有在目标日期 plan 实际为 `ready` 且 slots 同步成功后才返回 complete；不再用 `plan?.status ?? 'ready'` 伪造状态。
- 自定义 composition 中没有 `ensure()` 的旧 daily-plan repository 保留原处理分支，避免破坏现有测试和外部注入；默认生产 composition 使用新端口。
- 不改现有 migration、job schema 或 resolver precedence；新日没有 ready plan 时仍可回退 legacy routine/baseline。

## 回滚形状

代码回滚不需要数据迁移；新增的 daily plan/baseline/job 行仍符合既有 schema。若新 handler 禁用，可回到旧 handler 消费已有 job，但不会删除已生成的未来日计划。
