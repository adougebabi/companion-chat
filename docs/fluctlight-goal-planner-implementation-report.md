# Fluctlight 独立目标规划实施交付报告

本轮实际实现并接通代码、版本化迁移、正式 API、Vue 页面、原生 Planner/Tool 与生产 Worker。最短业务路径、原完成链、容量/版本/幂等/依赖以及真实 PG/Redis/Temporal Worker 重启均有执行证据。**未宣称任务书 60 项和八条业务场景全部完整验收**：真实 Provider 未配置；矩阵中的 PARTIAL/NOT_VERIFIED/NOT_RUN_FULL_SCENARIO 列出未覆盖组合，不能算 PASS。本任务保持实施中，后续验收可从已有成果继续，不回退或重做旧闭环。

## 1. 基线与工作区

- 实现提交：`9feec97`（`feat(goal): integrate independent planner and governed goal sets`），分支 `codex/fluctlight-goal-planner-20261009`；仅本地提交。
- 实际初始 HEAD：`71f6066febe068e8eae028e97e4fe409747bf149`，原分支 master；初始工作区干净。
- 没有切换/回退历史定位 `ee1e468`。没有清空生产库、读取生产私聊或删除用户容器/卷。
- 用户的“上一轮能完成”是需求背景，不是自动化通过证据。实际基线使用随机隔离 PG，逐子场景运行：
  `TestFormalConversationWithoutIntentionCompletesOriginalExpressionGoal`、
  `TestScheduledAcquisitionGoalClockIndependentWearAndFinalWorkflow/{boots,brush}`、
  `TestGoalPauseAndCancelPreventOldDueAndDirectActivity/{pause,cancel}`。
- 保留原 Goal ID、criteria/evidence、Resolution、Stage、Commitment、Intention、日程、虚拟物品权威、执行授权及 Eino loop。
- 调用入口的源码/权限/收敛映射在 `.trellis/tasks/10-09-fluctlight-goal-planner/research/entry-map.md`。

## 2. GP00–GP12 实现映射

