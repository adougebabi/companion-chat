# R34C UI 审计修复报告

基线：`6775c299`，独立未提交变更集。没有执行 Git 写入、暂存或提交，没有修改 `/workspace/accept` 或任何 `*.env`。图片仅在 `/tmp/r34c-shots`。所有数据库测试使用随机一次性 PostgreSQL 数据库，仅删除各自创建的测试库。

## 逐项根因、文件、回归与 shadcn

| 项目 | 根因与结果 | 生产文件 | 回归 | 复用 shadcn |
| --- | --- | --- | --- | --- |
| C：诊断“需要处理”与列表不一致 | 原来只从最近 20 条 workflow 分类，遗漏旧失败；现与通知共用全部 `failed_task` 原分页来源，补齐旧事项并排除已恢复/替代失败。摘要、过滤、列表共用同一集合，读取失败不伪造 0。 | `approved-ui/task-presentation.mjs`、`shared/DiagnosticsWorkspace.jsx`、`core/pending_center.go` | `r34c-task-audit`、`TestR34CPendingAttentionSharesAllActionRequiredStates` | Button、Input、Tabs、Badge、Select |
| C：创作资源数与草稿卡片不同 | 原数字只数已发布资源，卡片还展示草稿；改为当前已读资源总数，明确已发布/草稿/归档明细和更多页 `+`；移动端保留可见统计。 | `views/StudioDirectory.jsx`、`styles/replica-studio.css` | `demo-replica-studio` 新统计行为回归；桌面/移动生产截图 | Card、Badge、Button |
| C：非当前世界人物/事件没有下一步 | 分支仅显示提示，未提供当前世界已有的准备入口；加入人物/事件底稿按钮，只导航，不切换绑定、不触发事件。 | `views/WorldPage.jsx` | `r34c-world-wizard` 精确导航与零写入回归 | Button、Card、Empty |
| C/R7：Toggle Sidebar 英文 | 本地 shadcn Sidebar 保留生成模板的隐藏英文和 rail aria/title；全部改为“切换侧栏”，调用处现有中文名称保留。 | `components/ui/sidebar.tsx` | `r34c-sidebar` 实际切换、sr-only/aria/title；生产两尺寸扫描 | Sidebar、Button |
| R5：默认居民称呼泛化 | 种子源硬编码“当地居民/常住居民”；改为“周予安/林晚晴”，与不可变 Studio 发布出处保持一致。 | `core/world_bootstrap.go` | `TestDefaultWorldResidentsHavePersonalNamesAndMatchingPublication` | 后端不适用 |
| R5：默认地点无描述 | 8 个默认地点只有名称；为每个种子补入独立场景描述。 | `core/world_bootstrap.go` | `TestDefaultWorldPlacesHaveDistinctSceneDescriptions`、`TestWorldBootstrapPreservesCustomFrameworkAndExistingResidents` | 后端不适用 |
| R5：世界列表与详情名称不同 | 详情 hero 硬编码“一座属于日常的世界”；统一使用实际 `world.name`。 | `views/WorldPage.jsx` | `r34c-world-wizard` 列表/详情名称断言 | Card |
| R5/R6：向导未选世界却声称已选日常世界 | 控制器禁用下一步后，保留草稿的 daily/发布底稿字段仍进入预览；RadioGroup、说明、预览现共用有效选择，未选一律“待选择”，不改原恢复草稿。 | `views/CharacterWizard.jsx` | `r34c-world-wizard` 历史 daily/发布底稿、实际选择、下一步状态；`r20-creation` | RadioGroup、RadioGroupItem、Button |
| R6：世界列表三个创建入口 | 页头两个入口加无条件虚线卡；保留一个“新建世界”主要入口，删除重复入口，空搜索也不再显示虚线创建卡。 | `views/WorldPage.jsx` | `r34c-world-wizard` 正常/空搜索入口数量、零写入 | Button、Empty |
| R6：世界事件时间含义不一致 | 诊断显示时钟建立时刻，详情显示小时槽，缺少含义；两端共享 `20:00 整点（20:39 开始）` 格式并保留日期，原时间字段和调度数据未改。 | `approved-ui/task-presentation.mjs`、`shared/DiagnosticsWorkspace.jsx`、`shared/WorldAutomaticEvents.jsx` | `r34c-task-audit` 同时渲染三处格式；生产截图 | Badge、Card、Button |
| R6：诊断缺少世界筛选 | 只有状态/类型；新增“任务世界”，两类服务端读取都传 `worldId`，列表和统计同步过滤。 | `shared/DiagnosticsWorkspace.jsx` | `r34c-task-audit` 操作真实 Radix Select 并检查请求参数 | Select |
| R5：衣柜准备与衣物数矛盾 | 目录与初始化是独立来源，所有未完成状态被笼统称为“衣柜正在准备”；现区分已登记衣物与尚未开始/进行中的穿搭准备，保留失败与恢复入口。 | `shared/WardrobePreparation.jsx` | `demo-replica-wardrobe` 新增 8 件衣物两种准备状态回归 | Button、Collapsible、Alert |
| R5：穿着摘要用中点串成长文本 | `WearingSummary` 将所有描述 join 成一段；改为每件衣物一条语义列表，完整保留描述与顺序。 | `shared/WearingSummary.tsx` | `demo-replica-wardrobe` 两条精确衣物描述；两尺寸生产截图 | 所属 Card；原生 ul/li 语义 |
| R5：无筛选的空记忆仍提示清筛选 | 所有空列表共用过滤空态；无筛选提供“去聊聊天”，有筛选保留清除行为。 | `pages/MemoryPage.jsx` | `demo-replica-memory` 无筛选→搜索→清除的完整行为 | Empty、Button |
| R7/PM：通知只称“新回复” | 投影标题固定，UI 未带来源角色；新记录按权威角色名入库，旧记录列表/详情也显示精确“<角色名> · 新回复”；其他业务通知原中文映射保留。 | `core/notification_center.go`、`approved-ui/notification-title.mjs`、`pages/InboxPage.jsx` | `TestReplyNotificationTitleUsesAuthoritativeCharacterName`、`demo-replica-inbox` 列表/详情及非回复标题回归 | Dialog、Card、Button |
| R7/PM：形象/日程等运行任务没有 15 分钟提示 | 无开始时间时先读不断变化的 updated_at，无效非空时间也挡住回退；所有 running 类型改为优先有效开始/创建证据，页面每 30 秒更新展示时钟，世界任务保留当前周期实际开始。 | `approved-ui/task-presentation.mjs`、`shared/DiagnosticsWorkspace.jsx`、`core/task_read.go` | `r34c-task-audit` 七种任务×四种 running 状态、真实形象/日程行；`TestR34CWorldTaskRetainsActualCycleStartForStalePresentation` | Badge、Button |
| R7：通知服务异常与任务中心数量不同 | 两页原来使用不同来源和状态范围；pending 补齐既有 attention 分类，诊断复用该来源；单条详情失败或并发恢复仍保留同一次 pending 快照成员和计数，可真实重试。 | 同第一项 | 同源分页、详情失败、并发恢复、Owner 迟到响应隔离回归 | Select、Button、Alert |
| R7：向导第二步标题被滚动遮挡 | 只让 h1 scrollIntoView，未重置实际 shell 滚动容器；步骤变化滚动 `.workspace-page` 到 0 并聚焦新标题。 | `views/CharacterWizard.jsx` | `r34c-world-wizard` 实际滚动 owner 与焦点断言；桌面/移动生产截图 | Button、RadioGroup |

