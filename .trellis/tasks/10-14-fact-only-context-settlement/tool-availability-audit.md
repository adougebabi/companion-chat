# 全 Tool 可用性审计与保留条件

基线 `b24fdd1`；当前分支 `codex/fact-only-reply-vision`。Conversation 目录38项，全局注册48项，加 Writer 私有 `reply.image.request` 共49个唯一名称。动态 Fact-only 替换沿用原名，另核验其不同 Schema。

本表是49项代码/恢复策略审计，不代表49项真实外部业务全部执行成功。执行证据独立记录在 `verification/tool-availability-results.json`；不可把 inventory、父测试 PASS 或 SKIP 当业务通过。

公共恢复统一采用 `handled`：Go error=nil、ErrorCode空、effect_applied=false、mutation_committed=false，保留 warning_code/disposition。纯读/Prepare可直接恢复；有业务事务时先回滚保存点再持久化技术回执。真实错误和已有真实失败结果不能伪装成功。

| Tool | 告警/程序兜底 | 保留的效果前提（不会猜出成功） |
| --- | --- | --- |
| `active_memory_event` | 可空/表示修复；普通时间、目标和生命周期冲突返回 handled，未知时间仍未知。 | 来源 fingerprint、原目标版本、conversation 和 Core 专属 expire 不赋予模型新的写入权。 |
| `actor.fact.record` | 合法 value:null；缺 assertion_type 按 inference；冲突无写入等待，真实 SQL 错误保真。 | 当前真实入站来源与实际 Actor、纠正对象归属；推断不得当成显式陈述或纠正依据。 |
| `actor.inspect` | 缺参数可查；无事实返回空集合，记录不存在与 SQL 故障分开。 | 只解析实际授权 Actor，不能读取其它 Actor 的私有事实。 |
| `affect_event` | confidence 默认0.5；缺/坏语义和版本冲突告警无效果。 | 真实 event.type、来源、当前状态版本；不从聊天猜事件或伪造情绪变化。 |
| `appearance.style` | 缺信息/不可用状态/CAS 等普通前提返回 handled。 | 临时造型不能冒充染发/剪发；未知真实发长不能补造。 |
| `capability.catalog` | 名称去重、批量、混合名称返回有效子集及 unavailable；Kev 建议不阻断执行。 | 未安装/其它私有 surface 无实际实现，不能凭名字授予执行范围。 |
| `capability.discover` | 同 catalog；超过8项按配置拆批，不整批报错。 | 保留实际安装范围与真实取消。 |
| `capability.request` | code/script 可保留；结构化移除真实 secret 字段并返回省略数；缺语义告警无效果。 | 只是 Owner 审阅提案，不能冒充已经安装或执行；不得持久化被识别的凭据。 |
| `conversation.reply` | 未对账可选 evidence hint 排除后继续 Writer；保持有效 hint；原 draft/发布回执重用。 | 真实会话/来源/私有 draft/lease/取消及发布权；实际 Provider 失败不能生成假正文或假送达。 |
| `goal.decide` | 普通输入/对象/版本前提告警无写入。 | Owner 专属治理；不能通过任意 Agent 声明完成目标或绕过 Planner 容量。 |
| `goal.evaluate` | 已终态/版本变化等告警无效果；真实排队不当作完成。 | 仅提供过的真实 Goal 和冻结版本；评估仍30分钟准入。 |
| `goal.evaluation.submit` | 技术文本不再过度必填；缺 Review 保留有效根评估；未知待证据；无进展有限收束。 | 真实私有 task/claim/目标/criteria/source；完成必须由原成功条件及证据验证。 |
| `goal.inspect` | 默认 list；detail 无目标降 list；无结果正常处理。 | 仍只读当前 profile/提供过的 Goal，更新读取不授权旧 token 的新版本。 |
| `goal.object.submit` | 无效/歧义 criterion 返回允许范围及待证据；业务错误保存点回滚为 handled。 | Stage/Commitment 属于当前 Goal；子对象结果不能冒充父 Goal 完成。 |
| `goal.plan.submit` | skip/abandon 不再继承 create 字段；已接受 target 重放；不适用状态告警。 | 真实对象/版本/依赖；不编造新 plan，不为终态 Goal 执行新计划。 |
| `goal.review` | 排队/同周期无新状态沿原回执；缺语义进入无效果告警。 | 真实本地日、窗口、水位；technical wait 不计错失/无效尝试。 |
| `goal_planner.commit` | 缺技术版本从此前已消费 snapshot 补；无快照则 needs_input；CAS 告警无写入。 | 真实 run claim、Owner 保护、真空位、并发版本和依赖；不能改写完成/证据。 |
| `goal_planner.query` | 默认 snapshot；无记录正常处理；查询次数不再强制 Agent 继续调用。 | 当前 Owner/profile 可见范围；真实读取失败不能当成空结果。 |
| `habit.decide` | 重复 append/同文 replace 在编译前 no-op；坏 index 不猜0；短 hash 不切片 panic。 | 真实 profile/source/compiled revision 和原 index；不能发布与新 Foundation 不一致的画像。 |
| `habit.inspect` | 空列表为正常结果；缺源与 SQL 故障不混为真实无习惯。 | 实际 speaking profile；不能以别的人格数据替代。 |
| `intention.decide` | 普通状态/目标/版本前提由公共保存点转 handled。 | 真实 Goal/Intention 生命周期、原任务/对象身份；不凭工具调用宣告执行完成。 |
| `intention.inspect` | 默认 list；无目标/无记录/分页前提正常 handled。 | 目标/profile 归属与现有实际活动关联。 |
| `intention.schedule` | 纯规划分支不再依赖 planner；非法生成计划无写入告警。 | 实际未来时段、真实原 Intention link 和行动意图；预约不是开始/完成。 |
| `item.use` | ID trim；同 activity start 不重置时间；stop 清除既有 use 不受 start 的睡眠/可用性门阻断；重复 stop no-op。 | start 仍需实际可用并持有的 object；不把衣物或未知物品称为使用成功。 |
| `life.activity.advance` | 多目标 needs_input；已 resolved no-op；结果不合约等待证据，无伪造物品/身体效果。 | 实际 elapsed/not_before、原 authority、真实结果及现有活动身份。 |
| `life.activity.start` | 普通输入/状态/版本前提告警无效果。 | 实际拥有或可获取的目标与活动类型；启动仅 accepted，不是购物/染发完成。 |
| `media.image.generate` | 普通输入/版本条件告警；handled 媒体结果允许原 draft 回 Writer。 | 真实 media intent/文件/renderer/任务作用域；accepted 不等于 ready/看过/发出。 |
| `memory.recall` | 输入表示修复；无匹配正常空结果；不把真实依赖失败伪装 no_match。 | 真实可见 Memory scope 和来源；不能查另一会话/Owner 的私有记忆。 |
| `memory_event` | revise 目标不存在/歧义/过期和生命周期 CAS 通过精确 code 树转 handled。 | 原对象、来源、修订版本和命令身份；不猜最近一条当纠正目标。 |
| `moment.publish` | 共享可选 hint 对账恢复；独立 feed Writer；原回执优先。 | feed/参与者范围、真实发布与 Writer 故障；不携带私聊策略/历史到动态。 |
| `persona.detail` | 默认 list；unknown section/cursor/source changed 无效果告警。 | current 与 historical Foundation 区分；不以旧身体/服装当当前事实。 |
| `persona.switch` | 拒绝/keep/no_change 转 handled；无效果不制造 mutation audit id。 | 实际已声明 profile、trigger/source/cooldown 和 CAS；不能造出切换画像。 |
| `persona.takeover` | 同 switch；rule 不适用等返回明确 no_change 警告。 | 真实已声明且适用的 rule/target/source；不自动新建授权规则。 |
| `presence_event` | clear 忽略无关字段；坏 TTL 用既有默认并警告；缺技术上下文等待。 | 真实 Actor/来源/当前 revision；不凭模型参数创建授权身份。 |
| `relationship.lookup` | 缺目标/别名不明 needs_input；授权范围内无 row 为 no_results。 | 目标必须在冻结范围且为实际参与者；关系行存在不授予访问权。 |
| `reply.image.request` | 第3次保留已有 handoff 并告警；坏输入 handled；不生成额外假图片。 | 仅私有 Writer；回执仅 requested_not_generated，不能称 generated/seen/sent。 |
| `scene_event` | end 无活动 no-op；普通状态和版本冲突等待/刷新。 | 真实当前 event identity/source；不关闭另一事件或制造外部活动已发生。 |
| `schedule.edit` | 缺模型技术 revision 绑定已冻结 Schedule；非法计划告警无写入。 | 真实选中 item、原 Intention link、历史/当前不可打断状态及 CAS。 |
| `schedule.inspect` | 默认 list；detail 无目标列清单；越界 cursor 返回末页。 | 真实 accepted schedule 与当前 profile，不以读取失败推断没有日程。 |
| `schedule.replan` | 确定性不合约生成计划转 handled；完整结果不做静默事实裁剪。 | 历史、真实日期/时区、执行中的 link/source 和 revision；不把坏计划写为 accepted。 |
| `visual_identity.commit_review` | 普通冲突走无效果等待；既有真实待配置/待文件回执保持可用。 | 真实候选文件/身份/视觉检查证明；不能审核未生成文件或冒用另一 session。 |
| `visual_identity.finalize` | 已完成原回执重放；未就绪返回实际等待原因。 | 真实 character sheet asset/完整 review 与原 session；不把 ready 检查省成成功。 |
| `visual_identity.generate_candidate` | 既有 intent 重用；不可生成/配置缺失返回实际等待。 | 实际 renderer、种子、真实媒体结果和 session；已失败媒体事实不改为无效果成功。 |
| `visual_identity.initialize` | 普通输入/状态/版本前提正常无效果处理，原排队回执保持。 | 当前 Owner/profile/初始化上下文；排队不是视觉身份已生成。 |
| `wardrobe.borrow` | 同 ID/effect 去重；无意义/版本前提无写入告警。 | 实际借入事实/来源/出借方；不能用同款描述当同一件物品或愿望当持有。 |
| `wardrobe.inspect` | 默认 list；detail 无 ID 列清单；outfits 补真实 next_cursor。 | 库存不完整不能推断现实不存在；只能读当前实体/profile。 |
| `wardrobe.outfit.save` | 普通输入/槽位歧义/版本变化返回无效果选择/等待。 | 真实物品集合/归属/原 outfit revision；保存不等于穿上或获取。 |
| `wardrobe.return` | 重复 ID 去重；真实 confirmed return 证据可识别跨 operation 已归还。 | unavailable/lost 本身不是归还证明；owned 不当借物，未知对象不宣称归还。 |
| `wardrobe.wear` | 相同 ID 去重；重叠 remove 丢弃；partial 空和已达成 no-op，无 revision/history。 | 实际持有可穿物；不同 ID 同槽需选择；full 空仍为真正脱下。 |

