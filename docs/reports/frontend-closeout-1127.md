# 正式前端实现收口（2026-11-27）

起点：codex/fluctlight-meromero-plugin-humanization，bcc4074b，工作区干净。
AEL：frontend-closeout-1127。只记录实际观察；静态核验不等于浏览器验收。源码简写中 pages/shared/views 指 apps/web/src/react-app 下对应目录，v3-domain/proto 指 apps/web/src/v3-domain 和 apps/web/src/v3/proto。
截图放仓库外 /private/tmp/frontend-closeout-shots，不提交图片。

| 入口与范围 | 实现位置 | 问题 / 依据 | 复用与处理 | 状态 / 证据 |
| --- | --- | --- | --- | --- |
| 管理中心模型调用、聊天本轮调用详情 | shared/ModelCallDetails.tsx、TurnTimeline.tsx、views/ManagementPage.jsx | 属性平铺、顶部技术按钮显隐发生在正文末尾 | ResponsiveTabsList、DetailFields、Feedback、RawContent，概览/错误优先，技术按钮直接切换调用信息 | 已完成首批7cadbadd；Loop补充及最终验证见下文 |
| /diag/events 及其他分区、单条记录 | v3-domain/diagnostics.js、proto/components/diagnostic-card.js | 主要查询与高级/维护混排；reason_code 无条件失败 fallback | 常用查询直接展示，高级按需展开（手机共享 Sheet），显示已应用条件；状态范围驱动解释 | 已完成首批7cadbadd；Loop补充及最终验证见下文 |
| 调用运行回放 | shared/DiagnosticReplayPanel.tsx、scenarios/install.ts | 独立 pre；mock 缺九来源与关联范围 | RawContent、DetailFields；补来源分页与 correlation fixture | 已完成首批7cadbadd；Loop补充及最终验证见下文 |
| /diag/task/:id、任务抽屉、业务恢复 | pages/TaskDetailPage.tsx、shared/TaskExecutionResults.tsx | 空调用推断停止阶段、阶段原因不完整及残留分隔符 | 沿用 AsyncTaskStatus、WorkflowRecovery、MediaTaskRecovery；移除无依据推断与死分支 | 已完成首批7cadbadd；Loop补充及最终验证见下文 |
| 日程 /life/schedule → /planning?section=promises | pages/SchedulePage.jsx、v3-domain/life-schedule.js | 可见约定入口进入另一完整日程工作区；旧页日期操作误跳普通日程路由，section未被主页面消费 | 收敛入口与专注分区，保留命令/快照 | 待当前批次完成 |
| 目标主列表详情/编辑、/life/goal/:id | pages/GoalsPage.jsx、v3-domain/goal-editor.js | 同字段双实现；旧对象页与主页面在终态动作、只读约束、范围编辑和回执恢复上不等价，不能直接删入口 | 进一步比对字段与入口，局部共享 | 已完成：双入口复用，旧实现与样式已清理；正常门禁14通过/1配置跳过 |
| 日程/目标/记忆/日记/衣柜/人物/世界 Tabs | pages 与 views 同名文件；replica-life/profile/world.css | 重复滚动与触发器几何；缺共享激活项自动入视野 | ResponsiveTabsList，保留业务状态值 | 待当前批次完成 |
| /growth/memory/:id、反思对象页 | v3-domain/memory.js、reflection.js | 已确认路由，字段重复未确认 | 保留兼容，继续核实具体入口 | 不删除 |
| /life/diary/:id、/life/shared，收藏/注记/回顾依据 | v3-domain/diary.js | 正文、来源、尝试已分层，独立命令仍必要 | 现有共享桥接与渐进披露 | 静态无需调整，未声称浏览器通过 |
| 衣柜主列表/衣物对象详情 | views/WardrobePage.jsx、v3-domain/life-wardrobe.js | 对象详情包含治理历史，不等于旧列表 | 保留对象能力 | 除 Tabs 外静态无需调整 |
| 人物目录/详情/新建/编辑/关于你 | pages/PeoplePage.jsx、views/PeoplePage.jsx | 各入口已经使用同一个 React 页面 | 保留 | 除 Tabs 外静态无需调整 |
| 世界、地点、事件与物品对象页 | pages/WorldPage.jsx、v3-domain/world-*.js | 共享字段与命令弹层已有；不同对象各有业务职责 | 保留对象入口，复核组合 | 待实际场景核验 |
| /media、媒体对象、视觉身份与生成 | pages/MediaPage.tsx、views/MediaWorkspace.jsx | 已统一 React 页面及共享筛选抽屉 | 保留 | 静态无需调整 |
| /studio、/worlds、创作编辑/预览/素材 | DomainPage.tsx、feature-routes.mjs | 明确 native React 所有者 | 保留 | 静态无需调整 |
| /feed、动态对象、筛选与发布 | views/FeedPage.jsx | DetailSheet、WorkspaceCommandDialog 已复用 | 保留 | 静态无需调整 |
| /inbox | pages/InboxPage.jsx | 两项 Tabs 无确定布局问题 | 保留 | 静态无需调整 |
| /library、创建、资料查看 | App.tsx、views/CharacterWizard.jsx | 创建与资料抽屉已共享 | 保留 | 静态无需调整 |
| 旧命令/抽屉桥接 | shared/DomainPrimitives.tsx、OverlayHost.tsx | 当前业务事件/FormData 的兼容路径，非死代码 | 保留；不另造 renderer | 已核实用途 |

