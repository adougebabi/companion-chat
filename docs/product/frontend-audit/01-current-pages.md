# 01 — 当前页面与导航清单

审计对象是当前 `apps/web` 浏览器应用；页面路由、section、菜单和弹窗按可运行代码记录。代码证据路径均相对仓库根目录。
> 本文提及的 `App.vue`、`InstancesView.vue` 等前端文件名简写均相对 `apps/web/src/`；`stores/`、`components/`、`views/` 为该目录下子目录。


## 应用组织

- 前端为 Vue 3 + TypeScript，入口为 `apps/web/src/main.ts:1-9`；Pinia 在入口注册。Vite 配置见 `apps/web/vite.config.ts:13-25`，依赖见 `apps/web/package.json`。
- **未注册 Vue Router。** `apps/web/src/App.vue:20-25` 以 pathname 前缀选择五类 view；`App.vue:62-83` 手动 `pushState`、`popstate` 更新页面。未命中路径会落到实例页，没有专用 404 页面。
- `SettingsView` 与 `DiagnosticsView` 使用异步组件加载（`App.vue:15-16`）；这不是动态路由注册。
- URL 可恢复的状态只有 view、settings/diagnostics section 和 diagnostics `correlation_id`。`WorkspaceRoute` 类型另声明 instance ID、moments scope、instances mode（`app/navigation.ts:28-33`），但 App 的 URL 解析没有消费这些字段。
- 主导航为“聊天”（实例列表）、“动态”、“设置”、“诊断中心”（`app/navigation.ts:35-40`）。桌面上下文面板另有最近对话、搜索、分组和创建入口（`components/layout/DesktopContextPanel.vue:31-38,78-88`）；搜索与排序基于已加载的本地实例数组。
- 已认证后由 `AppShell` 展示内容与导航；未认证时显示 `AuthPanel`，隐藏主导航（`App.vue:137-145`）。没有按角色细分的前端菜单权限证据；当前入口由 owner session 认证门槛控制。

## 路由及页面

| 实际 URL 匹配 | 页面/功能区 | 主要交互 | 数据来源与跳转 |
|---|---|---|---|
| `/` 及所有未匹配路径；`/instances*` 也落在此类 | `InstancesView` 实例列表 | 选择分组、创建分组、按最近对话排序、打开对话/详情/治理、创建实例 | Pinia `conversations` 的实例列表与 `control-center` 分组；进入对话为 `/chat`；详情是全局 Dialog；治理通过 `/instances` 内嵌视图切换。`App.vue:20-25,148-155`、`InstancesView.vue:81-110,302-305` |
| `/chat*` | `ChatView` | 选择发送者、加载更早消息、发送、取消、失败重试/重发/忽略、打开实例详情、返回实例列表 | conversation store 与 BrowserClient；消息/turn 经 Core browser boundary；返回实例列表。`App.vue:148`、`ChatView.vue:27-36,72-105,132-152,184-209` |
| `/moments*` | `MomentsView` | global/当前实例 scope、显示隐藏内容、reaction、评论、隐藏/恢复、媒体播放 | control-center 的 moments API；媒体通过 `/api/media/:id`；不提供手动发布表单。`MomentsView.vue:25-54` |
| `/settings*`，但排除更具体的 `/settings/diagnostics*` | `SettingsView` | overview 与 URL section 切换；每个 section 内 accordion；保存配置/测试连接/复制/导入工作流 | control-center settings/providers API；退出 owner session。`App.vue:21-25,71-77`、`SettingsView.vue:114-146` |
| `/settings?section=kev\|model-role\|reply\|vision\|endpoint\|binding\|media\|operations\|owner` | Settings 九个区 | Kev 决策；模型角色绑定；私聊回复；视觉理解；模型 Endpoint；当前绑定只读；ComfyUI/媒体工作流；运行策略；所有者密码/会话 | section 名来自 `app/navigation.ts:6-16`；实现分布于 `SettingsView.vue`、`KevSettings.vue`、`ReplyVisionSettings.vue`、`MediaSettings.vue`、`OperationalSettings.vue`。
| `/diagnostics*`、兼容路径 `/settings/diagnostics*` | `DiagnosticsView` | 七个诊断 section；过滤、应用/清除筛选、刷新、导出、清空确认；按关联 ID 跳转 | control-center diagnostics API；从 correlation link 回到 model-runs。`App.vue:21-24,28-40`、`DiagnosticsView.vue:131-180,204-217` |
| `/diagnostics?section=kev-decisions\|lifecycle\|agent-runs\|model-runs\|media-prompts\|events\|workflows` | Diagnostics 七个区 | Kev decisions、生命周期、Agent runs、model runs、media prompts、events、workflows | section 定义 `app/navigation.ts:18-26`；生命周期/Agent/媒体等子视图在 `DiagnosticsView.vue` 与 `KevDiagnostics.vue`。有 `correlation_id` 且无 section 时默认 `model-runs`（`App.vue:28-35`）。 |