## 确实保留的硬边界及具体原因

1. **实际资源授权/私有任务范围**：所有 Tool 的应用 envelope，relationship 的实际参与者，Goal/Visual 的 task/session/claim，以及 draft/action 的真实归属。错误身份没有可用的受授权替代对象；程序不执行、不泄露其它范围数据。普通目标选择/版本变化已经转无效果告警；此处只保留真实跨主体/私有任务冒领拒绝。
2. **取消与失效的真实执行租约**：不能在调用已取消或任务已经移交之后提交旧任务结果。保留停止/原回执；新的真实任务可重新准备，不把旧 claim 改成新 claim。
3. **持久化/Provider/网络故障与内部损坏**：SQL/网络/真实模型调用不能凭程序补出已写入事实、正文或文件。事务回滚并保留真实错误和诊断；不能当无匹配或成功。
4. **同一命令身份对应不同 payload**：共有 ledger 与 Provider call ID 保留原回执身份，不覆盖、不二次执行；Goal 已接受 target 另有明确原回执重放。此项仍可能返回 identity conflict，需真实新业务 operation/call 身份，不能由模型冒用旧身份。
5. **真实已发生的失败业务结果**：例如已有 media intent 真正失败，必须保留失败事实，不能改成“没有执行”或生成成功。缺 renderer/asset/visual proof 使用原实际 waiting/unavailable 回执；不生成假文件。
6. **用户明确指定的业务准入**：Evaluation 每实例30分钟、Planner 真空位及合法 completion/day/Owner episode；保持正常等待、不消费未解决来源。不会用降级路径额外调用模型。

