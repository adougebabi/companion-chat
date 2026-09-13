# A批详细技术设计

状态：规划已审阅；业务尚未实施，实现前仍须完成技术核验。类型与字段是拟定合同，命名可随现有代码调整，语义和验收不得省略。B设计另见roadmap.md。

## 1. 全局边界

Core是领域事实唯一写入者，BFF负责浏览器授权/DTO，PostgreSQL保留事实、revision、幂等及outbox。Temporal执行任务，Redis只加速。
普通聊天正常路径一次认知产生提案和可见文本，确定性校验后执行和提交。不用第二次模型调用润色或修复无效JSON。
诊断与领域事务隔离；已有cognition lease、冻结结果、CAS、稳定消息/工作流ID保持。暂停、退休、取消策略不由恢复代码绕过。
源码在变化，按evidence中的符号与验收核对；已有修复只补必要验证，不重做、不还原。

## 2. A01 输入合同

### 2.1 改动位置

provider_context.go、provider_prompt_composer.go、mutations.go、autonomy.go、wakeup.go及对应测试。验证必须捕获provider.go真正发送的最终HTTP payload，不能仅测格式helper。
先修现有合同，不在A把所有map改造成通用框架。

| 场景 | 必需内容 | 异常行为 |
| --- | --- | --- |
| conversation | sender Actor、current_message.content、当前profile、上下文及合法工具 | 不能从transport role=user猜Human；矛盾输入在模型前明确失败 |
| wake-up | 触发类型、可引用来源、当前profile/状态、治理条件 | 不把内部哈希全量送模型，不能丢本轮触发语义 |
| 存量realization | action_type、response_intent或冻结response_plan、允许的context | 缺行动合同则有界失败，不能只给背景让模型重新猜 |
| reflection | evidence_ref、payload、appraisal及允许窗口 | RawMessage/[]byte先解码，非空证据不能静默丢弃 |
| initialization | 用户描述、初始化schema、允许的引用 | 不要求尚未存在的人格runtime |

composer输入按角色/场景区分，必需字段有校验；非必需字段舍弃可记录诊断。不能为了保留字段把任意内部map变成system指令。工具完整schema只在tools发送。

### 2.2 当前人格

稳定profiles/switching等定义仍来自Core Persona；当前主导值来自同一份PersonalityRuntime。
规范动态块active_personality包含active_profile_id、必要profile内容和已有合法切换状态。system中初始active值必须从“当前权威”位置去重或明确归入初始定义。
目标、记忆视角、关系筛选用同一份profile；冻结回复后不重新读取另一版人格。
runtime不存在时仅使用既有合法默认并记录来源；runtime引用不存在profile要报数据合同错误，不能猜。绝不把运行切换反写Foundation。

### 2.3 当前消息

producer统一current_message{sender,content}。旧text/current_user_text由一个兼容入口归一化，同文只保留一次；冲突不悄悄拼接。
历史去掉同一源消息优先用身份判断，不误删过去另一次同文发言。已有历史/工具去重保留。

### 2.4 单次调用

纯文本、memory_event、已有tool reply的正常聊天不新增action_realization。显式重试与后台reflection另计。
存量后台两阶段只修缺失输入；后台统一归B04，不能因A兼容而新增两阶段调用。

## 3. A02/A03 错误关联与诊断

### 3.1 标识

| 标识 | 语义 |
| --- | --- |
| correlation_id | 一次源操作/事实的关联链，可覆盖显式重试 |
| operation_id | 一次入口执行，区分同源的不同执行 |
| model_run_id | 一次模型attempt；queued/running/terminal更新同一行，新attempt新ID |
| provider_request_id | 现有上游关联/幂等用途，不能因日志改造随意改变 |
| workflow_id/run_id/activity/attempt | 稳定任务与实际运行，不能混为一个ID |

入口在配置读取前建立关联，BFF经受信内部头传Core，复用WithProviderCorrelation。外部传入ID需有界格式校验，不直接信任任意字符串。
model_run_id同步在内存分配，不依赖诊断写库成功。provider_queue.go/runProviderQueued及provider_redis_queue.go/acquireProviderRedisSlot当前依赖diagnosticID，必须始终获得非空ID；诊断失败不能绕过Redis并发限制。异步化的是日志，不是准入/取消/限流责任。
旧行/旧ID继续可读，不补造历史attempt。

