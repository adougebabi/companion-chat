# 实施进度 — 2026-10-27

分支：`codex/demo-workspace-replica-20261027`，基于 `codex/fluctlight-functional-recovery` 的 `b625e97e`。任务状态 `in_progress`；完成第一增量，整个逐页迁移尚未完成。

## 已实施：公共壳层 + 角色库

- Demo 米白/鼠尾草配色、218px 侧栏及大小屏断点、67px 折叠、700px 以下 Radix 导航抽屉、62px 顶栏、三组直接入口、侧栏角色选择与账户菜单。
- 快速前往支持页面/真实角色搜索、Cmd/Ctrl+K、Escape 和无匹配结果。
- 角色库引导首屏、继续相处、装饰封面卡片、搜索标签、网格/列表、真实空目录、继续创建和初始化恢复入口。
- 数据继续从 workspace 的正式目录/detail/configuration/recovery 读取；没有 Demo store、固定角色、模拟回复、假任务或新后端 API。
- 角色名字、头像、职业、简介、标签、运行状态来自真实投影。缺失世界名和最近聊天时间不伪造；WorldArt/StoryMark 仅装饰。
- Owner 范围的筛选与视图随编辑持久化到 sessionStorage；导航返回和刷新均恢复。

## 五态核对

| 状态 | 实际执行 | 证据 |
| --- | --- | --- |
| 正常 | 正式 dist 从隔离 API 读两位角色；资料/聊天入口、列表切换、长名字、缺头像 | production-desktop.png / list-desktop.png |
| 空白 | 真实空目录与搜索无匹配分别呈现；清除筛选恢复列表，空目录可创建 | empty-desktop.png / no-results-desktop.png |
| 编辑 | 本页无角色字段写表单；已核对搜索/标签/视图草稿、进入角色档案后返回、直接刷新恢复最新输入；角色编辑仍由已有档案控制器负责，档案页尚未复刻 | React tests + browser refresh readback |
| 加载 | 延迟 detail 时保留目录卡片，显示独立读取状态 | loading-desktop.png |
| 失败 | 首次 detail 503 保留卡片与失败原因；重新读取得到正式投影，无新增 mutation | failure-desktop.png / recovered-desktop.png |

## 视觉和交互证据

浏览器通过 CUA 对同一个活动页设置固定视口；先运行离线 Demo，再运行 production dist。非默认 viewport 已在收尾恢复。

| 1348×925 几何 | Demo | 正式实现 |
| --- | --- | --- |
| 首屏引导区 left / top | 252 / 94 | 252 / 94 |
| 首屏引导区 width / height | 1062 / 297.78125 | 1062 / 297.78125 |
| 网格卡片 width | 341.328125 | 341.328125 |
| 桌面侧栏 | 218 | 218 |

- 390×844：document.scrollWidth=390，无横向溢出；卡片宽356；抽屉打开/关闭。
- 701px：桌面侧栏185，无横向溢出；折叠宽67，所有直接导航仍有标签。
- QuickJump 搜索与 Escape 关闭，页面不发生跳转；控制台无异常。
- 邻接聊天与管理入口做壳层冒烟；桌面聊天输入区处于视口内，两页均无 document 横向溢出。它们尚未做 Demo 页面复刻验收。
- 原始 Demo 和正式截图保存在 `evidence/library/`；原始交付截图与说明在 `reference/`。

## 质量结果

- `pnpm --filter @fluctlight/web typecheck`：通过。
- `pnpm --filter @fluctlight/web test`：426/426 通过。
- `pnpm --filter @fluctlight/web build`：通过；生产 fixture 排除和 React-only import graph 检查通过。
- `git diff --check`：通过。
- 构建仍提示已有主 chunk >500kB；本轮没有为消除提示改构建策略。
- 独立只读核验指出 route query 滚动隔离和刷新前 UI 持久化两项 P2；主代理已修复并增加回归。子代理没有修改代码或运行最终验证。

## 验收运行方式

```sh
pnpm --filter @fluctlight/web build
FLUCTLIGHT_UI_FIXTURE_PORT=4190 node apps/web/test/fixtures/ui-quality-server.mjs
FLUCTLIGHT_UI_FIXTURE_PORT=4191 FLUCTLIGHT_UI_FIXTURE_LIBRARY=empty node apps/web/test/fixtures/ui-quality-server.mjs
FLUCTLIGHT_UI_FIXTURE_PORT=4192 FLUCTLIGHT_UI_FIXTURE_LIBRARY=slow node apps/web/test/fixtures/ui-quality-server.mjs
FLUCTLIGHT_UI_FIXTURE_PORT=4193 FLUCTLIGHT_UI_FIXTURE_LIBRARY=failure node apps/web/test/fixtures/ui-quality-server.mjs
```

这些场景仅在测试服务器生效，不进入生产 bundle。浏览器样本不构成 live PostgreSQL、Temporal、模型服务或 NAS 联调通过。

## 继续位置

下一页：聊天。亲读 Demo `src/pages/CompanionPages.tsx` 的 ChatPage 及 companion.css；对照正式 ChatPage.tsx，保留 conversation.ts 的草稿、流式、原 turn 重试与角色隔离。按 implement.md 后续队列逐页推进，不把第一增量当作全站完成。

本轮代码保留在工作区，未提交、未推送。原先 10-22 任务下未跟踪材料未修改。整个任务未归档。


## 增量 02 — 聊天（2026-10-27）

已复刻页头、755px 聊天主面板、285px 此刻侧栏、425px 消息视区、消息/时间/气泡/输入区和历史弹窗。正常/空白/编辑/初始加载/初始失败恢复/回应失败重试/长等待/历史失败恢复均有 `evidence/chat/` 截图；390px 无横向溢出，此刻按最新 Demo 移到下方。1100px 与1600px 折叠后 panel=layout 全宽。

历史使用正式 client.conversationDirectory / messages / searchConversationMessages / selectDirectConversation；阅读不切换，明确继续才切换，保留输入；分页失败重试原 query/cursor；初次目录期间禁止搜索；选择进行中禁止关闭，确认后仍按 Owner/角色协调当前会话。关闭回到原按钮。

修复原会话控制器首次读取失败后 loading 无法释放，增加真实 controller 失败→重读回归。正式 Chat 原先过滤 stream-* 导致增量不可见，现恢复真实流式消息；原 turn 重试、canCancel、原生附件与 IME 均保留。独立核验3项P2已修复。

质量：typecheck/build通过；全量433/433 tests；fixture/React生产guard通过。没有 live PostgreSQL/模型联调结论。下一页此刻。


## 增量 03 — 此刻（2026-10-27）
已按 Demo 复刻活动首屏、日程时间线、生活切片、下一项安排与日记入口；复用正式 LifePage controller 和同一穿着摘要。保留已有生活时间线的次要入口；无演示天气、温度或虚构完成状态。timeline只按action_status显示执行/完成。

