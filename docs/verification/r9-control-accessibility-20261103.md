# R9：按钮焦点、图标尺寸与移动端点击区

## 逐项根因与修复

| 项目 | 根因 | 修复与主要文件 |
| --- | --- | --- |
| 首页「继续创建」「先连接模型」的 outline 链接没有可见焦点环 | `asChild` 渲染后的锚点保留了完整的 shadcn `focus-visible:ring-[3px]` 类；`workspace-replica.css` 中不属于 CSS layer 的 outline 装饰规则设置 `box-shadow:none`，优先于 Tailwind utilities 的 ring。 | 删除这条共享覆盖，所有工作区及相关弹层中的 outline Button 链接都恢复原生 shadcn 焦点环。文件：`apps/web/src/styles/workspace-replica.css`。 |
| 图标按钮尺寸不一致 | 页面 CSS 分别写死了铃铛 26×30、资源查询 28×32、卡片菜单 24×24、动态菜单 28×28、日记/记忆搜索 32×32 等尺寸；通知和诊断重读、本地备份删除漏配 `size="icon"`；Dialog/Sheet/SidebarTrigger 仍用 `icon-sm`；部分移动端最小高度及数字步进按钮的拉伸样式继续覆盖原语。 | 统一调用 `Button size="icon"`，共享 variant 保持 `size-9` 并显式零 padding；删除尺寸覆盖，文字按钮样式排除 icon；移动端快速前往复用 `useIsMobile(701)` 切换到 icon。完整文件见下文。 |
| 手机点击区不足 | 首页标签的视觉高度约 28.5px；Radio/Checkbox 的负 inset 伪元素没有明确保证足够的物理点击高度；通知重读又被移动端 28px 网格列和宽度限制。 | `react.css` 在 <=700px 为 icon、首页标签、RadioGroupItem、Checkbox 增加透明的 44×44px `::after` 点击区。通知工具栏列宽改为 36px。单选与复选框的视觉原语代码未改，仍保留原有 size 类和指示器；点击区没有通过放大圆点实现。 |

共享 icon 点击区的定位放在 components layer，保留调用者 `absolute` 工具类的优先级，避免增加 `.relative` 类后误中既有选择框布局规则。日历 `[data-day]` 格子保留原来的布局和日期标记。首页图标排除条件使用 `:where` 保持原选择器权重，标签外观仍为 28.5px 高。

## 文件

- 共享原语：`apps/web/src/components/ui/button.tsx`、`dialog.tsx`、`sheet.tsx`、`sidebar.tsx`。
- 共享业务控件：`apps/web/src/react-app/components.tsx`、`controls.tsx`、`shared/DiagnosticsWorkspace.jsx`、`shared/LocalBackupsPanel.tsx`、`shared/WorkspaceQuickJump.tsx`。
- 页面入口：`apps/web/src/react-app/pages/InboxPage.jsx`。
- 样式：`apps/web/src/styles/react.css`、`workspace-replica.css`、`r16-layout.css`、`replica-chat.css`、`replica-diagnostics.css`、`replica-feed.css`、`replica-life.css`、`replica-management.css`、`replica-media.css`、`replica-resources.css`、`replica-studio.css`。
- 新增 DOM/CSS 回归：`apps/web/test/r9-control-accessibility.test.mjs`。
- 新增生产浏览器回归：`apps/web/scripts/verify-r9-controls.mjs`。
- 约定：`.trellis/spec/frontend/component-guidelines.md`。

复用 shadcn/ui：Button（`asChild`、`size="icon"`）、RadioGroup/RadioGroupItem、Checkbox、Dialog、Sheet、Sidebar/SidebarTrigger、DropdownMenu；没有引入另一套按钮组件或依赖。

## 验证

Node 使用 `/tmp/node-v24.12.0-linux-x64/bin`；pnpm 使用现有 `/tmp/r8-corepack/v1/pnpm/11.0.7/bin/pnpm.cjs`。

