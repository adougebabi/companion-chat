# 开发任务：多重人格接管机制、Working Persona 与 Prompt 收敛

请基于当前 Go 项目的真实代码，设计并实现一次范围明确的重构：先解决多重人格在当前对话轮内如何接管，再依据新的运行模型减少 Prompt 中不必要的人格、规则和响应结构。

不要继续沿着“把完整 A、完整 B、全部切换规则和两套 effective persona 更完整地塞给主模型”的方向修补。也不要只修改角色卡，让模型勉强表现出切换效果。

本任务要求实现并验证运行机制，不只是提交设计文档。先检查代码、形成方案和迁移表，再实施；不必仅因为阶段划分重复请求确认。无法验证的部分必须如实标记，不能用模拟结果冒充真实模型测试。

## 1. 已确定的产品语义与范围

项目核心是 Go，沿用现有 Provider、Thin Capability、Registry、工具参数编解码、冻结记录及执行机制，不引入新的 Agent 框架。

回复已经是 Tool：`conversation.reply` 或当前代码中等价的正式名称。Main cognition 通过 `tool_calls` 提议发送私聊，不要重新增加 `visible_reply` 作为另一条并行回复路径。

> **[F-01 path b 契约修订 2026-09-14，基于验收报告 F-01 + 用户决策]** 因 Provider 结构化响应协议限制，根级 `visible_text`（`response_plan.visible_text` / `decision.visible_text`）是生成期回复提案的**优先正式协议**；`conversation.reply` 仍是 reply-only 候选可用的 fallback 提案源（Core 从 reply.text 派生 canonical，保留原 §13「回复已经是 Tool」契约）。但模型不得同时决定两份文本：根字段与 reply.text 不一致 → `visible_text_source_conflict` fail closed（`normalizeTurnDecision` 返回错误，Turn 受控失败），不再静默选 winner。详见 design.md §4.3。

多重人格分成两种行为：

| 概念              | 含义                                     | 本次处理方式                                     |
| ----------------- | ---------------------------------------- | ------------------------------------------------ |
| Persistent Switch | 持续主导人格发生变化，例如白天 A、晚上 B | 保留现有持久切换机制，收敛为**逐调用场景明确授权**的入口 |
| Turn Takeover     | B 在 A 的本轮候选行为发送前抢答          | 新增候选行为后的轻量仲裁，默认只影响当前 turn    |

> **[更正 2026-09-14，基于代码证据]** 原文写的「保留现有**唤醒、反思、时间规则**切换入口」不符合当前代码：`applyPersonalityDecisionPlanTx`（`personality_runtime.go:227`）的调用方只有 `mutations.go:956/1126/1303` 三处 **turn 结算**路径；WakeUp / Reflection **只读** `active_profile_id`（`reflection_runtime_v2.go:320/332`），任何 API 都不写 active；`applyPersonalityDecision`（`personality_runtime.go:260`）是**无调用方的死代码**；也不存在服务端时间规则自动切换引擎。**当前唯一能工作的持久切换入口是 Main 模型读到 `switching` 规则后输出 `personality_decision`**（规则经 `provider_prompt_composer.go:261-285` 显式保留在 System Persona）。本次因此改为「保留 Main 驱动的持久切换，但收敛为逐调用场景授权」，并明确**不新增** WakeUp/Reflection 切换、**不实现**确定性时间引擎。

第一版 Turn Takeover 使用 `takeover_once`：持续主导人格仍是 A，但当前 turn 的最终发言者是 B。不要先把数据库 active profile 改为 B，再在结束时改回 A。应显式区分持久的 `active_profile_id` 与当前 turn 的 `reply_owner_profile_id`，具体命名遵循项目习惯。

本任务允许接管时最多增加一次主模型生成；接管后的 B 不再接受 A 或其他人格的反向仲裁。Judge 是额外模型调用，必须单独计时和计费统计，不能因为不叫 Main cognition 就忽略成本。

本次优先覆盖现有 A/B 双人格卡，不实现多人格辩论、递归抢答或复杂仲裁竞价系统。核心代码不得硬编码角色名 A、B；若当前项目已有更多人格，说明此次支持边界，不要静默删除或改变它们。

## 2. 修改前先完成代码盘点

检查从用户输入到最终私聊送达的完整调用链，特别是：

