# R10：真实部署页面、旧标签页与本地修复对照

## 结论与逐项根因

本次直接检查 `http://127.0.0.1:13001`。审计截图对应的现存浏览器标签页没有加载 d6843350 的入口资产：

| 来源 | JS 入口 | 主 CSS | CSS SHA-256 |
| --- | --- | --- | --- |
| 审计浏览器现存标签页 | `index-C6xLBlmw.js` | `index-CmVhmy0N.css` | `37361ff9fc550b3f53e9b6f8f4bb0309eb74c32ef5efe4ec96cdd95fad7f2835` |
| 13001 新浏览器 / 修改前本地生产构建 | `index-B6VE0C5M.js` | `index-bRyegPHY.css` | `d1a31add3998f800fd2dfa9a762889fe59d60d69b84d343de1a6a6843233e00f` |
| 本次本地修复构建 | 新构建入口 | `index-BWsIeYEn.css` | `85eea57cb08e07add7e41df7f7338959db0e204fba6fd9c80c50aec1a951c8f3` |

从现存标签页用 GET 读取的新 HTML 已引用 `index-B6VE0C5M.js` / `index-bRyegPHY.css`，HTML 响应为 `Cache-Control: no-cache`；旧 CSS URL 已是 404，但旧 CSSStyleSheet 仍驻留在该页面。结合现存 DOM 的旧 `data-size`，可确定这是 SPA 标签页继续执行旧入口的结果，不是当前服务器重新发出了旧 CSS。证据：`/tmp/r10-shots/asset-attribution.json`。

1. **首页 outline asChild 链接**：旧标签页真实 Tab 后，两个链接均为 `:focus-visible=true`，`--tw-ring-shadow` 已有 3px、ring color 也正确，但未分层的 outline 主题规则以 `box-shadow:none` 覆盖 utilities layer。当前部署已删除这一覆盖，两条真实链接均显示 3px ring。本次不重复修改已经修好的 Button / outline 主题。
2. **图标大小**：旧页面仍加载世界/创作中心的 28×32 搜索动作覆盖、媒体的 32×32 覆盖和移动页头的 44px 高度覆盖。当前部署的这些实际元素均为 `data-slot="button" data-size="icon"`、36×36、零 padding，桌面和手机都通过。旧通知重读更是 `data-size="default"`，并被 28px 列宽和 39px min-height 限制；当前已是 icon。复测没有将默认 Button 或其他页面同名控件代入。
3. **手机盒外命中**：旧页面首页/通知没有生成扩展伪元素，Radio/Checkbox 的原 inset 只生成 38×30 的伪元素，因此上下 10px 都不中。当前部署已为 Radio/Checkbox 提供 44×44，未选中视觉框仍为 16×16，上下 10px 实测已通过；但首页标签 28.5px 高时，44px 只上下扩展约 7.75px；36px 重读图标只扩展 4px，因此当前部署这两个控件在 10px 处仍失败。本次在 `react.css` 为首页筛选和通知重读增加 `max(44px, calc(100% + 24px))` 透明点击区，保留边框及边缘采样余量，不改变可见尺寸。

上一轮本地回归通过的原因分为两类：焦点与尺寸确实在新资产上通过，但审计浏览器仍驻留旧入口；命中回归只测了首页 4px、单选/归档 6px，伪元素 >=44px 的断言无法证明 10px 命中。此次还确认真实空目录的向导没有「日常生活世界」来源 tab，脚本会按实际 DOM 决定是否切 tab，而不沿用有数据夹具的固定假设。

## Live-vs-fixed 实测表

所有手机结果均为 390×844；桌面为 1440×1000。外侧坐标由对应控件的 `getBoundingClientRect()` 当场计算。焦点使用真实 Tab，不使用 `.focus()` 冒充键盘交互。

