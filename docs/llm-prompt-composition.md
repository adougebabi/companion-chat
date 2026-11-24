# LLM 提示词组成清单

更新：2026-11-09。代码权威是 `apps/core-go/internal/core/formal_agents.go`
与各 typed task，规范在 [Prompt Composition Contract](../.ael/knowledge/spec/legacy-9b89c41b4be449b963dcba95.md)。

## 统一构造

所有正式 Agent 共享一个前置 `# 运行协议` system 边界。已有任务协议不叠加
第二套协议；相同规则去重；没有人格资料的分析任务不加入空人格段。
结构化业务数据统一 JSON，稳定任务资料可单独放在运行时事实之前；实际用户
输入保持原文一次。原生 Tools、ToolCall/ToolResult、response schema 和图片
保持原生结构。传输层不根据用户文本形状转换 JSON/YAML/TOON。`ComposeTaskMessages` 只对
调用方显式标记的 `promptTaskData` 进行资料提取/编码，Owner 的 JSON 描述仍是原文。

投影组装器在进程内记录每条消息的来源身份与已选片段引用；工具后刷新及请求
诊断使用该身份，不把用户正文中的 `[RUNTIME CONTEXT]` 字样当作来源。身份
不会进入模型消息。直接构造消息的旧入口尚未全部提供细粒度 Slot 来源；不能
把默认分类当作已经完成逐任务来源追踪。受控 HTTP 续轮回归检查了伪造标签的
历史与当前正文、真实状态刷新和 ToolCall/ToolResult 配对。
续轮物理请求的消息来源会使用刷新后的组装来源，未改变的历史保留原来源；
来源列表不会在诊断上下文中按 64 条静默截断。投影组装记录 Slot、来源引用
和实际选择原因；物理请求的 `prompt_slots` 补充当前工具目录、输出格式及
原生工具续轮，并在刷新时更新实际替换的部分。每次普通/流式上游重试的诊断
事件关联该快照，写入仍是尽力而为。`context_projection.selected` 仍是初次
组装记录，刷新后显式标记；未声明细粒度来源的旧入口标记为未知，尚不能
声称所有 Agent 的逐任务来源追踪已完成。
会话累计/片段/每日摘要、衣柜生成与选衣、NPC 相遇/动态回应和 Studio 文本任务
已在各自构造点声明输入来源；来源引用只进入诊断，不追加到模型正文。受控
物理请求与隔离数据库检查了片段、衣柜两阶段、NPC 相遇和 Studio 助手记录。
初始化、人格编译（含校验修复）、表达学习、动作一致性检查、日程生成/重排、
虚拟活动、Goal Planner、日记和世界事件也在各自任务入口声明来源。日程、Goal、
日记表达、人格编译和世界事件已通过受控请求及隔离数据库诊断回读。
媒体主体解析、规范化、头像与文本提示词、图片质量审核及视觉身份任务声明了
冻结意图、实际候选图/参考图或阶段资料的来源；规范化阶段只记录包中实际
存在的字段，不把缺失的质量反馈记为已提供。媒体规范化、图片审核和视觉身份工具的受控请求与隔离数据库
回归检查了来源记录和原生图片/工具结构，不代表真实生图或视觉质量已验收。
聊天认知、Writer、WakeUp、后台认知、Owner 生活请求、NPC 机会、日审、人格
切换、Reflection 和 Goal 评估的当前任务数据也分别声明来源；Goal 评估区分
稳定目标/原成功条件与本次状态、证据和已提交结果。聊天、WakeUp、Reflection
的隔离数据库请求已回读对应记录；Goal 评估真实入口的来源回读仍待验证，其
现有基线用例首次未进入模型的问题已单独登记，不能据此声称通过。
最终结构化纠错及发布遗漏纠错分别记录原模型候选和 Core 校验反馈；Writer 的
未发布草稿、真实动作回执与媒体状态分别记录。兼容聊天包装只标明消息来自
调用方，不从正文推断其业务来源。普通/流式受控请求已核对这些新增记录。
显式 direct 的媒体完成声明检查使用 `reply_action_check` 的
`direct_media_delivery_check_v1` 变体：只在候选与会话上下文涉及媒体交付时
提供未发布候选、当前输入、授权历史及同会话已提交交付的局部不透明引用。
检查任务只提取文本片段和候选引用，不执行工具；分享意愿、图片内容介绍以及明确的
文字消息发送动作不等于图片交付。Core 保留多分句原文中的否认/意愿限定，不能只取
限定词之前的首句；肯定的已交付声明仍需真实附件证据。校验调用保留实际父回合
`parent_run_id`，同源 `reply.media_delivery.checked` 记录候选、原提取、排除项、
拒绝片段与精确原因。可纠正拒绝通过原生工具结果返回片段和原因，不发布候选或伪造图片。
Core 独立核验引用、任务主体和
实际消息附件，并在发布事务重查。默认继承会话文本模型，显式 Agent/variant
覆盖仍生效；这不是对所有自然语言事实的完整识别保证。
动态正文也可复用该变体：检查输入区分当前选中的可读图片、当前会话内已
提交的私聊交付和可核对的历史动态图片；这些作用域不能相互代替。历史动态
证据要求动态附件、引用、已就绪图片和同源生成任务同时存在，生成意图不是
画面事实，动态创建时间也不是附件交付时间。Owner 选定素材只有实际可读
图片才能作为图片附件；音视频不能授权照片声明或保留自拍占位。选定清单、
素材类型/版本/摘要和被引用的历史交付在发布事务中重查。没有可靠意图的
上传素材保持未知，具体画面主体及真实模型语义仍未验收。
新建 direct 私聊 turn 的认知模型绑定在首次请求前保存在 Core 运行元数据中，
同一 turn 的恢复使用该绑定；旧 turn 无快照时记录原模型未知并按现行配置继续。
绑定、凭据引用与版本不进入模型提示词，真实凭据仍由服务端安全存储恢复。
私聊 Writer 的 Actor 背景按已知外壳移除 Core 引用，Actor 自有 value 原样
保留，包括嵌套 schema、业务引用、大整数和时间；受控物理请求已核对。
当前生活和外貌也按各自已知外壳投影；身体值与世界环境保留不透明业务数据，
衣物、活动记录的内部身份不进入 Writer。私聊和日记的已选 Working Persona
仅移除 Core 拥有的外层选择/来源字段，内部业务对象保留；受控请求和日记
恢复回归已核对。Moment Writer 的 Actor 背景也按相同外壳/业务值边界处理，
其他独立事实投影仍按各自来源检查。

每个任务按下表选择资料。统一构造并不表示所有任务共享整套摇光状态、记忆
和私聊；共享 Agent ID 的不同子任务仍各有独立资料范围。
逐 Agent/变体的必需与可选输入、禁止自动输入、来源、人格/历史/状态、工具及
输出消费方见 [Agent 与任务变体输入契约](llm-task-input-contracts.md)。

