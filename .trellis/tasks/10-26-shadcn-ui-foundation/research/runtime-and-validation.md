# React 迁移接缝与验证证据

## 运行时

- ApprovedApp.mjs:29-35 创建 workspace 并 Vue provide；renderers/App.mjs:169-186 集中分发 9 个页面控制器。
- 页面 keys 为聊天 fid、衣柜/世界/profile 前缀+fid、life fid、browse page+fid、library/creation 固定值。迁移要显式保留 reset/reuse 边界。
- 9 个 page controller 共 3,775 行；12 个 renderer 共 2,255 行，合计约 6,030 行。另有 App 240 行、workspace 1,006 行、共享组件与会话 store；这些是范围估计，不是工时承诺。
- renderer 使用 Vue createVNode/vModel/compiler cache；shared/render-runtime.mjs:1-22 依赖 setBlockTracking。应重新建立 JSX 源码，不把 Vue compiler body 当 React 代码。
- workspace.mjs:1,173-203 依赖 reactive/computed/ref/inject；`:958-1004` 提供 state/current/client/conversation/bootstrap/load/command/select/services/dispose façade。
- workspace.mjs:260-281 用 Owner/fid/generation/epoch 防迟到读取；`:632-707` 保留冻结 command/body/key 和深层 reactive operation；`:995-1003` 清 observer、timer、visibility listener。React 通知必须覆盖深层 operation 变化。
- v3/proto/store.js:7-23 是 plain snapshot + Set subscribers；services.js:4-39 是明确方法注册表，可保留。
- v3/providers/real.ts:2-3,36 依赖 Pinia conversation；stores/conversations.ts:300-344 起包含 stream、retry、cancel、队列和 request epoch。仅改根挂载不能消除 Vue 状态耦合。
- ChatPage.mjs:40-44,91-104 按 Owner/fid 保存草稿，受理后只清对应快照；`:87-151` 控制订阅、poll、unmount/invalidate。
- LibraryPage.vue:9-66 是最小完整真实只读切片，可作首个集成检查点，但不能作为全部基础控件及写命令迁移完成的证据。

## 设计限制

不能为每个 Button/Input 创建 React root：Vue children/context 不能直接跨 root；额外 host 会影响 form/flex/grid；Radix compound component 需要同一 context；逐 root 订阅/卸载容易泄漏。使用完整 React 页面与根树，legacy 仅在明确路由出口集中挂载。

旧样式含 data-v-* scoped selector，React 不会生成这些属性；迁移时重建语义类/utility，不能静默丢掉样式，也不能为伪装 Vue 长期复制 scope hash。approved-ui 与 globals 的 --secondary 等变量名语义冲突，需要作用域和 token 映射。

## 可保留测试

- v3-persistent-command.test.mjs:6-42：丢响应/重载同 body+key、存储失败、401、CAS、logout 迟到回调，框架中立。
- approved-ui.test.mjs:62,104,188：configuration/staged-command/routing 的纯契约。
- runtime-config.test.mjs:6-13：运行时 API origin；verify-product-build.mjs:7 起：生产 fixture 排除。
- Provider/BrowserClient 的 Vite ssrLoadModule 不自动等于 Vue 耦合，按实际 imports 判断。

## 必须改造的覆盖

- 82 个 test 文件中 25 个直接出现 Vue/Pinia/.vue/SSR/compiler；其余也不能自动归为框架无关。
- approved-ui.test.mjs:212,230 的按钮 SSR/custom renderer；ui-contracts.test.mjs:74-113 的 Vue SSR。
- ui-interactions.test.mjs:10-19 截取 SFC script 注入 Vue；它覆盖异步导入、保存后新草稿、角色隔离、失败 dialog 等有价值行为，不可直接删除。
- layout.test.mjs:64-67 强制 shadcn-vue/Button.vue；skeleton/accessibility 只扫描 .vue，必须更新扩展名防空扫描假绿。
- v3-real-provider.test.mjs:24-33 的 innerHTML/form.value 负向源码断言不足以证明 React composer 保持 focus/selection/IME。

## 验证命令和证据边界

1. pnpm --filter @fluctlight/web typecheck
2. pnpm --filter @fluctlight/web test
3. pnpm --filter @fluctlight/web build
4. node apps/web/test/fixtures/ui-quality-server.mjs（构建后启动；默认 http://127.0.0.1:4175）
5. pnpm regression:v3:self-test（框架中立工具自检；不是浏览器）

lint 当前等于 vue-tsc，React 后改为实际 TSX 校验，遗留 Vue 如仍保留需独立校验。

ui-quality-server.mjs:1-17 服务真实 dist，只绑定 loopback，支持 NAS_ADDITIONS/PENDING_CHAT/FAILURES/WARDROBE_WORLD；`:76-88` 分两段 NDJSON；`:53-56` 与 /fixture/status 记录 request telemetry；`:119` 未实现 API 明确 404 fixture_route_missing；`:128` 写 dist/runtime-config.js。它能证明交互/请求形状，不能证明真实模型、持久数据库或 Temporal 执行成功。

v3-contract-server.mjs 默认 4180，可用 cookie v3loggedout=1 测匿名；多数写请求仅静态受理，不能验证完整写回。

现有分配范围内未发现自动 Playwright/Puppeteer/WebDriver 浏览器驱动。实施时通过现有浏览器工具完成实际静态构建检查，或添加合适的交互回归设施；不能拿 SSR 代替 Radix 焦点、键盘、portal、外点关闭验证。

必验：disabled/submit、label/error 关联、Select 键盘和空值、Switch/Checkbox、Tabs、Dialog/Sheet Escape/focus return、表单原生 validation、IME、流式刷新时 draft/selection 保持、角色切换、hash 深链/刷新/返回、1440 与 390px 布局、无重复订阅/命令。

真实 API/Worker/PG/Redis/Temporal/模型验证需要隔离环境；不向 NAS 真实账号生成无意义写入，不自动部署。当前仅盘点，未执行上述测试。