证据在 evidence/now/：正常、空白、loading.life、日程503与重新读取恢复、390px；衣柜入口/返回验证。1348×925网格750px+290px，gap22；hero left252/top212.828125，内容真实条数/长短影响自然高度。390px单列356px，document宽390无溢出。此页没有字段编辑表单，核对了真实衣柜编辑入口；保存/换装在衣柜页继续验收。

独立只读核验发现重读loadLife可能抛出详情错误未接住，已修复并补渲染回归。typecheck/build/生产guards通过；435/435 tests；git diff --check通过。

继续位置：04 日程。Demo SchedulePage :3333-3832 已完整亲读；正式 LifePage :48-123 既有建议提交已读，待定位日期、建议目录和规划的正式接口及领域行为，再实施新视图。


### 当前继续状态
最新code gate：435/435 tests、typecheck、build及两项生产guard、diff --check均通过。此刻最终失败与恢复截图已按最新构建重录。浏览器viewport已恢复默认；任务/完整目标继续active/in_progress，未归档、未提交、未推送。

已核对并保存日程接口研究 `research/schedule-contracts.md`，包含建议CAS、原输入/幂等重放、日期生成独立通道、真实请求状态。日程UI尚未实施；后续所有未勾选页面和跨页深链接审计仍在原完整目标内。


## 增量04 — 日程（2026-10-27）
新增正式React SchedulePage，以Demo时间线/建议/生活倾向和详情/编辑/规划弹窗为基准，复用life provider；Core没有Owner直接采纳/拒绝建议API，移除演示模拟按钮，以实际请求结果呈现。保留原日程恢复、取消建议、约定/意向与时间线的来源入口。

- 日期范围、元数据、分页按Owner/fid/date epoch隔离；刷新失败保留同日期快照；首次未读数据不冒充0项/空白。
- 编辑冻结原日程/条目/生活版本，取消不保存；精确时间原样保留。JSON DTO在进入编辑前使用jsonValueSnapshot脱离嵌套Valtio代理，扩展字段/正文仍保留；真实浏览器暴露的DataCloneError已修复且新增代理消费回归。
- 条目与规划的未知回执恢复原body/key；缺失/跨角色回执在provider释放身份前拒绝。明确首次409才能人工review新基线；unknown后的冲突不能释放。规划改日期前重读目标日期并保护已有record，含items=[]。
- RequestStatus观察真实OwnerRequest/ScheduleRequest；deferred/awaiting_goal仍跟进，隐藏暂停、失败保留、达到读取次数后可手动继续；不把accepted意向当执行完成。
- 正常/缺失日期/编辑取消/加载/失败恢复/真实建议读回/真实规划读回/未知刷新恢复/规划CAS恢复及空条目record保护的截图在evidence/schedule。request-attempts.json记录未知两次尝试同body同key，明确CAS拒绝后date/reason相同、只更新已review版本与身份。
- 桌面header top94,height91.828125，toolbar top212.828125,height55，grid top287.828125，columns750+290 gap22与运行Demo一致；实际条目数量造成自然高度差。390px单列356，document390无溢出，日期按钮44×44。
- 独立核验3项P2（规划409恢复/切日期还原原输入/等待目标观察）均修复并增加回归。typecheck/build/生产guards/diff检查通过，最终449/449测试。所有浏览器数据是隔离夹具，未声明live DB/模型验收。

当前继续：05目标。Goal治理规范及Demo GoalsPage3906-4317已亲读；将核对正式Goal命令、证据进展和恢复模块后实施。整个任务与完整目标仍in_progress/active，未提交或推送。

## 增量05 — 目标（2026-10-27）
目标列表、四类筛选、搜索、详情、编辑和状态命令按Demo复刻，目录/证据/历史均从正式BrowserClient分页读取。保留独立目标规划与治理入口。目标重要度/紧迫度只读；没有演示优先级写入或直接完成操作。

- 正确拆分current/history目录；各自分页/失败重读。当前版本、目标层级、criterion_id共同决定条件判断；stage/commitment及过期版本不能算入，后端progress保持权威。无证据分页不降级已保存判断。
- 新建/编辑/暂停/继续/取消/放下/核对均使用原命令模型及真实回执。未知恢复保持body/key；已接受后失败只读恢复；提交和读回期间新输入保留。late create回执将新目标身份合入当前草稿，重开和刷新后可review并继续update，不重复create。
- 评估只观察真实evaluation_request.request_id，等待与终态分开，完成判定仍读领域记录。Owner/角色边界和旧响应保护保留；Owner切换清理目标恢复槽。
- 新建后详情会清理旧目标的证据/历史并重读新目标。独立只读核验两项P2均修复及回归。
- evidence/goals/含正常/空白/编辑取消与保存/加载/失败恢复、未知刷新恢复、1348×925和390px截图。request-attempts.json实录两次create尝试同body、一个key。
- 桌面toolbar top212.828,height55；卡片top287.828,width343.328，内容区1062。运行Demo对应top213.965/288.965,width343.328（源字号渲染约1px差异）；内容高度随真实文案自然变化。390px卡片356、document390；长角色名导致页头操作自然换行。编辑弹窗356×776.469，内部滚动，取消/关闭可返回。
- typecheck/build/生产guards/diff检查通过，全量459/459测试。样本是隔离API，未声明live DB/Temporal/模型验收。

最新进度：5/16组完成；当前继续06衣柜。其余11组及最终跨页/深链接审计仍在原完整目标内。任务in_progress，完整目标active，未提交或推送。

## 增量06 — 衣柜（2026-10-28）
沿用正式WardrobePage控制器、inventory provider和workspace.command，以Demo ResourcePages的三入口、穿着条、服装四列卡片、穿搭方案卡片、搭配工作台和资料弹窗替换旧视图。首入口改为全部衣服。服装图形移植为明确标注的类别示意，不作为实物照片或颜色事实。