| 精确页面与元素 | 审计旧标签页 | 当前 13001 新浏览器 | 本地修复生产构建 |
| --- | --- | --- | --- |
| 首页「继续创建」「先连接模型」 | focus-visible=true；shadow=none | 两者均为 3px ring | 两者均为 3px ring |
| worlds「刷新创作资源」「查询」 | 28×32 | 36×36 | 36×36 |
| studio「刷新创作资源」「查询」 | 28×32 | 36×36 | 36×36 |
| media「查询媒体」 | 32×32 | 36×36 | 36×36 |
| 手机「查看通知」 | 36×44 | 36×36 | 36×36 |
| 手机首页「全部」 | 46×28.5；无 after；10px 四向不中 | 46×28.5；after 44×44；10px 四向不中 | 46×28.5；after 68×50.5；10px 四向命中并接收鼠标事件 |
| 手机「重新读取通知」 | 28×39；无 after；10px 四向不中 | 36×36；after 44×44；10px 四向不中 | 36×36；after 60×60；10px 四向命中并接收鼠标事件 |
| 手机向导「使用日常生活世界」「创建新的世界底稿」 | 未选中 16×16；after 38×30；上下 10px 不中 | 未选中 16×16；after 44×44；上下 10px 命中 | 同当前部署，真实鼠标事件也命中 |
| 手机 worlds/studio「已归档资源」 | 16×16；after 38×30；上下 10px 不中 | 16×16；after 44×44；上下 10px 命中 | 同当前部署，真实鼠标事件也命中 |

本地还在 10px 处实际切换了归档勾选、选择了日常世界，并确认通知重读触发原有 GET。选中态视觉使用与关闭 after 的**同一状态**严格比较：既有单选选中态为 18×16，关闭透明命中区后也是 18×16；未选中态仍严格为 16×16。没有把原有选中态尺寸变化归因于本次命中区修复。

原始表：`stale-measurements.json`、`live-measurements.json`、`fixed-measurements.json`，都在 `/tmp/r10-shots`。每份均记录精准选择器、属性、矩形、outline、box-shadow、ring 变量、伪元素、祖先几何/overflow/position/pointer-events，以及 10px 处实际命中的 DOM。

## CDP 中真正获胜的规则

脚本对每个精确元素调用 `CSS.getMatchedStylesForNode`，保存完整响应和按 layer / specificity / 顺序计算的审计属性胜出规则。完整文件为 `/tmp/r10-shots/{stale,live,fixed}-cdp-*.json`。

| 位置 | 旧资产中的获胜声明 | 当前资产 / 本次修复 |
| --- | --- | --- |
| outline 链接 shadow | `:where(.replica-workspace,… ) :where([data-slot="button"],…)[data-variant="outline"] { box-shadow:none }`，未分层，压过 ring utilities | `focus-visible:ring-[3px]` 生成的 shadow 生效；ring color 为 primary 50% |
| worlds/studio 查询动作 | `.replica-studio .r-search-actions button { width:28px;height:32px;min-height:32px }` | `.size-9` 宽高 36px；`.p-0` 零 padding |
| media 查询 | `.replica-media .media-toolbar .r-search > button { width:32px;height:32px }` | 仍保留该选择器的绝对定位；尺寸交回 `.size-9` |
| 手机页头通知 | `.replica-topbar-icon[data-slot="button"]` width 26px；移动联合规则 min-width 36px / height 44px，因此实际 36×44 | `.size-9` 宽高 36px |
| 手机通知重读 | `.replica-inbox-toolbar > button:last-child` width 28px；`.replica-inbox-toolbar button` min-height 39px | 当前 icon 为 36×36；本次 ` .replica-inbox-toolbar > button[data-size="icon"]::after` 扩展到 60×60 |
| 手机首页「全部」 | 无生成的 after | 原 44px 通用规则；本次更晚的 `.replica-library .tag-filter > button::after` 扩展到 68×50.5 |

祖先检查没有把「伪元素尺寸不足」误判成裁剪：单选卡片虽有 overflow:hidden，但上下 10px 坐标仍在卡片内；归档 label 与通知工具栏 overflow 为 visible。当前 Radio 根为 absolute，Checkbox 根为 relative，伪元素 absolute、pointer-events:auto。外层工作区有正常滚动/裁剪边界，但本次采样点均在可见窗内。通用图标定位位于 components layer，媒体查询自身的 absolute 仍获胜。

本次首轮本地复测还捕获了一次真实级联失败：仅写 `.replica-inbox-toolbar > [data-size="icon"]::after` 会被通用 `:is(button,a)[data-size="icon"]:not([data-day])::after` 的类型选择器权重压过，计算值仍为 44px。最终选择器包含实际 `button`，匹配权重后由后置声明获胜。未通过放宽断言解决。

