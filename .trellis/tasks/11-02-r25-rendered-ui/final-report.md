# R25 UI 实际渲染验证报告

基线提交：`09a1b46f`。本轮为其上独立的未提交改动；未 git commit、未 git add，未触碰 `/workspace/accept` 或 `*.env`，未修改已有图片。截图与下载的部署静态资源只放 `/tmp`。参考 `/workspace/ux/demo/fluctlight-demo.html` 的 Sage 布局、叶子空态及扁平 Switch 行；创建文案按本轮明确要求使用中文。

## 实际入口与逐项结论

统一入口：`apps/web/index.html` → `src/main.tsx` → `react-app/App.tsx`。

| 项目 | 真实 DOM / 文件 | 根因及本轮处理 | 新增生产浏览器回归 | shadcn 组件 |
| --- | --- | --- | --- | --- |
| P1-4 | `shared/DiagnosticsWorkspace.jsx` 的 `.replica-task-row h3/p`；`shared/DiagnosticStatus.tsx`；`styles/replica-diagnostics.css` | 最新提交已按标题分段去重 metadata，并明确徽标大小。修改前即通过；不重复改写正确代码。真实请求形状使用 `intent_type: wake_up.current`、`title: 日常唤醒`。 | 标题只有“日常唤醒”，副行保留角色与 16:41，不重复标题；徽标实测 12px、28px。 | Badge、Tabs、Card、Input |
| P2-1 | `pages/ManagementPage.jsx` → `views/ManagementPage.jsx` 的 `#quiet-enabled` 与 `.replica-quiet-toggle`；`styles/replica-management.css` | 实际已是 Field + FieldLabel + Switch，不是截图中的旧输入框；仍有 1px 底边。本轮显式清除全部边框、圆角、背景与阴影。 | 四边均 0px，标签与行左缘差 0px，Switch 与右缘差 0px；桌面与右列 Select 底部差 0px，窄屏正确堆叠；开关可切换并恢复。 | Field、FieldLabel、Switch、Select |
| P2-2 | `shared/WorkspaceNavigation.tsx` 的 `.replica-character-switch` 和 Radix portaled `[role=menu]` | 最新提交已经使用 `align=start` 与 trigger-width；修改前即通过。不存在另一条仍在用的角色弹层入口。 | 桌面 trigger/menu 同为 x=13、w=213；窄屏同为 x=13、w=170；键盘展开/Escape；实际切换角色并切回，覆盖窄屏导航自动关闭后重开。 | DropdownMenu、DropdownMenuTrigger、DropdownMenuContent、DropdownMenuItem、Button、Avatar |
| P2-3 | `pages/DiaryPage.jsx` 的 `.lp-diary-index .lp-empty-inline` 与 `[data-slot=empty]` | 最新提交已使目录内“没有找到这段回忆”仅在仍有选中正文时出现，空本仅显示一个主空态；修改前即通过。 | 空本和输入无结果搜索时均恰好一个 Empty、一枚 Leaf，不同时显示两段空态；保留已有选中正文/搜索行为旧测试。 | Empty、EmptyMedia、Tabs、Switch、Input |
| P2-4 | `pages/InboxPage.jsx` 的 `.replica-notice-panel > .replica-page-empty`；`shared/Presentation.tsx`；`styles/replica-life.css`、`replica-management.css` | 最新提交已选择 page Empty 并移除承载 Card 的框；修改前即通过。 | 叶子存在；Empty 和承载 Card 的 computed border 均为 0；未读空态的“查看全部通知”能切回全部页签。 | Empty、EmptyMedia、Card、Tabs、Button |
| P2-5 | `pages/CharacterWizardPage.jsx` → `views/CharacterWizard.jsx` 的 WorkspacePageHeader、`.creation-aside .replica-eyebrow` 与准备区 eyebrow | 上轮只翻译页头，测试只停在 world 步骤，漏掉共享侧栏。本轮 YOUR NEW CHAPTER → 新的篇章，PREPARING A LIFE → 生活正在展开；页头原有“新的相识”保留。 | 真实 world 页面继续到 source 后检查页头、侧栏与英文缺席；桌面/窄屏均走同一流程。窄屏侧栏按原设计隐藏，校验其 DOM 文案且截图检查可见步骤。 | Card、RadioGroup、RadioGroupItem、Button、Input |
| P2-8 | `/data` → `pages/DomainPage.tsx` → `v3-domain/data-tools.js` → `shared/DomainPrimitives.tsx` FieldPort → `controls.tsx` FieldInput → `shared/FilePicker.tsx` | 上轮已传 framed=false，仅消除 wrapper 虚线，但 FilePicker Button 仍固定 outline，叠加上传 label 与 Card 仍显多层框。本轮仅无框模式改用 ghost Button；默认带框模式继续 outline。 | 统计上传区全部可见后代 computed borders，只保留 label 一层虚线；实际文件名显示、JSON 草稿传递；不自动校验或导入；默认 framed=true 和 disabled drop 另有 DOM 回归。外围 Card 保留。 | Card、Button（ghost/outline）、Textarea；保留原生隐藏 file input 语义 |