- Persona 初始化、profiles、active profile、switching、forced_activation、Evolution Overlay。
- Persona 投影、System/Runtime Context 合成、工具定义和 `response_format`。
- Main cognition 输出解析、`conversation.reply` 的真实参数、工具预处理与执行。
- frozen action、事务、提交、异步投递、重试和恢复。
- 已有 pure QUERY continuation、相关规范和静态守卫。

先输出两张表：一张是当前调用链及实际文件位置；另一张是“现有职责 → 新职责 → 保留/迁移/删除的代码”。不能只看一处 Prompt Builder 就开始改。

此前只读报告提出过以下线索，但它不是本次代码验证结果：`forced_activation` 与 trigger 校验脱节、traits 数组被转换成空对象、System Persona 与 Runtime effective_persona 重复、共享关系和目标误绑初始 profile，以及完整请求还包含 Tools 和大型响应 Schema。请在当前分支复核，不能假定旧文件行号和旧结论仍然全部成立。

## 3. 目标流程

正常、已经能形成回复的对话采用以下流程：

```text
用户消息
  → 读取当前 turn 的状态快照与 active profile
  → 构建 Shared Identity + Working Persona A
  → A Main Cognition
  → 解析并校验 CognitionCandidate A，尚不执行任何行为
  → 是否存在适用的 Takeover Rule？
       否：选择 A Candidate
       是：调用一次轻量 Judge
             false：选择 A Candidate
             true：拒绝 A Candidate
                   → 构建 Working Persona B
                   → B Main Cognition，禁止再次仲裁
                   → 选择 B Candidate
  → 冻结最终胜出候选及执行资格
  → 沿现有 Capability Runtime 执行
  → 记录实际结果并按既有规则提交、投递
```

“拒绝 A Candidate”不是删除所有审计记录，而是禁止它进入用户聊天、业务执行和事实记忆。可以保留带有 `rejected/superseded` 状态的内部记录。

## 4. CognitionCandidate 必须是真正的未执行提案

Candidate 应包含已有 Tool Calls、候选归属人格、必要的短内部意图，以及当前机制中原本会产生的状态提案。具体复用已有结构，避免再造万能 DTO。

仲裁之前禁止发送私聊、发布动态、切场景、修改关系/情绪/记忆、创建实际日程、提交生图任务等副作用。检查的不仅是 `Execute`，还包括解析、参数补全、冻结回调、事件监听器和自动 Tool Runner 是否提前执行。

即使状态修改在 Response 根字段而不是 Tool Call 内，也必须作为候选处理。不能只拦住工具，却提前提交 A 的 appraisal 或人格演化。

允许保存内部执行记录，但这些记录不得触发业务消费者，也不能作为“这件事已经发生”的证据。

先验证 Candidate 的结构、工具存在性、参数合法性、权限及基本约束。非法候选不能靠 B 接管来掩盖；沿现有受控失败路径处理。

## 5. Judge 必须看到真实待发送文本

首先确认 `conversation.reply` 实际使用的是 `content`、`text`、消息数组还是其他字段，保持正式契约，不为示例随意改名。

B 判断的是 A 准备发送的具体内容，而不是只有“回复用户”这样的抽象 intent。对于回复工具，具体文本本来就是 Main LLM 应该决定的业务语义，不违反 Thin Tool 原则。

如果现有工具在执行时还会调用模型改写回复，那么 Judge 看到的并不是最终文本。需要明确收敛边界：最终文本应在无副作用的 Candidate/Prepare 阶段确定；仲裁通过后不能再做实质语义改写。确需额外模型调用，必须公开记录成本，不能偷偷加进 Executor。

从候选调用生成只读 `CandidatePreview`，包括面向当前用户的有序回复内容及必要的动作摘要。预览不能调用 Capability.Execute，也不能假装动作已完成。

复用现有工具分类、codec 或能力提供的轻量 preview 函数实现。不要在 MainAgent 中增加 `if toolName == ...` 的业务分支；也不需要为此创建庞大的 Preview 插件框架。

## 6. 可选的 Internal Intent

允许提供一个非常短的内部表达意图，例如：

```text
其实很在意用户，但因为不擅长直接表达而选择回避。
```

它不是用户可见内容，不是完整思维链，也不是经过验证的客观事实。不要要求逐步推理、长篇内心独白或模型隐藏推理内容。