- Web lint、typecheck：通过，日志 `/tmp/r9-shots/lint.log`、`typecheck.log`。
- Web production build：通过，包括 fixture 排除和 React-only 400 个本地模块入口检查。日志 `/tmp/r9-shots/build.log`；保留既有大 chunk 提示。
- 针对性 JS：17 个文件、180 个不同案例通过。三组日志为 `targeted-tests.log`（30）、`shared-control-tests.log`（78）、`native-control-tests.log`（79）；新增 R9 的 7 个案例在后两组重复，去重后 180。
- Chrome：当前本地生产 dist + 任务独占、仅绑定 loopback 的内存 HTTP fixture；桌面 1440×1000、手机 390×844。45 项检查通过，183 个可见图标按钮均为 36×36、四边 padding 为零，无页面 JS 异常、无横向溢出。
- 实测页面包括首页、创建向导、世界底稿、创作中心、通知、诊断、动态、媒体、此刻、日程、目标、衣柜、日记、记忆、运行世界、人物关系、角色档案、设置，以及快速前往 Dialog 和手机导航 Sheet。
- `git diff --check`：通过。

关键实测：

| 控件 | 可见尺寸 | 手机透明点击区 | 行为验证 |
| --- | --- | --- | --- |
| 首页「继续创建」「先连接模型」 | 保留原尺寸 | — | 真实 Tab 到达后 `:focus-visible=true`，计算样式含 3px ring；重新注入旧 `box-shadow:none` 后 ring 消失，移除后恢复。 |
| 查看通知、世界刷新/查询、通知重读 | 36×36px | 44×44px | 复用实际生产 Button，零 padding。 |
| 首页「全部」 | 46×28.5px | 44×44px | 可见盒外上方的坐标通过 `elementFromPoint` 命中原按钮。 |
| 向导单选圆点 | 未选中 16×16px，原 size/indicator 样式保留 | 44×44px | 在可见盒上方 6px 点击后选中；关闭透明伪元素时，同一选中状态的可见几何完全相同。 |
| 世界「已归档资源」 | 16×16px | 44×44px | 在可见盒上方 6px 点击后切换选中。 |

Dialog 几何测量等待有限的入场动画结束，保留严格的 36px 断言，避免在 Radix 初始 `scale(.95)` 帧采样。

原始数据：`/tmp/r9-shots/measurements.json`。截图全部位于 `/tmp/r9-shots`，没有加入仓库。

浏览器复现命令（需先完成 Web build；Playwright Core 使用已有独立安装）：

```bash
PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH \
PLAYWRIGHT_MODULE=/tmp/r8-browser/node_modules/playwright-core/index.mjs \
node apps/web/scripts/verify-r9-controls.mjs
```

## 断言审计

既有测试文件修改 **0**，既有断言变更 **0**，删除/跳过/放宽断言 **0**。审计数据：`/tmp/r9-shots/assertion-audit.json`。

新增 7 项 DOM/CSS 测试，理由分别是：

1. 验证 `asChild` 链接保留焦点类，并禁止 outline 装饰覆盖 ring。
2. 验证首页两个真实链接，以及视图切换和搜索清除图标采用共享 Button。
3. 验证通知/诊断重读的 icon size，且通知重读仍执行一次读取。
4. 验证世界刷新/查询的 icon size，归档 Checkbox 仍正确切换。
5. 验证透明 44px 点击区及 Radio/Checkbox 原视觉 size 类。
6. 验证手机快速前往采用 icon、桌面保留文字尺寸，Dialog 关闭按钮采用相同尺寸，导航功能有效。
7. 验证数字步进的 icon 不再拉伸，同时加减功能保持正确。

生产浏览器新增断言覆盖精确宽高、零 padding、透明区域、盒外命中与真实点击、Tab ring、无溢出和无页面异常；没有用容差替代 36px 的精确尺寸要求。

## 边界

没有访问 `/workspace/accept` 或验收服务，没有修改任何 `*.env`，没有 Git 写操作、暂存或提交，也没有修改或新增仓库图片。全部测试使用独占 DOM/内存夹具；未连接 PostgreSQL、未建表或清理数据库行。截图和测量资料均在 `/tmp/r9-shots`。
