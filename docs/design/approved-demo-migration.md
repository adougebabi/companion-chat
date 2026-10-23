# 用户 HTML demo 迁移记录

来源：用户 2026-10-23 提供的 `index.html`。SHA-256：
`63cde9b40bb72c5eec726472f898e6f4c7243aa6c7bf4f4c32b65bd68a699da9`。
用户确认：正式页面对接真实后端，模拟开关仅演示可见。不新建任务。

保留原始样式、SVG 图标几何、原生 DOM 根元素、布局、间距和交互结构。
源文件是打包后的 Vue 应用：迁移保留其 render 结构，页面行为由可读的
controller 对接现有真实服务，复用项目 Vue，不引入第二套运行时。

## 公共组件

| 组件 | 复用范围 |
| --- | --- |
| ActionButton / Surface | 原生按钮与语义容器，迁移时抽出 102 个按钮、32 个容器调用点 |
| CurrentOutfit | 聊天摘要、此刻页的实际穿着 |
| ActionFeedback | 日程、日记、记忆、世界、衣柜、动态的原操作反馈 |
| FilterChips | 记忆、人物、媒体、角色库、诊断的搜索与筛选 |
| DetailDrawer | 统一详情/编辑抽屉、键盘焦点与关闭 |
| GarmentArt / icons | 原 demo 服装插图与 Lucide SVG；附原许可说明 |

代码位于 `apps/web/src/approved-ui/`，样式位于
`apps/web/src/styles/approved-ui.css`。抽组件改变了编译器原先的块边界，
因此对保留的 render 使用普通 Vue patch，避免旧优化列表导致页面切换残留。
回归检查覆盖实际切页和动态文本、属性、事件更新。

## 真实接入的必要差异

- 正式构建不含模拟开关和 demo 数据；显式 mock 构建保留用户原始演示行为。
- 日期、日程、状态、角色资料、穿着、记忆、动态、通知和媒体读取真实接口。
- 配置采用 CAS 与确切提交回执版本；新输入不被迟到成功覆盖，凭据不持久化。
- 世界扩写对每个实际请求保留原 body/key；重试应用时不重新生成版本基线。
- 安静时段增加可选 weekdays，跨夜按开始日计算，旧策略仍按每天生效。
- 当前记忆接口只支持正文修订/遗忘，其他元数据只读。服务端没有方案删除
  接口，因此删除按钮禁用并说明，不再显示本地假成功。
- 原 demo 的创作中心只有“即将重新设计”；保留此状态并提供真实创作工作台入口。
- 原有登录、创建角色、治理及资源/诊断深链继续通过 LegacySurface 可达。

## 验证与证据

最终自动检查：Web 360/360、Browser client 39/39、Core client 3/3；
workspace typecheck/build、生成产物检查、mock 构建通过，安静时段 Go race
检查及 Go Core/Browser vet、全包 build 通过。

- 原始 HTML 与隔离 mock 构建的聊天首屏在相同 1280×720 尺寸比较，
  921,600 个像素中差异为 **0**。这只证明该页面/该状态，不扩展为所有页面零差异。
- 生产构建使用受控 API 数据检查全部主页面、日程跳转、衣柜动态摘要、
  日记日历、记忆抽屉、角色库和模拟开关隔离。
- 新检查覆盖配置完整预校验/回执/新输入保留、临时 endpoint ID、原阶段恢复、
  路由兼容、原生 DOM、动态组件、重复切页、作用域/迟到响应和请求观察恢复。
- `pnpm generate`、workspace typecheck/test/build、mock 构建与 Go 安静时段
  race 检查见提交时运行记录。Go vet/build 通过。
- Go 全量回归有 6 项既有失败：两个初始化背景类型检查、无参 Tool schema
  parity、旧 Phase8 CI gate 断言，以及两个 shared-experiences overview
  路由矩阵断言。本次没有改动这些逻辑。
- 未运行真实 PostgreSQL/模型环境端到端验收；受控浏览器数据不代表该验收通过。

证据：`evidence/approved-demo/reference-chat.png`、
`evidence/approved-demo/isolated-demo-chat.png` 和生产构建预览截图。
