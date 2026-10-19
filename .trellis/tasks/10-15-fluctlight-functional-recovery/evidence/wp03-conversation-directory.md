# WP-03 会话目录、当前选择、全历史搜索与已读（2026-10-16）

本切片交付真实 Go Core → in-process browser boundary → BrowserClient 契约，前端由 T03 切片接入。没有数据库迁移、付费模型调用或旧任务库重置。

## 实现边界

复用 `fluctlight_direct_conversations` 保存 Owner + Fluctlight 的当前选择，复用 conversations/participants/heads/read_positions 保存历史。目录只包含 Owner 创建、且 active participants 恰为 Owner 与该 Fluctlight 的两人会话；不会返回另一 Owner、另一 Fluctlight 或群聊记录。普通旧两人会话仍可选，新接口不会创建新 Fluctlight，也不写 Memory/Relationship/Goal/生活状态。

新建在同一事务内写 conversation/head/participants/read positions/current pointer，命令回执复用 owner_surface_idempotency。相同 key 与相同请求返回原会话；更改 payload 返回 409 `conversation_command_conflict`。旧新建请求在后续选择后重放不会重新改动当前选择。首次 legacy get-or-create 改为锁定稳定 Fluctlight 行，避免不存在映射行的并发首次创建冲突。

目录和搜索用 (created_at,id) 向前翻页。cursor 绑定 Owner、Fluctlight、查询种类和搜索词。搜索在数据库完整历史上做不区分大小写的字面子串匹配，不将 `%` 或 `_` 解释为搜索通配符。分页 limit 默认 50、上限 200。目录返回真实未读计数、读/送达游标、最后消息。角色目录未读也改为真实读游标的跨会话聚合，而非累计全部回复。

精确来源消息回读保留 conversationId/messageId/sequence/text/turn identity；浏览器消息含 `sourceRef: "message:" + id`。历史与搜索均复用 History 的 authoritative durable turn 状态投影。标已读保持单调与幂等，省略 deliveredSequence 时默认 readSequence，拒绝超前超过真实最大消息序号；active participant 权限仍由服务端校验。

## 浏览器接口

| 方法/路径 | 请求 | 响应 / Client |
| --- | --- | --- |
| GET `/api/fluctlights/{fid}/conversations` | `?limit=50&cursor=...` | `{items:[{conversation,unreadCount,lastReadSequence,lastDeliveredSequence,lastMessage?}],currentConversationId,nextCursor}`；`conversationDirectory(fid,{limit,cursor})` |
| POST `/api/fluctlights/{fid}/conversations` | `{idempotencyKey,title?}` | `BrowserConversationPage`；`newDirectConversation(fid,body)` |
| POST `/api/fluctlights/{fid}/conversations/{cid}/select` | `{}` | `BrowserConversationPage`；`selectDirectConversation(fid,cid)` |
| GET `/api/fluctlights/{fid}/messages/search` | `?q=文字&limit=50&cursor=...` | `{items:BrowserMessage[],nextCursor}`；`searchConversationMessages(fid,q,{limit,cursor})` |
| GET `/api/conversations/{cid}/messages/{mid}` | 无 | `BrowserMessage`；`conversationMessage(cid,mid)` |
| POST `/api/conversations/{cid}/read` | 既有 `{readSequence,deliveredSequence?}` | 既有 204；`markRead` |
| GET `/api/fluctlights/{fid}/conversation` | 既有接口 | 返回持久当前选择；`directConversation(fid)` |

所有新 mutation 继承 cookie + Origin + CSRF。内部同路径 `/internal` 提供 snake_case 请求/响应并在 CoreClient 同步方法。Turn POST+NDJSON/显式 cancel/重试/Worker 职责未改。

## 实际验证

使用 `/tmp/fluctlight-recovery-1015/envexec.py` 的 admin 连接，每个测试自动创建随机临时数据库、应用当前迁移、测试后删除；没有重置现有任务数据库。

- `TestDirectSessionDirectoryPersistsSelectionScopesSearchAndReadPositions`：双 Owner/双角色范围；五个会话目录 2 条/页；真实 AcceptTurn 接受 13 条用户历史；history 1 条/页、搜索 3 条/页，无重复/漏项；精确来源一致；选择→重建 App→legacy 当前读回；新建幂等/冲突；旧命令重放不改后续选择；已读幂等与真实未读数；拒绝未来游标；Fluctlight/inner state 完整行未变化。初始 incoming assistant 历史为明确 SQL fixture，待验收的标已读与未读读取均走实际服务。
- `TestEnsureDirectConversationConcurrentFirstUse`：6 个并发旧 get-or-create 返回同一会话。
- `TestConversationBrowserDirectoryUsesPersistentCurrentSelection`：实际 HTTP→browser boundary→App→PG 创建/选择/目录/搜索/精确回看/已读；409 原样保留稳定 code；拒绝坏 cursor 与伪造 participant 范围。仅 opaque session resolution 是测试 seam，其余为生产实现。
- 上述 Core/HTTP 测试 `-race` 全过（Core 4.753s、HTTP 3.438s，无 skip）。
- 既有 `TestDurableBrowserObserverDisconnectDoesNotCancelWorker`、`TestExplicitCancelIsRetryableWithoutDuplicateUserMessage`、`TestProviderFailureExposesRetryableTurnAndReusesUserMessage`、`TestNativeProseFailureDurableRetryPreservesSenderIdentity`、`TestFailedConversationHistoryRetainsCommittedMutationRetryFence` 及同批 retry 选择集全部通过，受控 Provider，无付费模型。
- BrowserClient 全 30 个测试和 typecheck 通过；CoreClient typecheck 通过。
- Core/browser/httpapi 编译通过，Core/browser/httpapi `go vet` 通过；新会话相关浏览器路由矩阵全部通过。
- `infra/acceptance/check-core-openapi.sh` 通过。
- 浏览器包全量 `-race` 在本次快照仅因并行 world/catalog 路由的 OpenAPI artifact 漂移失败（`GET /api/fluctlights/fl-1/world/catalog/world-1`），不是本切片测试失败；交付主线程最终生成后复跑。

## 后续集成与限制

前端通过上述 typed Client 方法接历史面板/搜索/新建/选择；本切片没有改前端。需要主线程统一最终 generate 与全栈验收。查询为完整历史的数据库字面搜索，不包含未提交草稿或隐藏 cognition。大目录/搜索每页为有界查询，命中逐条复用既有 History 投影；本轮未做大数据量性能验收或付费 live Provider 验收。原 UI 范围移除图片发送，未恢复附件发送入口。
