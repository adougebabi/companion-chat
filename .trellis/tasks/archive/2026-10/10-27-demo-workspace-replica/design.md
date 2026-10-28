# 技术设计

## 差距与边界
当前 React 正式壳层为 256px inset sidebar，角色切换在顶栏；角色库是普通卡片网格。Demo 为 218px 细边框侧栏、侧栏角色切换、62px 顶栏、带引导首屏和封面卡片的角色库。最小行为差距位于 React App / WorkspaceNavigation / LibraryPage 的表现层。

## 第一页修改所有权
- App.tsx：壳层尺寸、顶栏路径、跳转与通知，既有路由/数据加载保留。
- WorkspaceNavigation.tsx：扁平三组导航及角色选择，既有 canonical page 值保留。
- WorkspaceQuickJump.tsx：Radix Dialog 搜索页面/角色，Cmd/Ctrl+K，键盘和焦点交还。
- LibraryPage.jsx：复用现有 controller 的真实角色数据、配置检查与创建恢复；Demo 版首屏、搜索/标签、网格/列表及无结果态。
- WorkspaceArtwork.tsx：仅移植 Demo 的装饰性 WorldArt / StoryMark，不作为真实角色头像、世界事实或生成结果。
- styles/workspace-replica.css：明确命名及作用域，独立于未迁移页面的业务布局。全局设计变量采用新 Demo。
- sidebar/use-mobile：允许根导航指定 700px 断点，聊天侧栏保留既有独立控制。
- test：真实 React 交互验证及仅测试服务器的空白/延迟/失败场景。

## 数据流
目录及头像来自 workspace → 实际详情补充职业/标签/简介 → React 只投影。选角色仍 workspace.select + canonical navigate。草稿恢复仍 creation-recovery；列表 UI 状态按 Owner 保存在 sessionStorage，不保存第二份业务数据。错误保留已确认卡片，提供重新读取。未提供最近聊天时间或状态不伪造。

## 兼容与回滚
不改 BrowserClient、后端、聊天 store、初始化激活和工作流。既有测试中的旧导航层级断言迁移到新 Demo 的直接入口。页面逐一提交/验收；布局样式有独立 import，可回滚单页而不回滚领域功能。

## 风险
全局 rem/token 调整可能影响未迁移页，必须检查邻接聊天、管理页并记录。图库封面是装饰，不代表真实世界。fixture 不能替代真实模型/数据库验收。


## 聊天与生活页表现层
聊天当前按最新 Demo 在<=1050px 把此刻移到下方，取代旧移动 Sheet；旧测试更新为内联折叠保留草稿。WorkspacePageHeader/WorkspaceDialog 提供统一页头及有界内容滚动。对话历史只读状态独立于会话 store，明确恢复才协调服务端当前会话；原聊天领域协议不变。首次失败 loading 修复发生在 clearActiveConversation 的状态所有者。生活页继续复用 LifePage controller，只替换逐页视图。

## 第16组：通知、管理、任务、数据与账户

main已读Demo InboxPage、SettingsPage/ModelSettings、DataPanel、任务详情与AuthPage的主体及相关样式。Auth设计在Demo src/App.tsx:338–418，使用index.css:1265–1320及响应式规则；不是pages目录内单独文件。定位报告见research/management-scope-map.md，报告是线索，不替代实施者亲读要改代码。

- 通知采用Demo列表+320px待处理侧栏和同页详情，读取正式通知/待处理分页与as_of。已读与真正处理分开；只沿实际来源/任务/提议恢复，不生成演示图片或按计时完成。
- 管理采用Demo180px二级目录+28px主栏，服务/四用途子页签、分区字段及sticky保存栏。复用ManagementPage controller、configurationPlan/ack、真实字段定义和配置版本/原命令恢复。实际服务标识/协议/密钥语义及校验以正式DTO为准；不把演示名称、模型目录、范围/默认值写进正式配置。已保存连接没有删除API，不实现假删除/自动转移用途。
- 诊断与任务采用Demo列表、状态、阶段/记录/信息详情和明确原因确认。现有TaskDetailPage/WorkflowRecovery及provider为动作所有者；allowed_actions、run ID、revision和resettable checkpoints决定可操作性，queued/accepted不代表完成。保留全部诊断与来源深链接。
- 数据采用Demo统计、导出、导入预览、备份卡及确认布局，但资源包与部署业务备份分区说明实际含义。正式包仅复制独立定义草稿，不覆盖运行人物、存档或自动发布/应用；导入绑定精确原文本/digest/验证结果。业务备份由部署维护operator执行，恢复到显式隔离目标，不在浏览器伪造本地完整快照、下载/删除备份、清空工作区或时间线回滚。
- 账户采用Demo双栏故事插画/表单和窄屏单栏，保留真正setup/login/logout流程、密码12字符/初始化令牌/精确确认/错误重读，不增加假邮箱注册、直接体验绕过认证或本地密码保存。未认证页不读取/展示私有角色目录；可根据正式认证上下文调整外壳位置，记录对照边界。
- 源码装饰与业务结果分离。样式必须以replica-*作用域落地，不回退旧Vue/旧路由。所有既有对象深链接仍可达；路由到同页详情时保留来源、过滤、列表位置和未保存输入。
- 不改后端API、LLM/prompt/token策略。模型/KEV相关请求仅复用现有配置控制器与真实字段，不新增或改变输入/输出预算、截断或拒绝机制。