路径前缀：前端为 `apps/web/src/react-app/`（另列 `approved-ui/`、`components/`、`styles/` 的使用 `apps/web/src/`），后端为 `apps/core-go/internal/`；测试分别在 `apps/web/test/` 与 `apps/core-go/internal/core/`。

默认名字和描述只影响显式新世界种子。按现有共享世界合同，没有自动回填既有世界，也没有覆盖用户自定义名称、场景、出处或历史事实。

## 验证结果

- 定向 JS 共 **157 项通过**：剩余 UI 58 项；诊断/任务 39 项；世界/向导与 r20 15 项；相关 world/r4/r25 43 项；r16 中两条世界选择用例。未跑全量 Web JS。
- 通知标题最终补强后，`demo-replica-inbox` **8/8 再通过**，包括其他业务通知原中文映射保留。
- 定向 Go 共 **11 项通过**：种子新增 3 项及既有世界创建/并发/回滚 4 项，任务 2 项，通知 2 项。全部使用给定 PostgreSQL URL 派生的隔离临时库，`GOCACHE=/tmp/fluctlight-go-cache`。
- Web `typecheck`、`lint`、`build` 通过。Node 为 `/tmp/node-v24.12.0-linux-x64/bin/node`；系统 Corepack 不兼容/缓存目录只读，使用已有 `/tmp/r4-corepack/pnpm/11.0.7/bin/pnpm.cjs` 运行原 package scripts，没有修改系统或项目配置。
- 本地生产 `dist` + 只绑定 loopback 的隔离 HTTP fixture，headless Chrome 桌面 1440×1000 和移动 390×844：**28 张截图通过**，检查横向溢出、真实控件、计数、空态、滚动和无页面异常；见 `browser-results.json`。世界事件说明在窄屏独占一行，增加实际文字列宽至少 200px 的几何断言，避免时间被操作控件挤成碎行。fixture 不连接真实 Core、模型或用户数据库。
- 构建保留既有大 chunk 提示，生产样本排除和 React-only 图谱检查均通过。`git diff --check` 通过。

## 所有修改的旧断言

1. `r16-regressions.test.mjs`：查找“使用日常生活世界”按钮 → 精确 `role=radio` 与 `aria-label`；旧按钮已被 R4 的 Radix RadioGroupItem 取代。点击目标同步迁移，原禁止隐式建世界/下一步门禁/零写入断言保留。
2. 同一用例 `/创建世界/` 文案 → 精确“创建新的世界底稿”radio 存在性；当前文案改变，保持并加强真实入口覆盖。
3. `diagnostic-workspace.test.mjs`：combobox 数 `2` → 精确 `[任务状态, 任务类型, 任务世界]` aria-label 数组，因为新增世界筛选；校验更严格。
4. `r23-ui.test.mjs`：`10-31 14:15` → `10-31 14:00 整点（14:15 开始）`，保留日期和真实时刻，同时覆盖新显示要求。
5. 同一用例的无周期证据 world workflow `/进行中/` → 精确“状态待确认”。D7 已明确生命周期 running 不能证明本小时生成，旧期待已失效；实现没有恢复旧错误状态。

报告的两项全量套件旧失败均已在对应定向用例中复现、解释并修复过时测试，没有削弱其他已有断言。

## 可选项

未加入强制英文过滤器。检查发现私聊表达协议属于按版本/hash 冻结并可持久恢复的历史合同；本次没有为可选的零散英文优化改动该协议、旧草稿或模型入口，避免在 UI 修复中改变对话恢复契约。没有新增任何输入/输出 token 限制。

规范同步：`.trellis/spec/frontend/feedback-ux-contract.md`。任务保持独立可审阅文件，没有运行会自动提交的 archive/journal 脚本。
