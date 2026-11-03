# 设计：沿现有领域权威补齐完整生活闭环

## 架构与实施归属
这是一个承担代码与联合验收的集成修复任务；T00—T10 作为内部可验证切片，不把平台基础和业务闭环拆成分别宣称完成的子任务。旧任务只作为历史参考，不改其状态。主线程决定方案、修改代码和最终验证；子代理只探索/只读核验，遵守用户 AGENTS.md。

唯一运行链路保持：正式业务入口 → formal Agent 输入/Prompt Slot → Eino ChatModelAgent/Runner → ExecuteTool → 授权与领域校验 → 短事务内事件/状态/receipt/outbox → 真实 ToolResult → 后续原生决策。长动作由已有 durable intent/Temporal 恢复；accepted 表示真实受理，不表示完成。工具已经提交的事实不会因后续 Agent 失败而消失。Provider 与外部 I/O 不放进 DB 事务，不增加 Agent Loop 或通用工作流。

## 1. 时间与有效生活状态（T01/T03）
沿 App.Clock/App.now 扩展业务现在；投影固定 as_of，相关日程、睡眠、活动、获取、目标复盘用同一可控 Clock，提交前必要条件重查。审计记录使用真实运行钟，Temporal 工作流内部仍使用确定性 workflow 时钟，虚拟时钟只用于活动/领域测试和有明确身份的回放。SQL 生效判断传入业务 instant，审计 created_at 保持真实数据库时间，避免 App.Clock 和数据库 now() 分裂。

建立单一 instant formatter：固定 `2006-01-02T15:04:05.000-07:00`，UTC 显式 +00:00；输入接受 Z；API/Tool/结构化系统日志规范 UTC；运行视图统一到 actor_self 默认参考 IANA 时区并标 reference_timezone/as_of、双方本地时间与 unknown。用户原文不改，生日/每日钟点/时长保持各自类型。现有 timestamptz 与高精度排序字段保留，Actor 时区名另存。

重复本地日程遇 DST gap 拒绝并返回需改期，fold 默认较早 instant 且稳定实例身份防双执行；沿用已记录偏移，不重解释历史。跨午夜、改时区的未来触发器按已有版本化日程重新安排。

生活状态复用 Event > accepted Schedule > pending 权威，但计划获准、执行到期、当前有效、完成分别投影。睡眠资格由已授权的结构化日程/事件状态表达（在现有 schema 中补 sleep/interruptibility 语义字段若缺失），不按自然语言关键词/职业/钟点猜测。周期先 reconcile 到期合法状态再判睡眠；背景摘要不制造起床事件。消息发布、外出/购物等所属领域入口复核合法生活权限，独立 Tool 同样有效；不建工具名 switch 或 Main 门禁。真实用户消息沿既有打扰政策，在 T00 查清并回归，不暗增强制唤醒政策。

系统触发保留 trigger_source/target_actor，speaker 仅来自实际入站身份；补有界最近入站/主动发送/未回复/主题摘要。延续现有 cooldown 并增加同主题无新目的抑制配置，不固定每日数量。当前剩余/下一时段修改以可中断且未完成为边界，计划新版本与旧任务处置同事务；正在执行不可打断项明确拒绝，不能笼统禁止所有当前段调整。

## 2. Actor 事实与派生失效（T02/T08）
复用 actors、cognition claims、Memory source links 和 context generation。补最小属性级事实载体（优先扩展已有 claim；无法承载时加窄 actor_fact 表，不建知识图谱）：subject_actor_id、attribute/assertion、value、source kind/ref/revision、effective range、status、corrects/supersedes、revision。每次修正区别“原断言无证据/误判”与“过去真实但当前改变”。明确原始人类自述、初始化、成功事件与模型 inference 的证据资格，verified 来源可追溯不等于内容真。

模型在正式结构化入口提出事实/修正候选，服务端验证实际作者/主体/来源/范围/权限并提交；不 regex 扫聊天填事实。补小背景 Runtime Slot 与现有/最小 Actor 查询 Tool；详细事实按需查询。必要事实少量常驻：双方异地、用户在国外、所在地/时区未知、未确认见面。事实变更在同一事务推进 generation，按来源 links 失效冲突 Memory、Resident、summary/Developing Self 及缓存；历史查询明确原误解与纠正，原始消息留存。属性权威按领域划分，拒绝统一人格高于实际状态的规则。

T08 不是自动按相似文本推断错误：scoped repair 命令读取明确 Actor/source refs、时间范围和原因，dry-run 输出分类/计数；apply 用稳定 repair ID、revision/CAS、旧值日志写隔离/失效，rollback 仅在仍满足版本前提时恢复。未知来源归档可查但不作当前事实；真实起床与持有来源保留；不由图片/愿望补库存，不全库加减八小时。缓存/索引恢复使用现有 generation 机制。

## 3. 目标、获取与使用（T04/T05/T06）
Goal/Intention 是唯一共享持久目标，不建第二推进库。沿 intention.decide、intention.schedule、Reflection V2 的结果引用，补阶段、最近尝试/结果、下一步、阻碍/复盘与生命周期的最小字段/投影。检查正式 surfaces 后给相关 Agent 最小 Tool 集；不只向人格注入目标。关系目标不硬编码剧本/表白触发；送达、待回应、拒绝、合理等待、错失机会分别记真实结果。

需求描述不要求库存 ID。查询要带 completeness/失败类型；同一未完成需求通过稳定目标/规格/数量身份复用，重试身份不同于模型 call ID，补购可明确新建。目标关联 accepted schedule/action_plan；schedule 失败保留待安排，取消改期同步目标阶段。

