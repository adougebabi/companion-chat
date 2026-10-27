# v6.2 r12 修复与回归记录

范围以《摇光-UIUX走查-r12.md》为准。用户要求不创建任务，仅对原问题做两轮回归；最后要求停止追加 QA，提交当前改动，后续视觉问题另提一轮。

## 已提交的修复

- 角色库与世界共用 EntityCardGrid，恢复 Card 网格；单世界不拉满。角色名两行截断配合 Tooltip，头像保留真实数据与无图回退。世界仅展示已配置地点场景，无图时明确说明。
- 侧栏采用 SidebarGroup/Menu/MenuButton/Sub 原语，宽度 256px、导航字号 14px，保留生活折叠、角色范围、全局入口及手机跳转关闭。清除旧 nav-item 多轨 CSS，active 使用明确 true 值。
- 行内更多菜单统一 end，顶栏角色切换保留 start。普通操作按钮使用内容宽度；继续创建、准备刷新、日程、创建向导、测试连接不再受 Card stretch 影响。
- 衣柜、记忆、世界确认使用完整 AlertDialog 标题、说明、Footer、Action、Cancel。保留原回调与关闭时机；世界/记忆异步失败保留确认框，取消不写入。
- 修复 Surface 误收 Disclosure 富文本 title/open 的调用：衣柜准备保持可读步骤，设置高级分组改为真正 Disclosure，章节定位可展开且折叠保留编辑值。
- 聊天消息级失败反馈使用 Alert；旧域日记无正文生成态移除原生 progress，未知进度用 Skeleton，失败说明保留恢复入口。

## 核实后保留的语义

- DateField 实际为 Calendar/Popover；time 为 InputGroup 文本时间输入，补足 undefined carrier props 下的默认格式约束。
- number 保留 shadcn Input 的数值、边界与键盘语义，配合自定义 +/-，隐藏浏览器 spinner。
- Disclosure、通知整行与 Select 的结构宽度保留；Button asChild 包 a 保留链接、新窗口语义。
- 网格末行不补假数据；媒体/创作的追加加载保留真实 append 语义，不伪造上一页游标。衣柜 AspectRatio 为报告可选项，未扩大调整。

## 验证证据与边界

- Web 自动测试 395/395；typecheck/build 通过。生产 fixture 排除与 React import graph 守卫通过（314 个本地模块）。既有主 chunk 体积警告保留。
- 两轮均覆盖角色库、单/多世界、衣柜、日程、创作、设置、创建、聊天的 1280/1440 CSS 宽度，以及菜单、取消、设置章节、数值步进、侧栏折叠、日历、390px 卡片和侧栏跳转。
- [第一轮](./evidence/v62-r12/round-1.jsonl)与[第二轮](./evidence/v62-r12/round-2.jsonl)记录测量快照；[汇总](./evidence/v62-r12/summary.json)。早期未等待父页测量同步的两条确认采样被后续 footer-ready/cancel-ready 采样替代，不作为验收证据。
- 1280 下卡片约 301.66px；1440 下 261.25px；390 下 349px。继续创建 147.25px，准备刷新 117.5px，日程 105.25px，测试连接 68.5px。
- 使用真实 production dist 与隔离合成数据；桌面通过精确 CSS 宽度 iframe 测量，外层 IAB 缩略显示，不宣称原生 1440 截图。窄屏截图见 [library-narrow.png](./evidence/v62-r12/library-narrow.png)。
- 浏览器记录无业务/生成写入；记录中的一次 /auth/logout 请求由隔离服务以 503 拦截，未修改真实会话。未恢复模型/媒体 API、未改提示词、未调用真实模型、未部署 NAS。几何/行为检查通过不等于最终视觉验收，后续按用户新一轮反馈继续。

## 重复根因与防复发

属于 C（变更传播遗漏）、D（组合测试缺口）、E（隐含布局假设）：仅换组件名或去旧 class，没有核对实际网格宿主、Card stretch、菜单默认 align、Surface/Disclosure 接口差别。前轮静态标签检查无法发现真实几何与 JSX 富文本属性问题。

已增加 r12 行为回归与只读 DOM 测量 helper，覆盖真实侧栏原语、数值/日期、确认失败与取消、设置折叠草稿和错误标题；相关契约写入 frontend 规范。此仓库无 Trellis 模板源目录，不创建无关模板或任务。
