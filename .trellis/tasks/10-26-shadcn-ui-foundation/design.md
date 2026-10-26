# 最终实现设计

正式入口：index.html → main.tsx → react-app/App.tsx。React 壳层显式分派当前页面；legacy=1 最先拒绝，无旧渲染 fallback。未知角色不会借用当前角色的数据。页面生命周期按 Owner 隔离。

基础 UI：components/ui/*.tsx 为官方 shadcn radix-nova 来源；controls.tsx 仅补项目语义变体、精确领域值、语义根节点和关闭契约。Tailwind/theme 在 react.css，原视觉布局限定在 react-layout.css components layer。Radix 的隐藏 form 控件由相对定位包装约束在正确滚动区域。

业务 UI：React pages/shared/components 与具名 views。壳层、聊天、登录及支持页直接使用 hooks；复杂长寿命模型通过 useController 管理订阅、提交后效果与清理。它不解释 Vue options/template/VDOM/directives。该层的类型收敛是后续维护候选，具体见交付清单。

状态：state/workspace.mjs、conversation.ts、creation-controller.mjs 复用原领域逻辑，以 Valtio vanilla 提供可订阅状态与缓存快照。保留 BrowserClient/providers、请求 epoch、冻结 body/key、CAS、Owner/fid 约束和原请求回执。AbortController/DOM 节点不代理。

旧实现：已移除 SFC、Pinia adapter、旧 compiled renderer、proto pages/modules/app、LegacySurface、旧 QA HTML/demo。共享的 DTO、校验、展示映射、持久命令和服务 facade 保留；不按目录名误删业务基础。

验证：生产构建同时运行 fixture 排除和 React/HTML/import guard；领域测试与 React DOM 行为测试并行保留。浏览器证据为本机隔离样本。用户已接回下一轮使用验收，不继续扩大测试范围。

回退：按本任务 diff，不能清除用户 storage、原请求身份或覆盖其他会话文件。未变更后端契约、数据库或模型提示词。
