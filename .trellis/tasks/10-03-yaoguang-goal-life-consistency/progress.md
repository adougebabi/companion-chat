# 2026-10-03 实施进度（持续更新）

用户已于本轮批准实施，task status=in_progress，分支 codex/goal-life-consistency。未提交/推送。开发由主线程负责，所有子代理仅只读。

## 环境
已启动本机 OrbStack；独立测试容器 codex-goal-life-pg-20261003，pgvector/pgvector:pg16，端口127.0.0.1:32768。测试 DSN postgres://fluctlight_test:fluctlight_test@127.0.0.1:32768/postgres?sslmode=disable，为本轮测试新建的随机per-test DB；没有访问用户业务库。结束时清理此容器。真实 Provider/ComfyUI 尚未配置到本轮测试；仓库 .env 和 infra/compose/fluctlight.local.env 存在，后续安全核对，不打印凭据。

## 已写代码（仍需联合验证）
- instant.go：固定毫秒数字偏移；ContextProjection as_of/reference timezone/time_view；聊天同一参考时区；用户时区只有明确 Actor fact 才展示。
- App.Clock 已串联 projection、activity start/advance/result、scheduled trigger、intentions、context resolver、部分schedule/media/scene CAS；SQL活动due用业务instant。仍需全入口Clock清查，API其他序列化时间未全部统一。
- 修复活动Event缩短时revision与result.revision不一致（虚拟Clock测试暴露）。
- sleep typed item_type= sleep/Event kind=sleep → behavior_state；ProcessWakeUp静默不调用模型/生成cognition/reflection evidence。sleep_cycle按 lifecycle→wake row→Life lock 顺序，last_settled cycle/epoch防重放推迟计时。
- period target与speaker分开；旧私有viewer仍按授权目标，当前current_speaker空。Native/daily其他system入口尚需贯通。
- life start、wear、scene、Moment/media和自主回复领域边界补sleep校验，真人新消息回复策略保持既有。
- 0043 actor_facts：明确来源与属性/时间/纠正关系；actor.inspect/actor.fact.record正式Tool；轻量Actor背景；memory_event.actor_fact_ids关联派生项；correct退休错误，change保留原时间段。旧Actor源码未全量污染迁移，Summary/DevelopingSelf/Active lineage生产挂点尚需补齐；源消息编辑失效trigger尚需实现。
- 0044库存使用：existing wardrobe items扩展item_kind=wearable/object，object没有slot；item.use独立使用与事件；当前snapshot used_items。购物套装acquired_items全成员成功，部分结果拒绝；稳定member来源ID；Advance将真实item IDs回填模型。
- 新增Tool已加independentToolProductInventory/phase8固定列表，但 formal adapter/live runner固定矩阵还需加行，既有矩阵不得删减。
- Tools+response schema部分预算cap由32768改49152，完整MaxInput/context/output安全约束不变；需在spec/report记录，实际wire预算持续测试。

## 已执行结果
Core无DB普通测试在加入事实Tool前通过；加入Tool后已修固定inventory并缩短schema描述，当前全量需重跑。
已通过真实隔离PG测试：TestShoppingUsesInjectedBusinessClockWithoutWaitingOrChangingAuditTime、TestTwentySleepingChecksDoNotWakePublishShopOrCreateReflectionEvidence、TestFormalWakeUpAdvancesElapsedHaircutWithoutSendingMessage、原virtual shopping/extension、Actor correction/history/change/foreign source、0042 immutable/Actor upgrade rerun、普通物品获取→独立use重放、套装部分失败→完整入库幂等。
初始time/sleep选择集合JSON日志在evidence/time-sleep-tests.jsonl（exit0）；未来变更后要重跑。只有主线程测试结果为验收证据，子代理仅核验。

## 下一步
T02补来源编辑/撤回与派生lineage并测试原生Loop真实source；T05目标/日程与非服装闭环测试；T06最终可信媒体边界（目前仍自由文本prompt，未修）；T07累计摘要/定时/CAS；T08scoped dry-run/apply/rollback；T09后端全局排序+稳定分页+Web；T10全矩阵/真实服务/报告/spec-sync。不得宣称任务已完成。

## 2026-10-04 后续实施（覆盖上面的旧进度）