### 3.2 错误合同

内部保留cause、code、stage、retryable。stage区分request_validation/configuration/queue/provider_transport/provider_envelope/provider_structure/domain_validation/persistence/workflow_execution。
浏览器保留稳定code，增量details.correlation_id/stage和有界安全字段。Owner诊断保留内部原因，普通页面不透传SQL、连接串或原始Provider body。
初始化错误不再全映射422：输入/人格验证为既有4xx，配置/网络/超时按已有边界分类；改映射前核对BFF与生成客户端，不能单改Core。
在退出边界记录一次主要错误，其他层只补causation，避免每层重复刷屏。

### 3.3 Provider证据

记录HTTP状态、允许的upstream request ID、content type、耗时、安全error code/message、解析位置/期望结构。
优先提取已知错误字段并脱敏；未知body无法可靠脱敏时只保存类型/大小，不强存原文。所有摘要有长度上限与truncated标记。
transport成功与parse/schema/domain成功分开。StructuredFallback显式可见；后续领域拒绝不能被completed状态掩盖。不新增模型“修JSON”调用。

### 3.4 持久化旁路

每个进程复用一个有界sink，Provider/Core经窄接口提交；不得每次App{DB:...}新建goroutine。
拟定初值：最多1024条、总缓冲8MiB，任一上限触发则立即drop并输出结构化stderr摘要；单条64KiB上限，超大prompt保留摘要和原大小。计量在截断前完成。
writer批次最多50条，独立最长2秒context；排队不阻塞业务。终态必须单调，乱序queued不得覆盖terminal。
DB/序列化失败、满队列、关闭flush超时走不递归stderr，含ID/阶段/安全原因/drop计数。不得再调用自身报错。
停机flush最多2秒；崩溃可能丢尚未持久化诊断，如实记录限制。领域审计不依赖此sink。
若现有writer已满足要求就复用，不再套第二层。数字可以经负载测试收敛，但不能用无限缓冲/长期阻塞解决。

### 3.5 页面入口

失败发生的位置直接给出可理解的摘要和“查看原因”入口，打开时自动带当前operation/correlation与角色上下文。用户无需先复制ID、找诊断页再手工筛选；复制ID只是辅助。
后台失败可在现有诊断页按角色与最近失败定位，不新建监控平台。已恢复的最新状态要更新，旧失败尝试保留在展开详情中，不能一直用旧错误遮盖成功结果。
完全网络断开时只显示实际可知的连接失败及本地请求ID，不假装已经获得服务端根因。

## 4. A04 页面直观定位问题

### 4.1 首屏回答四个问题

1. 哪里出问题：操作名称、角色/时间、失败阶段，区分排队、执行中、自动重试、失败、取消、被新操作取代。
2. 为什么：已有证据支持的原因与具体字段/上游错误。明确区分已证实原因、仅观察到的现象、证据缺失。
3. 影响是什么：哪些步骤已提交、哪些未执行、是否有部分成功；依据领域结果显示，不从模型输出猜测是否已记住/已发出。
4. 下一步怎么办：等自动重试、打开对应配置、按现有幂等接口重试，或查看缺失证据。只有当前操作实际支持时显示可操作入口。

技术ID、原始JSON和完整事件列表不占首屏；它们作为按需展开的证据。用户应能在首屏判断问题类别，至多展开一层查看具体依据。

### 4.2 具体显示约定

下列是故障夹具对应的拟定文案形状，不是当前生产故障报告：

| 证据 | 首屏呈现 |
| --- | --- |
| Embedding模型绑定缺失，记忆事实已提交 | “记忆已保存，向量索引未生成：未配置Embedding模型。”给出对应设置入口；不显示“记忆保存失败” |
| Provider确实返回429 | “模型服务限制请求，当前未完成。”显示实际重试状态/次数；有服务端retry时间才显示时间，不猜是余额不足 |
| 结构校验定位到某字段 | “模型返回内容不符合要求：缺少〔实际字段路径〕。”展开显示期望类型与安全响应片段，不仅显示persona_invalid |
| 超时但没有上游原因 | “请求超时；尚未获得上游失败原因。”显示发生阶段/耗时/重试状态，不断言是上下文太长 |
| 记忆检索/使用异常但请求整体成功 | 显示各阶段实际结果，如“已找到3条，最终输入包含2条”，并说明另一条未选入原因；不直接断言模型已经理解或使用 |
| 查询诊断本身失败 | 显示“诊断记录暂时无法读取”及可知原因/更新时间，保留已加载证据，不伪装“没有问题” |