从 `assembleProjectionPromptForSurface` 构造的请求在进程内携带组装时选定的
Provider 绑定；FormalAgent 运行时复用该快照，若调用方同时给出不同的冻结绑定
则报冲突，不静默换模型。快照字段不序列化到 Prompt 或普通 Trace。快照归属
具体 Agent/variant，嵌套 Agent 不因逻辑用途相同而继承父任务模型；显式视觉
绑定和同一任务纠错继续使用各自快照。此机制并不代表所有后台任务的跨进程
恢复已经实现。正式请求的 `context_projection` 诊断增加 Agent ID 与任务
变体：默认使用 schema 名，`conversation_reply` 区分私聊、主动消息、动态及
修复；它们是请求来源信息，不注入模型正文。受控物理请求测试已
核对这两个字段和凭据不进入诊断，不等于完整 Trace 或真实模型验收。
ADK 上游重试的每次物理请求另记录模型和配置版本事件；事件不含密钥，
按现行诊断规则异步尽力写入，不阻断模型请求。
私聊、WakeUp、原生认知、日审、持久切换、反思、Goal 评估、NPC 相遇机会和
Owner 生活请求的生产调用会将完整组装结果传入 Runner；工具后刷新继续使用同一
进程内绑定。其他自行构造消息的任务仍沿各自既有绑定和恢复规则。
新的私聊回复草稿会分别保存认知模型和 Writer 的无密钥绑定；等待媒体后的
认知续接使用原认知绑定，Writer 继续使用自己的绑定。旧草稿没有可证明的
认知快照时保留原选模行为并记录诊断，WakeUp 不冒充私聊认知绑定。
新媒体 intent 在首次规范化请求前保存不含密钥的 Agent/variant 绑定，同次
纠错和跨进程重载复用该绑定；新的 intent 使用当时配置。旧 intent 没有可
证明的快照时暂沿用既有选模行为，并写入 `legacy_unfrozen` 诊断，仍待补齐
可靠恢复策略。普通文本、头像提示词及视觉质量审核也在首个请求前保存
各自的任务级绑定，重载后沿用；质量审核继续逐次检查视觉开关，并以本次
审核时的设置版本作为提交围栏，不能误用首次选模时的旧设置版本。快照损坏
或恢复失败不能降级为跳过审核并交付。该快照不进入媒体诊断、导出或恢复历史。
隔离 PostgreSQL 与受控 Provider 已检查这些阶段的重载、开关和失败边界，
不代表真实媒体或模型验收。
人格编译在同一次 `persona_compilation_response` 任务的语义纠错中复用首次
模型、端点和采样绑定；新的人格准备 intent 在首次需要模型时保存无密钥快照，
其多个 profile 和自动重试复用该绑定。Owner 显式恢复创建的新 intent 可选用
当时配置，嵌套调用不会继承其他 Agent 的冻结绑定；旧 intent 没有可证明快照
时暂保留原行为并记录诊断。分配失败仍记录 preflight 诊断。受控 Provider 与
隔离 PostgreSQL 回归覆盖配置变更、并发选择及错误边界，不代表真实 Provider
已验收。
新 Goal 评估请求首次调用前将不含密钥的模型绑定保存到请求，后续 claim
与部分目标已提交后的恢复使用同一绑定；新请求使用当前配置。快照缺失或
损坏时停止，升级前旧请求缺少可证明绑定则沿旧行为并记录未冻结诊断。
隔离 PostgreSQL 和受控 Provider 已验证部分提交、并发选择与进程重建；
这不代表真实 Provider 验收。
新每日记忆任务按来源代次保存无密钥的模型绑定，同代自动重试和服务重建
沿用首次选择；重开后的新代使用当时配置。模型快照单独保存，不修改
Temporal 调度比较的任务内容；旧代活动不能结算新代。升级前未冻结的旧代
保留既有选模行为并记录诊断。隔离 PostgreSQL 和受控 Provider 已检查恢复、
并发选模和纠错屏障；真实 Temporal 及真实模型尚未验收。
新对话片段摘要按来源代次冻结无密钥模型绑定，同代重试复用，新代重新
选配置。旧代缺少可证明快照时保留原有调用后配置一致性校验并记录诊断；
快照缺失或损坏不会静默重新选择。隔离 PostgreSQL 与受控 Provider 已检查
并发选择、重试和纠错重建；真实模型效果尚未验收。
新的累计会话摘要也在首次请求前保存无密钥模型绑定，同一任务失败重试和
进程恢复复用；旧任务沿用原选模及调用后校验并记录诊断。隔离数据库验证
了事务失败重试、启动恢复和新任务选用当前模型。
衣柜准备在首次模型请求前将无密钥绑定保存到服务端的准备记录；库存生成、
选衣及重启恢复复用同一绑定，Owner 回读不暴露该内部快照。旧任务没有可核实
快照时保留已完成步骤并明确失败，Owner 显式恢复后可为剩余阶段选用当前
配置；已有换衣回执优先重放。隔离 PostgreSQL 和受控 Provider 已覆盖这些
恢复边界及重复迁移，不代表真实模型选衣效果已验收。
新视觉身份会话分别保存文本决策和视觉审核的模型绑定，跨候选和重启恢复
沿用相应绑定。视觉审核仍读取本次开关和设置版本，并在提交时复核当前版本；
未配置视觉模型时继续保留候选供人工审核。旧会话没有可证明快照，暂沿原
选模行为并记录未冻结诊断。隔离 PostgreSQL、受控 Provider 和本机图片存储
已覆盖这些路径；不代表真实视觉模型已验收。
Diary 的素材选择与正文表达各自作为独立阶段，在首次物理请求前将无密钥的
模型绑定保存在原 attempt 的服务端 provenance；同阶段恢复使用原绑定并按原
凭据用途重取密钥。显式新 attempt 对未完成阶段重新选配置，已完成阶段继续
复用；升级前已运行且没有快照的阶段明确失败、保留材料并允许显式重试。
普通日记读取不返回内部绑定或凭据用途。两种阶段的受控物理请求及隔离
PostgreSQL 恢复用例已检查，真实 Provider 仍未验收。
Studio 普通助手、文字预览及角色定义准备同样在各自模型阶段首次请求前保存
服务端绑定，自动重试保留该绑定，显式新 attempt 对未完成阶段可重新选配置；
已保存的有效输出继续复用。升级前 running 且无快照/输出的任务明确失败并
保留草稿，供显式重试。Studio 图片预览在确实需要 `media_subject_parse` 模型
回退时也保存该阶段绑定；KEV 已能完成规划时不额外要求模型。后续独立媒体
工作流的跨进程绑定仍需分别验证。四种阶段的受控请求及部分隔离数据库恢复
用例已通过，真实 Provider 尚未验收。
经 `ProviderClient.assignment` 解析的绑定、连接和凭据版本在同一数据库读取中
取得；该路径的物理请求诊断记录可获得的版本与非敏感采样值，不写入凭据。旧快照没有
来源版本时标为未知；新草稿冻结服务端凭据用途引用，恢复时重新读取该用途及其
当前凭据版本。旧草稿缺少用途引用且端点地址已变化时明确失败，不向旧地址发送
新连接的凭据。数据库版本读写和跨进程恢复仍需隔离 PostgreSQL 回归。

正式 Agent 的文本绑定按 variant → Agent → 默认用途解析，保存在现有
`model_roles`；普通聊天认知、WakeUp、NPC 和新增表达候选提取默认继承 `conversation_reply`，
其余尚未验证的内部任务保留现有默认。视觉输入和 embedding 不由文本覆盖
接管。实际 React 管理页可提交覆盖和恢复继承；受控页面及隔离数据库验证了
配置保存、读取和任务隔离，本机 MeroMero 普通聊天也已实际调用。真实 Qwen
任务覆盖和全部旧任务恢复仍未验收。Writer 的显式任务
覆盖保留各自的物理请求超时；私聊、主动私聊和动态仍受现有
`conversation.reply` 总时限约束，日记与 Studio 使用各自的工作流时限。
温度与 Top P 可从同一模型绑定读取；Writer 对未配置字段分别回退到原全局
回复设置。受控物理请求已检查采样参数，数据库迁移和真实配置回读仍未验收。

## 记忆与历史数量

表中 P 为该任务协议，D 为所需生活事实，M 为记忆，S 为累计摘要，H 为近期对话。

- D：按任务选择主体/时间/当前身体与穿着/情绪/场景/活动/地点/关系/目标/日程/
  近期结果；当前事实不被人格偏好或旧聊天覆盖。
- M：活动记忆查询最多 30 条；长期检索私聊 6 条、WakeUp/DailyReview 8 条、
  其他操作 12 条；常驻记忆来自已发布且有效的快照。这些是既有语义查询契约，
  没有改成 token 配额。
- S：0 或 1 条已持久化累计摘要，附内部源覆盖范围；摘要文本产品长度设置保持
  原样。只有实际选入且有效的覆盖才能代替原始消息。
- H：有会话时通常读取最近 200 条候选，必要时回补窗口前真实触发 user；排除
  当前输入和已覆盖完整交互组。最终数量可变，并非固定 10 句。私聊保护最新
  两个完整交互组，不表示只发送两组。正文/顺序/角色/发送时间保持。
- 同来源不同陈述不合并；半组摘要覆盖保留整个交互组。WakeUp/Moment 的 H
  是 `historical_conversation` 数据，不模拟当前用户发来旧请求。
- `memory.recall` 的原始会话记录回执包含消息类型、实际发言者与发生时间；
  使用角色自身、当前说话者、Owner、会话内稳定匿名标签或 unknown，不暴露
  数据库 Actor ID，也不把动态 Actor 别名误认为跨续轮稳定身份。发言只证明
  说过，不能单凭原话推定外部事实成立。
- 表中“0”表示不追加通用私聊；Goal、日记、摘要的显式证据可包含源消息。
- Reflection 的 `conversation.turn` 来源消息以完整原文进入模型请求，不按字符
  截断或去除首尾空白；来源引用仍指向同一条消息。反思结果中的证据展示快照
  目前仍有独立长度限制，不能把该快照当作完整来源原文。

