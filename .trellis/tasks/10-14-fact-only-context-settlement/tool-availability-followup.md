# F18：全工具告警与兜底

## 用户最终政策

在F17 reply evidence之后，用户再报goal.evaluation.submit的goal_review_decision_required与invalid_arguments，要求38个Tool全部检查、所有Tool必须可调用。最新明确：必须保留的拦截逐条具体说明，否则统一告警+程序兜底。无部署/生产数据/真实workflow执行授权；同一任务/分支继续。

源码基线b24fdd1已包含F14–F16。六名只读探子覆盖公共入口、Goal/Intention、发布/媒体、life/schedule、memory/actor/persona/habit/relationship、wardrobe/item；affect_event由主代理亲读补齐。实际全局48注册，conversation38、wake_up37、autonomy22、native_cognition34、Owner1、Planner2、Evaluation3、Visual3；Writer私有reply.image.request是第49个unique name。inventory/schema实验输出/tmp/tool-audit-common-inventory.txt，主代理保存机器清单/49行报告。不能把registered当全部业务已经执行通过。

## 已核实的问题

- 合法required value:null被公共validator当missing，type union数组实际上未做类型分派；Core副本与canonical两套都要修。
- native/canonical schema在binder/preparer/normalizer前，多余字段、可空技术文本/optional null会先invalid_arguments。Goal review在schema可选却在已验证根评估后报required，回滚合法根；可选改善、unknown quote、skip/abandon等被过度required。
- Kev披露/selection/window/config改变在执行期变成capability_not_disclosed/tool_not_selected，与实际资源权限不同。应保留选择建议和raw audit，按已安装且当前surface合法的工具恢复执行。
- 重复/已关闭/不存在/游标越界/缺可确定技术字段/lookup无记录，多模块把它们标failed；wear/item/habit相同状态还制造revision/outbox，增加后续CAS冲突。
- 真正资源身份、来源/版本、原子事务、取消、新旧命令幂等、真实文件/动作结果、用户明确频率/容量规则仍需防止不可靠副作用；这些不等于普通业务前提必须让整轮报错。

## 架构决定：真实handled，不伪造completed

新增内部CapabilityResult.Status=handled，明确表示调用已处理、没有提交请求的业务效果。统一output含handled=true、effect_applied=false、mutation_committed=false、disposition（needs_input/no_change/awaiting_state/unavailable等）及受控warning_code。ErrorCode留空、Go error为nil；诊断用warning，UI显示已处理/告警。真实执行成功仍completed/accepted，真实依赖故障/取消/实际Owner授权失败仍保留原因。

必须端到端处理：Core/canonical result Validate、Runtime output validation、Tool ledger terminal status、model projection、Direct/linked ActionOutcome和Goal/Intention/证据消费者、诊断label。handled不得要求异步externalRef、不得变成Pending挂死或completed业务证明；映射现有ActionOutcomeSuppressed/明确no_effect并不增Goal进度、不完成Attempt、不制造Memory事实。不要新增表/状态权威；若API现有enum真的封闭需同步schema/client，否则不做无关生成。

统一恢复层只能在无副作用的前置分支，或明确事务/savepoint已rollback之后处理可恢复business error。不能catch所有error改成功；DB/net/Provider/context cancel真实错误、未知真实IO、凭空未提交的output、跨Owner/私有claim forged、真实外部副作用结果不能伪造。可对state_changed/target_unresolved返回handled且不写，这保留写入前提但不需要fatal。

建议共享helper稳定接口：`handledCapabilityResult(inv CapabilityInvocation, code, disposition string, output map[string]any) CapabilityResult`。Prepare需no-op可用Core-owned prepared marker或等价明确接口，由统一runtime识别并进入真实receipt边界，模型不可注入；实际字段以stage1产物为准。后续各域复用同一结果形状，不能各造一套completed假成功。

## 阶段1：共同层

1. required只判断是否存在，null交属性schema；两套validator都支持union types并拒绝错误object/array，保留nullable显式unknown语义。
2. 在model-owned schema/binder前只做确定性表示修复：已知schema默认值、无歧义标量类型、optional非nullable null省略、受控额外字段丢弃；nullable null保留，语义required缺失不能猜。仍无法解析返回handled needs_input及安全required字段信息，避免不断invalid_arguments。真正server envelope、ContextSnapshot、PreparedPayload/owner/source不可从模型args补授权。
3. remove empirical raw arguments 64KiB拒绝，改元数据告警，合法JSON继续；保持必要字段/DB表示/真实Provider容量原因，不静默截断事实。
4. Kev为建议/披露窗口，不作为已安装同surface工具执行许可。自动加载或直接路由真实scope registry并警告；仍不能执行未安装、其它私有surface工具。names>8拆批/去重，mixed names返回有效子集与unavailable警告，不整批拒绝。模型不能由此获得外部资源权利。
5. 共同errors依据有证据的业务类别soften，不把任意IO或真实业务失败结果改handled；事务回滚和receipt/audit原子性要保持，handled不产生业务成功证明。
6. 固定真实48+1工具覆盖清单及有效native/canonical schema检查，额外动态替换单独覆盖。测试不能用nil依赖fixture的失败当所有Tool不可用，也不能给stub伪造业务成功。