不新增LLM自动分析日志：使用已有结构化错误、阶段结果和规则映射。无法归因时清楚指出缺哪段证据，不生成看似确定的根因。

### 4.3 查询与投影合同

沿用并扩展现有查询边界；handler/query共用correlation/角色/状态/分页校验。当前操作入口自动带筛选，手动筛选只服务深入排查。非法filter不得静默放宽到所有数据。
Core/BFF输出受Owner授权的诊断投影，拟含operation_label、status、failed_stage、reason{certainty,code,message,evidence_refs}、impact[]、next_steps[]、attempts、updated_at与各数据源可用状态。
其中certainty区分confirmed/symptom_only/unknown；impact状态来自领域事实；next_steps带可用条件与目标入口，不允许模型或错误文本任意指定执行URL/命令。保留稳定code/ID供开发定位。
复用现有事件、model runs和workflow关联，按需查询并有界分页；首屏不等待全部workflow历史读取。一个源失败不阻断其余，partial提示清楚区分“没记录”与“读取失败”。
诊断写入丢失时尽量显示可从独立状态获取的记录不完整信息；数据库和进程都不可达时，只能显示连接/读取失败，不能保证页面仍持有根因。stderr仅作为开发旁路，不把打开终端列为日常排错成功标准。

### 4.4 工作流证据按需展开

工作流相关的首屏内容是失败活动的可理解名称、原因、尝试次数/当前重试状态和已知影响；不要求用户先理解Temporal event类型。
保留event_types/event_count兼容字段，必要时增量events：event_id、occurred_at、type、可用activity ID/type、有界failure；attempt未知为null。
failure cause最多4层、每层512字符且脱敏。最新失败摘要应可直接读取；历史分页只用于展开后的证据，page_size默认50最大200，cursor绑定workflow/run与授权。
实施前查当前SDK失败摘要/分页API，不能为首屏而全量扫历史，也不能只给前50条导致末端失败不可见。Temporal不可用单独显示，不误报Owner未授权。
取消、被取代与正常等待不得渲染为未知错误；自动重试和终态失败分别展示。手动重试只复用现有授权/幂等接口，不新增批量Reset或自动试错流程。

### 4.5 范围

本批不做导出、下载、日志打包或导出参数修复；原导出缺陷留在证据记录，排除本批交付。现有导出接口不顺手删除。交付标准是用户在页面上看懂问题，不是新增一个能下载更多日志的功能。

## 5. A05 PostgreSQL quiet deadline

### 5.1 选择

本方案选择PG保存deadline，复用Worker周期扫描；保留单轮WakeUpWorkflow。Redis expiry仅提示“检查是否到期”，不再直接cycle+1或提前deadline。
理由：保持当前防抖/单轮执行形状，比恢复长期Temporal timer减少历史切换范围；不存在两个时间权威。

### 5.2 数据

platform_workflow_intents增量字段next_wakeup_at timestamptz可空、wakeup_schedule_revision bigint默认0，仅wake_up.current使用；建立按类型/状态/deadline部分索引。
next_attempt_at仍是失败重试时间，不混用quiet时间。保留workflow ID和payload.cycle；新派发input可选携带schedule revision，不注入模型。
列名可按既有迁移约定调整，语义固定。

### 5.3 状态与事务

| 事件 | 原子变化 |
| --- | --- |
| 成功用户回合 | 与完成事实同事务deadline=完成时间+现有间隔，revision递增；提交后尽力种Redis |
| 成功唤醒 | 同事务保存事实；schedule revision匹配才更新下一deadline，若新聊天已推进则保留其deadline |
| completed且due | 行锁重查deadline/治理状态；completed→retry；cycle只加1；保存派发revision；next_attempt_at=now |
| pending/retry/started | scanner不再增加cycle、不抢lease，由现有dispatch/reconcile负责 |
| 已排队后新聊天延后 | Provider前比对revision/deadline；返回quiet deferred不调用模型，保留future deadline |
| Provider运行中被新聊天/治理取代 | 提交前检查cognition lease、状态与版本；旧结果不外显 |
| 工作流失败 | 现有reconcile retry/backoff负责，不由quiet scanner覆盖 |
| deliberate cancel/retired | 不自动复活 |