## 31 个正式 Agent

表内文件均在 `apps/core-go/internal/core/`；这些文件与函数可用于定位具体入口。

| Agent ID | 当前提示词组成 | 通用原始聊天 | 入口/构造位置 |
| --- | --- | --- | --- |
| world_event_generation | 世界规则/时区/真实地点与 NPC/实际在场角色/近期已发生事件；只提议或 no_event，独立 Kev 审批后才可入可执行库 | 0 | world_event_generation.go |
| conversation_cognition | 私聊认知协议 + Working Persona + 私聊 D/M/S + H + 当前输入；legacy 与 fact-only 投影不同 | H | cognition_agent.go；conversation_runtime.go |
| conversation_reply | 用户提供的 v8 私聊运行协议 + Working Persona + 独立事实视图/能力边界/实际 Tool facts + M/S + H + 当前输入 | H | reply_context.go；reply_provider.go |
| reply_action_check | 换装未生效时核验未发送正文；实际动作回执 + 当前状态 + 候选正文，无人格扮演，无工具 | 0 | reply_action_consistency.go |
| expression_learning | Owner 显式选择的已发送 human 消息原文和局部来源引用；仅提议连续原文摘录及适用语境，无人格、关系、生活状态或工具。Core 校验来源身份、版本、引用及摘录并持久化待审核候选；该任务不能自行启用候选 | 仅调用方显式提供的消息 | expression_learning_extract.go；expression_learning_task.go |
| wake_up | 专属完整唤醒协议 + Working Persona + D/M/S + 历史资料 H + 本次唤醒事件 | H，作为历史数据 | agent_result_adapter.go；capability_prompt_policy.go |
| takeover_judge | Judge 协议 + 本轮 user/candidate/接管规则/控制与关系事实 + 可选 recent references | 无全量 H/M | turn_takeover.go；conversation_runtime.go |
| takeover_reply | 接管任务规则 + caller 提供的选定人格、上下文及 messages + 授权 Tools | 取决于 caller | conversation_runtime.go |
| wardrobe_preparation | 分阶段准备/选衣规则 + 穿搭所需身份/性格/偏好 + 本地时间/当前情境/实际身体与穿戴引用 + 唯一衣物目录/方案成员引用 + 按需选衣纠错反馈 selection_feedback | 0 | wardrobe_preparation.go；wardrobe_preparation_prompt.go |
| initialization | 初始化解析协议（显式完整姓名包含前缀/连字符，不拆为昵称） + Owner 描述一次 + 预览字段、类型、输出结构和按 schema 生成的数组路径/元素说明；自然语言分析先恢复格式错误的数组并保留原值，再校正明确姓名，JSON 导入继续使用原校验 | 0 | app.go；model_tasks.go；initialization_name.go；initialization_array_recovery.go |
| persona_compilation | 人格编译协议 + 选定 profile 和共享身份；必要时校验反馈 | 0 | persona_compilation.go |
| media_prompt | 媒体任务规则 + 冻结意图/主体/拍摄方案/视觉事实和目标尺寸；`media_subject_parse` 与 `media_prompt_normalized` 使用各自 JSON Schema 作为严格 Provider `response_format` 并返回结构化字段；最终渲染提示词与头像构思仍使用文本输出。头像子任务使用冻结角色设定、已确认外貌和视觉偏好，由角色自主选择包括非人物在内的主体与构图，输出英文生图提示词；重试附此前 prompt 和质量反馈 | 0 | model_tasks.go；generated_avatar.go；media_capture_plan.go；media_prompt_branches.go |
| media_quality | 质量检查协议 + 冻结媒体概念/实际 renderer prompt/重试情况 + 图片；角色设定图和头像的检查同时携带已确认参考图与待审候选图，头像仅在选择本人出镜时比较本人外貌，只有真实通过才自动接受 | 0 | media_quality.go；model_tasks.go |
| visual_identity_vision | 视觉检查协议 + identity snapshot/视图要求/renderer 约束 + 图片 | 0 | model_tasks.go |
| visual_identity_patch | 修正决策协议 + candidate/snapshot/constraints/vision findings | 0 | model_tasks.go |
| visual_identity | 视觉身份流程规则 + 身份快照/任务状态/尝试历史/前次审查的真实观察、评分与修正建议 + 必要图片；参考图与设定图共用角色卡模板，头像由独立 media_prompt 子任务自主选择主体，本人出镜时保留已确认外貌，始终要求适合圆裁及无文字。阶段落库后撤下已执行工具并按真实状态收尾 | 0；尝试历史是任务历史 | visual_identity_agent.go；visual_identity.go；generated_avatar.go |
| conversation_summary | 累计：旧摘要+Actor 更正+源消息；片段：真实参与者/说话人别名与名称+时间范围+源消息，显式 JSON 合同；daily：日期/时区+segments | 指定源范围，非通用 H | conversation_summary.go；model_tasks.go |
| schedule_generation | 排程规则 + 含已声明住所的 identity/life/goals/intentions/outcomes/承诺 + 共享世界规则/时区/真实地点候选 | 0 | model_tasks.go；schedule_places.go |
| native_cognition | 认知协议 + Working Persona + D/M + 本次事件/事实 | 0，无 S | model_tasks.go |
| studio_assistant | 定义编辑协议 + 隔离的定义 body/创作要求/允许引用（移除 job 身份） | 0，无生活 M | studio_runtime.go |
| studio_text_preview | 虚构示例协议 + 隔离的定义/请求 snapshot | 0，无生活 M | studio_runtime.go |
| npc_response | NPC 规则 + NPC 私有定义/world/机会/公开来意；动态评论分支为已接受遇见+公开 post | 0 | world_encounter_response.go；moment_npc_interactions.go |
| daily_review | 日复核规则 + Working Persona + D/M + local_date，不自动附加私聊 | 0 | model_tasks.go |
| diary_material | 选材规则 + 本地日期/时区 + source_window | 0；材料可含当天消息证据 | diary_runtime.go |
| diary_expression | 日记表达协议 + Working Persona + 日期/时区/as_of + 选定真实材料 | 0 | diary_runtime.go |
| persistent_switch | 切换判定规则 + Working Persona/切换规则 + D/M/S/H + user_text/candidate_reply/response_intent | 可带 H | model_tasks.go |
| reflection | 反思专属规则 + Working Persona + 按需状态/记忆 + 明确范围的 evidence；记忆候选依据来源窗口，不附加公共当前穿着或进行中的购物活动 | 0，无 S；来源材料本身提及的衣着仍保留 | model_tasks.go |
| schedule_replan | 重排规则 + 冻结 intent + 当前日程/生活事实/已声明住所 + 共享世界规则/时区/地点候选 | 0 | model_tasks.go；schedule_capability.go |
| goal_planner | Planner 协议 + run/mode + goal-set snapshot；后续 query/commit 原生结果 | 0 | goal_planner.go |
| goal_evaluation | 评估协议 + Working Persona + stable 目标/criteria/actor refs + 专属事实/记忆/其他目标背景 + 当前目标状态/sources/reviews/accepted submissions；重复正文通过 evidence_payloads 复用 | 0；sources 包含消息证据 | goal_evaluation_task.go；goal_evaluation_wire.go |
| virtual_activity_result | 活动结果规则 + 请求/当前 appearance/life/近期结果/开始和最早结束时间 | 0 | model_tasks.go |

