# Fluctlight 全站风格与交互一致性巡检报告

巡检日期：2026-10-13（Asia/Shanghai）。目标：[Fluctlight](http://192.168.1.20:13001)。使用现有登录态；登录画面及密码修改未测试。桌面视口为 1440×1000，窄屏视口为 390×844，结束时已恢复原视口。

本次访问了全部 4 个一级入口、设置的 7 个二级入口、诊断中心的 7 个二级入口，以及聊天会话、人格详情四个标签和编辑与治理。菜单访问覆盖完整；窄屏、Hover、提交校验属于重点抽查，不能等同于每个控件和每种异常条件均已测试。实际操作见 [覆盖矩阵](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/coverage.md)。共保留 22 张现场截图。

### 1. 总体风格一致性健康度评估

**健康度：中等偏低。主导航与工作面外壳已形成稳定语言，但内部控件和反馈机制尚未统一。** 视觉主体是亮色、蓝色操作强调、圆角与玻璃表面，聊天、动态、设置及诊断中心的整体关系清楚。底部导航采用 Lucide，手机端可切换为单个工作面；在抽查的治理、Kev 设置和系统事件页面中，390px 视口没有页面级横向溢出。

未发现整页仍采用复古凸起边框、拟物图标或 WinXP 风格。问题集中在组件级：治理页的目标规划、Actor 编辑、Kev 设置/诊断使用了未对齐产品样式的共享控件与原生控件。采用明确计数口径：**20 个被访问的页面/栏目工作面 = 4 个一级入口 + 14 个二级入口 + 聊天会话 + 治理；其中 3 个工作面存在明显的未统一控件集中区，即治理、Kev 设置、Kev 诊断，约 15%。** 这是具有遗留体验特征的工作面比例，不是旧代码量比例，也不能据此断言组件的代码年代。其余问题如原始 ID、时间格式与 JSON 默认展示跨越更多工作面。

共同根因已由运行时与源码互相印证：产品按钮采用蓝色 `--accent`、至少 42px 高和 12px 圆角；共享 Button 默认采用深色 `--primary`、32px 高。全局 `button { color: inherit }` 又使 Kev 默认保存按钮呈深色底配深色文字，实测对比度约 **1.1:1**。目标组件已有局部文字颜色补丁，但没有把尺寸、颜色、Hover 与触控标准一并统一。

按用户要求的 Awwwards/FWA/CSS Design Awards 精品完成度衡量，当前主要差距是组件精度、信息层级、错误恢复、加载反馈和图标一致性。主页面已具备可用的产品骨架，但尚不足以支持“每日最佳同等品质”的判断。后续精品化应保留聊天阅读和频繁配置操作的效率，把创意投入品牌排版、导航转场及状态反馈；不能只依靠更多玻璃卡片或装饰动效提升品质。

本报告包含 **16 项现场问题：高 2 项、中 13 项、低 1 项**，以及 **1 项中级源码遗留线索**。同一组件会对应不同用户影响，实施时应合并处理根因，避免把条目数当作独立改造项目数。

### 2. 详细问题清单

“现场”表示浏览器实测；“源码”表示只读代码核对。本地源码与运行表现基本吻合，但未核验部署 commit，组件位置作为可信定位建议使用。

| 页面 / 路由 | 违规元素 / 交互位置 | 现场表现（老旧/割裂） | 预期设计（规范基准） | 严重级别 (高/中/低) | 疑似代码/组件定位建议 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| F01 · `/settings?section=kev` | 保存 Kev 配置 | 深色底配深色文字，标签几乎不可读；运行时背景 `oklch(0.205 0 0)`，文字 `oklch(0.145 0 0)`，对比度约 1.1:1。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/21-mobile-kev-button-contrast.jpg) | 可读的主按钮前景/背景组合，普通文字至少 4.5:1；统一 default、hover、focus、disabled 状态 | 高 | [KevSettings.vue:32](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevSettings.vue:32) 默认 Button；[button/index.ts:11](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/ui/button/index.ts:11)；[app.css:40](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/styles/app.css:40) 全局继承颜色 |
| F02 · `/instances` → 编辑与治理；Kev 设置/诊断 | 目标规划、保存顺序、过滤按钮 | 黑色整行按钮及 32px 默认控件混入蓝色圆角操作语言；`--primary` 与产品 `--accent` 没有统一，`--accent` 还存在同名覆盖。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/18-mobile-governance.jpg) | 统一产品语义 token、按钮等级和尺寸；保留业务主次关系，不依赖每个页面加颜色补丁 | 中 | [GoalPlanningPanel.vue:32](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/GoalPlanningPanel.vue:32)、[globals.css:55](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/styles/globals.css:55)、[app.css:96](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/styles/app.css:96) |
| F03 · 治理、`/settings?section=kev` 窄屏 | 上下移、规划按钮、决策点复选框 | 上下移约 32×24px；规划按钮高 32px；Kev checkbox 18px，整行 label 约 20px，10项紧贴。移动端容易误触。[治理现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/18-mobile-governance.jpg) · [Kev现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/19-mobile-kev-settings.jpg) | 项目操作最小高 40px、主导航 44px；上下移用 Lucide 操作按钮；复选项整行可点击并增加组间距 | 高 | [GoalPlanningPanel.vue:33](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/GoalPlanningPanel.vue:33)、[KevSettings.vue:23](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevSettings.vue:23)、[app.css:138](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/styles/app.css:138) |
| F04 · 治理 → 创建目标 | 空值保存 | 出现浏览器原生“请填写此字段”浮动气泡；创建人格则空值禁用按钮、分析错误显示横幅。未统一必填、校验与错误定位。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/22-mobile-goal-native-validation.jpg) | 统一字段内联错误、必填标记和首个错误聚焦；保留真实字段校验及全空白输入防护 | 中 | [GoalPanel.vue:152](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/GoalPanel.vue:152) `required` + submit、无 `novalidate`；[GoalPanel.vue:79](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/GoalPanel.vue:79) 无说明时直接 return |
| F05 · `/` 或 `/instances` → 创建人格 | 解析失败后关闭弹窗 | 弹窗内的英文服务错误关闭后仍显示在实例列表，错误越过所属任务边界。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/03-chat-search-mismatch.jpg) | 创建错误保留在创建会话内；关闭时清理局部错误，列表只显示列表加载/操作错误 | 中 | [InstancesView.vue:320](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/InstancesView.vue:320)、[InstancesView.vue:385](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/InstancesView.vue:385)、[control-center.ts:135](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/stores/control-center.ts:135) |
| F06 · 创建人格、`/chat`、Kev 测试连接 | 用户错误提示 | 分别显示英文服务错误、`agent_run_failed`、`unavailable · request_failed`，中文界面直接透出实现码。[聊天现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/17-mobile-chat-error.jpg) | 中文原因 + 可执行下一步；诊断码放可展开的技术详情并携带关联信息 | 中 | [control-center.ts:1186](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/stores/control-center.ts:1186)、[conversations.ts:538](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/stores/conversations.ts:538)、[ChatView.vue:179](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/ChatView.vue:179)、[KevSettings.vue:13](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevSettings.vue:13) |
| F07 · 治理 → Actor背景与用户背景 | 两套背景编辑表单 | 职业、背景、所在地、时区重复出现；一套普通 input，另一套标准 Input/Textarea；变更/纠正语义和载入按钮文案也不同 | 统一字段组件及操作语言，明确编辑对象与来源；保留 Actor 选择、用户专属关系字段和修订语义 | 中 | [GovernanceView.vue:122](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/GovernanceView.vue:122)、[ActorContextEditor.vue:9](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/ActorContextEditor.vue:9) |
| F08 · `/moments` | 评论作者、动态元数据 | 用户作者显示完整 `human_...`，动态旁显示完整 `fluctlight_...`；名称头部与技术元数据没有分层。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/07-moments-internal-ids.jpg) | 作者统一解析为显示名/“你”；完整 ID 移到次级详情，需要时可复制 | 中 | [MomentsView.vue:48](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/MomentsView.vue:48)、[MomentsView.vue:49](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/MomentsView.vue:49) |
| F09 · 人格详情 → 基础资料/生活脉络 | 属性文本与默认空项 | 大量“未设定”、重复身高字段、`自定义字段（...）` 和英文枚举占据主阅读面；衣柜结构拼成很长一段文字 | 常用资料优先；未知项收起；衣柜/外貌采用结构化展示，原始属性保留在高级详情 | 中 | [InstanceDetailsDialog.vue:319](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/InstanceDetailsDialog.vue:319) 属性遍历及 `formatDisplayValue`；核对生活脉络区的同类格式化 |
| F10 · 治理 → 最近规划；人格目标详情 | 时间、状态展示 | 原样展示 UTC RFC3339、`succeeded/failed/apply/no_change`；诊断页面又有显示时区选择与格式化，跨页面读法不同 | 共享时间格式器和状态词典；用户时区一致，保留完整精确时间的次级详情 | 中 | [GoalPlanningPanel.vue:40](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/GoalPlanningPanel.vue:40)、[DiagnosticsView.vue:105](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/DiagnosticsView.vue:105) |
| F11 · `/settings?section=media`、`section=operations` | 配置主表单 | 媒体工作流和运行策略以裸 JSON 文本域为主要操作；普通并发数、定期唤醒开关也要求编辑 JSON，且说明直接使用 Core/canonical reference 等术语。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/10-operations-json-form.jpg) | 常用配置表单化；媒体提供模板/导入与校验结果；JSON 留在明确高级入口 | 中 | [SettingsView.vue:208](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/SettingsView.vue:208)、[SettingsView.vue:65](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/SettingsView.vue:65) 及运行策略表单 |
| F12 · `/diagnostics?section=events`、媒体/模型详情 | 默认诊断数据面 | 系统事件默认展开整段 payload；手机上出现工作面滚动 + JSON 内滚动。其他诊断已经使用折叠详情，机制不同。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/15-mobile-events.jpg) | 摘要先显示类型、状态、时间、原因与关联；JSON 采用一致的可展开查看器、复制和明确滚动边界 | 中 | [DiagnosticsView.vue:264](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/DiagnosticsView.vue:264)、[DiagnosticsView.vue:260](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/DiagnosticsView.vue:260) |
| F13 · `/diagnostics?section=workflows` | 工作流列表 | 同一 Workflow ID 反复出现，按钮只显示 ID，没有 Run、状态或时间，无法区分各次运行；查询结果为完整 JSON | 每行显示 Workflow 名称、Run短标识、状态、时间；唯一运行 key；选中/查询/加载反馈统一 | 中 | [DiagnosticsView.vue:269](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/DiagnosticsView.vue:269)、[DiagnosticsView.vue:109](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/DiagnosticsView.vue:109) `workflowIdFor` |
| F14 · `/diagnostics?section=kev-decisions` | 过滤表单与窄屏栅格 | 9个输入框默认全部展开，手机工作面首屏大部分被多层容器及筛选占据；时间要求手填 RFC3339，缺少生命周期页已有的“清除”操作。[现场](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/16-mobile-kev-filters.jpg) | 常用条件优先、高级条件折叠；统一清除按钮、时间选择和活动过滤摘要；减掉重复容器内边距 | 中 | [KevDiagnostics.vue:8](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevDiagnostics.vue:8)、[KevDiagnostics.vue:26](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevDiagnostics.vue:26) |
| F15 · 详情、治理、设置手机列表 | 关闭和折叠图标 | 主导航已用 Lucide，但详情关闭用字符 `×`，折叠用 `⌄`，设置列表用 `›`，线宽与对齐不统一。未发现固定 UI 使用表情符号 | 使用同一 Lucide X/Chevron 系列，统一尺寸、stroke-width与按钮点击区 | 低 | [InstanceDetailsDialog.vue:297](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/instances/InstanceDetailsDialog.vue:297)、[DialogContent.vue:46](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/ui/dialog/DialogContent.vue:46)、[SettingsView.vue:92](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/SettingsView.vue:92) |
| F16 · Kev 设置、模型运行/详情、工作流 | 初始加载反馈 | 工作流显示“正在加载列表”、人格详情显示加载文本；模型详情跳转短暂出现“暂无当前诊断记录”后才显示记录；Kev重载先显示初始配置再恢复已保存配置，首帧与最终值不一致 | 区分未加载、加载中、已加载为空、失败；配置读取前明确占位或锁定表单，不能把临时默认值表现为已保存值 | 中 | [KevSettings.vue:8](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevSettings.vue:8)、[KevSettings.vue:14](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/components/KevSettings.vue:14)、[SettingsView.vue:62](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/SettingsView.vue:62)、[DiagnosticsView.vue:184](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/views/DiagnosticsView.vue:184) |
| F17 · 诊断清空逻辑（仅源码） | `clearDiagnostics` | 源码保留原生 `window.confirm`；未在线上触发，不能当作现场弹窗发现 | 接入标准 AlertDialog，保持取消语义与进行中反馈 | 中（源码线索） | [control-center.ts:1035](/Users/vinson/Documents/project/个人/local-ai-companion/apps/web/src/stores/control-center.ts:1035) |

