# 目标与意图：推进链路诊断与最小修复设计

日期2026-09-08，版本v1。用户指出目标与意图可能只是字面数据，要求核验并纳入基础规划。本轮只读探索和主代理关键代码抽查，未读取真实业务数据、未跑模型或故障注入；以下区分代码事实与效果推论。

## 1. 已有路径与断点

真实路径：初始化/反思写Goal与Intention → 角色wake-up读取上下文 → 模型提议工具/自主动作 → 策略验证 → 冻结Action → Temporal执行 → 成功结果进入反思。
目标确实被模型参考，能力执行也是真实的；但这不证明某条意图到期后被推进、某次行动完成后原目标被正确更新。

| 证据ID | 已确认代码 | 锚点（均以当前符号为准） | 用户可见风险 |
| --- | --- | --- | --- |
| E15 | 反思上下文中的目标/意图丢实体引用，写端却要求goal_id/intention_id；新建意图也需已有goal ID | provider_context.go:215/242；workflow_ops.go:455/1042/1077/1098；对应context测试明确断言去ID | 能读文字，却无法可靠指定更新哪个实体；同名尤其危险 |
| E16 | schema接受progress等数值，goal UPDATE不写；complete只改status；意图UPDATE不改trigger/time/expiration | provider_schemas.go的goal候选；workflow_ops.go:1038/1063/1118 | 进度提议被丢，可能completed但原始progress仍0，执行条件无法更新 |
| E17 | 初始化与反思create意图固定expiration=now+24h；读取用expiration>now过滤，未发现独立到期/expired结算消费者 | app.go:924；workflow_ops.go:1094；autonomy.go:259；workflow dispatcher当前注册项 | 24h后退出模型视野，详情仍可能pending；没有依据宣称已按意图到期执行 |
| E18 | frozen Action和tool/result没有标准goal/intention归属；成功有autonomy.result+reflection，已检查失败路径只有状态写入 | wakeup.go冻结payload；tool_contract.go ToolCallV1/ToolResultV1；workflow_ops.go:292/314/326 | 做了事不等于推进原意图；失败难回到目标的阻塞/下一步 |
| E19 | 详情平铺文字/状态/进度，“活跃目标”用所有goal长度；意图投影缺触发/截止/结果 | detail.go readAgency；apps/web/src/components/instances/InstanceDetailsDialog.vue:342/349 | 无法看出最近做了什么、下一步和阻塞；活跃计数包含其他状态 |
| E20 | E2E验证初始化非空、聊天/媒体/主动联系发生，未断言同一goal/intention前后推进 | scripts/e2e/persona-layers-real.mjs:295/338等；reflection/context tests | 现有绿色用例不能证明目标推进闭环 |

上表简写Go文件位于apps/core-go/internal/core。工作区持续变化，开工前必须刷新；不能声称线上所有目标都不推进或反思一定失败。
精确边界：某些对话额外关系system context可能带原始意图信息；ProcessReflection使用的紧凑上下文没有对应可回解映射，不能用前者推断后者已经拿到更新ID。
paused意图目前未被读取过滤排除；它可以作为反思背景，但不能据此视为可执行。权限评估走独立action policy，不等于意图已qualified。

## 2. 建议A08：最小可验证推进闭环

作为本批基础修复的新增审阅范围，复用现有wake-up/反思/Action执行，不新增另一套agent循环或每轮第二次回复生成。
A08交付：目标实体可定位、合法更新真实写回、现有行动有归属、成功/终态失败可反馈、过期与暂停可解释，以及页面看到真实结果。A08不承诺完整定时意图调度或复杂自动重规划（B05）。

### 2.1 引用与输入快照
为需要更新的场景建立goal_ref/intention_ref和服务端映射，短引用不必暴露原始数据库ID。
引用绑定本次持久context/证据窗口的实体ID、owner、profile和expected revision；排序变化不改变同一请求重试的映射，不能按描述或数组下标猜目标。
模型的update/complete/pause/create-intention提案引用短ref，服务器解析后验证作用域/版本。未知、重复、跨角色/跨profile、过期引用明确拒绝，不自动创造同名实体。
同次create-goal+create-intention支持响应局部client_ref：先验证引用关系、预分配稳定ID，再在同一事务创建目标与意图；禁止模型猜服务器将生成的ID。重复提案重放同一结果。
普通聊天不必携带所有目标ID；只为合法候选暴露可引用的最小集合。引用必须经composer/metadata清理后仍可用，不能再次被“去技术字段”抹掉。

### 2.2 更新与生命周期
目标可更新字段和schema保持一致，至少progress/importance/urgency/description；字段缺省保留已有值，不能归零。数值范围[0,1]、owner/profile与revision由Core验证，语义进度由模型基于证据提案。
complete必须有合法完成依据，状态与progress一致（完成时为1）；普通update progress=1不自动代表业务已完成，需显式完成提案及证据。修正误判允许有理由的进度下降，不硬设单调递增。
revision审计需要before/after progress、状态、证据和理由，不能仅写from_status/to_status丢掉数值变化。
意图已有schema能提出但实现不能处理的字段必须明确合同：本批要么支持必要的confidence/time/expiration更新并校验，要么从该操作schema移除未支持项；不得接受后静默忽略。
读取/维护时为过期pending意图作明确expired生命周期记录，保留事实，不自动延期或重新创建。过期清理沿用已有Worker维护入口和幂等修订，不加第二调度服务。paused可作为背景但标不可执行；Owner暂停不允许模型自行resume。
24h默认值本批不贸然改成永不过期；显式呈现截止/过期原因。长期意图默认寿命和动态延期在B05单独收敛。

