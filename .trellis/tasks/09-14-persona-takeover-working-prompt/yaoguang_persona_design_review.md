# 摇光多重人格重构方案：设计审核

## 结论与审核范围

**审核结论：总体方案可保留，但当前不建议通过 G0、直接开始实施。先修订设计和实施计划中的契约冲突。**

本次读取了压缩包中的全部 11 个实质文件：8 份 Markdown、`task.json`、`implement.jsonl`、`check.jsonl`。压缩包通过 CRC 检查，JSON/JSONL 可解析，Markdown 代码围栏成对，主要文档没有明显断尾。`request.md` 与上一版完整任务提示词均为 360 行，差异仅为 Markdown 表格对齐，没有发现需求段落因中断缺失。`task.json` 仍为 `planning`。

**附件没有 Go 源码、Git diff 或实际测试输出。以下审核确认的是文档内部一致性、与已确定需求的符合程度，以及设计是否足以支撑其保证；不能据此断言仓库中的具体函数已经被独立核实。**

下文的 `design.md:Lx–Ly` 等均是压缩包内文档行号。文档引用的 Go 文件位置仍需本地 Agent 在实际代码上复核。

源文件副本位于本报告旁的 `persona_review_sources/09-14-persona-takeover-working-prompt/` 目录。

## 可保留的设计

- `active_profile_id` 与 `reply_owner_profile_id` 分离，takeover_once 不通过临时改写持久人格来实现。
- A 的输出是候选，Judge 在正式动作执行之前判断，B 接管后不再反向仲裁。
- Working Persona 与 Takeover Control View 从同一人格数据派生，不复制两套独立人设。
- 不引入新的 Agent 框架、MCP 或并行私聊发送服务。
- 主生成、Judge、接管生成、QUERY 合成分别记账；测试报告区分真实执行和环境跳过。

这些方向不需要推倒重来。问题主要发生在将原则翻译为具体 schema、SQL、恢复分支和投影调用时。

---

## F01｜P0：Main Prompt 又承载接管规则，切换职责没有真正迁出

**依据**：`design.md:L279–296`、`implement.md:L111–117`；对照 `request.md:L202–214`。

设计明确将 `persistent_switch.rules` 和 `takeover_rules` 放入普通 Main 的 System Persona，并在“实例存在切换入口”时保留 `personality_decision`。这不是单纯保留人格标识：它仍要求正常主生成看到控制规则，且有机会提出持久切换。

对于本次目标角色卡，“存在规则”几乎一直成立。条件式保留不能消除 Main 与 Judge 的职责重叠。B 接管路径又复用 `preparePersonalityDecision`，后续通用结算仍调用 `applyPersonalityDecisionPlanTx`，因此“不在 turn_takeover.go 直接调用写人格函数”无法证明 takeover_once 不会改变 persistent active。

**修正要求**：

1. 普通 A/B 生成只负责当前发言人格的候选行为；不注入整套接管条件，不暴露本轮无权提交的持久切换输出。
2. Judge 独占本轮接管条件；Persistent Switch 使用独立、明确的控制入口。
3. 如果因既有产品语义必须暂时保留某个持续切换入口，要逐场景命名，不用“有规则的实例一律允许”代替权限边界。
4. 在候选校验和结算权限处禁止 takeover 产生 persistent switch，不只检查 Prompt 或函数名。

**验收**：B 返回一个合法格式的持久切换提案时也不得改 active；A 的普通请求没有 B 的接管规则全文；确定性的早晚切换仍通过明确控制入口工作。

## F02｜P0：Judge 所见文本不一定是实际发送文本

**依据**：`design.md:L61`、`L168–186`；`research/inventory-prompt-schema.md:L85–86`、`L104–116`；`research/inventory-turn-chain.md:L98–103`。

方案称没有 `visible_reply` 旁路，但盘点同时记载 `response_plan.visible_text`、根级 `visible_text` 和 `conversation.reply.arguments.text` 是并存来源，发送端前两者优先。预览却只提取 reply 工具文本。

因此，可能出现：Judge 审核“我不是不在意你”，实际送达“随便吧”。一个网络发送管线，不等于只有一个消息内容权威。

**修正要求**：在 Judge 之前，形成唯一 canonical 候选消息内容，预览与最终执行使用同一冻结值。普通 Tool 回复优先维护已有 reply 契约；旧字段只在一个明确的兼容入口归一化。有两个来源且内容冲突时明确拒绝，不能不同阶段采用不同优先级。