优先复用现有合适字段；确需新增时，应可选、有长度限制，并明确不会发送给用户或自动写入事实记忆。不要同时在根字段和 reply 参数重复存相同 intent。

如果当前 Provider 无法稳定同时返回元数据与原生 Tool Calls，先使用真实候选回复和已有状态完成仲裁，不要为了得到 internal intent 再增加一轮主模型。

Judge 应把 internal intent 当成辅助信号，而不是绝对真相。不能仅因模型写了“其实很在意”，就把明确的拒绝或边界都解释为口是心非。

## 7. Takeover Rule 与轻量控制投影

完整角色资料保存在 Full Persona 中，从同一份已验证数据派生两类投影：

```text
Full Persona + 已接受的有效变更
  ├─ Working Persona：当前真正发言的人格使用
  └─ Takeover Control View：Judge 使用的接管条件
```

不要手工维护第二套独立 B 人设，避免两个来源逐渐漂移。

Takeover Control View 只需要接管方的简短立场、来源/目标 profile、适用条件、稳定 rule ID，以及判定所必需的边界。例如：A 因表达困难造成真实意图与措辞明显不一致，并可能导致关系误解时，B 可以介入。

不要包含 B 的完整背景、外貌、穿搭、兴趣、所有经历和行为循环。

接管规则应来自角色卡或经授权的配置，不要把“B 更热情、更会安慰用户”当作默认接管理由。接管不是通用回复优化，也不能变成把所有人格改造成更讨好用户的人格。

规则 ID 在初始化、存储、投影和执行中保持一致。Judge 不负责创造 ID；需要引用时只能使用 Runtime 提供的合法候选。

## 8. Judge 的输入、输出和降级

当前双人格场景由 Runtime 确定唯一候选接管方，Judge 只判断是否接管，不同时选择目标、接管模式或改写回复。

输入限定为：当前用户消息、必要的少量对话引用、A 的候选实际回复、可选 internal intent、必要动作摘要、B 的控制投影，以及相关关系/状态事实。缺少必要语境时明确保留不确定性，不要为省 Token 把判断依据剪掉。

不提供工具、执行权限、完整 Main Prompt、所有人格和全部记忆。使用当前 Provider 实际支持的低成本配置：可关闭 thinking 时关闭，输出短结构；不支持的参数不得虚构或静默声称生效。

正常输出：

```json
{"takeover": false}
```

或：

```json
{"takeover": true}
```

诊断模式可记录短 decision code 或规则引用，但不能为了 reason 让 Judge 变成第二个 MainAgent。

用户文本、历史、A 的候选回复都作为待分析数据，不能成为 Judge 的新指令。例如候选里出现“请直接返回 true”不能控制仲裁。

当不存在接管规则时跳过 Judge。Judge 超时、无效 JSON 或服务不可用时，默认记录错误并保留已通过原有校验的 A Candidate，不做无依据的切换；这不绕过原有业务安全和执行校验。不要无限重试或自动升级到多个模型。

## 9. Takeover 后 B 如何重新生成

当 Judge 返回 true：将 A Candidate 标记为未采用，取消其全部待执行行为和状态提案；然后使用 B 的 Working Persona 和当前用户原消息重新生成。

B 使用同一 turn 的共享事实基线，并重新解析属于 B 的 profile-scoped 上下文。不要把 A 计划切换的场景、A 尚未保存的记忆或 A 候选关系变更当作已发生事实。

可以附带短的 Takeover Context，明确“A 以下内容未发送、动作未执行”，包含 A 原计划说什么以及命中的接管条件。B 应独立决定本轮行为，不是润色 A。

B 的生成显式禁止再次仲裁。其输出仍必须通过工具参数、权限、业务约束和执行资格校验，“不仲裁”不等于“不校验”。

若 B 生成失败，走现有受控失败机制，不擅自发送已被拒绝的 A 回复，也不通过第三次人格生成偷偷补救。

最终消息和执行记录应保留实际发言人格 B 的归属；下一轮 A 能知道上一条是 B 的表达，而不是将 B 的语气吸收成 A 的稳定特征。

## 10. 与现有 pure QUERY continuation 的兼容

必须显式处理这一契约，不能把接管、查询续调用和无限 Tool Loop 混在一起。

