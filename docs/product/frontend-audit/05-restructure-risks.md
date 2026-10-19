# 05 — 前端重构必须保留的能力与兼容约束

本文列出基于现有代码/API 的保留约束。它不指定视觉风格或新组件结构。全路径相对仓库根目录；`core/...`、`migrations/...` 等简写相对 `apps/core-go/internal/`，`InstanceDetailsDialog.vue` 相对 `apps/web/src/components/instances/`。

## 路由、导航与可访问性

1. 当前 view 选择由 `App.vue:20-35` 手动 pathname/query 解析，section registry 同时服务菜单和 URL 校验（`app/navigation.ts:6-43`）。迁移路由时须保留 `/chat*`、`/moments*`、`/settings*`、`/diagnostics*` 的现有入口与 section IDs。
2. `/settings/diagnostics*` 是兼容入口（`App.vue:21-24`），`correlation_id` 无 section 时要落到 `model-runs`（`:28-35`）。未命中路径现在 fallback 到 Instances，不应无意中变成空白或破坏历史 deep link。
3. 当前 URL 不序列化实例 ID、Moments scope、实例创建 mode；这些状态存在 store/component，而不是 `WorkspaceRoute` 类型就代表可恢复 deep link（`app/navigation.ts:28-33`）。原型若承诺分享/刷新恢复，需要单独设计并同步实现。
4. 治理视图是 `/instances` 内嵌模式，详情是全局 Dialog；两者之间的跳转会关闭详情并加载治理依赖（`App.vue:85-110,155`、`InstancesView.vue:302-305`）。导航重构不能只照搬顶级路由表而遗失这些隐藏面板。

## API 契约、授权与操作闭环

1. `BrowserClient` 负责 credentials、CSRF header、错误规范化、NDJSON turn（`packages/browser-client/src/index.ts:252-275,187-204`）。重构 API 层时保留 cookie session/CSRF 和 `BrowserApiError` 可读错误语义；不要绕过 Core browser boundary。
2. 聊天流是 HTTP POST + NDJSON；客户端处理 token/message/media/completed/error，检查 turnId/sequence，完成后再拉消息校准（`stores/conversations.ts:657-793`）。取消有真实 cancel API；单纯导航只 invalidate 本地 observer（`:816-914`）。不要把“停止读取流”当作服务端取消。
3. 重试使用 idempotency/turn identity；sender 时间随 durable turn 冻结并在 retry 复用（`conversations.ts:259-274,535-546`）。发送者可由 active participants 切换；失败的部分提交与可重试状态要保持差别。
4. Goal、Actor context、Foundation、Relationship、Memory、Schedule 和 Workflow 等命令携带 expected revision/version、reason、evidence refs、idempotency 或具体 history point。迁移表单时不能丢掉 CAS/冲突恢复/原因与来源链。
5. server snapshots（detail、Goals、Memory、current context、diagnostics）由 Core API 生成；前端不能把本地 draft 或历史 Foundation 字段视作当前权威状态。Goal revisions/evidence 与 Memory provenance 链应保留可追踪入口。

## 前端状态、缓存和异步安全

1. 当前两个 Pinia store 所属边界为 `conversations`（auth/instances/conversation/messages/turn retry）和 `control-center`（settings/providers/detail/governance/moments/diagnostics）。状态分布在 store 与 view/local drafts；不要在迁移中复制两份 server authority。
2. 已知 localStorage keys：`fluctlight.selected-instance-id`、`fluctlight.retry-turn.v2`、`fluctlight.queued-turn.v1`（`stores/conversations.ts:186-256`）；`fluctlight.selected-group-id`（`InstancesView.vue:66-79`）。只持久化的应理解为辅助选择/队列状态，不是核心领域数据。
3. detail、actor draft、diagnostics source 各用 requestId/target ID/epoch/cursor 防止旧请求覆盖当前选择（`control-center.ts:216-318,410-452`）。衣柜分页切换实例/关闭会重置（`InstanceDetailsDialog.vue:73-132`）；Settings 同时加载的请求用 WeakMap 合并（`control-center.ts:25,1073-1085`）。
4. 登出和密码变更会清 conversation/retry/queue（`conversations.ts:412-450`）；审计未找到 control-center 对称 reset。重构需复核跨认证会话的数据隔离，不能因复用全局 store 泄露前一会话的缓存。
5. 轮询条件与间隔属于功能约束：chat pending/settling、diagnostics 可见、visual identity dialog 可见且未终态；当前为局部轮询，不是全局 realtime bus。若重构成统一刷新层，要保留 visibility、取消和 request epoch 条件。