QUERY 无 Tools 合成结果可以沿既有适配进入同一发送管线，但不能变成普通动作轮可绕过 preview 的第二权威。

**验收**：构造 root、response_plan 与 reply.text 冲突的输入，证明不能“审一条、发另一条”。

## F03｜P0：只覆盖 frozen payload，不足以成立“胜出候选唯一执行”保证

**依据**：`design.md:L15–17`、`L137–166`、`L318–349`；`implement.md:L163–195`、`L222–233`；`research/inventory-turn-chain.md:L49–60`、`L122–143`。

当前设计把 `status='frozen'` 当成“未执行”的充分条件，并用 `UPDATE ... WHERE id=? AND status='frozen'` 加 `RowsAffected()==1` 表示并发安全。这个条件只说明行仍处于该状态；文档并未证明准备、查询、外部动作进行期间它不会继续为 frozen。两个 worker 在同一状态下各更新一次，也都可能得到 RowsAffected=1。

还存在四个具体缺口：

- Judge=true 后的决策直到 B 生成结束才与 B 候选一起记录，未描述“Judge 已完成、B 尚未冻结”时的恢复。
- 恢复只列出 takeover_b 和 Judge 错误降级，没有明确 kept_a、skipped、Judge 运行中、B 生成中等情况。
- Replace 示例只替换 decision 与 invocations；frozen 中还有 context snapshot、结果等相关状态，并且内存中已经缓存了 responseMode、visible、personalityPlan 等派生值。
- `implement.md:L174–175` 的示例先从旧 frozen 读取 decision，再 LoadFrozenTurn，顺序直接与“重载最终候选”目标不符。

**修正要求**：不强制新表，但必须有可检查的阶段和执行资格：A 已冻结 → 仲裁结果已冻结 → 必要时 B 已冻结 → 最终候选可执行 → 执行/完成。调度、恢复和正式执行都必须要求最终胜出者已经确定。

Judge 的已确认结果先持久化，再调用 B；B 候选、owner、控制决策与所需的派生状态应一致提交。覆盖使用预期候选版本/阶段及既有 worker ownership 或租约校验；不能只有 id+frozen。

选出胜出者后统一重建或重读所有执行用变量，不能只换 JSON 而留下 A 的本地变量、prepared 数据或上下文。

A 的 assessment/proposal 审计记录可以保留，但必须检查消费者，证明它们不会以 accepted 事实进入反思和记忆，而不只是禁止读取 `takeover.rejected_candidate`。

**验收**：至少在 A 冻结后、Judge 决策冻结后、B 返回但尚未冻结、胜出者冻结后、准备后等边界注入中断；确认无旧候选执行、无已冻结决策重判、无错误文本发送。

注意：对“模型响应到达但持久化前崩溃”的窗口，不能无依据保证物理请求恰好一次。应区分逻辑阶段、实际请求尝试和不确定结果，明确有界重试或失败策略。

## F04｜P0：B 只换 Working Persona，不足以保证 B 的上下文和写入归属正确

**依据**：`design.md:L115–135`、`L248–275`；`implement.md:L104–115`、`L166–190`；`request.md:L153–165`；`research/inventory-tests-specs.md:L79–83`。

实施步骤写清了 B 的人格投影，但没有同样明确：B 的关系、记忆、goal/intention、reference index、影响来源及 Capability Context 是否全部使用本轮 reply owner。已有盘点明确指出多处读取按 active profile 过滤。

这会产生“用 B 的口吻，读取 A 的私有人格记忆，并把 B 的关系变化写回 A”的风险。persistent active 不能改，不代表读写时还应一律用 persistent active。

另外，`resolveTurnPersonaScope` 放在 A 生成完成后再读当前 runtime，不能自动证明这个 active/revision 就是 A 生成时所使用的版本。

**修正要求**：在 A 生成前固定 turn snapshot；区分 persona 内容修订、runtime 切换修订和 overlay 修订。接管时共享事实基线不变，明确为 B 重建/选择 profile-scoped 读上下文、参考索引和权限范围；执行和持久化采用冻结的 reply owner 或被显式标记的 shared scope，不回读当前 active 来决定写入主体。

