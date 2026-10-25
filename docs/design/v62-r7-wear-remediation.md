# v6.2-r7 换装：真实本地模型回归

基线 `ce0bdc1f`。本轮处理已拥有衣物的聊天换装、Owner 套装请求及页面结果恢复。
用户明确授权调用本机模型；继续直接修复，不创建 Trellis 任务。

## 证据与根因

r7 已证明 r6 的 Owner 工具发现修复有效：方案库能真正执行 wardrobe.wear，
此刻页也读到更新。普通聊天仍只选择 conversation.reply，真实原生初始工具为
reply / Moment / catalog。目录入口存在，但精简认知协议没有明确表达阶段与
行动执行阶段的职责，工具名字未知时也可能直接猜 names。

上轮 scripted Provider 按预定顺序返回 catalog/inspect/wear，只证明协议链可执行，
不能证明实际模型会自主选择正确动作。本轮使用本机 MLX 已有模型
`huihui-ai-Huihui-Qwen3.8-27B-abliterated-MTPLX`，公开 loopback API 为
`http://127.0.0.1:11234/v1`。全部模型调用串行，使用专用一次性 pgvector PostgreSQL；
角色、衣柜、聊天均是合成验收数据，不调用用户的云 Provider 或业务数据库。

基线实际复现：模型回复“好了，换好了”，但 SQL 检查没有穿上短靴，断言失败。
另一轮还出现猜测 wardrobe.list/wardrobe.unequip、以错误物品 ID 重复调用 wear。
初版证据导出脚本的列名/SQL fixture 90 秒上下文也分别报错；它们已修正，不把
导出失败或超时样本当成完整正向证据。后续每次物理请求独立保存完整本机 wire，
最终 SQL 检查另用有界审计上下文，避免长推理丢失验证证据。

仅补认知职责后，普通换装、动态复合与流式已实际成功，但不可用衣物场景
暴露更深的数据传递缺口：

- 父 Agent 已收到 `wardrobe.wear` 的 handled/awaiting_state、
  `warning_code=wardrobe_item_unavailable`、`effect_applied=false`。
- 父 Agent 调用 reply 时没有选填 evidence_refs。Writer 的物理请求因此没有
  tool_facts，也没有本轮失败。Writer 的 recent_outcomes 原本不自动暴露；
  当前动作也尚未进入对话最终结算的 ActionOutcome 数据库。
- 当前穿着被正确标成部分视图，故“未列出短靴”不能独立解释失败。Writer 又说
  “靴子换上了”。该轮 SQL 无变更虽然正确，语义验收明确失败。

这类问题属于跨阶段事实丢失与测试覆盖缺口，不能靠放宽写入校验、强制角色服从、
解析回复文字改变穿着，或仅要求模型“不要幻觉”来处理。

## 修复边界

1. 明确认知负责自主决定及执行本人生活动作；reply 只负责表达。接受且可执行的
   动作先查目录、查询真实状态并执行；拒绝和延后仍保留。未知工具名先列目录，
   不猜 names，不把查询或加载当成执行。
2. Writer 当前事实视图注明表达阶段与完整角色能力不同，文字聊天不禁用本人
   生活能力，正文和括号叙述都不能改变状态。保留旧 Writer protocol/hash 和人格原文。
3. `currentActionFacts` 从服务端当前 ADK trace 读取真实 invocation/result，
   按授权 Actor、角色、会话、source fact 配对，补入 mandatory current_action_result。
   不依赖模型 refs，也不等待 DB 最终结算；保留失败状态、warning/disposition 和
   false 变更标记。排除 query、reply 自身、模型参数、父推理和原始错误详情。
   实测表明仅补回执仍不足以约束此本地模型，因此增加只针对未生效换装的
   `reply_action_check`：用认知模型检查未发送正文，不加载人格、不提供工具。
   有冲突时原 Writer 只纠正一次，再次不符或核验出错即失败，不发布正文，
   父循环在下一模型调用前立即停止。普通成功、no_change、纯讨论不增加检查。
   纠正继承实际 fallback 绑定，保留文体候选链、原修复次数、媒体与旧合同。
4. Owner 选择接受后必须消费真实 wear 回执；目标已穿由 Tool 返回 no_change。
   accepted 文本仍不能伪装成功，权限、版本、可用性和目标精确匹配保持。
5. 页面每个 owner/角色/方案保留最新原请求身份，刷新后重新 GET 恢复结果；
   sessionStorage 不存业务结果。旧 pointer 按提交顺序收敛，迟到响应不改新旧顺序。
   账号切回重新挂回观察记录，同一时刻重复点击在 202 返回前也复用原提交。
6. no_change 显示“已经穿着这套搭配，本次无需换装”；accepted 不再被
   mutation_committed=false 误写为目标已满足。普通界面保留中文反馈，原始诊断不改写。
   方案按钮无障碍名称带角色和方案名，衣柜入口使用角色名/中性文案。