正式获取继续使用 virtual_shopping + life.activity.start/advance/result Tool，不新增绕过获取入口。合法场景、到期、权限、状态版本在提交前重查。已在店内且条件满足可当期执行，不机械等待新周期；未来阶段不能在一个 Loop 人为推进时钟。终态交易原子写获取 Event、item(s)、来源、receipt/outcome 和目标完成；无 acquired item 的“逛完”不完成购买目标，失败不制造库存。稳定 item 来源键沿现有 actor/event/item member；套装成员整体原子成功，未有独立合法部分成功契约则不增加隐式部分成功。

普通物品沿现有获取/物品权威做最小增量，优先扩展 item kind 与允许 non-wearable item，服装继续用已有 wardrobe slot/worn 表。只在现有聚合无法表达时加必要物品/使用载体；不保留两份同物库存、不迁移成完整商城。非服装使用有独立持久领域命令和事件，不能强塞穿着槽位。对外可保留旧物品表名作为实现细节；实际迁移布局在 T00 全字段核对后定稿，不改变上述产品行为。

获取不改 worn/used。wear/use 校验 Actor 所属、存在/availability、组合/槽位、场景、CAS；初始明确穿着由受控初始化登记。人格切换/重启只读共享有效 body/worn，不恢复初始化服装。

## 4. 最终渲染边界（T06）
复用冻结的 appearance/body/wardrobe/generation 与真实 worn item 来源；当前自拍在受理时读取一致快照，锁定 snapshot/version。选择“保留受理拍摄时快照”策略，后续换装不拼入另一版本，完成可继续标 stale；不把后来变化等同受理快照非法。受理前版本冲突需重读或失败。缺关键服装数据明确返回 missing/conflict，不默认为无衣或自由补全。

最终 ComfyUI/model 请求服装/所用物品只由服务端快照投影构造，LLM 管动作/构图/表达；服装自由文本/参考图权限不覆盖快照。需在真实 media prompt/workflow 提交入口建立 typed 合并与校验，不能仅靠提示词或正则识别禁词。Canonical identity reference 用于身份；未验证服装不会通过参考图变成当前。生成/发布都不反写库存/穿着/身体，已有显式 preview 仍非事实，本轮不增加绕过入口。程序保证输入契约，像素还原只记录真实服务评价。

## 5. 累计摘要与每轮预算（T07）
复用 ConversationSummaryWorkflow/正式摘要 Agent、source digest/sequence 与 revision CAS。现有 episode 可留历史查询但不按时间追加常驻。每会话/Actor 范围一份 active bounded cumulative runtime summary：旧版本与 covered_through → 固定新增范围 → 分批模型生成/来源校验 → 原子提交新摘要/游标；编辑/撤回/纠正显式失效修订，生成或事务失败游标不动。

检查默认 5 分钟，配置范围 5—10 分钟；有新增且时间满足才总结，输入阈值可提前触发，保持完整 ToolCall/ToolResult 单元和有界最近尾部。摘要默认复用现有约 2048 rune 预算并配置 token 上限，由完整 wire budget 控制；配置变更不能取消模型安全限。无思考模式只在 assignment capability 支持时使用，不支持要诊断。摘要调用自身也分批计预算。

runtimeContextRefresh 继续同 Slot 替换，保留原生 ADK 历史角色。eino_model_runtime 每次 Generate/Stream 最后检查 messages、Tool schemas/results、response format、图片与输出预留。补完整分项与 tokenizer/count provenance；有匹配 tokenizer/模板才精确计数，否则 estimated+安全余量，Provider usage 并列。保护当前请求、纠正、有效状态与正在消费结果，required overflow 明确拒绝，不截坏协议。

## 6. 诊断全局排序与客户端（T09）
模型排序 `(created_at DESC NULLS LAST,id DESC)`；Agent 以稳定创建/发起字段（当前 agent_runs.started_at 与 termination event.created_at）归一 recorded_at 和含 source 的唯一键。后端 union/归一、排序后一次分页；不分别各 LIMIT 后合并。游标保留真实精度及同刻唯一键并绑定 filters/固定查询高水位，首屏 refresh 新快照、后续页沿原快照稳定读取，去重有稳定 identity。

Core/browser DTO、OpenAPI 与生成 clients 同步 page/cursor/snapshot 契约；Web 维持服务端顺序，逻辑组列表也按固定组发起时间排序，不能重新错误置顶。详情模型→Tool→结果保留 sequence 正序。用户选择诊断展示 IANA 时区，复用统一固定毫秒数字偏移 formatter，不按展示字符串比较瞬时。sleep no-op 和真实 error 分类分别呈现，已提交 Tool 与 budget/version 可查。

## 兼容、迁移与回滚
新增必要字段用 additive migration，先 empty→head/previous head→new head/repeat apply 验证；旧瞬时类型保留。历史来源不明只隔离不猜测。代码切换最终保留一条正式入口，旧 formatter/全量 history 注入/自由文本服装兜底等只在新 producer/consumer 接通后删除，无常驻双轨旗标。原始摘要/聊天/receipt 作为审计保留不代表两套 active runtime。

回滚精确本任务 commit/hunk，数据库 repair 通过审计/CAS 回滚；外部已发生资产/已提交合法获取不随代码回滚虚构消失。真实行为测试与图片服务有配置依赖，代码验证和 live 证据分开记录。执行中若必须改变产品范围或权威边界，回规划重新审阅。
