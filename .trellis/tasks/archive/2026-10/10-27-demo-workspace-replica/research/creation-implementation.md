# 第 15 组初始化实施与验收（2026-10-28）

本记录收口 progress.md 最后一段初始化 WIP；不修改任务勾选、提交或其他组。生产界面通过真实 dist + loopback 隔离 API 验证，Demo 为原 4178 服务。浏览器全部使用 CUA。

## 本轮修改

- `state/creation-controller.mjs`：首次明确 400/422 拒绝和明确分析版本拒绝后重新整理前，保留完整预览副本；移除 analysis_id/correlation_id，清除旧分析授权。原描述、各来源输入及新副本独立保留。
- `react-app/views/CharacterWizard.jsx`：副本可编辑、刷新保留、导出原文本，明确采用后成为 JSON 输入，必须重新整理和审阅。导出文件实测成功，证据 `exported-rejected-preview.json`。
- `approved-ui/creation-progress.mjs`（主线程增补授权）：worldConfirmed 必须为非空字符串与实际 `/world` 当前 ID 相符；ready/retryable 共用判定。两个 undefined/null/空白 ID 不能完成。旧快照缺本地 worldSelectionConfirmed 但实际读回匹配时仍可完成。
- `styles/replica-creation.css`：补偿正式滚动容器底部 padding，保持 Demo 操作栏桌面距视口底部 15px、手机 8px。
- `test/demo-replica-creation.test.mjs`：补 400/422 完整 JSON 编辑保留/无旧授权重新分析、缺失世界身份与旧本地检查点兼容回归。原 13 项 + 本轮 2 项 = 15 项；对应 UI 专项 3 项，总专项 18 项。
- `test/fixtures/creation-fixture.mjs`：增加 analysis-cancel 15 秒首次响应和 activation-rejected 首次 422；无产品模拟。`ui-quality-server.mjs` 增加显式 CREATION_AUDIT 文件输出。

## 已实际执行

- 4354 原 2.2 秒分析加载已观察；取消使用专门 4356 场景。加载时禁用输入，停止等待保留描述；15 秒迟到结果不推进步骤、不产生激活。`analysis-{loading,cancelled}-desktop.png`、`analysis-late-result-ignored.png`、`cancel-request-audit.json`。
- 4355 首次分析 503 后原描述保留，再次整理进入预览。`analysis-failure-desktop.png`、`analysis-retried-preview-desktop.png`。
- 4357 四来源切换、描述草稿恢复、精确发布模板选择，五主题、结构弹窗编辑取消及焦点返回、完整 JSON 明确应用；422 后副本刷新/导出/转为 JSON 输入并重新整理，随后成功创建、7/7、刷新读回、原资料弹窗及新建确认取消。`source-*`、`preview-*`、`structured-edit-*`、`snapshot-mobile.png`、`new-creation-confirm-mobile.png`、`complete-*`。
- `rejected-and-reanalyzed-request-audit.json`：首次模板分析仅 resourceId=creation-template/resourceVersion=published:template:1；第二次 JSON 分析无旧 analysis_id/correlation_id，仍含编辑过的背景和 false 扩展。API 分析回包为正式权威，样本回包本身只回显 core_persona；不把请求字段保留误称模型必然原样返回。
- 4359 衣柜恢复首次回执 503，而实际阶段已 ready。原恢复入口仍可见；刷新后核对原提交，重试 body/key 完全相同，只有 1 次激活。`recovery-unknown-*-mobile.png`、`recovery-unknown-request-audit.json`。
- 世界来源 revision3 首次冲突，经实际读取 revision4、明确确认后复制与选用独立世界。发现并修复双方缺失 ID 假 ready；最终 4360 复验冲突时 6/7，实际绑定读回后 7/7，仍只有 1 次激活。`world-source-{conflict,review,recovered}-mobile.png`、`world-conflict-request-audit.json`。
- 既有成功/空世界/空模板/衣柜失败恢复/未知激活五态证据保留；旧 `*-wip` 不是最终截图，旧 4349 的路由夹具问题不作为产品缺陷。

## 同视口对照

`geometry.json` 为实际 DOM 测量。所有最终命名截图已保存至 `evidence/creation/`；Demo 对照以 `demo-` 开头。

- 1348×925：正式与 Demo 步骤条 x252/y212.828/1047×76.5；布局 x252/y319.328/宽1047；主栏774、gap25、摘要248；操作栏 x252/y830/1047×80。均实际相同。系统 15px 滚动条使可用宽为1047，不能写成1062。
- 390×844：正式及 Demo 来源卡两列网格宽341、高236；主栏 x17/宽341；摘要隐藏。正式来源页布局 y348.641，Demo y340.641，8px来自正式返回按钮保留44px触控尺寸。手机操作栏正式高74、Demo72，底边都在836（距视口8px），2px来自正式主按钮44px。
- 手机结构弹窗宽356，界面可见关闭/取消/应用，Escape和关闭返回来源；长原请求资料只滚动弹窗内容。正式 document.scrollWidth=390，无横向溢出。
- 正式预览及准备页含真实字段/版本/来源/技术详情，内容自然高度不同；没有复制 Demo 示例字段或计时成功。装饰插画不作为实际形象。
- 临时 viewport 已 reset，当前两个实现者标签已 markHandoff；主线程原标签没有被触碰。

## 检查和边界

- 最终 Web **609/609**：`/private/tmp/demo-replica-creation-tests.log`。
- 类型检查通过：`/private/tmp/demo-replica-creation-typecheck.log`。
- 最终生产构建通过（371 local modules），fixture exclusion 与 React-only guards通过：`/private/tmp/demo-replica-creation-build.log`。Vite仍有既有大chunk提示。
- `git diff --check` 通过。最新生产页面控制台无新增错误；分析503及激活422为明确注入场景。
- 全部为隔离 API/fixture，不能替代 live Core、数据库、worker、模型、ComfyUI、存储验收。无后端/API/prompt/token策略修改；无提交/推送/任务勾选。

当前实现者服务：4356 cancellation session76072，4357 rejected session52840，4358旧world-conflict session46395，4359 recovery-unknown session86914，4360修复后world-conflict session90000。旧服务保留，未因观察超时重启。