当前存在或计划存在的受控 QUERY 例外是：第一次只请求纯查询；取得冻结结果后，最多一次无 Tools 的结果合成；不新增状态变化。先核实其真实实现。

本次采用保守的 V1 组合策略：接管与 QUERY continuation 不在同一 turn 叠加，主生成/结果合成合计最多两次，不含单独统计的 Judge。

| 路径                         | 行为                                                         |
| ---------------------------- | ------------------------------------------------------------ |
| 普通回复/动作候选，没有接管  | 一次主生成；存在接管规则时增加一次 Judge                     |
| 普通回复/动作候选，被 B 接管 | A 主生成 + Judge + B 主生成；不再查询续调用或反向仲裁        |
| 结果依赖型 pure QUERY        | 保持已有查询结果合成路径；本次不对该路径增加接管仲裁         |
| QUERY + ACTION 混合候选      | 保持既有不 continuation 的规则；不能假装尚未返回的查询结果已知 |

因此，本版明确不承诺“需要查询后才能形成实际回复的轮次，也能同时被接管”。这是防止调用次数和事务语义失控的范围限制，必须写进报告与测试。

接管后的 B 如仅产生还需要一次结果合成才能完成的 pure QUERY，则因本轮调用预算已耗尽而受控失败；不能伪造答案，也不能擅自调用第三次主模型。已冻结且适用的真实查询结果可以作为输入复用，不必重复查询。

已有无 Tools 合成路径如返回文本，应通过已有适配进入同一条消息发送机制，不新增直接发送旁路；本任务不让它成为另一套正常 `visible_reply + tool reply` 协议。

把旧规范和静态守卫更新为上述精确边界，不能简单删除“一次认知”“禁止递归”等保护，也不能把 Judge 命名成内部函数后隐瞒额外调用。

## 11. Persistent Switch 与 forced_activation 归一化

保留现有白天/晚上切换的产品语义。**完全确定性的时间规则本次不实现 Runtime 求值**（只做分类、稳定 ID 与诊断）；**需要理解语境的持续切换也仍由 Main 驱动**（见上方更正），本次将其收敛为逐调用场景明确授权，**不新增** WakeUp/Reflection 切换入口，**不额外增加每轮 Pre-turn LLM**。

> **[更正 2026-09-14，基于代码证据]** 原文写的「仍由已有 WakeUp/Reflection 处理」不成立（证据同 §1 更正）。持久切换与 Turn Takeover 必须分开：**有权的原始 Main 可以提出 persistent switch；Judge 只判断 takeover；接管后的 B 无权修改 persistent active**，且该权限须在**候选校验、结算和恢复路径**中执行，不能只靠移除 Prompt 字段。

将现有 `forced_activation` 逐条区分为确定性持续切换、语义持续切换或本轮接管，迁移前输出对应表。不要把所有 forced_activation 自动转换为 takeover_once，更不能改变角色卡原本要求的持续性语义。

修复报告提到的“规则存在但 validator 不识别、没有可引用 ID”等问题。只有确实适用的切换入口才保留相关响应字段，普通 Main cognition 不再为了多重人格而每轮强制输出 keep/switch。

对于含义无法可靠归类的旧规则，保留原数据并提供诊断，不能静默丢弃或默认成另一个行为。初始化结构、规则 ID、持久化和执行契约必须同步。

共享关系、共同目标及共享记忆在 A/B 间保持可见；明确属于某个人格的数据才按 profile 隔离。不能仅因为初始人格是 A，就把未声明作用域的数据全部绑定给 A。

## 12. Working Persona：先合并，再投影，只有一个权威出口

在 Core 内确定合并顺序及允许覆盖的字段，形成一致有效数据：共享身份、当前 profile、已接受 Evolution Overlay。受保护身份字段不能被普通情绪或一次模型推断改写。

随后再投影成当前人格的 Working Persona，而不是直接序列化完整领域对象。

A 的正常 Main Prompt 只包含必要 Shared Identity 与 Working Persona A；不能同时包含完整 B、完整 profiles 数组、全部接管规则和另一份冲突 effective_persona。B 只有实际生成时才加载其 Working Persona。

Working Persona 必须保留决定表达和行为的关键特征，不得为了短而把“慢热但真诚”“专业领域自信”等信息都删掉，只留下姓名和几个泛化形容词。