- 当前穿着只消费正式worn_items，与聊天/此刻共用WearingSummary；列表存在不证明正在穿。初次未读显示未知，成功空数组显示未登记。
- 搜索/类型/季节/归档独立筛选；归档目录与修订历史使用正式分页，失败重试保留原cursor。归档项只读；Core没有恢复归档写接口，未添加模拟恢复按钮。拥有关系/可用状态使用正式可写字段。
- 保留服装登记、方案保存/改名/编辑、标签维护、换装请求及操作恢复。保存方案不改变穿着；换装受理、等待和实际终态分别显示，正式读回后才更新穿着。拍穿搭进入实际生成画面路由。
- 缺失或改变部位的选中衣物必须明确移除重选，不能被normalize静默丢弃。确认框打开后变只读仍阻止归档；归档失败留在确认框内。保存/读回期间的新输入保留；成功且输入未改变时进入方案库。
- workspace.command已接受的直接写入若读回失败，record.readbackPending阻止重复受理；恢复动作只重复onSaved读取，不再执行mutation。独立实际workspace回归证明1次写入、2次读回。此保护覆盖已挂载操作记录；没有声称直接写回执跨刷新持久化。
- evidence/wardrobe/保存正常/空白/编辑取消和保存/加载/失败重读、方案预览和真实换装等待→完成、归档读回、未知回执刷新恢复、1348×925及390px证据。未知两次PUT同body、一个key，见request-attempts.json。
- 桌面卡片252×299，与运行Demo相同；grid top471.172，Demo470.669，约0.5px差。390px两列171px、document390；长文案导致自然卡片增高。编辑弹窗356×776.469，内部滚动；关闭回到真实来源按钮，归档详情字段禁用且没有保存动作。
- 两个独立只读审查中的P2均修复并回归。全量466/466、typecheck/build/两项生产guard、diff检查通过。fixture证据不等同live PostgreSQL/Temporal/模型验收。

最新继续位置：07日记，再08记忆。已完成6/16组；剩余10组和最终跨页/深链接/字段一致性审计未完成。分支和任务继续保留，完整目标active，未提交/未推送/未归档。

## 增量07 — 日记（2026-10-28）
新增正式DiaryPage，按Demo左侧日期目录、右侧纸页、纸页下方批注和共同经历布局实施，使用现有BrowserClient。正文没有正式title/mood字段，标题显示真实日期，状态显示实际生成状态，不伪造日记标题、情绪或写于夜里的事实。

- 全部/收藏目录独立分页和重试，共同经历、留言记录使用各自真实目录。空白和未读取分开；搜索只筛选已经读到的日记，更早目录仍可加载。日记详情、列表、观察响应有身份/Owner/fid/epoch及版本读取次序保护。
- 批注在纸页下方编辑，独立annotation.revision CAS、5000 Unicode字符限制、保留原文/空格；取消丢弃未提交草稿。收藏变更同时刷新全部和收藏缓存；独立核验指出的缓存P2已修复并增加回归。
- WorkspaceCommandDialog复用原命令模型，新增表现层inline模式供批注使用。未知批注原body/key跨刷新恢复；明确批注CAS冲突需读最新版本并确认；已收到回执后只读恢复，读回期间新输入保留。
- 整理使用实际date/zone和pending/running/ready/no_content/failed/inactive。今天/未来不能生成；失败或未运行需要核对原日记版本；已有ready/no_content由Core保护。观察原diary_id和date结果，不制造正文、百分比或completed状态。生成身份指针供刷新后继续读取。
- 留言CAS使用日记revision，返回OwnerRequest；受理与会话回复分开。可读取历史、刷新记录和跟进原请求；共同经历明确为来源消息的远程交流，不推断同场或身体事实。
- evidence/diary/保存正常/空白/纸页编辑取消与保存/加载/失败重读、真实留言受理、整理实际正文读回、共同经历、批注未知刷新恢复和桌面/390px截图。request-attempts.json实录未知两次PUT同body，一个key。
- 桌面目录220px、间隔24px、纸页818px，与运行Demo一致；内容区1062，top287.828，Demo289.421（字体渲染约1.6px差）。390px纸页356、document390、目录横向滚动；长角色名和正式收藏筛选使工具栏自然换行。手机批注取消后回到来源按钮，无横向溢出，控制台无错误。
- 全量474/474测试，typecheck/build/两项生产guard、diff检查通过。浏览器数据均来自隔离fixture，没有live PostgreSQL/Temporal/模型验收结论。

最新进度：7/16组完成；下一组08记忆。剩余9组和最终跨页/深链接/字段一致性审计仍在完整目标内。任务in_progress、完整目标active，未提交、未推送、未归档。

统一隔离预览：4300端口，同时启用GOALS=normal、DIARY=normal、WARDROBE_WORLD=1和WARDROBE=normal的ui-quality-server。用于浏览当前已迁移页面；原4190实例保留。Demo仍使用4178/fluctlight-offline.html。

## 增量08 — 记忆与成长（2026-10-28）
新增MemoryPage，依Demo三入口、记忆卡片/小册、宽详情与内容/来源/修订标签，反思卡片/处置详情，自我认识卡片/依据/历史实施。读写继续使用正式BrowserClient，无本地业务快照、模拟采纳或虚构认识保存接口。

- Memory/Reflection/Self各自分页与失败重读；只读useWorkspaceRead按Owner/fid/查询/对象/版本隔离，保留失败原loader/cursor；详情接受已验证读回，后续独立刷新失败不覆盖新版本。路由query变化更新tab/筛选并关闭旧对象。
- 记忆API搜索正文，没有正式标签字段，不伪造标签；重要度按后端0–1展示。小册只统计当前已读记录，aboutOwner交给Core有效来源/actor_refs语义。无数据未读显示未知，不用0冒充。
- 修订/遗忘/补偿恢复没有可消费的客户端幂等key，使用CAS-only人工核对路径，body只含正式expectedRevision/content/evidenceRefs/targetRevision。未知结果刷新后保留草稿，核对实际当前版本；正文已相同则阻止再写。已收到回执后只读恢复；原来源按历史版本保留，当前来源明确pin revision。
- 记忆正文4096 UTF16、说明150字符和单引用256码点遵循实际接口；Owner说明作为支持引用，修正会替换当前支持集合，旧版仍可查。当前修订说明优先semantic_reason，不能把合并的旧Owner支持引用误当本版说明。
- 反思只展示实际proposal/dispositions/result，赞同/异议提交真实OwnerRequest。反馈受理不改写认识或直接采纳候选。自我认识没有直接改正文API，提供真实复核/遗忘/历史恢复；复核不提高revision或confidence，恢复检查完整已接受快照并由Core校验来源/期限/谱系。
- evidence/memory/保存正常/三个空白tab/编辑取消与保存/加载/失败恢复，原始来源/历史来源、反思反馈受理、认识复核及历史恢复、记忆补偿恢复与遗忘、未知刷新人工核对，以及桌面/390px截图。
- cas-review-audit.json实录未知回执只有1次PUT且不带客户端key，实际当前revision3。governance-readback.json证实记忆遗忘到revision5但保留原文和历史，认识恢复到revision3且confidence仍0.72、复核历史针对旧revision2。
- 桌面750+290、gap22；正式grid top351.828，Demo349.676（控件/字体约2.2px差），卡片宽750、高139.297，Demo140.094。短目录保持内容自然高度。390px卡片/认识356、document390；宽纠正弹窗356×683.938；取消/关闭回到来源卡片。
- 两个独立只读复核：读取无P1/P2；命令指出验证后详情二次读取失败可能仍显示旧版，已修复并回归。12项新增回归，全量486/486、typecheck/build/两项生产guard、diff检查通过。所有浏览器证据是隔离API，不构成live DB/模型验收。

