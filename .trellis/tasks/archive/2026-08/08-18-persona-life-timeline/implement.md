# 人格生活模型与事件时间线：实施计划

## 实施原则

- 先建立 v2 安全生活模型与 validator，再接入 LLM；任何阶段都保留可运行 fallback。
- 先增加编排层和幂等记录，再替换随机生活变体；不要继续向 `maybeCreateLifeVariation()` 追加模板分支。
- 先让聊天成为 overlay，再实现睡眠延迟批次；两个机制都必须保持现有聊天兼容性。
- 每个阶段完成对应测试和迁移验证后再进入下一阶段。

## 阶段 0：规格与基线

- [ ] 复核 `.trellis/spec/backend`、`.trellis/spec/frontend` 和跨层思考指南。
- [ ] 记录现有事件、日计划、状态投影、聊天和 job 的基线测试结果。
- [ ] 确认 migration 当前最高版本为 v6，所有新增表从 v7 开始。
- [ ] 为初始化 prompt、life model schema、事件类型、决策状态和睡眠 batch 建立版本常量。

验证：`npm test`；数据库迁移测试；现有 `test/companion-api.test.mjs` 全量通过。

## 阶段 1：Life Model v2 与默认世界

- [ ] 扩展 `buildInitialBlueprint()` 生成 deterministic-safe v2 fallback。
- [ ] 新增 `world.locations/rooms/defaultSceneRef`，为旧人格生成稳定默认房间。
- [ ] 定义 fixed、flexible、random positive、random negative 四类模板 normalizer。
- [ ] 新增输入 canonicalizer，将 identity locks 与可推演资料分离。
- [ ] 新增严格 JSON extraction、shape/reference/time/risk/size validator。
- [ ] 新增 blueprint revision 表和 current projection 写入逻辑。
- [ ] 将 `publicBlueprint()` 仅暴露非敏感生成元数据，不暴露 raw prompt/raw output。
- [ ] 将默认场景解析接入 `scheduledState()`、`resolvedStateFor()`、`contextFor()` 和媒体上下文。

风险点：旧 blueprint 形状、旧前端字段、默认时区和历史 persona 删除逻辑。

验证：schema 单测、旧 blueprint fallback 单测、迁移升级/降级数据保护测试、现有人格 API 回归。

## 阶段 2：同步 LLM 初始化

- [ ] 在 `activateInterview()` 的 preview 与 `createPersona()` 之间接入同步 `lmCompletion(stream=false)`。
- [ ] 增加低 temperature、超时、非 2xx、空响应和多 JSON 结果处理。
- [ ] 设计并版本化 life model generation system prompt / user prompt。
- [ ] 通过 validator 后 merge generated model；任一失败全量回退 deterministic baseline。
- [ ] 记录 source/model/prompt/schema/fallback/warnings/input hash 等审计信息。
- [ ] 初始化 supporting characters 的稳定 profile、provenance 与数量限制。
- [ ] 确保 createPersona、initial state、default room、supporting cast、first daily plan 使用同一最终 blueprint。

验证：LLM 成功、超时、错误 JSON、违反 identity lock、风险输出、冲突输出和 fallback 创建测试。

## 阶段 3：Timeline slots 与 event decisions

- [ ] 新增 `companion_timeline_slots`、`companion_event_decisions`、`companion_event_links` migration。
- [ ] 实现 slot 状态机和稳定 `slot_id`，避免依赖可被 daily plan 删除重插的 schedule id。
- [ ] 实现 decision 状态机、`decision_key` 幂等约束和 rationale/candidate 审计。
- [ ] 实现 daily timeline composer：确认日程 → fixed → flexible → free windows → opportunities。
- [ ] 将 `companion_daily_plans.plan_json` 扩展为完整 timeline snapshot，并继续投影兼容 `companion_schedule_items`。
- [ ] 支持 `no_event`、冲突裁决、优先级和 `none/overlay/replace/block`。
- [ ] 修改 `scheduledState()` 读取 active event/slot 的显式优先级，而不是按最新写入时间偶然覆盖。
- [ ] 修正 `reconcilePersona()`，避免 active event 被每次 tick 复制成新的事实事件。

验证：固定/弹性冲突、explicit schedule 优先级、slot 重建、重复 tick、服务重启和事件结束回落测试。

## 阶段 4：人格驱动机会事件

- [ ] 用 `maybeInstantiateRandomLifeEvent()` 替代固定三模板逻辑。
- [ ] 从 life model 候选池读取 positive/negative templates。
- [ ] 实现 cooldown、daily/weekly budget、focus、scene compatibility、existing cast reuse、novelty ledger。
- [ ] 将 `no_event` 纳入候选，并允许“今天什么特别事件都没有”。
- [ ] 实现 conditional child event / parent event / recovery event / causal links。
- [ ] 在 `createEvent()` 前统一调用 `assertEventInstanceAllowed()`，收紧 debug simulate 复用同一守卫。
- [ ] 保留 visual/proactive/activity 副作用，但改为事件投影策略独立决定。

