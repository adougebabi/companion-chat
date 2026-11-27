# 正式前端实现收口（2026-11-27）

起点：codex/fluctlight-meromero-plugin-humanization，bcc4074b，工作区干净。
AEL：frontend-closeout-1127。只记录实际观察；静态核验不等于浏览器验收。源码简写中 pages/shared/views 指 apps/web/src/react-app 下对应目录，v3-domain/proto 指 apps/web/src/v3-domain 和 apps/web/src/v3/proto。
截图放仓库外 /private/tmp/frontend-closeout-shots，不提交图片。

| 入口与范围 | 实现位置 | 问题 / 依据 | 复用与处理 | 状态 / 证据 |
| --- | --- | --- | --- | --- |
| 管理中心模型调用、聊天本轮调用详情 | shared/ModelCallDetails.tsx、TurnTimeline.tsx、views/ManagementPage.jsx | 属性平铺、顶部技术按钮显隐发生在正文末尾 | ResponsiveTabsList、DetailFields、Feedback、RawContent，概览/错误优先，技术按钮直接切换调用信息 | 已完成本批：完整 UI gate pass（343/0/27），最终来源补充 affected gate pass（40/0/1）；相关 60 项单测通过 |
| /diag/events 及其他分区、单条记录 | v3-domain/diagnostics.js、proto/components/diagnostic-card.js | 主要查询与高级/维护混排；reason_code 无条件失败 fallback | 常用查询直接展示，高级按需展开（手机共享 Sheet），显示已应用条件；状态范围驱动解释 | 已完成本批：完整 UI gate pass（343/0/27），最终来源补充 affected gate pass（40/0/1）；相关 60 项单测通过 |
| 调用运行回放 | shared/DiagnosticReplayPanel.tsx、scenarios/install.ts | 独立 pre；mock 缺九来源与关联范围 | RawContent、DetailFields；补来源分页与 correlation fixture | 已完成本批：完整 UI gate pass（343/0/27），最终来源补充 affected gate pass（40/0/1）；相关 60 项单测通过 |
| /diag/task/:id、任务抽屉、业务恢复 | pages/TaskDetailPage.tsx、shared/TaskExecutionResults.tsx | 空调用推断停止阶段、阶段原因不完整及残留分隔符 | 沿用 AsyncTaskStatus、WorkflowRecovery、MediaTaskRecovery；移除无依据推断与死分支 | 已完成本批：完整 UI gate pass（343/0/27），最终来源补充 affected gate pass（40/0/1）；相关 60 项单测通过 |
| 日程 /life/schedule → /planning?section=promises | pages/SchedulePage.jsx、v3-domain/life-schedule.js | 可见约定入口进入另一完整日程工作区 | 收敛入口与专注分区，保留命令/快照 | 待当前批次完成 |
| 目标主列表详情/编辑、/life/goal/:id | pages/GoalsPage.jsx、v3-domain/goal-editor.js | 同字段双实现；对象入口仍注册，不能直接删 | 进一步比对字段与入口，局部共享 | 待当前批次完成 |
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
