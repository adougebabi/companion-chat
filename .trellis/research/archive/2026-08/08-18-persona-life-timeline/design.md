# 人格生活模型与事件时间线：技术设计

## 1. 设计目标与边界

本设计把现有“生活事件事实 + 状态投影 + 动态/作业副作用”扩展为完整的生活模拟系统，但保留现有 API 和主要存储兼容性。

核心边界：

```text
稳定人格事实 / 用户明确设定
        ↓
可版本化 life model（LLM 生成，服务端校验）
        ↓
当天 timeline slots（固定、弹性、候选、空闲）
        ↓
orchestration decisions（采纳、压制、跳过、延迟、替换）
        ↓
life event facts（已发生事件）
        ↓
状态投影 / 动态 / 媒体 / 主动消息 / 关系结果
```

LLM 只生成生活模型和受限计划建议，不直接执行事件；服务端 validator、调度器和安全策略拥有最终权限。

## 2. 与现有代码的边界

### 保留的现有职责

| 对象 | 保留职责 |
|---|---|
| `companion_persona_life_blueprints` | 当前人格的 life model projection，继续供 `blueprint()` / prompt 层读取 |
| `companion_persona_foundation_revisions` | 不可变基础人格版本；生活模型不得回写 foundation |
| `companion_daily_plans` | 每日计划的日期级审计和状态 |
| `companion_schedule_items` | 用户确认日程及当前可显示的计划实例，继续兼容 `scheduledState()` |
| `companion_life_events` | 已发生的不可变生活事实；动态、状态、主动消息继续关联它 |
| `companion_persona_states` | 当前状态缓存/检查点，不成为唯一事实来源 |
| `companion_jobs` | LLM、媒体、主动消息、关系演化等耐久异步作业 |

### 新增职责

建议追加 migration v7，引入：

1. `companion_persona_life_blueprint_revisions`：生活模型版本审计。
2. `companion_timeline_slots`：某日时间线槽位，区分固定、弹性、候选、空闲和结果。
3. `companion_event_decisions`：编排器选择的幂等、可审计决策。
4. `companion_event_links`：事件间因果、替换、恢复、后续关系等多对多关联。
5. `companion_chat_deferred_batches`：睡眠或低可用场景下的合并延迟回复批次。

如果迁移风险需要分批，前 3 张表为第一批必需；`event_links` 和 deferred batches 仍必须在同一完整任务内交付，不允许以“以后再做”替代需求。

## 3. Life Model v2

`blueprint_json` 扩展为兼容旧字段的 v2 文档：

```json
{
  "schemaVersion": 2,
  "timezone": "Asia/Shanghai",
  "world": {
    "defaultSceneRef": {"locationId": "home", "roomId": "private_room"},
    "locations": [
      {
        "id": "home",
        "kind": "home",
        "name": "住处",
        "isDefault": true,
        "rooms": [
          {"id": "private_room", "kind": "private_room", "name": "自己的房间", "scene": "...", "activityTags": ["rest", "study", "chat"]}
        ]
      }
    ]
  },
  "fixedTimeEvents": [],
  "dailyFlexibleEvents": [],
  "randomPositiveEvents": [],
  "randomNegativeEvents": [],
  "supportingCastPolicy": {},
  "eventPolicy": {},
  "attentionBudget": {},
  "generation": {}
}
```

旧 `routine`、`interests`、`eventPolicy`、`attentionBudget`、`supportingCast` 保持可读；旧人格通过 deterministic-safe fallback 迁移到 v2 默认 world 和四类事件。

### 3.1 四类模板契约

每个模板至少包含：

- `templateId`、`family`、`title`、`situation`；
- `timeMode`（fixed / flexible / opportunity / conditional）；
- 时间窗口和 duration 范围；
- `sceneRefs`、参与者候选、前置条件；
- `priority`、`preemptionMode`、`skippable`；
- cooldown、daily/weekly budget；
- `riskLevel`、`reversible`、effects、recovery；
- `provenance`（user / generated / fallback）。

随机负向模板必须满足 `riskLevel=mild`、`reversible=true`、存在 recovery，并通过 hard-block taxonomy 校验。

### 3.2 生成与校验

接入点为 `activateInterview()` 的 `previewInterviewAnswers()` 与 `createPersona()` 之间：

```text
normalize answers
  → buildInitialBlueprint() 生成安全 baseline
  → buildPersonaLifeGenerationInput()（锁定事实与可推演提示分离）
  → lmCompletion(stream=false, low temperature, timeout)
  → strict JSON extraction
  → normalizeGeneratedLifeModel()
  → validate shape / references / time conflicts / risk / size
  → merge into baseline or use full fallback
  → createPersona(final blueprint)
```

任何解析、引用、时间、安全或规模校验失败都使用完整 fallback，不做部分信任的半修复。

生活模型生成 prompt 必须版本化；`identityLocks` 只读，禁止生成器修改 foundation、characterCard、用户关系边界和明确 supporting cast 事实。

## 4. 当日时间线与编排器

### 4.1 Timeline slot

`companion_timeline_slots` 建议字段：

```text
id, persona_id, plan_date, slot_key, slot_kind,
starts_at, ends_at, status, source, priority,
schedule_id, plan_revision, constraints_json, outcome_json,
created_at, updated_at
```

slot 状态：

```text
proposed → confirmed → active → completed
                         ├→ skipped
                         ├→ replaced
                         └→ cancelled
```

`slot_id` 是稳定因果锚点；不能依赖 `runDailyPlanJob()` 删除重插后的 `schedule_id`。