**验收**：设置 A-only、B-only、shared 三组关系和记忆。B 接管能看到 B/shared，不错误读取 A-only；B 的动作不会写入 A 私有作用域；下一轮 A 保持原来的视角。

## F05｜P0：声称已完成 Candidate 校验，但盘点中的校验仍在 Judge 之后

**依据**：`design.md:L166`、`L17`；`research/inventory-turn-chain.md:L148–160`；`implement.md:L167–190`。

设计要求 A/B 都先完成工具存在性、参数、权限和基本业务校验才进入 Judge；盘点却把这些校验的一部分定位在 Execute、Prepare 和结算阶段。将 Judge 插在 Prepare 之前，不会自动把这些校验也移到之前。

**修正要求**：明确一层无副作用的 Candidate Validation，复用 registry、codec 和 schema/参数校验；执行所需的外部预检仍留给胜出候选。校验与 IO Prepare 不要混为一谈。

**验收**：不存在工具、非法参数、空 reply、超长 reply、无权动作等输入应在进入 Judge 前按契约失败，不能由 Judge 抢答掩盖。

## F06｜P1：QUERY+ACTION 混合轮是否允许仲裁，文档前后不同

**依据**：`request.md:L167–188`；`design.md:L301–315`、`L363–365`；`implement.md:L180–181`、`L209–218`。

需求只规定混合轮不 continuation；设计矩阵额外写了混合轮“不叠加 takeover”。实际伪代码却只跳过 `responseMode == query_continuation`，混合 final 轮仍可能被仲裁。

**推荐收敛**：互斥的是“结果依赖型 QUERY continuation 路径”和“takeover 路径”，不是“出现任意 QUERY 工具就禁止 takeover”。已有有效、无需查询结果才能成立的可见候选，可以沿普通候选机制仲裁；未返回结果不得被当成已知事实。纯查询续调用路径则整个跳过仲裁。修正文档、判断函数和测试矩阵为同一语义。

如果产品确实选择所有 mixed 轮都不仲裁，也必须作为新增限制明确写入 request，而不能让实现跟随另一份相反的矩阵。

## F07｜P1：仓库没有实例数据，不代表用户角色卡没有规则可迁移

**依据**：`design.md:L70–101`；`implement.md:L63–95`；此前用户提供的实际评估报告中的 `forced_activation.conditions/target_profile/trigger_profile` 示例。

“仓库内没有具体 forced_activation 数据”可以是盘点结果，但只能支持“当前未拿到持久化实例”，不能支持“用户没有待迁移规则”。此前实际角色卡就是没有 mode、只有条件和 source/target 的开放对象；按照新计划缺 mode → unclassified，旧卡依然不会接管。

**修正要求**：以用户实际角色卡/持久化实例导出构建回归 fixture；明确旧形状到规范规则的迁移。含义不明保留并提示人工确认；已经明确的接管语义则必须有可执行入口，不能全体归入诊断而称迁移完成。

新规则结构目前只有 TargetProfileID，没有明确 SourceProfileID；选择函数只检查 target 非当前人格。需要检查来源人格、enabled、现有 cooldown 等规则资格。若同一 B 有多条可触发条件，不应只按 priority 取一条后忽略其他条件；可给唯一 B 提供有预算的合格规则集合。

稳定 ID 还需区分规则身份和规则修订：全文 hash 可用作内容版本，但修改一个条件就换永久 ID 是否符合语义应明确。`switch:<id>` 这种新前缀若进入 Prompt，validator 必须解析同一个规范 ID，而不能继续只匹配原始 `<id>`，否则会重现 trigger_not_found。

**验收**：实际 A/B 卡在迁移后可产生合法接管候选；不能只用新写的理想 typed fixture 证明成功。

## F08｜P1：持续切换入口不存在，却在验收中写成“保持原有即可”

**依据**：`research/inventory-persona.md:L40–58`；`design.md:L87–101`；`implement.md:L201`、`L254–260`；`prd.md:L64–65`。

盘点称 WakeUp/Reflection 只读取 active，不切换，服务端无确定性时间切换；但实施计划又要求回归证明“时间/唤醒/反思入口仍能更新 active”。这两种陈述不能同时直接用于验收。

**建议**：以代码复核结论纠正需求中“已有”的假设。用户实际用早晚切换调试，应实现最小的确定性时间窗口求值，复用已有持久切换/CAS，不新增 LLM 和调度框架。将它在已有 turn/wakeup 时间边界调用的行为说清楚。

