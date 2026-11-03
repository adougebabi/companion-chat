# 人格状态与当天计划一致性：技术设计

## 问题边界

当前系统有三套可能彼此分叉的事实：

```text
companion_daily_plans.plan_json
  → companion_schedule_items
  → scheduledState() / stateShape() / chat prompt
```
当日计划仅保存离散活动块。计划首块尚未开始或两个活动之间存在空隙时，`scheduledState()` 会落到旧的 `routine`，大学生人格因而会被错误投影为“上课中”。聊天层只收到文本状态，未收到可信的结束时间，因此会把错误状态进一步扩写为虚构的课程安排和下课时间。

## 设计原则

1. `daily_plan.status = ready` 后，日计划是全天状态的唯一权威。
2. 服务端而非 LLM 负责补齐计划空隙；模型只提供显式活动块。
3. 时间线任何时刻只能有一个主状态来源：明确事件 > 明确用户日程 > 当前 daily-plan slot > 日计划基线 slot > legacy routine。
4. 聊天只能引用状态投影中明确给出的 `endsAt`；缺失时禁止输出具体结束时间。
5. 旧人格的当前 blueprint 在读取时要获得兼容的 v2 fallback，不能因缺少新字段退回到大学生通用作息。

## 连续日计划投影

日计划生成后保留 LLM 返回的显式活动块，并在服务端生成两类补充 slot：

| 时段 | slot 类型 | 生成规则 |
| --- | --- | --- |
| 当天开始到首个活动 | `baseline_sleep` / `baseline_idle` | 若首块语义含“睡、赖床、自然醒、起床”，则为睡眠/起床前；否则默认房间空闲 |
| 两个活动块之间 | `baseline_idle` / `baseline_transition` | 默认房间/合理回归地点；不使用旧 routine |
| 最后活动到当天结束 | `baseline_idle` / `baseline_rest` | 默认房间休息；夜间使用休息状态 |

每个补充 slot 都写入 `companion_timeline_slots`，并以 `details_json`/slot metadata 投影为兼容 `companion_schedule_items` 或直接由状态解析读取。计划 JSON 应保存完整连续时间线，便于调试和重建。

## 状态来源解析

`scheduledState(persona, at)` 改为：

```text
active explicit/exception life event (遵守 preemption)
  > active user-confirmed schedule
  > active daily-plan explicit slot
  > active daily-plan baseline slot
  > legacy routine only if daily plan unavailable
```

状态对象统一增加：

```js
{
  situation,
  scene,
  location,
  room,
  source,
  sourceId,
  startsAt,
  endsAt,
  timeFact: 'known' | 'unknown'
}
```

旧人格的 `blueprint()` 读取路径应补齐 v2 world/default room，但不能写回历史事实；是否持久化 migration/backfill 由单独安全写入路径处理。

## 聊天事实契约

聊天 prompt 中新增一段应用自有状态事实：

```text
当前主状态来源：<source>
可信结束时间：<ISO 或 无>
下一可信时间边界：<ISO 或 无>
```

固定系统规则：

- 只有 `timeFact=known` 才能对用户说具体结束时间。
- `timeFact=unknown` 时不得根据“学生”“上课”等身份猜测课程、时长或下课时刻。
- 不得把计划外 baseline 状态叙述为课程、工作或已确认活动。

## 兼容与迁移

- 当前已有的 `ai_daily_plan` schedule 继续保留 API 兼容形状。
- `companion_daily_plans.plan_json` 从离散 items 扩展为包含显式活动和 baseline slots 的完整 timeline，但旧数组读取仍兼容。
- 重建同一 local day 时先按本地时区 day bounds 删除该天旧的 AI/baseline projection，再原子插入完整投影。
- 不能删除用户确认日程、已发生 life event、聊天消息或媒体记录。

## 回归案例

固定 `Asia/Shanghai` 日期和时间：

```text
计划：10:00–13:00 睡醒后在宿舍打游戏/追番
查询：08:47
期望：睡眠/赖床或默认房间基线；不是上课
聊天：询问“什么时候下课”
期望：不得回答“10:30”；应说明目前没有课程/精确下课时间
```