## 验证记录

- 修改前实际调用详情和事件列表：手机 390×844、桌面 1440×900 截图已保存在上述目录。
- 类型检查、单元回归、真实路由浏览器交互、布局、视觉与后端联调分别记录，未运行的不计通过。
- 后端真实联调仍归既有 ui-1125-task-diag-real-backend 环境等待项，本轮 fixture 不能证明线上数据行为。

## 第一批实际交付

- 调用详情按对象状态、简短原因、请求与返回、完整调用信息组织；顶部技术入口直接切换到可见的信息区。
- 嵌在聊天时间线中的调用详情不再套 Tabs，也不重复显示同一模型概览和失败说明。长错误保留可展开和复制的完整值，错误代码仍可直接查看。
- 运行回放先给按时间排列的来源/状态总览，再定位单条证据。继续保留九来源各自的快照、游标、重试与只读约束。
- 事件列表手机首屏保留搜索与查询，高级条件用共享底部 Sheet；已应用条件与编辑草稿分开，导出和清理移到结果后的维护区。
- 修复原生 Sheet 关闭或从未打开时字段脱离 FormData 的真实缺陷；只调整桥接节点归属，没有修改领域请求。
- 普通 reason_code 不再推断失败；未知原因保持未知，事件明确标为发生时快照；非失败当前状态不套用残留 last_error。
- 移除任务空调用记录推断停止阶段、业务阶段的未用分支与空分隔符。清理调用详情的旧属性列表样式、重复弹层尺寸与嵌套 pre 滚动。
- 完整来源使用现有原始记录下载组件；直接跳转后可返回原排查来源。保留正常复制、精确 null/空串/false/0、媒体扩展和业务恢复回执。

## 本批验证与限制

| 检查 | 实际结果 |
| --- | --- |
| Web typecheck | pass |
| 正式 build、fixture 排除、React 依赖图 | pass，435 个本地模块；未混入 fixture |
| 相关内容语义/任务/回放单测 | 60 项通过 |
| 原生领域控件与命令桥接补充单测 | 30 项通过 |
| 完整 UI gate | pass：343 通过，0 失败，27 配置跳过 |
| 最终来源下载与返回路径局部门禁 | pass：40 通过，0 失败，1 配置跳过 |
| 新业务三视口与本轮详情五状态 | 24 项通过；实际截图复核后通过既有命令审核基准 |
| 全量单测探索 | 初轮 1406 项中 1402 通过、4 失败；两项旧语义断言随本批修正后通过。inbox-toolbar-cascade、r24-settings-panels 两项在未改动业务上独立复验仍失败，待后续入口批次核实；不称全量单测通过 |
| 后端真实联调 | 未验证，沿既有环境等待任务，不以 fixture 代替 |
| 真机键盘/安全区 | 未验证，浏览器模拟不能替代真机 |

代表性截图：phone/desktop-calls-before.png 与 phone/desktop-calls-after.png；五状态本轮详情为 phone/tablet/desktop-turn-*.png，均在仓库外截图目录。事件列表修改前 fixture 未支持 events，旧截图仅作查询布局参考，不用来声称前后业务数据一致。

后续尚未完成的批次、兼容路径与静态无需调整项沿上表继续；本批通过不等于整个收口任务完成。

## 诊断补充：多轮 Loop 集中展示

用户补充要求：诊断必须便于明确排查，且同一次 Loop 的多轮调用应一起展示。