补充交互观察：左侧聊天搜索只过滤左栏，右侧实例列表仍展示原实例，[截图03](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/03-chat-search-mismatch.jpg) 已记录。源码明确两处查询状态独立。这可能是“搜索最近会话”的既定语义，故暂不计入缺陷总数；建议先明确搜索作用域，再决定共享查询状态或改提示文案。不要无条件把右栏同步当作已确定产品需求。

**关键现场凭证**

F01：保存按钮标签几乎不可读。

![Kev 保存按钮低对比度](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/21-mobile-kev-button-contrast.jpg)

F02/F03：治理目标区的黑色操作与小尺寸上下移。

![目标规划控件风格与触控尺寸](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/18-mobile-governance.jpg)

F04：创建目标采用浏览器原生必填气泡。

![目标空值原生校验](/Users/vinson/Documents/project/个人/local-ai-companion/artifacts/ui-audit-2026-10-13/22-mobile-goal-native-validation.jpg)

### 3. 统一化重构优先级建议

**第一期按以下顺序处理，优先解决频繁可见的问题与共享根因。**

1. **统一 Button 和 token 语义，修复不可读文字（F01/F02/F03）。** 先梳理 `primary/accent/foreground/ring` 的用途和 CSS 层叠，建立产品按钮 default、secondary、destructive、ghost 契约。共享组件直接满足颜色、圆角和最小触控高度，逐步移除目标组件的颜色补丁。回归 Kev 保存、目标保存/规划、过滤及 Actor 背景，检查文字对比和所有状态。此项投入小、覆盖广。
2. **统一移动端操作行与选择控件（F03/F15）。** 上下移采用可见 Lucide 图标与至少 40px 点击区；Kev 用可点击整行的标准复选/开关组；原生多选替换为有选中摘要的统一多选组件。默认不增加拖拽依赖，先让所有设备上移/下移都明确可用。
3. **收敛表单错误与状态归属（F04/F05/F06/F16）。** 建立 FieldError、ErrorNotice、Loading/Empty 状态契约；创建错误改为局部状态，关闭后不污染列表；服务码映射成中文原因与恢复动作；读取配置完成前显示占位并锁定保存。保留原始码供诊断关联。验收包含空值、空白、格式错误、服务失败、关闭重开和重载。
4. **统一用户名称、状态与时间展示（F08/F09/F10/F13）。** 复用名称解析与时间格式器，列出业务状态词典；完整 ID 和 UTC 原值保留在技术详情。工作流列表增加 Run/状态/时间，避免相同 ID 的多条按钮看起来完全一样。投入适中，能迅速降低“半成品后台”观感。
5. **统一诊断列表与过滤（F12/F14/F17）。** 抽出摘要行、过滤栏、状态徽标、原始数据查看器和标准确认弹窗。Kev与生命周期都有应用/清除/时间条件；窄屏高级过滤可收起，JSON 默认折叠。保留技术用户对原始数据的可达性。