## 阶段2：各域确定性修复

- Goal/Intention：thin metadata默认，operation-specific stage skip/commitment abandon schema；缺review对已真实完成根作技术结清/superseded，对未完成根evaluation_incomplete/wait，不增missed/ineffective streak、不pause/abandon，不回滚合法根。adjust无真实plan降wait。root actual complete判定仍由原证据validator。read/no target/已terminal正常查询/选择/状态回执；Planner Core metadata只能绑定已消费snapshot，不用最新revision偷换CAS。
- Reply/Media：完成F17可选hint降级；published/prepared replay优先，same draft action failure/非选中/handled都能真实解释回到Writer，不再卡住；handoff过限复用已有请求+告警，不造asset。capability.request可排除无关敏感key/optional无效元数据，不因code/script名字拒绝全部提案。
- Read/Memory/Persona/Habit：no records、未知section、cursor末端、重复/unchanged转normal结果；可确定默认但不NLP猜事实，unknown时间/推断维持未知。habit重复无revision/编译、短hash安全prefix。明确source/CAS不适用返回handled未修改而非假成功。
- Wardrobe/Item：ID去重、替换槽与remove重叠去冗余、partial空/已达成no-op；return只在真实归还证据存在时already_returned，不能把lost/unavailable当已归还；item.stop可清除既有use且不被start的可用性/睡眠门误挡，同状态不制造revision/history；DB错误正确分类。歧义槽位不挑赢家，正常选择请求。
- Life/Schedule/Affect：inspect无目标/越界分页正常结果；scene end无event/current相同、presence empty/clear多余metadata、活动已resolved/current/notDue有可用当前/等待结果；intention.schedule planning-only分支不依赖不需要的planner。明确无法完成的状态用handled等待；真实购物/染发/剪发必须来自实际elapsed result与同一authority，不能软件补完成。affect event缺技术confidence可政策默认并记录，不能从聊天文字推断type，CAS/来源不写旧状态。

所有阶段保持已实施F15 Evaluation30min及Planner真空位、专用Goal/Visual/Owner分工（用户此前确认），以及真实取消/事务/外部依赖边界。保留项必须进最终逐tool清单，说明条件、原因和非fatal替代，不用“安全原因”泛称。

## 实施组织与验证

先完成共同结果/normalization API，再并行独立产品文件家族，避免跨代理改同一文件。主代理只写任务/spec/report并亲读根架构；trellis-implement代码，trellis-check独立核验。所有worker知道不是独自写库，不revert他人，无git提交/部署。

先真正行为RED（nullable、pre-schema、Kev No仍合法执行、review可选回滚、重复状态），再GREEN。证明handled不写/不Pending/不Goal成功，真实错误保真；确保应用入口而不只是函数Helper。实际49项调用覆盖按pure/schema/native controlled/PG/external分层，不把schema序列化或父容器PASS当实际业务执行。

可尝试仅/tmp下自建disposable PostgreSQL+pgvector来执行原受控PG/native suites：不得启动OrbStack/系统服务、读取未知DSN、连接已有DB或使用真实Provider；工具获取与数据完全任务独占、可清理。若不具备环境仍明确SKIP，不宣称全部业务实际通过。

## 当前执行组织与收口

按用户最新 AGENTS.md，公共层交接之后，领域修改与最终验证由主线程亲自完成；子代理仅做只读探索/独立核验。两轮只读审计给出的精确出处用于主线程修改，未再派领域实现者。F17/F18统一实现已落地，49行审计见tool-availability-audit.md；共有handled的canonical fresh-Prepare、preclaimed due Attempt、stale refresh缺口均已补回归。

实际自建/tmp私有PostgreSQL17.6+pgvector0.8.1+pgcrypto，只连接该cluster。完整PG探索失败不能作为全部业务通过；本轮新增受控PG/race逐项单列。旧F15/F16已在b24fdd1；本轮不自动提交、归档、push、部署或操作真实工作流。用户真实环境验收仍待完成。
