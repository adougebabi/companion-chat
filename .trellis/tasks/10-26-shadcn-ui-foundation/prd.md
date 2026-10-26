# React shadcn/ui 基础组件迁移

## 目标与已确认决策
将正式前端的标准控件迁移至 **React 版 shadcn/ui + Tailwind CSS + Radix UI + lucide-react**，更新规范，报告剩余手写/可封装组件。
用户明确接受 Vue 到 React 的框架迁移；不能用 shadcn-vue/Reka 替代。
用户进一步要求删除旧 Vue 文件、迁移或移除 Vue 测试；legacy=1 的页面和路由必须删除，任何方式都不能再进入旧界面，不做兼容或重定向。
用户最新要求停止扩大代理回归，迁移后由用户进行一轮完整使用回归，再按反馈修复。

## 需求
- R1：正式入口和9个主页面实际使用 React，不能只安装依赖。
- R2：Button/Input/Textarea/Label/Select/Checkbox/Switch 等首批标准控件，以及页面已采用的卡片/头像/状态/弹层/标签页/披露/菜单，使用官方 React shadcn Radix 组件。
- R3：保留当前业务的真实 API、Owner/角色范围、冻结请求身份、CAS、原请求恢复、草稿及输入法语义。用户明确删除的旧界面入口不保留。
- R4：移除 Vue/Pinia/Reka、SFC/compiled renderer、旧路由挂载与 QA 演示入口。有效的当前详情/编辑入口由显式 React 页面承接；legacy=1 和未知旧路由不可用。
- R5：可维护的 React 源码、具名业务绑定和清楚的 UI/状态/服务边界；不保留 Vue 编译协议。长期异步模型可使用单一 React lifecycle/subscription hook，简单 UI 优先直接 hooks。
- R6：旧实现专用测试退役，领域/协议测试保留；关键 UI 行为迁至 React DOM。同步当前规范与剩余清单。

## 验收
- 默认生产图不含 Vue、Reka、旧页和旧 router；无额外可执行旧 HTML 入口。
- 标准组件被实际界面引用，Tailwind 真正加载，普通 UI 图标使用 Lucide。
- 基础输入、选择、禁用、提交、键盘、弹层关闭与必要状态回归有证据；类型检查与生产构建通过。
- 交付报告准确列出迁移范围、删除项、剩余手写/组合组件及验证边界。
- 后续完整使用验收由用户执行；不以 fixture 冒充真实服务/部署验收。

## 交付状态
已实施，见 migration-report.md。代码按用户指令提交，未推送/未部署；任务保留 in_progress 接收用户回归反馈。