更复杂的 semantic persistent switch 需指出真实消费入口；不存在则明确新增或未实现，不可“只读入口不改动”同时宣称该能力已保留。不能靠继续把切换规则塞回普通 Main 来掩盖。

## F09｜P1：Judge 独立记账，不等于独立低成本配置

**依据**：`design.md:L19`、`L190–244`；`implement.md:L139–147`；`research/inventory-prompt-schema.md:L160–167`。

新增 role 映射 `generic_llm` 能支持分类统计，但这本身不能证明可选择更小模型或具有独立输出预算/超时。文档也承认 enableThinking=false 只会省略开启字段，不是显式关闭；因此不得称“已实现不思考”。

`decision_code: string` 未限定长度，所以“两个短字段天然约束输出上限”的说法不成立。超预算后丢弃输出，也不同于请求前限制生成成本。

**修正要求**：落实名义之外的 Judge 配置：实际 model/endpoint 绑定或明确复用主模型、timeout、输入预算、最大输出、已支持的采样参数和 thinking 控制。以捕获真实 wire payload 为验收；不支持关闭时明确未关闭，不杜撰字段。

可选诊断字段使用短 enum 或 maxLength；无必要就只保留 boolean。记录主模型和 Judge 实际调用参数与成本，不能仅凭 role 名称宣传便宜。

## F10｜P1：静态函数出现次数不能代替运行时调用预算

**依据**：`design.md:L418–434`；`implement.md:L16`、`L188`、`L217`、`L264–270`。

把 B 的调用放到新文件以维持 mutations.go “只出现一个调用点”，只能约束代码位置，不能证明没有循环、递归、重入或 Provider 重试。一次调用点也可能运行很多次。

**修正要求**：保留静态守卫作为补充；增加 turn 级运行时阶段/调用预算，对正常 A、B、QUERY 合成与恢复路径计数。明确 logical invocation 与实际 HTTP attempt；既有 Provider 重试会产生额外物理请求时必须统计并设定边界。

**验收**：Fake Provider 记录实际调用序列；异常恢复后不能重新走已经完成的逻辑阶段。无法判定上一尝试是否完成的窗口采用明确策略，不声称物理请求绝对恰好一次。

## F11｜P1：Working Persona 仍需要语义保真与明确的大小目标

**依据**：`design.md:L259–275`、`L438–442`；`implement.md:L104–123`；`research/inventory-prompt-schema.md:L120–135`。

设计说保留关键人格，但实施字段重点仍是 traits.* 数值和几个 categorical 字段。若“慢热、真诚、笨拙、专业领域自信”的描述性 traits 只有数组形态，转换为数值或报错不能替代将语义准确交给模型。

另，沿用 SystemTokensCap=16384 等大硬上限，只能防超限，不能证明 Prompt 已经收敛。

**修正要求**：明确描述性 traits/关键约束的规范载体与投影保真；不通过臆造数值替代原词。为同一实际复杂卡记录重构前后 full request 大小、Working Persona 大小和 Judge 大小，并设置可配置的工作预算；不要以“仍装得下”作为缩短成功。

非激活 profile 扩大时，Main prompt 不随其全文同步扩大；Judge 也应有独立输入上限。

## F12｜P1：事务内外的盘点结论自相矛盾，不能直接作为安全保证

**依据**：`research/inventory-turn-chain.md:L36`、`L122–127`；`design.md:L347`；`request.md:L244–258`。

调用链表说 settleDeferredCapabilitiesTx 期间有外部调用且处于事务窗口内；同一报告后面又称“能力 Execute 均在事务之外”，并把“事务窗口内的外部 IO”描述成近似事务外。两者不是一回事。

**修正要求**：对实际 IO 逐项给出准确发生位置。纯 DB 状态修改在短事务内是合理的；网络/图片/外部服务在持有该事务期间运行则不满足原任务要求。若这属于已有机制限制，要明确本次如何避免、怎样复用既有外部执行阶段，或列成未解决阻塞，不能以文字解释抹平。

本条不要求新建大型事务框架，但要求不能承诺不存在的原子回滚与恢复能力。

## F13｜P1：research 的设计建议与主设计冲突，且被再次加载为实施上下文

**依据**：`research/inventory-turn-chain.md:L184`、`L192`、`L204`；`research/inventory-persona.md:L164–173`；`implement.jsonl:L9–11`、`check.jsonl:L8–9`。

