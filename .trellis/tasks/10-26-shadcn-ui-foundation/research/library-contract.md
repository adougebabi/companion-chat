# 官方库契约核对（2026-10-26）

使用 find-docs / ctx7，在沙盒外按 library → docs 查询；无凭据进入查询。

## 用户决策

严格 React shadcn/ui + Tailwind CSS + Radix UI + Lucide；用户明确拒绝以 Vue 对应实现作为本次目标。shadcn-vue 是 Vue 移植，Reka 是 Vue primitive，均不能替代 React/Radix 的验收。

## shadcn/ui

Context7 解析 ID：/shadcn-ui/ui。

- https://github.com/shadcn-ui/ui/blob/main/apps/v4/content/docs/changelog/2026-07-base-ui-default.mdx
  最新默认基座已经变为 Base UI；Radix 必须显式 `shadcn init -b radix`。不要仅运行默认 init。
- https://github.com/shadcn-ui/ui/blob/main/packages/registry/src/preset/preset.ts
  当前 preset 支持 radix/base/aria 前缀；组件配置应选择 radix-*，例如 radix-nova。最终以安装时 CLI 生成的有效 schema 为准，锁定版本与生成结果。
- https://github.com/shadcn-ui/ui/blob/main/apps/v4/content/docs/installation/vite.mdx
  Vite React 为支持路径；在现有 workspace 中配置，不另建脱离正式入口的演示项目。
- https://github.com/shadcn-ui/ui/blob/main/apps/v4/content/docs/(root)/monorepo.mdx
  每个拥有组件的 workspace 保持 components.json；别名对齐；Tailwind v4 的 tailwind.config 留空。
- https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/bases/radix/registry.ts
  Radix registry 共用 lucide-react、radix-ui、class-variance-authority 等依赖；样式含 tw-animate-css 和 shadcn/tailwind.css。现有 cn(clsx, tailwind-merge) 可统一到本地 utils；不要混用多份 helper。
- https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/bases/radix/ui/button.tsx
  Button 输出原生 button 或 asChild 时的 Slot.Root；data-slot=button。原生元素本身不是“手搓”的证据。
- https://github.com/shadcn-ui/ui/blob/main/skills/shadcn/rules/base-vs-radix.md
  Radix 用 asChild 组合 Button/anchor/DialogTrigger，避免嵌套 button 和多余 DOM。
- https://github.com/shadcn-ui/ui/blob/main/apps/v4/registry/new-york-v4/ui/dialog.tsx
  Dialog 组合 Radix Root/Trigger/Portal/Overlay/Content/Title/Description/Close，使用 lucide-react；不要只复制外观、再自己实现焦点管理。

## 实施时核对

锁文件记录确切 React、react-dom、shadcn、Radix、lucide-react、Vite React plugin 版本。不要在计划里猜版本；不需要无关升级 Tailwind/Vite。组件由官方 Radix registry 引入后按本项目视觉 token 调整，保留组成与无障碍行为。

普通 Input/Textarea、原生 date/time/file 输入仍使用相应原生语义，包装为统一组件；不为了“全 Radix”伪造不存在的 primitive。侧栏抽屉用 Radix Dialog-backed Sheet；不要为本轮额外引入 Vaul Drawer。

## React external store

Context7 ID /reactjs/react.dev，官方来源 https://react.dev/reference/react/useSyncExternalStore 。subscribe(callback) 必须返回 unsubscribe；未变化时 getSnapshot 返回同一值，变化时返回 immutable snapshot；不得每次读取创建新对象，否则产生无限 render。subscribe 函数保持稳定，SSR 测试需要一致 getServerSnapshot。该机制用于框架中立 controller，不是 Vue proxy 的永久桥接。