| 任务 | 实现与实际生产调用方 | 验证/限制 |
|---|---|---|
| GP00 | 实际 HEAD/状态、六个原 Goal 写入入口盘点；基线完成/真实入库/暂停门禁 | 基线实际执行；未读取生产数据 |
| GP01 | `app.insertAgency`、`goal_initial_imports`、`goal_initial_source_reviews`；稳定来源和精确既有结果关联；超额 candidate；Working Persona/compiler/persona.detail 移除具体 desires/goals；初始化 UI 独立初始目标 | 一次导入/暂停取消复用/来源重入通过；任意混合文本保留原文并由 Owner 语义复核，不能当自动语义回填已完成 |
| GP02 | `actor_context.go` 复用 actor_facts/relationship revisions；带 Actor context digest 的 API；兴趣/偏好；Actor fact/relationship edit 请求规划并标记相关 Goal 待复核；Goal 执行启动门禁 | 实际 API、页面保存刷新、409、A/B 隔离通过；Owner关键地点/时区变化对未指定对象的 Goal 保守复核，粒度仍可细化 |
| GP03 | `goal_set.go`、`persistGoalAuthorityTx`、`goal_set_guard`；实例级策略/版本、5活动/20候选默认；原子创建/暂停/恢复/排序/依赖；同键并发重放 | 真 PG 并发/SQL兜底/回滚/过量存量通过；不以进程 mutex 保证 |
| GP04 | `FormalAgentGoalPlanner` / `goal_planner_v1` / 私有 `goal_planner` surface；`goal_planner.query` + `commit`；同一 Eino Runner，最大12轮；有界有效人格首轮摘要，按需真实查询、正式提交、回读 | 原生脚本 Provider、真实工具和 DB；独立成功/失败/权限/幂等测试；真实 Provider未验收 |
| GP05 | 原后继写入撤销，Resolution保留真实结果/残余动机/线索；历史/Actor/当前生活/日程查询；source/精确结果去重；模型语义比较规则和差异来源 | 精确来源/历史拒绝和原链收敛通过；改写同义、周期新需求、拒绝后的候选质量不声称已通过真实模型 |
| GP06 | effective_order；已有默认在前，新激活追加；Owner人工完整顺序；Planner不覆盖人工排序；局部作用域调整保留隐藏目标位置；ordinary agency/Intention读取顺序 | 实际人工顺序保存刷新/重启、默认追加通过；Deadline插队质量和首目标等待选择下一个的独立完整脚本仍未覆盖 |
| GP07 | goal_dependencies；同实例/合法作用域/AND/环检查；集合串行边界；Intention创建/提升/Worker旧到期/动作启动复核；前置变化排原GoalEvaluation | 并发环、回滚、旧due拒绝、实际前置表达完成后原虚拟动作启动通过 |
| GP08 | goal_planning_events/runs + `goal.plan` workflow intent；合并、processed_run_id、私人事件待激活、claim/lease/fencing、明确commit_succeeded、崩溃后补偿；Worker原repair ticker；策略阻塞保留并待授权恢复 | 真实 Temporal Worker/Redis/PG、Provider临时失败+Worker重启通过；参数可由Owner收紧调整：2s合并/60s退避/300s租约/5尝试/20候选默认 |
| GP09 | goal.decide/intention.decide/Reflection create/resume/Resolution自动链统一请求Planner；intention.schedule消费planning_requested且不先调用日程模型；Goal激活仍排既有评估/推进；goal_policy独立精简运行slot | Main真实Tool回填、Scheduled wish不暗建Goal/Intention、旧获取链保持；普通聊天无固定新增Planner模型调用 |
| GP10 | GoalPlanningPanel/ActorContextEditor/GoalPanel保留证据历史；开关/人工顺序/依赖/来源复核/保护/Actor/profile创建/恢复参数；生成Core+Browser OpenAPI/client；正式HTTP/BFF映射 | 生产构建真实认证/DB页面，Actor编辑、排序、依赖、关闭与刷新保持；390px无横向溢出、无控制台错误；实际页面创建/暂停/恢复/候选激活及第六个拒绝另行验证，保留草稿与数据库计数一致 |
| GP11 | `0054_goal_planner` additive，从0053及空库；保留所有业务关联；有界 `goal-planner-migrate --after --limit` dry-run/apply；旧超额不强制删除/暂停；旧愿望人工源治理 | 真实隔离库迁移/重入/历史关联/超额测试；没有操作实际生产存量，也没有调用模型做启动无界回填 |
| GP12 | 分层测试、60行矩阵、E2E映射、截图、复跑脚本、开销和剩余风险 | 查看每行范围；没有把父测试PASS/HTTP200/SKIP/模型台词当全部业务通过 |

## 3. 权威、权限和数据规则

`active` 是 Goal 唯一活动生命周期；执行中/等待/受阻保留 active，因此占用实例名额。candidate/paused/completed/cancelled/abandoned 不占活动名额。Stage/Commitment/Intention 不另计。所有正式写入汇入同一持久化边界，数据库另有短事务 advisory lock 的 guard。

自动规划开关只管集合补充/激活/重新编排。既有 Goal 执行、日程和预算仍用原权限；关闭不取消它们。Owner手动请求在关闭时是 suggestions，Tool不能自称approved；在途关闭/实例暂停/Actor事实版本变化/切换人格/旧租约均拒绝陈旧提交。

人工排序禁止Planner覆盖；新目标恢复/激活追加。依赖和 derived_from 分离；多前置全部完成才能开始新的关联动作，历史完成不代表当前仍持有资源或长期拥有对方许可。虚拟购物还是原获取链，没有现实支付权限。

背景/关系来自 actor_facts 和 relationships。Owner配置来源、版本和有效时间仍保留；配置不能创造对方发出的消息。不同Actor数据/目标复核隔离。Actor有未知地点/时区时不补服务器或Owner时区；时刻使用带区RFC3339、持久化UTC。

初始导入来源稳定ID+输入版本+Goal映射；原始文本不覆盖。相同来源重试、修订不隐式复活。历史desires/goals结构字段被隔离为待语义复核来源，Owner可关联现有历史ID、保存非活动候选或排除当前行动含义；稳定人格/关系修改继续用原正式编辑权威。

## 4. Agent / Tool 实证

`TestGoalPlannerE2EAllGoalsFinishThenNewGoalFinishesOriginalChain`：三个不同有限表达目标，实际发送消息→原GoalEvaluation→3个Resolution；结束请求合并；Planner snapshot/persona/history/Actor/life查询→commit→snapshot回读，共8次物理模型请求；创建一个不同的新有限目标，另一次实际消息→原证据链→第4个Resolution。没有SQL写completed，没有新执行系统。

