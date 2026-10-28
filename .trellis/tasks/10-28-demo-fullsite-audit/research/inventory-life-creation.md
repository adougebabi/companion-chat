# Life / Creation source inventory

共 80 行；Demo 来源固定为 `/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/`。仅源码检索，没有浏览器、截图、真实交互或持久化验证。所有视觉、交互、业务状态均为待检查。

## 覆盖

- 此刻；日程三标签、详情调整、规划；目标列表、表单、证据、状态；日记阅读、批注、留言、生成、共同经历；记忆正文/来源/历史、纠正/遗忘/恢复；反思反馈；自我认知治理。
- 管理中心八分类、模型服务/用途、图像与视觉、自主性/安静时段、后台通知、诊断任务/模型调用/健康、数据导出导入备份恢复。
- 初始化世界三来源、四种输入、分析、五个预览标签、复杂数据编辑、激活/原请求恢复、七阶段、世界变更/衣柜/视觉恢复、完成/起点/新草稿。
- 正式专属能力以 Demo不适用登记，视觉状态仍待检查；未删除任何入口。

## 有出处的差异与核查优先项

- **life-now-current** — Demo LifePages.tsx:3204 有固定小雨/18°C；正式 NowPage.jsx:12-16 仅真实日期，无天气组件，需登记内容条件差异，不能注入假天气。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:3179`; `apps/web/src/react-app/views/NowPage.jsx:10; apps/web/src/react-app/pages/LifePage.jsx:17`。
- **life-now-schedule** — Demo :3234 使用当天 schedule.slice(-4)，正式 NowPage.jsx:18 使用 upcoming.slice(0,4)，LifePage.jsx:40-46 排除已结束条目；同场景应分别核查当前/过去/未来条目的可见集合。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:3223`; `apps/web/src/react-app/views/NowPage.jsx:10; apps/web/src/react-app/pages/LifePage.jsx:17`。
- **life-schedule-detail** — Demo :3386 accept 直接前端采纳建议并改 runtime；正式 :375 runItem 提交 owner request，不能照搬本地接受即成功。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:3673`; `apps/web/src/react-app/pages/SchedulePage.jsx:350`。
- **life-schedule-suggestions** — Demo 的独立 accept/reject 本地状态与正式 owner request 审核机制不同；请求状态不能宣称实际日程已采用，需检查 readCalendar 回读。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:3550`; `apps/web/src/react-app/pages/SchedulePage.jsx:265`。
- **life-goals-edit** — 字段差异明确：Demo :4254-4268 分类及优先级；正式 :540-575 仅 general/relationship 范围、soft/hard 截止及时间，没有 Demo 分类和优先级控件。需按正式 Goal 契约决定适配/明确差异，不可伪造字段。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:4210`; `apps/web/src/react-app/pages/GoalsPage.jsx:391`。
- **life-diary-read** — Demo :4477-4482 标题及 mood；正式 :753-757 以日期作标题且无 mood 徽标。必须对齐实际日记数据契约，缺字段不能捏造。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:4360`; `apps/web/src/react-app/pages/DiaryPage.jsx:43`。
- **life-diary-annotation** — Demo :4509 内联 Textarea，正式 :358 WorkspaceCommandDialog；对照时应登记展示结构差异并实际检查，不代表业务有错。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:4490`; `apps/web/src/react-app/pages/DiaryPage.jsx:358`。
- **life-diary-reply** — Demo 立即追加 replies；正式留言是实际 owner request，并保留处理记录，不能将已受理当作角色已回应。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:4530`; `apps/web/src/react-app/pages/DiaryPage.jsx:432; apps/web/src/react-app/pages/DiaryPage.jsx:826`。
- **life-diary-shared** — Demo :4561 c.id===linwan 硬编码共同经历；正式 :934 渲染真实列表，没有理由复制该 ID 限制。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:4559`; `apps/web/src/react-app/pages/DiaryPage.jsx:920`。
- **life-reflection-detail** — Demo 反思弹窗直接修改本地 decision；正式 submitReflectionFeedback 是反馈请求，原反思实际处置应由 selected.dispositions/result 表示，不能覆盖成 Demo decision。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:5397`; `apps/web/src/react-app/pages/MemoryPage.jsx:1278; apps/web/src/react-app/pages/MemoryPage.jsx:593`。
- **life-self-detail** — Demo :5529 允许修改 claim 文本；正式 :549 manageSelfClaim 使用 review/forget/restore 契约。不能伪造文本修改成功；核查 source capability 并记录允许差异。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:5479`; `apps/web/src/react-app/pages/MemoryPage.jsx:501; apps/web/src/react-app/pages/MemoryPage.jsx:1362`。
- **management-conversation** — Demo failurePolicy fallback/stop 与正式 error/legacy_fallback 枚举不同，交接上限也受正式约束；UI文案可贴近，提交必须正式枚举。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:773`; `apps/web/src/react-app/pages/ManagementPage.jsx:34; apps/web/src/v3/proto/components/configuration-fields.js:9`。
- **management-diagnostic-detail** — Demo :2821 明示前端演示，无后台调用；正式不得复刻自动计时进度/静态允许动作。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:2673`; `apps/web/src/react-app/shared/WorkflowRecovery.tsx; apps/web/src/react-app/pages/TaskDetailPage.tsx`。
- **management-data-overview** — 语义差异：Demo :1797 整个 state 快照；正式 :20 导出独立定义包，排除生活记录/凭据。需明示差异，不可改成导出浏览器状态冒充业务备份。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:1771`; `apps/web/src/v3-domain/data-tools.js:18; apps/web/src/react-app/pages/DomainPage.tsx:21`。
- **management-data-import** — Demo :1809 限8MB/version3 并按ID合并；正式配置包≤3MiB且校验digest，导入独立草稿，不能照搬覆盖已有数据。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:1940`; `apps/web/src/v3-domain/data-tools.js:24`。
- **management-data-backups** — Demo 本地备份名与删除/下载不等同正式维护副本；正式 :business-backups.js:12 要求停止写入并真实核验，不提供相同删除按钮，不能伪造。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:2014`; `apps/web/src/v3/proto/components/business-backups.js:9; apps/web/src/v3-domain/data-tools.js:35`。
- **management-data-demo-reset** — Demo专用工具允许差异，应登记；不得给正式加清库入口。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/LifePages.tsx:2179`; `明确不适用：正式无 Demo 状态重置入口`。
- **creation-blank** — 正式 creation-controller.mjs:377 空白分支直接 preview；激活 :653 使用 blank_slate。不得复用非空白分析结果。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/CreationPage.tsx:1346`; `apps/web/src/react-app/views/CharacterWizard.jsx:630; apps/web/src/state/creation-controller.mjs:352`。
- **creation-review-relations** — Demo :1870 独立人格模式 select、:1908 双人格示例按钮；正式 :1119 统一结构化编辑人格系统，功能入口结构不同，需实测编辑可达性。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/CreationPage.tsx:1858`; `apps/web/src/react-app/views/CharacterWizard.jsx:1111`。
- **creation-seven-stages** — Demo PREPARATION_STEPS src/lib/creation.ts:387；正式实际阶段数据来自 readApprovedCreationProgress，不能复制 Demo 定时推进。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/CreationPage.tsx:946`; `apps/web/src/react-app/views/CharacterWizard.jsx:1244; apps/web/src/approved-ui/creation-progress.mjs; apps/web/src/state/creation-controller.mjs:705`。
- **creation-demo-scenarios** — Demo专用控件可不进入正式；同状态应通过固定fixture或真实错误制造并独立验证。 来源：`/private/tmp/fluctlight-demo-replica/fluctlight-workspace-demo/src/pages/CreationPage.tsx:2014`; `明确不适用：正式无生产演示开关`。

## 使用边界

数据链记录可定位的 browser/Core 路由或具体实现符号，属于静态映射。路由存在、调用存在并不证明服务可运行、写入被执行、结果持久化或跨页回读正确。主线程必须用同内容/同状态桌面及窄屏场景补充证据。

Demo 数据快照/前端定时器/直接采纳与正式持久化任务语义明显不同。这些差异需要明确记录，不能把 Demo 的本地结果当作正式任务结果。