**第二期处理信息架构与精品完成度。** 在明确业务语义后整理重复背景入口（F07），把常见运行策略表单化、媒体提供模板和校验（F11），将人格资料的常用内容与技术属性分层（F09）。之后再统一排版节奏、表面层级和导航转场，制定动效时长/缓动及 reduced-motion 契约。这些属于体验改造，不能仅修改 CSS 就宣布完成。

统一化验收建议：桌面 1440px 和手机 390px；关键操作高至少 40px，主导航至少 44px；普通按钮文字至少 4.5:1；纯 UI 图标全部 Lucide；表单错误行内可定位；关闭弹窗不残留跨工作面错误；加载中不显示“已加载为空”或伪装成已保存配置；主要列表有可读名称、状态与统一时间；原始 JSON/ID 作为次级详情。

此次未修改产品代码，没有发送聊天或动态评论，没有创建有效人格/目标，没有保存配置、改变密码、删除记录或执行工作流控制命令。JSON解析及 Kev连接测试产生了正常测试请求/诊断记录。未执行诊断清空，因此原生 confirm 仅作为源码线索。桌面截图存在捕获缩放软化，关键问题以清晰窄屏截图和运行时 DOM 测量交叉印证；软化没有被当成网站字体缺陷。未进行独立性能计时、网络节流、200%浏览器缩放或全面屏幕阅读器测试。