两Worker竞争靠行锁+状态条件只成功一次；事务不等网络。提交前CAS避免陈旧结果，但极窄竞态可能浪费已发出的模型调用，需记superseded，不能承诺零浪费。
每次scan最多20项，正常循环每秒执行，独立超时；验证在两个成功scan周期内进入可派发，不把Provider排队算scanner故障。
Redis旧通知、Key丢失和重启不能改变持久deadline。Worker重启不重新延长所有completed角色一个周期。disabled/paused语义沿用现有治理，避免新增busy loop。

### 5.4 迁移/回滚

1. 先增列/索引，不启新scanner；旧库读写兼容。
2. completed缺deadline从最近成功用户回合/唤醒的权威时间+间隔回填；没有可靠时间用迁移时刻+间隔并记fallback，不制造同时立刻唤醒。
3. pending/retry/started/cancel_requested保留cycle/run和状态，迁移不能重放已完成效果。旧input无revision由既有lease/幂等保护，新合同在明确接管后使用。
4. 新PG-deadline开关启用前所有同队列Worker须支持该合同，关闭旧expiry推进逻辑，禁止新旧时间权威并行。
5. 尽量不改workflow命令序列；若实际修改Temporal指令顺序，保存历史replay与版本路由必须先通过。
6. 回滚先停新scan/新增派发、对账run，再切兼容代码，并从PG剩余deadline重建旧TTL提示。不删列、不重置cycle。回旧版本会重新暴露旧通知风险，明确记录并人工对账。

## 6. A06/A07 记忆与最终请求证据

reflection visibility schema与领域共享已有private/owner/participants枚举，移除不受支持public，不扩大权限。
保留已有reflection JSONB/appraisal/evidence去重修复，覆盖实际DB返回形状到最终prompt。
记忆诊断分阶段：created(tool/reflection, source/revision)→embedding pending/ready/failed/stale→retrieved候选/选中→injected最终ID。越权排除原因不得泄露未授权正文。
最终payload形成后计messages片段、tools、response schema的序列化字节；保存prompt_version/来源/计量方法。可靠tokenizer才记录估算与版本；上游usage才标actual；缺失为null。rune/字符不是token。
A只计量，不引入统一输入硬上限；B01负责预算，不将“可看大小”当“已经治理长度”。

## 7. 记忆验证边界

三层分别是存储/检索确定性验证、最终HTTP payload验证、真实模型行为对照。
合成事实的答案必须不在recent history、Foundation、summary、目标/关系或本轮问题。先同一授权会话让原话退出近期12条窗口，避免为了测试取消权限。
A修合法候选到prompt的传递缺陷，交付真实评估入口。>200候选、自动revise、全新跨会话scope作为B基线明确失败，不增历史让其假通过。
真实模型多样性不能由fake输出替代；真实回答正确也不能替代权限/事务证明。

## 8. 兼容与发布

Core/BFF/OpenAPI/生成客户端/route inventory一并更新；details现有命名保留适配。新增诊断字段可空，旧记录未知不造值。
增量迁移同时测空库→head和旧库→head；字符串contains测试不够。诊断内容保留既有权限和保留策略。
本批不部署；实施报告提供通过/失败/跳过、B残留、schema版本与回滚步骤。文档中的SDK细节是实施前核实项，不是假定已验证API。

## 9. A08 目标与意图推进

详细合同以[目标推进诊断与设计](goals-progression.md)第2节为准，包含短引用/服务端快照映射、同批create关联、progress与生命周期写回、FrozenAction来源关联、成功及终态失败结算、过期与暂停处理、页面真实进度。
不得只修进度显示就宣称闭环。A08复用现有wake-up/反思，不增加每轮第二次回复生成；完整time/event intention调度与重规划归B05。与A04共享直观原因展示，不做导出。
