# F15：Goal Evaluation / Planner 频率纠正

## 用户最新要求与范围

2026-10-14 用户多次反馈目标评估/规划高频，最新明确：Goal Evaluation 改为30分钟一次；Goal Planner 必须在目标有空位（例如正式完成释放名额）时才能调用。用户已明确本轮不讨论、不修改普通认知/WakeUp的目标状态Tool。

沿当前任务和 codex/fact-only-reply-vision 实施，不重新规划领域模型。容量修复已由用户另行提交到ec1ed82，当前源码基线工作区干净。当前追加授权包含代码与本地隔离验证，不包含自动提交/push/部署/线上DB或workflow操作。

## 已核实原因与历史

- evaluation 是事件驱动2秒入队，pending claim不检查available_at；失败1分钟retry，SQL trigger和frozen retry有独立入口。没有固定周期或跨请求/跨profile的30分钟fence。
- planner SQL/claim/repair/settlement后续触发均允许Actor/profile立即规划，生命周期触发也把paused/cancelled/abandoned当完成；Owner/full review可绕过容量。
- 10-09-fluctlight-goal-planner/prd.md:546与历史原话已记录“目标完成有空位或当天正式日程出来后一次”。10-09-kev-decision-integration/prd.md:114记录普通聊天/WakeUp不提供修改目标状态Tool。因此维持专用Evaluation提交权威。

## 必须成立的行为

1. 以Fluctlight为粒度建立持久化的30分钟evaluation周期/准入，不按单Goal或profile各跑一轮。事件可合并/积累证据，不把周期提前。实际Process入口覆盖pending、retry、冻结快照重试、旧SQL触发与直接Activity调用；同一Agent内部原生Tool回合属于一次评估，不逐物理请求sleep30分钟。
2. 补齐周期工作生产/修复，让有有效active/paused目标的实例即使没有新事件也有30分钟评估机会。相同事实memo仍可跳过无意义模型工作；无目标时不做模型工作。使用DB时间与原PostgreSQL intent/Temporal timer，不引入第二调度器。
3. defer提供明确reason/not_before，保留pending证据、forced IDs、claim CAS、冻结refs、accepted submission journal。等待周期不增加失败attempt；失败/recovery不能一分钟又跑一轮。不得sleep阻塞持有的logical lease。
4. Planner模型调用前统一重查真实active Goal数量与max_active_goals；所有入口（含Owner显式、Actor/profile、旧queue、retry、recovery）满额均不调用模型、不绕过为suggestions。Stage/Commitment不占目标名额。
5. 自动Planner只接受正式完成后的补位或本地自然日一次的日规划，且两者必须有空位。Actor/profile/relationship/wish等只积累上下文，不能每次变化独立调用。暂停/取消/放弃不是“完成后立即补位”。初始化/恢复可由空位条件下当天唯一的日规划承接，避免零目标无法自举。
6. 每日按现有实例IANA timezone与当地日期，不按滚动24h。日程正式接受保留稳定每日source；需要补齐漏日/重启的日保障，但满额和相同日反复repair保持静默。自然日预算按自动Agent episode实际准入记账，失败不能每60秒重新获得同一日/同一完成事件预算；新完成或下一日本可重新尝试。Owner明确请求有空位时保留其既有显式语义，但不能由模型伪造。
7. 旧queued run也必须执行新准入；已有durable commit恢复/replay先处理，不能重调模型或改写历史。未消费事件、private profile证据、Goal/evidence/history保持；不重置旧failed/committed记录。

## 实施边界与技术决定

行为位于Goal请求的持久化调度、claim/Agent准入、Planner触发函数与repair，不在Prompt口头要求或可选Kev。引入一个最小、Go-owned的operational cadence持久化fence（可用单表/现有适当operational字段），保存跨请求/Worker的evaluation下一周期和Planner每日/完成触发的已准入记录；它不成为目标完成权威。

新增0063 additive migration（实际编号按runner当前0062 head），不修改已发布0054/0055等migration常量。Runner每次Apply会重装旧SQL，新的request_goal_planning/trigger/fence必须在其后安装，覆盖fresh、0062升级和head重跑。完成事件去重必须保存逐事件或等价精确标记，不能用全局watermark吞掉尚未激活private profile的低序号证据。

主代理负责方案/契约/任务文档与最终验收；trellis-implement负责下面代码与关联测试，读取原文后直接实施；trellis-check做独立检查。代码边界预计：

- core新增goal cadence模块与测试：DB原子准入、30min/daily计算、稳定due生产、可见诊断。
- core/goal_evaluation_runtime.go、goal_sources.go及必要旧source trigger兼容：统一claim延迟、冻结retry、周期修复；不改完成判断/证据规则。
- core/goal_planner.go、goal_planner_recovery.go及必要触发装配：空位/完成/每日统一准入、旧queue/late event/failed重试处理。
- internal/migrations新增cadence bundle，runner.go及迁移验证/升级测试。
- cmd/worker/main.go在既有repair lifecycle接入持久化due修复；workflow本身优先复用现有deferred not_before，不改变已录制history命令序列。
- 对应已有Goal memo/recovery/cadence测试按新的时序契约更新；不能靠全局关闭cadence或将旧集成测试全部skip来变绿。

不改Tool权限/完成条件、Goal容量值、模型/采样、非Goal Agent频率，不访问生产或更改发布渠道。

## 必要验证

- 纯时间/准入回归：29m59s无第二轮、30m边界可执行；同实例多请求/profile与并发只放一轮；pending/retry/frozen路径不绕过；defer不消费sources/attempt；周期无新消息仍能due。
- Planner满额（包括Actor/profile/owner/old queue/retry/repair）零模型调用；真实完成+空位允许；pause/cancel不立即触发；有空位每日一次、同日重复/无变化repair不重跑；失败同一触发不高频补打；新完成/次日恢复。
- 原due workflow not_before与durable commit恢复、private profile保留、CAS/授权/完成证据及partial accepted submission回归。
- 新migration fresh/0062→head/head rerun，旧函数恢复后Apply重装新规则；断言历史/来源不改写。
- 本地Go定向后全量test/race/vet/build及工作区必要质量检查；真实DB仅可使用任务新建的disposable隔离环境。缺失环境如实SKIP，不能当数据库/线上通过。

## 实施收敛

F15已实现，独立检查收紧daily为捕获的合法当日事件。最终Go与工作区门禁通过；真实PG时序/原子性/升级SKIP，详情见verification/goal-runtime-check-summary.json。源码未提交，不部署。