这些前提保护具体身份/真实效果；不是对普通 Schema、引用未对账、Kev No、经验容量、重复状态或缺空文本的全面拦截。普通缺语义返回 needs_input，实际未执行就明确写未执行。

## 验证与限制

- 新增固定49名称单测；nullable/union及70KiB参数有行为 RED→GREEN；Goal薄输入另有实际RED。
- 受控PG新增证明：业务保存点中途写入回滚、handled ledger重放、预领取 due Attempt suppressed结清且无失败计数/Goal进度、缺Review保留有效根、不可用item的stop及重复use。
- 真实数据库暴露的 Goal queue/workflow-intent/outcome/Planner repair SQL 参数类型问题已补显式类型。旧Cadence Fixture使用完整Goal并显式cast；没有移除生产频率/容量检查。
- 全量PG第一次运行不是绿灯；具体失败名称、摘要和基线抽样对照保留于验证JSON。本文不把抽样基线复现推广为全部失败都已证明来自基线。
- 无真实Provider、ComfyUI/对象存储、Redis或Temporal运行；无部署、生产数据修改、推送。任务独占测试PostgreSQL最终停止。

## 最终可审阅状态

本轮独立快照：Go全包1829 PASS/564 SKIP/0 FAIL；Core/Workflow race 1489 PASS/499 SKIP/0 FAIL；受控PG/race重点92 PASS，最终完整性另10 PASS；Go vet/build和前端类型检查/128测试/build通过。完整PG尚未全绿：b24fdd1基线275个FAIL事件，中间当前快照160个；24个current-only叶失败均在重点回归重跑并通过。其余未接受，见机器报告原始断言。

工作区另一项private-chat任务正在改相同分支及三个共享reply文件。本轮未覆盖它；按基线加本轮hunk构造独立可提交快照并记录源码hash。commit-plan和owned patch以该快照为准，不能git add整个共享文件。独占PostgreSQL服务已停止，安装和数据目录已删除。用户真实环境验收、提交批准仍待完成。
