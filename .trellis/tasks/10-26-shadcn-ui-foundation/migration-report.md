# React shadcn/ui 基础组件迁移交付

日期：2026-10-26。分支：codex/fluctlight-functional-recovery。
状态：迁移代码已交付工作区；按用户最新要求，停止扩大代理回归，等待用户统一使用回归。用户已授权本轮 Git 提交；未部署、未归档。

## 已迁移

正式入口为 `apps/web/index.html → src/main.tsx → react-app/App.tsx`。
技术栈为 React / React DOM、官方 React shadcn/ui 的 **radix-nova** 基础库、Tailwind CSS 4、Radix UI、lucide-react。
基础组件目录 `src/components/ui/*.tsx` 有 18 个实际接入的组件家族：

| 家族 | 当前用途 |
|---|---|
| Button / Input / Textarea / Label | 页面操作、聊天、配置、资料、世界及资源表单 |
| Select / Checkbox / Switch | 类型/角色/协议选择，多选集合和独立开关 |
| Card / Badge / Avatar | 列表、状态、角色头像与失败回退 |
| Tabs / Separator | 衣柜、管理中心与导航结构 |
| Dialog / AlertDialog / Sheet | 预览、确认、详情编辑和手机导航 |
| DropdownMenu / Accordion / Collapsible | 更多操作、治理记录和独立展开区 |

9 个主工作区页面控制器已经转为 React；登录、当前界面使用的诊断详情、资源编辑、头像/形象与治理入口由明确的 React 页面承接。
当前相处界面、配置和页面布局仍保留已确认的 Sage 视觉方向，没有改变后端协议或模型提示词。

## 旧实现清理

- 删除 122 个 Vue SFC；`apps/web/src` 现有 `.vue` 文件数为 0。
- 共清理 235 个旧 UI/桥接/编译渲染/路由/QA 入口文件，列表见 `research/removed-ui-files.json`。
- 移除 Vue、Pinia、Reka UI、VueUse、Vue 图标、shadcn-vue、Vue Vite 插件和 vue-tsc 依赖。
- 删除旧 proto pages/modules/app、LegacySurface、旧 QA HTML 和演示 UI 注入模块。
- `legacy=1` 直接判为不可用，不重定向、不挂载旧页。重复/编码参数也有回归断言。
- 当前真实服务、DTO 映射、持久命令和纯辅助函数仍复用原实现；目录名 v3/proto 不等于仍保留旧页面。

## 维护性处理与仍可改进之处

React 壳层、聊天、登录和支持页使用直接的 React hooks；通用交互来自官方 Radix 组件。复杂业务页拆成具名控制器与 React 展示视图，不再保留 Vue 编译缓存、指令、SFC 或双框架桥接。

长期异步业务仍通过一个 React controller hook 连接可订阅模型。它管理稳定快照、提交后的 DOM 效果与清理，不解释 Vue 配置。该层和复杂页绑定的类型仍可进一步细化；部分历史 JavaScript 业务逻辑没有在本轮强行改写为完整的 TypeScript 领域模型。这是后续维护性优化项，不能说所有页面都已成为最简原生 hooks 实现。

移植期间的媒体动态标签、筛选结果接线、记忆 ID 状态、确认关闭、评论 IME 和隐藏表单控件外层滚动问题均已修复；相应行为已纳入测试或浏览器证据。

## 剩余手写控件 / 可封装组件

| 优先级 | 现状 / 位置 | 后续建议 |
|---|---|---|
| P1 | 日记月历：`react-app/views/LifePage.jsx` | 用 shadcn Calendar / DatePicker 统一日期选择；保留业务时区、可选日期和已加载记录规则 |
| P1 | 模型目录与模型 ID：`shared/ModelDirectory.jsx`、`views/ManagementPage.jsx` | 整合为 Combobox / Command；目前是标准 Input 加 datalist/搜索列表 |
| P2 | 筛选 chips：`react-app/components.tsx` | 可组合 ToggleGroup；现已使用 shadcn Button，但筛选状态与分组布局仍是业务封装 |
| P2 | 目标进度、音频进度：`views/LifePage.jsx`、`views/BrowsePage.jsx` | 可统一为 Progress；音视频播放本身应保留原生媒体元素 |
| P2 | 可滚动列表与长详情 | 可统一 ScrollArea 和滚动容器；目前保留受控原生滚动与布局 CSS |
| P2 | Loading / Empty / Error | 可提炼 AsyncState、EmptyState、ErrorNotice，结合 Skeleton / Alert，避免每页重复文案与布局 |
| P2 | 配置字段 / ProfileForm / ProfileField / SettingsField | 继续收敛类型化 Field/FieldGroup，统一 null/number/boolean、标签与错误关联 |
| P2 | ActionFeedback、原请求与 readback 恢复 | 已有共享组件；可进一步统一 RequestFeedback / RecoveryActions，保留原身份与状态语义 |
| P2 | 日程条目、目标卡片、世界地点树、媒体卡片 | 可提取稳定业务组件，降低大型页面视图长度 |
| P3 | CurrentOutfit / GarmentArt / SemanticTree / PreparationSteps | 属于领域展示，应保留自定义；内部基础控件继续使用组件库 |
| P2 | `react-app/page.tsx` 与复杂页面具名绑定 | 进一步类型化 controller/view 边界，并逐步把简单页面收敛为直接 hooks |