- 继续实施，主线程已读取 trellis-continue/check/update-spec。两项新只读探子因未及时收敛已中断，没有用其未交卷结论。所有代码/验证仍由主线程负责。
- 普通 Go 全量先前失败两项已修：conversation compact policy 保留正式 Agent 且 <=900 rune；Browser fake diagnostics 改 page envelope。
- Native cognition system speaker 置空；daily Review/WakeUp 同样无伪造人类发言人。
- 新 migration head **0048_proactive_topics**（0043 Actor facts, 0044 inventory usage,0045 runtime summary,0046 diagnostic pagination,0047 history repair 仍保留）。新增主题/目的/真实入站sequence/业务occurred_at审计，product.autonomy.topic_suppression_seconds 默认43200，允许300..604800秒；自主回复必须提供稳定topic_key/purpose，普通回复不要求。communication_state进入Runtime。
- appearance.style 补事务内 sleep guard + Clock；业务 Clock 补 AcceptedSchedule、Goal/Intention初始化、detail、scene/presence准备、active memory查询、memory.recall、visual identity 初始化、发送者时间；还有其余业务 Clock/serializer审计待收口。
- 目标 execution 只读投影来自现有Goal/Intention/Activity，包含stage/next_step/last_attempt/last_result，不建第二进度权威。Goal reference snapshot必须排除execution动态派生，防止意图due转移导致Goal ref错误漂移。独立和scheduled真实活动结果均携Goal结果引用，单criterion且无未完成Intention才完成。intention.inspect/decide开放Autonomy（phase8固定矩阵已扩展）。
- 非服装受控初始化item_kind=object允许无slot，不能initial worn。actor.inspect/fact.record加入service缺失保护。
- formalToolAdapterInventory原有29行+actor.fact.record/actor.inspect/item.use=32行已通过；**还需加入schedule.inspect/edit/intention.schedule，和strict runner独立完整匹配**。已修旧wire alias断言，通过真实provider ref codec处理，不删原测试。
- current_capture最终封闭style plan扩展body_detail/scene和姿态；服务端保留显式capture.mode/camera/framing/angle，unsupported angle/framing拒绝；加入最终workflow exact prompt placeholder/positive conditioning ancestry/LoadImage reference检查，不能模板追加衣物。近期补稳定视觉identity traits及current身体字段，尚需重新测试；reference仍严格匹配effective_life/body+wardrobe revision，不同衣服参考拒绝（保守策略，真实配置未验）。
- Actor facts按必要背景属性优先。Reflection提交同事务验证当前Actor fact集合和版本并锁Actor scope；所有由该模型读入事实产生的Memory/Active/DevelopingSelf保守挂该输入事实精确revision依赖。Runtime summary也挂输入事实依赖。ActiveMemory读取过滤失效edges。**还需检查已有revision修改/confirm的依赖延续、stable persona evolution overlay依赖、source no-op/edit/delete/restart整链测试**。
- Runtime summaryworkflow遇pending等待durable Temporal timer再执行，不将早启动任务当完成；5–10min interval已配置，输出2048rune目前仍固定，**可配置摘要预算未补完**。已有高水位/去重/CAS实现和测试。
- history repair apply/rollback加RepeatableRead。inventory审计包含target worn/used refs和wardrobe聚合，apply后全批final aggregate revision落audit，rollback先全批核验后精确恢复目标refs（同owner只推进一次state revision），新状态拒绝恢复。Actor source fingerprint恢复前复查。**还需实际隔离CLI invocation、mixed periodic tests、apply replay rolled_back status修复**。
- removal: compileOneWorkingPersona已移除model失败synthesizeBaseline兜底，只保留明确blank_slate revision0的受控初始化。修复失败cognition inbox带已提交mutation的agent_partial不能重新赋新operation重跑；纯查询失败仍可重试。selected恢复回归已通过。
- 新共享internal/instant包统一format，API/browser writeJSON做声明timestamp字段固定毫秒UTC序列化，保留opaque原始audit/prompt/snapshot/persona资料；Core instant.go委托该包。**新增Marshal测试和所有边界回归待补**。
- DST local wall time解析补gap拒绝/fold选择earliest，显式ISO偏移按原瞬时不改；LA/Berlin测试通过。还有24:00无偏移T格式恢复需核对。