Goal Planner 初始快照保留已选目标的完整 desired_outcome/motivation，以及已选
稳定人格字段的完整 JSON；按状态保留多少个目标的既有查询范围和后续按目标
查询详情的协议不变。完整性回归不代表真实模型已通过 Goal 验收。
Goal 评估的人格投影只去除已知的人格选择与来源外壳；画像文本、旧分类事实和
其中看似 JSON 的业务内容保持原值。受控 Provider 请求覆盖该边界，尚不代表
真实模型或 Goal 评估生产入口已通过验收。
通用运行上下文按已知 Goal 执行记录字段投影，原始成功条件及其业务值保留；
WakeUp 不携带上次评估正文。受控 WakeUp 请求验证了原始条件及旧评估边界。
Moment Writer 同样保留原始目标条件，但不接收旧执行计划、评估或 Goal 执行规则；
该边界已用受控 Writer 请求核对。
Moment 选中的 Actor 事实、驱动、偏好和触发偏好分别去除已知 Core 外壳并保留
业务值；Goal 由独立目标视图提供，避免重复注入。选中的 Active Memory 保留内容、
状态、时间和可信度等字段，Core 中的 `actor_refs` 仅在当前投影中映射为人物别名
或未知人物，并明确标为关联人物，不把关联者断言为记忆内容主体。真实 Moment
准备路径在隔离 PostgreSQL 和受控 Writer 下验证引用核对与物理请求；这不代表真实
模型已验收。选中的外貌证据保留身体字段的状态和业务值，以及当前穿戴语义。
伤情数据仅在同角色已确认身体事件与具体字段相符时移除 Core 事件标识，
Owner JSON、旧字符串和无法证明属于 Core 的字段保持原值。事件查证在 Core
证据准备阶段完成。已选场景/生活、日程、在场、情绪和生成人格变化证据按
各自生产快照构造声明字段视图，保留来源、状态及业务 JSON。世界设定、规则、
氛围、效果值和日程调整策略保留业务原值；World/Place/NPC 等 Core 身份通过
已知外壳隔离或人物别名表达。Outcome 复用真实结果状态视图；ScheduleItem
的执行计划与 AffectProfile 的数值 reducer 策略留在 Core。ToolFacts 的后续
组装保留已经构造的类型视图。
Moment 默认运行背景同样按场景、状态、日程、人物和自我描述的声明字段构造，
其业务值使用原值副本。默认外貌在 Core 准备阶段复用身体事件来源核对；已选
同一当前外貌证据时只保留已选的一份。Moment Writer 的通用递归删键函数已移除。
日程生成和重排按各自声明的身份、生活、状态与行动字段投影；偏好、身体值、
行动计划及结果中的嵌套业务数据按原值保留，不再按字段名后缀递归删除。已知
Core 协调字段只在对应结果外层移除；真实地点和计划引用仍按原领域契约绑定。

私聊 `direct` 模式使用同一个 `conversation_cognition` Agent 和原生
`conversation.reply`：主 Agent 的工具参数同时包含自拟正文和该轮 decision。
运行协议保留人格差异、自然表达、可信来源和真实工具结果边界；decision
按既有认知 schema 与当前 Core 引用验证。当前关系可带由 Core 签发、绑定会话
作用域的不透明引用，供该轮 influence 使用；原始关系数据库标识不进入请求。
正文、工具回执和认知结算在同一
数据库事务提交。未显式配置回复模式时，新私聊默认选择 `direct`；已保存的
`fact_only`、`legacy` 和 `direct` 仍保留，用户可在设置中切换。旧草稿、WakeUp、
Moment 和独立工具沿各自原合同。受控 Provider 的隔离 PostgreSQL 测试覆盖
默认模型、显式变体覆盖、模式切换、提交、先执行工具再回复、重放和结算失败
回滚；旧草稿各状态选择旧合同且 ready 草稿实际回读。2026-11-12 本机真实
MeroMero 默认私聊出现三次工具参数无法解析，停止且没有发布；该真实场景
尚未通过，详情见本轮实施台账。

`direct` 主 Agent 若在同一原生模型响应中同时发出图片生成与回复调用，Core
分别返回未执行的拒绝回执，主 Agent 可基于回执自行决定下一步。该保护不检查
自由文本的事实语义；连续三次无其他结果地重复该组合会终止，不伪造回复。
无媒体调用时虚假完成声明的已知复现已有局部修复；跨轮媒体引用的真实模型
核验仍有失败，不能将上述保护视为所有表达均已验证。

Direct 私聊有授权的近期对话、且图片能力启用时，`direct_image_discovery_v1`
用同一会话的角色、对话原文和当前输入判断是否需要查图片能力。输出只包含能力名或
`none`，没有工具。Go 根据该结果组织原生循环。普通聊天仍可直接回复。
主循环首次仍提供 `capability.catalog`、`conversation.reply`、`moment.publish`
中的启用项，目录排在首位。模型选中图片发现时，第一次原生请求聚焦于当前输入与
近期对话。若先发了其他调用，Core 返回未执行的目录提示，由模型发出只读查询。
收到目录回执后恢复完整私聊上下文，由模型决定是否拍摄。
目录回执给出已加载的原生工具。图片工具的模型 schema 只要求 `intent`，
其他参数仍可提供，原有权限及入参校验继续生效。图片任务一旦受理，模型用
`conversation.reply` 根据回执告知已提交。附件交付仍沿用现有流程。
场景回归还核对普通对话、无副作用的续接及同一回合的幂等重放。

Direct 最终回复的工具参数收到两次校正反馈后，若仍有形状不合法的独立可选
认知标注，Core 可省略该标注，再按原契约验证整次回复。正文和其余有效字段
保持原值，必需字段、事实主张、证据、偏好和目标提议继续执行原有校验。
图片任务受理后，若模型明确停止且没有回复，沿用一次只提供私聊回复工具的
发布修正请求，保留实际生图回执。

原生 `media.image.generate` 可选的 `delivery_target=current_moment` 表示为
本次任务先前已完成的 `moment.publish` 请求配图；Core 从真实工具回执绑定
动态并再次核验所有权，不向模型暴露或接收内部动态 ID。同一模型响应中的
调用不能依赖彼此的结果，缺少先前完成回执时不创建图片任务。省略目标时
沿用原请求作用域；图片受理或生成中不等于已交付，最终仍走既有媒体发布。

`direct` 的原生 `conversation.reply` 工具对 18 个认知字段补充各自的语义和类型
说明；已有认知 schema、引用验证、事务与发布条件不变。此前模型侧 schema
将原本可选的 15 个字段标为必填但可为 `null`；现在该工具的 `decision` 只要求
三个业务必需字段，其余字段仍有原生 schema，可省略或为 `null`。后端将可选
字段的 `null` 视为缺省，有值时仍按原类型及业务规则校验；非法数组不会静默
转换成 `null`。此例外通过工具定义显式指定，其他工具和最终结构化输出仍用
原有严格转换。受控物理请求验证工具 schema、模型输入和无输出 token 上限。
本机合成对照的一次可选字段输出通过参数校验，但正式私聊一次没有可用工具
调用，后续复测又遇 GPU 可用内存不足，尚未通过端到端验收。

已审核的表达候选在 fact-only Writer 和 `direct` 主 Agent 路径均按来源
会话和当前输入触发词选择；传给模型的只有原文摘录和适用情境，并标记为非
指令资料。`direct` 在本轮首次请求前冻结引用，以 user 级 JSON 随工具续接
保留；Core 在发布事务中重新校验候选、来源版本与使用开关。参考失效时保留
真实失败回执并停止当前轮，已执行的独立工具效果不回滚或重做。隔离 PostgreSQL
与受控 Provider 已覆盖选择、撤销、来源/开关往返、先行动再回复、取消和
已发布后的重放；真实 MeroMero 的完整私聊和表达使用仍未验收。
表达参考的称呼须结合当前对象的真实关系、已有称呼及当前人格自主选择，
原文不形成恋爱、亲属或共同经历证明。fact-only 认知和 Writer 视图保留正式
`role.addressing.preferred/self_reference`，沿用当前人格与当前对象过滤；
表达候选本身仍只提供摘录和适用情境。

WakeUp 协议区分未回应的同一话题与真实的新事件：前者没有新进展时不追问，
后者可成为自主分享机会，但不强制联系；明确拒绝、自治权限、安静时段、主题
冷却与实际 Tool 回执仍由原流程约束。受控物理请求只核对协议传输，尚不能
证明真实模型的主动或静默选择。

空白人格首版由 `working_persona_persistence.go` 本地合成。共享特质被合并到
profile 后，完全相同的输出行仅保留一次；同名但不同取值的事实均保留，原始
Core Persona 不改写。`r4_remaining_regression_test.go` 覆盖该去重边界。

初始化使用 `json_object` 输出格式；该请求不向模型发送完整 JSON Schema，因此
初始化协议中的规范 JSON 形状显式列出第三步的身份、共同性格、表达和生活资料
字段。外貌说明保留描述原文；共同性格与 0–1 表达参数使用数字，回应延迟为
秒，文本表达使用字符串。描述未支持的字段可省略或为 null，不能为了填满预览
编造事实。物理请求还根据初始化 schema 列出固定数组路径、元素类型、对象必需
字段及允许对象或数组的联合类型；关系种子与初始关系都要求明确指向用户，不把
普通生活事实转换成关系。模型仍返回不匹配的数组时，仅在自然语言模型分析路径
按声明的字段恢复：保留合法内容，将无法安全采用的原值和提示放入可审阅扩展；
目标位置、衣物编号和人格引用不能因过滤错误项而静默指向其他内容。用户直接
导入 JSON 和激活时继续执行既有校验。模型漏掉字段时，
Core 延续空值和空容器兜底，让用户在预览中补充。初始衣物的规范位置是
`core_persona.life_profile.appearance.wardrobe_items`；预览编辑此来源并保留
已有草稿的原始结构。模型将衣物数组放在 `life_profile.wardrobe_items` 时，
Core 仅在规范位置缺省或为 null 时移入该位置；两处均有值时保留规范值，并把
上一级的冲突值放在可审阅扩展中。`TestInitializationPreviewPhysicalRequestAndMissingFields`
核对实际 Provider HTTP 请求及缺省、零和 false 的保留，另有别名和冲突测试；
受控响应不等于真实模型的语义提取质量。