- 管理中心与任务详情复用同一 modelCallGroups 投影，按已有 run identity、主体、角色/绑定、Agent/用途和来源边界整理；不按时间相近、模型名称或 parent_run_id 硬合并。不同模型可属于同组，Writer 子运行和 Kev 保持各自记录。
- 每组平铺实际调用，显示已读取数量、失败与进行中数量，保留真实序号/阶段及各条错误摘要。统计只针对当前筛选与已读记录，不把最后一次模型成功当作任务完成。
- 单条已有关联时也保留稳定组容器。跨页补齐不替换原入口；详情打开期间追加数据后仍保持当前调用，关闭返回原入口。
- 同一弹层底部固定上一条/下一条与当前条目，阅读长内容时不必关闭再打开；每条请求/返回/错误/原始关联与恢复入口保持独立。
- 合并诊断页重复标题，移除组外多套的一层卡片；诊断分区复用 ResponsiveTabsList，清理紧凑态36px的业务覆盖，由共享模式管理标签尺寸。
- 技术动作使用 WorkspaceDialog 的共享 headerAction 槽；实际量测原动作与关闭触控区重叠10px，现为两个独立44px区域。几何只写入 overlay.css，已移除业务页脚的绝对定位。
- 技术按钮定位到当前信息区；来源入口按实际去向命名为“查看调用来源与关联记录”。原始来源统计数与已读组数量分别说明，不伪造最终 Loop 轮数。

验证：冻结版完整 UI gate **pass：376通过、0失败、29配置跳过**；build、fixture排除、React依赖图与typecheck通过。相关单测26项、后续旧断言对齐回归已通过；真实分页3视口通过，360/430手机紧凑态、固定导航、字段进入视口和防重叠检查通过；基准审核55通过、2配置跳过。全量1409单测的旧断言失败中，调用错误邻接与36px假设已修正并局部复验；收件箱CSS计算器、设置插件开关两项既有失败仍待对应批次，不称全量通过。

限制：现有 run_id 有时会复用业务 correlation，不能证明严格的单次执行边界。前端按既有运行关联整理，重复序号明确提示并保留各次时间、状态与来源；未改后端接口或自行创造执行标识。媒体调用未套用文本Loop归组语义。

截图仍在 /private/tmp/frontend-closeout-shots：phone/tablet/desktop-loop-groups.png、*-loop-call-detail.png；均来自正式组件与隔离fixture，不是线上后端联调证据。

## 剩余管理入口补扫结论

| 入口 | 实现位置（apps/web/src 下） | 结论 / 后续处理 |
| --- | --- | --- |
| 设置各分类、旧设置别名、移动分类目录 | react-app/pages/ManagementPage.jsx、react-app/views/ManagementPage.jsx、react-app/shared/SettingsSections.jsx | 已复用共享字段、目录、版本与幂等保存；不整体重写。插件开关旧单测断言待核实 |
| 认证与初始化 /auth | react-app/pages/AuthPage.tsx、react-app/App.tsx | React字段错误聚焦、忙态与会话处理完整；静态无需调整，既有浏览器布局不等于真实登录联调 |
| 桌面账户菜单 / 手机更多退出 | react-app/App.tsx | 确认重复的退出处理，可合并；保持后端确认成功后才清会话 |
| 数据管理 /data、导入/导出 | v3-domain/data-tools.js、react-app/pages/DomainPage.tsx | 可见控件已桥接共享React；存在重复管理页头，单作品导出需手输ID，待后续收紧；导入校验与未知回执约束保留 |
| 导入草稿 /data/pack/:id | v3-domain/data-tools.js | 首屏包ID/digest和资源ID占主位，待改为业务名称/类型与明确草稿入口，完整标识保留原始层 |
| 修订查询 /data/revisions | v3-domain/data-tools.js | 已有中文摘要与原始下载；精确对象ID可归入高级条件，保留查询语义 |
| 本地备份、回档说明、维护备份详情 | react-app/shared/LocalBackupsPanel.tsx、v3-domain/backups.js、v3-domain/data-tools.js | 明确备份边界、Owner隔离及隔离恢复，技术维护字段有实际消费方，静态无需整体调整 |
| 身份修订与能力治理 | react-app/pages/GovernancePage.tsx | 状态约束与来源摘要已共享；身份提议目前只填JSON，需核实能否复用既有身份字段编辑器；能力交付版本是必要信息，保留 |

## 目标批次：对象入口复用