最新进度：8/16组验收完成。下一组09角色档案；剩余8组与最终跨页/深链接/字段一致性审计继续保留。任务in_progress、完整目标active，未提交/未推送/未归档。

记忆统一预览为4306端口（GROWTH=normal，同时启用目标/日记/衣柜normal）；4301已有服务未触碰。4302空白、4303加载、4304首次读取失败、4305未知CAS用于隔离验收。

## 增量09 — 角色档案（2026-10-28）

按Demo复刻档案首屏、身份与背景/性格与人格/生活与此刻/修订与来源四主题、分组编辑、人格切换、身份提议与能力审核。继续使用现有Profile模型、BrowserClient和正式治理命令；未知字段保留在设定结构及完整来源中。初始外貌和实际穿着分别读取，性格维度显示服务返回的0–1数值，不伪造缺失设定。

- 编辑取消须确认丢弃且不写入。保存继续提议→接受两阶段；完整接受回执及最低身份版本决定成功，已接受但读回失败只恢复GET。原A未知→后续B编辑→重试A时，保存回调和操作记录使用实际执行的A，B仍保留为编辑草稿。共享workspace.command的回调新增可选第二参数，既有调用兼容。
- 资料未知结果跨刷新有明确原提交恢复入口，使用原body/key/提议/reason；浏览器request-attempts.json实录两次proposal相同body/key、两次accept同路径和body，身份版本只增一次。
- 人格切换保留完整实际OwnerRequest、双版本、目标/原因/确认和原提交恢复。pending显示旧当前人格；完成必须终态目标正确且detail版本不落后；rejected/failed即使当前碰巧是目标也不能写成功。终态失败保留目标/原因并清除确认。旧响应、不同请求/Owner/角色不能覆盖当前状态。
- 身份提议接受/拒绝/历史恢复使用正式CAS-only命令；能力申请使用自身revision、原idempotencyKey和合法状态迁移，交付必须匹配实际注册版本。自我认识保留已验收记忆页入口。独立核验的详情刷新P2已修复，刷新能力申请会同时更新当前详情和审核历史。
- evidence/profile/保存正常、空白、编辑取消/恢复、加载、首次失败及重读、未知保存跨刷新与原提交恢复、人格pending/实际完成、身份对照与接受、能力交付，以及桌面/390px证据。所有场景是隔离API样本，不能替代live Core/DB/模型联调。
- 1348×925正式hero x252/y235.828/width1062，Demo位置和宽度相同；正式grid y525.188，Demo524.156（hero自然高度差约1px）；双栏638.172+401.828，gap22，与Demo一致。390px页面宽390、卡片356，无横向溢出；手机编辑与治理弹窗均宽356、内部滚动，取消回到原编辑按钮；控制台无错误。临时viewport已恢复默认。
- 最终Web全量498/498测试、typecheck、build、两项生产guard及diff检查通过。未提交/未推送/未归档；原10-22任务未跟踪材料保留。

最新进度：9/16组验收完成。下一组10世界；剩余7组和最终跨页/深链接/字段一致性审计仍待完成。完整目标active，任务in_progress。

最新统一隔离预览：4316（PROFILE/GROWTH/GOALS/DIARY/WARDROBE normal，WARDROBE_WORLD=1），浏览器当前位于角色档案。4317空白、4318加载、4319首次失败、4320接受回执未知（已恢复）用于档案验收；4190与4306原服务保留。下一页继续复用现有服务，不因旧场景已消费而重复启动。

## 增量10 — 世界（2026-10-28）

完整亲读Demo ResourcePages.tsx的世界页/物品面板和resources.css。正式WorldPage现以同一工作区承载世界目录、场地分类、地点编辑、世界设定、物品目录、事件模板/执行与生活事件；真实字段和命令仍来自BrowserClient。原AI扩写的Studio底稿→任务→保存→发布→应用流程保持原阶段与输入，提取到world-expansion.mjs；完整独立工具入口继续可达。

- 世界目录按角色读取，快照按世界校验；浏览非运行世界不切换角色。物品、模板及生活事件目录仅能读当前绑定世界，非运行世界标签明确说明范围。世界绑定与世界配置分别CAS，选择世界不确认位置或活动。没有把世界kind/scene等Demo专用字段写入API；插画仅为Demo装饰。
- 创建/编辑/分类/地点/绑定/移除复用WorkspaceCommandDialog。取消不保存；新建不自动绑定。未知创建跨刷新显示原提交恢复入口，继续原body/key。迟到接受先将真实ID合入草稿，后续输入可基于真实对象继续修改，不能重复create。
- 地点名称与已有分类固定。未改动的嵌套陈设/连接/房内物品/世界物品池不进入写请求，避免改别名时重写未知扩展。明确采用新基线会同步未编辑字段，保留自己的真实修改；恢复原地点创建时把已接受名称/分类设为固定基线，同时显式保留未应用的后续名称/分类和可编辑字段。
- 地点历史/默认字段使用实际place/default双版本和已验证基准，支持查看来源、明确捕获和恢复。旧地点没有基准时不伪造初始值。从编辑进入历史会保留未保存草稿；浏览器实测恢复已保存字段后，重新编辑仍读到原未保存输入。字段历史不冒充完整空间引用历史。
- 物品配置使用目录物品自身revision与精确来源版本，不写库存；提出取得请求只追加到同Owner/角色的既有聊天草稿，须用户确认发送。事件启停使用模板版本；触发返回queued，实际applied/filtered/failed分别呈现；恢复原失败执行使用模板和原run双版本。原开关回执恢复以实际执行body的目标核对，不能取本次打开时的反向目标。等待执行只观察GET，隐藏/卸载会暂停或清理。
- 独立只读核验确认的三项P2已修复并回归：事件原目标读回、迟到地点创建的固定身份、冲突基线下的未编辑嵌套值。另有两条建议混淆了world与place契约，经world_runtime.go:299–347和world_validation.go:86–95点验排除：world description可省略，world status不是可写字段。
- evidence/world/保存正常/空目录/地点取消与保存/加载/首次失败重读、原创建跨刷新恢复、默认字段恢复、空物品/空事件、物品范围保存、事件queued→applied→实际生活记录，以及1348×925和390px截图。request-attempts.json验证两次创建同body/key；command-attempts.json验证物品CAS3且无库存写入、两次绑定CAS4/5；binding-readback.json最终绑定回原世界revision6；world-readback.json验证地点恢复到revision3、嵌套camelCase值仍原样、生活context_revision未变化。
- 桌面目录toolbar x252/y212.828/height39，卡片x252/y272.828/width340，与运行Demo一致。详情hero x252/y248.328/width1062一致；实际较短背景使hero高度252，Demo261.094，后续区块按同样间距自然上移9.094。分类栏195、gap22、地点卡片414，与Demo一致。390px目录/地点卡片356、document390；分类栏折为横向换行，编辑弹窗356×776.469且内部滚动；取消回到原按钮，控制台无错误，临时viewport已恢复默认。
- 最终全量513/513测试、typecheck、build、两项生产guard及diff检查通过。UI与API记录均来自隔离场景，不构成live Core/DB/worker/模型验收。分支未提交/未推送，任务未归档，原10-22材料保留。

