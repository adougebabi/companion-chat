# 前端 UI 质量门禁

真实 Chromium（Playwright）中的布局、交互与视觉检查。jsdom 断言和 CSS 文本检查
不能替代它；只有门禁结果为 `pass`（或 `not-applicable`）时才能报告 UI 验收通过。

设计规范与共享组件清单见 [`.impeccable.md`](../.impeccable.md)。

## 组成

| 部分 | 位置 |
| --- | --- |
| 共享组件 | `apps/web/src/components/patterns/`、`responsive-sheet.tsx`、`responsive-tabs.tsx` |
| UI Lab（仅开发与测试，固定 fixture） | `apps/web/ui-lab/`，`/ui-lab/?fixture=gallery`、`?fixture=schedule-settings` |
| 布局检查器 | `apps/web/e2e/support/layout.ts` |
| 用例 | `e2e/schedule-settings.spec.ts`（@schedule）、`e2e/gallery.spec.ts`（@patterns）、`e2e/visual.spec.ts`（@visual）、`e2e/layout-checker.self-test.spec.ts`（@gate） |
| 改动影响映射 | `e2e/impact-map.json` |
| 门禁 | `apps/web/scripts/ui-gate.mjs` |
| 视觉基准管理 | `apps/web/scripts/ui-baselines.mjs`，基准位于仓库根 `.ui-baselines/<环境>/`（不提交） |
| CI | `.github/workflows/fluctlight-ui-quality.yml` |
| 静态归属检查 | `apps/web/test/overlay-geometry-ownership.test.mjs`（jsdom 套件内） |

三个视口：390×844（手机）、768×1024（平板）、1440×900（桌面）。时钟固定为
2026-11-16 10:30（北京时间），数据全部来自 fixture，不访问网络。

## 检查内容

1. 页面与浮层超出视口、页面横向滚动（`page-overflow`、`container-outside-viewport`）。
2. 容器内非预期裁切与无法触达的控件（`control-clipped`、`control-outside-container`）。
3. 文字溢出控件（`content-overflow`，原日程抽屉错位即此类）；手机端触控高度 < 40px（`touch-target`）。
4. 标签、工具栏、按钮在窄屏下全部可达；激活标签完整可见。
5. 表单滚动、焦点（错误获得焦点、关闭后焦点归还）、浮层关闭（Esc / 关闭按钮 / 忙碌时不可关闭）。
6. 关键业务状态截图与经过审核的共享组件视觉基准（`toHaveScreenshot`）。

`layout-checker.self-test.spec.ts` 证明检查器会对已知坏布局（含重新注入旧 `75vw`
样式）报错；它失败说明门禁失效。

## 本地使用

```sh
pnpm --filter @fluctlight/web test:ui:install      # 首次：安装 Chromium
pnpm --filter @fluctlight/web ui-lab                # 浏览 UI Lab
pnpm --filter @fluctlight/web ui:gate               # 按改动范围检查（相对默认分支）
pnpm --filter @fluctlight/web ui:gate -- --files src/v3-domain/life-schedule.js
pnpm --filter @fluctlight/web ui:gate:full          # 发布前 / 全站
pnpm --filter @fluctlight/web ui:baselines status   # 基准审核状态
```

本机因文件系统不允许 `chown` 时 pnpm 的依赖检查会失败，可直接
`node apps/web/scripts/ui-gate.mjs …`。

## 范围策略

- **普通业务改动**：`impact-map.json` 中的区域标签对应的用例（含该区域的视觉基准）+ `@gate`。
- **共享组件或全局 CSS 改动**（`shared`）：全部用例与全部视觉基准。
- **完整全站检查**：`--full`，CI 每晚 02:30（北京时间）及手动 `mode=full`（发布前）。
- **没有浏览器覆盖的 UI 文件**：结果为 `unverified` 并列出文件，需要补 UI Lab fixture / 用例，
  或在映射中如实归类；不会被静默判为通过。
- **浏览器不可用 / Playwright 未产生报告**：`unverified`。