验证：正向适配、负向 mild/reversible、重复惩罚、冷却、无候选、父事件插入、恢复和副作用关联测试。

## 阶段 5：Supporting characters 与生活后果

- [ ] 初始化时写入稳定角色候选，保留 user/generated provenance。
- [ ] 首次事件引入角色时写 introduced event；后续事件复用角色 ID。
- [ ] 增加关系发展证据和速度限制，禁止偶遇直接高亲密升级。
- [ ] 将 event links 与 relationship evolution 对接；关系变化不覆盖基础人格。
- [ ] Debug Inspector 展示角色首次出现、最近证据和关系变化原因。

验证：角色稳定 ID、重复复用、首次介绍、关系升级阈值、人格隔离和删除级联测试。

## 阶段 6：跨场景聊天 overlay

- [ ] 扩展 `contextFor()` / `userVisibleChatPrompt()`，加入当前事件、地点、注意力、interactionAvailability 和可中断性。
- [ ] 在聊天入口读取当前 active event/slot，但不允许模型直接改变时间线。
- [ ] 为短聊天、长聊天、固定事件、弹性事件和恢复状态实现 overlay/attention cost。
- [ ] 长聊天达到阈值时，只调整 flexible slots；固定事件和用户确认日程保持不变。
- [ ] 将明确的用户计划继续通过现有 explicit plan/schedule contract 写入 timeline。
- [ ] 保证聊天消息流、媒体 intent、关系演化和事件投影兼容。

验证：上课/吃饭/逛街/休息/固定日程下聊天；短聊不改 timeline；长聊平移 flexible；固定承诺不被覆盖。

## 阶段 7：睡眠决策与延迟回复批次

- [ ] 新增 `companion_chat_deferred_batches` migration 与状态机。
- [ ] 定义 intimacy band、message importance、sleep phase、random draw、wake window 的纯函数决策输入。
- [ ] 亲密度只提高即时回复概率；每个区间保留 deferred 结果。
- [ ] 首条消息决定本批次 immediate/deferred；deferred 后后续消息只能 append，不得打断或改写。
- [ ] 计算内部 `deliverAt`，不向用户显示睡眠占位、预计时间或系统状态。
- [ ] 到 `deliverAt` 后合并批次消息，生成并持久化一条普通 assistant message。
- [ ] 使用同一 batch key 保证重启、重复 tick、lease retry 不重复回复。
- [ ] 投递失败只重试同一 batch；成功后下一条新消息才开启新决策。

验证：不同亲密度随机分布、固定随机种子、睡眠延迟、多消息合并、不可打断、deliverAt、服务重启、重复投递和自然醒后回复测试。

## 阶段 8：前端与 Debug Inspector

- [ ] 普通聊天继续只显示自然 assistant 消息，不展示 deferred batch、概率、评分和内部原因。
- [ ] 当前聊天和人格详情显示结构化地点/房间的自然场景文本。
- [ ] 动态页保持事件投影语义，不展示全部内部事件流水账。
- [ ] Debug Inspector 展示今日 timeline、active event、候选、decision、因果、压制原因、睡眠 batch 和作业。
- [ ] 增加手动模拟 timeline slot、positive/negative candidate、sleep decision 的本地调试入口，并复用生产 validator。
- [ ] 后台 polling 只刷新持久化投影，不重复触发决策。

验证：桌面/移动聊天、动态、场景展示、Debug 开关、敏感字段脱敏和轮询不重复行为测试。

## 阶段 9：数据迁移、文档、质量门禁

- [ ] 将 persona 删除事务扩展到所有新增表。
- [ ] 为旧 persona 生成 v2 fallback blueprint，不重写历史事件和消息。
- [ ] 增加 retention/索引策略，确保 timeline/decision/link/batch 查询可控。
- [ ] 更新 README、backend/frontend specs、事件模型和调试接口说明。
- [ ] 完成全量单元、集成、迁移、前端行为、LLM fallback、安全和幂等测试。
- [ ] 运行 Trellis check，修复跨层数据流、文档同步和质量问题。
- [ ] 用户审阅 `prd.md`、`design.md`、`implement.md` 后，才执行 `task.py start`。

## 风险与回滚

- 任一新编排功能可以通过 feature flag 回退到 v2 deterministic timeline，而不是恢复旧的固定三模板。
- LLM 初始化失败只回退 baseline，不影响人格创建。
- 新表均通过 migration 增加；部署前备份 SQLite，迁移失败不删除旧表或旧数据。
- 现有 `companion_life_events` 写入路径保持兼容，新增编排元数据采用可选字段。
- 睡眠延迟批次失败时回退为持久化用户消息，不自动伪造 assistant 回复，也不改变原生活事件。

## 关键验证命令

```bash
npm test
node --test test/companion-api.test.mjs
git diff --check
python3 ./.trellis/scripts/get_context.py --mode packages
```
