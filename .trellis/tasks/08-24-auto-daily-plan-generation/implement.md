# 自动生成后续生活安排：执行计划

## 实现清单

- [x] 读取 backend database/job/timeline 规范，确认日期、幂等、ready plan 和 job settlement 约束。
- [x] 提取共享 daily-plan 默认值与时区日期函数，保持创建日 baseline 输出兼容。
- [x] 扩展 daily-plan repository：按目标日期读取、`ensure()` 事务、persona-scoped blueprint timezone。
- [x] 将默认 runtime composition 注入新的 daily-plan repository 依赖。
- [x] 改造 `daily_plan` maintenance handler：使用 dispatcher 当前时间逐日追赶、同步 slots、幂等 enqueue 下一日，并严格校验 ready 状态。
- [x] 增加跨日、重复执行、停机追赶、非 UTC/DST 日期和已有 slot 保护回归测试。
- [x] 运行目标测试、后端全量测试、typecheck/build，并检查 migration/schema 未产生漂移。

## 验证命令

- `node --test test/life-state-integration.test.mjs test/timeline-proactive-trigger.test.mjs`
- `node --test test/daily-plan-generation.test.mjs`
- `npm test`
- `npm run typecheck`
- `npm run build`

## 风险与回滚点

- 风险：共享 baseline helper 改动创建日 JSON/时间边界。回归点：`test/life-state-integration.test.mjs` 的创建日和 Asia/Shanghai 断言。
- 风险：handler 逐日追赶和 job 去重与通用 dispatcher lease 交互。回归点：重复 claim/replay、旧 job payload、worker restart。
- 回滚：先回退 handler/repository 代码；不执行数据库 destructive 操作。

## 开始实现前检查

- [x] PRD 已确认本轮只优先恢复跨日生成，不承诺新 planner 内容。
- [x] `research/daily-plan-generation-contract.md` 已读取并纳入实现上下文。
- [x] `implement.jsonl` 与 `check.jsonl` 已加入真实代码/spec 条目。