表内未标为本轮处理的源码文件仅追踪、核查，没有为制造差异而修改。

## 为何 R3 与上一轮结果不同

`baseline-results.json` 是本轮代码构建前的当前 HEAD 产物：七项 × 两种宽度中，P1-4、P2-2、P2-3、P2-4 已通过；安静时段残余底边、侧栏英文和导入按钮边框失败。修复后 14/14 通过。

只读 GET `http://127.0.0.1:13001/` 及其引用的 JS/CSS，记录在 `deployment-comparison.json`。当时部署入口为 `/assets/index-DHNTDt5M.js`，静态资源已有中文创建页头、Field 安静开关、日记目录去重与通知空态去框，但仍有 YOUR NEW CHAPTER。这说明 R3 PNG 与当前可取得的产物不完全一致。无法从 PNG 还原截图当时究竟是旧部署、旧 HTML/chunk 还是标签页缓存；没有证据支持“上一轮改了未使用组件”的统一解释。本轮不改部署/缓存设置，也未登录或调用 13001 业务 API。

## 测试与截图

全部命令使用 `/tmp/node-v24.12.0-linux-x64/bin/node`（v24.12.0）。系统 pnpm 的旧 Corepack 启动器不可直接运行，使用其下载到 `/tmp/r25-corepack/pnpm/11.0.7/bin/pnpm.cjs` 的真实 pnpm 11 CLI；未改变仓库依赖。

- 针对性测试：`node --test apps/web/test/r25-rendered-ui.test.mjs apps/web/test/r23-ui.test.mjs apps/web/test/demo-replica-diary.test.mjs apps/web/test/demo-replica-management.test.mjs apps/web/test/r11-domain-primitives.test.mjs`，55/55 通过。
- `pnpm --filter @fluctlight/web typecheck`、`lint`：均通过（仓库 lint 脚本为 tsc --noEmit）。
- `pnpm --filter @fluctlight/web build`：通过；production fixture 排除与 React-only graph 校验均通过。仅保留构建原有大 chunk 提示。
- `node .trellis/tasks/11-02-r25-rendered-ui/verify-browser.mjs`：使用真实 production dist 与独立本地 HTTP fixture，无头 Google Chrome，1920×1113 和 390×844，14/14 通过，零 pageerror、无页面水平溢出。其业务请求只到独立 fixture，不到 13001。
- 截图：`/tmp/r25-shots/desktop-*.png`、`mobile-*.png`；对照为 `before-*.png`。另有 quiet-hours-detail 与 data-import-detail 局部图。文字结果为本目录的 `baseline-results.json`、`browser-results.json`。

## 改动的旧断言（共 1 条）

`apps/web/test/r23-ui.test.mjs` 的 P2-8：`assert.equal(picker.querySelector('button').dataset.variant, 'outline')` → 精确等于 `'ghost'`。

原因：无框模式的 outline 本身就是残留嵌套边框之一；新的精确样式契约要求 ghost。没有删除或放宽断言；原有 wrapper 无 border、文件回调名称、文件名显示断言全部保留。新增真实 /data 组合测试与 Chrome computed border 计数，同时新增默认 framed=true 仍等于 outline 的隔离断言。其余旧断言未修改。
