# 第四阶段迁移与验收记录：浏览器接入并入 Go API

## 1. Baseline

- 前置阶段：Eino/ADK 基础、ConversationRuntime、WakeUp 共享 ADK、Temporal/Worker/Outbox/领域事务均保持原实现。
- 原拓扑：Web → `apps/gateway-go` → Core `/internal/*` HTTP。
- 目标拓扑：Web/Nginx（静态资源和基础同源 proxy）→ Core API `/auth/*`、`/api/*`、`/health/*` → browser transport boundary → in-process App/Repository。
- 当前 worktree：`/Users/vinson/Documents/project/个人/local-ai-companion-yaoguang-eino-adk`；分支 `codex/yaoguang-adk-phase2`。

## 2. Route / responsibility migration table

公共路径和方法保持 `packages/browser-client/openapi.json` 的正式合同。下面的表逐项列出当前 artifact 中的操作；“Backend”不再是 URL client，而是同进程的明确 operation adapter。

| Method | Public path | Browser target | Direct service / downstream | Verification |
|---|---|---|---|---|
| GET | `/health/live` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/health/ready` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/platform/ping` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/auth/setup` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/auth/setup-status` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/auth/login` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/auth/logout` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/auth/session` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/auth/revoke-all` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/auth/password` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/settings` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/settings` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/capability-requests` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/capability-requests/{requestId}/review` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/providers/endpoints` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/providers/endpoints` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/providers/endpoints/{endpointId}/models` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/providers` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/providers/roles` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/conversations` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/actor-groups` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/actor-groups` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/actor-groups/{groupId}/members` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| DELETE | `/api/actor-groups/{groupId}/members/{actorId}` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights/{fluctlightId}` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights/{fluctlightId}/conversation` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights/{fluctlightId}/moments` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/moments/read` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/moments` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/memories/{memoryId}` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/memories/{memoryId}/forget` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/relationships/rollback` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/fluctlights/{fluctlightId}/relationships/{targetActorId}` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights/{fluctlightId}/autonomy-actions` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/autonomy-actions/{actionId}/govern` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/events` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/events/{eventId}/cancel` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/fluctlights/{fluctlightId}/presence` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/schedules` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/cancel` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/workflows` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/workflows/{workflowId}/status` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/workflows/{workflowId}/history` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/diagnostics/workflows/{workflowId}/pause` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/diagnostics/workflows/{workflowId}/resume` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/diagnostics/workflows/{workflowId}/cancel` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/diagnostics/workflows/{workflowId}/reset` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/diagnostics/workflows/{workflowId}/restart` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights/{fluctlightId}/detail` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/fluctlights/{fluctlightId}/developing-self` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/developing-self/{claimId}/rollback` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/developing-self/{claimId}/forget` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| PUT | `/api/fluctlights/{fluctlightId}/status` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/retire` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/foundation-revisions` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}/accept` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}/reject` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlights/{fluctlightId}/foundation-revisions/rollback` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/moments/{momentId}/comments` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/moments/{momentId}/reactions` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/moments/{momentId}/hide` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/moments/{momentId}/restore` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlight-creations/analysis` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/fluctlight-creations/activate` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/conversations/{conversationId}/messages` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/conversations/{conversationId}/read` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/conversations/{conversationId}/turn` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| DELETE | `/api/diagnostics` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/lifecycle` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/model-runs` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/media-prompts` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| POST | `/api/diagnostics/media-prompts/{mediaIntentId}/retry` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/diagnostics/export` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |
| GET | `/api/media/{assetId}` | browser.Handler + Backend | existing Core App/Repository service | browser-client + route matrix |

认证/安全职责统一在 `internal/httpapi/browser`：

- `fluctlight_session` opaque HttpOnly/Secure/SameSite=Lax Cookie；Core Repository 解析 hash/过期/撤销。
- `fluctlight_csrf` 双提交 token、精确 Trusted Origin、OPTIONS/CORS、请求体限制和 public DTO 校验。
- 失效 Session 统一 401 并清理 cookie；login/setup 成功必须得到非空 session token；logout 对失效 Cookie 仍幂等清理。
- actor/user/sender 字段只是业务参数，认证主体只来自 Core 验证 Session；内部 `/internal/*` 仍要求 service identity。