`takeover_judge`、`takeover_reply` 是已注册兼容入口，本轮没有发现非测试生产
调用方；保留注册和构造覆盖，不据此宣称正在生产运行，也不删除。

`conversation_reply` 另有 Moments 分支：Moment 协议 + Working Persona +
生活/关系/目标/能力/真实工具事实 + 检索记忆/S + 按既有会话范围取得的 H。
H 现在明确放在历史数据块。Moment 不是私聊续写，不能复制父 Agent 草稿或
把私聊全文作为公开动态。默认关系上下文和已选关系证据以关系业务值和语义人物
呈现，Core 关系外壳和身份留在 Core；关系业务对象内的字段按原值保留，不按名称
递归删键。若已选证据与默认关系是同一 Core 引用及快照，只保留已选证据一份；
无效引用不移除默认关系，不同关系即使内容相同也分别保留。

私聊 Writer 输出通过 `parseReplyDraft` / `extractReplyBody` 提取正文，
`ToolPublicationService.PublishConversationReplyTx` 对原生 Tool 和准备草稿再次
使用同一解析器。中英文回复段、重复段、围栏和缺失段均不能把认知、事务记录、
时间或工具元数据写成用户可见消息。JSON 畸形、额外外壳字段或标题标记不完整时，
回退到清理后的可恢复正文并正常发布；不能直接保存原始外壳，也不能提取嵌套私有
字段。清理后确实没有正文才拒绝发布，媒体参数及原生工具授权仍按原规则校验。
原始候选只保留在
既有私有诊断路径。该防线不改变人格、输入协议或 token 设置，
`reply_body_integration_test.go` 覆盖真实 Provider HTTP 传输与 PostgreSQL 发布。

独立 Writer 的默认协议为 `fact-only-reply.v8`：运行协议采用用户提供的聊天规则，
强调以当前人格本人自然交流、区分聊天与现实同处、尊重真实状态与明确拒绝、
自主推进而不虚构结果、按需调用图片工具，以及只输出实际聊天正文、括号动作
或必要工具调用。提示词正文由 `reply_expression.go` 按段落保存；system renderer
提供唯一的“运行协议”标题。普通文字闲聊不调用图片工具，工具受理不代表图片
已生成或发送；未理解图片时不声称看过，外貌细节经 `persona.detail` 查询。
v4/v5/v6/v7 的原协议、hash、模型可见工具描述和冻结草稿恢复保持原样；设置页
保留旧版本选项。已经显式保存旧表达版本的设置不会自动升级。
聊天正文解析后的统一输出清洗 `stripReplyMarkdownHeadings` 去掉行首 1–6 个
`#` 加空白（或空标题）的 ATX 标记与合法尾标记，保留标题文字。普通正文、
话题标签、行内代码、缩进代码及反引号/波浪线围栏内的内容保持原样；围栏
按实际标记类型与长度闭合，不因代码内较短或异类围栏提前结束。动态场景
沿用其原有标题规则；聊天 Writer、原生 Tool、准备草稿及重复回复投影均清洗。
`reply_markdown_test.go` 覆盖清洗与共享解析路径；`reply_identity_test.go`
核对空白/已定义人格的 system 约束、实际 Provider HTTP 请求、无输出 token
上限及 v4–v7 旧协议 hash，并验证 v8 用户协议到真实 Provider HTTP 请求的传递。
受控 Provider 验证传输和解码，不代表真实模型行为验收。

WakeUp 主动私聊没有新的用户消息：Writer 在任务规则中明确这是主动发言，
当前输入为标明 `has_new_user_message: false` 的结构化任务事件，不再将内部
指令或历史消息当作用户的新话。对话历史仍按原角色和正文保留，即使旧消息内容
恰好与任务事件相同也不能被当成当前输入去重；普通私聊的用户输入保持原文一次。
主动私聊把时间、位置、状态和关系视为内部背景。高置信度的状态附录、自称 AI、
角色自身第三人称场景或复述内部回复规则，使用既有同模型、无工具、最多一次的
未提交正文修复；普通第三人称、引用、规则话题和明确要求的创作仍保留。
任务规则不改写旧版 Writer 协议及持久草稿 hash。头像的 `media_prompt`
子任务只接收冻结的角色设定、当前活动人格、已确认外貌、视觉参考和实际重试反馈，
由角色选择头像内容与画风；旧队列在首次构思前补冻角色资料，已缓存的生图提示词
沿用，模型失败不伪造固定模板结果。
`TestProactiveReplySeparatesTaskEventOnPhysicalWriterRequest` 和
`TestProactiveReplyRepairsProtocolSelfReferenceOnPhysicalWriterRequest` 验证实际
受控请求中的任务事件、历史保留以及未提交正文修复；它们不证明真实模型的输出质量。
`TestProactiveReplyRepairsPrivateAppendixOnPhysicalWriterRequest` 与
`TestGeneratedAvatarUsesCharacterOwnedPromptOnPhysicalRequest` 核对实际
受控请求；它们不能证明真实模型输出或图像质量。

动态发布在 `publishInvocationMomentTx` 复用同一正文解析器，识别“发布动态”
协议标题并保留普通正文回退；没有实际附件时删除 `[自拍照：…]` 占位文本，
清理后没有正文则拒绝发布。用户建议携带的素材仍须通过原有权限和可读取校验，
已提交请求继续返回原正文。`moment_body_integration_test.go` 覆盖原生工具、
独立 Writer、用户建议、实际附件关联与重复请求。

R7 换装回归明确阶段职责：fact-only `conversation_cognition` 自主决定是否接受
本人生活动作；接受且可执行时先发现、查询和执行，再交 `conversation.reply`。
未知能力名先以 `{"names":[]}` 查询 catalog，不能猜名字；模型侧 strict schema
要求提供 `names`，直接调用仍兼容 `{}`。目录加载和库存查询不算执行。
Image/Moment 的 Owner 停用状态独立于 Kev 推荐：目录可报告已安装但停用，
不能加载其新工作 schema；当前进程内已准入运行沿用其快照，持久回执先回放。
点名查询时，目录以完整安装注册表和当前运行范围区分未安装、已安装但无权、
已停用；没有完整安装信息时报告未知。正式 Agent 冻结本次允许的工具集合，
未授权或跨场景调用先返回无副作用的拒绝回执，不暴露被拒工具的参数要求。
不使用 Kev 的阶段 Agent 也遵守该集合；有效的已提交回执仍按现有幂等路径重放。
独立目录调用可以查询目录工具本身及同类工具，但不会创建 Agent 工具窗口。
运行中的空目录查询另报告当前已加载的披露窗口；该窗口位于物理请求策略之前，
不替代后续业务权限或状态校验；设置不可读取时按实际回退状态或未知状态报告。
Image 停用时 Writer 的 capability facts 标记不可用且物理请求不含私有图片工具；
目录与 Writer 的受控请求回归见 `domain_plugins_test.go`，不代表真实模型已验证。
Writer 的当前事实视图增加 `action_execution_boundary`：当前为表达阶段、
其工具不是角色完整目录、文字聊天不禁用本人生活动作、叙述不能改写状态。
当前人格原文、旧 Writer protocol/hash 和原生工具授权仍使用原契约。
Owner 换装子任务即使目标看似已满足，也由真实 wear Tool 返回 no_change；
仅有 accepted 文本或目录回执不能结算为换装成功。
私聊 Writer 的 `tool_facts.current_action_result` 还直接读取本轮实际原生动作
回执，不依赖父模型选填 evidence_refs。按 owner/角色/会话/source 配对，
保留 handled 的 warning_code、disposition 与 effect_applied/mutation_committed=false。
普通查询、参数、父推理、原始错误 detail 不进入这条通道。本轮未结算的失败不能
指望历史 ActionOutcome 数据库读取自动补齐。
R8 增加专用 `moment.inspect`：只读本人已发布动态，真实结果以
`published_moments`（正文/发布时间/是否还有下一页）交给 Writer。实际完成的
`moment.publish` 回执补充 published_text；accepted/failed 草稿不冒充发布正文。
不由用户关键词推断发布/承诺，不把原聊天删除或改写成新事实。
本轮存在未生效的 wardrobe.wear 时，reply_action_check 使用已配置的认知模型
核验待发布正文；无冲突直接发布，有冲突只允许原 Writer 做一次无工具纠正，
再核验仍失败或核验服务出错则不发布。已有草稿失败栅栏阻止重做；父循环在
下一次物理模型调用前立即结束。成功、no_change、纯讨论不增加此检查。

