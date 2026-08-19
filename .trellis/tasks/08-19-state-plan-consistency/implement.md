# 人格状态与当天计划一致性：实施计划

## 1. 建立红色回归测试

- [x] 为 08:47 / 10:00–13:00 睡醒游戏案例写入纯函数与状态投影测试。
- [x] 断言 ready daily plan 存在时，legacy routine 不会产生“上课中”。
- [x] 断言 state shape/chat context 提供可信 `endsAt`，且未知时为 `null`/`unknown`。
- [x] 为聊天时间事实规则添加 prompt/行为测试，禁止无事实的“10:30”。

## 2. 连续计划投影

- [x] 规范 daily plan item 语义，区分显式活动与服务端 baseline。
- [x] 实现按人格时区补齐 00:00–24:00 空隙的 slot composer。
- [x] 将 baseline slots 写入 timeline，并保证重建按本地 day bounds 幂等。
- [x] 维持兼容 `companion_schedule_items` 的当前活动投影。

## 3. 状态解析与旧人格兼容

- [x] 为旧 blueprint 添加 read-time effective v2 fallback。
- [x] 修改 `scheduledState()`，在 ready daily plan 存在时优先使用连续 slot，不使用 legacy routine。
- [x] 返回 state source、边界时间和场景/地点/房间的一致 projection。
- [x] 不让 reconcile 持久化过期/低优先级 legacy routine 状态。

## 4. 聊天时间事实边界

- [x] 扩展 life-state prompt 层，注入可信时序字段。
- [x] 添加应用自有“禁止编造结束时间”契约，置于人格/用户文本之后。
- [x] 验证状态、媒体上下文和聊天 prompt 的来源一致。

## 5. 验证与收尾

- [x] 运行 `npm test`、`node --check server.js`、`node --check src/companion-main.js`、`git diff --check`。
- [x] 对本地 SQLite fixture 重演苏芷柠案例。
- [x] 更新 backend/frontend specs 与 README 的连续计划 / 时间事实契约。
- [x] 运行 Trellis check，审查 dirty worktree 是否包含其他任务的改动，单独提交本任务文件。

## 风险与回滚

- 计划补齐只新增 AI/baseline projection，不触碰用户确认日程或历史事件。
- 若连续投影失败，ready daily plan 本身仍可被读取；状态降级为默认房间基线，不能回退到学生上课 routine。
- 任何 LLM 时间输出只作为文本建议，不获得状态/日程写入权限。