`TestGoalPlannerNoViableCandidateIsQuietAndDistinctFromFailure`：读实际snapshot/history，空位不作为新动机；记录原因/复核条件；重复处理同一run不增加调用。联合Worker测试的空白实例没有新兴趣或实际关系接受，落盘no_viable_candidate，不重复表达。

`TestGoalPlannerCommitToolStandaloneReplayConflictAndProtection` / `ToolsIndependentSuccessFailureAndPolicy` / `QueryAndCommitDependencyFailureWithoutAgent`：两个Tool分别独立成功/失败/越权/无服务/幂等，查询没有隐藏写入。`commit_succeeded`独立于applied[]，纯排序/依赖也可恢复；已提交run不能另开第二个命令假装一份事务。

## 5. 实际验证和命令

本轮专属镜像/容器：`pgvector/pgvector:pg16`、`redis:7-alpine`、`temporalio/auto-setup:1.29.7`，仅本机端口55439/56389/57239。测试自建/清理随机DB；少数既有诊断测试使用另建的`goal_planner_suite`空测试库。

```bash
export GO_CORE_TEST_DATABASE_URL='postgres://fluctlight:fluctlight@127.0.0.1:55439/goal_planner_suite?sslmode=disable'
export GO_CORE_TEST_REDIS_URL='redis://127.0.0.1:56389/0'
export GOCACHE=/private/tmp/fluctlight-go-cache
go -C apps/core-go test ./... -count=1 -json
go -C apps/core-go test -race ./internal/core -run '^TestGoalPlanner' -count=1 -json
GO_GOAL_TEST_TEMPORAL_ADDR=127.0.0.1:57239 GO_GOAL_TEST_REDIS_ADDR=127.0.0.1:56389 \
 go -C apps/core-go test ./test/integration -run '^TestGoalJointPostgresRedisTemporalWorkerRestart$' -count=1 -v
pnpm generate
pnpm typecheck
pnpm test
pnpm build
bash infra/acceptance/check-core-openapi.sh
go -C apps/core-go vet ./...
go -C apps/core-go build ./...
git diff --check
```

完整最终结果、实际执行时间及leaf事件以 `docs/verification/goal-planner/20261009/` 下的 `go-tests.jsonl` / `planner-race.jsonl` / `joint-worker.log` / `browser-tests.log` 为准。早期失败被修复后重跑：协议长度/续轮预算不放宽；新fixture人格补齐习惯权威；等待和租约用数据库时间，不比较VM与主机毫秒偏差。

最终结果（`test-summary.json` 可机读）：

| 检查 | 实际结果 |
|---|---|
| Go 全仓 `go test ./... -count=1 -json` | 1939 PASS事件，1804叶测试PASS，25测试SKIP，0 FAIL；2026-10-09 13:22:27–13:31:11 +08:00 |
| Planner `-race` | 32 PASS事件，30叶测试PASS，0 SKIP/FAIL |
| 最后容量提示修复后 HTTP 两包复跑 | 188 PASS事件，179叶测试PASS，0 SKIP/FAIL（`http-final.jsonl`） |
| `pnpm test` | Web 72 + Browser client 18 + Core client 2 = 92 PASS，0 SKIP/FAIL |
| generate/typecheck/build、OpenAPI route inventory、Go vet/build、diff | 全部PASS；最后修改的HTTP/Web包已复跑 |
| PG/Redis/Temporal Worker 临时失败与重启 | 单独真实设施运行PASS，`joint-worker.log`；全仓运行缺少专用Temporal参数时的SKIP不作为此证据 |

跳过清单完整保存于 `test-summary.json`，主要为真实模型/视觉外部设施。早期一次 `pnpm test` 被sandbox的tsx IPC socket权限阻断，移出sandbox复跑PASS；浏览器测试夹具一小时登录过期已延长为12小时。实际页面发现容量错误曾被误报为版本变化，已修复Core/BFF专用错误码和Vue提示，新增草稿保持测试并在生产构建复测。