## 共享 Agent 子任务与辅助调用

| 入口 | 数据范围 | 不可改变的边界 |
| --- | --- | --- |
| media_prompt 的主体解析/分支规范化/最终 prompt | 主体解析与分支规范化是同一 Agent ID 下显式选择的结构化输出变体，实际请求携带对应的严格 JSON Schema；最终 renderer prompt 与头像构思继续使用文本输出。本阶段冻结意图、拍摄关系、可见主体及必要视觉字段；校验失败反馈 | 不加载无关 persona/history；真实图像继续用 image content part |
| conversation_summary 的累计/片段/每日模式 | 旧摘要与未覆盖消息 / 精确片段源 / 本地日与 segments | 不将通用聊天窗口额外叠加到待总结来源 |
| npc_response 的相遇/动态模式 | NPC 定义、世界规则、真实历史相遇、公开来意或动态正文 | 原始 target/CAS 在 Core，不冒充 Human 或 Fluctlight |
| Kev context.select/tools.select | 当前任务输入与本批可选上下文/工具候选 | 保留 `{state,questions}` 外部协议；state 为 JSON；context.select 异步执行，整阶段最多等待 60 秒，较早的请求或父级截止时间仍有效；超时保留原上下文 |
| Kev persona.switch | profile/rule 条件、提议、实际 Tool arguments、当前输入与语义状态 | event identity 在 Core；实际授权切换仍走工具 |
| Kev 自动流程/媒体 gate | 当前候选流程/意图/约束/拍摄方案 | 不增加无关私聊；不改远端 API。启用的自动 WakeUp 须经 Kev 允许，拒绝、未明或服务不可用会延期并在到期后再判断；手动触发及其他 gate 保留原有语义 |
| native_cognition 的 NPC 相遇机会判断 | Working Persona + native 生活事实/记忆 + 公开角色/已认识状态/地点/真实机会与有效时间 | `world_opportunity_cognition.go`；仅安装 world.npc.encounter，保留 exact opportunity id/revision，其余来源身份由 Core 绑定 |
| Owner life request | 本次请求所需字段、原始意图、冻结目标与真实状态；缺少执行回执时追加 `owner_life_execution_feedback` 业务续跑反馈 | `_request_id`/CAS 留服务端；必要 item/intention/asset target 保留；续跑沿用作用域内工具与原生回执，不重复已提交动作 |
| ADK/Writer/persona/media correction | 原任务上下文、必要真实结果和校验/质量反馈 | 保留工具隔离、原生 transcript、旧草稿 contract；不重做已提交效果 |
| vision probe | 诊断规则、测试文字和图片 | 非业务人格任务 |
| embedding | 文本向量输入 | 不使用 chat 运行协议；新任务冻结端点、模型和凭据引用，诊断记录实际版本而不记录密钥。旧任务无可证明快照时沿旧行为并记录诊断 |

换装、整套换装与建议发动态的首次认知保留原生工具和角色拒绝、推迟的空间。
系统规则只包含这类请求所需的处置、具体目标、回执和公开理由。用户原文、
冻结目标与原生工具回执的来源和结构保持不变。

模型明确接受却没有结算回执时，Core 对换装与建议发动态继续执行。Core 识别
结构化决定和明确的 `decision: accepted` 行。含义不明的候选先通过无工具的
DTO 校验修复，拿到明确接受后才进入执行。已明确拒绝或推迟的候选不会触发
要求调用工具的续跑。其他 Owner 请求类型沿用已有续跑。

续跑保留原输入和原生工具交互，只追加一条含 `required_capabilities`
与 `instruction` 的 JSON 用户消息。已接受但尚未执行的助手草稿不再带入续跑，
避免模型照抄在正文里生成的伪工具调用。换装在原作用域内经正常能力披露取得
`wardrobe.wear`，建议发动态使用 `moment.publish`。主人换装的原生调用传空对象，
由 Core 从原请求绑定单件或套装，避免模型抄错 ID。这个作用域的原生参数是封闭
空对象，显式传入其他目标会被拒绝。Core 仍在事务内核对冻结目标和版本。有明确接受且回执缺失
的物理请求只提供该工具，设置 `tool_choice: required`，并关闭执行轮的思考
模式。模型只输出文字而且没有回执时，追加一次纠正并重新要求原生调用，正文
始终不会转成工具调用。工具失败可根据回执纠正，三个目标工具结果都无法结算
时停止。动态正文仍由独立 Writer 生成。

该消息的 slot 为 `owner_life_execution_feedback`，source 为
`core_owner_life_execution_validation`，reason 为
`required_execution_receipt_missing`。取得实际执行回执或由工具确认无需
变更后，首次认知环和续跑的结束阶段都使用无工具请求。若模型仍试图调用工具，已确认的回执可以
安全终止该阶段。工具目录与查看结果不算执行回执。无法得到实际结果时返回
`owner_life_execution_missing`，对外明确显示本次未执行成功。

受控工具协议、回执和拒绝边界见 `owner_life_execution_recovery_test.go`。
可选的真模型回归 `TestLiveOwnerLifeRequestExecution` 使用独立 PostgreSQL，
通过真实换装和建议入口核对完成回执、穿着变化、动态 Feed 与请求重放。

## 必要标识例外

“去掉数据库字段”不能破坏可执行协议；以下字段有实际消费者，明确保留：

| 字段类别 | 必要性 |
| --- | --- |
| 目标评估 `goal_ref/criterion_ref/object_ref/eN` | 选择冻结对象和证据，由 Core 恢复真实 ID/version |
| persona profile/rule selectors | 选择已声明且获授权的人格/规则 |
| Owner 请求的 item/intention/asset IDs | 对应工具及缺回执后的业务续跑必须精确匹配用户选择的目标，不能猜测或改对象 |
| 初始穿搭的 registered_items.id / item_refs / wear_item_refs | 方案和当前穿戴引用唯一衣物目录，选衣结果由 Core 校验并交给真实 wardrobe.wear；不发送方案自身 ID 或库存来源流水 |
| 日程 place_ref/place_id、新地点局部 key；事件 place_id/npc_id/target_actor_ref | 显式选择授权世界实体或冻结的实际在场角色；Core 校验/绑定后才登记、排程或执行，不能猜运行时 ID |
| Studio body/allowed_references 中的资源 ID/version | 创作定义的合法引用词表；job 自身 id/version 不发送 |
| Goal Planner run/goal/promise selectors、revision/facts_revision | 当前 query/commit 的实际 CAS 与目标协议；调度 lease/retry knobs 已去除 |
| NPC 相遇机会 id/revision | world.npc.encounter 的精确选择与 CAS；来源事件/presence/profile 身份不进入模型 |
| 视觉候选 asset 引用与原生 ToolCall ID | 绑定真实候选或匹配原生工具结果 |

新增例外必须指出消费者并补测试，不能把完整 DB 行作为“方便上下文”。

## 本次样本验证

用户附件经本地重建后走实际 assembler：消息 content 合计从 **57,020** 降到
**24,447** Unicode 字符，减少 **57.1%**；仍为 4 条初始消息、32 个独立证据
事件，8 份重复正文使用共享 payload。去重保留 source ref、主体、时间、有效性
与成功证明能力。主因是移除历史 catalog 的无关工具契约、选中目标重复和日程
派生表示，并复用完全相同的证据正文。

这是基于用户提供 prompt 的重建输入实验，未调用 Provider，未重放原始数据库
快照，也不证明模型效果、实际 tokenizer 数、缓存命中或延迟。私人原始数据
仅保存在附件和本次临时目录，不作为仓库 fixture。

## 验证入口

- `prompt_composition_contract_test.go`：30 Agent 协议边界、原文与图片保真、
  必要引用、字段白名单、源事件独立、schema/领域数据保护、日程去重。