- 目标对象URL继续保留，改为打开正式GoalsPage的同一详情、编辑与命令；对象不在首批列表时仍按目标ID独立读取。
- 未读、读取失败和终态不提供编辑；原始读取错误可下载，重读后旧错误清除。目标状态显示在页眉，成功空证据/修订直接说明，有内容、加载、失败与分页继续保留。
- 核实后端goal_owner.go明确拒绝已存在目标变更scope，复用主页面的范围只读符合实际契约；版本核对、幂等身份、原提交GET恢复、评估跟进与全部原始记录保留。
- 删除无生产引用的v3-domain/goal-editor.js及其旧样式。现有goal-editor命令身份/存储键保留，避免破坏原草稿和回执。
- 正式路由三视口验证直达/列表/取消/返回，包含360手机与503重读后的长内容。初次共享范围检查381通过、1项resize同步失败；改为严格等待实际740px高度后，最终正常适用门禁pass（14通过、0失败、1配置跳过）。相关33项单测通过，最终错误原文与记录补充19项通过（有交集）；typecheck和build通过，首次业务基准三视口已审核。
- 目标之后仍需继续日程、记忆/反思对象复用及管理入口，不把本批通过称作全站收口完成。

## 深层对象补核

| 入口 | 结论与处理边界 |
| --- | --- |
| 世界/地点完整编辑 | 与主页面快速编辑部分字段重叠，但完整工具另有默认字段恢复、完整世界框架与草稿；保留真实兼容能力，不整页删除 |
| 地点详情、世界事件 | 有人物/日程/实际影响等独有阅读内容，已渐进披露和原始下载；静态无需重做 |
| 世界物品 | 主页面与独立详情均有取得范围，独立页另有来源、衣柜与聊天取得请求；两者已有版本/快照保护，保留语义边界 |
| 人物关系修订 | 已有CAS与原始下载；确认保存/恢复await后缺少存活检查与忙锁，需修正旧请求离页后导航/刷新的问题 |
| 记忆对象详情 | 当前React详情已覆盖正文、按修订来源、补偿恢复、纠错/遗忘和回执核对；可承接对象URL，须保留独立读取与来源返回 |
| 反思对象详情 | React详情已覆盖来源、提议/实际处置、反馈原请求跟进；可承接refId，不能把认同改成直接采纳 |
| 反思时间线 | 仍独有latest_task/window进度观察、隐藏页暂停与输入窗口诊断；未承接前保留该兼容入口 |

## 日程规划批次：直接工作面与重复清理

- `/life/schedule/planning` 直接展示约定、意向、未来七天；移除整个外层设置 Sheet，以及“日程设置→补全”“调整建议→填写”两层中间菜单。新增建议直接打开原表单，约定的提出/改期/撤销/证据/请求回执仍用原控制器与契约。
- 删除该入口重复的当天时间线、生活倾向与调整/取消实现；主 SchedulePage 保留这些能力、补全规划、初始日程保护与原请求恢复。旧 today/habits 深链接转到主实现并保留日期。
- 修复日期查询错误跳回主日程并丢失规划分区；UI Lab 也使用真实 `/planning` 路径。未来安排保留原始记录、地点/约定来源、展开状态及实际时间标签更新；建议记录保留独立读取、失败重试与游标。
- 清理 DomainPrimitives 中不再使用的日程 Sheet 专用分支及对应 CSS；时间线摘要在共享原组件中明确设置文字列与换行，三个视口可读。检查器继续注入真实长日程的旧固定高度来证明能捕获溢出。
- 验证：相关22项单测通过，typecheck通过；三视口交互18项通过，已审核页面/长文本/空态的本地基准。完整共享范围 UI gate **pass：382通过、0失败、29配置跳过**；补充正式主日程→规划入口→返回导航后，正常适用门禁 **pass：41通过、0失败、1配置跳过**。完整单测1407项中1405通过、2项既有失败（收件箱旧CSS层级假设、插件旧switch角色），继续在对应批次核实，不称全量通过。
- 截图在 `/private/tmp/frontend-closeout-shots/*-planning-*.png`，不提交图片。线上Core联调及真机行为仍未验证。

## 记忆与反思批次：对象详情复用

