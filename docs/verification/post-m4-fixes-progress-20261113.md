# M4 后续可独立修复项

起点 a01fb8fc9dd3a85624f7df0932e6fd23714da2d6，延续同分支与四处原稿保护。Owner新授权先修可独立验证事项，需要Owner判断或真实模型/媒体体验的项目保留。

| 单元 | 目标 | 状态 | 证据 |
| --- | --- | --- | --- |
| F01 | 确认并修复换衣失败后的显式技术回退及纠错场景 | VERIFIED | post-m4-fixes-evidence/postwear-* |
| F02 | 同ID无权限/关闭窗口回执跨轮重放有界 | VERIFIED | postdenial-* |
| F03a | 诊断详情接入现有原媒体任务人工核对 | VERIFIED | entry-* |
| F03b | Studio同源ready资产人工确认恢复 | VERIFIED | poststudio-*及studio-* |
| F03c | 视觉身份同源ready资产人工恢复 | VERIFIED（同源ready/原paused Run范围） | postvisual-*及visual-* |
| F04 | direct生成中的候选正文增量预览 | VERIFIED（direct预览范围） | postpreview-*及preview-* |

F01修正当前用例配置，保留r5夹具fact_only模式；使用允许显式回退的503；在数据库独立配置generic_llm备用模型，保留原Writer冻结值。原输出、穿着、次数、未授权重执行及最终模型元数据断言保留。只改此用例，生产回退策略diff 0。红测与第一轮只修两前置仍失败的日志全部保留。

M4旧回执数量勘误：R6私聊组为22个子场景，旧日志20 PASS/2 FAIL；原回执“其他24”应为“其他20”。本次修正后的完整组统计由新日志记录。原失败日志内容保留。

F01整组及回退边界回归通过。共22个R6私聊子场景全部通过，400/角色拒绝/错误媒体声明不触发fallback的原测试通过。原断言保留，修改范围为r6_chat_wear_test.go局部前置。

F01提交eb7b48db后7个顶层回归和22个私聊子场景通过，无SKIP；包含四原稿的只读叠加目标2场景也通过。AEL首次assess因提交后新增证据文件使候选变化而拒绝，先保全证据后重新生成同代码候选，技术验证结果与版本保留。

F02保留首次执行身份及单一权威回执，以已有真实模型响应批次统计拒绝投递。3轮拒绝后仅允许回复，继续忽略2轮后停止；同批多调用算一轮，真实动作重置旧计数，同批合法动作仍可执行。7项原红测修复后通过，新增同批5次普通查询保留首次收尾机会的回归通过。独立静态审查发现初稿将普通查询条数纳入5轮停止，已限定为物理拒绝轮次。首次编译漏更新2个旧测试签名，已修正且原断言保留。较大回归首次180秒超时，600秒时限复跑210.515秒通过，无SKIP，失败日志均保留。四原稿SHA-256保持。

F02提交9a11ec65后干净源码67项顶层回归237.111秒通过，无SKIP；四原稿只读叠加8项目标11.939秒通过。AEL首次登记因工件路径在仓库外返回UNSAFE_PATH，现将脱敏日志保全在仓库并重建文档收尾候选；技术结果保持。

F03a只调整诊断详情导航。failed/submission_unknown指向当前execution.intent_id对应的正式原任务页，复用既有React人工勾选、ready资产恢复、版本与幂等流程。缺当前执行身份时提示重新读取，不使用历史关联任务替代。既有恢复drawer不再由该正式入口调用。新回归修复前失败，修复后相关22项媒体测试通过，typecheck退出0。整文件修改前28项23通过5失败，修改后29项24通过同样5失败，属于既有独立UI合同偏差，已登记diagnostic-task-detail-baseline-regressions，本项未调整。基线测试临时副本已删除。