## 3. Direct application paths

典型调用关系：

- 认证：browser → `browserBackend` → `App.Login/Setup/Revoke*/ResetPassword`、`Repository.ResolveSession`。
- 设置/provider：browser → `App.ReadSettings/UpdateSettings/ProviderEndpoints/ProviderBindings/ConfigureProvider*`。
- 实例/会话：browser → `Repository.List/GetFluctlights/History/DirectConversationID`、`App.CreateFluctlight/EnsureDirectConversation/CreateConversation/MarkRead`。
- 治理/生活世界：browser → `App.SetFluctlightStatus`、Foundation/Memory/Relationship/Life Event/Presence/Schedule/Moment/Autonomy/Capability Request methods。
- 诊断/workflow：browser → `App.DiagnosticsFiltered/LifecycleDiagnostics/ModelRunsFiltered/MediaPromptsFiltered/DiagnosticsExportFiltered/Workflow*`。
- 对话：browser → `App.StreamTurn`；pipe writer 只作为同进程协议适配，`TranslateCoreNDJSON` 增量转换，不发起 HTTP 请求。
- 媒体：browser → `App.AuthorizeAsset` → `App.ServeMedia`，只转发允许的 Range/ETag/MIME/Length，绝不暴露 MinIO key/credential。

## 4. Removed BFF / forwarding artifacts

- 删除 `apps/gateway-go/README.md`、Dockerfile、`cmd/gateway`、独立 Go module、配置和平台 health package。
- 删除 BFF route/core HTTP client、Core service-key/session HTTP forwarding、旧 NDJSON/media proxy 进程；传输组件迁入 Core API browser package。
- 删除 Compose 的 BFF service、镜像/端口/healthcheck/memory limit、`CORE_BASE_URL`、BFF origin/port/image 变量。
- 删除 CI 的 gateway test/vet/build/gofmt/cache/matrix；镜像只保留 Core 和 Web。
- Web runtime/Vite 统一使用可选 `apiOrigin`/同源入口；没有旧地址失败重试或双轨 fallback。

## 5. Deployment and client changes

- Compose Core 保持不 host expose；Web Nginx 只将 `/api`、`/auth`、`/health` proxy 到 `core:8080`。
- `FLUCTLIGHT_TRUSTED_ORIGIN` 现在是用户实际打开的 Web origin；缺失或非法 origin 时 API/Worker 启动失败。
- BrowserClient 路径与字段合同不变；Vue stores 仍是唯一 API 调用方，媒体 URL 统一经过 `apiOrigin`，包括视觉身份详情。
- API 与 Worker 仍是独立进程；Temporal、Redis、PostgreSQL、MinIO 和长任务配置未被合并或删除。

## 6. Verification evidence

已执行/应执行的门禁：

- `go test -mod=readonly ./...`
- Core API/browser boundary race、`go vet ./...`、`go mod tidy -diff`、`gofmt`、`git diff --check`
- browser boundary route matrix + OpenAPI inventory、auth/CSRF/NDJSON/no-HTTP-hop tests
- `pnpm --filter @fluctlight/web typecheck`
- `pnpm --filter @fluctlight/web test`
- `pnpm --filter @fluctlight/web build`
- `pnpm --filter @fluctlight/browser-client test`
- Compose config/smoke and real browser login → read → conversation → output → logout（若环境可用）

本轮已执行 Go/Web/browser-client、OpenAPI、Compose config 和静态删除门禁；真实
PostgreSQL、Temporal、Provider、MinIO、Docker Compose smoke 和可用浏览器环境未在
当前会话提供，因此登录→读取→对话→消费输出→退出的真实浏览器验收标记为未验证，
不把 Fake 或 curl 结果描述成完整生产验收。
