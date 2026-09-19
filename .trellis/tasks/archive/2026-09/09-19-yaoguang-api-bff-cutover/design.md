# 第四阶段技术设计：浏览器接入并入 Go API

## 1. Baseline and target topology

前三阶段的基线是当前分支 `codex/yaoguang-adk-phase2`：Eino/ADK、ConversationRuntime、WakeUp 共享 ADK runtime、Core App/Worker/Temporal/Redis/PostgreSQL 已有正式边界；本阶段不改变这些职责。

当前运行链路：

```text
Browser/Vue
  -> apps/gateway-go public boundary
  -> apps/core-go /internal HTTP
  -> Core App/Repository
```

目标运行链路：

```text
Browser/Vue
  -> apps/core-go public browser boundary (/auth, /api, /health)
  -> browser DTO/auth/CSRF/NDJSON/media transport layer
  -> explicit in-process browser service adapter
  -> existing Core App/Repository
  -> PostgreSQL/Redis/MinIO/Temporal/Provider
```

`/internal/*` 保持机器/内部合同和授权，不因为合并进程而自动公开。`apps/core-go/cmd/worker` 与 Temporal Worker 不合并到 API。

## 2. Package boundaries

- `apps/core-go/internal/httpapi/browser`：从现有 Go browser boundary 迁入的纯浏览器传输组件。它拥有公共路由 dispatch、Cookie/CSRF/CORS、请求校验、browser DTO、错误脱敏、NDJSON translator 和媒体响应头白名单；不导入 PostgreSQL、Redis、Temporal、Provider 或 domain repository。
- `apps/core-go/internal/httpapi`：组合 API server，并实现 `browser.Backend`。Backend 是明确的应用服务桥接，不携带 URL、HTTP client、service key 或伪请求；每个 operation 映射到既有 `*core.App`/`core.Repository` 方法。
- `apps/core-go/internal/core`：继续拥有 Session、授权、领域写入、事务、幂等、ADK/Task/Worker 逻辑。浏览器 boundary 只传入经过 Session 解析得到的 actor ID。
- `apps/web` / `packages/browser-client`：继续使用唯一 BrowserClient；只切换 API origin/开发代理和统一媒体 URL，不复制路由调用。

Browser boundary 的 Backend 是同进程 operation adapter；路由清单中的 route-shaped key 只用于稳定映射和审计，不是 URL，也不会被 `http.Client` 发送。对话和媒体使用 writer-aware service 方法：App 直接向 boundary 提供的协议 writer 产出，边界在同一进程翻译/过滤，不调用另一个 HTTP handler。

## 3. Authentication and request context

浏览器 boundary 保留：

1. `fluctlight_session` opaque HttpOnly/Secure/SameSite=Lax cookie；成功 login/setup 必须得到非空 token。
2. `fluctlight_csrf` 非 HttpOnly cookie；所有非 GET/OPTIONS mutation 要求精确 Trusted Origin 和 constant-time 双提交 token。
3. `OPTIONS` 只允许显式 Trusted Origin/Credentials/headers；不放宽 wildcard CORS。
4. `Backend.ResolveSession` 调用 Core repository/App 权威解析；失败统一 401，并在公共响应清理 Session cookie；actor/user/body 字段只作业务参数，不能覆盖认证主体。
5. `/internal/*` 仍要求 service identity；公共 `/auth`/`/api` 不要求浏览器提交 service key。服务身份和 Human Session 保持独立。

Trusted Origin 在 API 启动配置中做 URL/origin 形状校验；缺少安全配置时启动失败，不进入 anonymous mode。公共错误只暴露稳定 code/message 和 bounded safe details。

## 4. Route and DTO migration

所有 72 个 public path/method 组合沿用现有 Browser OpenAPI。迁移表记录：

```text
public method/path
  -> browser handler + validation/auth
  -> Backend operation
  -> existing App/Repository method
  -> browser DTO/status/error
  -> test/e2e evidence
```

核心特殊边界：

- `/api/conversations/:id/turn`：camelCase request 映射为 App payload；App.StreamTurn 直接写入 incremental protocol writer；browser NDJSON translator 保持 `message/token/media/completed/error/heartbeat`、sequence、terminal uniqueness、redaction 和 cancellation。
- `/api/media/:assetId`：先由 App.AuthorizeAsset 校验 actor，再 App.ServeMedia；只转发允许的 Range/ETag/Content-Type/Length headers，禁止对象存储地址和空 body 成功。
- diagnostics/workflows：继续使用 Core App 的 owner authorization 和 bounded filters；query field mapping 明确转换，不做递归 key rewrite。
- history/direct conversation：保持 participant/owner authorization 和 `messages`/`history` 的公共字段转换。

## 5. Deployment and client cutover

- 删除 `apps/gateway-go` module/image/service/healthcheck、BFF-only env、CoreBaseURL 和 gateway CI matrix。
- Core API 通过 Web Nginx 同源 proxy 承接公共路由；Core 不需要 host port，Web 仅依赖 Core readiness。Nginx 只负责基础反向代理，不承载业务 DTO/auth logic。
- Runtime config/API client 统一使用 `apiOrigin`（默认同源只在明确开发配置下启用）；不保留旧 BFF origin fallback 或失败重试。
- Vite proxy、Compose、CI、acceptance scripts、README、MinIO/media docs 和 OpenAPI route checks 全部指向 API。
- `packages/browser-client` 的 OpenAPI artifact 与 generated client 只从新的 API route inventory 生成，保持 public path 不变时不人为增加第二套路径。

## 6. Compatibility and rollback

回退以版本级部署回退为准：上一版本仍可独立部署 BFF，但新代码不保留旧 BFF 二进制、feature flag、双地址或自动 fallback。迁移不改持久数据、Session token 格式、资源 key 或 Temporal history。切换前通过隔离 Compose smoke 验证；切换失败回退整个版本，不在新版本内恢复双运行路径。

## 7. Security and failure invariants

- Core/DB/Provider error 不泄漏 stack、prompt、reasoning、secret、raw response；NDJSON 无效序列只产生一个 bounded terminal error。
- 请求取消只停止尚未完成的同步传输/Provider context；已提交消息、异步 intent 和 Temporal 生命周期保持原语义。
- Session 401 清理 cookie；业务 403/404/409/422/502/503 按现有 public contract 映射。
- API 进程启动时必须拥有服务需要的安全配置；不能通过关闭 CSRF、Origin 或 service/session 检查使测试通过。