## 媒体、上传与浏览器能力

1. 媒体读取走经过鉴权的 `/api/media/:assetId`，runtime config 决定 apiOrigin；Nginx `/api` 代理关闭 buffering 以支持 turn stream（`runtime-config.ts:9-14`、`apps/web/nginx.conf:18-30`）。不要在浏览器改为直连 ComfyUI 或裸对象存储。
2. Chat 和 Moments 显示 image/video/audio；详情显示视觉身份候选/参考图/角色卡；Settings 的工作流文件读取先形成本地 draft，只有保存才提交（`MediaSettings.vue:25-71`）。导入文件并不等于上传媒体。
3. 尚未找到用户附件选择/上传闭环；turn 内部可以传 `attachmentRefs` 不能作为上传能力证据。原型需先确定上传 API、引用生命周期、撤销、错误恢复和权限后，才能认定可复用入口。

## 领域 authority 与兼容行为

1. Foundation/Persona、effective current state、Developing Self 和 temporary inner state 各自独立；unknown/cleared 不回退到 Foundation，body/wardrobe 与 profile-scoped habits 生命周期不同（`.trellis/spec/backend/persona-layer-contract.md:38-43`、`core/effective_life.go:14-18,40-52`）。重构详情页时需标清来源和时间语义。
2. Schedule 是 life plan，不是 cron；Event、Presence、Activity authority 与 result provenance 分别处理（`.trellis/spec/backend/fluctlight-life-world-contract.md`；`core/life_activity_authority.go:15-38`）。Scene/Location 当前是文本上下文，不是目录资源。
3. Inventory ownership、wearing、outfit、ordinary item use 是不同关系；“未列出”不等于“不存在”（`migrations/effective_life.go:51-94`、`inventory_usage.go:14,24`、`InstanceDetailsDialog.vue:530`）。不能通过单一“物品状态”字段覆盖所有关系。
4. Conversation attachments、Memory source links、Goal evidence、Reflection proposals都保留各自 source/revision/fingerprint/provenance。若 UI 归并为统一活动流，应保留原 domain source type 与权威 ID。
5. Model configuration 当前可见角色只有 generic_llm、embedding、conversation_reply、vision_understanding；provider assignment 对遗留 generic role row 有明确兼容选择（`core/settings.go:340-344`、`provider.go:103,113,119-121`）。不要从旧文档/UI 截图恢复不存在的旧角色 tab。
6. 人格切换由对话 Core runtime 的受授权 `persona.switch` capability 执行（`core/persona_action_capabilities.go:14`、`core/builtin_capabilities.go:164`）；前端没有直接 profile switch control。重构时保留 Chat turn、Kev decision 审计与 profile authority 的边界，不能把它改造成前端修改运行时或改写 Agent 逻辑。

## 分散、遗留与待核验入口

- 同一实例详情分布在 Instances row、Chat 标题、Desktop context panel；治理与详情互相跳转。重构导航须保留每条现有进入路径。
- Model configuration、visual identity 与 Diagnostics 分散于 Settings、Details、Diagnostics；Media prompt retry 与 visual asset review 不是同一入口。
- `removeActorGroupMember` 有 client/store 操作但 UI 未见移除控件；`createConversation` 在 client 有方法但页面调用链未找到；`WorkspaceRoute` 有未序列化字段。它们属于真实差异点，不可因新页面清单遗漏而删除，也不可宣称已有可操作页面。
- `.trellis/spec/frontend/state-management.md` 有旧 personas/localStorage/SSE/debug inspector/API 路径，当前运行代码不一致。现状应以 `apps/web/src`、`packages/browser-client`、Go browser route 为准；旧 spec 只作为文档漂移记录，不作为当前行为。
- 当前已有产品能力不因导航改版自动失效。改动前要逐一映射本审计 Feature ID，确认替代入口、数据读写和回退路径。

## 回退要求

如果页面迁移后出现服务端状态不同步、流式 turn 不完整、错误无法恢复、context/detail 请求串写、media 不可读或旧 deep link 失效，应能恢复到旧 route / store 行为；不要同时切换前端字段语义与后端 schema。实际发布策略需下一阶段制定，本审计不设计 rollout。