- `working_memory_test.go` 与 `prompt_context_assembler_test.go`：同来源不同
  事实、完整对话组、摘要覆盖和大输入保留。
- ordinary structured/stream、Writer、native 第三次工具结果进入第四轮的
  实际 HTTP 请求测试：没有 max_tokens/max_completion_tokens，输入完整。
- `provider_prompt_cache_prefix_test.go`：稳定事实前缀与变化后的动态后缀。
- `r7_live_wear_test.go`：显式启用本地真实模型与一次性 PostgreSQL，从自然语言
  请求进入生产 handleTurn，模型自主发现和执行；核对实际穿着、其他衣物保留、
  原生回执和重载读回。含动态复合、不可用、只讨论、SSE 与 Owner no_change。
  不能用 scripted Provider 或回复包含衣物名称替代此验收；还需审阅真实正文语义。

任何提示词精简都不能通过 token 上限、按估算截断或预算拒绝实现。

## 图片禁字与可选视觉审核（2026-11-02）

2026-11-09 质量策略调整：`media_quality` 继续根据实际图片返回原始审核结论；
首张不通过可按既有规则重试一次，第二张仍不通过时不得声称会自动交付。普通
媒体以真实 `retry`/`reject` 结束，视觉身份候选可保留给显式审核，只有实际
`pass` 自动采用。历史 `retry_accepted` 记录不等于审核通过；未完成的普通媒体
恢复时停止自动发布；有对应目标和附件回执证明已交付时只补结算、不重发，
已完成记录和真实效果保留。现有受控物理请求
测试检查实际图片和更新后的系统文本，不代表真实模型看图质量已验证。

`media_prompt` 的普通、质量重试和分支规范化指令明确要求最终图片无文字、
伪文字、字母、数字、标题、标签、字幕、签名、徽标和水印。Visual Identity
使用无文字角色卡模板，只投影外观与穿着，不额外拼接姓名、年龄元数据；
原始外观正文保持原样。模板和媒体补充指令不要求资料、简介、性格关键词或
签名分区，不预设领结、校徽。保留三视图、六种表情、服装/实际配饰、细节和
色卡。`visual_identity_vision`、`visual_identity_patch` 与生产 `visual_identity`
Agent 一致拒绝可见文字、伪文字、字母、数字、签名、徽标、水印，包括不可读
或零散字符；缺少文字和资料/签名分区不构成缺陷。视觉检查必须记录字符缺陷
并降低 identity_match，修正与生产审核即使人脸一致也必须选择 regenerate。
所有生产出图入口（聊天、角色视觉身份与 Studio）
共用 `submitComfyPrompt`：在提交时保留原正向描述并追加固定禁字协议；沿
ComfyUI `negative`/`negative_prompt` 声明边找到负向文本编码输入，保留原质量
排除项并追加禁字负向词。直接声明的 `negative_prompt` 同样处理，可选
`{{negative_prompt}}` 占位符解析为固定禁字负向词。无负向通道的模型保留其
原图结构，通过正向协议禁字；不创建虚构的负向节点。该固定输出政策不推断
主体、场景、构图，不改写用户输入、冻结意图、原生图片或数字渲染参数。
最终物理请求完整记录在原诊断 `request_payload`，缓存的模型 prompt 保持原样。

头像设计请求使用角色本人性格、经历、偏好和确认形象；允许选择动物、物件、
风景或人物等主体。若是本人肖像，维持已确认的脸型、五官、肤色和发型；头像
应为正方形，圆形裁切后核心清晰。文本模型只输出英文生图提示词，Core 不再
追加强制单人人像模板。视觉审核保留参考图与候选图，并依据实际选定主体判断
是否需要比较本人外貌。旧任务已冻结的 prompt 和已提交的 Provider job 不改写。

图片参数由任务冻结：头像比例为 1:1，新角色参考图与设定图为 2:3，普通图片
默认 9:16。普通图片可选择 1:1、3:2、2:3、16:9、9:16；分辨率可选 `512`、
`768`、`1024`、`1280`、`1536`、`2048`，默认 `768`。接收独立比例和分辨率
的 ComfyUI 节点可使用 `"ratio": "{{ratio}}"`、`"resolution": "{{resolution}}"`；
实际发送的分辨率是字符串（例如 `"768"`），不是拼接的宽高。`{{aspect_ratio}}`
与 `{{ratio}}` 等价。支持数字宽高的工作流也可使用 `"{{width}}"`、`"{{height}}"`，
它们根据所选比例和分辨率近似长边、按 8 像素对齐后作为 JSON 数字发送；独立
resolution 节点的实际输出尺寸由该节点决定，模型输入只包含所选比例和分辨率，
不把回退宽高当成节点实际像素。占位符必须放在真正控制尺寸的节点；
固定宽高或由参考图决定尺寸的工作流不会自动改变。

旧任务保留已冻结的 3:4 或 `宽x高` 分辨率值，以及已提交的 Provider job。若旧
任务尚未提交，但现在使用的节点不接受旧值，应新建生成任务；不要静默改变旧任务。
新旧角色卡审核分别兼容 2:3 和 3:4 竖版。

`vision.understanding` 继续默认关闭并独立绑定视觉模型。关闭时不发送图片
理解请求，候选图片保留为尚未检验、等待显式人工评审。返回摘要和时间线投影
说明设置入口及独立模型要求；开启设置不会自动重试此前跳过的审核。机器原因
`vision_disabled` 和既有权限保持原契约，无迁移或强制启用。

`TestMediaNormalizedPromptUsesProviderSchema` 通过实际 Provider HTTP JSON 请求核对
`media_prompt_normalized` 的严格 schema、必需的 `declared_human_count` 和无输出
token 上限，并让受控响应经过后端校验。`TestMediaPromptPhysicalRequestsRequireTextFreeImages`
继续核对普通、重试媒体提示词的协议和无输出 token 上限。
`TestImageSubmissionForbidsTextInPositiveAndNegativeProviderInputs` 通过真实
ComfyUI 提交函数检查三类入口的物理 HTTP 请求、SDXL 多文本负向编码、组合
条件、原始排除项、真实参考图片文件名和大整数 seed 保真；不等于已验证实图
质量或保证图像模型绝不产生字符。

`visual_identity_text_policy_test.go` 覆盖不同快照结构的姓名/年龄投影排除、
原始外观与源快照保真、无预设配饰和信息分区，以及 seed/character_sheet、
生产 Agent 多模态审核和两个 typed review task 的实际 Provider HTTP JSON。
typed task 使用一次性 PostgreSQL 数据库，其余测试无需数据库；受控响应只
验证协议传输和结果契约，不代表已完成真实模型识字或实图质量验收。

## 初始衣柜任务输入（2026-10-30）

`wardrobe_preparation` 的生成阶段只准备初始库存和 1–4 套方案，选衣阶段只
返回已登记衣物引用；每次只发送本阶段的规则。生成权限
`starter_items_allowed` 仅在生成阶段出现。既有四字段输出 schema 和独立
`wardrobe.wear` 执行、可用性/主体覆盖校验、版本栅栏、恢复语义保持原契约。
协议明确 `outer` 不能代替 `upper_body`；主体覆盖需要上衣与下装或既有
`full_body`。本地时钟从真实 instant/timezone 转换；月份、时区、职业及方案
名称不能证明天气、所在场景或当前活动。

选衣阶段仍要求 `items=[]`、`plans=[]`，以 `wear_item_refs` 选择已有库存；
`wardrobe_preparation_v1` 保留四字段传输 schema 和现有模型绑定，使旧式重复
库存输出可通过 Provider schema 校验后进入领域解析。Core 将重复 items 的
真实库存 ID 或唯一且完全相同的 `description + clothing_type` 解析为库存引用；
显式 wear refs 优先，仅在缺少 wear refs 时采用重复 items。非空 plans、
模糊或歧义匹配仍拒绝，不新增或修改库存，来源、可用性、所有权、主体覆盖、
当前情境版本及独立 `wardrobe.wear` 校验保持原契约。

选衣任务 JSON 按需增加 `selection_feedback`，提供上一候选的具体领域校验
错误及 `selectable_item_refs`。反馈只读取本次冻结库存和已保存候选，不追加
历史、运行元数据或重复库存详情；生成阶段不带此字段。输入仍为一条运行
协议和一份任务 JSON，不新增输入输出 token 上限。

