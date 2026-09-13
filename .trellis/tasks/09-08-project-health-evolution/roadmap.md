# B批基础迭代与C批功能保留方案

本文件保留后续细节，不属于A批实施承诺。进入相应批次再收敛参数与迁移，不能因讨论过就自动开工。

## B01 上下文构建与预算

流程：ContextBuilder读取事实 → RoleContextPolicy选择片段 → BudgetAllocator分配 → PromptRenderer单次渲染 → ProviderRequestGuard完整检查。
这是组合式Go边界，不是动态插件框架；不增加一个必经LLM判断该读什么。

片段合同：key、source refs/revision、purpose、required、priority、content、estimated size/tokens、estimate method。诊断记录included/excluded/compressed及原因，技术metadata不全量进入prompt。
conversation必要项：当前Actor消息、当前profile、有效规则和未完动作；按需近期对话/记忆/关系/目标/工具，拒绝默认所有slot和假设。
wake-up必要项：时间/状态/触发/治理；按需目标、日程和经历，不复制全部聊天。
reflection必要项：水位证据、相关旧事实与修订；不重新读整个角色历史。
initialization独立输入，不携带尚不存在的runtime。

输入额度=min(角色上限, 模型context limit-output reserve-protocol margin)，messages/tools/schema全部计入。
先保必需项，再按相关性分剩余；低相关丢弃、长历史可摘要、关键证据留原文。必需项本身超限则明确失败，不能默删当前问题。
未知tokenizer/context limit不能当无限或0，估算必须标明。A基线后再定上限和收益，不拍定4k或节省70%。
摘要记录source range/revision/version；原文保留，事实纠错后失效重建。不要用每轮同步LLM摘要取代预算治理。
工具按角色、权限、安装情况与明确状态选择，不用关键词代替模型语义判断；保留发现额外能力的路径。
验收六类固定样本：短中文、长消息、多profile、旧记忆、后台行动、反思；同模型比较输入、调用数、排队/推理耗时及质量。

## B02 长期记忆召回

授权/状态/版本过滤在候选阶段执行；在全部合法事实范围分别取词法与同模型维度的向量候选，去重融合再排序。top-k限制结果，不限制只看最新k条。
复用PostgreSQL/pgvector，先精确检索；近似索引须有规模、过滤后recall、延迟与NAS资源基准。
importance/confidence/recency和语义相关性分项，静态高重要性不能让无关记忆占满输入。中文改写需专测，不能仍靠按空格分词。
Embedding不可用时事实不丢，允许词法/元数据降级且可诊断。旧model/dimensions/revision索引不可混用。
结果含ID/revision/type/content/source/evidence/confidence/scope及当前profile视角。按B01预算选取，首条超长不得穿透；同文不同Actor事实不能合并。
Owner可看候选、选中、排除原因与索引版本，产品日常回复不强制每句编号引用。
验收：旧事实>200、中文同义、近期无关竞争、索引失效、无Embedding、越权隔离、最终prompt与真实模型对照。

## B03 记忆更新和范围

拟定模型操作record、confirm、revise/supersede；携带目标memory ID、expected revision、证据与幂等key。
复用现有人工revision领域服务，不让工具维护第二套SQL。原子提交新内容/状态、revision、evidence、旧embedding stale、新intent/outbox。
语义判断由同次认知或已有反思做；服务器校验，不用“我不再”关键词判断偏好。
默认不开放模型永久删除；撤回/遗忘策略进入本批前另定。历史修订保留，不能为省上下文直接删除事实。
CAS失败明确冲突，不暗中覆盖；重复请求返回同一结果。
“以前喝咖啡现在不喝”可保留经历但当前偏好采用新事实；“以前说错了”有纠错来源；不同场景/不同Actor不能粗暴合并。时间新不等于自动真实。
只按内容digest不是语义去重；模型提案仍需subject/scope/evidence校验。
拟拆source_conversation_id与适用scope/authorized actors，旧数据保守映射保持原权限，不全局提升。Owner相同不等于角色之间所有私人记忆互通。
native memory提交结果落定前不能声称已记住；失败以确定性操作状态反馈，不默认第二次模型生成。
验收：修正后当前输入不含旧版本，历史可回顾，旧embedding/summary失效，索引失败不复活旧事实，重试/CAS/权限完整。

## B04 执行与行为质量

后台主动消息/动态由同次assessment产生可见文本与行动，治理/冻结后执行。工具依赖外部结果的复杂任务另定结果投递协议，不能用预先猜测当结果，也不能以此让所有聊天两次生成。
反思每角色一个dirty/due责任，事实推进目标watermark；执行按有界窗口处理，锁/CAS竞争作为延后，运行中新增事实保留dirty，完成再调度。不丢来源事实。
成功聊天在提交结果时结清恢复intent；dispatcher结清processed残留，只对未完成且lease可接管者启动。保持稳定消息/冻结结果和崩溃恢复。
行为场景：称呼/当前人格、长期回忆、纠错、未知承认、行动兑现、情绪连续、主动性节制。确定性应用测试与模型多样本评估分开。
计量每用户回合和每角色日的语义/Embedding/反思调用、workflow/空跑数、等待/费用；有证据再决定维护循环并行化、缓存或换模型。

## B05 目标/意图完整推进

详细后续设计见[目标推进方案](goals-progression.md)第4节：time/event/semantic触发分工、资格与生命周期、长期方向与可完成目标、寿命/延期/预算、失败重规划及恢复。A08先接通现有行动归属和结果写回，B05再实现正式到期/事件驱动。定时意图不能仅依赖quiet-period唤醒，用户持续聊天不应无限拖延已到期的事实。

## C 新功能

| 方向 | 最小闭环 | 未来需决定 |
| --- | --- | --- |
| 头像 | 独立avatar资产/版本，列表/聊天/动态一致，裁切/失败首字回退 | 来源顺序；旧视觉自动触发暂停、存量处理 |
| 群聊 | 用户+两个角色，conversationId入口，authorActorId显示，点名/有界顺序 | 自主插话、轮数/成本/停止条件，参与者授权 |
| 后代 | 父母来源、继承依据、独立人格、双向关系、幂等创建 | 双方参与、Owner激活、年龄/数量；不复制父母全部私人记忆 |
| UI | 错误/状态/作者归属先修 | 页面实看后再做全面视觉与布局 |

后代Owner权限与父母身份分开，创建走已有领域和capability边界。图生图继续暂停。
恢复媒体前处理远端已接受、本地未保存job ID的不确定窗口；检查上游真实去重/查找能力，不靠无限重试宣称exactly-once。
B/C各批进入实施时再独立任务化；父子关系不是依赖，必须写明上一批输出与进入条件。本轮不批量建子任务。