### 本轮证据和状态
- `evidence/goal-recovery-tests.txt` exit0：黑靴/画笔持久Goal→future Schedule→Clock→真实获取→独立wear/use→snapshot→最终workflow入参；主题抑制；exact inventory repair/新版拒绝；capture camera/template overrides；20sleep。
- `evidence/life-wire-regression.txt` exit0：实际wire Life ref alias、scene causality、stale domain、formal adapter32行。
- `evidence/recovery-regression.txt` exit0：failed final持久mutation不重复；WakeUp failure replay；Foundation compiler失败不发布；Reflection原子回滚。
- `evidence/remaining-regression.txt`旧失败已修，不能作最终pass证据。
- 最近完整PG回归`evidence/go-postgres-tests.txt`有旧失败（尚未重跑修复后全量）：due Goal ref、旧schedule fixture overlap、新topic字段、final repair调用数、compiler fallback、phase8 Autonomy固定行；均已修或target通过。
- `.env`安全preflight：MTPLX模型qwen3.8-27b-abliterated-mtplx-optimized-speed，100.80.75.9:8001 connection refused(errno61)，ComfyUI8188 HTTP200。未打印key。user async已请求启动模型服务/可用配置，另请求FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE现有JSON路径。
- strict runner已实际调用tools/agents/all：日志`evidence/live-*-preflight.txt`和`evidence/live/`；tools Provider connectivity FAIL，agents/all缺visual config未通过。不能算真实行为/像素验收。
- pnpm generate/typecheck/test/build exit0：browser-client13 + core-client2 + Web58 tests通过（确切Core计数看日志），build日志web-build.txt。之后补Core OpenAPI诊断page/cursor契约，**需重跑generate契约同步**。
- postgres disposable base schema已迁移到0048，使使用shared GO_CORE_TEST_DATABASE_URL的旧测试有schema；多数业务测试仍随机per-test DB。容器结束需清理，未接触业务库。

### 还需完成
1. 扩展formal adapter另外3个既有schedule Tools；新增Core/browser envelope/cursor消费测试。
2. 补Actor源编辑/delete/no-op + late Reflection集合保护、摘要failure/DELETE重建/并发worker长消息预算、合法起床、当前可中断slot edit、实际Media worker最终提交防绕过测试。
3. 审计并补剩余Clock/输出、配置summary预算、repair CLI安全操作证据、固定live baseline。
4. 全量隔离PG test-race、vet/build、JS生成/typecheck/test/build、phase8 gates；主线程最终验证，修失败。
5. actual browser验证诊断排序/filters/load more（代码测试已过但未UI验）。真实模型服务/config若可用再strict live，若不可用明确blocked矩阵。
6. docs/fluctlight-goal-life-consistency-report.md完整38case映射/count/log/state/budget证据+spec-sync；task artifacts头门禁旧[ ]改已批准/实施。不要完成/archive。完整审阅后按Trellis提交具体批量commit计划供批准，未push。


## 2026-10-04 交付检查点（覆盖前述旧失败状态）

- 新增修复：Actor correction同属性校验，Reflection overlay Actor依赖与reload过滤；旧Working Persona hash失配fail-closed，需受控重编译。全部可见fact保守依赖的影响已写入报告，未声称candidate精准来源。
- 实际repair CLI发现typed nil map导致first apply输出null；修复后完整dry-run/apply/replay/rollback/replay均成功，stable batch与before/source输出保存。
- 摘要prompt max_runes动态一致；explicit enable_thinking=false到请求，support/effective mode诊断unverified。claims expiry接App.Clock；API/Worker结构日志瞬时统一数字偏移毫秒。
- 被替代累计summary producer/selector移至test-only历史fixture；memory.recall的历史episode查询保留。
- Browser实测发现partial older page被poll覆盖：explicit older-page state修复；agent-runs refresh导航修复。production dist+fixture API桌面/390px页追加、时间、刷新通过；scrollWidth=390，console errors=[]。
- 最后全量race：1641个test/subtest pass，24 live/external skip，15 package pass/10 no-test package skip，exit0。之后新增periodic repair负例、physical budget日志、instant日志回归独立通过。vet/build、generate/typecheck/build通过；browser-client14/Web59 tests全部通过。core-client没有test脚本，只记录generate/typecheck。
- phase8-delivery contract gate PASS，固定35Tool adapter未删项。
- 同数据集full-wire预算8703→2032 estimated tokens，summary覆盖36/40，tail4；五轮真实ADK物理请求估算42297/43866/44866/45681/46141，每轮独立门禁，累计222851。非实际usage，见budget.json/physical-budget.json。
- docs/fluctlight-goal-life-consistency-report.md与test-matrix.md已映射全部38场景。跨层7节可执行spec加入backend索引及frontend质量契约。
- 真实服务仍阻塞：MTPLX8001 errno61连接拒绝；ComfyUI8188 reachable但visual live config未提供。tools/agents/all的失败preflight保留，未计PASS。
- 未完成项：live行为/真实usage/Comfy+S3+像素；未知生产历史裸时间与repair清单需真实来源审阅。任务in_progress；无commit/push/deploy/生产修复，不archive。


## 2026-10-04 用户授权提交与清理