| 结果 | 含义 | 退出码 | 可否报告 UI 验收通过 |
| --- | --- | --- | --- |
| `pass` | 所选检查全部执行并通过 | 0 | 可以 |
| `not-applicable` | 改动不涉及需要浏览器检查的文件 | 0 | 可以（注明无 UI 改动） |
| `fail` | 布局、交互或视觉检查失败 | 1 | 不可以 |
| `unverified` | 浏览器不可用、缺少/不一致的已审核基准、改动无覆盖 | 2 | 不可以 |

结果写入 `apps/web/test-results/ui-gate.json`；CI 同时写入作业摘要并上传
`test-results/`（截图、差异图、trace）作为产物。

## 视觉基准

- 基准图片**不提交 Git**，位于仓库根 `.ui-baselines/<环境>/`（已在 `.gitignore`）。
  渲染与字体随机器不同，因此按环境分目录：`local-linux`、`ci-linux` 等。
- 每个环境目录有 `manifest.json`：审核人、原因、提交号、每张图的 sha256。比对前校验，
  文件被流程外修改即为 `unverified`。
- **只有经过设计变更审核才能更新**：
  - 本地：`pnpm --filter @fluctlight/web ui:baselines approve --reason "<设计变更与审核说明>"`。
  - CI：手动运行 *Fluctlight UI Quality*，`mode=approve-baselines` 并填写 `reason`；该作业绑定
    GitHub environment `ui-baseline-approval`（需在仓库设置中配置必需审核人）。产物
    `ui-baselines-ci-linux` 保留 90 天；把作业摘要中的 JSON 提交到
    `apps/web/e2e/visual-baselines.lock.json`，评审该提交即评审基准变更。
- 直接 `playwright test -u` 会被配置拒绝；`updateSnapshots` 默认为 `none`，缺少基准不会自动录制。
- 测试失败时不得重录基准使其通过。差异来自有意的设计变更时，先完成设计审核，再走上面的审核流程。
- CI 产物过期或锁文件与产物不一致时，视觉比对为 `unverified`，需重新审核。

## CI 与必需检查

`Fluctlight UI Quality` 是独立 workflow，不影响 `fluctlight-ci.yml` 的镜像构建与发布。
目前**不是必需检查**。满足以下条件后，再在分支保护中把 `ui-gate` 设为必需：

1. 已完成一次 CI 基准审核（锁文件非空）。
2. 连续两周夜间全量运行无不稳定（flaky）结果。
3. 期间所有失败都对应真实问题。

## 开发流程

见 `AGENTS.md`「前端 UI 验收」。

## 新版手机工作区（2026-11-26）

设计裁定记录在 `.impeccable.md`。手机底栏、全屏详情、底部筛选和紧凑弹窗
在 UI Lab 的 `mobile-navigation`、`detail-sheet`、`bottom-sheet`、`compact-dialog`
中覆盖适用状态。移动页面回归位于 `e2e/mobile-workspace.spec.ts`（`@mobile`），
使用正式 React 路由、生产 controller 与隔离 HTTP fixture；不是 Demo 的另一套 renderer。

本地查看全部业务页的 mock 工作区：

```sh
pnpm --filter @fluctlight/web dev:mobile-mock
# http://127.0.0.1:5197/#/library
```

该命令仅在 loopback 启动 Vite（默认5197）与 HTTP fixture（默认5196），不代理到真实
Core、模型或图片服务，不持久化生产数据。`UI_MOBILE_PORT` 与
`UI_MOBILE_FIXTURE_PORT` 可指定两个不同端口。原 `dev:mock` 保留 API scenarios
入口，侧重诊断与恢复场景。浏览器回归禁止外部网络请求；未支持的接口不能视为通过。

手机验收除390×844外，关键交互检查360×740、430×932，以及639/640/1023/1024
断点；768×1024与1440×900检查平板和桌面兼容。模拟可视视口变化用于验证布局机制，
不能代替iOS/Android真机的软键盘、安全区或地址栏伸缩验收。

### 聊天浮层的生产构建回归

`chat-mobile-overlays.spec.ts` 在隔离 HTTP fixture 提供的 **dist 产物**上检查手机会话菜单、
菜单到历史的切换、历史全屏几何、长内容滚动、固定底部操作、关闭焦点与输入草稿。
`e2e/support/build-product.mjs` 在每轮浏览器检查前构建一次生产产物，避免仅用 Vite
开发模式漏掉 CSS 优化产生的定位差异。开发模式通过不能替代该检查。