原生日期/时间/文件输入、audio/video、业务插画，以及纯布局元素不是需要机械消灭的手写控件。

## 迁移提交 1a372e55 时的验证（不代表之后的合并验证）

- 上一次完整前端回归：311 / 311 通过，见 `evidence/last-full-tests.txt`。
- 独立审查发现问题的 React 交互回归已通过，见 `evidence/review-regressions.txt`。
- 最终收尾执行 TypeScript 检查及生产构建，均通过。
- 构建检查默认产品无 fixture 数据/旧 QA；实际入口依赖图为 138 个本地模块，无 Vue/Reka/旧页面或旧路由；同时检查 root/public HTML 入口。
- 桌面 1440×900、窄屏 390×844 无文档横向溢出；修复后桌面文档高 900、手机文档高 844，滚动由页面内容拥有。
- 生产构建的 Select 弹层与 Escape、Tabs 切换、手机导航 Sheet 关闭及旧 URL 不可访问已检查。
- 截图为隔离样本：`evidence/react-desktop.jpg`、`react-mobile.jpg`、`legacy-route-unavailable.jpg`。
- 构建仍提示一个共享包超过 500 kB；未修改告警阈值掩盖它，页面已经按路由 lazy load，进一步拆包留作优化。

用户已明确接回下一轮完整回归，因此不继续扩大代理操作验收。隔离 fixture 和 DOM 测试不代表真实 PostgreSQL、Temporal、Provider、模型或 NAS 部署验收；本轮未部署，也未向真实服务发起无意义写入。

## 测试清理依据

旧 Vue 与已删除页面专用的源码/renderer harness 已退役，分别记录于 `research/retired-vue-tests.json` 与 `retired-route-tests.json`。纯领域、API provider、原请求恢复、CAS、时区和流式契约保留。配置异步保存、世界编辑、聊天草稿/IME、精确 Select 值、通知、媒体、记忆、关闭焦点、路由拒绝等迁到真实 React DOM 回归。

依赖清理曾被自动审批因验证覆盖不足拒绝；补齐 React 回归、完成当时完整测试及类型检查后，单独提交无使用点依赖的清理命令，已获允许并成功执行。当前无审批阻塞。

## 远端 R10 合并补充

最初迁移从本地 `23e04bc7` 开始时漏了 fetch；远端已存在
`195017fc`（R10 修复）和 `6cab8a7d`（journal）。这导致后续冲突。
本次使用 merge 保留迁移与远端两条历史，不整体取任意一侧。

- 保留远端 Core/browser-client 的独立世界配置复制契约，以及原有无 token 上限的提示词规则。
- React 创建流程接入已有世界配置选择、源版本冲突的明确复核、复制/选择/读回的持久阶段恢复。
- React 接入人格切换读回保护、登录状态读取重试、12 字符密码下限、退出确认失败保留会话、回复设置返回聊天、媒体全局默认、手机预览收起、无角色时全局页面可用等 R10 行为。
- 资源目录和媒体主列表由新增 React 页面承接，使用 shadcn 控件。
- 远端独有的对象编辑、历史、治理和恢复功能在 `v3-domain/` 中保留，通过 React `DomainPage` 和显式 `feature-routes.mjs` 接入。它们不是旧应用/路由/壳，但内部仍含命令式 HTML 控件。这是当前迁移边界，不应宣称全部深层页面已改为 React JSX。
- 仍禁止 `legacy=1`；远端原先“忽略该参数”的行为已按用户要求改为直接拒绝。Vue 页面、SFC harness 和模拟 reference-fixture 不恢复；R10 业务回归改为 React/共享状态层。

新增优先迁移项（P1）：`v3-domain/` 的领域表单、版本/历史列表、Drawer/Dialog、原请求恢复按钮，以及媒体上传 `v3/proto/components/media-upload.js`。后续应沿真实路由逐个用 React/shadcn 替换，并继续复用已有命令、CAS 和读回协议。`DomainPage` 只负责挂载和清理，不应增长为另一个框架适配器。

本次合并仅做必要的类型/构建和受影响边界检查；未再次进行全量浏览器、真实模型或数据库回归。具体记录见 `merge-r10.md`。原先另外任务的未跟踪研究与证据保持不动。