**真实 Provider：未验证。**环境未设置 FLUCTLIGHT_LIVE_PROVIDER_TEST/URL/MODEL。可重跑：
```bash
FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
FLUCTLIGHT_LIVE_PROVIDER_URL='http://your-provider/v1' \
FLUCTLIGHT_LIVE_PROVIDER_MODEL='your-model' \
GO_CORE_TEST_DATABASE_URL='postgres://TEST_ADMIN@TEST_HOST/postgres?sslmode=disable' \
go -C apps/core-go test ./internal/core -run '^TestFormalAgentE2E/goal_planner$' -count=1 -v
```
API key只用环境变量，不打印或提交。测试会自建随机库。输入包含多个合理稳定兴趣；无候选不会被自动判为通过。`scripts/e2e/goal-planner-acceptance.sh` 提供可重跑入口，明确记录缺失设施/真实Provider未配置。

## 6. 60 项和八条 E2E 的范围

完整60行见 `verification/goal-planner/20261009/acceptance-matrix.csv`。PASS_SCRIPT_PROVIDER只证明确定性链路，PARTIAL只证明列明子范围。测试事件总数包含子测试和父测试，不能当60条业务验收数量。当前矩阵：25 PASS、5 PASS_SCRIPT_PROVIDER、1 PASS_MEASUREMENT、25 PARTIAL、3 NOT_VERIFIED_REAL_PROVIDER、1 NOT_RUN_FULL_SCENARIO。

| 业务脚本 | 本轮实际证据 | 状态 |
|---|---|---|
| E2E-01 全完成→新集合→原链完成 | 专门E2E01测试，3旧Goal实际完成、1新Goal实际完成，合并run，8轮工具循环 | PASS脚本Provider；真实Provider未运行 |
| E2E-02 表达不复活/不等于关系 | 原真实表达/双方关系acceptance/refusal/quotation/open_signal、一次导入/源关联/人格Tool链回归 | PARTIAL：完整同一初始文本→重编译→WakeUp→接受新方向没有单条整合脚本 |
| E2E-03 Actor编辑影响可行性 | 实际页面/API/DB保存国外/时区，A/B独立版本、相关复核及启动门禁 | PARTIAL：海外背景到远程候选质量需真实Provider；完整原同城日程组合未跑 |
| E2E-04 并发五容量 | 真PG多连接Owner创建/恢复、直接SQL兜底、同键并发、响应丢失恢复 | PARTIAL：多OS Worker/Owner/Review同一时刻的独立进程脚本未跑 |
| E2E-05 旧优先/人工保持 | default追加/重启，实际页面人工顺序保存刷新，后端manual保护 | PARTIAL：真实近截止自动插队、第一目标等待而第二执行的整条场景未跑 |
| E2E-06 依赖有效 | 旧due与直接start被拦截，前置通过真实消息完成，原虚拟动作启动；并发环拒绝 | PARTIAL：五受阻+受保护+候选必要前置完整改组业务组合未跑 |
| E2E-07 安静/人工停止 | 合理空集合不重跑，关闭/旧租约/Owner保护，政策恢复补偿，实际UI关闭刷新 | PARTIAL：仅两个候选质量及所有开关/手动建议组合未作为单脚本运行 |
| E2E-08 迁移/恢复/资源 | dry-run/批次/重入/旧超额保留；纯排序提交后崩溃、私有事件、Provider失败、实际Worker重启；物理输入统计 | PARTIAL：生产存量未操作、所有故障在一份混合测试快照的完整脚本未跑 |

## 7. 迁移和旧路径清理记录

SQL迁移保留所有旧业务ID/事实/修订/日程关联。首次排序是稳定创建顺序，历史证据不清零。存量活动超过策略上限会显式标记违规，并禁止增长；必须由Owner治理，不能自动随意暂停。已有初始Goal映射为linked_existing，历史愿望原文保留为requires_semantic_review，不在启动中发无界模型请求。

`goal-planner-migrate` 使用显式Owner、实例、after水位和1..100批次，默认dry-run。初始legacy链接数/待复核/容量违规按具体实例输出；隔离测试的八超额Goal保持八条，新增被拦截。没有实际生产回填数量可宣称；生产迁移未执行。

撤销/停止的自主生成生产入口：Reflection直接create/resume、Resolution后继candidate写入、普通goal.decide激活、无GoalRef的模型intention.decide隐式Goal创建、模型intention.schedule隐式新目标+排程。它们现在只提供来源/请求。**显式受授权的直接Owner命令保留**，统一经过容量/版本门禁，不能将此混成旧自主生成链。

## 8. 运行开销

