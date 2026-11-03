# F15 调度与历史定位

基线ec1ed82，三名只读探子已完成；主代理亲读Goal Planner/Goal-Life契约及关键调度函数。以下为定位抓手，不替代实施者阅读将修改的原文。以goal-cadence-followup.md和用户最新要求为准，旧契约的即时review/full suggestions例外已被覆盖。

## Evaluation

- core/goal_sources.go:21 queueGoalEvaluationTx：owner+profile pending合并；:35 available_at=now()+2s；:40 intent同样2s。owner_reassess合并forced ids。SQL source trigger可直接入队，不经过此helper。
- core/goal_evaluation_runtime.go:46 claimGoalEvaluation：Life锁+request FOR UPDATE；:91只有retry检查available，pending可绕过；:112 frozen native retry直接重新claim，必须共用新fence。claim_revision、5min lease、attempt上限保留。
- 同文件:326 ProcessGoalEvaluationIntent：logical lease→sources reconcile→claim→projection→memo→Kev→frozen refs→真实RunGoalEvaluationTask→finalize。30min应作用于独立cycle准入，同一native loop内部多轮不是多次cycle。
- 同文件:482 failGoalEvaluation：原失败1min retry；workflow dispatcher也能1min重新运行，不能依赖queue delay或Kev限制频率。
- core/goal_assessment_memo.go:268逐Goal语义memo；forced/review/新证据使eligible，无最小周期。保留相同事实无模型优化，不能用时钟造事实。
- core/goal_evaluation_submission_runtime.go:113 stale replacement；:216/241 remainder；原partial accepted journal与未消费sources不可丢。
- migrations/goal_source_tracking.go:32 withdrawal直接INSERT request/intent；goal_governance.go:57 late review同样直接INSERT。这些旧SQL和升级前queue必须由实际claim gate兜住。
- workflow/workflow.go:680与710已支持deferred/not_before→Sleep→ContinueAsNew；kevVersionedDelay在:2501有history version分支。优先不改workflow命令序列。

## Planner

- core/goal_set.go:634 requestGoalPlanningTx→SQL public.request_goal_planning；旧0055定义在migrations/goal_planner_cadence.go:16，Actor/profile review可立即调用且满额可绕过。
- core/goal_planner.go:327 ProcessGoalPlanningIntent：committed恢复:364应先返回；:419旧claim-time reason/capacity gate；:449 claim；:478真实RunGoalPlannerAgent。统一vacancy/daily/completion gate必须在模型前，不能只修改SQL。
- 旧goal_planning_change在migrations/goal_planner.go:107对completed/paused/cancelled/abandoned全发立即lifecycle触发；新规则只completed立即补位。其余事件可以保留为上下文。
- Actor facts core/actor_facts.go:252，relationship_edit.go:228，profile DB trigger、app初始化、auto-enable、cognition/reflection wish均能积累events，但不应独立开高频预算。
- core/goal_planner.go:522成功时late Actor/profile可排下一2s run；:590 RepairGoalPlanning每分钟可直接INSERT repair run；goal_planner_recovery.go:81用actor_context_changed触发context review。全部需要新规则，不可成为旁路。
- 当前daily仅core/mutations.go:326 schedule:<timezone>:<localDate> accepted稳定事件，没有独立每日保障；full原本由SQL、claim、repair三处拦截。此次full gate扩展为所有调用（含Owner/review），daily只有有空位才做。
- DB now用于available/lease；readLifeContextTimezoneWith在life_context.go:90读取实例identity IANA时区。不能用host时钟对比数据库时间。
- 每owner live run唯一；private events在未激活profile前保留。不能用全局max watermark把旧private完成事件的预算吞掉。

## 迁移与测试

- migrations/runner.go Head=0062_reply_media；Apply每次重装goalPlannerSchemaSQL、goalPlannerCadenceSchemaSQL等。新增0063放在旧安装之后，released SQL不改。
- 现有goal_planner_cadence_postgres_test.go演示恢复旧函数+旧head，再Apply两次的真实升级测试。
- core/goal_assessment_memo_integration_test.go：重复memo无I/O、owner force、新proof及失败重试，需要适配30min。
- core/goal_evaluation_tools_integration_test.go：partial retry/frozen refs/stale replacement；goal_closure_recovery_test.go：claim fences与batch remainder。
- core/goal_planner_test.go:497旧full daily/review/owner suggestions断言与最新用户冲突，需定向更新；:438失败retry，:899 private events，:1072 durable commit crash，:1116 recovery policy。
- core/goal_planner_recovery_test.go旧context repair应改为积累/等待合法daily或completion，不清理旧failed历史。
- 本地测试DB必须由任务自建disposable环境，不能读取未知服务URL；无环境明确SKIP。主机command -v只发现docker，无postgres/initdb/pg_ctl/psql。

## 已核实历史原话

原用户会话01a11e4b-9c1a-7381-b8a8-eebc40c4e4a6 turn40：goal_planner与goal_evaluation运行密集；相同情况无更多动态不会完成目标；planner应在goal_evaluation完成某目标有空位后执行，或者每天日程出来后一次。任务10-09-fluctlight-goal-planner/prd.md:546与design.md:11–12已落盘。历史提交bc90e52实现0055，但不证明现在线上符合。

10-10用户另有明确“普通聊天/WakeUp目前先不用提供修改目标状态tool，让Goal Evaluation agent专门处理”，见10-09-kev-decision-integration/prd.md:114–116。本轮用户确认不讨论Tool，因此维持现状。

历史半小时原话针对Reflection，没找到旧Evaluation30min明确要求；Evaluation30min以本轮用户明确指令为准。会话列表时间不等于具体turn发送日期，勿伪造精确历史时间。