F03a真实IAB浏览器在loopback受控Vite页面加载正式诊断renderer，观察到“接单状态待人工核对”及“查看原任务并核对恢复”，目标为#/diag/task/isolated-original-task。截图在工具会话中查看，未提交图片。页面使用合成诊断数据和无业务写入的服务夹具，未覆盖产品全套样式或真实图片服务。初次启动受sandbox监听权限拒绝，获得工具审批后仅绑定127.0.0.1；夹具中间件位置首次被SPA路由接管，调整测试夹具后核验通过，生产导航实现未因此变更。临时服务已停止。

F03a提交3be7c69d后22项媒体回归通过，AEL原入口工作项已验收completed。父任务状态不据此改为完成。

F04实现现有Redis跨进程临时快照，key含inbox与claim owner，每次物理模型响应有独立candidate ID。仅direct且存在当前claim与已配置Redis时开启同一正式Agent的Stream。未引入第二套模型、工作流、配置项、Prompt或token限制。观察者只聚合原生conversation.reply顶层text，忽略Content/ReasoningContent及其他工具参数，完整秘密和分片尾部秘密前缀均不透出。JSON转义、UTF-8与代理对只显示完整字符。候选独立于正式messages和附件，浏览器只接受unverified快照并以纯文本区域显示。新物理请求替换、正式commit/终态/失效/断流清除；断流在历史补偿读取之前立即清除。旧claim迟到写无法覆盖新claim。缓存3分钟TTL只作用临时预览，不进入正式业务数据。

F04隔离DB、测试专用miniredis与受控原生SSE实测：Worker未结束、正式助手消息0条时收到增量；观察器断开后Worker继续，重连得到同一candidate的当前完整快照；正式消息到达前清空候选并正常结束。开启此路径时零交付依据的照片完成声明仍3次检查后拒绝，媒体任务0、正式助手消息0。Core/边界相关35项顶层回归通过，浏览器边界全包86项通过，前端聊天/恢复/React60项通过，无SKIP。新增旧epoch候选及迟到cancel隔离断言通过。Web与browser-client typecheck及产品build通过。

F04失败记录均保留。首次前端typecheck发现nullable narrowing，已修正；首次Core测试的工具wire名误写为双下划线，按源码单下划线修正测试后5项通过。独立静态审查发现断流清理晚于补偿读取，用延迟history回归复现后修正。延迟夹具首次通过替换global fetch未生效，因为client已捕获fetch引用；改为测试局部控制Response.json后准确复现并通过，未改共享夹具。真实IAB在仅loopback受控页面加载正式ChatPage和产品CSS，截图核对独立“生成预览·尚未核验”及字面<img>文本，未产生附件。截图只在工具会话查看，未提交图片，临时服务已停止。

F04当前预览适用direct。fact_only继续可选且保持原行为，本轮未为Writer新增预览。真实模型速度、自然回复体验与真实媒体生成仍按Owner授权保留未验收。以上受控Provider结果不计为真实模型验收，亦不代表T04/T06或全链完成。

F04提交42653f8c后干净源码Core/边界回归149.736秒通过，前端60项通过，无SKIP。首次submit因组成清单知识源变化被拒绝，已审阅实际变更并重新index/context，当前候选assess已completed。未改变业务代码来处理管理限制。

F03b为Studio增加resume-reviewed正式领域命令及浏览器/客户端/React原任务入口。事务核对Owner、job/media revision、current attempt、原workflow，复用media-intent执行锁，原媒体workflow仍active/paused/cancel_requested时拒绝。只有asset_<原intent>的Studio scope、Owner/job、provider request和原workflow全部同源且ready，质量结论不是retry/retry_accepted/reject时可恢复。保留job/attempt/media/provider request与生成围栏，仅为已有结果新建恢复媒体workflow并同步原attempt绑定，记录人工核对审计。ProcessMediaIntent使用reviewed_ready的已有ready分支结算，Studio完成分支纳入该状态。旧workflow迟到回调不能结算新身份。未自动查询或重新提交生成请求，未增加外部ID/图片导入。