普通聊天三个同类路径的物理模型调用数量仍是1/2/3，没有固定额外Planner模型调用；新请求只由真实领域变化/Owner动作触发。before为实际初始HEAD的既有wire统计，after为最终运行统计（见cost-comparison.json）。

| 普通路径 | 物理请求前→后 | 估算累计输入前→后 | 增量 |
|---|---|---|---|
| natural_final | 1→1 | 39833→42668 | +7.117% |
| reply_tool_then_final | 2→2 | 80060→85730 | +7.082% |
| takeover_reply_then_final | 3→3 | 122957→131462 | +6.917% |

字符和token均为脚本HTTP真实请求体的估算，不是真实Provider计费量。

Planner完整8轮示例：累计估算输入102777、单轮峰值19279、总wire字符82219。原五轮工具链峰值49124/49152，仅剩28估算token，虽然回归通过，增长余量很小。不要仅看第一轮短Prompt。角色共用原Provider队列/预算；最大12轮，默认2秒合并、60秒退避、5分钟租约、5次自动尝试，耗尽不无限自激。一次原五轮工具链的峰值仍接近49152上限，已通过原预算检查，后续复杂数据应继续监控，不宣称免费优化。

## 9. 部署、前向修复和剩余风险

1. 在部署环境按原备份流程备份PostgreSQL和配置/密钥；本轮没有读取或备份生产库。
2. 停旧API/Worker写入，运行正式 `cmd/migrate` 到0054；按Owner/实例 dry-run `cmd/goal-planner-migrate`，审查来源与超额；apply逐批源关联，不改变终态。
3. 运行原 `cmd/persona-backfill --owner ... --fluctlights ...` preview/apply，重编译受源结构隔离影响的Working Persona；保留原文本，不能重建实例。没有可用Provider时，不把这一步说成已经完成。
4. 超额实例Owner暂停/治理到策略容量。保留原授权/自治关闭。启动新API/Worker和新Web，验证Goal/Actor配置与实际原执行链。
5. 优先前向修复；旧二进制会恢复旧人格注入/自主入口，并不理解新策略。即使DBguard仍在，不能把“只回滚二进制”视为完整安全回滚。恢复旧备份也不能静默抹掉部署后的新事实。

剩余：真实模型候选质量/语义去重/拒绝边界未验收；任意混合历史文本需Owner语义复核；Owner背景变化对未指定对象Goal的失效粒度保守；60矩阵和八场景中未完成的组合按表记录；生产存量/旧Persona模型回填未操作；普通五轮输入预算接近上限。所有这些都不是SKIP通过，也不是“模型说完成”证据。

## 10. 页面实证

随机数据库、真实认证、正式BFF/Core，生产构建页面完成：关闭自动规划、保存Actor所在地/时区/兴趣、保存人工顺序、保存依赖、刷新重新读取保持。控制台error/warn为空，390px document.scrollWidth=390，无横向溢出。截图中的pending run是没有启动Provider/Worker的独立UI夹具，不代表Planner业务成功；业务成功使用上面的真实工具/数据库/Worker证据。

![窄屏实际治理页面](verification/goal-planner/20261009/ui-mobile.jpg)

页面生命周期补查：实际创建Goal `goal_owner_67e9967148803e1f50c7db22f582daf8`→pause→resume（revision 3）；候选 `goal_owner_569667eaaa8bc497faf020a2e4497d52`→active（revision 2）；满5时第六条没有入库。数据库记录见 `ui-lifecycle-db.log`。新构建用另一份满5随机库复测容量专用提示、草稿不丢失，见 `ui-capacity.jpg`。UI夹具无Provider/Worker，其pending不是规划成功证据。

![实际页面拒绝第六目标并保留草稿](verification/goal-planner/20261009/ui-capacity.jpg)

临时资源回收：两份浏览器随机数据库均查询确认已删除；夹具进程与临时浏览器页已关闭；本轮专属PostgreSQL/Redis/Temporal容器及网络已移除。没有删除用户容器或卷。

## 11. 后续评估/规划频率修复

2026-10-09 后续用户反馈已执行代码修复：前向0055、满额/提示/修复门禁、每日日程一次、缺覆盖修正与固定合同终止。具体线上证据边界、测试及尚未部署项见 [Goal频率修复报告](fluctlight-goal-cadence-fix-report.md)。原0054交付和原60矩阵的真实Provider/完整组合缺口保持历史含义，不因本次定向修复自动变为全部通过。
