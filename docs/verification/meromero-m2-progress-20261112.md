# M2 主对话与插件迁移：当前进度

起始提交 `838bb902b18280cbe5eeb119fb500db5da30ffa2`；沿用 M1 证据和四处原有未提交草稿。用户澄清：MeroMero 是在现有设置中选择的默认文本模型，不能将模型仓库或 ID 写死；默认私聊应读取现有配置并移除固定二段式。

| 子项 | 原任务 / 可验收行为 | 状态 | 证据与下一步 |
| --- | --- | --- | --- |
| M2-01 | T04/T06：跨轮媒体话题中省略图片名的已交付声明进入现有核验 | BLOCKED | 正式路径无依据红测已修复，受控 Provider 的匹配、无关交付和普通闲聊对照通过；两次真实 MeroMero 的无依据场景均通过，但匹配交付场景未通过：首次是后端对完整引用误拦，修正后第二次模型返回空 `subject_quote`，后端保守拒绝。按用户授权不继续调 Prompt 或模型字段，保留全部证据，不标 VERIFIED。 |
| M2-02 | T04：默认私聊使用现有配置的主模型及原生工具链，不固定进入独立 Writer | BLOCKED | 未设置 mode 时默认 direct；已保存或显式选择的 fact_only/legacy 继续生效，开关保留，不写死模型 ID。隔离库与受控 Provider 已验证默认选模、variant 覆盖、模式切换、原生查询、认知结算、幂等、旧草稿和相关回归；页面测试通过。本机真实 MeroMero 默认私聊发出 3 次请求后因 `direct_reply_arguments_unresolved` 未发布，真实模型合同未通过。保留日志 `meromero-m2-evidence/m2-02-live-default.log` 与请求证据 `meromero-m2-evidence/m2-02-live-default.json`，按用户要求不调整 Prompt、采样或合同。 |
| M2-03 | T05：Image 停用时默认私聊与纯文字动态隔离 | VERIFIED | 受检提交 `1d67046e`：一次性数据库中，正式设置关闭 Image 后，默认 direct 私聊经真实原生工具调用发布纯文字动态、消费结果并回复；独立 Moment 调用及重放通过，新的图片调用被拒绝且无媒体 intent。干净提交的 10 个相关顶层受控回归通过、无 SKIP；含四处原有草稿的工作区主场景也通过。日志 `meromero-m2-evidence/m2-03-postcommit.log`、`meromero-m2-evidence/m2-03-working-tree.log`。Provider 是测试替身，真实模型和渲染不在此项证据范围内。 |
| M2-04 | T05/T06：已保存渲染 Job ID 的未完成图片跨进程恢复 | VERIFIED | 受检提交 `707ac2ad`：正式 Image 请求受理后，第一个进程在 Job ID 落库后遇到受控 503；Image 停用后第二个独立进程恢复同一 Job，经受控质量检查和存储后只交付一次，重放不重复；新图片请求被拒绝。干净提交的 7 个定向顶层回归通过、无 SKIP，含四处原有草稿的工作区主场景也通过；日志 `meromero-m2-evidence/m2-04-postcommit.log`、`meromero-m2-evidence/m2-04-working-tree.log`。渲染服务已接单但 ID 尚未落库的崩溃窗口仍为父任务缺口；受控服务不冒充真实 Temporal、ComfyUI 或模型。 |
| M2-05 | T05/T06：渲染服务已接单、Job ID 未落库时避免重复提交 | VERIFIED | 受检提交 `6ba41249`：用户选择直接暂停、等待人工核对。发送前持久化标记；丢失响应后暂停，旧任务无可靠 Job ID 也保守处理；普通媒体经明确核对后可恢复已就绪资产或明确重新生成。干净提交的 19 个 Core、6 个受控工作流、4 个迁移、3 个浏览器边界及 11 个定向页面用例通过，均无 SKIP；含原有草稿的工作区主场景通过。旧红测和提交后证据均保留在 `meromero-m2-evidence/m2-05-*.log`。Studio 可核对后显式丢弃任务，Studio/视觉身份尚无核对后接回原任务的领域入口，不能据此称完整恢复；真实 Temporal/ComfyUI 未验证。 |
| M2-06 | T05：无人物图片的 SDXL 正向文本边界 | VERIFIED | 受检提交 `d7e5f8ce`：正向 `text_g`/`text_l`、`positive_prompt` 和连接的 `PrimitiveString` 接入现有工作流事实边界。干净提交与隔离 PostgreSQL 上 12 个顶层专项及 4 个当前拍摄子场景通过，含四处原有草稿的工作区主场景通过，均无 SKIP；红测和提交后日志见 `meromero-m2-evidence/m2-06-*.log`。模型和渲染为受控替身，不代表真实生图或 T05 完整验收。`enum-rejected` 同环境基线失败单列 AEL；图片无字规则在 `PrimitiveString` 的独立问题登记为 `image-no-text-primitive-string-boundary`。 |
| M2-07 | T04/T05：direct 主链先换衣再自拍的事实与交付 | VERIFIED | 受检提交 `04ac059f`：正式 `HandleTurn` 中，实际衣柜查询回执驱动换衣，受理自拍前核对持久穿着，再核对媒体快照、受控渲染请求及附件幂等交付。干净提交的 9 个顶层回归及含四处原有草稿的工作区主场景通过，均无 SKIP，见 `meromero-m2-evidence/m2-07-postcommit*.log`；首两次测试夹具错误及修正日志保留。模型、渲染、质量审核及存储均为受控替身，不代表真实模型或真实生图验收。 |
| M2-08 | T05/T06：direct 私聊中动态附图的目标绑定与交付 | VERIFIED | 受检提交 `dc78610b`：正式入口红测证明原图片任务落到会话而非动态；现可显式绑定本轮前一响应中实际发布的动态，经既有所有权及 Image 门禁交付。无目标、同批、非法目标、外部动态、Image 停用、普通会话图片和不确定提交均有对照。干净提交的 18 个顶层回归和含四处原有草稿的工作区主场景通过、无 SKIP，见 `meromero-m2-evidence/m2-08-postcommit*.log`；真实模型和真实渲染未验收。 |
| M2-09 | T04：direct 主链 Goal 候选的实际下游结算 | BLOCKED | 受检提交 `ef717eb1` 的正向场景证明：候选先遵守原静默窗口，隔离库模拟窗口结束后，正式评估消费已提交消息和原 Goal 条件，完成领域结算及幂等重放；提交后 7 个定向用例通过、无 SKIP。扩展回归中 3 项既有 Goal 测试失败，在干净提交 `f74fbc1c`、无源码覆盖及同一隔离库环境复现；其中两项实际返回静默期延期，另一项只报告未出现预期错误，原因待核查。见 `meromero-m2-evidence/m2-09-precommit.log`、`m2-09-baseline-goal.log` 和 `m2-09-postcommit.log`；已关联 AEL `goal-cadence-additional-baseline-regressions`，未更改 Goal 规则或既有测试，不能宣称本子项全部验收通过。 |
| M2-10 | T05：Moment 停用时默认私聊与 Image 独立运行 | VERIFIED | 受检提交 `763ab46c`：正式设置关闭 Moment 后，旧动态回执可重放、新动态拒绝；默认 direct 私聊及独立 Image 工具可用。干净提交的 11 个顶层插件和主链回归通过、无 SKIP，含四处草稿的工作区主场景通过，见 `meromero-m2-evidence/m2-10-postcommit*.log`。Provider 为受控替身，未实际渲染。 |
| M2-11 | T05/T06：动态正文与实际媒体交付一致性 | BLOCKED | 提交 `b3249912` 在既有无附件和媒体类型保护上，增加当前附件、历史动态与私聊交付的作用域检查和事务重查；保留先前红测。干净候选 9 项定向和 27 项回归、含四处草稿的工作区 3 项场景均通过；干净提交后 4 项关键场景通过，均无 SKIP，见 `meromero-m2-evidence/m2-11-resume-*.log` 和 `m2-11-postcommit-resume.log`。生成意图不是图片内容依据；已选素材具体画面主体和真实模型语义仍未验收，未知交付时间的保守拒绝可能误挡合法表达。本项保持阻塞，不据此宣称 T05、T06 或 M2 完成。 |
| M2-12 | T04：direct 关系引用与认知影响结算 | VERIFIED | 受检提交 `f6042a06`：原请求有关系语义却无可结算引用的红测已修复；被现有规则选入的关系携带 Core 不透明引用并在正式事务冻结 influence，重放不重复，伪造及跨会话引用被拒绝。干净候选 3 项定向和 12 项回归、含四处草稿的工作区 3 项、干净提交后 6 项关键场景均通过、无 SKIP，见 `meromero-m2-evidence/m2-12-*.log`。未更改模型、提示词文本或 fact_only 开关；受控 Provider 不代表真实模型通过，T04 与 M2 仍未完成。 |
| M2-13 | T05：动态 NPC 评论与停用恢复 | VERIFIED | 受检提交 `b1b4498f`：隔离 PostgreSQL 与受控 Provider 下，正式动态的已结识 NPC 评论和事件各持久化一次、重放不再生成；停用前无决定时抑制新互动，已有决定时在受控插入故障后可停用并恢复收尾。最终三种场景及提交后复验通过、无 SKIP，因磁盘不足显式使用 `-vet=off`；此前默认 vet 的 5 项相关回归通过。日志及两次磁盘失败见 `meromero-m2-evidence/m2-13-*.log`。不代表真实模型、真实 Temporal 或整个 T05/M2 已验收。 |
| M2-14 | T04：已提交回复的持久流回归 | VERIFIED | M2 起始提交 `838bb902` 与当前 `02db92f3` 在同一隔离库及受控 Provider 下通过同一 direct durable 场景；浏览器 NDJSON 11 项和 Web 60 项已有测试通过，相关代码到当前无变化，见 `meromero-m2-evidence/m2-14-*`。该子项仅证明已提交消息与终态链路；生成中的正文增量在起始版本即缺失，已有工单 `durable-chat-missing-live-token-stream`，完整实时流式体验及 M2 仍未验收。 |
| M2-15 | T03：动态关系证据的业务值与主体 | VERIFIED | 受检提交 `ccb6b4a6`：隔离库的真实 Moment 准备路径红测证明关系业务嵌套字段被误删；修复后保留业务原值、映射已知或未知人物，排除所选关系的 Core 外壳。干净候选 2 项定向和 11 项回归、含四处草稿的工作区 2 项、提交后 5 项均通过、无 SKIP，见 `meromero-m2-evidence/m2-15-*.log`。受控 Provider 不代表真实模型效果；其他证据类型及 T03/M2 尚未完成。 |
| M2-16 | T03：动态默认关系上下文业务值 | VERIFIED | 受检提交 `b0fc38fd`：正式 Moment 准备路径红测证明默认关系业务嵌套字段被误删；修复后保留原值及原关系选择与人物视图。干净候选 2 项定向和 12 项回归、含四处草稿的工作区 2 项、提交后 5 项均通过、无 SKIP，见 `meromero-m2-evidence/m2-16-*.log`。默认关系与同一已选关系证据的去重尚需核验；受控 Provider 不代表真实模型通过，T03/M2 仍未完成。 |
| M2-17 | T03：动态关系上下文与证据去重 | IN_PROGRESS | 验证默认关系与同一已选关系快照是否重复进入 Writer；不同关系即使内容相同也必须保留。 |