### 4.2 Event decision

`companion_event_decisions` 建议字段：

```text
id, persona_id, slot_id, decision_key, decision_type,
status, run_at, expires_at, priority, preemption_mode,
candidate_json, rationale_json, event_id, job_id,
created_at, updated_at
```

通过 `(persona_id, decision_key)` 唯一约束保证重复 tick、服务重启和 job retry 幂等。

decision 状态：

```text
proposed → accepted → executed
          ├→ suppressed
          ├→ expired
          ├→ cancelled
          └→ failed (可重试)
```

候选生成必须把 `no_event` 作为候选，不能为了填充动态而强行生成事件。选择分数至少考虑 persona fit、因果延续、slot 兼容、关系关联、新颖度、重复惩罚、冲突、注意力成本和负向风险。

### 4.3 编排顺序

每天：

1. 固化时区和本地日期。
2. 放置用户明确确认的高优先级日程。
3. 放置固定时间事件。
4. 在剩余间隔中放置每日弹性事件。
5. 建立空闲窗口并解析默认房间。
6. 在候选窗口生成正/负机会事件，允许 `no_event`。
7. 持久化 daily plan、slots 和候选决策。

运行时：

1. 事件边界、用户消息、日程变更、恢复条件或周期 tick 触发评估。
2. 读取 active event、active slot、明确日程和当前世界位置。
3. 先结束到期事件，再按优先级选择下一状态。
4. 执行 decision → 原子创建 life event → 更新 slot outcome → 投影状态。
5. 按 visibility policy 独立决定动态、媒体、主动消息和记忆。

状态优先级：

```text
safety/recovery > explicit commitment > active exception
> fixed time > flexible slot > routine/default room
```

`preemptionMode` 支持 `none`、`overlay`、`replace`、`block`。事件结束后读时回落到仍有效的下层状态，不通过复制事件制造“恢复事件”。

## 5. 跨场景聊天与睡眠延迟批次

### 5.1 Overlay interaction

聊天不再被视为只能在空闲时段发生的生活事件，而是当前生活事件上的 overlay：

- 读取当前地点、场景、事件、注意力和交互可用性；
- 短聊天不替换当前事件；
- 长聊天按注意力/时长阈值推迟或压缩 flexible slot；
- 不覆盖固定事件、用户确认日程和安全/恢复状态；
- 聊天只有在形成明确计划或明确生活影响时，才经显式契约进入 timeline。

### 5.2 睡眠决策

睡眠收到第一条消息时创建一个不可打断的 decision/batch：

```text
sleeping + first user message
  → intimacy / message importance / sleep phase / random draw
  → immediate reply OR deferred batch
```

亲密度只提高立即回复概率，任何亲密度都保留继续睡眠的结果。

如果选择 deferred：

- 用户消息正常存储，不显示睡眠占位、预计时间或系统提示；
- 创建 `companion_chat_deferred_batches`，记录 `batch_key`、`deliver_at`、`status`、随机决策摘要和首条消息；
- `deliver_at` 前后续消息只追加到同一批次，不能打断、唤醒或改写该批次决定；
- 到达 `deliver_at` 后合并理解所有消息，只投递一条普通助手消息；
- 重试复用同一 batch，不生成重复消息；
- 批次成功后，下一条新消息才进入新的睡眠互动决策。

`deliver_at` 应由当前睡眠窗口、自然醒时间、随机偏移、关系上下文和消息批次生成，不向用户暴露概率或内部评分。

## 6. Supporting characters

初始化 LLM 可生成少量稳定角色候选，但不得让 runtime 每次随机事件临时创造人物。

- 继续使用 `companion_supporting_characters`，增加 provenance/首次出现/稳定 profile 所需字段或在 `profile_json` 中规范化。
- 事件首次引入人物时记录 `introduced_event_id`。
- 关系发展必须关联证据事件或用户互动，不允许单次偶遇直接升级为高亲密关系。
- 新人物有数量、出现频率和冷却策略；已有角色优先复用。

## 7. 事件投影与异步作业

事件事实、通知意图和实际投递解耦：

```text
life event recorded
  ├─ persona state projection
  ├─ optional activity projection
  ├─ optional media job
  ├─ optional proactive delivery intent
  └─ optional memory / relationship effect
```

现有 `companion_jobs` 继续使用 queued → leased → complete/failed lease 模式。主动消息建议扩展 delivery intent/result，记录 delivered/skipped/expired 及原因。

## 8. 版本、编辑与迁移

- 追加 migration v7，禁止修改 v1–v6。
- 当前 blueprint 作为 current projection；新表保存 blueprint revisions。
- 用户明确修改身份、职业、居住地、兴趣、作息或关系等影响生活的字段时生成新 life model revision。
- 语气、外观文案和互动边界修改不自动重建整套生活模型。
- 历史事件、历史消息和已经发生的事实不回写。
- 当前固定日程不被静默删除；新模型只影响未来 timeline 和未来候选，采用渐进切换。
- 删除人格时，新表必须纳入现有删除事务。

## 9. 可观测性与安全

Debug Inspector 展示：当前状态、今日 slots、active event、候选、decision 状态、压制/跳过原因、因果链、睡眠 batch、作业状态。

普通用户只看到当前状态、场景、动态和自然聊天，不看到概率、评分、策略名或内部调度原因。

安全守卫必须位于模型输出、计划落库和 `createEvent()` 实例化前，禁止不可逆风险、重大医疗/财务/关系变化、现实义务和情感勒索。