用户明确回复“提交吧，一些测试数据就删除”。HEAD d6636ef已包含前轮tracked edits，本次补交剩余新增源码/测试/迁移/spec/report。删除77个临时产物（5951602 bytes），保留13个精简结果JSON及README；报告链接同步，不再声称原始日志仍可查阅。无生产数据清理，live验收未完成，不归档。


## 2026-10-04 Wake-up静默诊断路由

用户报告conversation.reply{text:no_op,purpose:周期静默诊断}。当前源码原本已有精确
no_op拦截，因此ToolCall本身不证明私聊成功。补齐正式Tool描述与final schema/policy：
内部原因放response_intent，无需消息Tool；周期result保存该字段。typed error优先
保留reply_control_value_invalid及纠正指导；failed/rejected result不强制completed/
capability，真正送达回复仍以实际结果为准。
独立普通/autonomy及原生WakeUp误调用→失败反馈→静默final→持久原因→重放回归
通过，零assistant消息；既有自然私聊、生活活动、预算/Tool registry回归保留。
定向race测试128 pass、0 fail、4 live skip；vet/build通过；只读核验无发现。
测试使用新临时PG容器，原始日志仅/tmp且测试结束清理；精简统计追加到validation-summary.json。
本次补丁尚未提交或部署，无生产数据操作。


## 2026-10-04 显式actor_user设置入口

补齐前轮遗漏：可选顶层actor_user.background，创建预览/JSON导入/激活透传；
与core_persona并列，认证Owner作为subject，在激活事务写现有actor_facts。
当前背景在详情独立只读展示；治理表单支持8项简单字段、未知null/见面false、
correct/change语义、原因、版本CAS、稳定请求与单事务批次。0049命令ledger只
用于重放审计，没有第二份运行背景authority。聊天纠正与设置读写同一事实层。
相关初始化44 test/subtest pass、0 fail、6 DB-dependent skips，含3个本次数据库用例；
Browser边界race通过；客户端15/Web62测试通过；generate/typecheck/build/vet通过。
浏览器production dist+syntheticAPI实测详情→治理→保存反馈；390px无横向溢出，
console errors为空。它不是实际Backend/DB E2E。
真实数据库验证仍待完成：Docker daemon未运行，Mac锁定无法启动OrbStack，已
异步请求用户解锁并启动。新增激活/CAS/重放/批次回滚测试已写，未把skip记PASS。
本次未提交/部署或对生产库迁移，任务保持in_progress。


## 2026-10-05 framing枚举与兜底验证

用户明确授权current capture framing失败兜底first_person/full_body。模型指令从
同一schema枚举生成，字段与合法值都明确列出。known aliases照常归一；缺失/非法
framing恢复全身构图，有效pose/expression/lighting/style保留，其他非法枚举与额外
物理字段拒绝，不吞掉invalid pose中的衣物描述。prepared plan与fallback标记持久化，
保留原capture与原context_binding；cache重渲染、quality视图使用相同effective capture。
新的quality重试生成先清除旧标记，不把旧兜底永久覆盖合法显式请求。
真实隔离PG+脚本Provider/Comfy transport验证最终submit及保存标记，source snapshot
不变。定向38 test/subtest通过且无skip；更宽media/capture/provider context/prompt
race回归164通过、0失败、3 live外部skip；vet/build通过。真实图片像素未验收。
本次恢复OrbStack后，也补验此前actor_user数据库创建、Owner CAS/重放、批次回滚、
聊天纠正共享事实：全部通过，迁移到0049通过。此前数据库阻塞记录已由新证据覆盖。
代码未提交/部署，无生产迁移；清理临时PG/运行日志，保留精简结果JSON。


## 2026-10-05 MediaPrompt规范化职责修正

用户再次报告current_capture_framing_conflict并澄清规范化用途。旧实现只兜底非法
framing，仍把Main提示与规范输出字面比较，职责偏差。本次把Main capture/framing
定位为语义提示，MediaPrompt产出标准照片方案，新增标准capture输出（相机模式、
相机、角度、镜面/设备可见性），取消字面冲突路径。渲染/quality读取准备好的同一
方案；原请求留审计，身体/衣物/物品与workflow核验保留。旧plan兼容时只保留已知
相机提示，未知几何文字不当作硬约束。无法规范framing仍first_person/full_body。
真实隔离PG+脚本Provider/Comfy传输回归166 pass、0 fail、3 live skip；vet/build通过。
新增portrait/selfie/front→full_body/mirror_selfie/rear可准备并实际走到捕获提交，
quality与渲染一致，原快照/请求不变；已有物理事实负例不删。
本次未提交或部署，真实模型/图片像素未验收；清理临时PG与日志，留精简JSON。