M1 中同一调用 ID 重放的非阻断缺陷已登记 AEL，按用户要求暂缓，不在 M2 顺手修复。其余 M2 验收仍按原 T03～T06 和里程碑要求逐项处理，不以局部完成替代父任务验收。

M2-02 的受检代码已保全为 `bd148af710aaffd72e761647684e0021c722ad40`。
提交后使用干净导出、无额外源码覆盖及一次性 PostgreSQL 复跑 8 个顶层 Go
用例，退出 0、无 SKIP；Web 定向 4 项退出 0、无 SKIP。日志分别为
`meromero-m2-evidence/m2-02-postcommit.log` 和 `meromero-m2-evidence/m2-02-web-postcommit.log`。
真实模型失败前的请求证据含前两次工具参数，第三次响应仅保留工具名及调用 ID
是否存在，不能当作完整原始响应；首次测试脚本不兼容本机 Python、以及切换测试
读取版本类型错误的失败和修正也分别保留在对应日志中。

M2-05 扩大回归中，诊断页面 5 项失败及头像人工审核快照 1 项失败，在干净
基线 `b3f5cd40` 的同环境均可复现；已分别登记 AEL
`diagnostic-task-detail-baseline-regression` 和
`avatar-manual-review-snapshot-baseline-failure`，未在本项修改。

M2-05 的静态复核还发现：提交歧义错误若包装上下文取消或超时，工作流活动
可能重试；现有持久标记仍会阻止第二次图片提交。真实 Temporal 行为尚未验证，
已按任务外非阻断问题登记 `media-submission-ambiguous-context-retry-feedback`，
未在 M2-11 顺手修改。

人工核对后的领域处理缺口另关联
`studio-unknown-submission-manual-recovery` 和
`visual-identity-unknown-submission-manual-recovery`。Studio 当前可核对后丢弃；
视觉身份仍缺完整的原工作流衔接。没有可证明同源的本地结果时继续暂停，外部
结果归属和接管规则未确定；不自动查询或重新提交。
