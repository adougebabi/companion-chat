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
| 目标主列表详情/编辑、/life/goal/:id | pages/GoalsPage.jsx、v3-domain/goal-editor.js | 同字段双实现；旧对象页与主页面在终态动作、只读约束、范围编辑和回执恢复上不等价，不能直接删入口 | 进一步比对字段与入口，局部共享 | 待当前批次完成 |
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