## 验证方法

`r7_live_wear_test.go` 直接转发真实 HTTP，不预设 ToolCall、回复或业务回执。
仅在显式 FLUCTLIGHT_LIVE_PROVIDER_TEST=1 和隔离 PostgreSQL 配置下运行。
正向检查真实穿着集合等于原集合加目标鞋，真实 wear receipt 一次，重建 App
后再次询问；副作用必须由模型的生产 handleTurn 链产生。

负向检查 SQL 不变，并审阅实际正文。回复只包含目标衣物名称、测试进程退出 0，
都不能证明语义正确；不可用场景的中间轮正是靠正文审阅发现的第二个缺陷。
动态检查仅一条 publication；流式使用真实 SSE 原生工具协议；Owner no_change
检查无新增穿着版本且重放不再次调用模型。

本机证据保存在 `/private/tmp/r7-live-*`；仓库仅收录合成样本的简明结果，
不包含用户 r7 诊断原文、凭据或模型隐藏推理。

前端回归执行真实 workspace 与生产 WardrobePage，覆盖终态 reload、no_change、
accepted、失败、202 前重复提交、旧 pointer 乱序、账号和角色往返、未决查询失败、
读回错误独立显示。生产资源通过本机受控 API 做浏览器复核；这是页面证据，
独立于真实模型和真实数据库的业务执行证据。

## 最终结果

合成样本简明结果见 [local-model-results.json](evidence/v62-r7/local-model-results.json)。
不同验收结果明确区分：

| 真实本地模型场景 | 实际结果 |
| --- | --- |
| 普通聊天换已有短靴 | 模型自主目录→查询→wear，SQL 穿着增加短靴，保留其他衣物；重载询问一致 |
| 先发动态再换装 | 一条动态，随后真实穿靴，原衣物保留，重载一致 |
| 只讨论搭配、不换装 | 穿着不变，无换装效果 |
| 流式普通聊天 | 真实 SSE 原生 ToolCall，实际穿靴，重载一致 |
| Owner 已穿同一套 | completed/no_change、mutation=false，版本与穿着不变；重放不再调用模型 |
| 不可用短靴，最终发布保护 | 本地模型仍生成不实草稿，两次核验拒绝；0 条助手消息、穿着不变，立即终止本轮 |

最后一项是 `blocked_inconsistent_draft`，不是“模型正确说出了失败原因”。
前面的数据库负向检查曾退出 0，但人工读正文发现了虚假完成；这一中间样本
明确不计语义通过。最终阻断场景在包含核验和立即终止策略的代码上运行。
正向场景此前已实际运行，其成功路径不触发后加的失败核验，随后由完整受控
数据库/race 回归核验整合。没有把不同版本样本混称为全部在最终提交上实测。

本地模型多次不遵守失败表达约束，事实核验可以识别这些已观察到的错误。
失败换装最多增加两次核验和一次纠正；这里优先保证不发布不实完成，没有
声称解决 r7 的聊天耗时。核验本身仍是模型判断，不保证任何模型、任意输出。
另一个 LM Studio 模型的交叉探测在本机后端 HTTP 400 失败，未计入验收。

质量检查：工作区 444 项（Web 401 + clients 43）通过；类型检查、客户端生成
无漂移、生产构建及 fixture 排除、独立 mock 构建通过；Go vet/build 与相关
真实 PostgreSQL/race 回归通过。新增正式任务清单和无工具边界已同步。
Go 全量仍单列六项既有基线失败：

- TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground
- TestInitializationOwnerBackgroundKeepsStrictScalarTypes
- TestNoArgCatalogSchemaValidationAndCatalogParity
- TestPhase8CIUsesTheRealGateAndVectorEnabledDatabase
- TestBrowserRouteMatrixMatchesOpenAPIArtifact
- TestSurfaceRoutesMatchCoreAndMatrix

最终浏览器使用生产资源验证失败结果刷新后仍可见、已穿同一套显示明确 no_change、
对应方案按钮可区分，以及刷新后实际短靴读回；最终控制台 warning/error 为空。
截图：[失败结果保留](evidence/v62-r7/v62-r7-failed-after-refresh.png)、
[已穿提示刷新后保留](evidence/v62-r7/v62-r7-no-change-after-refresh.png)。

## 本轮未关闭项

购物入库依赖世界场地、物品目录及活动链；本轮不将已有衣柜换装通过等同于
“买衣→入库→换装”通过。r7 的世界场地、记忆、初始日程依赖、约定、日记等
其余问题不在本轮换装提交中标记关闭。当前实测模型也不能替代用户 Haiku 的
真实环境验收；应对部署本轮构建后的普通聊天再复测。