F03b覆盖未确认、缺ready、错误request/workflow、旧attempt、job/media版本变化、质量拒绝、active/paused/cancel_requested、丢弃、Owner、执行锁、旧failed提交围栏、重复/并发命令。独立静态审查未发现阻断实现问题，并指出四处覆盖缺口，已追加恢复与discard/settlement竞争、admission后质量变更、SDK TestActivity错误workflow身份拒绝、已受理但丢回执后React重挂载原身份重放。Core目标与Studio整组通过，浏览器边界全包通过，前端Studio34项、客户端48项通过，无SKIP；Web/client typecheck和产品build通过。IAB使用合成数据加载正式StudioJobs和产品CSS，已查看待人工核对状态、仅同源ready入口及明确人工核对确认对话框，未提交真实操作。截图只在工具会话查看，临时服务已停止。

F03b保留所有失败。红测确认旧入口Unknown Studio control fields。首次实现测试缺Owner账号前置，补齐局部夹具；下一次测试将幂等回执的_replay标记与原JSON整串比较，改为核对原id/revision/result和单次workflow计数。首个浏览器回归使用旧HEAD的OpenAPI导致新路由不匹配，私有测试运行器现允许显式叠加本工单当前OpenAPI后通过。客户端测试首次因sandbox拒绝tsx本机IPC失败，工具审批后相同命令48项通过。重新生成客户端时发现F04新事件原先只写到生成产物，现同步generate.mjs源模板，确保再生成保留candidate_preview，生产事件语义未变。

F03b沿用既有Studio兼容规则，旧ready资产空quality verdict允许恢复，已记录拒绝/重试结论继续拒绝。恢复确认用于归属核对，未将其标成质量通过。没有同源本地ready或需要外部结果接管的情况继续暂停，外部归属、导入与补生成规则仍需Owner决定。真实图片内容/ComfyUI与真实Temporal服务验收仍保留，本轮只包含控制数据、SDK Activity harness及真实数据库/正式入口测试。

F03b提交a6b92eb7后37项Core整组102.796秒、前端34项通过，无SKIP，AEL原Studio工作项已completed。外部接管/无ready继续暂停，父T05及全链状态不改为完成。

F03c增加视觉会话resume-reviewed正式领域命令及公开浏览器/客户端/React入口。仅处理当前active session/current attempt的候选图或设定图，对Owner、visual/media/原workflow revision与原Run建立围栏，核对asset_<原intent>的fluctlight scope、owner、provider request、media workflow、ready及非空SHA。原媒体workflow仍active或持有执行锁时拒绝。原视觉workflow必须仍可在当前Run resume；终态、不存在或Runtime不可用时保持暂停，不创建替代视觉工作流。

F03c复用既有markMediaIntentCompleted的事务正文，抽出同义markMediaIntentCompletedTx，使已有ready结果、衍生图审核/ActionOutcome、Owner审计和幂等回执在一次事务提交。图片结算后用现有OwnerWorkflowCommand向原Run发送同一持久resume命令；RPC未知统一503并保留原身份。已确认sent的命令只回读；未确认时会核对current session/attempt/visual revision，原来源已被替代时停止继续发信号。迟到旧media失败因completed状态围栏不能逆转结算。归属核对继续保留原质量结论，设定图仍进入现有awaiting_review，未将人工核对标为质量通过。历史没有derived review的分支沿用现有完成逻辑并明确保持无review记录。

F03c原入口红测失败。首个实现回归因局部夹具将workflow状态更新后的实际revision假定为1而失败，按数据库回读当前revision后候选/设定图及未知RPC重放通过。新增Owner、执行锁、旧attempt/visual/media/workflow版本与Run、终态原Run、无ready、错误request/workflow、质量reject/retry/retry_accepted、旧非法prompt、来源替代、原运行在本地提交后切换、历史无评审及同身份并发回放测试通过。同身份并发只有一个持久control command和一个受控resume效果；未知RPC重试保持相同原Run与command ID。

F03c独立静态审查未发现功能性缺陷；指出的额外质量、历史无review、并发及本地提交后Run变化覆盖已补齐。前端两种stage均覆盖明确勾选、缺确认零写入、已结算但RPC回执丢失后remount原输入/原key回放。媒体/Studio前端整组58项通过，浏览器边界全包88项通过，client48项、Web/client typecheck、产品build通过。实际IAB加载正式VisualWorkspace和产品CSS，核验原候选暂停、同源ready恢复入口、人工确认checkbox和说明表单，使用合成数据且未提交真实命令；截图未提交Git，临时服务器已停止。