最新进度：10/16组验收完成。下一组11人物关系与关于你；剩余6组和最终跨页/深链接/字段一致性审计仍待完成。完整目标active，任务in_progress。

最新统一隔离预览4321，同时启用WORLD/PROFILE/GROWTH/GOALS/DIARY/WARDROBE normal。4322空世界、4323首次加载、4324首次失败、4325未知创建（已恢复）为本组场景；既有4190/4306/4316服务保留。浏览器当前在4321世界目录。


## 增量11 — 人物关系与关于你（2026-10-28）

按 Demo CompanionPages 的人物卡、关于你双栏、关系指标及详情/编辑流程复刻。新增 PeoplePage，人物/Owner/其他 Human 数据继续通过 BrowserClient；人物目录与世界使用同一真实快照。目录、详情、编辑、新定义深链接均接到原生人物页，既有关系治理独立工具仍可达。

- 已结识只依据指向 Actor、属于当前人格或全局 null 的实际关系。目录成员、预测在场、拒绝接触不算相识；当前在场、规律预计、默认地点与历史确认位置分别呈现。
- 自填资料只提交修改字段，保留未动/未知/null/布尔值，年龄与性别保持自由字符串。Owner facts_gen 与其他 Human opaque context 双域 CAS 分开，读回不能借用较新的 context ID 证明旧 Owner 详情。自填 owner_defined 与交流事实、真实记忆分开；历史数量指已读字段记录。
- 运行人物编辑保留完整规律、出现、位置模式、归档状态。新人物走定义保存→发布明确版本→真实预览→明确应用，不直接创建运行人物。新建不呈现保留空值选择，local_overrides 不承诺强制覆盖。迟到回执合入真实身份、保留后续输入；后续版本已超越原应用时如实说明。
- 真实接触须明确确认、地点、过去且不超过24小时的时间、合法证据和 life-context revision；回执及读回核对实际事件、关系目标和记忆事件引用。机会重试只重新评估，不强制结识。引用阻止删除时保留原输入，归档使用正式更新。
- 独立只读核验的接触完整回执、后续资源版本误判均已修复并回归。共享 command 完成读回后清除已解决草稿，防止刷新残留恢复提示；内联编辑和重新挂载的人物详情按钮通过真实 ref/afterPaint 恢复焦点。
- evidence/people/含人物/关于你/关系正常与空白、编辑/取消/保存、加载、首次失败及恢复、定义保存/发布/预览/应用、实际接触与机会待评估、资料未知提交跨刷新恢复，以及1348×925/390px证据。request-attempts.json记录两次未知 PUT 的同 body、同 key、facts_gen_5，且只写 full_name。再次刷新不残留恢复提示。
- 桌面 tabs x252/y235.828/width1062、toolbar y313.828/height39、人物卡 y375.828/width340.656 与运行 Demo 一致；关于你双栏638.172+401.828、gap22。390px人物双列172、document390；详情/编辑弹窗356且内部滚动。关系72/64/78进度条实际呈现对应长度；取消关于你回到编辑按钮，取消人物编辑回到人物编辑按钮，关闭详情回到原卡片；控制台无错误，临时视口已恢复。
- 最终全量531/531、人物专项18/18、typecheck、build、两项生产guard及diff检查通过。浏览器和API数据均为隔离fixture，不构成live Core/DB/模型验收。任务仍in_progress，完整目标active；未提交/推送/归档，原10-22材料保留。

最新进度：11/16组完整验收。下一组12动态；其后13媒体、14创作中心、15初始化、16通知/管理/任务/数据/账户，另有最终跨页/深链接/数据一致性审计。不要按组数百分比推断剩余工时。

当前统一隔离预览4326（PEOPLE/WORLD/PROFILE/GROWTH/GOALS/DIARY/WARDROBE normal），浏览器在人物目录。4327空白、4328人物加载、4329人物首次失败、4330资料原提交未知、4331资料首次失败、4332资料加载已消费并核对；保留既有4190/4301/4306/4316服务。后续复用现有服务，不因已消费场景重复启动。


## 增量12 — 动态（2026-10-28）

亲读 Demo FeedPage、样式与正式 moments provider/Core 契约，新增原生 FeedPage 和同页宽详情，接回原 /feed/:postId 深链接。列表筛选、真实评论预览、发布/建议弹窗、评论和回应、隐藏/恢复、建议结果与人物互动保持正式 BrowserClient。现有 React 列表此前未接出的用户发布和分享建议入口已补齐。

- 动态列表、评论、建议和人物互动各自分页、失败重试与Owner/查询/对象/epoch隔离。角色筛选是动态所属范围，不假定作者；“我发布的”明确筛选已读记录且保留加载更早页。无角色也可读全局动态和发布自己的内容。
- 发布始终以Owner身份；发布范围 worldId 使用原接口的角色ID语义，不传运行世界ID。附件只选真实素材、保留来源及可读状态，不复制Demo画面或将图片当事件。历史异常正文使用已有阅读投影且保留原文。
- 评论按动态ID隔离草稿，回应/隐藏/恢复使用真实动态revision、原body/key。版本冲突须读新版本并确认；回执已接受后读取失败只恢复GET。评论回执是更新后的动态，读回核对实际评论作者/正文与revision/count，并沿后续分页定位原评论。详情刷新会把真实新记录同步列表，隐藏项不再残留于可见筛选。
- 分享建议保持OwnerRequest：受理、角色判断和实际发布分开。只有实际effects目标才出现“查看角色实际发布”；失败原文可带回单独草稿，经明确确认再提出新请求。可跟进另一个角色的原请求而不切换当前角色。人物互动重试保持原interaction/revision，不制造关系或同场事实。
- 原发布未知结果跨刷新显示“继续核对动态发布”，恢复同body/key，只有唯一动态。迟到成功保留后续新输入；Owner切换实际清除旧槽，迟到响应不能重新写入。只读审查确认的评论后续分页、详情刷新同步两项P2已修复并回归；另一项Owner恢复建议与既有登出清理边界冲突，实际Owner切换测试确认无旧草稿或回执重生，未修改共享身份清理合同。
- evidence/feed/保存正常/空白/编辑取消与发布、评论保存、隐藏/恢复、建议pending→实际发布、加载、首次失败重读、原发布未知刷新恢复、桌面/390px截图及请求审计。request-attempts.json证实原发布两次完全同body/key，正常用户发布1次、建议1次，未发送伪造作者字段。所有数据是隔离fixture，不能替代live Core/DB/worker/模型验收。
- 桌面toolbar x252/y235.828/height39，feed x252/y297.828/width1062，左757+右280、gap25与运行Demo完全相同。390px卡片356、document390；范围130、搜索217，顺序与Demo一致；窄屏隐藏右栏后，建议记录仍由独立折叠入口可达。手机详情356×776.469、编辑356×634.297，内部滚动；取消回到发布按钮，控制台无错误，视口已恢复。移动主操作保留既有44px触控尺寸，首卡y368.641，Demo约362，差异来自控件高度；真实正文/评论数量导致自然卡片高度差。
- 最终Web全量548/548、专项17/17、typecheck、build、两项生产guard及diff检查通过。任务in_progress、完整目标active，未提交/推送/归档；原10-22材料保留。