- memory/:memId 与 reflection/:refId 使用同一 MemoryPage 详情、命令和回执，按ID独立读取，不依赖首批列表命中；未读/读取失败不展示纠错、遗忘或反馈操作，角色范围检查保持。
- 来源返回参数仍只接受内部路由；对象关闭返回原来源或对应记忆库分区。反思保留独立时间线及处理进度入口，认同仍提交反馈请求，不直接应用候选。
- 当前记忆读取后采用真实修订号加载来源；历史版本可切换对应来源并保留独立游标、原消息链接、原记录下载和补偿恢复。移除重复重要度和重复返回动作，按钮按实际来源命名。
- 删除无生产引用的 memory.js 旧列表/详情/编辑，以及 reflection.js 旧对象详情/反馈实现；保留时间线的 latest_task/window 观察、隐藏页暂停、任务身份与失败状态。原回归迁移到正式React组件，完整记录含null/false/0及未知字段仍逐对象核对；退役旧日历记忆测试由当前真实client的aboutOwner=false回归承接。
- 记忆主分区与详情使用共享 ResponsiveTabsList，移除详情专用标签几何覆盖；不扩大旧DOM桥接。
- 验证：相关53项单测、typecheck通过；正式对象三视口覆盖独立读取、原来源返回、修订来源切换、反思反馈表单取消、失败重读与长内容。六张本地业务基准已审核，完整正常 UI gate **pass：394通过、0失败、29配置跳过**。原始详情的旧基准缺失和有意标签视觉差异在审核前如实失败，未放宽阈值。
- 截图只在本地基准与仓库外目录；后端真实联调、真机输入仍未验证。其余管理入口和关系修订继续处理，不将本批称为全站完成。

## 关系修订与回归维护批次

- 人物关系修订沿用原CAS、证据和恢复契约；补上读取/保存/恢复的页面存活与Owner检查，提交前即加忙锁。离页、Owner切换或晚到失败不再跳转、刷新或改写旧页面；失败后原字段保持。
- 真实关系对象三视口验证等待提交时控件锁定，提交版本/证据正确，409拒绝后保留草稿并允许继续核对。独立单测覆盖读取晚到、保存离页、保存Owner切换、恢复离页及防重复提交。
- 收件箱真实浏览器量测发现：手机刷新控件44px，但旧网格末列固定36px，超出容器8px。列宽改为按共享控件自动分配，未覆盖按钮尺寸；390/360/430手机和桌面回归覆盖边界、触控、中心对齐、筛选与重读。
- 删除自行模拟CSS级联且写死36px/旧层级的测试，替换为上述正式Chromium回归。插件设置旧switch断言改为真实共享checkbox的标签、初值、勾选草稿、保存按钮启用且不自动写入。按钮扫描继续逐条检查variant/size，改用实际领域控制器和共享工厂证明扫描覆盖，移除“至少600个旧按钮”数量门槛。
- 验证：最终完整正常 UI gate **pass：399通过、0失败、29配置跳过**；typecheck通过，关系/插件/布局相关14项及补充扫描回归15项通过（有交集）。全量1412项中1411通过，唯一失败为退役后按钮数量门槛；该断言修正后局部通过，最终全量在余项完成后再跑，不将局部通过冒充全量结果。

## 身份提议批次：共享资料字段与命令浮层

- 身份提议复用现有 ProfileForm/ProfileField/profileFields/profileChanges，提供常用资料、表达与生活字段，仍保留高级JSON全量变化通道及旧JSON草稿。字段/JSON切换保留表单外变更、未知字段、null/false/0；核对新基线保留原提议与理由，并采用新版本。
- 保存仍只调用propose，接收后是待审核提议，不使用会自动接受的profileSaveCommand。后台允许的personality/provenance等高级输入仍可提交，运行人格切换与自我认识的后端约束未变。
- ProfileField增加可选受控值回调，既有变更方式保留；ProfileForm使用共享ResponsiveTabsList。对象补丁保留字面键而不将其变成JavaScript原型，原记录数据不改写。
- 通用CommandDialog改为复用WorkspaceDialog：手机完整工作面、标题/提交区固定、正文唯一滚动，忙态不可误关闭；错误滚动到可见位置并聚焦。原提交重试、状态查询、只读读回、同步最新基线和完整诊断下载保持各自语义。
- 正式三视口9项浏览器用例覆盖正常提议、JSON错误保留与重读、字段切换、实际请求体/提交身份、只提议不应用、忙锁、原提交未知后的原样重试及等待期间新草稿保留。三张本地基准经实际截图审核，未放宽断言。
- 验证：相关35项单测通过；**完整单测1416通过、0失败**；typecheck通过；完整正常UI gate **pass：408通过、0失败、29配置跳过**。真实Core联调与真机输入仍未验证。