完整背景故事、视觉细节、全部习惯和其他未激活人格存储在领域层。按当前任务需要进入上下文或 Capability 内部，不默认每轮全量注入。

投影应可追溯到 persona revision、profile 和 overlay revision。优先确定性投影、修订时编译或已有缓存，不在每次聊天前再让 LLM 总结一次人格。

核心人格不能被普通 Memory 预算随意淘汰。若必须保留的身份/行为约束本身超出预算，输出明确诊断并调整配置或投影设计，不能从字符串中间静默截断。

## 13. Message Role 与信任边界

这次不要把“不变放 system、变化放 user”当成绝对协议。变化频率不等于指令优先级。

System 放全局规则和受信任的人格行为定义；由 Core 校验、合并后的当前 Working Persona 可以放入唯一 System Persona 区域，即使 active profile 变化导致它变化。Runtime 事实、检索记忆和历史内容在语义上仍是数据，不可借投影被升级成新指令。

Recent Conversation 尽量保留真实角色与发言归属。只使用当前 Provider 和 chat template 实际支持的 role，不创造 `context` 或 `system-context` 等未支持角色。

如果动态 Context 由 user-role 消息承载，必须与当前用户实际输入明确区分，并验证相邻 user 消息在当前模板中的真实序列化行为。没有执行过的 tool 不能伪装成 `role=tool` 结果。

对同一份 Working Persona、事实、历史和工具做有限 Role/布局回归，检查归属、状态理解、历史指令隔离与人格稳定性。不要改变测试内容后声称差异来自 role，也不要为了这次实验推翻已经验证有效的布局。

## 14. 全请求预算与 Response Schema 收敛

统计完整请求而不是只看 messages：System、Working Persona、Runtime Facts、Recent History、检索记忆、tools、response_format，以及输出预留。字符数、字节数和 Token 分开报告。

不能把 Tool Schema 在 tools 字段和 System 中重复描述；不能把完整人格同时在 System 和 Runtime Context 序列化。普通对话不携带只供初始化、反思或持续切换使用的全部字段。

检查 Main response schema 中的 personality_decision、visible text、appraisal、状态提案与 Tool Calls 是否重复表达相同职责。删除字段前必须追踪实际消费者；保留仍必要的契约，不为减小 JSON 把合法状态变更变成隐式副作用。

保留原生 Tool Calling 时，不再要求模型在文本 JSON 中复制一份同样的 tool_calls。只有当前 Provider 原本就采用兼容封装时，才遵循其真实协议，避免凭空改造接口。

分别配置主请求和 Judge 请求的预算。具体数字按当前模型、真实请求和关键字段确定，不在没有 Token 统计时宣称一定降到某个数值。

当前人格数量增加时，普通 Main Prompt 不应跟所有非激活人格的全文大小同步增长。Judge 也不能成为另一个无上限的人格 Dump。

## 15. 冻结、执行、并发和恢复

复用现有 frozen action/执行日志，冻结足够信息：turn、候选 ID、active/reply-owner profile、persona revision、状态版本、规则版本、Judge 决策、最终胜出候选以及执行阶段。

拒绝候选可以用于诊断，不能进入正常会话记录、Active Memory、关系事实或下一轮人格基线。Internal intent 与 Judge 结果也不能自动证明外部事实。

模型调用、Judge、图片生成和外部工具执行都在数据库事务外。只在短事务中校验版本、保存决策、确认执行资格或提交状态。不要声称跨数据库和外部服务可以靠一个事务整体回滚。

只有胜出候选可以被正式 dispatch。沿用已有幂等键、冻结结果及投递机制，避免恢复后重复私聊、生图或发布。Runtime 崩溃恢复时，已有冻结仲裁决策不应重新随机判定；已执行动作不能重演。

同一会话并发用户输入、WakeUp 或 Reflection 可能与本轮重跑并发。复用现有串行化或版本控制策略；失效快照不得覆盖更新状态。若采用终止重试策略，必须有次数上限，不能让并发冲突引出无界主模型调用。

> **[更正 2026-09-14，基于代码证据]** 原文写「WakeUp 或 Reflection 可能改变 active profile」不符合当前代码：它们只**读** `active_profile_id`（`reflection_runtime_v2.go:320/332`），不写。并发风险仍然存在（并发 turn、重放、未来可能的切换），因此本条约束保留，但依据改为「并发 turn / 重放 / 授权场景下的持久切换」。