输入按任务字段白名单构造。衣物详情只发送一次，方案与当前穿戴仅引用成员；
失效/缺失方案成员显式保留为不可用引用，目录外当前穿戴不能变成可选库存。
Core 保留库存/人格/Life 版本、来源流水和运行身份，不把完整数据库行发给模型。
身体事实取当前状态；共享和所选 profile 的通用 preferences、character_constraints
作为不透明业务资料保留，避免丢失存放其中的面料禁忌等约束。Foundation 只恢复
明确的 style_preferences 和 daily_outfit_preferences，不恢复旧伤情、旧库存或
旧穿着。旧卡中明确声明的结构化穿搭偏好继续提供给衣柜任务；可能混合社交
策略的 system_prompt 和 image_prompt 原文不再注入，源资料本身不作改写。

`wardrobe_preparation_prompt_test.go` 覆盖字段投影、未知/false/zero/大整数与
领域字段保真、失效成员、两阶段实际 Provider HTTP JSON 和无 token 上限。
`GO_CORE_WARDROBE_PROMPT_CAPTURE` 可显式指定本地私人 request/response 文本，
重建消息并比较字符数、核验原选择的主体覆盖；不入库、不调用模型，不能据此
宣称真实模型选择质量、token 数或延迟已验证。

`TestWardrobeSelectionRepeatedInventoryControlledProvider` 和
`TestWardrobeSelectionFeedbackControlledProvider` 覆盖无数据库的受控 HTTP 请求；
`TestWardrobeSelectionRepeatedInventoryCompletesRealWear` 需要隔离 PostgreSQL，
覆盖实际初始化、库存提交、独立穿戴与重放。这些测试不代表真实模型语义质量验收。

## R9 传输与工具进度补充

`EnableThinking=false` 在两条 Eino 及 legacy 请求中均显式传 false；初始化、
初始日程、片段摘要不再依赖网关的默认推理开关。输入输出仍无应用 token 上限。
诊断保留完整原生调用序列与 tool_call_id/name，不再只展示前 64 条消息。

私聊重复目录加载先进入已选只读查询阶段，真实读取结果后才可表达；重复相同
事实或无效果回执收敛到真实 Writer。执行侧同样校验阶段，模型忽略工具列表
不能执行其他动作。已接受的真实动作和媒体续写继续保留。未知名字返回原生
handled 回执，不猜工具、不伪造成功。此处未改人格和 Writer 的表达提示词。

新增 `promise.propose` 能力使用实际当前 Owner 消息原文，父认知只能提交明确
时间窗口，不能改写提议或声称接受。Writer 事实视图读取真实提议状态；已接受
约定由原 Goal/Intention/日程流程关联。独立购物由真实到期结果完成入库，只有
开始时选择的购后穿戴才执行独立 wear。正式 Agent 注册表目前共有 30 个。

## R10 初始化的世界来源

前端初始化仍使用一次分析请求。选择复用已有生活世界时，人物描述只补充所选
世界的已保存配置（背景、规则等），不把原角色身份、人物、运行位置、事件或
生活历史加入初始化输入。激活后由持久命令复制世界和地点配置，再显式选用；
该过程不调用模型，也不复制运行事实。没有修改 Agent、Writer、Kev 或模型
修复提示词，没有恢复输入输出 Token 上限。本轮验证为 UI、命令恢复和独立
PostgreSQL 配置复制，没有运行模型效果回归。

## R11 媒体主体引用执行

媒体工具将 `visible_subjects[].actor_id` 中既有的 `actor_self` / `self`
绑定到当前调用的任务角色，与当前事实中的 Actor 语义一致。只处理该声明字段，
不改写用户正文、媒体意图或其他人物引用。确定的人物权限拒绝作为非重试工具
结果交回原生循环，依赖故障仍保持真实失败。本次修复未修改提示词、模型入口
或任何输入输出 Token 控制；静态上下文回归不代表实际媒体生成成功。

## 共享世界与事件（2026-10-30）

世界提议不加载任一角色的 Working Persona 或私聊。实际在场角色使用本次冻结的
aN 引用，Core 保留绑定；人物 ID、世界运行和调度元数据不进入提议提示词。
Kev 使用 world.event.admit 的独立 JSON state，只携带候选涉及的地点/NPC/在场
角色及必要世界规则、时间和近期事实。No/unclear/disabled/异常不等于允许入库。

NPC 独立回应仍保留该 NPC 的共享已发生互动，新增 participant_name 和
is_current_character，防止把与甲的经历当作与乙的关系。scene_event 的稳定地点
身份贯穿准备、执行和结果，实际 NPC 在场仍以确认事件或有效已开始活动为准。
输入输出继续不设应用 token 上限，诊断估算不参与截断或拒绝。

### D7–D10 execution boundaries (2026-11-02)
- Owner-request automatic attempts keep retry ordinal/timing/cause in Core, out
  of the real Provider messages. A stale wardrobe request gets a new assessment
  with the same original target and a freshly frozen execution CAS; original
  Owner text and payload remain unchanged. Physical controlled HTTP regression:
  `TestOwnerImageAssessmentTimeoutRetriesThenCreatesExactlyOneImageIntent`.
- World event input is unchanged. Incomplete worlds stop before model/Kev I/O;
  each world/hour owns one staged result and a 10-minute execution deadline.
  New-world baseline configuration is materialized transactionally, without an
  additional model entry or fabricated Life facts. No token limits were added.

## 旧记忆的自动输入边界

2026-11-13 用户确认完全无法解析来源的旧记忆暂停自动注入，保留记录待 Owner
重新确认来源。长期记忆按现有 live-source 校验，零有效支持退出候选窗口；混合
来源仍按既有 partial 语义保留，仅将 live refs 标为有效支持。ActiveMemory 的
检索、resident 发布与既有缓存读取同样要求真实来源命令仍有效。既有 Owner
记忆修订或现有 ActiveMemory 确认命令记录新的可核验来源后，可重新进入读取。
来源、历史 revision 与 Owner 管理记录继续保留。

Reflection 派生 DevelopingSelf 的运行输入读取复用既有 proposal `context_snapshot`
中的 Memory 绑定，按实体、修订、request digest 及递归来源 liveness 复查。Owner
查看保留记录及来源失效状态，恢复旧快照同样复查原绑定。来源元数据留在 Core。

人格 overlay 的有效值和历史跨窗口证据复用原 Reflection proposal/context_snapshot
及 accepted candidate disposition。Memory 来源失效时，相关字段和依赖其绝对值
基线的后继 overlay 退出有效人格；独立字段及新有效基线保留。缓存继续根据
有效 persona 内容与 overlay revision 校验。

同一画像证据检查覆盖既有 ActiveMemory 和 Outcome：核对原实体修订、来源命令/
fingerprint，并按业务时钟处理 ActiveMemory 及其 ActorFact 依赖的适用时间。
人格编译与发布复核使用同一业务时间，过期来源退出有效画像/overlay。

## 持久私聊的未核验候选预览

`direct` 私聊在已有共享 Redis 可用且持有当前 inbox claim 时启用同一正式
Conversation Cognition 的原生 Stream。输入组装、Tools、模型绑定、采样和输出
合同继续沿用现有入口。队列内的观察者只提取原生 `conversation.reply` 参数的
顶层 `text`，不发送 Content、ReasoningContent、decision 或其他工具参数。
候选快照按 inbox 和 claim 身份隔离，TTL 仅用于临时显示缓存。浏览器以独立纯文本
区域显示“生成预览 · 尚未核验”，正式消息继续经过原发布核验。新物理候选、取消、
失败、提交和观察请求失效会替换或清除临时显示。`fact_only` 保留现有可选行为，
本次尚未为 Writer 增加候选预览。受控原生 SSE 与隔离数据库回归证明候选到达时
尚无正式助手消息，并核对无交付依据的完成声明仍被正式发布检查拒绝。

### 2026-11-24 后台生活链路修复

`life.activity.start` 的模型契约使用 `intention_ref` / `schedule_item_ref`，由当前授权投影绑定到 canonical direct ID；模型不能用任意数据库 ID 扩大范围。其他购物、地点与活动参数保留原契约。WakeUp 在原生工具回执明确禁止相同参数重试时终止重复调用；不强制发送私聊、不伪造完成，也不设置输入或输出 token 上限。

Goal 自动评估保留真实新活动后的 30 分钟静默；日程接受、日常回顾和重复请求本身不构成新证据。无新活动的每日回顾由角色本地 03:00 的持久入口创建，回顾前一完整本地日，同日和重启均复用已登记请求；Owner 显式重评和同轮恢复保留。