F03c较大Core回归64项顶层通过、1项既有头像手工审核失败、2项真实模型门禁SKIP，整条命令退出1，不能计为全绿。相同环境与输入对起始10d797c4和当前代码单跑TestGeneratedAvatarRealMediaHTTPWaitsForManualReview，均为media_model_snapshot_failure: media_model_snapshot_invalid，且发生在媒体完成事务之前。沿用既有avatar-manual-review-snapshot-baseline-failure工单，本项未修复该非阻断基线问题。两个真实用例TestVisualIdentityLiveE2EPreflight、TestVisualIdentityAgentReviewsRealObjectImageAndSavesCanonical保留未验收。提交后将明确选择其余已执行的相关场景复验，并保留本次完整失败/跳过日志。

F03c验证使用真实隔离PostgreSQL及正式Core入口，RPC来自受控WorkflowRuntime探针；未声称真实Temporal原Run已完成或真实图像/模型验收通过。无本地同源ready、无法证明外部结果、原workflow已终结或未知恢复结果已跨新session/Run时继续暂停核对。T05/T06、M1～M4与全链状态不据此改为完成。

## 本轮最终回执

受检生产实现提交f908e28737194aa3045c9170644dfc1cf4663693。F01、F02、F03a、F03b、F03c和F04（direct预览范围）已分批提交并完成各自AEL验收。父任务与M1～M4整体验收沿用既有未验收事项。

| 最终验证 | 实际结果 | 证据 |
| --- | --- | --- |
| 干净提交后明确选择的相关Core场景 | 65项顶层PASS，退出0，无SKIP，141.793秒 | postvisual-postcommit.log、postvisual-postcommit-selection.json |
| 干净提交后浏览器边界全包 | 88项顶层PASS，退出0，无SKIP | postvisual-browser-postcommit.log |
| 干净提交后媒体/Studio前端 | 58 PASS，退出0，无SKIP | visual-web-postcommit.log |
| 最终聊天/观察/React回归 | 60 PASS，退出0，无SKIP | final-chat-regression.log |
| 最终诊断媒体入口与原任务恢复 | 22 PASS，退出0，无SKIP | final-diagnostic-media.log |
| 四原稿只读叠加的兼容性 | 4项顶层及R6组22子场景PASS，退出0，无SKIP，48.676秒 | postfinal-workingtree.log |
| 生成客户端、类型与产品构建 | client48 PASS，Web/client typecheck、产品build退出0 | visual-client-tests.log、visual-typecheck-final.log、visual-client-typecheck.log、visual-build.log |

65项选择清单明确排除已有独立头像基线失败1项和按Owner决定保留未验收的真实模型门禁2项；前一次较大回归的64 PASS/1 FAIL/2 SKIP日志和前后同环境同输入对照保留。task detail整文件既有5项UI合同失败也保留在diagnostic-task-detail-baseline-regressions工单。上述缺陷继续按独立工作项处理，本轮局部通过不能替代它们的验收。

Studio与视觉恢复均以本地可证明同源的ready记录为范围。外部结果接管、无法证明归属、缺ready、原视觉工作流已经终结及跨新session/Run的未知恢复结果继续保持暂停。真实模型native流式能力、实际回复体验、真实ComfyUI图片及真实Temporal服务完成结果，本轮没有补跑，继续保留未验收。fact_only保留现有可选行为，Writer候选预览本轮未新增。

四原稿最终SHA-256与起点一致，测试叠加只读取副本。所有图片仅通过本地工具会话查看；Git提交通过禁图片钩子。临时Vite验证服务器均已停止。验证使用一次性隔离数据库与合成内容，受控Provider/RPC、SDK Activity harness在证据中明确标识。最初红测、夹具错误、编译/类型错误、超时、sandbox IPC限制及基线失败均按原结果保留，成功日志未覆盖失败样本。