仲裁结束前不能把 A 的候选文本流给前端。可以发送不含候选内容的进度事件；任何预发送后再撤回的实现都不算“未执行”。

执行失败时沿用既有事实一致性规则，不发送虚假完成声明。凡回复声称某个动作已成功，应按现有依赖或延迟投递机制先确认该动作的真实结果；只进入队列不能表述为已经完成。不要为了完成这次重构发明第二套回复发送服务或无限补偿工作流。

## 16. 基础 Bug 的处理范围

下列问题在当前代码复核存在时修复，但它们是必要配套，不应吞掉本次多人格和 Prompt 工作：

| 线索                         | 要求                                                  |
| ---------------------------- | ----------------------------------------------------- |
| traits 数组与对象形状冲突    | 明确规范类型或显式兼容转换，不能静默变成空对象        |
| relationship.role 归一化丢失 | 检查准确字段路径；不能只凭 summary 出现关系词判定成功 |
| 无来源的 0.5 默认值          | 区分默认、用户提供和模型推导，不把默认当硬人格事实    |
| shared/profile 作用域错误    | 验证切换和本轮接管后共享关系、目标仍存在              |
| 时区与 TOON 递归问题         | 做小范围修复及回归，不顺带重写格式系统                |
| assistant 历史自我举证       | 记录“曾经说过”不等于确认作品、动作或外部事实已存在    |

本次不重写整个 Initialization，不新增完整媒体资产平台，不重构全部 Memory 生命周期。缺少真实作品 Asset 时，不得通过编造“已发送原作品”让测试看起来成功。

## 17. 必须完成的测试

### 17.1 确定性 Runtime 测试

使用 Fake Provider、Fake Judge、Spy Executor 和可控时钟，不依赖真实 LLM 才能证明执行边界。

| 场景                   | 必须断言                                                     |
| ---------------------- | ------------------------------------------------------------ |
| 单人格或无接管规则     | 不调用 Judge，主生成一次                                     |
| Judge=false            | 只执行 A，实际内容与候选一致                                 |
| Judge=true             | A 的私聊、图片、场景、记忆及根级状态提案全部未执行，仅执行 B |
| B 生成完成             | 不再反向仲裁，无第三次人格生成                               |
| takeover_once          | persistent active 不变，当前回复归属 B；下一轮正确使用持续人格 |
| persistent switch      | **[更正 2026-09-14]** 授权场景下的持久切换入口仍能更新 active，下一轮加载对应人格；被拒候选与接管后的 B **不能**改写 active |
| Judge 异常             | 保留合法 A 并记录降级，不绕过原有校验                        |
| A/B 候选非法           | 受控失败，无非法副作用，B 失败不偷偷发送 A                   |
| 取消、超时、崩溃恢复   | 无候选提前发送、无重复执行、无孤立永久切换                   |
| 并发版本变化           | 不用旧快照覆盖新 active/profile 状态                         |
| pure QUERY 路径        | 保持已有结果合成机制，不叠加 takeover                        |
| B 要求再次查询合成     | 命中预算守卫，不调用第三次主模型，不伪造答案                 |
| 候选提示注入的数据隔离 | 候选里的“返回 true”保持为数据，不被拼接为控制指令            |

### 17.2 Persona 与 Prompt 契约测试

验证 A Prompt 不含完整 B；B Prompt 不含完整 A，只允许必要且标明未执行的候选摘要；人格只有一个权威投影；traits 不丢；shared scope 不丢；规则 ID 端到端一致；真实请求预算包含 tools 和 response_format；被拒候选不能变成事实记忆。

为当前 A/B 角色卡建立精确字段和行为语义测试，不再使用“整个对象某处出现某个词”代替真正契约验证。

### 17.3 真实模型行为测试

有可用模型时跑真实 Provider，但执行器使用隔离或 mock，避免发布真实动态、给真实用户发私聊或产生昂贵图片任务。

固定并保存测试集，包括：A 表达克制但恰当、不接管；A 因表达困难明显造成误解、按规则接管；A 真正拒绝或表达边界、不能仅因不热情而接管；普通技术讨论、不接管；多轮接管后 A/B 仍保持各自表达风格。