最新进度：12/16组完整验收。下一组13媒体，再14创作中心、15初始化、16通知/管理/任务/数据/账户，以及最终跨页/深链接/同字段一致性审计。当前统一隔离预览4333（含FEED normal与之前各组normal）；4334空白、4335首次失败、4336加载、4337发布未知已消费并核对。保留原4190/4301/4306/4316/4326服务，后续复用，不因观察超时或场景已消费重复启动。


## 增量13 — 媒体（2026-10-28）

亲读 Demo MediaPage、公共字段组件与 resources.css 以及正式媒体/视觉/头像契约。媒体库、视觉身份、生成画面、显示头像四页签现由 MediaWorkspace 原生 React 承载；原素材、视觉定义/身份、生图与头像独立路由接入同一界面，深链接关闭保留明确的 returnTo。正式上传组件及原文件指纹恢复控制器保留。

- 媒体库按Owner/明确角色范围、类型、来源、搜索和隐藏状态分页；失败保留原游标，不把未查询输入带入追加读取。素材详情保存名称/隐藏使用资产与元数据双CAS，来源/字节/引用保留，未动字段三方采用新基线；未知保存同body/key，已接受后只恢复GET。图片、视频、音频均尊重 readable，初次失败与空白明确分开。
- 视觉定义四字段与独立修订保存；保存不改变原候选/规范/身体/穿搭。生成同时冻结定义和视觉版本；实际 pending/queued 分开，不宣称 supersedeSessionId 结束旧会话，不合成任务ID或百分比。
- 人工评审提交原session/attempt/visual版本及0—1评分，实际观察/缺失分区/理由保留。接受规范与准备设定图分开；只有实际设定图资产才显示完成。旧评审独立恢复入口在候选已经决定、下一次尝试出现后仍可重开原上下文；原重新生成会话已推进时可沿真实timeline核对。较晚规范不会被冒充为原接受版本。
- 图片请求选择登记物品、地点和明确参考授权及各自版本；采用来源检查编辑指纹，授权变更须明确重选。受理、角色判断、生成任务和资产readable分别呈现，GET观察隐藏暂停，不增加假取消或计时成功。重置当前草稿不改变原冻结请求，未知原提交刷新可按同body/key恢复。
- 显示头像独立CAS与资产版本保存，不更改规范或授权。保存态 avatar=null 时遵守Core回退，不能按旧ID展示另一版本；新明确选择才读当前图片，并核对选择的version/SHA/revision。保存后实际刷新共享资料，浏览器侧栏出现已选择头像。
- 参考授权限定真实目标/用途/确认，可撤销并查历史/使用；已撤销行仍能恢复原撤销。删除查询完整reference_count，受理后资产不可读，后台状态独立跟进；失败只重试原删除，没有恢复已删字节功能。初始引用不存在/未知不等于0。上传继续使用既有64MiB与同文件指纹恢复，没有复制Demo本地备份/图片保存。
- 三个独立只读核验发现的P2已全部修复并回归：已决候选原评审入口、已撤销授权原请求入口、失效头像来源不能被按ID绕过。浏览器另外发现并修复媒体弹窗未继承作用域、组合表单被共享两栏限制成半宽、可读取标签与未知/空白混淆。共享命令仅新增显示className和安全当前草稿重置；禁用来源按钮关闭后焦点退回页头，头像重挂载后回到保存按钮。
- evidence/media/含四页签正常/空白/加载/首次失败和恢复、素材编辑取消与双版本保存、定义保存/排队/人工接受/实际设定图、图片pending→实际资产、独立头像及侧栏读回、原评审未知刷新恢复、上传入口和桌面/390px截图。failure-identity-before-fix.png只记录发现的问题；更新后的failure-identity-desktop.png已确认显示待读取而不是初始空白。
- request-attempts.json实录原评审两次完全相同的body/key/session/attempt，expectedVisualRevision1、attempt2；正常图片请求1次、头像选择1次。runtime-readback.json确认规范revision2、canonical为实际候选、设定图已读回，而显示头像独立指向replica-unused的资产revision3；素材资产revision仍3，库信息独立增到2。
- 桌面媒体卡340、图215、工具栏39，与Demo一致；视觉双栏519+519、gap24、图345，头像310+25+其余宽。源数据长名称、完整时间和正式管理控件造成自然高度差。390px媒体双列172、视觉/生成/头像356，document390；详情和定义弹窗356×776.469、内部滚动，头像115。取消返回来源，控制台无错误，临时视口已恢复。
- Demo独有喜欢保存与名字头像模式没有正式写接口，未接虚构收藏或initial写入；名字仅用于实际头像不可用的回退。公开图片装饰只用于空白视觉，不冒充实际候选或生成结果。所有浏览器/API证据为隔离样本；上传入口与既有控制器已核对，未声明live Core/DB/存储/模型联调完成。
- 最终Web全量575/575、媒体专项27/27、typecheck、build、两项生产guard及diff检查通过。仍未提交/推送/归档，任务in_progress、完整目标active，原10-22材料保留。

最新进度：13/16组验收完成。下一组14创作中心及编辑/发布/应用；之后15初始化、16通知/管理/任务/数据/账户，再做全站深链接/返回/Owner角色隔离/同字段一致性审计。完整目标未缩小。

当前统一隔离预览4338（MEDIA正常，包含此前FEED/PEOPLE/WORLD/PROFILE/GROWTH/GOALS/DIARY/WARDROBE正常）；4339四页签空白、4340加载、4341首次失败、4342未知原评审已消费并核对，4343专用于未知/空白显示修复后的视觉失败复验。旧服务和原证据保留。浏览器在4338媒体库，视口恢复默认。

## 增量14 — 创作中心（2026-10-28）

以 Demo ResourcePages/资源样式为依据完成目录、九类编辑、保存/发布、版本历史、独立复制、差异应用和沙盒双栏。继续使用 studio provider、BrowserClient 与 createStudioEditor，不接入 Demo store 或新后端接口。正式目录/编辑/NPC/预览/素材深链接均保留。