## 修改文件与复用组件

- `apps/web/src/styles/react.css`：仅增加上述两个紧凑控件的透明命中区覆盖。
- `apps/web/scripts/verify-r10-controls.mjs`：同一套生产页面测量，支持 `live` / `stale` / `fixed`；固定精确尺寸、真实 Tab、CDP、10px 命中、鼠标派发、资产身份和异常检查。
- `apps/web/test/r10-control-evidence.test.mjs`：新增 5 项回归自检，确保审计会拒绝旧 shadow、旧几何、只有伪元素尺寸却实际不中、鼠标事件未命中，以及放大单选视觉框的替代方案。
- `.trellis/spec/frontend/component-guidelines.md`：记录旧 SPA 入口识别和 44px/10px 的区别。
- 本报告：`docs/verification/r10-live-controls-20261103.md`。

复用已有 shadcn/ui **Button（asChild / icon）、RadioGroup / RadioGroupItem、Checkbox**。没有增加另一套控件或依赖。

## 验证与复现

- Web lint、typecheck、production build 通过；构建的 fixture 排除与 React-only 图检查通过。保留既有 chunk 大小提示。
- 定向 Node JS 测试 **46/46**：R10 新增 5 项，及既有 R9 控件、R4 修复、studio、inbox 用例。
- 相同测量脚本：旧标签页、当前真实部署、本地生产构建各 **38 个控件样本**，覆盖六页、桌面和手机；修复版严格断言全部通过，无页面 JS 异常、无漏捕获 API、无写请求。
- 补充有数据的隔离 HTTP 夹具：保留 R9 浏览器脚本的全部原断言，仅将临时副本的导入/仓库/输出路径改为绝对路径及 `/tmp/r10-shots/populated`。**45 项检查、184 个图标样本**通过，包含媒体有角色、通知、studio、世界、运行世界及多个生活页面和弹层，无页面异常。仓库内 R9 脚本没有改动。
- `git diff --check` 通过。

日志在 `/tmp/r10-shots/{lint,typecheck,build,targeted-tests,live,fixed,populated}.log`。

```bash
export PATH=/tmp/node-v24.12.0-linux-x64/bin:$PATH
export PLAYWRIGHT_MODULE=/tmp/r8-browser/node_modules/playwright-core/index.mjs
node apps/web/scripts/verify-r10-controls.mjs live
node /tmp/r8-corepack/v1/pnpm/11.0.7/bin/pnpm.cjs --filter @fluctlight/web build
node apps/web/scripts/verify-r10-controls.mjs fixed
# 可选：已有审计标签页的 CSS 保留时，读取它的 CDP 现场。
node apps/web/scripts/verify-r10-controls.mjs stale
```

`live` 仅允许 GET/HEAD 和一次登录 POST；密码只填入登录框，不输出、不保存。新浏览器将创建流程需要的 localStorage 写入留在文档内存 overlay，支持瞬时「继续创建」DOM，不写入浏览器持久存储。`fixed` 仅绑定随机 loopback 端口，提供本地 dist 并重放读取到的 API JSON；没有代理到真实部署，不连接 PostgreSQL，不需要建库、删行或清理任何数据库数据。真实部署的目录为空，重放精确保留这一状态；有数据覆盖由独占内存夹具补充。

## 断言审计与边界

既有测试文件和既有断言修改 **0**；删除、跳过、放宽断言 **0**。新增 5 项 JS 回归及生产浏览器断言。要求仍为精确 36×36、零 padding、未选中单选/复选 16×16、选中态同基线严格相等、真实 Tab 3px ring、实际 10px 命中；没有改为尺寸容差或只看类名。

没有修改 `/workspace/accept`、任何 `*.env`、Docker 或 Git 元数据；没有暂存、提交或部署。图片只保存在 `/tmp/r10-shots`，仓库仅新增文字/脚本。修复构建尚未部署；旧 SPA 标签页需要重新加载才能采用服务器当前入口，本次本地新增的 10px 命中修复需要随后续正常发布生效。
