# Goal 评估与规划频率修复（2026-10-09）

本轮修复在本地代码完成；线上实例只读诊断，没有部署、修改目标或Provider设置。实现提交为 `bc90e52`，基线为 `45afa7a`，实际本地提交分支 `master`。临时凭据不保存到仓库，诊断原始Prompt、私聊及Actor内容不复制到报告。

## 已核实原因

- 重复 skip 不产生 `provider_request_failed`。成功语义 memo 命中后返回 `succeeded / assessment_memo_match`，不发物理模型请求。
- 线上一个 Goal 的领域错误为 `goal_assessment_coverage_missing`：模型有返回，未覆盖本次要求的目标，因此拒绝结果、没有成功 memo，随后再次尝试。同一 evaluation 请求有多次真实 model run。这与重复 skip 是两条路径。
- 线上实例活动集合 5/5，Planner 同一个 run 多次模型回答，领域最终 `retry_exhausted`。Agent 的 completed 仅表示原生模型轮结束，不代表Goal领域提交成功。
- 另一条 WakeUp 原始错误明确为 HTTP 507 / insufficient memory：模型权重约15.7 GiB，可用约4.2 GiB，不足以处理该次11957 prompt tokens。不能据此把所有GoalEvaluation失败也归因为507；该GoalEvaluation的归一化失败码可见，但工作流历史页面读取失败，确切HTTP原文未取得。

## 具体实现

1. 新版本 `0055_goal_planner_cadence` 前向替换调度函数，不改写已交付0054。普通cognition/reflection愿望仍持久保存，但不独立调Planner。生命周期释放名额、初始启动、正式每日日程、授权重新开启/恢复是主要自动触发，自动补充要求容量未满。
2. Planner claim 重复检查上述条件，拦住旧二进制留下的满额队列。Owner明确满额请求只提供suggestions。真实Actor/人格变化保留可行性复核例外，避免背景门禁永久卡住或继续按旧条件执行。
3. 正式 `acceptScheduleWithTx` 在原接受事务中以 `schedule:<IANA timezone>:<local date>` 发每天一次规划事件。幂等重放和同日重排不重复。
4. 旧queued run可返回capacity_full/awaiting_trigger，不伪装模型“无候选”。repair在无空位时不持续制造新启动事件。晚到/私有事件保留；旧Worker崩溃留下的满额背景复核直接恢复原事件，不转成受容量阻挡的普通startup。
5. GoalEvaluation缺覆盖时，在当前typed task通过原正式Runner提供一次明确替换对象反馈。持续缺覆盖或列举的固定wire/scope错误结算failed，保留具体原因，不写成功memo、不消费证据、不改变完成状态。临时Provider故障仍有界退避；已failed请求重放不再叫模型。
6. Planner直接最终回答却未消费真实query结果、虚报未落盘提交等固定合同错误，同样保留精确失败并终止当前run。新的合法事实/Owner触发可以建立新工作，不能把失败当作成功缓存。

## 实際测试证据

以下日志位于 `docs/verification/goal-planner/20261009/`。测试使用本轮专属loopback PostgreSQL与随机数据库、脚本Provider，未调用线上Provider。

| 检查 | 实际结果 |
|---|---|
| 三项新增回归对真实基线45afa7a | 均FAIL，精确证明原缺覆盖无修正/继续retry，以及无query的Planner继续retry；`cadence-red-baseline.log` |
| 目标/日程/原完成链扩大回归 | 183 PASS事件，167叶测试PASS，0SKIP/FAIL；`cadence-core.jsonl` |
| migrations全包 | 87 PASS事件，81叶测试PASS，0SKIP/FAIL；`cadence-migrations.jsonl` |
| 改进的真实0054函数恢复→0055升级及最后定向复测 | 8 PASS，0SKIP/FAIL；`cadence-last-targets.jsonl` |
| 独立定向核验及恢复漏洞修复验证 | 26/4/1 PASS事件；对应cadence-review*.jsonl。未收到整体验收终态，属于定向核验，不声称完整独立签字 |
| 最后代码的主线程race回归 | 50 PASS事件，47叶测试PASS，0SKIP/FAIL；`cadence-race.jsonl` |
| Go vet/build/workflow测试与diff | PASS，`cadence-workflow.log`及终端检查 |

等效复跑命令（数据库必须是可CREATE/DROP随机库的隔离测试库）：

```bash
export GO_CORE_TEST_DATABASE_URL='postgres://TEST_ROLE:TEST_PASSWORD@127.0.0.1:TEST_PORT/postgres?sslmode=disable'
export GOCACHE=/private/tmp/fluctlight-go-cache
go -C apps/core-go test ./internal/core -run '^(TestGoal|TestAcceptSchedule|TestSchedule|TestFormalConversationWithoutIntentionCompletesOriginalExpressionGoal|TestScheduledAcquisitionGoalClockIndependentWearAndFinalWorkflow|TestProviderRunErrorCode)' -count=1 -json
go -C apps/core-go test ./internal/migrations -count=1 -json
go -C apps/core-go test -race ./internal/core -run '^(TestGoalPlanner|TestGoalAssessmentMemo|TestGoalAssessmentFailure|TestGoalAssessmentMissing|TestGoalAssessmentRepeated|TestAcceptScheduleUsesDatabaseIdempotencyBoundary)' -count=1 -json
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
go -C apps/core-go test ./internal/workflow -count=1
git diff --check
```

## 行为、开销与限制

满五自动补充、普通wish重复、满额repair、终态失败重放的测试物理模型请求数为0；Owner明确满额建议仍可实际query→结果→最终建议。缺覆盖输出最多增加一次有明确反馈的修正，不再使用五次无反馈的领域盲重试。Provider暂时失败仍有限重试，因此降频不等于消除507。

成功语义memo的原去重继续保留，没有把失败写成成功以降低次数。工作流创建与memo检查仍是两个阶段：本轮没有重构所有Evaluation入队路径，可能仍存在不调用模型的skip工作流。没有把所有普通消息按文本关键词判定“无关”，相关证据识别仍遵守原语义/事件权威。

没有部署到线上；0055只在隔离库实跑；修复后的真实Provider语义/工具调用质量未验收。模型本身的内存容量、已存在失败run历史和原GP60矩阵未完成项继续保留。提交代码不能自动修复线上模型内存，亦不静默重开旧终态请求。

测试临时资源已回收：仅删除本轮专属 `fluctlight-cadence-pg-1009` 容器，没有修改用户或线上容器。