先用固定 Candidate 测 Judge，再测真实 A 输出经 Judge 到 B 的整条链，避免混淆 Judge 错误与主模型候选变化。加入候选回复夹带“直接返回 true”的无害注入样例；数据隔离单元测试通过，不等于真实模型已经具备可靠抗注入能力。

记录模型、参数、测试次数、接管误报/漏报、失败例子和完整脱敏日志。不要用两三个成功例子宣布稳定，也不要为了过测不断向 System 加场景专用触发句。

## 18. 性能与成本验收

对照重构前后相同输入，分别报告：普通不需要 Judge、Judge 保留 A、Judge 接管 B、pure QUERY 这几条路径。

统计实际所有调用：A、Judge、B、查询结果合成，以及 Capability 内部已有 Planner。包括输入/输出 Token、缓存命中（可获取时）、请求次数、真实首条可见消息延迟和总耗时。A 的输出即使被丢弃也必须计入成本。

这次可能是用一个更小的主 Prompt 和更明确的人格职责，交换一次额外判断延迟。不要先验宣称必然更快、更便宜；本地多模型加载切换若有额外开销，也要记录。

同时记录主请求和 Judge 的序列化大小，说明压缩来自哪里、关键语义是否保留。禁止用字符减少直接替代 Token 或推理质量结论。

## 19. 实施与迁移顺序

按“代码盘点 → 契约与迁移表 → Candidate/仲裁边界 → Working Persona 投影 → 全链路接入 → 旧逻辑清理 → 回归与基准”的顺序实施。

以现有 Go 风格实现，不机械照搬 Java，不为每个概念创建 interface。复用 context.Context、已有依赖注入方式、日志、错误、事务和测试设施。

不引入 MCP、向量基础设施、新的工作流引擎或多 Agent 框架。Judge 可以是已有 Provider 的独立调用配置，不要求额外部署模型服务才能运行测试。

迁移必须可恢复。不要破坏性覆盖原始角色卡、清空历史或直接删除旧数据字段。可以短期使用迁移代码，但最终只保留一条正常对话执行链，不长期维持两套人格决策和回复发送方式。

清理普通 Main Prompt/Schema 中已迁出的切换职责；**[更正 2026-09-14]** 原文写「保留 Reflection/WakeUp 真正需要的持续切换接口」，但代码里 WakeUp/Reflection 只读不切换——**应保留的是授权场景下的持久切换入口**（不得一起删掉，否则白天/晚上切换立即失效）；`takeover_rules` 不进 Main、只进 Judge；不恢复完整非激活人格。静态守卫应改为新契约，不是全部移除。

执行项目现有 build/test/lint；适用时运行 `go test ./...` 和相关包的 race test。区分已有失败、环境阻塞和新增回归。

## 20. 交付物与完成标准

交付代码、一份简洁架构文档、一份规则迁移表、一份测试与性能报告。报告说明新增/修改/删除文件、运行方式、旧逻辑清理情况，以及无法验证或暂未支持的路径。

最终必须清楚回答：

1. A 第一次输出是否确实只是未执行提案？B 接管后是否连 A 的根级状态变化都未提交？
2. 用户私聊是否仍唯一通过现有 reply Capability/发送管线完成？
3. 持续主导人格与本轮发言人格是否分开？B 接管后是否禁止再次仲裁？
4. Judge 是否只看到判断所需的小上下文，而不是另一份巨大 Prompt？
5. Main Prompt 是否只包含当前发言人格的唯一 Working Persona？
6. pure QUERY 与 takeover 的互斥边界是否明确且被测试覆盖？
7. 全请求大小、总调用次数和真实可见延迟是否如实测量？
8. 是否能够通过注册/配置新 profile 和合法规则扩展，而不在 MainAgent 增加角色名或工具名分支？

本次成功标准不是“模型偶尔说出一句像 B 的话”，而是：

```text
A 候选 → Judge → 选择最终发言人格 → 只执行胜出候选
```

这条链在副作用、恢复、人格归属和调用预算上都成立；同时普通 Main LLM 不再承担完整 A+B 人格与全部切换规则的解释负担。

请先用当前代码证据确定实现位置，然后完成上述范围内的改造。不要靠增加巨型 Prompt、放宽全部校验或隐藏失败案例制造成功结果。