- 草稿opaque版本与资源数字revision分开；发布精确保存版，预览并行读取不可变publication与实际目标差异。世界/地点/NPC/物品/搭配/事件/人格/视觉/模板均保留正式字段及完整JSON。世界结构异常保留原值；事件范围变化清理不再合法的place_id。
- 编辑保存、任务、复制/归档均在pending释放前持久化真实接受回执；普通卸载仍可恢复，Owner清理后迟到响应不重建槽。创建先记录实际ID再导航；等待期间后续输入保持。原保存已受理后只恢复GET，未知受理继续原body/key。旧localStorage目录命令使用原DTO、原key、原存储恢复，未核对时不发新归档/恢复。
- 应用使用精确发布kind；运行地点名、NPC局部覆盖保持锁定。NPC提交前重读世界/绑定，明确不声称原子CAS。Foundation提议与激活分开，文本解析沿真实job进入沙盒。物品登记、搭配保存、视觉参考分别说明实际结果；其他真实库存ID输入保持可达。历史副本独立未发布。
- 沙盒冻结当前草稿，不自动采用建议；完整JSON待核对时阻止采用，确认期间新增JSON同样保留。任务状态、文本/实际图片与素材分别呈现；素材取消保留只解除关联，不删除原任务与字节。目录Owner筛选偏好保留。弹窗显式返回来源按钮。
- 独立只读核验发现的两项P2（未核对JSON被采用覆盖、事件范围切换留下非法地点引用）已修复并回归；另补了迟到回执、原Owner清理、历史复制、素材保留回归。
- 浏览器 evidence/studio 含目录正常/空白/加载/首次失败重读、编辑/发布取消和确认、应用差异/回执、沙盒初始与建议、八类其他编辑表单、新创建未知刷新恢复及桌面/390px截图。request-attempts.json验证两次POST完全同body/key，只创建studio-created-1；服务器保留原名称，页面保留等待期间后续新名称。
- 1348×925引导x252/y212.828/h101.297、toolbar y336.125/h39、首卡y396.125，与Demo一致。目录三列gap20；编辑1fr+285/gap24、沙盒0.8fr+1.2fr/gap24。系统出现15px滚动条时实际内容宽1047、首卡335.656，否则1062/340.656；没有将系统滚动条差异伪称完全相同。390px目录与单栏内容356，document390；应用弹窗356且长内容内部滚动。实际取消焦点回到应用按钮。
- 全量Web591/591、Studio/控件专项27/27、typecheck、build、两项生产guard、git diff --check通过；生产图372模块。所有结果为隔离fixture，不构成live Core/DB/worker/模型/存储验收。未提交/推送/归档，原10-22未跟踪材料保留。

最新进度：14/16组验收完成。下一组15初始化四步/四来源/七阶段恢复；之后16通知、管理、任务、数据、账户，再做全站深链接/返回/Owner角色隔离/同字段一致性总验收。完整目标active，任务in_progress。

最新统一隔离预览4344（STUDIO+此前MEDIA/FEED/PEOPLE/WORLD/PROFILE/GROWTH/GOALS/DIARY/WARDROBE正常），session23679。4345空白session41489、4346加载session82779、4347首次失败session91285、4348未知创建session31471已核对，原服务保留。Demo4178因连接拒绝恢复服务session27304。浏览器参考标签已关闭；生产标签2继续保留，使用前核实视口/页面。

## 第15组实施中 — 初始化（2026-10-28，尚未验收完成）

本轮是实质进展：已亲读 Demo CreationPage.tsx 的布局/各来源/完整分组预览/七项准备/弹窗及 creation.css 全部样式；复用现有 createCreationController、creation-world/reuse-world 和正式 BrowserClient，新增原生 views/CharacterWizard.jsx、replica-creation.css，CharacterWizardPage 保留原长生命周期控制器。implement.md 的第15组仍未勾选。

已实施：
- 四步图标导航、三类世界来源、四种人物来源、右侧摘要、分组资料预览、完整JSON显式应用、创建后只读原请求快照、正式七阶段与恢复。新 WorkspaceCharacterSketch 仅为 Demo 原SVG装饰，明确标记非实际形象，不进入头像/候选或业务数据。
- 现有世界不再排斥明确选择已发布底稿/日常世界；取消自动选第一个已有世界，要求显式选择。已发布卡片按精确 immutable publication 读取正文，读失败保留已确认来源，分页重读保留原cursor且不重复追加卡片。直接打开初始化也独立读取实际分组，未读/失败不伪装无分组。
- c.previewDefinition/updatePreviewDefinition 合并未保存的分字段编辑并保留完整嵌套结构与扩展，分析ID/correlation/schema来源不能被替换。未应用JSON持久保留，阻止激活和其他结构覆盖。删除/清空/非字符串名字或生活时区均不会退回旧输入；显式缺失在刷新时不被默认时区填回。
- checkpointCreation在原激活pending释放前保存activationSnapshot。激活未知结果立即进入准备页，独立说明原提交未确认，不误称分析失败。recoveryPending保持已推进为ready后的原阶段恢复入口；已知首次CAS拒绝必须读实际版本、明确解除旧记录后再决定新恢复，不自动重发。newDraft阻止busy期间重置并清理旧错误状态。
- 独立只读核验的两项P2已修复：完整JSON删名字仍用旧名字、显式null被无编辑转换为空列表。对应回归通过。

当前验证：最新全量607/607、typecheck、生产build与两项guard通过，生产图371模块；git diff --check通过。日志 /private/tmp/demo-replica-creation-{tests,typecheck,build}.log。专项 demo-replica-creation.test.mjs（13项）和 demo-replica-creation-ui.test.mjs（3项）；原R11/R16世界选择断言已按Demo显式来源卡更新。fixture是 apps/web/test/fixtures/creation-fixture.mjs，由 ui-quality-server 环境变量CREATION显式开启，未进入产品包。

浏览器已有证据（evidence/creation，多数带-wip，尚不能作为本组最终验收）：
- 4350成功：已发布世界published:world:1、实际分组group-fixture、描述→预览→1次激活→7/7真实DTO读回；创建后步骤锁定、只读原请求。
- 4351失败恢复：空白创建+已有世界，衣柜失败时2/7、后续blocked，恢复原角色衣柜后7/7，只有1次激活。
- 4352未知：空白路径0分析，两次激活body完全相同（requestId亦同），原角色恢复后7/7，不带analysisId/corePersona。三个 *-request-audit.json 已落盘。
- 4353空世界/空模板截图已保存；4354分析延迟、4355分析首次失败尚未消费。
- 旧4349只有6/7是夹具误用detail路由（原写成空tail，实际/detail），非生产逻辑；夹具已修正，4350之后成功为7/7。保留4349与旧证据，不以此冒充完成。