### 隐藏或非独立页面的视图

- **治理视图不是路由。** `GovernanceView` 在 `InstancesView` 中替换列表内容，关闭/退休后回列表；URL 仍为实例页（`InstancesView.vue:302-305`）。治理入口先加载 detail、autonomy actions、capability requests（`:102-110`）。
- **实例详情是全局 Dialog。** 从实例行、聊天标题/更多打开；包含“基础资料、此刻状态、生活脉络、关系记忆”四个 tab（`InstanceDetailsDialog.vue:323-328`）。详情中的“进入编辑与治理”关闭 Dialog 并切换到实例页治理区（`App.vue:85-110`、Dialog `:581-583`）。
- **创建实例是 Dialog/表单。** 三种模式：空白、描述生成、JSON 导入；预览可以编辑并应用初始目标；最终创建仍走 analyze/activate 接口，成功后选中新实例并打开聊天（`InstancesView.vue:112-139,142-202,205-230,324-341`）。
- **确认对话框/抽屉和高级折叠**分布在治理、设置、诊断、详情内部，不形成 URL。包括退休确认、foundation 提案审查、JSON 查看器、Actor 上下文草稿、Goal/Goal Set 详情和历史、媒体提示词详情、workflow 高级操作等。证据见 `GovernanceView.vue:184-339`、`GoalPanel.vue:63-192`、`GoalPlanningPanel.vue:25-79`、`DiagnosticsView.vue:131-217,240-318`。
- 详情内衣柜数据是单独分页请求，按需展开；不能将详情摘要当作全量库存（`InstanceDetailsDialog.vue:73-132,521-535`）。详情中的 GoalPlanning/GoalPanel 是只读挂载，治理页则挂载可写版本（`:548-551`、`GovernanceView.vue:143-146`）。

## 页面跳转

```text
主导航“聊天” → InstancesView → 选择实例 → /chat
                           ├→ 详情 Dialog → “编辑与治理” → InstancesView 内嵌 GovernanceView
                           └→ 创建 Dialog → activate 成功 → 选择新实例 → /chat
ChatView → 返回 → InstancesView
ChatView / InstancesView correlation → /diagnostics?correlation_id=…&section=model-runs
InstancesView → MomentsView / SettingsView / DiagnosticsView（主导航）
Settings 与 Diagnostics section 切换 → 更新 query section
Diagnostics lifecycle / Agent runs → model-runs（保留 correlation）
```

证据：`App.vue:44-77,85-110,147-155`；`InstancesView.vue:92-110,125-139,278-282,344,377`；`DiagnosticsView.vue:156-165,233-246`。

## 数据、认证与刷新

- `packages/browser-client/src/index.ts` 是浏览器 HTTP 边界，带 credentials/CSRF 处理、结构化错误和 NDJSON turn 请求（`:252-275,187-204`）。对话 turn 由 `conversations` store 解析 token/message/media/completed/error 并在结束后重新查询校准（`stores/conversations.ts:657-793`）。未发现全局 WebSocket 或 EventSource 订阅。
- 两个 Pinia store：`conversations` 管认证、实例、会话、消息、重试与排队；`control-center` 管设置、模型、详情与治理、动态、诊断。数据主要通过 BrowserClient/Go Core API 获取；localStorage 仅用于选择实例/分组以及 turn retry/queue 辅助状态（`stores/conversations.ts:186-256`；`InstancesView.vue:66-79`）。
- 可见页面的轮询是局部且条件化：Chat pending/settling 且页面可见时每 2 秒刷新（`ChatView.vue:115-122`）；Diagnostics 可见时每 2 秒刷新（`:167-180`）；visual identity Dialog 打开、可见且非发送/终态时每 5 秒刷新（`InstanceDetailsDialog.vue:280-297`）。
- 图片/视频/音频展示通过 API media URL，不直连 ComfyUI；runtime `apiOrigin` 来自全局 runtime config，否则当前 origin（`runtime-config.ts:9-14`；`ChatView.vue:79`、`MomentsView.vue:17,51`、`InstanceDetailsDialog.vue:41`）。
- Nginx 提供 SPA fallback，转发 `/api,/auth,/health` 到 Core，并对代理流关闭 buffering、配置较长 read timeout（`apps/web/nginx.conf:18-35`）。
