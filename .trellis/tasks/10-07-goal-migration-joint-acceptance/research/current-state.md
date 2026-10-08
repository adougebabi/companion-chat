# D 当前状态（2026-10-08，整体仍待真实验收）

用户明确分工：真实模型／实际使用场景由用户验收；主代理负责边界修复、技术验证与交付整理。勿重复请求开始或任务创建许可。分支 codex/fluctlight-goal-closed-loop，基线 ee1e468。A 已提交；B/C/D 仍 in_progress，源码／生成契约／规格已提交 f80a7b2，验收证据与 journal 正在收尾。不要覆盖这批改动。

## 已完成的边界修复
- 原生 due 目录接入 conversation.reply；触发与执行均使用已存在的 Owner 私聊权威，构建同作用域快照。旧 due 引用在 contact 出现后按持久 Intention/Goal/Attempt 身份重新绑定；版本／状态／人格和投影父链接检查保留。实际短期表达结算成功，未获回复的长期关系 Goal 进度仍为零。
- 重复抑制、QUERY/no_op 与业务成功分离；同步和异步聚合保留失败码，并让策略／权限拒绝优先于临时失败，禁止因此继续重试。
- 计划不替代评估，不吞来源；全 unknown 不伪记 no_change；assistant 自述不证明业务完成；全部 verdict 的证据遵守实例／人格／目标绑定；反证可以解释撤销但不能证明成功。
- 非关系 Goal 不能夹带关系确认；子对象限定于领取快照，及时证据可以结算已过期承诺；阶段／承诺证据建立持久关联，不因消费 journal 而消失。不同投递身份不重复阶段、关系、后续目标或 Resolution。
- Goal 终止后的成功／失败回调保留实际结果但不复活目标；硬期限后已开始活动可结算，不再次启动。活动证据撤销实际重评；已完成 Goal 保留真实评估／Resolution／历史并标记复核。
- 存量对账多批次中断重启／重放保持原状态，不复活 cancelled/abandoned，不伪造行动。
- Core、BFF、生成客户端与 GoalPanel 接入有界证据分页；Owner/Goal/evidence 游标绑定，时间与 ID 倒序。UI scope/selection epochs 丢弃旧分页，CAS 保留草稿及冻结版本。
- 生产 StartWorkers 返回的 Worker 使用 sync.Once 统一显式 Stop 与 context shutdown，修复真实 race 联合验收暴露的重复 Stop panic。

## 当前验证
- 完整受影响包 race：d-boundary-release-core-race.jsonl 1798 PASS events / 0 FAIL / 24 live/media SKIP（此后 native/contact/worker-stop 有新增修复，须以后面的最终全包日志为准）。
- d-final-three-e2e-identities.jsonl：10 PASS events，0 FAIL/SKIP；具体 ID 在 d-three-e2e-evidence-identities.json。
- d-native-contact-scope-rebinding.jsonl：19 PASS events，0 FAIL/SKIP；d-native-catalog-scope-targeted.jsonl：3 PASS，0 FAIL/SKIP。
- d-persona-initialization-neutral-review-fixed.jsonl：6 PASS events，0 FAIL/SKIP；两人格真实切换／可见性／工具和证据作用域，初始化有界规划，三类中性 Review 零惩罚。
- d-joint-worker-stop-race-fixed.jsonl：真实 PG/Redis/Temporal/生产 Workers 的重启、HTTP Owner、503 retry、重复消费，race 1 PASS，0 FAIL/SKIP；d-worker-stop-concurrency.jsonl 1 PASS。
- browser-client 17/17；core-client 2/2；Web 68/68；类型检查、Web build 已通过。Core OpenAPI 路由清单检查已补齐并通过。
- 最终全包并含实际联合设施：d-final-go-all-with-worker-race.jsonl 已完成：1842 PASS events / 1712 leaf PASS / 0 FAIL / 24 live/media SKIP；15 包 PASS，11 包无测试文件。go vet/build 和三个 client/web 类型检查全部通过。不要把之前有 fixture／清理失败的日志算通过；均保留原记录。

## 验收分工与待收尾
46 项矩阵当前 41 verified_scripted_or_domain / 5 pending_user_acceptance，没有 partial_verified。真实验收为 C04/C05/C11、C12 表达风格、D08 实际浏览器操作与观感；技术 scope／状态机／HTTP／客户端／UI异步逻辑已有证据。见 research/user-live-acceptance.md。24 live/media SKIP 不计通过。
最终统计与预算报告已保存，Core／browser／Web 检查通过，待用户真实验收；整体及 B/C/D 只有符合各自完整门禁后才 archive。不能把用户尚未反馈的真实场景写成完成。

## 隔离设施和秘密边界
Schema head 0053_goal_reconciliation。仅任务自有 PG fluctlight-goal-test-7ef70c25、network fluctlight-goal-joint-7ef70c25、Redis fluctlight-goal-redis-7ef70c25、Temporal fluctlight-goal-temporal-7ef70c25。运行包装脚本在 /tmp/fluctlight-goal-test-runtime；runtime.json 是秘密，禁止打印 URL/密码；joint.json 是自有设施清单。测试使用随机子数据库，不访问生产。收尾只能清理这些自有设施，勿碰其它容器、镜像、卷。
测试会改 apps/core-go/internal/core/testdata/turn_path_cost_report.json；保存需要的预算证据后，只恢复此 incidental 文件，不覆盖其它源码。

## 设施收尾
2026-10-08 已移除三个任务自有测试容器及唯一自有网络，证据 d-task-facility-cleanup.json。其它容器／镜像／卷未修改。包装脚本仍在 /tmp，但旧端点已无服务；后续重跑需重新创建隔离设施。

## 2026-10-08 实际推荐验收发现与修复

实际推荐目标卡在评估输入超限（53609 > 16384），页面刷新不能恢复。已修复 Provider 来源投影、12000 输入预算准入、新消息优先、只消费本轮证据及剩余来源续批。真实 PostgreSQL 的完整 Goal race 回归：104 PASS events，0 FAIL/SKIP；vet/build 通过。独立复核发现并修复 Actor 工具与数据库源字段形状差异。详细证据见 `.trellis/tasks/10-07-goal-migration-joint-acceptance/research/d-live-recommendation-input-budget.md`。尚未部署到用户实际环境，线上完成状态和本次真实验收仍待验证。

## 2026-10-08 后台持续占队列的最新修复

唤醒改为实际最后聊天后10m及后续10m，启动只在无key且无待处理认知时放行一次；反思改为聊天后30m、有新未处理证据才一次，按实例合并/独立epoch；新认知入队取消同实例WakeUp/Reflection并拒绝晚到提交。静默/检查回执不驱动Reflection或Goal水位，真实结果及Goal writer保留。最终全Go race1866 PASS events，0 FAIL，36 SKIP；vet/build与Web68/68/type/build通过。详细证据见 `research/d-background-trigger-closure.md`。未部署，真实验收仍进行中。