继续工作：
1. 完成4354实际加载/停止等待/迟到结果、4355分析失败重读与输入保留；补世界首次读取失败/恢复（如有必要）。
2. 用最新dist对照运行Demo的1348×925截图和390px。已修旧character-workflow限宽、完成步骤删除线、step滚动顶距、准备说明字号；这些修正后需最终截图。核对四来源切换草稿、JSON/模板实际选择与payload、预览五主题/取消/完整JSON、弹窗焦点和无横向溢出。
3. 视需要补recovery-unknown、world-conflict夹具的浏览器路径（现有Core控制器测试已覆盖）；不要丢失原恢复入口。检查Core明确400/422拒绝时，新完整JSON编辑是否仍有可修复/导出入口，避免仅保留原描述而失去预览编辑。
4. 更新本组契约/证据，五态与窄屏完成后才勾选15；之后16和最终全站深链接/Owner/角色/数据一致性审计仍完整保留。

进程：4349 normal旧夹具session82142；4350 normal修正session50630；4351 wardrobe-failure已恢复session77908；4352 activation-unknown已恢复session72001；4353 empty session95204；4354 analysis-loading session77615；4355 analysis-failure session66296。原4344/4338等服务均保留，不能因观察超时重启。CUA browser2/prod tab2，当前4353空模板页，临时viewport已reset并markHandoff。Demo4178 session27304；浏览器旧构建一次动态chunk错误已用reload解决，不是当前页面新增错误。切换新dist后需reload同源旧页面，勿重启服务解决缓存。

整体仍14/16组正式验收，完整目标active、任务in_progress；没有提交/推送/归档，原10-22材料保持。

## 增量15 — 初始化验收完成（2026-10-28）

实施代理和独立检查分别完成第15组收口，详见 research/creation-implementation.md 与 research/creation-check.md。主线程已复核新增代码、测试日志、最终几何和截图；本节取代前述初始化WIP。第15组现已勾选。

- 四步/三种世界来源/四种人物来源/五主题完整预览、结构编辑取消、完整JSON、独立分组目录及创建后只读原请求均以Demo布局和正式控制器连接。明确拒绝后保留可编辑、可导出、刷新可恢复的无旧分析授权副本，经新分析才能再次创建。
- 世界进度与恢复使用非空实际ID匹配，修复双方缺失ID假7/7；旧本地标志缺失时，真实ready绑定同时决定完成及聊天入口，避免7/7却无法进入聊天。独立检查新增组件回归并自修后重新验证全量。
- analysis503→原输入保留重试、15秒等待→停止观察→迟到结果不推进、四来源切换草稿与精确模板、422副本刷新/导出/新分析、原阶段未知503后ready仍可恢复、世界revision3冲突→明确核对revision4→独立复制/绑定，均已在浏览器验证。未知恢复body/key相同；世界冲突与阶段恢复各只激活一次。
- 正式与Demo在1348×925的步骤条x252/y212.828/1047×76.5、内容x252/y319.328/774+25+248、操作栏x252/y830/1047×80一致。390px主栏341、来源网格高236、弹窗356，document390；正式44px触控产生8px内容偏移与2px操作栏高度差，底边同为836。来源/主体内容实际长度自然不同；插画明确为装饰示意。
- 最终全量610/610、lint/typecheck、生产build、两项生产guard、git diff --check通过，371 local modules。日志 /private/tmp/demo-replica-creation-check-tests.log、demo-replica-creation-check-build.log。evidence/creation/最终命名图片及请求审计齐备；旧-wip不是最终验收图。
- 仍是隔离API样本，不声明live Core/数据库/模型/worker/存储联调完成。没有后端API/prompt/token策略修改，没有提交/推送/归档，原10-22材料保留。

当前进度：15/16组页面验收完成。第16组通知、管理、任务、数据、账户，以及最后全站深链接/返回/Owner角色隔离/同字段一致性审计仍未完成。用户已再次授权继续；Trellis任务in_progress，完整范围不变。

第16组只读定位已写 research/management-scope-map.md（非实施报告）；账户Demo源码尚需定位。新增初始化样本4356取消、4357拒绝、4359阶段未知、4360修正后世界冲突均保留，具体session见实施报告。检查代理只改变旧草稿聊天入口条件，未动样本进程；临时视口由实施代理reset。主线程原标签与子代理标签相互独立，使用前重新确认。

## 增量16与全范围验收收口（2026-10-28）

第16组通知/管理配置/诊断任务/数据/账户均完成，独立检查622/622，详见research/management-implementation.md、management-check.md。保留实际API、配置ack/CAS/secret、workflow allowed_actions和原请求；数据为既有有界领域端口，不宣称全原生；认证按正式setup/login/logout，未认证不读私有目录。独立检查修复durable回执GET失败后重发、Owner清理遗漏、Sheet多字段Tab焦点，以及chat/history读者不一致。补正确1348x925与390x844的第16组check-demo图片。

最终主线程完成9类保留对象抽验和统一矩阵：research/final-object-matrix.md。发现并修复两项真实接线遗漏：衣物详情返回q未消费；共同经历的chat?conversation/message没有打开原交流。现在复用同一原生历史读者，在当前角色目录确认后读精确消息与sequence+1窗口；无自动resume，Escape回原来源；衣柜恢复搜索与稳定tab/state。新增3项行为回归，最终独立只读检查无阻断项。

正面跨页链：4375保存replica-unused独立头像，侧栏/聊天/档案实际图片加载；方案plan-1仅clothing-0/1，原OwnerRequest pending时保留旧六件，completed并GET后衣柜/聊天/此刻/档案均两件。切B为profile-shirt且无A头像选择，切回A保留。4376补原交流精确来源、关闭返回及衣柜筛选复验；4374诊断record真实GET失败→重读恢复。证据和实际readback JSON见evidence/final/，来源消息390px弹窗356/document390。Owner切换与同Owner重入的迟到回复防护由协议回归验证；不冒称两个真实Owner数据库验收。

最终完整Web625/625，0失败/跳过，targeted23/23；lint、typecheck、生产build与fixture exclusion/React-only两项guard、git diff --check通过，372 local modules。日志已复制evidence/final/quality。只修改测试夹具dispatcher以让可变库存优先于静态profile fallback，未将夹具接入生产。规格已补七节合同：第16组、持久回执/Owner/focusout、独立history/原消息/衣柜返回。

实施与验收完成，implement.md16组及三项最终清单已勾选，正式完成记录research/final-acceptance.md。按继续至完成授权进行本任务本地提交和Trellis收尾；没有push/deploy，也无backend API、prompt或token策略修改。10-22未跟踪材料保持原样，不进入本任务提交。live Core/PostgreSQL/Temporal/模型/NAS/备份operator不由fixture验收替代。

源码/测试/规格本地提交：b8991b1c（feat(web): replicate demo workspace with formal domain workflows）。暂存全范围检查另清理一处WorkspaceMomentArt文件末尾空行，无行为变化；验收证据随后单独提交，再执行当前任务归档与日志。