### 2.3 行动归属
模型在现有提案中可给source_intention_ref/goal_ref；服务端冻结时解析并验证一致性，将goal_id/intention_id、版本与source fact写入FrozenAction的受控metadata。
工具业务参数不被强行塞通用数据库ID；归属在执行封装/结果事实传播。没有目标来源的普通闲聊仍合法，不能给所有回复硬绑一个目标。
一个Action可先限定一个主Intention；多目标贡献以后另定，不由文本相似度自动归属。意图完成不自动等于目标完成，进度更新仍需证据支持。

### 2.4 成功/失败结果与反思
统一有界结算边界：结果带action/execution ID、来源goal/intention、结果状态、产物/evidence与失败code。成功和终态失败均形成可追踪结果事实供既有反思处理。
瞬时重试中的attempt错误只记诊断，不在每次尝试就把目标改失败。终态失败与后续显式新执行要区分；稳定结果键绑定执行/结算，重复投递不得重复加进度或创建反思效果。
“工具调用成功”只能证明动作产物，不由Core固定加0.5或完成目标；模型基于结果提出完成/进度/阻塞，Core校验提交。同一结果被重复反思也不能重复累计。
若失败结果不能落库，执行责任仍可对账重试，不能先标所有状态结束后丢失反馈。
用户取消、被新消息取代、权限阻塞与真实执行失败分别呈现，不统一为“目标失败”。未来动作或重规划不得绕过取消、权限和预算。

### 2.5 页面与直观诊断
按目标→意图→最近行动/结果分组，显示当前状态、进度依据、最近推进时间、下一步或阻塞、截止/过期及可用操作。没有下一步事实时显示“尚未形成下一步”，不能凭空生成。
活跃计数只统计定义中的active，completed/paused分开。旧记录无归属明确“未关联行动”，不靠名称回填虚假历史。
复用A04页面原因视图，直接点开同次action/result与反思；不需要导出或手填ID。领域投影与浏览器DTO同时更新，仍以Owner授权为边界。
目前没有结构化next step/blocker的地方，新增受控读模型或可审计字段前明确来源；不能把一段模型闲聊当成真实计划。

## 3. 验收：一个目标完成两次可验证交付

用隔离测试角色预置同名不同ID的G/G2，给G两个意图I1/I2，分别对应具备可验证产物的已安装能力。
1. 模型输入有可回解ref；首次认知选择I1，对应Action冻结后带归属；G2保持不变。
2. I1真实执行成功，结果事实进入反思；受控反思提案把同一I1完成、G进度由0到0.5并给证据。0.5是测试提案值，不是生产硬编码。
3. I2发生一次可重试错误时仅显示当前attempt；最终失败产生结果事实，G不误完成，页面显示阻塞/下一步或未知。
4. 同一结果重复投递/反思，不重复推进、不重复产物、不重复增加revision。
5. 显式允许重试后I2成功，反思有完成依据则I2完成、G completed且progress=1；取消或已过期时不能借重试绕过。
6. 另测同批创建goal+intention、过期pending结算、paused不可执行、profile/权限/CAS冲突，以及数值字段缺省不被重置。

这是A08的最小闭环。若只修progress SQL和页面、没有结果归属与证据链，不能宣称目标已能推进。

## 4. B05：完整意图推进与重规划（后续）

- 定义preferred time、到期time trigger、event trigger与semantic trigger各自的正式执行入口，使用既有durable runtime/inbox；到期只产生待评估事实，不直接按文字执行动作。
- 具备goal→qualified intention→冻结action→result→进度/完成/blocked→后续意图的状态表、过期策略、权限/预算快照和恢复合同。
- 开放目标不一定有百分比；区分长期方向与可完成目标，避免用数字制造假进度。
- 缺能力、需人输入、安静时段、预算不足、临时失败、终态失败要有不同下一次评估条件。不能每次wake-up机械重试同一失败计划。
- 复用A05持久唤醒保障与A08引用/结果链，但精确时间意图不能只靠quiet-period唤醒：用户持续聊天不应无限推迟已明确约定的到期事实。
- 目标拆解与重规划由同次认知或已有后台反思提出，不新建每次聊天必经规划模型。复杂外部工具结果依赖另定协议。
- 未明确完成准则/预算/可执行工具的目标显示等待计划或能力缺口，不假报执行中；不自动安装缺失工具。
- 必须在可丢弃环境验证真实due、延迟、事件重复、取消、重启和结果反馈。A08通过不代表B05定时自动推进已完成。

## 5. 实施约束
主代理实现/验证；代码归属按开工时diff刷新。A08依赖A01输入、A02/A03证据和A05可靠唤醒，在基本链路之后实施。修改schema、context、写回、冻结metadata、settlement与Web时同步所有生产消费者。
迁移只增量：goal progress审计快照/归属metadata等保留旧行可读；旧行动不按描述猜目标，旧completed且progress不一致的行单列不强改生产数据。事实回填/自动修正历史另需具体方案。
回滚保留新增列与修订，关闭新提案解析/归属写入时不能丢已冻结行动的读取能力；没有兼容验证不切版本。