冲突示例：

- turn-chain 报告建议由 Judge 生成 B 的 decision，而主设计要求 Judge 只给 boolean、B 独立主生成。
- 同报告建议接管时取消“正在进行的 A”，但目标流程中 A 已生成完毕。
- 同报告讨论接管时覆盖 QUERY continuation 状态，而需求明确两者不叠加。
- persona 报告建议 takeover 挂到修改 active 的入口、同时维护三个 Persona 出口，与 takeover_once 和唯一 Working Persona 相反。

这些文件又被 implement/check context JSONL 显式列为依据。因此即使 request 完整，后续 Agent 仍可能读到多套互相竞争的方案。

**修正要求**：research 保留代码事实；将已被否决的“建议/推断”标记 superseded，并写明替代设计。明确 request > 经评审 design > implement，research 的旧建议不是另一个架构权威。

清理两个 JSONL 的 `_example` 占位行。是否添加 design/prd 到上下文清单应检查当前任务加载器，不能凭本包推断加载器一定会或不会自动注入。

## F14｜P2：基础复核与测试证据需要降低过度断言

**依据**：`research/inventory-persona.md:L142–155`；`implement.md:L150–153`、`L241–250`；`research/inventory-tests-specs.md:L194–199`。

1. “时区问题不成立”的依据检查的是 canonicalTimezone 简单 switch，而旧问题是历史 timestamp 格式化丢时区。不能用前者推翻后者；应改成该具体路径尚未充分复核，或提供真实历史序列化测试。TOON 是否递归丢标志也应看对应递归调用链，而不是仅证明最后的表格渲染是扁平的。
2. Fake Judge 固定返回 false，只能验证数据布局/调用流程，不能证明真实模型不受提示注入。把两类测试分开报告。
3. 无 GO_CORE_TEST_DATABASE_URL 时大量数据库集成测试会跳过。文档对此已有诚实说明，应保留；最终 G1/G2 不得用整体 go test 绿色代替关键数据库链路确实运行。
4. Prompt 盘点的 role 表把 recent 全写成 user，而测试盘点描述 user/assistant 真实角色。需要以实际 wire message 或代码测试澄清，不能把表当已验证输出。
5. implement.md 的多个 `cd apps/core-go && ...` 连续粘贴运行会改变后续相对目录。运行说明改为各自子 shell 或已有 go -C 形式。

这些不应抢走多人格主线，但也不能以“复核完成”掩盖证据不足。

---

## 对 G0 三个问题的建议

| 待定项 | 审核建议 |
|---|---|
| 确定性时间规则只分类还是实现 | 实现最小早晚时间窗口求值，复用现有切换持久化与已有时间边界；不新增模型调用或调度框架。仅分类不能满足用户当前调试场景。 |
| takeover_rules 与 forced_activation | 运行时只消费一种规范结构；旧 forced_activation 作为受控迁移输入。可以有多个输入形状，但不能有两套执行语义。必须用实际角色卡证明迁移。 |
| 无新表，写 frozen payload | 可以，但不批准“只有 status=frozen + 两字段覆盖”。增加必要的 payload 阶段、胜出候选标识、预期版本/执行资格守卫；是否新表不是正确性的核心。 |

## 建议的下一步任务（交回本地 Code Agent）

先不要开始业务代码实施，也不要重新写一整套更大的架构。保留本次需求与盘点的有效部分，完成以下设计收口：

1. 修正 F01–F05 的核心执行契约：Main/Judge 权限、唯一回复文本、候选资格与恢复阶段、B 全链路 profile scope、无副作用校验。
2. 明确 F06–F08 的产品路径：QUERY 互斥矩阵、真实角色卡迁移、实际存在/新增的持续切换入口。
3. 将 Judge 参数、运行时调用预算和 Prompt 收敛目标落实到可测试配置，而非 role 名称、函数出现次数或大窗口不溢出。
4. 统一 design、implement、research 中冲突的建议；保留证据，不把旧建议继续当执行要求。
5. 对每一项修改列出“原文位置 → 修改后的契约 → 自动化验收”。同时补一张阶段恢复表和一张 A/B 作用域读写表。

完成文档收口后，再进入 Foundation/Migration。无需推翻当前方案，也无需扩大为新 Agent OS；重点是让已同意的机制在具体执行边界上真正成立。
